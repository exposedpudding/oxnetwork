package backup

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
)

// newDB creates a migrated database with representative rows, including session data.
func newDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "oxnet.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := database.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec(`INSERT INTO inventory_nodes(id,name,role,platform,updated_at,addresses_json) VALUES('n1','Node One','Compute','linux','2026-10-18T12:00:00Z','[]')`)
	exec(`INSERT INTO workflows(id,name,enabled,definition_json,simulated,created_at,updated_at) VALUES('wf_1','Check',1,'{}',0,'2026-10-18T12:00:00Z','2026-10-18T12:00:00Z')`)
	exec(`INSERT INTO audit_events(occurred_at,actor,action,target_type,target_id,outcome) VALUES('2026-10-18T12:00:00Z','tester','workflow.created','workflow','wf_1','succeeded')`)
	exec(`INSERT INTO auth_sessions(token_hash,issuer,subject,email,csrf_token,created_at,last_seen_at,expires_at) VALUES('h','i','s','e@x','SECRET-CSRF',1,1,99999999999)`)
	exec(`INSERT INTO oidc_login_flows(state_hash,nonce,code_verifier,return_path,expires_at) VALUES('s','n','v','/',99999999999)`)
	return path, database
}

func TestCreateProducesVerifiedSessionFreeBackup(t *testing.T) {
	path, _ := newDB(t)
	dest := filepath.Join(t.TempDir(), "backups", "oxnet-1.db")
	info, err := Create(context.Background(), path, dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != dest || info.Integrity != "ok" || info.MigrationVersion != db.SchemaVersion() || info.Bytes == 0 || len(info.SHA256) != 64 {
		t.Fatalf("info: %+v", info)
	}
	if info.Rows["inventory_nodes"] != 1 || info.Rows["workflows"] != 1 || info.Rows["audit_events"] != 1 || info.Rows["auth_sessions"] != 0 {
		t.Fatalf("rows: %+v", info.Rows)
	}
	stat, err := os.Stat(dest)
	if err != nil || stat.Mode().Perm()&0o077 != 0 {
		t.Fatalf("backup must be owner-only: %v %v", stat, err)
	}
	sum, err := os.ReadFile(dest + ".sha256")
	if err != nil || !strings.HasPrefix(string(sum), info.SHA256+"  oxnet-1.db") {
		t.Fatalf("checksum file: %q %v", sum, err)
	}
	for _, leftover := range []string{dest + ".partial", dest + "-wal", dest + "-shm"} {
		if _, err := os.Stat(leftover); err == nil {
			t.Fatalf("leftover file %s", leftover)
		}
	}
	// The sensitive session values are not in the file at all, not merely deleted rows.
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET-CSRF") {
		t.Fatal("session secret found in backup bytes")
	}
	// The source is untouched.
	var sessions int
	source, _ := sql.Open("sqlite3", path)
	defer source.Close()
	if err := source.QueryRow(`SELECT COUNT(*) FROM auth_sessions`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("source sessions: %d %v", sessions, err)
	}
}

func TestCreateRefusesExistingDestinationAndMissingSource(t *testing.T) {
	path, _ := newDB(t)
	dest := filepath.Join(t.TempDir(), "b.db")
	if _, err := Create(context.Background(), path, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(context.Background(), path, dest); !errors.Is(err, ErrExists) {
		t.Fatalf("second create: %v", err)
	}
	if _, err := Create(context.Background(), filepath.Join(t.TempDir(), "none.db"), filepath.Join(t.TempDir(), "x.db")); !errors.Is(err, ErrNoSource) {
		t.Fatalf("missing source: %v", err)
	}
	// A failed backup leaves nothing behind and does not create the source.
	missing := filepath.Join(t.TempDir(), "absent.db")
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("missing source should not have been created")
	}
}

func TestCreateWhileApplicationIsWriting(t *testing.T) {
	path, database := newDB(t)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_, _ = database.Exec(`INSERT INTO audit_events(occurred_at,actor,action,target_type,target_id,outcome) VALUES('2026-10-18T12:00:00Z','writer','load.test','workflow','wf_1','succeeded')`)
		}
	}()
	time.Sleep(20 * time.Millisecond)
	info, err := Create(context.Background(), path, filepath.Join(t.TempDir(), "live.db"))
	close(stop)
	wg.Wait()
	if err != nil || info.Integrity != "ok" || info.Rows["audit_events"] < 1 {
		t.Fatalf("backup during writes: %+v %v", info, err)
	}
}

func TestBackupKeepsAuditAppendOnlyTriggers(t *testing.T) {
	path, _ := newDB(t)
	dest := filepath.Join(t.TempDir(), "b.db")
	if _, err := Create(context.Background(), path, dest); err != nil {
		t.Fatal(err)
	}
	copyDB, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(dest))
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if _, err := copyDB.Exec(`DELETE FROM audit_events`); err == nil {
		t.Fatal("audit triggers must survive the backup")
	}
}

func TestVerifyDetectsCorruptionChecksumMismatchAndForeignFiles(t *testing.T) {
	path, _ := newDB(t)
	dir := t.TempDir()
	dest := filepath.Join(dir, "b.db")
	if _, err := Create(context.Background(), path, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), dest); err != nil {
		t.Fatalf("good backup: %v", err)
	}
	// Flip bytes in the middle: the checksum file catches it even if SQLite still reads the file.
	raw, _ := os.ReadFile(dest)
	flipped := append([]byte(nil), raw...)
	flipped[len(flipped)-1] ^= 0xFF
	flippedPath := filepath.Join(dir, "flipped.db")
	if err := os.WriteFile(flippedPath, flipped, 0o600); err != nil {
		t.Fatal(err)
	}
	sum, _ := os.ReadFile(dest + ".sha256")
	if err := os.WriteFile(flippedPath+".sha256", sum, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), flippedPath); err == nil {
		t.Fatal("a modified backup must fail verification")
	}
	// Truncated file.
	truncated := filepath.Join(dir, "truncated.db")
	if err := os.WriteFile(truncated, raw[:len(raw)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), truncated); err == nil {
		t.Fatal("a truncated backup must fail verification")
	}
	// Not an oxnet database.
	other := filepath.Join(dir, "other.db")
	o, _ := sql.Open("sqlite3", "file:"+filepath.ToSlash(other))
	if _, err := o.Exec(`CREATE TABLE t(x)`); err != nil {
		t.Fatal(err)
	}
	o.Close()
	if _, err := Verify(context.Background(), other); !errors.Is(err, ErrNotOxnet) {
		t.Fatalf("foreign database: %v", err)
	}
	notDB := filepath.Join(dir, "text.db")
	_ = os.WriteFile(notDB, []byte("hello, this is not sqlite"), 0o600)
	if _, err := Verify(context.Background(), notDB); err == nil {
		t.Fatal("garbage must fail verification")
	}
	if _, err := Verify(context.Background(), filepath.Join(dir, "missing.db")); !errors.Is(err, ErrNoSource) {
		t.Fatalf("missing: %v", err)
	}
}

func TestVerifyRefusesBackupFromNewerSchema(t *testing.T) {
	path, database := newDB(t)
	if _, err := database.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, CURRENT_TIMESTAMP)`, db.SchemaVersion()+1); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "future.db")
	if _, err := Create(context.Background(), path, dest); err == nil || !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("a newer-schema database should not produce a restorable backup: %v", err)
	}
}

func TestRestoreReplacesDatabaseKeepsSafetyCopyAndRunsMigrations(t *testing.T) {
	path, database := newDB(t)
	ctx := context.Background()
	backupFile := filepath.Join(t.TempDir(), "good.db")
	if _, err := Create(ctx, path, backupFile); err != nil {
		t.Fatal(err)
	}
	// The live database moves on after the backup.
	if _, err := database.Exec(`INSERT INTO workflows(id,name,enabled,definition_json,simulated,created_at,updated_at) VALUES('wf_2','Later',1,'{}',0,'2026-10-19T12:00:00Z','2026-10-19T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	database.Close()

	if _, err := Restore(ctx, backupFile, path, RestoreOptions{}); !errors.Is(err, ErrTargetExists) {
		t.Fatalf("restore over an existing database must need force: %v", err)
	}
	fixed := time.Date(2026, 10, 19, 8, 30, 0, 0, time.UTC)
	result, err := Restore(ctx, backupFile, path, RestoreOptions{Force: true, Now: func() time.Time { return fixed }})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result.SafetyCopy, ".pre-restore-20261019T083000Z") || result.Restored.Rows["workflows"] != 1 {
		t.Fatalf("result: %+v", result)
	}
	// The safety copy still holds the later workflow, so a mistaken restore is reversible.
	safety, err := Verify(ctx, result.SafetyCopy)
	if err != nil || safety.Rows["workflows"] != 2 {
		t.Fatalf("safety copy: %+v %v", safety, err)
	}
	for _, leftover := range []string{path + ".restore-tmp", path + "-wal", path + "-shm"} {
		if _, err := os.Stat(leftover); err == nil {
			t.Fatalf("leftover %s", leftover)
		}
	}
	// The restored file opens normally, migrations are idempotent, and sessions were not restored.
	reopened, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var workflows, sessions int
	if err := reopened.QueryRow(`SELECT COUNT(*) FROM workflows`).Scan(&workflows); err != nil || workflows != 1 {
		t.Fatalf("workflows: %d %v", workflows, err)
	}
	if err := reopened.QueryRow(`SELECT COUNT(*) FROM auth_sessions`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("sessions: %d %v", sessions, err)
	}
}

func TestRestoreIntoEmptyLocationAndRejectsBadBackup(t *testing.T) {
	path, _ := newDB(t)
	ctx := context.Background()
	backupFile := filepath.Join(t.TempDir(), "good.db")
	if _, err := Create(ctx, path, backupFile); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "fresh", "oxnet.db")
	result, err := Restore(ctx, backupFile, target, RestoreOptions{})
	if err != nil || result.SafetyCopy != "" || result.Restored.Rows["inventory_nodes"] != 1 {
		t.Fatalf("restore to a new location: %+v %v", result, err)
	}
	bad := filepath.Join(t.TempDir(), "bad.db")
	_ = os.WriteFile(bad, []byte("not a database"), 0o600)
	existing := filepath.Join(t.TempDir(), "keep.db")
	if _, err := Create(ctx, path, existing); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(existing)
	if _, err := Restore(ctx, bad, existing, RestoreOptions{Force: true}); err == nil {
		t.Fatal("a corrupt backup must be refused")
	}
	after, _ := os.ReadFile(existing)
	if string(before) != string(after) {
		t.Fatal("the existing database must be untouched when the backup is refused")
	}
}

func TestRestoreRefusesWhenAnotherConnectionIsActive(t *testing.T) {
	path, _ := newDB(t)
	ctx := context.Background()
	backupFile := filepath.Join(t.TempDir(), "good.db")
	if _, err := Create(ctx, path, backupFile); err != nil {
		t.Fatal(err)
	}
	// A second connection holding a read transaction blocks a full WAL checkpoint.
	reader, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, backupFile, path, RestoreOptions{Force: true}); !errors.Is(err, ErrInUse) {
		t.Fatalf("restore while in use: %v", err)
	}
}

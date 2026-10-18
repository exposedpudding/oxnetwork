// Package backup creates, verifies and restores consistent copies of the oxnet.work
// SQLite database (ADR-016).
//
// A backup is made with SQLite's VACUUM INTO, which reads one consistent snapshot
// while the application keeps running. Transient authentication tables (session
// tokens, CSRF tokens, login flows) are removed from the copy, so a backup never
// carries a usable session. Every backup is integrity-checked before it is
// published, written with owner-only permissions, and accompanied by a
// `.sha256` file in `sha256sum` format so a copy kept elsewhere can be checked.
package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/exposedpudding/oxnetwork/internal/db"
)

var (
	ErrExists       = errors.New("backup destination already exists")
	ErrNoSource     = errors.New("database file does not exist")
	ErrCorrupt      = errors.New("database failed its integrity check")
	ErrNotOxnet     = errors.New("file is not an oxnet.work database")
	ErrNewerSchema  = errors.New("backup was made by a newer version of oxnet.work")
	ErrTargetExists = errors.New("target database already exists; use force to replace it")
	ErrInUse        = errors.New("target database is in use; stop oxnet.work first")
	ErrChecksum     = errors.New("backup does not match its .sha256 file")
)

// transientTables hold credentials that must not travel in a backup. They are
// emptied in the copy; users simply sign in again after a restore.
var transientTables = []string{"auth_sessions", "oidc_login_flows"}

// countedTables are reported so an operator can sanity-check a backup.
var countedTables = []string{"inventory_nodes", "inventory_services", "host_observations", "workflows", "workflow_runs", "audit_events", "auth_sessions"}

// Info describes a verified database file.
type Info struct {
	Path             string           `json:"path"`
	Bytes            int64            `json:"bytes"`
	SHA256           string           `json:"sha256"`
	MigrationVersion int              `json:"migration_version"`
	Integrity        string           `json:"integrity"`
	Rows             map[string]int64 `json:"rows"`
}

func openRaw(dsn string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	return database, nil
}

func fileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), n, err
}

// Verify checks that path is a healthy oxnet.work database this build can use.
func Verify(ctx context.Context, path string) (Info, error) {
	info := Info{Path: path, Rows: map[string]int64{}}
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return info, ErrNoSource
		}
		return info, err
	}
	if stat.IsDir() {
		return info, ErrNotOxnet
	}
	// mode=ro never creates or modifies the file; a backup artifact has no WAL.
	database, err := openRaw("file:" + filepath.ToSlash(path) + "?mode=ro&_busy_timeout=5000")
	if err != nil {
		return info, err
	}
	defer database.Close()
	var integrity string
	if err := database.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return info, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	info.Integrity = integrity
	if integrity != "ok" {
		return info, fmt.Errorf("%w: %s", ErrCorrupt, integrity)
	}
	var present int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&present); err != nil || present == 0 {
		return info, ErrNotOxnet
	}
	var version sql.NullInt64
	if err := database.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil || !version.Valid {
		return info, ErrNotOxnet
	}
	info.MigrationVersion = int(version.Int64)
	if info.MigrationVersion > db.SchemaVersion() {
		return info, fmt.Errorf("%w (schema %d, this build knows %d)", ErrNewerSchema, info.MigrationVersion, db.SchemaVersion())
	}
	for _, table := range countedTables {
		var exists int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil || exists == 0 {
			continue
		}
		var rows int64
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&rows); err != nil {
			return info, err
		}
		info.Rows[table] = rows
	}
	if info.SHA256, info.Bytes, err = fileSHA256(path); err != nil {
		return info, err
	}
	// A checksum file kept next to the backup must match; this catches damage
	// that leaves the database readable.
	if recorded, err := os.ReadFile(path + ".sha256"); err == nil {
		if fields := strings.Fields(string(recorded)); len(fields) == 0 || !strings.EqualFold(fields[0], info.SHA256) {
			return info, ErrChecksum
		}
	}
	return info, nil
}

// Create writes a verified, session-free copy of the database at dbPath to dest.
// It never modifies the source and refuses to overwrite an existing file.
func Create(ctx context.Context, dbPath, dest string) (Info, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return Info{}, ErrNoSource
		}
		return Info{}, err
	}
	if _, err := os.Stat(dest); err == nil {
		return Info{}, ErrExists
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return Info{}, err
	}
	partial := dest + ".partial"
	_ = os.Remove(partial)
	cleanup := func() {
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			_ = os.Remove(partial + suffix)
		}
	}
	// Read-only at the connection level: the source cannot be modified by a backup.
	source, err := openRaw("file:" + filepath.ToSlash(dbPath) + "?mode=ro&_busy_timeout=10000")
	if err != nil {
		return Info{}, err
	}
	_, err = source.ExecContext(ctx, `VACUUM INTO ?`, partial)
	source.Close()
	if err != nil {
		cleanup()
		return Info{}, fmt.Errorf("snapshot failed: %w", err)
	}
	if err := scrub(ctx, partial); err != nil {
		cleanup()
		return Info{}, fmt.Errorf("could not remove session data from the copy: %w", err)
	}
	if err := os.Chmod(partial, 0o600); err != nil {
		cleanup()
		return Info{}, err
	}
	info, err := Verify(ctx, partial)
	if err != nil {
		cleanup()
		return Info{}, fmt.Errorf("backup did not verify: %w", err)
	}
	if err := os.Rename(partial, dest); err != nil {
		cleanup()
		return Info{}, err
	}
	info.Path = dest
	line := fmt.Sprintf("%s  %s\n", info.SHA256, filepath.Base(dest))
	if err := os.WriteFile(dest+".sha256", []byte(line), 0o600); err != nil {
		return info, fmt.Errorf("backup written but checksum file failed: %w", err)
	}
	return info, nil
}

// scrub empties the transient auth tables and leaves the copy in a single, WAL-free file.
func scrub(ctx context.Context, path string) error {
	database, err := openRaw("file:" + filepath.ToSlash(path) + "?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer database.Close()
	for _, table := range transientTables {
		var exists int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		if _, err := database.ExecContext(ctx, `DELETE FROM `+table); err != nil {
			return err
		}
	}
	var mode string
	if err := database.QueryRowContext(ctx, `PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `VACUUM`); err != nil {
		return err
	}
	if err := database.Close(); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(path + suffix)
	}
	return nil
}

// RestoreOptions controls Restore.
type RestoreOptions struct {
	// Force allows replacing an existing database. The previous database is
	// always kept next to the target as <target>.pre-restore-<timestamp>.
	Force bool
	Now   func() time.Time
}

// RestoreResult reports what Restore did.
type RestoreResult struct {
	Restored   Info   `json:"restored"`
	SafetyCopy string `json:"safety_copy,omitempty"`
}

// Restore replaces the database at dbPath with a verified backup. The application
// must be stopped: Restore refuses when another connection is active, but an
// idle connection cannot be detected, so callers (see deploy/home-gw/restore-oxnet.sh)
// must stop the service first.
func Restore(ctx context.Context, src, dbPath string, opts RestoreOptions) (RestoreResult, error) {
	var result RestoreResult
	if _, err := Verify(ctx, src); err != nil {
		return result, fmt.Errorf("backup is not restorable: %w", err)
	}
	_, existsErr := os.Stat(dbPath)
	exists := existsErr == nil
	if exists && !opts.Force {
		return result, ErrTargetExists
	}
	if exists {
		if err := ensureNotInUse(ctx, dbPath); err != nil {
			return result, err
		}
		now := time.Now
		if opts.Now != nil {
			now = opts.Now
		}
		safety := fmt.Sprintf("%s.pre-restore-%s", dbPath, now().UTC().Format("20060102T150405Z"))
		if _, err := Create(ctx, dbPath, safety); err != nil {
			return result, fmt.Errorf("could not save the current database before replacing it: %w", err)
		}
		result.SafetyCopy = safety
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return result, err
	}
	staged := dbPath + ".restore-tmp"
	_ = os.Remove(staged)
	if err := copyFile(src, staged); err != nil {
		_ = os.Remove(staged)
		return result, err
	}
	if _, err := Verify(ctx, staged); err != nil {
		_ = os.Remove(staged)
		return result, fmt.Errorf("staged copy did not verify: %w", err)
	}
	// Old WAL and shared-memory files belong to the database being replaced.
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}
	if err := os.Rename(staged, dbPath); err != nil {
		_ = os.Remove(staged)
		return result, err
	}
	info, err := Verify(ctx, dbPath)
	if err != nil {
		return result, fmt.Errorf("restored database did not verify: %w", err)
	}
	result.Restored = info
	return result, nil
}

// ensureNotInUse fails when another connection holds locks that stop a full
// checkpoint. It cannot see an idle connection, which is why callers stop the app.
func ensureNotInUse(ctx context.Context, dbPath string) error {
	database, err := openRaw("file:" + filepath.ToSlash(dbPath) + "?_busy_timeout=0")
	if err != nil {
		return err
	}
	defer database.Close()
	var busy, logFrames, checkpointed int
	if err := database.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		if strings.Contains(err.Error(), "locked") || strings.Contains(err.Error(), "busy") {
			return ErrInUse
		}
		return err
	}
	if busy != 0 {
		return ErrInUse
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

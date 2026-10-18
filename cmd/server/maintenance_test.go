package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/exposedpudding/oxnetwork/internal/db"
)

func run(t *testing.T, args ...string) (handled bool, code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	handled, code = runMaintenance(args, &out, &errOut)
	return handled, code, out.String(), errOut.String()
}

func TestNonMaintenanceArgumentsFallThroughToTheServer(t *testing.T) {
	for _, args := range [][]string{nil, {}, {"-config", "x.yml"}, {"-listen", "127.0.0.1:1"}} {
		if handled, _, _, _ := run(t, args...); handled {
			t.Fatalf("%v must not be treated as maintenance", args)
		}
	}
	if handled, code, out, _ := run(t, "help"); !handled || code != 0 || !strings.Contains(out, "restore") {
		t.Fatalf("help: %v %d %q", handled, code, out)
	}
}

func TestBackupVerifyRestoreEndToEnd(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	database, err := db.Open(filepath.Join(dataDir, "oxnet.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO inventory_nodes(id,name,role,platform,updated_at,addresses_json) VALUES('n1','One','Compute','linux','2026-10-18T12:00:00Z','[]')`); err != nil {
		t.Fatal(err)
	}
	database.Close()
	t.Setenv("OXNET_DATA_DIR", dataDir)
	t.Setenv("OXNET_CONFIG", "")
	dest := filepath.Join(dir, "out", "b.db")

	handled, code, out, errOut := run(t, "backup", dest)
	if !handled || code != 0 {
		t.Fatalf("backup: %d %s %s", code, out, errOut)
	}
	var info struct {
		SHA256    string           `json:"sha256"`
		Integrity string           `json:"integrity"`
		Rows      map[string]int64 `json:"rows"`
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil || info.Integrity != "ok" || info.Rows["inventory_nodes"] != 1 || len(info.SHA256) != 64 {
		t.Fatalf("backup output: %v %s", err, out)
	}
	if _, code, _, errOut = run(t, "backup", dest); code != 1 || !strings.Contains(errOut, "already exists") {
		t.Fatalf("second backup: %d %q", code, errOut)
	}
	if _, code, out, errOut = run(t, "verify", dest); code != 0 {
		t.Fatalf("verify: %d %s %s", code, out, errOut)
	}
	// Corrupt the checksum file: verify must fail with a non-zero status.
	if err := os.WriteFile(dest+".sha256", []byte(strings.Repeat("0", 64)+"  b.db\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, code, _, errOut = run(t, "verify", dest); code != 1 || !strings.Contains(errOut, "sha256") {
		t.Fatalf("verify with bad checksum: %d %q", code, errOut)
	}
	_ = os.WriteFile(dest+".sha256", []byte(info.SHA256+"  b.db\n"), 0o600)

	// Restore needs -force over an existing database, and says so.
	if _, code, _, errOut = run(t, "restore", dest); code != 1 || !strings.Contains(errOut, "-force") {
		t.Fatalf("restore without force: %d %q", code, errOut)
	}
	if _, code, out, errOut = run(t, "restore", "-force", dest); code != 0 || !strings.Contains(out, "safety_copy") {
		t.Fatalf("restore: %d %s %s", code, out, errOut)
	}
	if _, code, _, _ = run(t, "restore"); code != 2 {
		t.Fatalf("missing argument should be a usage error, got %d", code)
	}
}

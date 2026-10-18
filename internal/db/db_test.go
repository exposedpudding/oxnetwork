package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationsAreRepeatable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "oxnet.db")
	for i := 0; i < 2; i++ {
		database, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := Ready(context.Background(), database); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 8 {
			t.Fatalf("expected eight migrations, got %d", count)
		}
		// Migration 007 replaces the unused 001 audit placeholder, keeping it renamed.
		var columns, legacy int
		if err := database.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('audit_events') WHERE name IN ('target_type','target_id','outcome','detail_json')`).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_events_v1_unused'`).Scan(&legacy); err != nil {
			t.Fatal(err)
		}
		if columns != 4 || legacy != 1 {
			t.Fatalf("audit table not migrated: columns=%d legacy=%d", columns, legacy)
		}
		if err := database.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

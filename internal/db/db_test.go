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
		if count != 4 {
			t.Fatalf("expected four migrations, got %d", count)
		}
		if err := database.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

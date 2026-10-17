package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	t.Setenv("OXNET_DATA_DIR", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen: 127.0.0.1:8090\nui:\n  site_name: Test lab\nfeatures:\n  demo_provider: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Listen != "127.0.0.1:8090" || c.UI.SiteName != "Test lab" || c.DatabasePath() != filepath.Join(os.Getenv("OXNET_DATA_DIR"), "oxnet.db") {
		t.Fatalf("unexpected config: %+v", c)
	}
	if err := os.WriteFile(path, []byte("features:\n  actions_enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unsupported actions to be rejected")
	}
}

func TestShippedExampleLoads(t *testing.T) {
	if _, err := Load("../../config/config.example.yaml"); err != nil {
		t.Fatal(err)
	}
}

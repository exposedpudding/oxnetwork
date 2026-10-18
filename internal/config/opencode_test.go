package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeServerValidation(t *testing.T) {
	good := OpenCodeServer{ID: "ox-c-01", Name: "OX-C-01", URL: "http://100.69.88.98:4096", Username: "opencode", PasswordFile: "/run/secrets/oc", NodeID: "tailscale-2365559827781033"}
	bad := map[string]func(*OpenCodeServer){
		"empty id":             func(s *OpenCodeServer) { s.ID = "" },
		"uppercase id":         func(s *OpenCodeServer) { s.ID = "OX" },
		"long name":            func(s *OpenCodeServer) { s.Name = strings.Repeat("n", 61) },
		"relative url":         func(s *OpenCodeServer) { s.URL = "100.69.88.98:4096" },
		"ftp url":              func(s *OpenCodeServer) { s.URL = "ftp://host/" },
		"credentials in url":   func(s *OpenCodeServer) { s.URL = "http://opencode:pw@host:4096" },
		"query in url":         func(s *OpenCodeServer) { s.URL = "http://host:4096/?password=x" },
		"bad node id":          func(s *OpenCodeServer) { s.NodeID = "ox-c-01" },
		"relative secret path": func(s *OpenCodeServer) { s.PasswordFile = "secrets/oc" },
	}
	for name, mutate := range bad {
		c := liveConfig()
		server := good
		mutate(&server)
		c.OpenCodeServers = []OpenCodeServer{server}
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	c := liveConfig()
	c.OpenCodeServers = []OpenCodeServer{good}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid server rejected: %v", err)
	}
	c.OpenCodeServers = []OpenCodeServer{good, good}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicates: %v", err)
	}
	c.OpenCodeServers = nil
	for i := 0; i <= MaxOpenCodeServers; i++ {
		s := good
		s.ID = "s" + strings.Repeat("a", i)
		c.OpenCodeServers = append(c.OpenCodeServers, s)
	}
	if err := c.Validate(); err == nil {
		t.Fatal("too many servers must be rejected")
	}
	demo := Config{}
	demo.Server.Listen, demo.Server.DataDir, demo.Server.WebDir, demo.UI.SiteName = "127.0.0.1:8080", "./data", "./web/dist", "T"
	demo.Features.DemoProvider = true
	demo.OpenCodeServers = []OpenCodeServer{good}
	if err := demo.Validate(); err == nil || !strings.Contains(err.Error(), "live inventory") {
		t.Fatalf("demo mode: %v", err)
	}
}

func TestOpenCodePollIntervalAndFileLoading(t *testing.T) {
	c := liveConfig()
	for _, seconds := range []int{1, 4, 301} {
		c.OpenCodePollIntervalSeconds = seconds
		if err := c.Validate(); err == nil {
			t.Errorf("interval %d should be rejected", seconds)
		}
	}
	c.OpenCodePollIntervalSeconds = 0
	if c.OpenCodePollInterval() != DefaultOpenCodePollInterval {
		t.Fatalf("default interval: %d", c.OpenCodePollInterval())
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "oc.yml")
	body := "opencode_poll_interval_seconds: 20\nopencode_servers:\n  - id: ox-c-01\n    name: OX-C-01\n    url: http://100.69.88.98:4096\n    password_file: /run/secrets/oc\n    node_id: tailscale-1\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"OXNET_DEMO_PROVIDER": "false", "OXNET_TAILSCALE_TAILNET": "-", "OXNET_TAILSCALE_OAUTH_CLIENT_ID": "id", "OXNET_TAILSCALE_OAUTH_CLIENT_SECRET_FILE": "/secret",
		"OXNET_OIDC_ISSUER_URL": "https://auth.example.test/", "OXNET_OIDC_CLIENT_ID": "c", "OXNET_OIDC_CLIENT_SECRET_FILE": "/s", "OXNET_PUBLIC_URL": "https://app.example.test",
		"OXNET_OPENCODE_SERVERS_FILE": file,
	} {
		t.Setenv(key, value)
	}
	cfg, err := Load("")
	if err != nil || len(cfg.OpenCodeServers) != 1 || cfg.OpenCodeServers[0].ID != "ox-c-01" || cfg.OpenCodePollInterval() != 20 {
		t.Fatalf("loaded: %+v %v", cfg.OpenCodeServers, err)
	}
	if err := os.WriteFile(file, []byte("opencode_servers:\n  - id: a\n    url: http://x/\n    command: rm\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil {
		t.Fatal("unknown key must be rejected")
	}
	t.Setenv("OXNET_OPENCODE_SERVERS_FILE", filepath.Join(dir, "missing.yml"))
	if _, err := Load(""); err == nil {
		t.Fatal("missing file must be an error")
	}
}

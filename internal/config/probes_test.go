package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func liveConfig() Config {
	var c Config
	c.Server.Listen, c.Server.DataDir, c.Server.WebDir, c.UI.SiteName = "127.0.0.1:8080", "./data", "./web/dist", "Test"
	c.Features.DemoProvider = false
	c.Tailscale.Tailnet, c.Tailscale.OAuthClientID, c.Tailscale.OAuthClientSecretFile = "-", "id", "/secret"
	c.Auth.IssuerURL, c.Auth.ClientID, c.Auth.ClientSecretFile, c.Auth.PublicURL = "https://auth.example.test/", "c", "/s", "https://app.example.test"
	return c
}

func TestServiceProbeValidation(t *testing.T) {
	good := ServiceProbe{ID: "auth-live", Name: "Authentik", URL: "https://auth.example.test/-/health/live/", ExpectStatus: []int{204}, TimeoutSeconds: 3}
	bad := map[string]func(*ServiceProbe){
		"empty id":         func(p *ServiceProbe) { p.ID = "" },
		"uppercase id":     func(p *ServiceProbe) { p.ID = "Auth" },
		"id with slash":    func(p *ServiceProbe) { p.ID = "a/b" },
		"long name":        func(p *ServiceProbe) { p.Name = strings.Repeat("n", 61) },
		"relative url":     func(p *ServiceProbe) { p.URL = "/readyz" },
		"file scheme":      func(p *ServiceProbe) { p.URL = "file:///etc/passwd" },
		"credentials":      func(p *ServiceProbe) { p.URL = "https://user:pw@auth.example.test/" },
		"fragment":         func(p *ServiceProbe) { p.URL = "https://auth.example.test/#x" },
		"status too small": func(p *ServiceProbe) { p.ExpectStatus = []int{99} },
		"status too large": func(p *ServiceProbe) { p.ExpectStatus = []int{600} },
		"negative timeout": func(p *ServiceProbe) { p.TimeoutSeconds = -1 },
		"timeout too long": func(p *ServiceProbe) { p.TimeoutSeconds = 11 },
	}
	for name, mutate := range bad {
		c := liveConfig()
		probe := good
		mutate(&probe)
		c.ServiceProbes = []ServiceProbe{probe}
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	c := liveConfig()
	c.ServiceProbes = []ServiceProbe{good}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid probe rejected: %v", err)
	}
	c.ServiceProbes = []ServiceProbe{good, good}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate ids: %v", err)
	}
	c.ServiceProbes = nil
	for i := 0; i <= MaxServiceProbes; i++ {
		probe := good
		probe.ID = "p" + strings.Repeat("a", i)
		c.ServiceProbes = append(c.ServiceProbes, probe)
	}
	if err := c.Validate(); err == nil {
		t.Fatal("too many probes must be rejected")
	}
}

func TestProbesRequireLiveInventoryAndSaneInterval(t *testing.T) {
	demo := Config{}
	demo.Server.Listen, demo.Server.DataDir, demo.Server.WebDir, demo.UI.SiteName = "127.0.0.1:8080", "./data", "./web/dist", "Test"
	demo.Features.DemoProvider = true
	demo.ServiceProbes = []ServiceProbe{{ID: "a", URL: "http://127.0.0.1:1/"}}
	if err := demo.Validate(); err == nil || !strings.Contains(err.Error(), "live inventory") {
		t.Fatalf("demo mode with probes: %v", err)
	}
	c := liveConfig()
	for _, seconds := range []int{1, 14, 301} {
		c.ServiceProbeIntervalSeconds = seconds
		if err := c.Validate(); err == nil {
			t.Errorf("interval %d should be rejected", seconds)
		}
	}
	c.ServiceProbeIntervalSeconds = 0
	if c.ProbeInterval() != DefaultProbeIntervalSecond {
		t.Fatalf("default interval: %d", c.ProbeInterval())
	}
	c.ServiceProbeIntervalSeconds = 60
	if err := c.Validate(); err != nil || c.ProbeInterval() != 60 {
		t.Fatalf("interval 60: %v %d", err, c.ProbeInterval())
	}
}

func TestProbesLoadFromFileNamedByEnvironment(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "probes.yml")
	body := "service_probe_interval_seconds: 45\nservice_probes:\n  - id: edge\n    name: oxnet.work edge\n    url: https://oxnet.work/readyz\n    expect_status: [200]\n    timeout_seconds: 4\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"OXNET_DEMO_PROVIDER": "false", "OXNET_TAILSCALE_TAILNET": "-", "OXNET_TAILSCALE_OAUTH_CLIENT_ID": "id", "OXNET_TAILSCALE_OAUTH_CLIENT_SECRET_FILE": "/secret",
		"OXNET_OIDC_ISSUER_URL": "https://auth.example.test/", "OXNET_OIDC_CLIENT_ID": "c", "OXNET_OIDC_CLIENT_SECRET_FILE": "/s", "OXNET_PUBLIC_URL": "https://app.example.test",
		"OXNET_SERVICE_PROBES_FILE": file,
	} {
		t.Setenv(key, value)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ServiceProbes) != 1 || cfg.ServiceProbes[0].ID != "edge" || cfg.ServiceProbes[0].TimeoutSeconds != 4 || cfg.ProbeInterval() != 45 {
		t.Fatalf("loaded: %+v interval %d", cfg.ServiceProbes, cfg.ProbeInterval())
	}
	t.Setenv("OXNET_SERVICE_PROBE_INTERVAL_SECONDS", "20")
	if cfg, err = Load(""); err != nil || cfg.ProbeInterval() != 20 {
		t.Fatalf("env interval override: %v %d", err, cfg.ProbeInterval())
	}
	// Unknown keys and unreadable files fail loudly instead of silently disabling probes.
	if err := os.WriteFile(file, []byte("service_probes:\n  - id: a\n    url: http://x/\n    shell: rm\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil {
		t.Fatal("unknown probe field must be rejected")
	}
	t.Setenv("OXNET_SERVICE_PROBES_FILE", filepath.Join(dir, "missing.yml"))
	if _, err := Load(""); err == nil {
		t.Fatal("missing probes file must be an error")
	}
}

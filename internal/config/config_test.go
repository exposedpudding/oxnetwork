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

func TestOIDCConfigurationRequiresCompleteSecureOrigin(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	c.Auth.IssuerURL = "https://auth.example.test/application/o/oxnet-work/"
	if err := c.Validate(); err == nil {
		t.Fatal("partial OIDC config accepted")
	}
	c.Auth.ClientID = "oxnet-work"
	c.Auth.ClientSecretFile = "/run/secrets/oidc_client_secret"
	c.Auth.PublicURL = "https://oxnet.example.test"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Auth.PublicURL = "http://home-gw.local"
	if err := c.Validate(); err == nil {
		t.Fatal("insecure non-loopback URL accepted")
	}
	c.Auth.PublicURL = "https://oxnet.example.test/subpath"
	if err := c.Validate(); err == nil {
		t.Fatal("subpath public URL accepted")
	}
}

func TestLiveModeRequiresTailscaleAndOIDC(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	c.Features.DemoProvider = false
	if err := c.Validate(); err == nil {
		t.Fatal("live mode accepted without Tailscale configuration")
	}
	c.Tailscale.Tailnet = "example.ts.net"
	c.Tailscale.APIKeyFile = "/run/secrets/tailscale_api_key"
	if err := c.Validate(); err == nil {
		t.Fatal("live mode accepted without authentication")
	}
	c.Auth.IssuerURL = "https://auth.example.test/application/o/oxnet-work/"
	c.Auth.ClientID = "oxnet-work"
	c.Auth.ClientSecretFile = "/run/secrets/oidc_client_secret"
	c.Auth.PublicURL = "https://oxnet.example.test"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

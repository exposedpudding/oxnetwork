package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen  string `yaml:"listen"`
		DataDir string `yaml:"data_dir"`
		WebDir  string `yaml:"web_dir"`
	} `yaml:"server"`
	UI struct {
		SiteName string `yaml:"site_name"`
	} `yaml:"ui"`
	Features struct {
		DemoProvider   bool `yaml:"demo_provider"`
		ActionsEnabled bool `yaml:"actions_enabled"`
	} `yaml:"features"`
	Auth struct {
		IssuerURL        string `yaml:"issuer_url"`
		ClientID         string `yaml:"client_id"`
		ClientSecretFile string `yaml:"client_secret_file"`
		PublicURL        string `yaml:"public_url"`
	} `yaml:"auth"`
	Tailscale struct {
		Tailnet               string `yaml:"tailnet"`
		APIKeyFile            string `yaml:"api_key_file"`
		OAuthClientID         string `yaml:"oauth_client_id"`
		OAuthClientSecretFile string `yaml:"oauth_client_secret_file"`
	} `yaml:"tailscale"`
	HostAgent struct {
		TokenDir string `yaml:"token_dir"`
	} `yaml:"host_agent"`
}

func Load(path string) (Config, error) {
	var c Config
	c.Server.Listen = "127.0.0.1:8080"
	c.Server.DataDir = "./data"
	c.Server.WebDir = "./web/dist"
	c.UI.SiteName = "Homelab"
	c.Features.DemoProvider = true
	if path != "" {
		body, err := os.ReadFile(path)
		if err != nil {
			return c, err
		}
		decoder := yaml.NewDecoder(strings.NewReader(string(body)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&c); err != nil {
			return c, fmt.Errorf("config: %w", err)
		}
	}
	if value, ok := os.LookupEnv("OXNET_LISTEN"); ok {
		c.Server.Listen = value
	}
	if value, ok := os.LookupEnv("OXNET_DATA_DIR"); ok {
		c.Server.DataDir = value
	}
	if value, ok := os.LookupEnv("OXNET_WEB_DIR"); ok {
		c.Server.WebDir = value
	}
	if value, ok := os.LookupEnv("OXNET_SITE_NAME"); ok {
		c.UI.SiteName = value
	}
	if value, ok := os.LookupEnv("OXNET_DEMO_PROVIDER"); ok {
		c.Features.DemoProvider = value == "true"
	}
	if value, ok := os.LookupEnv("OXNET_OIDC_ISSUER_URL"); ok {
		c.Auth.IssuerURL = value
	}
	if value, ok := os.LookupEnv("OXNET_OIDC_CLIENT_ID"); ok {
		c.Auth.ClientID = value
	}
	if value, ok := os.LookupEnv("OXNET_OIDC_CLIENT_SECRET_FILE"); ok {
		c.Auth.ClientSecretFile = value
	}
	if value, ok := os.LookupEnv("OXNET_PUBLIC_URL"); ok {
		c.Auth.PublicURL = value
	}
	if value, ok := os.LookupEnv("OXNET_TAILSCALE_TAILNET"); ok {
		c.Tailscale.Tailnet = value
	}
	if value, ok := os.LookupEnv("OXNET_TAILSCALE_API_KEY_FILE"); ok {
		c.Tailscale.APIKeyFile = value
	}
	if value, ok := os.LookupEnv("OXNET_TAILSCALE_OAUTH_CLIENT_ID"); ok {
		c.Tailscale.OAuthClientID = value
	}
	if value, ok := os.LookupEnv("OXNET_TAILSCALE_OAUTH_CLIENT_SECRET_FILE"); ok {
		c.Tailscale.OAuthClientSecretFile = value
	}
	if value, ok := os.LookupEnv("OXNET_HOST_AGENT_TOKEN_DIR"); ok {
		c.HostAgent.TokenDir = value
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(c.Server.Listen)
	if err != nil || host == "" || port == "" {
		return errors.New("server.listen must be host:port")
	}
	if strings.TrimSpace(c.Server.DataDir) == "" || strings.TrimSpace(c.Server.WebDir) == "" {
		return errors.New("server data_dir and web_dir are required")
	}
	if strings.TrimSpace(c.UI.SiteName) == "" {
		return errors.New("ui.site_name is required")
	}
	if c.Features.ActionsEnabled {
		return errors.New("actions are not implemented and cannot be enabled")
	}
	if c.HostAgent.TokenDir != "" && c.Features.DemoProvider {
		return errors.New("host agent requires live inventory")
	}
	if !c.Features.DemoProvider {
		hasOAuthID := c.Tailscale.OAuthClientID != ""
		hasOAuthSecret := c.Tailscale.OAuthClientSecretFile != ""
		if c.Tailscale.Tailnet == "" || hasOAuthID != hasOAuthSecret || (c.Tailscale.APIKeyFile != "" && hasOAuthID) || (!hasOAuthID && c.Tailscale.APIKeyFile == "") {
			return errors.New("live mode requires a tailnet and either an OAuth client ID and secret file or an API key file")
		}
		if !c.OIDCEnabled() {
			return errors.New("live mode requires OIDC authentication")
		}
	}
	if c.OIDCEnabled() {
		if c.Auth.IssuerURL == "" || c.Auth.ClientID == "" || c.Auth.ClientSecretFile == "" || c.Auth.PublicURL == "" {
			return errors.New("OIDC requires issuer_url, client_id, client_secret_file and public_url")
		}
		for _, raw := range []string{c.Auth.IssuerURL, c.Auth.PublicURL} {
			u, err := url.Parse(raw)
			if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback()))) {
				return errors.New("OIDC issuer and public URLs must use HTTPS or loopback HTTP")
			}
		}
		public, _ := url.Parse(c.Auth.PublicURL)
		if public.Path != "" && public.Path != "/" {
			return errors.New("OIDC public_url must be an origin without a path")
		}
	}
	return nil
}

func (c Config) OIDCEnabled() bool {
	return c.Auth.IssuerURL != "" || c.Auth.ClientID != "" || c.Auth.ClientSecretFile != "" || c.Auth.PublicURL != ""
}

func (c Config) DatabasePath() string { return filepath.Join(c.Server.DataDir, "oxnet.db") }

package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	// ServiceProbes are fixed, operator-configured read-only HTTP checks (ADR-014).
	// They are never accepted from API requests.
	ServiceProbes               []ServiceProbe `yaml:"service_probes"`
	ServiceProbeIntervalSeconds int            `yaml:"service_probe_interval_seconds"`
}

// ServiceProbe describes one endpoint to check. The probe sends a GET with no
// credentials, never follows redirects, always verifies TLS, and reads at most
// a small slice of the response body.
type ServiceProbe struct {
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	ExpectStatus   []int  `yaml:"expect_status"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

const (
	MaxServiceProbes           = 20
	DefaultProbeIntervalSecond = 30
)

var probeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

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
	if value, ok := os.LookupEnv("OXNET_SERVICE_PROBES_FILE"); ok && value != "" {
		body, err := os.ReadFile(value)
		if err != nil {
			return c, fmt.Errorf("service probes file: %w", err)
		}
		var file struct {
			ServiceProbes               []ServiceProbe `yaml:"service_probes"`
			ServiceProbeIntervalSeconds int            `yaml:"service_probe_interval_seconds"`
		}
		decoder := yaml.NewDecoder(strings.NewReader(string(body)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&file); err != nil {
			return c, fmt.Errorf("service probes file: %w", err)
		}
		c.ServiceProbes = file.ServiceProbes
		if file.ServiceProbeIntervalSeconds != 0 {
			c.ServiceProbeIntervalSeconds = file.ServiceProbeIntervalSeconds
		}
	}
	if value, ok := os.LookupEnv("OXNET_SERVICE_PROBE_INTERVAL_SECONDS"); ok && value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return c, errors.New("OXNET_SERVICE_PROBE_INTERVAL_SECONDS must be a whole number")
		}
		c.ServiceProbeIntervalSeconds = seconds
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
	if len(c.ServiceProbes) > 0 && c.Features.DemoProvider {
		return errors.New("service probes require live inventory")
	}
	if err := c.validateProbes(); err != nil {
		return err
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

// validateProbes rejects malformed probes. Zero values (no expect_status, no
// timeout, no interval) are valid and receive their defaults in package probe.
func (c Config) validateProbes() error {
	if c.ServiceProbeIntervalSeconds != 0 && (c.ServiceProbeIntervalSeconds < 15 || c.ServiceProbeIntervalSeconds > 300) {
		return errors.New("service_probe_interval_seconds must be between 15 and 300")
	}
	if len(c.ServiceProbes) > MaxServiceProbes {
		return fmt.Errorf("at most %d service probes are supported", MaxServiceProbes)
	}
	seen := map[string]bool{}
	for i, probe := range c.ServiceProbes {
		label := fmt.Sprintf("service_probes[%d]", i)
		if !probeIDPattern.MatchString(probe.ID) {
			return fmt.Errorf("%s: id must be 1-40 lowercase letters, digits or dashes", label)
		}
		if seen[probe.ID] {
			return fmt.Errorf("%s: duplicate id %q", label, probe.ID)
		}
		seen[probe.ID] = true
		if len([]rune(probe.Name)) > 60 {
			return fmt.Errorf("%s: name must be 60 characters or fewer", label)
		}
		u, err := url.Parse(probe.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return fmt.Errorf("%s: url must be an absolute http or https URL", label)
		}
		if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return fmt.Errorf("%s: url must not contain credentials, a query or a fragment", label)
		}
		for _, status := range probe.ExpectStatus {
			if status < 100 || status > 599 {
				return fmt.Errorf("%s: expect_status values must be HTTP status codes", label)
			}
		}
		if probe.TimeoutSeconds < 0 || probe.TimeoutSeconds > 10 {
			return fmt.Errorf("%s: timeout_seconds must be between 1 and 10", label)
		}
	}
	return nil
}

// ProbeInterval is the time between probe rounds.
func (c Config) ProbeInterval() int {
	if c.ServiceProbeIntervalSeconds == 0 {
		return DefaultProbeIntervalSecond
	}
	return c.ServiceProbeIntervalSeconds
}

func (c Config) OIDCEnabled() bool {
	return c.Auth.IssuerURL != "" || c.Auth.ClientID != "" || c.Auth.ClientSecretFile != "" || c.Auth.PublicURL != ""
}

func (c Config) DatabasePath() string { return filepath.Join(c.Server.DataDir, "oxnet.db") }

// DatabasePathFor resolves only the database location (config file, then
// OXNET_DATA_DIR) without validating the rest of the configuration, so
// maintenance commands such as backup work without the live-mode secrets.
func DatabasePathFor(configPath string) (string, error) {
	dir := "./data"
	if configPath != "" {
		body, err := os.ReadFile(configPath)
		if err != nil {
			return "", err
		}
		var partial struct {
			Server struct {
				DataDir string `yaml:"data_dir"`
			} `yaml:"server"`
		}
		if err := yaml.Unmarshal(body, &partial); err != nil {
			return "", fmt.Errorf("config: %w", err)
		}
		if partial.Server.DataDir != "" {
			dir = partial.Server.DataDir
		}
	}
	if value, ok := os.LookupEnv("OXNET_DATA_DIR"); ok && value != "" {
		dir = value
	}
	return filepath.Join(dir, "oxnet.db"), nil
}

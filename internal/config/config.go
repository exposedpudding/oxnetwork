package config

import (
	"errors"
	"fmt"
	"net"
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
	if !c.Features.DemoProvider {
		return errors.New("only demo_provider is implemented")
	}
	return nil
}

func (c Config) DatabasePath() string { return filepath.Join(c.Server.DataDir, "oxnet.db") }

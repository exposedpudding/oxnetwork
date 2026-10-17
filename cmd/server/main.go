package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/api"
	"github.com/exposedpudding/oxnetwork/internal/auth"
	"github.com/exposedpudding/oxnetwork/internal/config"
	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
	"github.com/exposedpudding/oxnetwork/internal/integrations/tailscale"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

func main() {
	configPath := flag.String("config", os.Getenv("OXNET_CONFIG"), "YAML configuration file")
	listen := flag.String("listen", "", "override listen address")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	if *listen != "" {
		cfg.Server.Listen = *listen
		if err := cfg.Validate(); err != nil {
			log.Error("configuration failed", "error", err)
			os.Exit(1)
		}
	}
	host, _, _ := net.SplitHostPort(cfg.Server.Listen)
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && host != "localhost" && !cfg.OIDCEnabled() && os.Getenv("OXNET_ALLOW_UNAUTHENTICATED_BIND") != "true" {
		log.Error("non-loopback bind requires OXNET_ALLOW_UNAUTHENTICATED_BIND=true; this demo has no authentication")
		os.Exit(1)
	}
	database, err := db.Open(cfg.DatabasePath())
	if err != nil {
		log.Error("database failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	repo := inventory.Repository{DB: database}
	var refresh func(context.Context, time.Time) error
	interval := 30 * time.Second
	mode := "demo"
	if cfg.Features.DemoProvider {
		provider := demo.Provider{}
		refresh = func(ctx context.Context, now time.Time) error {
			return repo.ReplaceSnapshot(ctx, provider.Snapshot(now))
		}
	} else {
		mode = "tailscale"
		interval = 60 * time.Second
		key, err := os.ReadFile(cfg.Tailscale.APIKeyFile)
		if err != nil {
			log.Error("Tailscale API key file unavailable", "error", err)
			os.Exit(1)
		}
		apiKey := strings.TrimSpace(string(key))
		if apiKey == "" {
			log.Error("Tailscale API key file is empty")
			os.Exit(1)
		}
		client := tailscale.Client{Tailnet: cfg.Tailscale.Tailnet, APIKey: apiKey}
		// Clear the demo on source change while retaining the last successful
		// live observations across restarts and temporary API failures.
		existing, err := repo.Snapshot(context.Background(), time.Now().UTC())
		if err != nil {
			log.Error("inventory source check failed", "error", err)
			os.Exit(1)
		}
		if containsSimulated(existing) {
			if err := repo.ReplaceSnapshot(context.Background(), domain.Snapshot{}); err != nil {
				log.Error("inventory source switch failed", "error", err)
				os.Exit(1)
			}
		}
		refresh = func(ctx context.Context, now time.Time) error {
			snapshot, err := client.Snapshot(ctx, now)
			if err != nil {
				return err
			}
			return repo.ReplaceSnapshot(ctx, snapshot)
		}
	}
	initialCtx, initialCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := refresh(initialCtx, time.Now().UTC()); err != nil {
		log.Warn("initial inventory refresh failed; retry scheduled", "source", mode, "error", err)
	}
	initialCancel()
	app := &api.Server{DB: database, Repo: repo, WebDir: cfg.Server.WebDir, SiteName: cfg.UI.SiteName, AuthEnabled: cfg.OIDCEnabled(), SourceMode: mode, Log: log}
	var handler http.Handler = app.Handler()
	if cfg.OIDCEnabled() {
		identity, err := auth.New(context.Background(), database, cfg)
		if err != nil {
			log.Error("authentication setup failed", "error", err)
			os.Exit(1)
		}
		handler = identity.Handler(handler)
	}
	server := &http.Server{Addr: cfg.Server.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := refresh(ctx, now.UTC()); err != nil && ctx.Err() == nil {
					log.Error("inventory refresh failed", "source", mode, "error", err)
				}
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Info("starting oxnet.work", "listen", cfg.Server.Listen, "inventory_source", mode)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func containsSimulated(snapshot domain.Snapshot) bool {
	for _, node := range snapshot.Nodes {
		if node.Simulated {
			return true
		}
	}
	for _, service := range snapshot.Services {
		if service.Simulated {
			return true
		}
	}
	for _, event := range snapshot.Activity {
		if event.Simulated {
			return true
		}
	}
	for _, connection := range snapshot.Connections {
		if connection.Simulated {
			return true
		}
	}
	return false
}

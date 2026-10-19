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
	"github.com/exposedpudding/oxnetwork/internal/probe"
	"github.com/exposedpudding/oxnetwork/internal/workflow"
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
	isLoopback := (ip != nil && ip.IsLoopback()) || host == "localhost"
	if !isLoopback && !cfg.OIDCEnabled() && os.Getenv("OXNET_ALLOW_UNAUTHENTICATED_BIND") != "true" {
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
	var prober *probe.Prober
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
		client := tailscale.Client{Tailnet: cfg.Tailscale.Tailnet}
		if cfg.Tailscale.OAuthClientID != "" {
			secret, err := os.ReadFile(cfg.Tailscale.OAuthClientSecretFile)
			if err != nil {
				log.Error("Tailscale OAuth client secret file unavailable", "error", err)
				os.Exit(1)
			}
			clientSecret := strings.TrimSpace(string(secret))
			if clientSecret == "" {
				log.Error("Tailscale OAuth client secret file is empty")
				os.Exit(1)
			}
			client.HTTP = tailscale.NewOAuthClient(cfg.Tailscale.OAuthClientID, clientSecret)
		} else {
			key, err := os.ReadFile(cfg.Tailscale.APIKeyFile)
			if err != nil {
				log.Error("Tailscale API key file unavailable", "error", err)
				os.Exit(1)
			}
			client.APIKey = strings.TrimSpace(string(key))
			if client.APIKey == "" {
				log.Error("Tailscale API key file is empty")
				os.Exit(1)
			}
		}
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
		if len(cfg.ServiceProbes) > 0 {
			prober = &probe.Prober{Probes: cfg.ServiceProbes, Interval: time.Duration(cfg.ProbeInterval()) * time.Second, Log: log}
			// First results are ready before the first inventory publish.
			probeCtx, probeCancel := context.WithTimeout(context.Background(), 15*time.Second)
			prober.ProbeAll(probeCtx)
			probeCancel()
			log.Info("service probes configured", "count", len(cfg.ServiceProbes), "interval_seconds", cfg.ProbeInterval())
		}
		refresh = func(ctx context.Context, now time.Time) error {
			snapshot, err := client.Snapshot(ctx, now)
			if err != nil {
				return err
			}
			// ReplaceSnapshot deletes every service row, so probe results must be
			// part of the snapshot it stores (ADR-014).
			if prober != nil {
				snapshot.Services = append(snapshot.Services, prober.Services(now)...)
			}
			return repo.ReplaceSnapshot(ctx, snapshot)
		}
	}
	initialCtx, initialCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := refresh(initialCtx, time.Now().UTC()); err != nil {
		log.Warn("initial inventory refresh failed; retry scheduled", "source", mode, "error", err)
	}
	initialCancel()
	hub := &workflow.Hub{}
	store := workflow.Store{DB: database}
	runner := &workflow.Runner{Store: store, Inventory: repo, Hub: hub, Log: log}
	if interrupted, err := store.RecoverInterrupted(context.Background()); err != nil {
		log.Error("workflow recovery failed", "error", err)
		os.Exit(1)
	} else if interrupted > 0 {
		log.Warn("marked interrupted workflow runs as failed", "count", interrupted)
	}
	if mode == "demo" {
		if err := workflow.SeedDemo(context.Background(), store); err != nil {
			log.Error("demo workflow seed failed", "error", err)
			os.Exit(1)
		}
	} else if removed, err := store.RemoveSimulated(context.Background()); err != nil {
		log.Error("demo workflow cleanup failed", "error", err)
		os.Exit(1)
	} else if removed > 0 {
		log.Info("removed simulated demo workflows", "count", removed)
	}
	app := &api.Server{DB: database, Repo: repo, WebDir: cfg.Server.WebDir, SiteName: cfg.UI.SiteName, AuthEnabled: cfg.OIDCEnabled(), SourceMode: mode, Log: log, Runner: runner, Hub: hub, MutationsAllowed: cfg.OIDCEnabled() || isLoopback}
	app.HostAgentTokensDir = cfg.HostAgent.TokenDir
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
	runner.BaseContext = ctx
	scheduler := &workflow.Scheduler{Runner: runner, Inventory: repo, Log: log}
	go scheduler.Run(ctx)
	if prober != nil {
		// Publish each probe round on its own so service health stays current even
		// while the Tailscale API is unreachable. Upsert never deletes other rows.
		go prober.Run(ctx, func(ctx context.Context, services []domain.Service) {
			if err := repo.UpsertSnapshot(ctx, domain.Snapshot{Services: services}); err != nil && ctx.Err() == nil {
				log.Error("service probe publish failed", "error", err)
			}
		})
	}
	go func() {
		<-ctx.Done()
		runner.Wait()
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

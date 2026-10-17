package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/api"
	"github.com/exposedpudding/oxnetwork/internal/auth"
	"github.com/exposedpudding/oxnetwork/internal/config"
	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
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
	provider := demo.Provider{}
	if err := repo.UpsertSnapshot(context.Background(), provider.Snapshot(time.Now().UTC())); err != nil {
		log.Error("demo inventory failed", "error", err)
		os.Exit(1)
	}
	app := &api.Server{DB: database, Repo: repo, WebDir: cfg.Server.WebDir, SiteName: cfg.UI.SiteName, AuthEnabled: cfg.OIDCEnabled(), Log: log}
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
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := repo.UpsertSnapshot(ctx, provider.Snapshot(now.UTC())); err != nil && ctx.Err() == nil {
					log.Error("demo inventory refresh failed", "error", err)
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
	log.Info("starting oxnet.work", "listen", cfg.Server.Listen, "demo", true)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

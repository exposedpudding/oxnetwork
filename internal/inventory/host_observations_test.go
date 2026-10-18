package inventory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
)

func TestHostReportSurvivesInventoryRefreshAndStales(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "host.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := Repository{DB: database}
	ctx := context.Background()
	now := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	node := domain.Node{ID: "tailscale-1", Name: "home-gw", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 180, Source: "tailscale"}}
	if err := repo.ReplaceSnapshot(ctx, domain.Snapshot{Nodes: []domain.Node{node}}); err != nil {
		t.Fatal(err)
	}
	sample := HostReport{CPUPercent: 21.5, RAMPercent: 42, DiskPercent: 37, VCPU: 4, RAMGB: 16}
	if err := repo.StoreHostReport(ctx, node.ID, sample, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceSnapshot(ctx, domain.Snapshot{Nodes: []domain.Node{node}}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Snapshot(ctx, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].Health.Source != "host-agent" || got.Nodes[0].Health.State != "healthy" || got.Nodes[0].Freshness != "fresh" || *got.Nodes[0].Metrics.DiskPercent != 37 {
		t.Fatalf("live sample missing after Tailscale refresh: %+v", got.Nodes)
	}
	got, err = repo.Snapshot(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got.Nodes[0].Freshness != "stale" {
		t.Fatalf("stopped host collector did not become stale: %+v", got.Nodes[0])
	}
}

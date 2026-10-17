package inventory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
)

func TestSnapshotPersistsAcrossReopenAndRecomputesFreshness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.db")
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	repo := Repository{DB: database}
	if err := repo.UpsertSnapshot(ctx, demo.Provider{}.Snapshot(now)); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo.DB = database
	snapshot, err := repo.Snapshot(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) != 3 || len(snapshot.Services) != 3 || len(snapshot.Activity) != 2 || len(snapshot.Connections) != 3 {
		t.Fatalf("unexpected snapshot sizes: %+v", snapshot)
	}
	if snapshot.Connections[1].Freshness != "stale" || !snapshot.Connections[1].Simulated {
		t.Fatalf("unexpected connection: %+v", snapshot.Connections[1])
	}
	if snapshot.Nodes[0].ID != "demo-gateway" || snapshot.Nodes[0].Freshness != "fresh" || !snapshot.Nodes[0].Simulated || len(snapshot.Nodes[0].Capabilities) != 1 {
		t.Fatalf("unexpected gateway: %+v", snapshot.Nodes[0])
	}
	if snapshot.Nodes[2].ID != "demo-worker" || snapshot.Nodes[2].Freshness != "stale" || snapshot.Nodes[2].Metrics.VCPU == nil || *snapshot.Nodes[2].Metrics.VCPU != 8 {
		t.Fatalf("unexpected worker: %+v", snapshot.Nodes[2])
	}
	if snapshot.Nodes[1].Metrics.CPUPercent != nil {
		t.Fatalf("sensor metric should be absent: %+v", snapshot.Nodes[1])
	}
	later, err := repo.Snapshot(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if later.Nodes[0].Freshness != "stale" {
		t.Fatalf("freshness did not age: %+v", later.Nodes[0])
	}
	if err := repo.UpsertSnapshot(ctx, demo.Provider{}.Snapshot(now.Add(2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	refreshed, err := repo.Snapshot(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Nodes) != 3 || refreshed.Nodes[0].Freshness != "fresh" {
		t.Fatalf("upsert failed: %+v", refreshed.Nodes)
	}
}

func TestReplaceSnapshotRemovesDemoAndPersistsTailscaleIdentity(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := Repository{DB: database}
	ctx := context.Background()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if err := repo.UpsertSnapshot(ctx, demo.Provider{}.Snapshot(now)); err != nil {
		t.Fatal(err)
	}
	lastSeen := now.Add(-time.Minute)
	live := domain.Snapshot{Nodes: []domain.Node{{ID: "tailscale-123", Name: "home-gw", Role: "Tailnet device", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 180, Source: "tailscale"}, Addresses: []string{"100.64.0.10"}, LastSeen: &lastSeen}}}
	if err := repo.ReplaceSnapshot(ctx, live); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Snapshot(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 1 || len(got.Services) != 0 || len(got.Activity) != 0 || len(got.Connections) != 0 || got.Nodes[0].ID != "tailscale-123" || got.Nodes[0].Addresses[0] != "100.64.0.10" || !got.Nodes[0].LastSeen.Equal(lastSeen) {
		t.Fatalf("unexpected replacement: %+v", got)
	}
}

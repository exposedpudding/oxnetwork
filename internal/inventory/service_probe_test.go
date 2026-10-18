package inventory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
)

func probeService(id, state, detail string, observed time.Time) domain.Service {
	return domain.Service{
		ID: "probe-" + id, Name: id, Endpoint: "https://oxnet.work/readyz", Detail: detail,
		Health: domain.Health{State: state, ObservedAt: observed, StaleAfterSeconds: 90, Source: "http-probe"},
	}
}

func openRepo(t *testing.T) Repository {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return Repository{DB: database}
}

func nodeSnapshot(now time.Time) domain.Snapshot {
	return domain.Snapshot{Nodes: []domain.Node{{ID: "tailscale-1", Name: "gw", Role: "Tailnet device", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 180, Source: "tailscale"}}}}
}

func TestServiceEndpointAndDetailRoundTrip(t *testing.T) {
	repo := openRepo(t)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	if err := repo.ReplaceSnapshot(ctx, domain.Snapshot{Services: []domain.Service{probeService("edge", "healthy", "HTTP 200 in 12 ms", now)}}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Snapshot(ctx, now.Add(10*time.Second))
	if err != nil || len(got.Services) != 1 {
		t.Fatalf("snapshot: %+v %v", got, err)
	}
	s := got.Services[0]
	if s.Endpoint != "https://oxnet.work/readyz" || s.Detail != "HTTP 200 in 12 ms" || s.Health.Source != "http-probe" || s.Freshness != "fresh" || s.Simulated {
		t.Fatalf("service: %+v", s)
	}
}

// ReplaceSnapshot clears every service row. Merging the latest probe results into
// the snapshot it stores keeps them; omitting them loses them (the failure mode
// ADR-014 designs against).
func TestReplaceSnapshotKeepsProbeServicesOnlyWhenMerged(t *testing.T) {
	repo := openRepo(t)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	probes := []domain.Service{probeService("edge", "healthy", "HTTP 200 in 9 ms", now), probeService("auth", "unhealthy", "HTTP 502 (expected 200/204)", now)}
	if err := repo.UpsertSnapshot(ctx, domain.Snapshot{Services: probes}); err != nil {
		t.Fatal(err)
	}

	unmerged := nodeSnapshot(now)
	if err := repo.ReplaceSnapshot(ctx, unmerged); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Snapshot(ctx, now); len(got.Services) != 0 {
		t.Fatalf("expected replace without probes to clear services, got %+v", got.Services)
	}

	merged := nodeSnapshot(now)
	merged.Services = append(merged.Services, probes...)
	if err := repo.ReplaceSnapshot(ctx, merged); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Snapshot(ctx, now)
	if err != nil || len(got.Nodes) != 1 || len(got.Services) != 2 {
		t.Fatalf("merged snapshot: %+v %v", got, err)
	}
	if got.Services[0].ID != "probe-auth" || got.Services[0].Health.State != "unhealthy" || got.Services[1].Health.State != "healthy" {
		t.Fatalf("services: %+v", got.Services)
	}
}

// A probe round can be published on its own (for example while the Tailscale API
// is down) without touching nodes or other services.
func TestProbeRoundUpsertDoesNotDisturbNodesAndUpdatesInPlace(t *testing.T) {
	repo := openRepo(t)
	t0 := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	if err := repo.ReplaceSnapshot(ctx, nodeSnapshot(t0)); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertSnapshot(ctx, domain.Snapshot{Services: []domain.Service{probeService("edge", "healthy", "HTTP 200 in 9 ms", t0)}}); err != nil {
		t.Fatal(err)
	}
	t1 := t0.Add(30 * time.Second)
	if err := repo.UpsertSnapshot(ctx, domain.Snapshot{Services: []domain.Service{probeService("edge", "unhealthy", "no response within 3s", t1)}}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Snapshot(ctx, t1)
	if err != nil || len(got.Nodes) != 1 || len(got.Services) != 1 {
		t.Fatalf("snapshot: %+v %v", got, err)
	}
	if s := got.Services[0]; s.Health.State != "unhealthy" || s.Detail != "no response within 3s" || !s.Health.ObservedAt.Equal(t1) {
		t.Fatalf("service not updated in place: %+v", s)
	}
	// A result that is not refreshed goes stale instead of staying "healthy".
	late, _ := repo.Snapshot(ctx, t1.Add(2*time.Minute))
	if late.Services[0].Freshness != "stale" {
		t.Fatalf("freshness: %+v", late.Services[0])
	}
}

func TestExistingServiceRowsKeepEmptyProbeFields(t *testing.T) {
	repo := openRepo(t)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	plain := domain.Service{ID: "plain", Name: "Plain", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 90, Source: "demo"}, Simulated: true}
	if err := repo.UpsertSnapshot(context.Background(), domain.Snapshot{Services: []domain.Service{plain}}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Snapshot(context.Background(), now)
	if got.Services[0].Endpoint != "" || got.Services[0].Detail != "" {
		t.Fatalf("unprobed service must have no endpoint or detail: %+v", got.Services[0])
	}
}

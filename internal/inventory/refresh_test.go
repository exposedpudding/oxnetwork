package inventory

import (
	"context"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

func TestRefreshedAtTracksSnapshotsButNotHostReports(t *testing.T) {
	repo := openRepo(t)
	ctx := context.Background()
	if _, ok, err := repo.RefreshedAt(ctx); err != nil || ok {
		t.Fatalf("empty inventory: ok=%v err=%v", ok, err)
	}
	now := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	if err := repo.ReplaceSnapshot(ctx, nodeSnapshot(now)); err != nil {
		t.Fatal(err)
	}
	first, ok, err := repo.RefreshedAt(ctx)
	if err != nil || !ok || time.Since(first) > time.Minute || first.Location() != time.UTC {
		t.Fatalf("after a snapshot: %v ok=%v err=%v", first, ok, err)
	}
	time.Sleep(15 * time.Millisecond)
	if err := repo.StoreHostReport(ctx, "tailscale-1", HostReport{CPUPercent: 1, RAMPercent: 1, DiskPercent: 1, VCPU: 2, RAMGB: 4}, time.Now()); err != nil {
		t.Fatal(err)
	}
	afterReport, _, _ := repo.RefreshedAt(ctx)
	if !afterReport.Equal(first) {
		t.Fatalf("a host report must not count as an inventory refresh: %v vs %v", first, afterReport)
	}
	time.Sleep(15 * time.Millisecond)
	if err := repo.ReplaceSnapshot(ctx, domain.Snapshot{Nodes: nodeSnapshot(now).Nodes}); err != nil {
		t.Fatal(err)
	}
	second, _, _ := repo.RefreshedAt(ctx)
	if !second.After(first) {
		t.Fatalf("a new snapshot must move the refresh time: %v then %v", first, second)
	}
}

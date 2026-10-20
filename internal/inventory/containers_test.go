package inventory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

var containersNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func sampleContainer(id, name, state string) Container {
	return Container{ID: id, Name: name, Image: "ghcr.io/example/app:1.2", State: state, Status: "Up 2 hours (healthy)", Health: "healthy",
		CreatedAt: containersNow.Add(-2 * time.Hour), Ports: []string{"127.0.0.1:8080->80/tcp"}, ComposeProject: "oxnet", ComposeService: "web"}
}

func repoWithNode(t *testing.T) Repository {
	t.Helper()
	repo := openRepo(t)
	node := domain.Node{ID: "tailscale-1", Name: "home-gw", Health: domain.Health{State: "unknown", ObservedAt: containersNow, StaleAfterSeconds: 180, Source: "tailscale"}}
	if err := repo.ReplaceSnapshot(context.Background(), domain.Snapshot{Nodes: []domain.Node{node}}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestContainerReportValidation(t *testing.T) {
	good := sampleContainer("0123456789ab", "web", "running")
	if err := (ContainerReport{Containers: []Container{good}}).Validate(); err != nil {
		t.Fatalf("good report rejected: %v", err)
	}
	bad := map[string]func(*Container){
		"short id":         func(c *Container) { c.ID = "abc" },
		"uppercase id":     func(c *Container) { c.ID = "0123456789AB" },
		"empty name":       func(c *Container) { c.Name = "" },
		"control in name":  func(c *Container) { c.Name = "web\nINJECT" },
		"long image":       func(c *Container) { c.Image = strings.Repeat("i", 257) },
		"unknown state":    func(c *Container) { c.State = "exploded" },
		"unknown health":   func(c *Container) { c.Health = "great" },
		"long status":      func(c *Container) { c.Status = strings.Repeat("s", 129) },
		"too many ports":   func(c *Container) { c.Ports = make([]string, 33) },
		"empty port":       func(c *Container) { c.Ports = []string{""} },
		"bad compose name": func(c *Container) { c.ComposeProject = "a b;c" },
		"no created time":  func(c *Container) { c.CreatedAt = time.Time{} },
	}
	for name, mutate := range bad {
		c := good
		c.Ports = append([]string(nil), good.Ports...)
		mutate(&c)
		if err := (ContainerReport{Containers: []Container{c}}).Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	if err := (ContainerReport{Containers: []Container{good, good}}).Validate(); err == nil {
		t.Error("duplicate ids must be rejected")
	}
	many := ContainerReport{}
	for i := 0; i <= MaxContainersPerReport; i++ {
		c := good
		c.ID = strings.Repeat("0", 8) + string("0123456789abcdef"[i%16]) + string("0123456789abcdef"[(i/16)%16]) + string("0123456789abcdef"[(i/256)%16]) + "0"
		many.Containers = append(many.Containers, c)
	}
	if err := many.Validate(); err == nil {
		t.Error("an oversized report must be rejected")
	}
}

func TestContainerReportsReplaceWholeListAndGoStale(t *testing.T) {
	repo := repoWithNode(t)
	ctx := context.Background()

	never, err := repo.NodeContainers(ctx, "tailscale-1", containersNow)
	if err != nil || never.Reported || never.Freshness != "unknown" || len(never.Items) != 0 {
		t.Fatalf("never reported: %+v %v", never, err)
	}
	first := ContainerReport{Containers: []Container{sampleContainer("bbbbbbbbbbbb", "Zeta", "running"), sampleContainer("aaaaaaaaaaaa", "alpha", "exited")}}
	if err := repo.StoreContainerReport(ctx, "tailscale-1", first, containersNow); err != nil {
		t.Fatal(err)
	}
	got, err := repo.NodeContainers(ctx, "tailscale-1", containersNow.Add(10*time.Second))
	if err != nil || !got.Reported || got.Freshness != "fresh" || len(got.Items) != 2 || got.Items[0].Name != "alpha" || got.Items[1].Name != "Zeta" {
		t.Fatalf("after first report (sorted case-insensitively by name): %+v %v", got, err)
	}
	if got.Items[1].Ports[0] != "127.0.0.1:8080->80/tcp" || got.Items[1].ComposeService != "web" {
		t.Fatalf("fields lost: %+v", got.Items[1])
	}

	// A new report replaces the list, so a removed container disappears.
	second := ContainerReport{Containers: []Container{sampleContainer("cccccccccccc", "gamma", "running")}}
	if err := repo.StoreContainerReport(ctx, "tailscale-1", second, containersNow.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.NodeContainers(ctx, "tailscale-1", containersNow.Add(31*time.Second))
	if len(got.Items) != 1 || got.Items[0].Name != "gamma" {
		t.Fatalf("old containers survived a replacement: %+v", got.Items)
	}

	// Zero containers is a real answer, different from never reporting.
	if err := repo.StoreContainerReport(ctx, "tailscale-1", ContainerReport{}, containersNow.Add(60*time.Second)); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.NodeContainers(ctx, "tailscale-1", containersNow.Add(61*time.Second))
	if !got.Reported || len(got.Items) != 0 || got.Freshness != "fresh" {
		t.Fatalf("an empty report must read as reported, fresh and empty: %+v", got)
	}

	// A stopped reporter must not leave the last list looking current.
	got, _ = repo.NodeContainers(ctx, "tailscale-1", containersNow.Add(60*time.Second+ContainerTTL*time.Second+time.Second))
	if got.Freshness != "stale" {
		t.Fatalf("expected stale, got %q", got.Freshness)
	}
}

func TestContainerReportsSurviveInventoryRefreshAndRejectUnknownNodes(t *testing.T) {
	repo := repoWithNode(t)
	ctx := context.Background()
	if err := repo.StoreContainerReport(ctx, "tailscale-404", ContainerReport{}, containersNow); err == nil {
		t.Fatal("a report for a node absent from inventory must be rejected")
	}
	if err := repo.StoreContainerReport(ctx, "tailscale-1", ContainerReport{Containers: []Container{sampleContainer("aaaaaaaaaaaa", "web", "running")}}, containersNow); err != nil {
		t.Fatal(err)
	}
	node := domain.Node{ID: "tailscale-1", Name: "home-gw", Health: domain.Health{State: "unknown", ObservedAt: containersNow, StaleAfterSeconds: 180, Source: "tailscale"}}
	if err := repo.ReplaceSnapshot(ctx, domain.Snapshot{Nodes: []domain.Node{node}}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.NodeContainers(ctx, "tailscale-1", containersNow)
	if len(got.Items) != 1 {
		t.Fatalf("a Tailscale refresh deleted container data: %+v", got)
	}
	summaries, err := repo.ContainerSummaries(ctx)
	if err != nil || len(summaries) != 1 || summaries[0].Count != 1 || summaries[0].Running != 1 || summaries[0].NodeID != "tailscale-1" {
		t.Fatalf("summaries: %+v %v", summaries, err)
	}
	// An invalid report must leave the previous list untouched.
	bad := ContainerReport{Containers: []Container{sampleContainer("zzzzzzzzzzzz", "web", "running")}}
	if err := repo.StoreContainerReport(ctx, "tailscale-1", bad, containersNow); err == nil {
		t.Fatal("expected validation error")
	}
	got, _ = repo.NodeContainers(ctx, "tailscale-1", containersNow)
	if len(got.Items) != 1 || got.Items[0].ID != "aaaaaaaaaaaa" {
		t.Fatalf("a rejected report changed stored data: %+v", got.Items)
	}
}

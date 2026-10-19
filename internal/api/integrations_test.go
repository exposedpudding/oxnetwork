package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/compute"
	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
	"github.com/exposedpudding/oxnetwork/internal/workflow"
)

var integrationNow = time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)

func agentNode(name, freshness string) domain.Node {
	return domain.Node{ID: name, Name: name, Freshness: freshness, Health: domain.Health{State: "healthy", Source: "host-agent", ObservedAt: integrationNow.Add(-20 * time.Second), StaleAfterSeconds: 90}}
}

func byID(items []Integration) map[string]Integration {
	result := map[string]Integration{}
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func liveInputs() integrationInputs {
	return integrationInputs{
		sourceMode: "tailscale", hasRefresh: true, refreshedAt: integrationNow.Add(-25 * time.Second), authEnabled: true, mutations: true,
		snapshot: domain.Snapshot{Nodes: []domain.Node{
			agentNode("home-gw", "fresh"), agentNode("ox-c-01", "fresh"), agentNode("ovh-vps", "fresh"),
			{ID: "pixel", Name: "Pixel", Health: domain.Health{State: "unknown", Source: "tailscale"}, Freshness: "fresh"},
		}},
	}
}

func TestInventoryAndHostAgentStates(t *testing.T) {
	items := buildIntegrations(liveInputs(), integrationNow)
	got := byID(items)
	if len(items) != 6 {
		t.Fatalf("expected six integrations, got %d", len(items))
	}
	inv := got["inventory"]
	if inv.State != integrationHealthy || inv.Freshness != "fresh" || inv.Detail != "4 devices; last refresh 25 seconds ago." || inv.ObservedAt == nil {
		t.Fatalf("inventory: %+v", inv)
	}
	agents := got["host-agents"]
	if agents.State != integrationHealthy || agents.Detail != "3 of 3 enrolled hosts reporting; 1 device has no agent." {
		t.Fatalf("agents: %+v", agents)
	}

	in := liveInputs()
	in.snapshot.Nodes = append(in.snapshot.Nodes, agentNode("desktop", "stale"), agentNode("ox-01", "stale"))
	degraded := byID(buildIntegrations(in, integrationNow))["host-agents"]
	if degraded.State != integrationDegraded || !strings.Contains(degraded.Detail, "3 of 5 enrolled hosts reporting; not reporting: desktop, ox-01") {
		t.Fatalf("degraded agents: %+v", degraded)
	}
	allStale := liveInputs()
	for i := range allStale.snapshot.Nodes {
		allStale.snapshot.Nodes[i].Freshness = "stale"
	}
	if got := byID(buildIntegrations(allStale, integrationNow))["host-agents"]; got.State != integrationUnhealthy {
		t.Fatalf("all agents stale: %+v", got)
	}

	none := liveInputs()
	none.snapshot.Nodes = none.snapshot.Nodes[3:]
	if got := byID(buildIntegrations(none, integrationNow))["host-agents"]; got.State != integrationNotConfigured {
		t.Fatalf("no agents enrolled: %+v", got)
	}

	// A stale or missing Tailscale refresh is reported, never hidden behind old data.
	old := liveInputs()
	old.refreshedAt = integrationNow.Add(-10 * time.Minute)
	if got := byID(buildIntegrations(old, integrationNow))["inventory"]; got.State != integrationUnhealthy || got.Freshness != "stale" || !strings.Contains(got.Detail, "10 minutes ago") {
		t.Fatalf("stale inventory: %+v", got)
	}
	never := liveInputs()
	never.hasRefresh = false
	if got := byID(buildIntegrations(never, integrationNow))["inventory"]; got.State != integrationUnknown || got.ObservedAt != nil {
		t.Fatalf("no refresh: %+v", got)
	}
}

func TestDemoModeSaysItIsSimulated(t *testing.T) {
	in := integrationInputs{sourceMode: "demo", snapshot: demo.Provider{}.Snapshot(integrationNow)}
	got := byID(buildIntegrations(in, integrationNow))
	if !strings.Contains(got["inventory"].Name, "Demo") || !strings.Contains(got["inventory"].Detail, "Simulated") {
		t.Fatalf("demo inventory: %+v", got["inventory"])
	}
	// Simulated nodes are never counted as real agents.
	if got["host-agents"].State != integrationNotConfigured {
		t.Fatalf("demo agents: %+v", got["host-agents"])
	}
}

func TestServiceProbeStates(t *testing.T) {
	probe := func(name, state, freshness string) domain.Service {
		return domain.Service{ID: "probe-" + name, Name: name, Freshness: freshness, Health: domain.Health{State: state, Source: "http-probe", ObservedAt: integrationNow.Add(-10 * time.Second)}}
	}
	in := liveInputs()
	in.snapshot.Services = []domain.Service{probe("edge", "healthy", "fresh"), probe("auth", "healthy", "fresh")}
	if got := byID(buildIntegrations(in, integrationNow))["service-probes"]; got.State != integrationHealthy || got.Detail != "2 of 2 services answering." {
		t.Fatalf("healthy: %+v", got)
	}
	// Probe intervals are configurable; the source's freshness decision is authoritative.
	in.snapshot.Services[0].Health.ObservedAt = integrationNow.Add(-2 * time.Minute)
	if got := byID(buildIntegrations(in, integrationNow))["service-probes"]; got.State != integrationHealthy || got.Freshness != "fresh" {
		t.Fatalf("long-interval probe must stay fresh: %+v", got)
	}
	in.snapshot.Services = []domain.Service{probe("edge", "healthy", "fresh"), probe("auth", "unhealthy", "fresh")}
	if got := byID(buildIntegrations(in, integrationNow))["service-probes"]; got.State != integrationDegraded || got.Detail != "1 of 2 services answering; failing: auth." {
		t.Fatalf("one failing: %+v", got)
	}
	in.snapshot.Services = []domain.Service{probe("edge", "healthy", "stale"), probe("auth", "unhealthy", "fresh")}
	if got := byID(buildIntegrations(in, integrationNow))["service-probes"]; got.State != integrationUnhealthy {
		t.Fatalf("a stale 'healthy' reading must not count as answering: %+v", got)
	}
	in.snapshot.Services = []domain.Service{{ID: "other", Name: "Other", Health: domain.Health{Source: "demo"}}}
	if got := byID(buildIntegrations(in, integrationNow))["service-probes"]; got.State != integrationNotConfigured {
		t.Fatalf("non-probe services must be ignored: %+v", got)
	}
}

func TestOpenCodeAutomationsAndSignInStates(t *testing.T) {
	provider := func(id, state, detail, freshness string) compute.Provider {
		return compute.Provider{ID: id, Name: id, Kind: "opencode", State: state, Detail: detail, Freshness: freshness, ObservedAt: integrationNow.Add(-10 * time.Second)}
	}
	in := liveInputs()
	if got := byID(buildIntegrations(in, integrationNow))["opencode"]; got.State != integrationNotConfigured {
		t.Fatalf("none configured: %+v", got)
	}
	in.compute = &compute.State{Providers: []compute.Provider{provider("a", compute.ProviderOnline, "", "fresh"), provider("b", compute.ProviderOnline, "", "fresh")}}
	if got := byID(buildIntegrations(in, integrationNow))["opencode"]; got.State != integrationHealthy || got.Detail != "2 of 2 servers reachable; 0 running sessions." {
		t.Fatalf("healthy: %+v", got)
	}
	in.compute.Providers[0].ObservedAt = integrationNow.Add(-2 * time.Minute)
	if got := byID(buildIntegrations(in, integrationNow))["opencode"]; got.State != integrationHealthy || got.Freshness != "fresh" {
		t.Fatalf("long-interval OpenCode provider must stay fresh: %+v", got)
	}
	in.compute = &compute.State{Providers: []compute.Provider{provider("a", compute.ProviderOnline, "", "fresh"), provider("b", compute.ProviderOffline, "connection refused", "fresh")}}
	if got := byID(buildIntegrations(in, integrationNow))["opencode"]; got.State != integrationDegraded || !strings.Contains(got.Detail, "unavailable: b (connection refused)") {
		t.Fatalf("one down: %+v", got)
	}
	in.compute = &compute.State{Providers: []compute.Provider{provider("a", compute.ProviderOnline, "", "stale")}}
	if got := byID(buildIntegrations(in, integrationNow))["opencode"]; got.State != integrationUnhealthy || got.Freshness != "stale" {
		t.Fatalf("stale reading: %+v", got)
	}

	if got := byID(buildIntegrations(in, integrationNow))["automations"]; got.State != integrationNotConfigured {
		t.Fatalf("automations off: %+v", got)
	}
	in.automations = &workflow.Summary{Enabled: 2, ScheduledCount: 1, GeneratedAt: integrationNow}
	if got := byID(buildIntegrations(in, integrationNow))["automations"]; got.State != integrationHealthy || got.Detail != "2 enabled workflows, 1 scheduled, 0 failed runs in the last 24 hours." {
		t.Fatalf("automations healthy: %+v", got)
	}
	in.automations.FailedLast24h = 3
	if got := byID(buildIntegrations(in, integrationNow))["automations"]; got.State != integrationDegraded {
		t.Fatalf("automations with failures: %+v", got)
	}
	in.automationsErr = true
	if got := byID(buildIntegrations(in, integrationNow))["automations"]; got.State != integrationUnknown {
		t.Fatalf("automations unreadable: %+v", got)
	}

	if got := byID(buildIntegrations(in, integrationNow))["authentication"]; got.State != integrationHealthy {
		t.Fatalf("auth enabled: %+v", got)
	}
	in.authEnabled = false
	if got := byID(buildIntegrations(in, integrationNow))["authentication"]; got.State != integrationDegraded || !strings.Contains(got.Detail, "loopback") {
		t.Fatalf("loopback only: %+v", got)
	}
	in.mutations = false
	if got := byID(buildIntegrations(in, integrationNow))["authentication"]; !strings.Contains(got.Detail, "changes are disabled") {
		t.Fatalf("no changes: %+v", got)
	}
}

func TestNameListIsBoundedAndSorted(t *testing.T) {
	if got := names([]string{"d", "b", "a", "c", "e", "f"}, 4); got != "a, b, c, d and 2 more" {
		t.Fatalf("names: %q", got)
	}
}

func TestIntegrationsEndpointUsesRealRefreshTime(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "i.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := inventory.Repository{DB: database}
	s := &Server{DB: database, Repo: repo, SourceMode: "tailscale", AuthEnabled: true, Now: func() time.Time { return time.Now().UTC() }}
	h := s.Handler()
	get := func() map[string]Integration {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/integrations", nil))
		if r.Code != 200 {
			t.Fatalf("status %d %s", r.Code, r.Body.String())
		}
		var body struct {
			Items []Integration `json:"items"`
		}
		if err := jsonUnmarshal(r.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return byID(body.Items)
	}
	if got := get()["inventory"]; got.State != integrationUnknown {
		t.Fatalf("before any refresh: %+v", got)
	}
	if err := repo.ReplaceSnapshot(context.Background(), domain.Snapshot{Nodes: []domain.Node{{ID: "tailscale-1", Name: "gw", Role: "Tailnet device", Health: domain.Health{State: "unknown", Source: "tailscale", ObservedAt: time.Now().UTC(), StaleAfterSeconds: 180}}}}); err != nil {
		t.Fatal(err)
	}
	got := get()
	if got["inventory"].State != integrationHealthy || got["inventory"].ObservedAt == nil || got["compute-not-present"].ID != "" {
		t.Fatalf("after refresh: %+v", got["inventory"])
	}
	if got["authentication"].State != integrationHealthy || got["automations"].State != integrationNotConfigured {
		t.Fatalf("auth/automations: %+v %+v", got["authentication"], got["automations"])
	}
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/compute"
	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

type stubCompute struct{ state compute.State }

func (s stubCompute) State(time.Time) compute.State { return s.state }

func computeServer(t *testing.T, source compute.Source) http.Handler {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	now := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	repo := inventory.Repository{DB: database}
	if err := repo.UpsertSnapshot(context.Background(), demo.Provider{}.Snapshot(now)); err != nil {
		t.Fatal(err)
	}
	return (&Server{DB: database, Repo: repo, Compute: source, Now: func() time.Time { return now }}).Handler()
}

func getJSON(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
	if r.Code != 200 {
		t.Fatalf("%s: %d %s", path, r.Code, r.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestComputeEndpointsWithoutAProviderStayEmptyAndHonest(t *testing.T) {
	h := computeServer(t, nil)
	if items := getJSON(t, h, "/api/v1/jobs")["items"].([]any); len(items) != 0 {
		t.Fatalf("jobs: %+v", items)
	}
	if items := getJSON(t, h, "/api/v1/compute/providers")["items"].([]any); len(items) != 0 {
		t.Fatalf("providers: %+v", items)
	}
	summary := getJSON(t, h, "/api/v1/compute/summary")
	if _, present := summary["jobs_source"]; present || summary["active_jobs"].(float64) != 0 {
		t.Fatalf("summary must not claim a job source: %+v", summary)
	}
	for _, feature := range getJSON(t, h, "/api/v1/system")["features"].([]any) {
		if feature == "opencode" {
			t.Fatal("opencode feature should not be listed")
		}
	}
}

func TestComputeEndpointsReportProviderJobsAndCounts(t *testing.T) {
	at := time.Date(2026, 10, 19, 0, 59, 50, 0, time.UTC)
	state := compute.State{
		ObservedAt: at,
		Providers: []compute.Provider{
			{ID: "ox-c-01", Name: "OX-C-01 OpenCode", Kind: "opencode", Endpoint: "http://100.69.88.98:4096", NodeID: "tailscale-2365559827781033", State: compute.ProviderOnline, Version: "1.19.3", ActiveSessions: 2, TotalSessions: 5, ObservedAt: at, StaleAfterSecs: 45, Freshness: "fresh"},
			{ID: "ox-c-02", Name: "OX-C-02 OpenCode", Kind: "opencode", Endpoint: "http://100.0.0.2:4096", State: compute.ProviderOffline, Detail: "connection refused", ObservedAt: at, StaleAfterSecs: 45, Freshness: "fresh"},
		},
		Jobs: []compute.Job{
			{ID: "ox-c-01:ses_a", Name: "Build report", State: compute.JobRunning, ProviderID: "ox-c-01", Source: "opencode", Agent: "build", Model: "ollama/qwen2.5:3b", Tokens: 36, StartedAt: at, UpdatedAt: at, Freshness: "fresh"},
			{ID: "ox-c-01:ses_b", Name: "Old", State: compute.JobRunning, ProviderID: "ox-c-01", Source: "opencode", Freshness: "fresh"},
			{ID: "ox-c-02:ses_c", Name: "On an offline host", State: compute.JobRunning, ProviderID: "ox-c-02", Source: "opencode", Freshness: "stale"},
			{ID: "ox-c-01:ses_d", Name: "Idle one", State: compute.JobIdle, ProviderID: "ox-c-01", Source: "opencode", Freshness: "fresh"},
		},
	}
	h := computeServer(t, stubCompute{state: state})
	jobs := getJSON(t, h, "/api/v1/jobs")["items"].([]any)
	if len(jobs) != 4 || jobs[0].(map[string]any)["agent"] != "build" || jobs[0].(map[string]any)["source"] != "opencode" {
		t.Fatalf("jobs: %+v", jobs)
	}
	providers := getJSON(t, h, "/api/v1/compute/providers")["items"].([]any)
	first := providers[0].(map[string]any)
	if len(providers) != 2 || first["kind"] != "opencode" || first["active_sessions"].(float64) != 2 || first["endpoint"] != "http://100.69.88.98:4096" {
		t.Fatalf("providers: %+v", providers)
	}
	summary := getJSON(t, h, "/api/v1/compute/summary")
	// Only jobs on a fresh, online provider count as active.
	if summary["active_jobs"].(float64) != 2 || summary["jobs_source"] != "opencode" || summary["queued_jobs"].(float64) != 0 {
		t.Fatalf("summary: %+v", summary)
	}
	found := false
	for _, feature := range getJSON(t, h, "/api/v1/system")["features"].([]any) {
		found = found || feature == "opencode"
	}
	if !found {
		t.Fatal("system features should list opencode")
	}
}

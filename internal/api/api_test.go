package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

func TestAPIListsDemoDataAndHealth(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	repo := inventory.Repository{DB: database}
	if err := repo.UpsertSnapshot(context.Background(), demo.Provider{}.Snapshot(now)); err != nil {
		t.Fatal(err)
	}
	s := &Server{DB: database, Repo: repo, SiteName: "Test", Now: func() time.Time { return now }}
	h := s.Handler()
	for _, endpoint := range []string{"/healthz", "/readyz", "/api/v1/system", "/api/v1/nodes", "/api/v1/services", "/api/v1/activity", "/api/v1/nodes/demo-worker/metrics", "/api/v1/compute/workers", "/api/v1/compute/summary", "/api/v1/jobs", "/api/v1/network/graph", "/api/v1/network/connections"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if r.Code != 200 {
			t.Fatalf("%s: %d %s", endpoint, r.Code, r.Body.String())
		}
		if r.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s missing request ID", endpoint)
		}
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil))
	var body struct {
		Items []domain.Node `json:"items"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 3 || !body.Items[0].Simulated || body.Items[2].Freshness != "stale" || body.Items[1].Metrics.CPUPercent != nil {
		t.Fatalf("unexpected demo data: %+v", body.Items)
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/nodes/missing", nil))
	if r.Code != 404 {
		t.Fatalf("missing node: %d", r.Code)
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/compute/summary", nil))
	var summary struct {
		TotalVCPU int    `json:"total_vcpu"`
		UsedVCPU  int    `json:"used_vcpu"`
		Simulated bool   `json:"simulated"`
		Freshness string `json:"freshness"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.TotalVCPU != 8 || summary.UsedVCPU != 0 || !summary.Simulated || summary.Freshness != "stale" {
		t.Fatalf("unexpected compute summary: %+v", summary)
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/network/graph", nil))
	var graph struct {
		Nodes []struct {
			ID        string `json:"id"`
			Simulated bool   `json:"simulated"`
		} `json:"nodes"`
		Edges []struct {
			ID        string `json:"id"`
			Freshness string `json:"freshness"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 6 || len(graph.Edges) != 3 || !graph.Nodes[0].Simulated || graph.Edges[1].Freshness != "stale" {
		t.Fatalf("unexpected graph: %+v", graph)
	}
}

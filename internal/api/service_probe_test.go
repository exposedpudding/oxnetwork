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
	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

func TestServicesEndpointExposesProbeEndpointAndDetail(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "probe-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	repo := inventory.Repository{DB: database}
	services := []domain.Service{
		{ID: "probe-edge", Name: "oxnet.work edge", Endpoint: "https://oxnet.work/readyz", Detail: "HTTP 200 in 11 ms", Health: domain.Health{State: "healthy", ObservedAt: now.Add(-10 * time.Second), StaleAfterSeconds: 90, Source: "http-probe"}},
		{ID: "probe-auth", Name: "Authentik", Endpoint: "https://auth.oxnet.work/-/health/live/", Detail: "no response within 3s", Health: domain.Health{State: "unhealthy", ObservedAt: now.Add(-5 * time.Minute), StaleAfterSeconds: 90, Source: "http-probe"}},
	}
	if err := repo.ReplaceSnapshot(context.Background(), domain.Snapshot{Services: services}); err != nil {
		t.Fatal(err)
	}
	h := (&Server{DB: database, Repo: repo, SourceMode: "tailscale", Now: func() time.Time { return now }}).Handler()
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/services", nil))
	if r.Code != 200 {
		t.Fatalf("status %d", r.Code)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items: %+v", body.Items)
	}
	auth, edge := body.Items[0], body.Items[1]
	if auth["id"] != "probe-auth" || auth["endpoint"] != "https://auth.oxnet.work/-/health/live/" || auth["detail"] != "no response within 3s" || auth["freshness"] != "stale" || auth["simulated"] != false {
		t.Fatalf("auth service: %+v", auth)
	}
	health := edge["health"].(map[string]any)
	if edge["freshness"] != "fresh" || health["state"] != "healthy" || health["source"] != "http-probe" {
		t.Fatalf("edge service: %+v", edge)
	}
	// Single-service lookup carries the same fields.
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/services/probe-edge", nil))
	var one map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &one); err != nil || one["endpoint"] != "https://oxnet.work/readyz" {
		t.Fatalf("service detail: %v %+v", err, one)
	}
}

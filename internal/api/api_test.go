package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
)

func TestAPIListsDemoDataAndHealth(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	s := &Server{DB: database, Source: demo.Provider{}, SiteName: "Test", Now: func() time.Time { return now }}
	h := s.Handler()
	for _, endpoint := range []string{"/healthz", "/readyz", "/api/v1/system", "/api/v1/nodes", "/api/v1/services", "/api/v1/activity"} {
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
	if len(body.Items) != 3 || !body.Items[0].Simulated || body.Items[1].Freshness != "stale" || body.Items[2].Metrics.CPUPercent != nil {
		t.Fatalf("unexpected demo data: %+v", body.Items)
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/nodes/missing", nil))
	if r.Code != 404 {
		t.Fatalf("missing node: %d", r.Code)
	}
}

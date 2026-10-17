package tailscale

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSnapshotMapsOnlyReportedDeviceIdentity(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, password, ok := r.BasicAuth()
		if !ok || key != "secret" || password != "" || r.Method != http.MethodGet || r.URL.Path != "/api/v2/tailnet/example.ts.net/devices" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"devices":[{"id":"123","name":"home-gw.example.ts.net","hostname":"home-gw","os":"linux","addresses":["100.64.0.10"],"lastSeen":"2026-10-17T11:59:00Z"}]}`))
	}))
	defer server.Close()
	snapshot, err := (Client{Tailnet: "example.ts.net", APIKey: "secret", BaseURL: server.URL}).Snapshot(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) != 1 || len(snapshot.Services) != 0 || len(snapshot.Connections) != 0 || len(snapshot.Activity) != 0 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	node := snapshot.Nodes[0]
	if node.ID != "tailscale-123" || node.Name != "home-gw" || node.Health.State != "unknown" || node.Simulated || node.Metrics.CPUPercent != nil || len(node.Addresses) != 1 || node.LastSeen == nil {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func TestSnapshotDoesNotExposeKeyOnAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret", http.StatusUnauthorized) }))
	defer server.Close()
	_, err := (Client{Tailnet: "example.ts.net", APIKey: "secret", BaseURL: server.URL}).Snapshot(context.Background(), time.Now())
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("unexpected error: %v", err)
	}
}

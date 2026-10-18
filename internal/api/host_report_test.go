package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

func TestHostReportRequiresTokenAndFixedInventoryNode(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	repo := inventory.Repository{DB: database}
	tokens := t.TempDir()
	if err := os.WriteFile(filepath.Join(tokens, "tailscale-1"), []byte(strings.Repeat("a", 32)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceSnapshot(context.Background(), domain.Snapshot{Nodes: []domain.Node{{ID: "tailscale-1", Name: "home-gw", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 180, Source: "tailscale"}}}}); err != nil {
		t.Fatal(err)
	}
	h := (&Server{Repo: repo, SourceMode: "tailscale", HostAgentTokensDir: tokens, Now: func() time.Time { return now }}).Handler()
	post := func(token, body string) int {
		r := httptest.NewRequest(http.MethodPost, "/internal/v1/host-report/tailscale-1", strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	good := `{"cpu_percent":12,"ram_percent":30,"disk_percent":40,"vcpu":4,"ram_gb":16}`
	if code := post("", good); code != 401 {
		t.Fatalf("missing token: %d", code)
	}
	if code := post(strings.Repeat("b", 32), good); code != 401 {
		t.Fatalf("wrong token: %d", code)
	}
	if code := post(strings.Repeat("a", 32), `{"cpu_percent":101,"ram_percent":30,"disk_percent":40,"vcpu":4,"ram_gb":16}`); code != 400 {
		t.Fatalf("invalid sample: %d", code)
	}
	if code := post(strings.Repeat("a", 32), good); code != 204 {
		t.Fatalf("valid sample: %d", code)
	}
	got, err := repo.Snapshot(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Nodes[0].Health.Source != "host-agent" || got.Nodes[0].Metrics.CPUPercent == nil {
		t.Fatalf("sample not applied: %+v", got.Nodes[0])
	}
}

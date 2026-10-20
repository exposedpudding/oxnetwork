package api

import (
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

const goodContainerBody = `{"containers":[{"id":"0123456789ab","name":"web","image":"example/web:1","state":"running","status":"Up 1 hour (healthy)","health":"healthy","created_at":"2026-10-19T10:00:00Z","ports":["127.0.0.1:8080->80/tcp"],"compose_project":"oxnet","compose_service":"web"}]}`

func TestDockerReportRequiresItsOwnTokenAndRejectsExtraFields(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := inventory.Repository{DB: database}
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	node := domain.Node{ID: "tailscale-1", Name: "home-gw", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 180, Source: "tailscale"}}
	if err := repo.ReplaceSnapshot(nil2ctx(), domain.Snapshot{Nodes: []domain.Node{node}}); err != nil {
		t.Fatal(err)
	}
	dockerTokens, hostTokens := t.TempDir(), t.TempDir()
	dockerToken, hostToken := strings.Repeat("d", 32), strings.Repeat("h", 32)
	if err := os.WriteFile(filepath.Join(dockerTokens, "tailscale-1"), []byte(dockerToken+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostTokens, "tailscale-1"), []byte(hostToken), 0600); err != nil {
		t.Fatal(err)
	}
	server := &Server{Repo: repo, SourceMode: "tailscale", HostAgentTokensDir: hostTokens, DockerAgentTokensDir: dockerTokens, Now: func() time.Time { return now }}
	h := server.Handler()
	post := func(path, token, body string) int {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	const path = "/internal/v1/docker-report/tailscale-1"
	if code := post(path, "", goodContainerBody); code != 401 {
		t.Fatalf("missing token: %d", code)
	}
	if code := post(path, strings.Repeat("x", 32), goodContainerBody); code != 401 {
		t.Fatalf("wrong token: %d", code)
	}
	if code := post(path, hostToken, goodContainerBody); code != 401 {
		t.Fatalf("a host-metrics token must not authorise container reports: %d", code)
	}
	if code := post("/internal/v1/host-report/tailscale-1", dockerToken, `{"cpu_percent":1,"ram_percent":1,"disk_percent":1,"vcpu":1,"ram_gb":1}`); code != 401 {
		t.Fatalf("a container token must not authorise host reports: %d", code)
	}
	if code := post("/internal/v1/docker-report/not-a-node", dockerToken, goodContainerBody); code != 404 {
		t.Fatalf("bad node id: %d", code)
	}
	for name, body := range map[string]string{
		"environment smuggled in": strings.Replace(goodContainerBody, `"name":"web"`, `"name":"web","env":["PASSWORD=x"]`, 1),
		"unknown top level field": `{"containers":[],"labels":{"a":"b"}}`,
		"trailing document":       goodContainerBody + `{}`,
		"bad state":               strings.Replace(goodContainerBody, `"running"`, `"flying"`, 1),
		"not json":                `containers`,
	} {
		if code := post(path, dockerToken, body); code != 400 {
			t.Errorf("%s: got %d, want 400", name, code)
		}
	}
	if code := post(path, dockerToken, strings.Repeat(" ", maxContainerReportBytes+10)+goodContainerBody); code != 400 {
		t.Fatalf("an oversized body must be rejected: %d", code)
	}
	if code := post("/internal/v1/docker-report/tailscale-2", dockerToken, goodContainerBody); code != 401 {
		t.Fatalf("no token file for that node: %d", code)
	}
	if code := post(path, dockerToken, goodContainerBody); code != 204 {
		t.Fatalf("good report: %d", code)
	}

	get := func(path string) (int, string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Code, w.Body.String()
	}
	code, body := get("/api/v1/nodes/tailscale-1/containers")
	if code != 200 || !strings.Contains(body, `"reported":true`) || !strings.Contains(body, `"name":"web"`) || !strings.Contains(body, `"freshness":"fresh"`) || strings.Contains(body, "PASSWORD") {
		t.Fatalf("containers: %d %s", code, body)
	}
	if code, _ := get("/api/v1/nodes/tailscale-404/containers"); code != 404 {
		t.Fatalf("unknown node: %d", code)
	}
}

func TestDockerReportDisabledWithoutTokenDirOrInDemoMode(t *testing.T) {
	for name, server := range map[string]*Server{
		"no token directory": {SourceMode: "tailscale", DockerAgentTokensDir: ""},
		"demo mode":          {SourceMode: "demo", DockerAgentTokensDir: t.TempDir()},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/internal/v1/docker-report/tailscale-1", strings.NewReader(goodContainerBody))
		server.Handler().ServeHTTP(w, r)
		if w.Code != 404 {
			t.Errorf("%s: got %d, want 404", name, w.Code)
		}
	}
}

func TestContainersEndpointSaysWhenNothingReported(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := inventory.Repository{DB: database}
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	node := domain.Node{ID: "tailscale-1", Name: "home-gw", Health: domain.Health{State: "unknown", ObservedAt: now, StaleAfterSeconds: 180, Source: "tailscale"}}
	if err := repo.ReplaceSnapshot(nil2ctx(), domain.Snapshot{Nodes: []domain.Node{node}}); err != nil {
		t.Fatal(err)
	}
	h := (&Server{Repo: repo, SourceMode: "tailscale", Now: func() time.Time { return now }}).Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/nodes/tailscale-1/containers", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"reported":false`) || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("never reported: %d %s", w.Code, w.Body.String())
	}
}

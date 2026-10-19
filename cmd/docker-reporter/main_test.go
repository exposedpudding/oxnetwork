package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

// dockerListing is what a real engine returns, including fields this program must
// never forward: environment-like labels, a command line and a mount.
const dockerListing = `[
 {"Id":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","Names":["/oxnet-work"],"Image":"oxnet/work:local","ImageID":"sha256:aaa","Command":"/app/oxnet-work --token=SUPERSECRET","Created":1760868000,
  "Ports":[{"IP":"127.0.0.1","PrivatePort":8080,"PublicPort":8080,"Type":"tcp"},{"IP":"::1","PrivatePort":8080,"PublicPort":8080,"Type":"tcp"},{"PrivatePort":9000,"Type":"tcp"}],
  "Labels":{"com.docker.compose.project":"oxnet","com.docker.compose.service":"web","DB_PASSWORD":"hunter2"},"State":"running","Status":"Up 3 hours (healthy)",
  "Mounts":[{"Source":"/opt/oxnetwork/deploy/home-gw/secrets","Destination":"/run/secrets"}],"HostConfig":{"NetworkMode":"default"}},
 {"Id":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","Names":["/old-job"],"Image":"sha256:0123456789abcdef0123456789abcdef","Created":1760800000,"Ports":[],"Labels":null,"State":"Exited","Status":"Exited (0) 2 days ago"},
 {"Id":"1111111111112222222222223333333333334444444444445555555555556666","Names":["/probe-me"],"Image":"x/y","Created":1760800000,"Ports":[],"Labels":{"com.docker.compose.project":"bad name;rm -rf"},"State":"running","Status":"Up 5 seconds (health: starting)"},
 {"Id":"short","Names":["/bad"],"Image":"x","State":"running"},
 {"Id":"9999999999999999999999999999999999999999999999999999999999999999","Names":["/odd"],"Image":"x","State":"teleporting","Status":"?"}
]`

// fakeDocker serves a Unix socket like the Docker engine and records every request.
func fakeDocker(t *testing.T, body string) (socket string, requests func() []string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "dk")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket = filepath.Join(dir, "d.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	var mu sync.Mutex
	var seen []string
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		if r.URL.Path != "/containers/json" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return socket, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), seen...) }
}

func TestListContainersMakesOnlyTheAllowedRequest(t *testing.T) {
	socket, requests := fakeDocker(t, dockerListing)
	list, err := listContainers(context.Background(), socket)
	if err != nil || len(list) != 5 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	got := requests()
	if len(got) != 1 || got[0] != "GET /containers/json?all=1" {
		t.Fatalf("the only request allowed is GET /containers/json?all=1, saw %v", got)
	}
}

func TestMappingKeepsOnlyAllowListedFields(t *testing.T) {
	socket, _ := fakeDocker(t, dockerListing)
	list, err := listContainers(context.Background(), socket)
	if err != nil {
		t.Fatal(err)
	}
	report, skipped := toReport(list)
	if skipped != 2 || len(report.Containers) != 3 {
		t.Fatalf("expected 3 reported and 2 skipped (short id, unknown state): %d reported, %d skipped", len(report.Containers), skipped)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("mapped report must pass the server's own validation: %v", err)
	}
	encoded, _ := json.Marshal(report)
	for _, secret := range []string{"SUPERSECRET", "hunter2", "DB_PASSWORD", "/opt/oxnetwork", "secrets", "Command", "Mounts", "NetworkMode"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("report leaks %q: %s", secret, encoded)
		}
	}
	web := report.Containers[0]
	if web.ID != "0123456789ab" || web.Name != "oxnet-work" || web.State != "running" || web.Health != "healthy" || web.ComposeProject != "oxnet" || web.ComposeService != "web" {
		t.Fatalf("web: %+v", web)
	}
	if strings.Join(web.Ports, ",") != "127.0.0.1:8080->8080/tcp,::1:8080->8080/tcp" {
		t.Fatalf("only published ports, sorted: %v", web.Ports)
	}
	if web.CreatedAt != time.Unix(1760868000, 0).UTC() {
		t.Fatalf("created: %v", web.CreatedAt)
	}
	old := report.Containers[1]
	if old.State != "exited" || old.Image != "sha256:0123456789ab" || old.Ports == nil || len(old.Ports) != 0 || old.Health != "" {
		t.Fatalf("exited container: %+v", old)
	}
	probe := report.Containers[2]
	if probe.Health != "starting" || probe.ComposeProject != "" {
		t.Fatalf("starting health and an invalid compose label must be dropped: %+v", probe)
	}
}

func TestListContainersErrors(t *testing.T) {
	if _, err := listContainers(context.Background(), filepath.Join(t.TempDir(), "none.sock")); err == nil {
		t.Fatal("a missing socket must be an error")
	}
	socket, _ := fakeDocker(t, `{"message":"not a list"}`)
	if _, err := listContainers(context.Background(), socket); err == nil {
		t.Fatal("a non-list response must be an error")
	}
}

func TestReportURLAndSend(t *testing.T) {
	for _, ok := range []string{"https://oxnet.work/internal/v1/docker-report", "http://127.0.0.1:8080/internal/v1/docker-report"} {
		if _, err := reportURL(ok, "tailscale-1"); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://oxnet.work/internal/v1/docker-report", "https://oxnet.work/internal/v1/host-report", "https://u:p@oxnet.work/internal/v1/docker-report", "https://oxnet.work/internal/v1/docker-report?x=1"} {
		if _, err := reportURL(bad, "tailscale-1"); err == nil {
			t.Errorf("%s should be rejected", bad)
		}
	}
	if _, err := reportURL("https://oxnet.work/internal/v1/docker-report", "../x"); err == nil {
		t.Error("a malformed node id must be rejected")
	}

	var gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	report := inventory.ContainerReport{Containers: []inventory.Container{}}
	if err := send(context.Background(), server.Client(), server.URL, strings.Repeat("t", 32), report); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer "+strings.Repeat("t", 32) || gotBody != `{"containers":[]}` {
		t.Fatalf("auth %q body %q", gotAuth, gotBody)
	}
	rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer rejecting.Close()
	if err := send(context.Background(), rejecting.Client(), rejecting.URL, strings.Repeat("t", 32), report); err == nil {
		t.Fatal("a 401 must be an error")
	}
}

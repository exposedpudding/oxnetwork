package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/config"
	"github.com/exposedpudding/oxnetwork/internal/domain"
)

var t0 = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func prober(probes ...config.ServiceProbe) *Prober {
	return &Prober{Probes: probes, Interval: 30 * time.Second, Now: func() time.Time { return t0 }}
}

func one(t *testing.T, p *Prober) domain.Service {
	t.Helper()
	results := p.ProbeAll(context.Background())
	if len(results) != 1 {
		t.Fatalf("results: %+v", results)
	}
	return results[0]
}

func TestHealthyProbeRecordsEndpointDetailAndSource(t *testing.T) {
	var sawAuth, sawUA atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth.Store(r.Header.Get("Authorization"))
		sawUA.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	service := one(t, prober(config.ServiceProbe{ID: "authentik", Name: "Authentik", URL: server.URL + "/-/health/live/?token=secret"}))
	if service.ID != "probe-authentik" || service.Name != "Authentik" || service.Health.State != "healthy" || service.Health.Source != Source {
		t.Fatalf("service: %+v", service)
	}
	if strings.Contains(service.Endpoint, "secret") || strings.Contains(service.Endpoint, "?") || !strings.HasSuffix(service.Endpoint, "/-/health/live/") {
		t.Fatalf("endpoint must drop the query string: %q", service.Endpoint)
	}
	if !strings.HasPrefix(service.Detail, "HTTP 204 in ") {
		t.Fatalf("detail: %q", service.Detail)
	}
	if service.Health.StaleAfterSeconds != 90 || !service.Health.ObservedAt.Equal(t0) {
		t.Fatalf("health: %+v", service.Health)
	}
	if sawAuth.Load() != "" || sawUA.Load() != "oxnet.work-probe/1" {
		t.Fatalf("probe must send no credentials: auth=%v ua=%v", sawAuth.Load(), sawUA.Load())
	}
}

func TestNameDefaultsToIDAndCustomExpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	service := one(t, prober(config.ServiceProbe{ID: "app", URL: server.URL, ExpectStatus: []int{401}}))
	if service.Name != "app" || service.Health.State != "healthy" {
		t.Fatalf("service: %+v", service)
	}
}

func TestUnexpectedStatusIsUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer server.Close()
	service := one(t, prober(config.ServiceProbe{ID: "app", URL: server.URL}))
	if service.Health.State != "unhealthy" || service.Detail != "HTTP 502 (expected 200/204)" {
		t.Fatalf("service: %+v", service)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()
	service := one(t, prober(config.ServiceProbe{ID: "app", URL: redirector.URL}))
	if service.Health.State != "unhealthy" || !strings.HasPrefix(service.Detail, "HTTP 302") || targetHits.Load() != 0 {
		t.Fatalf("redirect was followed or treated as healthy: %+v hits=%d", service, targetHits.Load())
	}
	// An operator may explicitly accept the redirect status itself.
	accepted := one(t, prober(config.ServiceProbe{ID: "app", URL: redirector.URL, ExpectStatus: []int{302}}))
	if accepted.Health.State != "healthy" || targetHits.Load() != 0 {
		t.Fatalf("accepted redirect: %+v hits=%d", accepted, targetHits.Load())
	}
}

func TestSlowServiceTimesOutAsUnhealthy(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()
	defer close(done)
	started := time.Now()
	service := one(t, prober(config.ServiceProbe{ID: "slow", URL: server.URL, TimeoutSeconds: 1}))
	if time.Since(started) > 3*time.Second {
		t.Fatalf("probe ignored its deadline: %s", time.Since(started))
	}
	if service.Health.State != "unhealthy" || service.Detail != "no response within 1s" {
		t.Fatalf("service: %+v", service)
	}
}

func TestUntrustedTLSCertificateIsUnhealthy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	// The default client verifies TLS and does not trust the test server's CA.
	service := one(t, prober(config.ServiceProbe{ID: "tls", URL: server.URL}))
	if service.Health.State != "unhealthy" || !strings.Contains(service.Detail, "TLS certificate not trusted") {
		t.Fatalf("service: %+v", service)
	}
	// Trusting the CA (as a deployment with a private CA would) makes it healthy.
	trusted := prober(config.ServiceProbe{ID: "tls", URL: server.URL})
	trusted.Client = server.Client()
	if got := one(t, trusted); got.Health.State != "healthy" {
		t.Fatalf("trusted TLS: %+v", got)
	}
}

func TestConnectionRefusedAndBadHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()
	refused := one(t, prober(config.ServiceProbe{ID: "down", URL: url}))
	if refused.Health.State != "unhealthy" || refused.Detail != "connection refused" {
		t.Fatalf("refused: %+v", refused)
	}
	missing := one(t, prober(config.ServiceProbe{ID: "nohost", URL: "http://does-not-exist.invalid/"}))
	if missing.Health.State != "unhealthy" || missing.Detail != "name did not resolve" {
		t.Fatalf("missing host: %+v", missing)
	}
}

func TestLargeResponseBodyIsNotReadInFull(t *testing.T) {
	var written atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, 64<<10)
		for i := 0; i < 160; i++ { // up to 10 MB
			n, err := w.Write(chunk)
			written.Add(int64(n))
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()
	started := time.Now()
	service := one(t, prober(config.ServiceProbe{ID: "big", URL: server.URL}))
	if service.Health.State != "healthy" || time.Since(started) > 2*time.Second {
		t.Fatalf("service: %+v after %s", service, time.Since(started))
	}
}

func TestServicesSortedAndStaleAfterThreeRounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	p := prober(config.ServiceProbe{ID: "b", URL: server.URL}, config.ServiceProbe{ID: "a", URL: server.URL})
	if got := p.Services(t0); len(got) != 0 {
		t.Fatalf("services before the first round must be omitted, got %+v", got)
	}
	p.ProbeAll(context.Background())
	fresh := p.Services(t0.Add(60 * time.Second))
	if len(fresh) != 2 || fresh[0].ID != "probe-a" || fresh[1].ID != "probe-b" || fresh[0].Freshness != "fresh" {
		t.Fatalf("fresh: %+v", fresh)
	}
	stale := p.Services(t0.Add(91 * time.Second))
	if stale[0].Freshness != "stale" || stale[0].Health.State != "healthy" {
		t.Fatalf("stale: %+v", stale[0])
	}
}

func TestRunPublishesEachRoundAndStopsOnCancel(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	p := prober(config.ServiceProbe{ID: "app", URL: server.URL})
	p.Interval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	rounds := make(chan []domain.Service, 16)
	finished := make(chan struct{})
	go func() {
		p.Run(ctx, func(_ context.Context, services []domain.Service) { rounds <- services })
		close(finished)
	}()
	for i := 0; i < 2; i++ {
		select {
		case services := <-rounds:
			if len(services) != 1 || services[0].Health.State != "healthy" {
				t.Fatalf("round: %+v", services)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no round published")
		}
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
	if hits.Load() < 2 {
		t.Fatalf("hits: %d", hits.Load())
	}
}

func TestRunWithoutProbesReturnsImmediately(t *testing.T) {
	done := make(chan struct{})
	go func() { (&Prober{}).Run(context.Background(), nil); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run with no probes should return")
	}
}

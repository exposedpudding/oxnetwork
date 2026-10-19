// Package probe checks fixed, operator-configured service endpoints and turns
// the results into inventory service observations (ADR-014).
//
// A probe is a single GET with no credentials. It never follows redirects,
// always verifies TLS, ignores proxy environment variables, and reads at most a
// small prefix of the response body, which is discarded. A recent host-agent
// report is never treated as service health: a service is "healthy" only when
// its own endpoint returned an expected status within the deadline.
package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/config"
	"github.com/exposedpudding/oxnetwork/internal/domain"
)

const (
	// Source labels observations so the UI can tell probes from simulated or
	// agent data.
	Source = "http-probe"

	defaultTimeout = 3 * time.Second
	maxBodyRead    = 1024
	// A result is stale after this many missed rounds.
	staleAfterRounds = 3
)

var defaultExpect = []int{200, 204}

// Prober runs the configured probes and remembers the latest result of each.
type Prober struct {
	Probes   []config.ServiceProbe
	Interval time.Duration
	// Client is used for every request; tests inject one that trusts a test CA.
	// When nil, a hardened client is built (TLS verified, no redirects, no proxy).
	Client *http.Client
	Now    func() time.Time
	Log    *slog.Logger

	mu      sync.RWMutex
	results map[string]domain.Service
}

func (p *Prober) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func (p *Prober) interval() time.Duration {
	if p.Interval > 0 {
		return p.Interval
	}
	return time.Duration(config.DefaultProbeIntervalSecond) * time.Second
}

func (p *Prober) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	transport := &http.Transport{
		Proxy:                 nil, // never route private probes through a proxy
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	return &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// ProbeAll checks every probe once, concurrently, and stores the results.
func (p *Prober) ProbeAll(ctx context.Context) []domain.Service {
	client := p.client()
	results := make([]domain.Service, len(p.Probes))
	var wg sync.WaitGroup
	for i := range p.Probes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = p.check(ctx, client, p.Probes[i])
		}(i)
	}
	wg.Wait()
	p.mu.Lock()
	if p.results == nil {
		p.results = map[string]domain.Service{}
	}
	for _, service := range results {
		p.results[service.ID] = service
	}
	p.mu.Unlock()
	return results
}

// Services returns the latest result of every probe that has completed at
// least once, ordered by id. Freshness is recomputed for the given time.
func (p *Prober) Services(now time.Time) []domain.Service {
	p.mu.RLock()
	defer p.mu.RUnlock()
	services := make([]domain.Service, 0, len(p.results))
	for _, service := range p.results {
		service.Freshness = service.Health.Freshness(now)
		services = append(services, service)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].ID < services[j].ID })
	return services
}

// Run probes immediately and then every interval until ctx ends. After each
// round it calls publish with that round's results.
func (p *Prober) Run(ctx context.Context, publish func(context.Context, []domain.Service)) {
	if len(p.Probes) == 0 {
		return
	}
	ticker := time.NewTicker(p.interval())
	defer ticker.Stop()
	for {
		round := p.ProbeAll(ctx)
		if publish != nil && ctx.Err() == nil {
			publish(ctx, round)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ServiceID is the inventory id for a probe.
func ServiceID(probeID string) string { return "probe-" + probeID }

func (p *Prober) check(ctx context.Context, client *http.Client, cfg config.ServiceProbe) domain.Service {
	timeout := defaultTimeout
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	expect := cfg.ExpectStatus
	if len(expect) == 0 {
		expect = defaultExpect
	}
	name := cfg.Name
	if name == "" {
		name = cfg.ID
	}
	started := p.now()
	service := domain.Service{
		ID:       ServiceID(cfg.ID),
		Name:     name,
		Endpoint: displayEndpoint(cfg.URL),
		Health: domain.Health{
			State:             "unhealthy",
			ObservedAt:        started,
			StaleAfterSeconds: int((p.interval() * staleAfterRounds).Seconds()),
			Source:            Source,
		},
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		service.Detail = "invalid probe URL"
		return service
	}
	req.Header.Set("User-Agent", "oxnet.work-probe/1")
	req.Header.Set("Accept", "*/*")
	begin := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(begin)
	// The observation time is when the answer arrived, not when it was asked.
	service.Health.ObservedAt = p.now()
	if err != nil {
		service.Detail = describeError(reqCtx, err, timeout)
		return service
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, maxBodyRead)
	if slices.Contains(expect, resp.StatusCode) {
		service.Health.State = "healthy"
		service.Detail = fmt.Sprintf("HTTP %d in %d ms", resp.StatusCode, elapsed.Milliseconds())
		return service
	}
	service.Detail = fmt.Sprintf("HTTP %d (expected %s)", resp.StatusCode, joinInts(expect))
	return service
}

// describeError turns a transport error into a short message that never
// echoes the full URL or any response content.
func describeError(ctx context.Context, err error, timeout time.Duration) string {
	var cert *tls.CertificateVerificationError
	switch {
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded:
		return fmt.Sprintf("no response within %s", timeout)
	case errors.Is(err, context.Canceled):
		return "probe cancelled"
	case errors.As(err, &cert):
		return "TLS certificate not trusted or does not match the name"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "no such host"):
		return "name did not resolve"
	case strings.Contains(message, "connection refused"):
		return "connection refused"
	case strings.Contains(message, "tls:") || strings.Contains(message, "x509:"):
		return "TLS handshake failed"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Sprintf("no response within %s", timeout)
	}
	return "request failed"
}

// displayEndpoint removes the query string, which may carry tokens.
func displayEndpoint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return u.String()
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprint(value)
	}
	return strings.Join(parts, "/")
}

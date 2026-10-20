package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/compute"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
	"github.com/exposedpudding/oxnetwork/internal/workflow"
)

// Integration states. "not_configured" is shown on purpose: a source that is absent
// is different from one that is failing, and the page should say which.
const (
	integrationHealthy       = "healthy"
	integrationDegraded      = "degraded"
	integrationUnhealthy     = "unhealthy"
	integrationUnknown       = "unknown"
	integrationNotConfigured = "not_configured"
)

// tailscaleStaleAfter is three of the 60 second Tailscale refresh rounds.
const tailscaleStaleAfter = 180 * time.Second

// Integration is the health of one data source or subsystem. It is derived only from
// data the server already holds; building the list makes no network calls.
type Integration struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	State      string     `json:"state"`
	Detail     string     `json:"detail"`
	ObservedAt *time.Time `json:"observed_at"`
	Freshness  string     `json:"freshness"`
}

type integrationInputs struct {
	snapshot       domain.Snapshot
	sourceMode     string
	refreshedAt    time.Time
	hasRefresh     bool
	authEnabled    bool
	mutations      bool
	compute        *compute.State
	automations    *workflow.Summary
	automationsErr bool
	containers     []inventory.ContainerNodeSummary
}

func ago(now, then time.Time) string {
	d := now.Sub(then).Round(time.Second)
	switch {
	case d < 90*time.Second:
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	case d < 90*time.Minute:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
}

func names(list []string, limit int) string {
	sort.Strings(list)
	if len(list) > limit {
		return strings.Join(list[:limit], ", ") + fmt.Sprintf(" and %d more", len(list)-limit)
	}
	return strings.Join(list, ", ")
}

func freshnessOf(at time.Time, staleAfter time.Duration, now time.Time) string {
	if at.IsZero() {
		return "unknown"
	}
	if now.Sub(at) > staleAfter {
		return "stale"
	}
	return "fresh"
}

// buildIntegrations summarises every source for the Integrations page.
func buildIntegrations(in integrationInputs, now time.Time) []Integration {
	var items []Integration
	ptr := func(t time.Time) *time.Time {
		if t.IsZero() {
			return nil
		}
		return &t
	}

	// Inventory source.
	if in.sourceMode == "tailscale" {
		item := Integration{ID: "inventory", Name: "Tailscale device inventory", Kind: "inventory", State: integrationUnknown, Freshness: "unknown", Detail: "No refresh recorded yet."}
		if in.hasRefresh {
			item.ObservedAt = ptr(in.refreshedAt)
			item.Freshness = freshnessOf(in.refreshedAt, tailscaleStaleAfter, now)
			if item.Freshness == "fresh" {
				item.State, item.Detail = integrationHealthy, fmt.Sprintf("%d devices; last refresh %s.", len(in.snapshot.Nodes), ago(now, in.refreshedAt))
			} else {
				item.State, item.Detail = integrationUnhealthy, fmt.Sprintf("Last successful refresh %s; the device list may be out of date.", ago(now, in.refreshedAt))
			}
		}
		items = append(items, item)
	} else {
		items = append(items, Integration{ID: "inventory", Name: "Demo inventory provider", Kind: "inventory", State: integrationHealthy, Freshness: "fresh",
			Detail: "Simulated data. No live infrastructure is connected."})
	}

	// Host agents: nodes whose current reading comes from an agent.
	var enrolled, fresh int
	var staleNames, noAgent []string
	var latest time.Time
	for _, node := range in.snapshot.Nodes {
		if node.Simulated {
			continue
		}
		if node.Health.Source != "host-agent" {
			noAgent = append(noAgent, node.Name)
			continue
		}
		enrolled++
		if node.Freshness == "fresh" {
			fresh++
		} else {
			staleNames = append(staleNames, node.Name)
		}
		if node.Health.ObservedAt.After(latest) {
			latest = node.Health.ObservedAt
		}
	}
	agents := Integration{ID: "host-agents", Name: "Host metrics agents", Kind: "agents", ObservedAt: ptr(latest), Freshness: "unknown"}
	switch {
	case enrolled == 0:
		agents.State, agents.Detail = integrationNotConfigured, "No host agent has reported."
	default:
		agents.Freshness = freshnessOf(latest, 90*time.Second, now)
		agents.Detail = fmt.Sprintf("%d of %d enrolled hosts reporting", fresh, enrolled)
		switch {
		case fresh == enrolled:
			agents.State = integrationHealthy
		case fresh == 0:
			agents.State = integrationUnhealthy
		default:
			agents.State = integrationDegraded
		}
		if len(staleNames) > 0 {
			agents.Detail += "; not reporting: " + names(staleNames, 4)
		}
		if len(noAgent) > 0 {
			unit := "devices have"
			if len(noAgent) == 1 {
				unit = "device has"
			}
			agents.Detail += fmt.Sprintf("; %d %s no agent", len(noAgent), unit)
		}
		agents.Detail += "."
	}
	items = append(items, agents)

	// Docker reporters (ADR-019): nodes that send their container list.
	docker := Integration{ID: "docker", Name: "Docker container reporters", Kind: "containers", Freshness: "unknown"}
	if len(in.containers) == 0 {
		docker.State, docker.Detail = integrationNotConfigured, "No Docker reporter has reported."
	} else {
		var fresh, total, running int
		var latest time.Time
		var silent []string
		for _, summary := range in.containers {
			total += summary.Count
			running += summary.Running
			if now.Sub(summary.ReceivedAt) <= inventory.ContainerTTL*time.Second {
				fresh++
			} else {
				silent = append(silent, summary.NodeID)
			}
			if summary.ReceivedAt.After(latest) {
				latest = summary.ReceivedAt
			}
		}
		docker.ObservedAt, docker.Freshness = ptr(latest), freshnessOf(latest, inventory.ContainerTTL*time.Second, now)
		docker.Detail = fmt.Sprintf("%d of %d reporting nodes are current; %d containers, %d running", fresh, len(in.containers), total, running)
		switch {
		case fresh == len(in.containers):
			docker.State = integrationHealthy
		case fresh == 0:
			docker.State = integrationUnhealthy
		default:
			docker.State = integrationDegraded
		}
		if len(silent) > 0 {
			docker.Detail += "; not reporting: " + names(silent, 4)
		}
		docker.Detail += "."
	}
	items = append(items, docker)

	// Service probes.
	var probes, probesOK int
	var probeBad []string
	var probeLatest time.Time
	probeStale := false
	for _, service := range in.snapshot.Services {
		if service.Health.Source != "http-probe" {
			continue
		}
		probes++
		if service.Freshness != "fresh" {
			probeStale = true
		}
		if service.Health.State == "healthy" && service.Freshness == "fresh" {
			probesOK++
		} else {
			probeBad = append(probeBad, service.Name)
		}
		if service.Health.ObservedAt.After(probeLatest) {
			probeLatest = service.Health.ObservedAt
		}
	}
	probeItem := Integration{ID: "service-probes", Name: "Service health probes", Kind: "probes", ObservedAt: ptr(probeLatest), Freshness: "unknown"}
	if probes == 0 {
		probeItem.State, probeItem.Detail = integrationNotConfigured, "No service probes are configured."
	} else {
		probeItem.Freshness = "fresh"
		if probeStale {
			probeItem.Freshness = "stale"
		}
		probeItem.Detail = fmt.Sprintf("%d of %d services answering", probesOK, probes)
		switch {
		case probesOK == probes:
			probeItem.State = integrationHealthy
		case probesOK == 0:
			probeItem.State = integrationUnhealthy
		default:
			probeItem.State = integrationDegraded
		}
		if len(probeBad) > 0 {
			probeItem.Detail += "; failing: " + names(probeBad, 4)
		}
		probeItem.Detail += "."
	}
	items = append(items, probeItem)

	// OpenCode compute providers.
	openCode := Integration{ID: "opencode", Name: "OpenCode compute servers", Kind: "compute", Freshness: "unknown"}
	if in.compute == nil || len(in.compute.Providers) == 0 {
		openCode.State, openCode.Detail = integrationNotConfigured, "No OpenCode servers are configured."
	} else {
		var complete, partial int
		var attention []string
		var observed time.Time
		providerStale := false
		for _, provider := range in.compute.Providers {
			if provider.Freshness != "fresh" {
				providerStale = true
			}
			if provider.State == compute.ProviderOnline && provider.Freshness == "fresh" {
				complete++
			} else {
				if provider.State == compute.ProviderDegraded && provider.Freshness == "fresh" {
					partial++
				}
				reason := provider.Name
				if provider.Detail != "" {
					reason += " (" + provider.Detail + ")"
				}
				attention = append(attention, reason)
			}
			if provider.ObservedAt.After(observed) {
				observed = provider.ObservedAt
			}
		}
		total := len(in.compute.Providers)
		openCode.ObservedAt = ptr(observed)
		openCode.Freshness = "fresh"
		if providerStale {
			openCode.Freshness = "stale"
		}
		openCode.Detail = fmt.Sprintf("%d of %d servers have complete session readings", complete, total)
		if partial > 0 {
			openCode.Detail += "; running session count incomplete"
		} else {
			openCode.Detail += fmt.Sprintf("; %d running sessions", in.compute.ActiveJobs())
		}
		switch {
		case complete == total:
			openCode.State = integrationHealthy
		case complete == 0 && partial == 0:
			openCode.State = integrationUnhealthy
		default:
			openCode.State = integrationDegraded
		}
		if len(attention) > 0 {
			openCode.Detail += "; needs attention: " + names(attention, 3)
		}
		openCode.Detail += "."
	}
	items = append(items, openCode)

	// Automations.
	automations := Integration{ID: "automations", Name: "Automations", Kind: "automations", Freshness: "fresh"}
	switch {
	case in.automationsErr:
		automations.State, automations.Freshness, automations.Detail = integrationUnknown, "unknown", "The automation store could not be read."
	case in.automations == nil:
		automations.State, automations.Freshness, automations.Detail = integrationNotConfigured, "unknown", "Automations are not enabled on this server."
	default:
		s := in.automations
		automations.ObservedAt = ptr(s.GeneratedAt)
		automations.Detail = fmt.Sprintf("%d enabled workflows, %d scheduled, %d failed runs in the last 24 hours", s.Enabled, s.ScheduledCount, s.FailedLast24h)
		if s.FailedLast24h > 0 {
			automations.State = integrationDegraded
		} else {
			automations.State = integrationHealthy
		}
		automations.Detail += "."
	}
	items = append(items, automations)

	// Sign-in.
	auth := Integration{ID: "authentication", Name: "Sign-in and change protection", Kind: "security", Freshness: "fresh"}
	switch {
	case in.authEnabled:
		auth.State, auth.Detail = integrationHealthy, "Sign-in is required; changes need a session and a CSRF token."
	case in.mutations:
		auth.State, auth.Detail = integrationDegraded, "No sign-in is configured. Changes are allowed only because the server is bound to loopback."
	default:
		auth.State, auth.Detail = integrationDegraded, "No sign-in is configured; changes are disabled."
	}
	items = append(items, auth)
	return items
}

func (s *Server) listIntegrations(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w, r)
	if !ok {
		return
	}
	now := s.now()
	in := integrationInputs{snapshot: snapshot, sourceMode: s.sourceMode(), authEnabled: s.AuthEnabled, mutations: s.Runner != nil && s.MutationsAllowed}
	if at, has, err := s.Repo.RefreshedAt(r.Context()); err == nil {
		in.refreshedAt, in.hasRefresh = at, has
	}
	if summaries, err := s.Repo.ContainerSummaries(r.Context()); err == nil {
		in.containers = summaries
	}
	if s.Compute != nil {
		state := s.Compute.State(now)
		in.compute = &state
	}
	if s.Runner != nil {
		if summary, err := s.Runner.Store.Summary(r.Context()); err == nil {
			in.automations = &summary
		} else {
			in.automationsErr = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": buildIntegrations(in, now)})
}

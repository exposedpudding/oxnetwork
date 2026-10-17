package demo

import (
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

type Provider struct{}

func percent(v float64) *float64 { return &v }
func integer(v int) *int         { return &v }

func (Provider) Snapshot(now time.Time) domain.Snapshot {
	status := func(state string, age time.Duration) domain.Health {
		return domain.Health{State: state, ObservedAt: now.Add(-age).UTC(), StaleAfterSeconds: 90, Source: "demo"}
	}
	nodes := []domain.Node{
		{ID: "demo-gateway", Name: "Demo Gateway", Role: "Edge", Platform: "Linux", Health: status("healthy", 10*time.Second), Capabilities: []string{"monitor"}, Metrics: domain.Metrics{CPUPercent: percent(18), RAMPercent: percent(36), VCPU: integer(4), RAMGB: integer(8)}, Simulated: true},
		{ID: "demo-worker", Name: "Demo Worker", Role: "Compute", Platform: "Linux", Health: status("degraded", 3*time.Minute), Capabilities: []string{"compute", "monitor"}, Metrics: domain.Metrics{CPUPercent: percent(54), RAMPercent: percent(68), VCPU: integer(8), RAMGB: integer(32)}, Simulated: true},
		{ID: "demo-sensor", Name: "Demo Sensor", Role: "Device", Platform: "Not reported", Health: status("unknown", 5*time.Second), Capabilities: []string{"monitor"}, Metrics: domain.Metrics{}, Simulated: true},
	}
	services := []domain.Service{
		{ID: "demo-private-access", Name: "Private access", Health: status("healthy", 12*time.Second), Simulated: true},
		{ID: "demo-monitoring", Name: "Monitoring", Health: status("degraded", 2*time.Minute), Simulated: true},
		{ID: "demo-backups", Name: "Backups", Health: status("unknown", 8*time.Second), Simulated: true},
	}
	activity := []domain.Activity{
		{ID: "demo-activity-1", Title: "Demo inventory refreshed", Detail: "Example data loaded", State: "info", OccurredAt: now.Add(-2 * time.Minute).UTC(), Simulated: true},
		{ID: "demo-activity-2", Title: "Demo worker needs attention", Detail: "Sample observation is stale", State: "warning", OccurredAt: now.Add(-7 * time.Minute).UTC(), Simulated: true},
	}
	connections := []domain.Connection{
		{ID: "demo-link-worker", Source: "node:demo-gateway", Target: "node:demo-worker", Kind: "tailnet", ObservedAt: now.Add(-15 * time.Second).UTC(), StaleAfterSeconds: 90, SourceSystem: "demo", Simulated: true},
		{ID: "demo-link-sensor", Source: "node:demo-gateway", Target: "node:demo-sensor", Kind: "tailnet", ObservedAt: now.Add(-3 * time.Minute).UTC(), StaleAfterSeconds: 90, SourceSystem: "demo", Simulated: true},
		{ID: "demo-host-private-access", Source: "node:demo-gateway", Target: "service:demo-private-access", Kind: "hosts", ObservedAt: now.Add(-12 * time.Second).UTC(), StaleAfterSeconds: 90, SourceSystem: "demo", Simulated: true},
	}
	for i := range nodes {
		nodes[i].Freshness = nodes[i].Health.Freshness(now)
	}
	for i := range services {
		services[i].Freshness = services[i].Health.Freshness(now)
	}
	for i := range connections {
		connections[i].Freshness = connections[i].ObservedFreshness(now)
	}
	return domain.Snapshot{Nodes: nodes, Services: services, Activity: activity, Connections: connections}
}

package network

import (
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
)

func TestGraphUsesRecordedConnectionsOnly(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	snapshot := demo.Provider{}.Snapshot(now)
	graph := FromSnapshot(snapshot)
	if len(graph.Nodes) != 6 || len(graph.Edges) != 3 {
		t.Fatalf("unexpected graph sizes: %+v", graph)
	}
	if graph.Nodes[0].ID != "node:demo-gateway" || graph.Nodes[0].Kind != "edge" || !graph.Nodes[0].Simulated {
		t.Fatalf("unexpected first node: %+v", graph.Nodes[0])
	}
	if graph.Edges[0].ID != "demo-host-private-access" || graph.Edges[0].Source != "node:demo-gateway" || graph.Edges[0].Target != "service:demo-private-access" {
		t.Fatalf("unexpected first edge: %+v", graph.Edges[0])
	}
	snapshot.Connections = append(snapshot.Connections, domain.Connection{ID: "orphan", Source: "node:missing", Target: "node:demo-worker"})
	if got := len(FromSnapshot(snapshot).Edges); got != 3 {
		t.Fatalf("orphan link included: %d edges", got)
	}
}

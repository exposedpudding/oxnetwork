package network

import (
	"slices"
	"strings"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

type GraphNode struct {
	ID         string        `json:"id"`
	ResourceID string        `json:"resource_id"`
	Kind       string        `json:"kind"`
	Label      string        `json:"label"`
	Health     domain.Health `json:"health"`
	Freshness  string        `json:"freshness"`
	Simulated  bool          `json:"simulated"`
}

type Graph struct {
	Nodes []GraphNode         `json:"nodes"`
	Edges []domain.Connection `json:"edges"`
}

// FromSnapshot uses recorded connections only. It never infers a link from
// inventory membership, which would misrepresent an unknown relationship.
func FromSnapshot(snapshot domain.Snapshot) Graph {
	graph := Graph{Nodes: []GraphNode{}, Edges: []domain.Connection{}}
	ids := map[string]bool{}
	for _, node := range snapshot.Nodes {
		kind := "device"
		switch strings.ToLower(node.Role) {
		case "edge", "compute", "network":
			kind = strings.ToLower(node.Role)
		}
		id := "node:" + node.ID
		ids[id] = true
		graph.Nodes = append(graph.Nodes, GraphNode{ID: id, ResourceID: node.ID, Kind: kind, Label: node.Name, Health: node.Health, Freshness: node.Freshness, Simulated: node.Simulated})
	}
	for _, service := range snapshot.Services {
		id := "service:" + service.ID
		ids[id] = true
		graph.Nodes = append(graph.Nodes, GraphNode{ID: id, ResourceID: service.ID, Kind: "service", Label: service.Name, Health: service.Health, Freshness: service.Freshness, Simulated: service.Simulated})
	}
	for _, connection := range snapshot.Connections {
		if ids[connection.Source] && ids[connection.Target] && connection.Source != connection.Target {
			graph.Edges = append(graph.Edges, connection)
		}
	}
	slices.SortFunc(graph.Nodes, func(a, b GraphNode) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(graph.Edges, func(a, b domain.Connection) int { return strings.Compare(a.ID, b.ID) })
	return graph
}

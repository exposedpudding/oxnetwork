package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ContainerTTL is how long a node's container list stays fresh. It matches host
// reports (90 seconds): three missed 30 second rounds.
const ContainerTTL = 90

// MaxContainersPerReport bounds one report. A homelab host runs tens of containers.
const MaxContainersPerReport = 300

var (
	containerIDPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)
	composeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)
	containerStates    = map[string]bool{"created": true, "running": true, "paused": true, "restarting": true, "removing": true, "exited": true, "dead": true}
	containerHealths   = map[string]bool{"": true, "healthy": true, "unhealthy": true, "starting": true}
)

// Container is the least that is useful to show: identity, state and exposure. It never
// carries environment variables, mounts, command lines or labels other than the two
// Compose names (ADR-019), so a report cannot expose secrets even if a collector is
// changed to send more.
type Container struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	State          string    `json:"state"`
	Status         string    `json:"status"`
	Health         string    `json:"health"`
	CreatedAt      time.Time `json:"created_at"`
	Ports          []string  `json:"ports"`
	ComposeProject string    `json:"compose_project,omitempty"`
	ComposeService string    `json:"compose_service,omitempty"`
}

// ContainerReport is a node's complete container list at one moment. The server
// assigns the node and the receipt time.
type ContainerReport struct {
	Containers []Container `json:"containers"`
}

// ValidComposeName reports whether a Compose project or service label is a plain name.
func ValidComposeName(value string) bool { return composeNamePattern.MatchString(value) }

func tooLong(value string, max int) bool { return len(value) > max }

func printable(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func (c Container) validate() error {
	switch {
	case !containerIDPattern.MatchString(c.ID):
		return errors.New("container id must be 12 hex characters")
	case c.Name == "" || tooLong(c.Name, 128) || !printable(c.Name):
		return errors.New("container name is invalid")
	case c.Image == "" || tooLong(c.Image, 256) || !printable(c.Image):
		return errors.New("container image is invalid")
	case !containerStates[c.State]:
		return errors.New("container state is not recognised")
	case !containerHealths[c.Health]:
		return errors.New("container health is not recognised")
	case tooLong(c.Status, 128) || !printable(c.Status):
		return errors.New("container status is invalid")
	case len(c.Ports) > 32:
		return errors.New("too many published ports")
	case c.ComposeProject != "" && !composeNamePattern.MatchString(c.ComposeProject):
		return errors.New("compose project is invalid")
	case c.ComposeService != "" && !composeNamePattern.MatchString(c.ComposeService):
		return errors.New("compose service is invalid")
	case c.CreatedAt.IsZero():
		return errors.New("container creation time is required")
	}
	for _, port := range c.Ports {
		if port == "" || tooLong(port, 64) || !printable(port) {
			return errors.New("published port is invalid")
		}
	}
	return nil
}

// Validate checks bounds and rejects duplicate container ids.
func (r ContainerReport) Validate() error {
	if len(r.Containers) > MaxContainersPerReport {
		return fmt.Errorf("at most %d containers per report", MaxContainersPerReport)
	}
	seen := map[string]bool{}
	for _, c := range r.Containers {
		if err := c.validate(); err != nil {
			return err
		}
		if seen[c.ID] {
			return errors.New("duplicate container id")
		}
		seen[c.ID] = true
	}
	return nil
}

// StoreContainerReport replaces the node's container list. The node must exist in the
// inventory, as for host reports.
func (r Repository) StoreContainerReport(ctx context.Context, nodeID string, report ContainerReport, receivedAt time.Time) error {
	if nodeID == "" {
		return errors.New("node ID is required")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	var exists int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM inventory_nodes WHERE id=?`, nodeID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("node %s is absent from inventory", nodeID)
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM node_containers WHERE node_id=?`, nodeID); err != nil {
		return err
	}
	for _, c := range report.Containers {
		ports := c.Ports
		if ports == nil {
			ports = []string{}
		}
		body, err := json.Marshal(ports)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO node_containers(node_id,container_id,name,image,state,status,health,created_at,ports_json,compose_project,compose_service)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, nodeID, c.ID, c.Name, c.Image, c.State, c.Status, c.Health, c.CreatedAt.UTC().Format(time.RFC3339), string(body), c.ComposeProject, c.ComposeService); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO container_reports(node_id,received_at) VALUES(?,?)
		ON CONFLICT(node_id) DO UPDATE SET received_at=excluded.received_at`, nodeID, receivedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

// NodeContainers is a node's latest list with its freshness. Reported is false when the
// node has never sent a container report.
type NodeContainers struct {
	Reported   bool        `json:"reported"`
	ReportedAt *time.Time  `json:"reported_at"`
	Freshness  string      `json:"freshness"`
	Items      []Container `json:"items"`
}

func freshnessAt(at, now time.Time) string {
	if now.Sub(at) > ContainerTTL*time.Second {
		return "stale"
	}
	return "fresh"
}

// NodeContainers reads one node's list. It performs no network call.
func (r Repository) NodeContainers(ctx context.Context, nodeID string, now time.Time) (NodeContainers, error) {
	result := NodeContainers{Freshness: "unknown", Items: []Container{}}
	var received string
	err := r.DB.QueryRowContext(ctx, `SELECT received_at FROM container_reports WHERE node_id=?`, nodeID).Scan(&received)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	at, err := time.Parse(time.RFC3339Nano, received)
	if err != nil {
		return result, err
	}
	result.Reported, result.ReportedAt, result.Freshness = true, &at, freshnessAt(at, now)
	rows, err := r.DB.QueryContext(ctx, `SELECT container_id,name,image,state,status,health,created_at,ports_json,compose_project,compose_service
		FROM node_containers WHERE node_id=?`, nodeID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Container
		var created, ports string
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.State, &c.Status, &c.Health, &created, &ports, &c.ComposeProject, &c.ComposeService); err != nil {
			return result, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, created)
		if err := json.Unmarshal([]byte(ports), &c.Ports); err != nil || c.Ports == nil {
			c.Ports = []string{}
		}
		result.Items = append(result.Items, c)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	sort.Slice(result.Items, func(i, j int) bool {
		return strings.ToLower(result.Items[i].Name) < strings.ToLower(result.Items[j].Name)
	})
	return result, nil
}

// ContainerNodeSummary is one node's report time and container count, for the
// integration health page.
type ContainerNodeSummary struct {
	NodeID     string
	ReceivedAt time.Time
	Count      int
	Running    int
}

// ContainerSummaries lists every node that has reported.
func (r Repository) ContainerSummaries(ctx context.Context) ([]ContainerNodeSummary, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT r.node_id, r.received_at,
		(SELECT COUNT(*) FROM node_containers c WHERE c.node_id=r.node_id),
		(SELECT COUNT(*) FROM node_containers c WHERE c.node_id=r.node_id AND c.state='running')
		FROM container_reports r ORDER BY r.node_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContainerNodeSummary
	for rows.Next() {
		var s ContainerNodeSummary
		var received string
		if err := rows.Scan(&s.NodeID, &received, &s.Count, &s.Running); err != nil {
			return nil, err
		}
		s.ReceivedAt, _ = time.Parse(time.RFC3339Nano, received)
		out = append(out, s)
	}
	return out, rows.Err()
}

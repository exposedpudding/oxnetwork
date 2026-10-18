package inventory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

const hostReportTTL = 90

// HostReport is a bounded, read-only sample from an installed host collector.
// The server assigns the node identity and receipt time; neither is trusted
// from the collector's JSON body.
type HostReport struct {
	CPUPercent  float64 `json:"cpu_percent"`
	RAMPercent  float64 `json:"ram_percent"`
	DiskPercent float64 `json:"disk_percent"`
	VCPU        int     `json:"vcpu"`
	RAMGB       int     `json:"ram_gb"`
}

func (r HostReport) Validate() error {
	for _, value := range []float64{r.CPUPercent, r.RAMPercent, r.DiskPercent} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			return errors.New("host percentages must be finite and between 0 and 100")
		}
	}
	if r.VCPU < 1 || r.VCPU > 4096 || r.RAMGB < 1 || r.RAMGB > 1048576 {
		return errors.New("host capacity is out of range")
	}
	return nil
}

func (r Repository) StoreHostReport(ctx context.Context, nodeID string, sample HostReport, receivedAt time.Time) error {
	if nodeID == "" {
		return errors.New("host node ID is required")
	}
	if err := sample.Validate(); err != nil {
		return err
	}
	var exists int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM inventory_nodes WHERE id=?`, nodeID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("host node %s is absent from inventory", nodeID)
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO host_observations(node_id,received_at,cpu_percent,ram_percent,disk_percent,vcpu,ram_gb)
		VALUES(?,?,?,?,?,?,?) ON CONFLICT(node_id) DO UPDATE SET received_at=excluded.received_at,cpu_percent=excluded.cpu_percent,
		ram_percent=excluded.ram_percent,disk_percent=excluded.disk_percent,vcpu=excluded.vcpu,ram_gb=excluded.ram_gb`,
		nodeID, receivedAt.UTC().Format(time.RFC3339Nano), sample.CPUPercent, sample.RAMPercent, sample.DiskPercent, sample.VCPU, sample.RAMGB)
	return err
}

func (r Repository) applyHostObservations(ctx context.Context, snapshot *domain.Snapshot, now time.Time) error {
	rows, err := r.DB.QueryContext(ctx, `SELECT node_id,received_at,cpu_percent,ram_percent,disk_percent,vcpu,ram_gb FROM host_observations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := make(map[string]*domain.Node, len(snapshot.Nodes))
	for i := range snapshot.Nodes {
		byID[snapshot.Nodes[i].ID] = &snapshot.Nodes[i]
	}
	for rows.Next() {
		var nodeID, received string
		var sample HostReport
		if err := rows.Scan(&nodeID, &received, &sample.CPUPercent, &sample.RAMPercent, &sample.DiskPercent, &sample.VCPU, &sample.RAMGB); err != nil {
			return err
		}
		node := byID[nodeID]
		if node == nil || node.Simulated {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, received)
		if err != nil {
			return err
		}
		if at.After(now.Add(5 * time.Second)) {
			continue
		}
		node.Health = domain.Health{State: "healthy", ObservedAt: at, StaleAfterSeconds: hostReportTTL, Source: "host-agent"}
		node.Freshness = node.Health.Freshness(now)
		node.Metrics = domain.Metrics{CPUPercent: &sample.CPUPercent, RAMPercent: &sample.RAMPercent,
			DiskPercent: &sample.DiskPercent, VCPU: &sample.VCPU, RAMGB: &sample.RAMGB}
		node.Capabilities = append(node.Capabilities, "monitor")
	}
	return rows.Err()
}

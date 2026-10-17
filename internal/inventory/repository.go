package inventory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

// Repository persists normalized inventory identity and the latest observation.
// The demo provider is reconciled at startup; API reads use this store.
type Repository struct{ DB *sql.DB }

func (r Repository) UpsertSnapshot(ctx context.Context, snapshot domain.Snapshot) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, node := range snapshot.Nodes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_nodes(id,name,role,platform,updated_at) VALUES(?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name,role=excluded.role,platform=excluded.platform,updated_at=excluded.updated_at`,
			node.ID, node.Name, node.Role, node.Platform, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("store node %s: %w", node.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO node_observations(node_id,health_state,observed_at,stale_after_seconds,source,cpu_percent,ram_percent,vcpu,ram_gb,simulated)
			VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(node_id) DO UPDATE SET health_state=excluded.health_state,observed_at=excluded.observed_at,
			stale_after_seconds=excluded.stale_after_seconds,source=excluded.source,cpu_percent=excluded.cpu_percent,ram_percent=excluded.ram_percent,
			vcpu=excluded.vcpu,ram_gb=excluded.ram_gb,simulated=excluded.simulated`, node.ID, node.Health.State, node.Health.ObservedAt.Format(time.RFC3339Nano),
			node.Health.StaleAfterSeconds, node.Health.Source, node.Metrics.CPUPercent, node.Metrics.RAMPercent, node.Metrics.VCPU, node.Metrics.RAMGB, node.Simulated); err != nil {
			return fmt.Errorf("store node observation %s: %w", node.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM node_capabilities WHERE node_id=?`, node.ID); err != nil {
			return err
		}
		for _, capability := range node.Capabilities {
			if _, err := tx.ExecContext(ctx, `INSERT INTO node_capabilities(node_id,capability) VALUES(?,?)`, node.ID, capability); err != nil {
				return err
			}
		}
	}
	for _, service := range snapshot.Services {
		if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_services(id,name,updated_at) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,updated_at=excluded.updated_at`, service.ID, service.Name, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO service_observations(service_id,health_state,observed_at,stale_after_seconds,source,simulated) VALUES(?,?,?,?,?,?)
			ON CONFLICT(service_id) DO UPDATE SET health_state=excluded.health_state,observed_at=excluded.observed_at,stale_after_seconds=excluded.stale_after_seconds,source=excluded.source,simulated=excluded.simulated`,
			service.ID, service.Health.State, service.Health.ObservedAt.Format(time.RFC3339Nano), service.Health.StaleAfterSeconds, service.Health.Source, service.Simulated); err != nil {
			return err
		}
	}
	for _, event := range snapshot.Activity {
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(id,title,detail,state,occurred_at,simulated) VALUES(?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET title=excluded.title,detail=excluded.detail,state=excluded.state,occurred_at=excluded.occurred_at,simulated=excluded.simulated`,
			event.ID, event.Title, event.Detail, event.State, event.OccurredAt.Format(time.RFC3339Nano), event.Simulated); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r Repository) Snapshot(ctx context.Context, now time.Time) (domain.Snapshot, error) {
	result := domain.Snapshot{Nodes: []domain.Node{}, Services: []domain.Service{}, Activity: []domain.Activity{}}
	rows, err := r.DB.QueryContext(ctx, `SELECT n.id,n.name,n.role,n.platform,o.health_state,o.observed_at,o.stale_after_seconds,o.source,
		o.cpu_percent,o.ram_percent,o.vcpu,o.ram_gb,o.simulated FROM inventory_nodes n JOIN node_observations o ON o.node_id=n.id ORDER BY n.id`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var n domain.Node
		var observed string
		var cpu, ram sql.NullFloat64
		var vcpu, ramGB sql.NullInt64
		if err := rows.Scan(&n.ID, &n.Name, &n.Role, &n.Platform, &n.Health.State, &observed, &n.Health.StaleAfterSeconds, &n.Health.Source, &cpu, &ram, &vcpu, &ramGB, &n.Simulated); err != nil {
			rows.Close()
			return result, err
		}
		if n.Health.ObservedAt, err = time.Parse(time.RFC3339Nano, observed); err != nil {
			rows.Close()
			return result, err
		}
		if cpu.Valid {
			n.Metrics.CPUPercent = &cpu.Float64
		}
		if ram.Valid {
			n.Metrics.RAMPercent = &ram.Float64
		}
		if vcpu.Valid {
			value := int(vcpu.Int64)
			n.Metrics.VCPU = &value
		}
		if ramGB.Valid {
			value := int(ramGB.Int64)
			n.Metrics.RAMGB = &value
		}
		n.Freshness = n.Health.Freshness(now)
		n.Capabilities = []string{}
		result.Nodes = append(result.Nodes, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	capRows, err := r.DB.QueryContext(ctx, `SELECT node_id,capability FROM node_capabilities ORDER BY node_id,capability`)
	if err != nil {
		return result, err
	}
	capabilities := map[string][]string{}
	for capRows.Next() {
		var id, cap string
		if err := capRows.Scan(&id, &cap); err != nil {
			capRows.Close()
			return result, err
		}
		capabilities[id] = append(capabilities[id], cap)
	}
	if err := capRows.Err(); err != nil {
		capRows.Close()
		return result, err
	}
	capRows.Close()
	for i := range result.Nodes {
		if values, ok := capabilities[result.Nodes[i].ID]; ok {
			result.Nodes[i].Capabilities = values
		}
	}
	rows, err = r.DB.QueryContext(ctx, `SELECT s.id,s.name,o.health_state,o.observed_at,o.stale_after_seconds,o.source,o.simulated FROM inventory_services s JOIN service_observations o ON o.service_id=s.id ORDER BY s.id`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var s domain.Service
		var observed string
		if err := rows.Scan(&s.ID, &s.Name, &s.Health.State, &observed, &s.Health.StaleAfterSeconds, &s.Health.Source, &s.Simulated); err != nil {
			rows.Close()
			return result, err
		}
		if s.Health.ObservedAt, err = time.Parse(time.RFC3339Nano, observed); err != nil {
			rows.Close()
			return result, err
		}
		s.Freshness = s.Health.Freshness(now)
		result.Services = append(result.Services, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	rows, err = r.DB.QueryContext(ctx, `SELECT id,title,detail,state,occurred_at,simulated FROM activity_events ORDER BY occurred_at DESC,id LIMIT 50`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var a domain.Activity
		var occurred string
		if err := rows.Scan(&a.ID, &a.Title, &a.Detail, &a.State, &occurred, &a.Simulated); err != nil {
			rows.Close()
			return result, err
		}
		if a.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred); err != nil {
			rows.Close()
			return result, err
		}
		result.Activity = append(result.Activity, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	return result, nil
}

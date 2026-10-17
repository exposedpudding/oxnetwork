package domain

import "time"

type Health struct {
	State             string    `json:"state"`
	ObservedAt        time.Time `json:"observed_at"`
	StaleAfterSeconds int       `json:"stale_after_seconds"`
	Source            string    `json:"source"`
}

func (h Health) Freshness(now time.Time) string {
	if h.ObservedAt.IsZero() {
		return "unknown"
	}
	if now.Sub(h.ObservedAt) > time.Duration(h.StaleAfterSeconds)*time.Second {
		return "stale"
	}
	return "fresh"
}

type Metrics struct {
	CPUPercent *float64 `json:"cpu_percent,omitempty"`
	RAMPercent *float64 `json:"ram_percent,omitempty"`
	VCPU       *int     `json:"vcpu,omitempty"`
	RAMGB      *int     `json:"ram_gb,omitempty"`
}

type Node struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Platform     string   `json:"platform"`
	Health       Health   `json:"health"`
	Freshness    string   `json:"freshness"`
	Capabilities []string `json:"capabilities"`
	Metrics      Metrics  `json:"metrics"`
	Simulated    bool     `json:"simulated"`
}

type Service struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Health    Health `json:"health"`
	Freshness string `json:"freshness"`
	Simulated bool   `json:"simulated"`
}

type Activity struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail"`
	State      string    `json:"state"`
	OccurredAt time.Time `json:"occurred_at"`
	Simulated  bool      `json:"simulated"`
}

// Connection is an observed relationship between graph resources. Source and
// Target use node:<id> or service:<id> so IDs cannot collide across kinds.
type Connection struct {
	ID                string    `json:"id"`
	Source            string    `json:"source"`
	Target            string    `json:"target"`
	Kind              string    `json:"kind"`
	ObservedAt        time.Time `json:"observed_at"`
	StaleAfterSeconds int       `json:"stale_after_seconds"`
	SourceSystem      string    `json:"source_system"`
	Freshness         string    `json:"freshness"`
	Simulated         bool      `json:"simulated"`
}

func (c Connection) ObservedFreshness(now time.Time) string {
	return Health{ObservedAt: c.ObservedAt, StaleAfterSeconds: c.StaleAfterSeconds}.Freshness(now)
}

type Snapshot struct {
	Nodes       []Node
	Services    []Service
	Activity    []Activity
	Connections []Connection
}

type Source interface{ Snapshot(now time.Time) Snapshot }

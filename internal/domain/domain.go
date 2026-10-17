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

type Snapshot struct {
	Nodes    []Node
	Services []Service
	Activity []Activity
}

type Source interface{ Snapshot(now time.Time) Snapshot }

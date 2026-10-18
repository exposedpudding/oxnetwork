package compute

import "time"

// Provider-neutral types for compute sources such as OpenCode servers (ADR-017).
// A provider reports where agent work can run and what is running now. It never
// reports prompt text, messages, file contents or credentials.

// ProviderState is the reachability of one compute provider instance.
const (
	ProviderOnline  = "online"
	ProviderOffline = "offline"
)

// JobState values. Sessions of an interactive agent server are not batch jobs, so
// a session that is not running is "idle" rather than succeeded or failed.
const (
	JobRunning = "running"
	JobIdle    = "idle"
)

// Provider describes one configured compute server and its latest reading.
type Provider struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Endpoint       string    `json:"endpoint"`
	NodeID         string    `json:"node_id,omitempty"`
	State          string    `json:"state"`
	Version        string    `json:"version,omitempty"`
	ActiveSessions int       `json:"active_sessions"`
	TotalSessions  int       `json:"total_sessions"`
	Detail         string    `json:"detail,omitempty"`
	ObservedAt     time.Time `json:"observed_at"`
	StaleAfterSecs int       `json:"stale_after_seconds"`
	Freshness      string    `json:"freshness"`
}

// Job is one unit of agent work reported by a provider.
type Job struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	State      string    `json:"state"`
	WorkerID   string    `json:"worker_id,omitempty"`
	ProviderID string    `json:"provider_id"`
	Source     string    `json:"source"`
	Agent      string    `json:"agent,omitempty"`
	Model      string    `json:"model,omitempty"`
	Cost       float64   `json:"cost"`
	Tokens     int64     `json:"tokens"`
	StartedAt  time.Time `json:"started_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Freshness  string    `json:"freshness"`
}

// State is a point-in-time copy of everything the providers last reported.
type State struct {
	Providers  []Provider `json:"providers"`
	Jobs       []Job      `json:"jobs"`
	ObservedAt time.Time  `json:"observed_at"`
}

// Source supplies compute state to the API. A nil Source means no provider is
// configured and the API reports no jobs.
type Source interface {
	State(now time.Time) State
}

// ActiveJobs counts running jobs on providers that are currently online.
func (s State) ActiveJobs() int {
	online := map[string]bool{}
	for _, provider := range s.Providers {
		if provider.State == ProviderOnline && provider.Freshness == "fresh" {
			online[provider.ID] = true
		}
	}
	count := 0
	for _, job := range s.Jobs {
		if job.State == JobRunning && online[job.ProviderID] {
			count++
		}
	}
	return count
}

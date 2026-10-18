package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/compute"
)

const (
	defaultInterval = 15 * time.Second
	staleRounds     = 3
	maxJobsPerHost  = 50 // per project directory
	maxProjects     = 20
	// maxJobsPerProvider bounds what one server can contribute: running sessions
	// first, then the most recently active.
	maxJobsPerProvider = 100
)

// Poller periodically reads every configured server and serves the latest result
// through compute.Source. It holds state in memory only: sessions are live
// activity, not inventory, so nothing is persisted.
type Poller struct {
	Clients  []Client
	Interval time.Duration
	Now      func() time.Time
	Log      *slog.Logger

	mu       sync.RWMutex
	readings map[string]*reading
}

type reading struct {
	provider compute.Provider
	jobs     []compute.Job
	jobsAt   time.Time // when jobs were last read successfully
}

func (p *Poller) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func (p *Poller) interval() time.Duration {
	if p.Interval > 0 {
		return p.Interval
	}
	return defaultInterval
}

func (p *Poller) staleAfter() time.Duration { return p.interval() * staleRounds }

// Run polls immediately and then every interval until ctx ends.
func (p *Poller) Run(ctx context.Context) {
	if len(p.Clients) == 0 {
		return
	}
	ticker := time.NewTicker(p.interval())
	defer ticker.Stop()
	for {
		p.PollAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// PollAll reads every server once, concurrently.
func (p *Poller) PollAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, client := range p.Clients {
		wg.Add(1)
		go func(client Client) {
			defer wg.Done()
			p.poll(ctx, client)
		}(client)
	}
	wg.Wait()
}

func endpoint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
}

func (p *Poller) poll(ctx context.Context, client Client) {
	server := client.Server
	at := p.now()
	name := server.Name
	if name == "" {
		name = server.ID
	}
	provider := compute.Provider{
		ID: server.ID, Name: name, Kind: "opencode", Endpoint: endpoint(server.URL), NodeID: server.NodeID,
		State: compute.ProviderOffline, ObservedAt: at, StaleAfterSecs: int(p.staleAfter().Seconds()),
	}
	health, err := client.Health(ctx)
	switch {
	case err != nil:
		provider.Detail = err.Error()
	case !health.Healthy:
		provider.Version = health.Version
		provider.Detail = "the server reports it is not healthy"
	default:
		provider.State = compute.ProviderOnline
		provider.Version = health.Version
	}
	var jobs []compute.Job
	jobsOK := false
	if provider.State == compute.ProviderOnline {
		var problems []string
		jobs, problems, jobsOK = readProjects(ctx, client, at)
		if len(problems) > 0 {
			provider.Detail = strings.Join(problems, "; ")
		}
		if jobsOK {
			provider.TotalSessions = len(jobs)
			for _, job := range jobs {
				if job.State == compute.JobRunning {
					provider.ActiveSessions++
				}
			}
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.readings == nil {
		p.readings = map[string]*reading{}
	}
	current := p.readings[server.ID]
	if current == nil {
		current = &reading{}
		p.readings[server.ID] = current
	}
	if jobsOK {
		current.jobs, current.jobsAt = jobs, at
	} else if current.jobs != nil {
		// Keep the last good list so the page can show it as stale instead of empty.
		provider.TotalSessions = len(current.jobs)
	}
	current.provider = provider
}

// readProjects visits the default project and every project directory the server
// reports, because OpenCode scopes the session list and status to a project. It
// returns the merged jobs, any problems worth showing, and whether at least one
// project was read (so the caller may replace its previous list).
func readProjects(ctx context.Context, client Client, at time.Time) ([]compute.Job, []string, bool) {
	server := client.Server
	var problems []string
	directories := []string{""} // "" is the server's default project
	projects, err := client.Projects(ctx)
	if err != nil {
		problems = append(problems, "project list unavailable: "+err.Error())
	} else {
		if len(projects) > maxProjects {
			problems = append(problems, fmt.Sprintf("only the first %d projects are read", maxProjects))
			projects = projects[:maxProjects]
		}
		directories = append(directories, projects...)
	}
	var jobs []compute.Job
	seen := map[string]bool{}
	read, failed := 0, 0
	for _, directory := range directories {
		sessions, serr := client.Sessions(ctx, directory, maxJobsPerHost)
		statuses, terr := client.Statuses(ctx, directory)
		if serr != nil || terr != nil {
			failed++
			continue
		}
		read++
		for _, job := range buildJobs(server.ID, server.NodeID, sessions, statuses, at) {
			if !seen[job.ID] {
				seen[job.ID] = true
				jobs = append(jobs, job)
			}
		}
	}
	if failed > 0 {
		if read == 0 {
			problems = append(problems, "session list unavailable")
		} else {
			problems = append(problems, fmt.Sprintf("%d of %d projects could not be read", failed, failed+read))
		}
	}
	if len(jobs) > maxJobsPerProvider {
		sortJobs(jobs)
		jobs = jobs[:maxJobsPerProvider]
	}
	return jobs, problems, read > 0
}

func sortJobs(jobs []compute.Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if (jobs[i].State == compute.JobRunning) != (jobs[j].State == compute.JobRunning) {
			return jobs[i].State == compute.JobRunning
		}
		return jobs[i].UpdatedAt.After(jobs[j].UpdatedAt)
	})
}

func buildJobs(serverID, nodeID string, sessions []Session, statuses map[string]Status, at time.Time) []compute.Job {
	jobs := make([]compute.Job, 0, len(sessions))
	for _, session := range sessions {
		if session.Archived {
			continue
		}
		state := compute.JobIdle
		if status := statuses[session.ID].Type; status == "busy" || status == "retry" {
			state = compute.JobRunning
		}
		jobs = append(jobs, compute.Job{
			ID: serverID + ":" + session.ID, Name: session.Title, State: state, WorkerID: nodeID,
			ProviderID: serverID, Source: "opencode", Agent: session.Agent, Model: session.Model,
			Cost: session.Cost, Tokens: session.Tokens, StartedAt: session.Created, UpdatedAt: session.Updated,
		})
	}
	sortJobs(jobs)
	return jobs
}

// State implements compute.Source. Freshness is computed for the given time, so a
// poller that has stopped cannot leave anything looking current.
func (p *Poller) State(now time.Time) compute.State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	state := compute.State{Providers: []compute.Provider{}, Jobs: []compute.Job{}}
	stale := p.staleAfter()
	for _, client := range p.Clients {
		r := p.readings[client.Server.ID]
		if r == nil {
			continue
		}
		provider := r.provider
		provider.Freshness = "fresh"
		if now.Sub(provider.ObservedAt) > stale {
			provider.Freshness = "stale"
		}
		if provider.ObservedAt.After(state.ObservedAt) {
			state.ObservedAt = provider.ObservedAt
		}
		state.Providers = append(state.Providers, provider)
		jobsFreshness := "fresh"
		if r.jobsAt.IsZero() || now.Sub(r.jobsAt) > stale {
			jobsFreshness = "stale"
		}
		for _, job := range r.jobs {
			job.Freshness = jobsFreshness
			state.Jobs = append(state.Jobs, job)
		}
	}
	sort.SliceStable(state.Providers, func(i, j int) bool { return state.Providers[i].ID < state.Providers[j].ID })
	sort.SliceStable(state.Jobs, func(i, j int) bool {
		if (state.Jobs[i].State == compute.JobRunning) != (state.Jobs[j].State == compute.JobRunning) {
			return state.Jobs[i].State == compute.JobRunning
		}
		return state.Jobs[i].UpdatedAt.After(state.Jobs[j].UpdatedAt)
	})
	return state
}

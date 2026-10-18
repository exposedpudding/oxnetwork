package opencode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/compute"
)

// dirData is what one project directory returns. OpenCode scopes both the session
// list and the status map to a project directory (confirmed against a real 1.19.3
// server), and the fake enforces the same: a request without ?directory= only sees
// the default project.
type dirData struct{ sessions, status string }

type fakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
	healthy  bool
	projects string
	dirs     map[string]dirData // "" is the default project
	failDirs map[string]bool
	user     string
	password string
}

func newFake(t *testing.T) *fakeServer {
	t.Helper()
	f := &fakeServer{healthy: true, user: "opencode", password: "s3cret", projects: `[{"id":"global","worktree":"/"}]`,
		dirs: map[string]dirData{"": {sessions: `[]`, status: `{}`}}, failDirs: map[string]bool{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		directory := r.URL.Query().Get("directory")
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		healthy, projects, data, fail := f.healthy, f.projects, f.dirs[directory], f.failDirs[directory]
		f.mu.Unlock()
		if user, pass, ok := r.BasicAuth(); !ok || user != f.user || pass != f.password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/global/health":
			if healthy {
				_, _ = w.Write([]byte(`{"healthy":true,"version":"1.19.3"}`))
			} else {
				_, _ = w.Write([]byte(`{"healthy":false,"version":"1.19.3"}`))
			}
		case "/project":
			_, _ = w.Write([]byte(projects))
		case "/session":
			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if data.sessions == "" {
				data.sessions = `[]`
			}
			_, _ = w.Write([]byte(data.sessions))
		case "/session/status":
			if data.status == "" {
				data.status = `{}`
			}
			_, _ = w.Write([]byte(data.status))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeServer) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeServer) set(fn func(f *fakeServer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeServer) client(id string) Client {
	return Client{Server: Server{ID: id, Name: "Server " + id, URL: f.URL, Username: f.user, Password: f.password, NodeID: "tailscale-1"}, Timeout: 500 * time.Millisecond}
}

const twoSessions = `[
 {"id":"ses_a","slug":"a","projectID":"p","directory":"/home/agent/workspaces/secret-project","title":"  Build the   report\nfor Q3  ","agent":"build","model":{"id":"qwen2.5:3b","providerID":"ollama","variant":"x"},"cost":0.5,"tokens":{"input":10,"output":20,"reasoning":1,"cache":{"read":2,"write":3}},"time":{"created":1792300000000,"updated":1792300600000}},
 {"id":"ses_b","title":"Older idle session","agent":"plan","model":{"id":"big-pickle","providerID":"opencode"},"cost":0,"tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}},"time":{"created":1792200000000,"updated":1792200100000}},
 {"id":"ses_old","title":"Archived","time":{"created":1,"updated":2,"archived":3}}
]`

const otherProjectSession = `[{"id":"ses_scratch","title":"Work in another project","agent":"build","model":{"id":"qwen2.5:3b","providerID":"ollama"},"cost":0,"tokens":{"input":5,"output":5,"reasoning":0,"cache":{"read":0,"write":0}},"time":{"created":1792301000000,"updated":1792301900000}}]`

func TestClientReadsHealthProjectsSessionsAndStatusesWithBasicAuth(t *testing.T) {
	f := newFake(t)
	f.projects = `[{"id":"global","worktree":"/"},{"id":"a","worktree":"/w/a"},{"id":"b","worktree":"/w/b"},{"id":"dup","worktree":"/w/a"},{"id":"empty","worktree":""}]`
	f.dirs[""] = dirData{sessions: twoSessions, status: `{"ses_a":{"type":"busy"},"ses_x":{"type":"retry","attempt":1,"message":"m","next":5}}`}
	c := f.client("s1")
	health, err := c.Health(context.Background())
	if err != nil || !health.Healthy || health.Version != "1.19.3" {
		t.Fatalf("health: %+v %v", health, err)
	}
	projects, err := c.Projects(context.Background())
	if err != nil || len(projects) != 2 || projects[0] != "/w/a" || projects[1] != "/w/b" {
		t.Fatalf("projects must drop the global pseudo-project, blanks and duplicates: %+v %v", projects, err)
	}
	sessions, err := c.Sessions(context.Background(), "", 10)
	if err != nil || len(sessions) != 3 {
		t.Fatalf("sessions: %+v %v", sessions, err)
	}
	a := sessions[0]
	if a.Title != "Build the report for Q3" || a.Model != "ollama/qwen2.5:3b" || a.Tokens != 36 || a.Agent != "build" || a.Updated.Unix() != 1792300600 {
		t.Fatalf("session a: %+v", a)
	}
	if !sessions[2].Archived {
		t.Fatal("archived flag lost")
	}
	statuses, err := c.Statuses(context.Background(), "")
	if err != nil || statuses["ses_a"].Type != "busy" || statuses["ses_x"].Type != "retry" {
		t.Fatalf("statuses: %+v %v", statuses, err)
	}
	// A non-default directory is empty here, proving the directory is really sent.
	if other, err := c.Sessions(context.Background(), "/w/a", 10); err != nil || len(other) != 0 {
		t.Fatalf("directory scoping: %+v %v", other, err)
	}
}

func TestClientErrorsAreSpecificAndNeverEchoResponses(t *testing.T) {
	f := newFake(t)
	bad := f.client("s1")
	bad.Server.Password = "wrong"
	if _, err := bad.Health(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong password: %v", err)
	}
	for name, handler := range map[string]http.HandlerFunc{
		"not json": func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>secret page</html>")) },
		"too large": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`"` + strings.Repeat("x", 3<<20) + `"`))
		},
		"500": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) },
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://127.0.0.1:1/", http.StatusFound)
		},
	} {
		server := httptest.NewServer(handler)
		c := Client{Server: Server{ID: "x", URL: server.URL}, Timeout: time.Second}
		_, err := c.Health(context.Background())
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret page") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer slow.Close()
	started := time.Now()
	_, err := Client{Server: Server{ID: "x", URL: slow.URL}, Timeout: 100 * time.Millisecond}.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no response within") || time.Since(started) > 2*time.Second {
		t.Fatalf("timeout: %v after %s", err, time.Since(started))
	}
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := closed.URL
	closed.Close()
	if _, err := (Client{Server: Server{ID: "x", URL: url}}).Health(context.Background()); err == nil || err.Error() != "connection refused" {
		t.Fatalf("refused: %v", err)
	}
}

func TestPollerSeesBusySessionsInEveryProjectAndOnlyMakesReadOnlyRequests(t *testing.T) {
	up, down := newFake(t), newFake(t)
	up.projects = `[{"id":"global","worktree":"/"},{"id":"scratch","worktree":"/w/scratch"},{"id":"empty","worktree":"/w/empty"}]`
	up.dirs[""] = dirData{sessions: twoSessions, status: `{}`}
	// Regression: a busy session in a non-default project is invisible to a reader that
	// only asks the default project, which is how the first version of the adapter failed.
	up.dirs["/w/scratch"] = dirData{sessions: otherProjectSession, status: `{"ses_scratch":{"type":"busy"}}`}
	down.healthy = false
	t0 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	clock := t0
	p := &Poller{Clients: []Client{down.client("b-down"), up.client("a-up")}, Interval: 10 * time.Second, Now: func() time.Time { return clock }}
	p.PollAll(context.Background())
	state := p.State(t0.Add(time.Second))
	if len(state.Providers) != 2 || state.Providers[0].ID != "a-up" || state.Providers[1].ID != "b-down" {
		t.Fatalf("providers: %+v", state.Providers)
	}
	upP, downP := state.Providers[0], state.Providers[1]
	if upP.State != compute.ProviderOnline || upP.Version != "1.19.3" || upP.ActiveSessions != 1 || upP.TotalSessions != 3 || upP.Freshness != "fresh" || upP.Kind != "opencode" || upP.StaleAfterSecs != 30 {
		t.Fatalf("up provider: %+v", upP)
	}
	if strings.Contains(upP.Endpoint, "@") || upP.NodeID != "tailscale-1" || upP.Detail != "" {
		t.Fatalf("endpoint/detail: %+v", upP)
	}
	if downP.State != compute.ProviderOffline || downP.Detail != "the server reports it is not healthy" {
		t.Fatalf("down provider: %+v", downP)
	}
	if len(state.Jobs) != 3 || state.Jobs[0].ID != "a-up:ses_scratch" || state.Jobs[0].State != compute.JobRunning || state.Jobs[1].State != compute.JobIdle || state.Jobs[2].State != compute.JobIdle {
		t.Fatalf("jobs (running first, archived hidden, all projects merged): %+v", state.Jobs)
	}
	if state.Jobs[1].Name != "Build the report for Q3" || state.Jobs[1].Tokens != 36 || state.Jobs[1].WorkerID != "tailscale-1" || state.Jobs[1].Source != "opencode" {
		t.Fatalf("job: %+v", state.Jobs[1])
	}
	if state.ActiveJobs() != 1 {
		t.Fatalf("active jobs: %d", state.ActiveJobs())
	}
	allowed := map[string]bool{"GET /global/health": true, "GET /project": true, "GET /session": true, "GET /session/status": true}
	for name, f := range map[string]*fakeServer{"up": up, "down": down} {
		for _, request := range f.recorded() {
			if !allowed[request] {
				t.Fatalf("%s server received an unexpected request: %s", name, request)
			}
		}
	}
	if got := down.recorded(); len(got) != 1 {
		t.Fatalf("an unhealthy server should only be asked for health: %v", got)
	}
	// health + project list + (session, status) for the default project and two others.
	if got := up.recorded(); len(got) != 2+2*3 {
		t.Fatalf("expected one pair of reads per project, got %d requests: %v", len(got), got)
	}
}

func TestSessionsAppearingInTwoProjectsAreCountedOnce(t *testing.T) {
	f := newFake(t)
	f.projects = `[{"id":"a","worktree":"/w/a"}]`
	f.dirs[""] = dirData{sessions: otherProjectSession}
	f.dirs["/w/a"] = dirData{sessions: otherProjectSession}
	p := &Poller{Clients: []Client{f.client("s")}, Interval: time.Second}
	p.PollAll(context.Background())
	if state := p.State(time.Now()); len(state.Jobs) != 1 || state.Providers[0].TotalSessions != 1 {
		t.Fatalf("duplicates: %+v", state)
	}
}

func TestOneUnreadableProjectDoesNotHideTheOthers(t *testing.T) {
	f := newFake(t)
	f.projects = `[{"id":"a","worktree":"/w/a"},{"id":"bad","worktree":"/w/bad"}]`
	f.dirs[""] = dirData{sessions: twoSessions}
	f.dirs["/w/a"] = dirData{sessions: otherProjectSession, status: `{"ses_scratch":{"type":"busy"}}`}
	f.failDirs["/w/bad"] = true
	p := &Poller{Clients: []Client{f.client("s")}, Interval: time.Second}
	p.PollAll(context.Background())
	state := p.State(time.Now())
	if state.Providers[0].State != compute.ProviderOnline || state.Providers[0].Detail != "1 of 3 projects could not be read" || len(state.Jobs) != 3 || state.Providers[0].ActiveSessions != 1 {
		t.Fatalf("state: %+v", state)
	}
}

func TestProjectListFailureFallsBackToTheDefaultProject(t *testing.T) {
	f := newFake(t)
	f.projects = `<html>nope</html>`
	f.dirs[""] = dirData{sessions: twoSessions, status: `{"ses_a":{"type":"busy"}}`}
	p := &Poller{Clients: []Client{f.client("s")}, Interval: time.Second}
	p.PollAll(context.Background())
	state := p.State(time.Now())
	if !strings.HasPrefix(state.Providers[0].Detail, "project list unavailable") || len(state.Jobs) != 2 || state.Providers[0].ActiveSessions != 1 {
		t.Fatalf("state: %+v", state)
	}
}

func TestOfflineServerKeepsLastJobsAsStaleAndStopsCountingThemActive(t *testing.T) {
	f := newFake(t)
	f.dirs[""] = dirData{sessions: twoSessions, status: `{"ses_a":{"type":"busy"}}`}
	t0 := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	clock := t0
	p := &Poller{Clients: []Client{f.client("s")}, Interval: 10 * time.Second, Now: func() time.Time { return clock }}
	p.PollAll(context.Background())
	if p.State(t0).ActiveJobs() != 1 {
		t.Fatal("expected one active job while online")
	}
	f.set(func(f *fakeServer) { f.healthy = false })
	clock = t0.Add(10 * time.Second)
	p.PollAll(context.Background())
	state := p.State(clock)
	if state.Providers[0].State != compute.ProviderOffline || len(state.Jobs) != 2 || state.Jobs[0].Freshness != "fresh" {
		t.Fatalf("just after going offline: %+v", state)
	}
	if state.ActiveJobs() != 0 {
		t.Fatal("jobs on an offline provider must not count as active")
	}
	later := p.State(t0.Add(45 * time.Second))
	if later.Jobs[0].Freshness != "stale" || later.Providers[0].Freshness != "stale" {
		t.Fatalf("after three missed rounds: %+v", later)
	}
}

func TestEveryProjectUnreadableKeepsThePreviousListAndSaysSo(t *testing.T) {
	f := newFake(t)
	f.dirs[""] = dirData{sessions: twoSessions, status: `{}`}
	clock := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	p := &Poller{Clients: []Client{f.client("s")}, Interval: 10 * time.Second, Now: func() time.Time { return clock }}
	p.PollAll(context.Background())
	f.set(func(f *fakeServer) { f.failDirs[""] = true })
	clock = clock.Add(10 * time.Second)
	p.PollAll(context.Background())
	state := p.State(clock)
	if state.Providers[0].State != compute.ProviderOnline || state.Providers[0].Detail != "session list unavailable" || len(state.Jobs) != 2 || state.Providers[0].TotalSessions != 2 {
		t.Fatalf("state: %+v", state)
	}
}

func TestNoClientsRunReturnsAndTitlesAreBounded(t *testing.T) {
	done := make(chan struct{})
	go func() { (&Poller{}).Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run with no servers should return")
	}
	long := truncate(strings.Repeat("é", 200), 80)
	if len([]rune(long)) != 80 || !strings.HasSuffix(long, "…") {
		t.Fatalf("truncate: %d runes", len([]rune(long)))
	}
}

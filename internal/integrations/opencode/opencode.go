// Package opencode reads worker health and session activity from OpenCode servers
// (`opencode serve`) so the Compute page can show real agent work (ADR-017).
//
// It is strictly read-only. It calls four stable endpoints, GET /global/health,
// GET /project, GET /session and GET /session/status (the last two once per project
// directory, because OpenCode scopes them to a project), and keeps only metadata: title, agent,
// model, token and cost totals, and timestamps. It never requests messages, file
// contents, permissions or configuration, and it never sends a prompt. Requests use
// HTTP basic authentication when a password is configured, never follow redirects,
// verify TLS, ignore proxy environment variables, and bound response size and time.
package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxResponseBytes = 2 << 20
	maxTitleRunes    = 80
	requestTimeout   = 5 * time.Second
)

// Server is one configured OpenCode server.
type Server struct {
	ID       string
	Name     string
	URL      string
	Username string
	Password string
	NodeID   string
}

// Client talks to one server.
type Client struct {
	Server Server
	HTTP   *http.Client
	// Timeout bounds each request; zero means the default of 5 seconds.
	Timeout time.Duration
}

func (c Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return requestTimeout
}

// NewHTTPClient returns the hardened client used in production.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: requestTimeout,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Health is the server's self-report.
type Health struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// Session is the metadata kept for one OpenCode session.
type Session struct {
	ID       string
	Title    string
	Agent    string
	Model    string
	Cost     float64
	Tokens   int64
	Created  time.Time
	Updated  time.Time
	Archived bool
}

// Status is a session's activity: "idle", "busy" or "retry".
type Status struct {
	Type string `json:"type"`
}

// ErrUnauthorized is returned for HTTP 401 and 403 so callers can say the
// credentials are wrong without echoing anything from the response.
var ErrUnauthorized = errors.New("the server rejected the credentials")

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return NewHTTPClient()
}

func (c Client) get(ctx context.Context, path string, query url.Values, into any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	target := strings.TrimRight(c.Server.URL, "/") + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return errors.New("invalid server URL")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "oxnet.work-opencode/1")
	if c.Server.Password != "" {
		username := c.Server.Username
		if username == "" {
			username = "opencode"
		}
		req.SetBasicAuth(username, c.Server.Password)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return describe(ctx, err, c.timeout())
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return errors.New("response could not be read")
	}
	if len(body) > maxResponseBytes {
		return errors.New("response too large")
	}
	if err := json.Unmarshal(body, into); err != nil {
		return errors.New("response was not the expected JSON")
	}
	return nil
}

func describe(ctx context.Context, err error, timeout time.Duration) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded:
		return fmt.Errorf("no response within %s", timeout)
	case strings.Contains(err.Error(), "connection refused"):
		return errors.New("connection refused")
	case strings.Contains(err.Error(), "no such host"):
		return errors.New("name did not resolve")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("no response within %s", timeout)
	}
	return errors.New("request failed")
}

// Health calls GET /global/health.
func (c Client) Health(ctx context.Context) (Health, error) {
	var health Health
	err := c.get(ctx, "/global/health", nil, &health)
	return health, err
}

type rawSession struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Agent string `json:"agent"`
	Model struct {
		ID         string `json:"id"`
		ProviderID string `json:"providerID"`
	} `json:"model"`
	Cost   float64 `json:"cost"`
	Tokens struct {
		Input     int64 `json:"input"`
		Output    int64 `json:"output"`
		Reasoning int64 `json:"reasoning"`
		Cache     struct {
			Read  int64 `json:"read"`
			Write int64 `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
	Time struct {
		Created  int64 `json:"created"`
		Updated  int64 `json:"updated"`
		Archived int64 `json:"archived"`
	} `json:"time"`
}

// Projects calls GET /project and returns the working directory of every project
// the server knows, without the global "/" pseudo-project. OpenCode scopes both the
// session list and session status to a project directory, so a reader that does
// not visit every project reports activity as idle while agents are working.
func (c Client) Projects(ctx context.Context) ([]string, error) {
	var raw []struct {
		Worktree string `json:"worktree"`
	}
	if err := c.get(ctx, "/project", nil, &raw); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	directories := make([]string, 0, len(raw))
	for _, project := range raw {
		if project.Worktree == "" || project.Worktree == "/" || seen[project.Worktree] {
			continue
		}
		seen[project.Worktree] = true
		directories = append(directories, project.Worktree)
	}
	return directories, nil
}

func directoryQuery(directory string, query url.Values) url.Values {
	if directory != "" {
		query.Set("directory", directory)
	}
	return query
}

// Sessions calls GET /session for root sessions in one project directory (empty
// means the server's default project), newest first, up to limit.
func (c Client) Sessions(ctx context.Context, directory string, limit int) ([]Session, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var raw []rawSession
	query := directoryQuery(directory, url.Values{"roots": {"true"}, "limit": {fmt.Sprint(limit)}})
	if err := c.get(ctx, "/session", query, &raw); err != nil {
		return nil, err
	}
	sessions := make([]Session, 0, len(raw))
	for _, item := range raw {
		if item.ID == "" {
			continue
		}
		model := item.Model.ID
		if item.Model.ProviderID != "" && model != "" {
			model = item.Model.ProviderID + "/" + model
		}
		sessions = append(sessions, Session{
			ID:       item.ID,
			Title:    truncate(item.Title, maxTitleRunes),
			Agent:    item.Agent,
			Model:    model,
			Cost:     item.Cost,
			Tokens:   item.Tokens.Input + item.Tokens.Output + item.Tokens.Reasoning + item.Tokens.Cache.Read + item.Tokens.Cache.Write,
			Created:  millis(item.Time.Created),
			Updated:  millis(item.Time.Updated),
			Archived: item.Time.Archived > 0,
		})
	}
	return sessions, nil
}

// Statuses calls GET /session/status for one project directory (empty means the
// default project). Idle sessions are usually absent from the map.
func (c Client) Statuses(ctx context.Context, directory string) (map[string]Status, error) {
	statuses := map[string]Status{}
	if err := c.get(ctx, "/session/status", directoryQuery(directory, url.Values{}), &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func millis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func truncate(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}

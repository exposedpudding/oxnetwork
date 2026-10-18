package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
	"github.com/exposedpudding/oxnetwork/internal/workflow"
)

type apiHarness struct {
	t       *testing.T
	handler http.Handler
	server  *Server
}

func newAPIHarness(t *testing.T, mutations bool) *apiHarness {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	now := time.Date(2026, 10, 18, 12, 7, 0, 0, time.UTC)
	repo := inventory.Repository{DB: database}
	if err := repo.UpsertSnapshot(context.Background(), demo.Provider{}.Snapshot(now)); err != nil {
		t.Fatal(err)
	}
	store := workflow.Store{DB: database, Now: func() time.Time { return now }}
	if err := workflow.SeedDemo(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	hub := &workflow.Hub{}
	runner := &workflow.Runner{Store: store, Inventory: repo, Hub: hub}
	server := &Server{DB: database, Repo: repo, Runner: runner, Hub: hub, MutationsAllowed: mutations, Now: func() time.Time { return now }}
	return &apiHarness{t: t, handler: server.Handler(), server: server}
}

func (h *apiHarness) do(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	h.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, reader)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return value
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	body := decode[struct {
		Error struct {
			Code    string            `json:"code"`
			Details map[string]string `json:"details"`
		} `json:"error"`
	}](t, w)
	return body.Error.Code
}

func validBody() map[string]any {
	return map[string]any{
		"name":    "Gateway check",
		"trigger": map[string]any{"type": "cron", "expression": "0 * * * *"},
		"steps": []map[string]any{
			{"id": "s1", "type": "action", "name": "Check gateway", "action_type": "check_node_health", "target_id": "demo-gateway"},
			{"id": "s2", "type": "notify", "name": "Note", "channel": "activity", "message": "done"},
		},
		"on_success": map[string]any{"notify_channel": "activity"},
		"on_failure": map[string]any{"behavior": "continue"},
	}
}

func TestWorkflowReadEndpoints(t *testing.T) {
	h := newAPIHarness(t, true)
	list := decode[struct {
		Items []workflow.Workflow `json:"items"`
	}](t, h.do("GET", "/api/v1/workflows", nil, nil))
	if len(list.Items) != 3 || !list.Items[0].Simulated || list.Items[0].Trigger.Type != "cron" {
		t.Fatalf("workflows: %+v", list.Items)
	}
	options := decode[workflow.Options](t, h.do("GET", "/api/v1/workflows/options", nil, nil))
	if len(options.ActionTypes) != 2 || len(options.Targets) != 3 || len(options.Channels) != 1 || len(options.PlannedActions) != 4 {
		t.Fatalf("options: %+v", options)
	}
	if got := h.do("GET", "/api/v1/workflows/"+list.Items[0].ID, nil, nil).Code; got != 200 {
		t.Fatalf("get: %d", got)
	}
	if got := h.do("GET", "/api/v1/workflows/missing", nil, nil).Code; got != 404 {
		t.Fatalf("missing: %d", got)
	}
	summary := decode[workflow.Summary](t, h.do("GET", "/api/v1/automations/summary", nil, nil))
	if summary.Workflows != 3 || summary.Enabled != 2 {
		t.Fatalf("summary: %+v", summary)
	}
	system := decode[map[string]any](t, h.do("GET", "/api/v1/system", nil, nil))
	if system["mutations_enabled"] != true {
		t.Fatalf("system: %+v", system)
	}
	// Raw JSON must carry for_seconds for health triggers (the editor reads it).
	if raw := h.do("GET", "/api/v1/workflows", nil, nil).Body.String(); !strings.Contains(raw, `"for_seconds":60`) {
		t.Fatalf("health trigger JSON missing for_seconds: %s", raw)
	}
}

func TestWorkflowCreateUpdateEnableDisableAndAudit(t *testing.T) {
	h := newAPIHarness(t, true)
	created := h.do("POST", "/api/v1/workflows", validBody(), nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	wf := decode[workflow.Workflow](t, created)
	if !strings.HasPrefix(wf.ID, "wf_") || !wf.Enabled || wf.Simulated || wf.NextRunAt == nil || len(wf.Steps) != 2 {
		t.Fatalf("created: %+v", wf)
	}
	update := validBody()
	update["name"] = "Renamed check"
	update["steps"] = []map[string]any{{"id": "only", "type": "notify", "name": "Just a note", "channel": "activity", "message": "hello"}}
	updated := h.do("PATCH", "/api/v1/workflows/"+wf.ID, update, nil)
	if updated.Code != 200 {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	got := decode[workflow.Workflow](t, updated)
	if got.Name != "Renamed check" || len(got.Steps) != 1 || got.Steps[0].ActionType != "" || got.Steps[0].TargetID != "" {
		t.Fatalf("updated: %+v", got)
	}
	// Omitted fields keep their stored values.
	partial := decode[workflow.Workflow](t, h.do("PATCH", "/api/v1/workflows/"+wf.ID, map[string]any{"description": "kept steps"}, nil))
	if partial.Description != "kept steps" || len(partial.Steps) != 1 || partial.Name != "Renamed check" {
		t.Fatalf("partial update: %+v", partial)
	}
	if disabled := decode[workflow.Workflow](t, h.do("POST", "/api/v1/workflows/"+wf.ID+"/disable", nil, nil)); disabled.Enabled || disabled.NextRunAt != nil {
		t.Fatalf("disable: %+v", disabled)
	}
	if got := h.do("POST", "/api/v1/workflows/"+wf.ID+"/run", nil, nil); got.Code != http.StatusConflict || errorCode(t, got) != "WORKFLOW_DISABLED" {
		t.Fatalf("run while disabled: %d %s", got.Code, got.Body.String())
	}
	if enabled := decode[workflow.Workflow](t, h.do("POST", "/api/v1/workflows/"+wf.ID+"/enable", nil, nil)); !enabled.Enabled {
		t.Fatalf("enable: %+v", enabled)
	}
	audit := decode[struct {
		Items      []workflow.AuditEvent `json:"items"`
		NextCursor *string               `json:"next_cursor"`
	}](t, h.do("GET", "/api/v1/audit?target_id="+wf.ID+"&limit=3", nil, nil))
	if len(audit.Items) != 3 || audit.NextCursor == nil || audit.Items[0].Action != "workflow.enabled" || audit.Items[0].Actor != "local" {
		t.Fatalf("audit page 1: %+v cursor=%v", audit.Items, audit.NextCursor)
	}
	older := decode[struct {
		Items      []workflow.AuditEvent `json:"items"`
		NextCursor *string               `json:"next_cursor"`
	}](t, h.do("GET", "/api/v1/audit?target_id="+wf.ID+"&limit=3&cursor="+*audit.NextCursor, nil, nil))
	if len(older.Items) != 2 || older.NextCursor != nil || older.Items[1].Action != "workflow.created" {
		t.Fatalf("audit page 2: %+v", older.Items)
	}
	if got := h.do("GET", "/api/v1/audit?cursor=abc", nil, nil).Code; got != 400 {
		t.Fatalf("bad cursor: %d", got)
	}
}

func TestWorkflowValidationErrors(t *testing.T) {
	h := newAPIHarness(t, true)
	body := validBody()
	body["steps"] = []map[string]any{{"id": "s1", "type": "action", "name": "Wake", "action_type": "wake_node", "target_id": "demo-worker"}}
	w := h.do("POST", "/api/v1/workflows", body, nil)
	if w.Code != 400 || errorCode(t, w) != "ACTION_NOT_SUPPORTED" || !strings.Contains(w.Body.String(), "step.0.action") {
		t.Fatalf("planned action: %d %s", w.Code, w.Body.String())
	}
	bad := validBody()
	bad["name"] = ""
	bad["trigger"] = map[string]any{"type": "cron", "expression": "nope"}
	w = h.do("POST", "/api/v1/workflows", bad, nil)
	if w.Code != 400 || errorCode(t, w) != "INVALID_WORKFLOW" || !strings.Contains(w.Body.String(), `"name"`) || !strings.Contains(w.Body.String(), `"trigger"`) {
		t.Fatalf("invalid: %d %s", w.Code, w.Body.String())
	}
	unknown := validBody()
	unknown["shell"] = "rm -rf /"
	if w := h.do("POST", "/api/v1/workflows", unknown, nil); w.Code != 400 || errorCode(t, w) != "INVALID_REQUEST" {
		t.Fatalf("unknown field: %d %s", w.Code, w.Body.String())
	}
	if w := h.do("PATCH", "/api/v1/workflows/missing", validBody(), nil); w.Code != 404 {
		t.Fatalf("patch missing: %d", w.Code)
	}
}

func TestWorkflowMutationGuards(t *testing.T) {
	h := newAPIHarness(t, true)
	// Non-JSON content type.
	r := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader("name=x"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form post: %d", w.Code)
	}
	// Cross-origin browser request.
	if w := h.do("POST", "/api/v1/workflows", validBody(), map[string]string{"Origin": "https://evil.example"}); w.Code != 403 || errorCode(t, w) != "CROSS_ORIGIN" {
		t.Fatalf("cross origin: %d %s", w.Code, w.Body.String())
	}
	// Same-origin requests pass (httptest requests use host example.com).
	if w := h.do("POST", "/api/v1/workflows", validBody(), map[string]string{"Origin": "http://example.com"}); w.Code != http.StatusCreated {
		t.Fatalf("same origin: %d %s", w.Code, w.Body.String())
	}
	// Server where mutations are not permitted.
	locked := newAPIHarness(t, false)
	for _, call := range []struct{ method, path string }{{"POST", "/api/v1/workflows"}, {"PATCH", "/api/v1/workflows/x"}, {"POST", "/api/v1/workflows/x/run"}, {"POST", "/api/v1/workflows/x/enable"}, {"POST", "/api/v1/workflows/x/disable"}} {
		if w := locked.do(call.method, call.path, validBody(), nil); w.Code != 403 || errorCode(t, w) != "MUTATIONS_DISABLED" {
			t.Fatalf("%s %s: %d %s", call.method, call.path, w.Code, w.Body.String())
		}
	}
	if got := locked.do("GET", "/api/v1/workflows", nil, nil).Code; got != 200 {
		t.Fatalf("reads must still work: %d", got)
	}
	system := decode[map[string]any](t, locked.do("GET", "/api/v1/system", nil, nil))
	if system["mutations_enabled"] != false {
		t.Fatalf("system: %+v", system)
	}
}

func TestWorkflowRunEndToEnd(t *testing.T) {
	h := newAPIHarness(t, true)
	list := decode[struct {
		Items []workflow.Workflow `json:"items"`
	}](t, h.do("GET", "/api/v1/workflows", nil, nil))
	var onDemand workflow.Workflow
	for _, wf := range list.Items {
		if wf.Name == "Check worker on demand" {
			onDemand = wf
		}
	}
	accepted := h.do("POST", "/api/v1/workflows/"+onDemand.ID+"/run", nil, nil)
	if accepted.Code != http.StatusAccepted || !strings.HasPrefix(accepted.Header().Get("Location"), "/api/v1/workflow-runs/run_") {
		t.Fatalf("run: %d %s", accepted.Code, accepted.Body.String())
	}
	run := decode[workflow.Run](t, accepted)
	h.server.Runner.Wait()
	finished := decode[workflow.Run](t, h.do("GET", "/api/v1/workflow-runs/"+run.ID, nil, nil))
	if finished.State != workflow.RunFailed || len(finished.Steps) != 3 || finished.Steps[0].State != workflow.StepFailed || finished.Trigger != "manual" || finished.Actor != "local" {
		t.Fatalf("finished: %+v", finished)
	}
	runs := decode[struct {
		Items []workflow.Run `json:"items"`
	}](t, h.do("GET", "/api/v1/workflows/"+onDemand.ID+"/runs", nil, nil))
	recent := decode[struct {
		Items []workflow.Run `json:"items"`
	}](t, h.do("GET", "/api/v1/workflow-runs?limit=5", nil, nil))
	if len(runs.Items) != 1 || len(recent.Items) != 1 || runs.Items[0].ID != run.ID || len(runs.Items[0].Steps) != 0 {
		t.Fatalf("run lists: %+v %+v", runs.Items, recent.Items)
	}
	// A second run while one is active is rejected.
	other := h.server.Runner.Store
	wf, _ := other.GetWorkflow(context.Background(), onDemand.ID)
	if _, err := other.CreateRun(context.Background(), wf, "manual", "t"); err != nil {
		t.Fatal(err)
	}
	if w := h.do("POST", "/api/v1/workflows/"+onDemand.ID+"/run", nil, nil); w.Code != 409 || errorCode(t, w) != "RUN_IN_PROGRESS" {
		t.Fatalf("busy: %d %s", w.Code, w.Body.String())
	}
	if w := h.do("GET", "/api/v1/workflow-runs/run_missing", nil, nil); w.Code != 404 {
		t.Fatalf("missing run: %d", w.Code)
	}
}

func TestAutomationEndpointsUnavailableWithoutRunner(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "plain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := (&Server{DB: database, Repo: inventory.Repository{DB: database}}).Handler()
	for _, path := range []string{"/api/v1/workflows", "/api/v1/automations/summary", "/api/v1/audit"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

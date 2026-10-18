package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/exposedpudding/oxnetwork/internal/auth"
	"github.com/exposedpudding/oxnetwork/internal/workflow"
)

const maxWorkflowBody = 64 << 10

func (s *Server) registerWorkflowRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/automations/summary", s.automationSummary)
	mux.HandleFunc("GET /api/v1/workflows", s.listWorkflows)
	mux.HandleFunc("POST /api/v1/workflows", s.createWorkflow)
	mux.HandleFunc("GET /api/v1/workflows/options", s.workflowOptions)
	mux.HandleFunc("GET /api/v1/workflows/{id}", s.getWorkflow)
	mux.HandleFunc("PATCH /api/v1/workflows/{id}", s.updateWorkflow)
	mux.HandleFunc("POST /api/v1/workflows/{id}/run", s.runWorkflow)
	mux.HandleFunc("POST /api/v1/workflows/{id}/enable", s.setWorkflowEnabled(true))
	mux.HandleFunc("POST /api/v1/workflows/{id}/disable", s.setWorkflowEnabled(false))
	mux.HandleFunc("GET /api/v1/workflows/{id}/runs", s.listWorkflowRuns)
	mux.HandleFunc("GET /api/v1/workflow-runs", s.listRecentRuns)
	mux.HandleFunc("GET /api/v1/workflow-runs/{id}", s.getWorkflowRun)
	mux.HandleFunc("GET /api/v1/audit", s.listAudit)
}

func (s *Server) automationsReady(w http.ResponseWriter, r *http.Request) bool {
	if s.Runner == nil {
		writeError(w, r, http.StatusServiceUnavailable, "AUTOMATIONS_UNAVAILABLE", "Automations are not enabled on this server")
		return false
	}
	return true
}

// mutation gates every state-changing automation endpoint. It returns the
// actor name recorded in the audit log.
func (s *Server) mutation(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !s.automationsReady(w, r) {
		return "", false
	}
	if !s.MutationsAllowed {
		writeError(w, r, http.StatusForbidden, "MUTATIONS_DISABLED", "Changes are disabled until sign-in is configured or the server is bound to loopback")
		return "", false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != r.Host {
			writeError(w, r, http.StatusForbidden, "CROSS_ORIGIN", "Cross-origin changes are not allowed")
			return "", false
		}
	}
	actor := "local"
	if identity, ok := auth.FromContext(r.Context()); ok {
		actor = identity.Email
		if actor == "" {
			actor = identity.Subject
		}
	}
	return actor, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWorkflowBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Request body is not valid JSON for this endpoint")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Request body must contain a single JSON object")
		return false
	}
	return true
}

func writeValidation(w http.ResponseWriter, r *http.Request, err error) bool {
	var validation *workflow.ValidationError
	if !errors.As(err, &validation) {
		return false
	}
	id, _ := r.Context().Value(requestIDKey{}).(string)
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": validation.Code, "message": "The workflow definition is not valid", "request_id": id, "details": validation.Fields}})
	return true
}

// workflowError maps store and runner errors to API errors.
func (s *Server) workflowError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workflow.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Workflow not found")
	case errors.Is(err, workflow.ErrDisabled):
		writeError(w, r, http.StatusConflict, "WORKFLOW_DISABLED", "Enable the workflow before running it")
	case errors.Is(err, workflow.ErrRunBusy):
		writeError(w, r, http.StatusConflict, "RUN_IN_PROGRESS", "This workflow already has a run in progress")
	case errors.Is(err, workflow.ErrLimit):
		writeError(w, r, http.StatusConflict, "WORKFLOW_LIMIT", "The workflow limit has been reached")
	default:
		if s.Log != nil {
			s.Log.Error("automation request failed", "error", err)
		}
		writeError(w, r, http.StatusServiceUnavailable, "AUTOMATIONS_UNAVAILABLE", "Automations are unavailable")
	}
}

func (s *Server) options(w http.ResponseWriter, r *http.Request) (workflow.Options, bool) {
	snapshot, ok := s.snapshot(w, r)
	if !ok {
		return workflow.Options{}, false
	}
	return workflow.BuildOptions(snapshot), true
}

func (s *Server) workflowOptions(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	if opts, ok := s.options(w, r); ok {
		writeJSON(w, http.StatusOK, opts)
	}
}

func (s *Server) automationSummary(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	summary, err := s.Runner.Store.Summary(r.Context())
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	items, err := s.Runner.Store.ListWorkflows(r.Context())
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	wf, err := s.Runner.Store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.mutation(w, r)
	if !ok {
		return
	}
	wf := workflow.Workflow{Definition: workflow.Definition{Enabled: true}}
	if !decodeBody(w, r, &wf) {
		return
	}
	opts, ok := s.options(w, r)
	if !ok {
		return
	}
	if err := workflow.Validate(&wf.Definition, opts); err != nil {
		if !writeValidation(w, r, err) {
			s.workflowError(w, r, err)
		}
		return
	}
	created, err := s.Runner.Store.CreateWorkflow(r.Context(), wf.Definition, false, actor)
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// updateWorkflow merges the request onto the stored definition, so omitted
// fields keep their current values, then validates the result.
func (s *Server) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.mutation(w, r)
	if !ok {
		return
	}
	wf, err := s.Runner.Store.GetWorkflow(r.Context(), r.PathValue("id"))
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	// The JSON decoder reuses existing slice elements when merging, which would
	// leave stale fields on edited steps; decode steps into a fresh slice and
	// fall back to the stored steps only when the request omits them.
	stored := wf.Steps
	wf.Steps = nil
	if !decodeBody(w, r, &wf) {
		return
	}
	if wf.Steps == nil {
		wf.Steps = stored
	}
	opts, ok := s.options(w, r)
	if !ok {
		return
	}
	if err := workflow.Validate(&wf.Definition, opts); err != nil {
		if !writeValidation(w, r, err) {
			s.workflowError(w, r, err)
		}
		return
	}
	updated, err := s.Runner.Store.UpdateWorkflow(r.Context(), r.PathValue("id"), wf.Definition, actor)
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) setWorkflowEnabled(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := s.mutation(w, r)
		if !ok {
			return
		}
		wf, err := s.Runner.Store.SetEnabled(r.Context(), r.PathValue("id"), enabled, actor)
		if err != nil {
			s.workflowError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, wf)
	}
}

func (s *Server) runWorkflow(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.mutation(w, r)
	if !ok {
		return
	}
	run, err := s.Runner.Start(r.Context(), r.PathValue("id"), workflow.TriggerManual, actor)
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/workflow-runs/"+run.ID)
	writeJSON(w, http.StatusAccepted, run)
}

func queryLimit(r *http.Request, fallback int) int {
	if value := r.URL.Query().Get("limit"); value != "" {
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func (s *Server) listWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	id := r.PathValue("id")
	if _, err := s.Runner.Store.GetWorkflow(r.Context(), id); err != nil {
		s.workflowError(w, r, err)
		return
	}
	runs, err := s.Runner.Store.ListRuns(r.Context(), id, queryLimit(r, 25))
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs})
}

func (s *Server) listRecentRuns(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	runs, err := s.Runner.Store.ListRuns(r.Context(), "", queryLimit(r, 10))
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs})
}

func (s *Server) getWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	run, err := s.Runner.Store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	if !s.automationsReady(w, r) {
		return
	}
	query := r.URL.Query()
	var before int64
	if cursor := query.Get("cursor"); cursor != "" {
		n, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || n < 1 {
			writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Cursor is not valid")
			return
		}
		before = n
	}
	events, more, err := s.Runner.Store.ListAudit(r.Context(), queryLimit(r, 50), before, query.Get("target_type"), query.Get("target_id"))
	if err != nil {
		s.workflowError(w, r, err)
		return
	}
	body := map[string]any{"items": events, "next_cursor": nil}
	if more && len(events) > 0 {
		body["next_cursor"] = strconv.FormatInt(events[len(events)-1].ID, 10)
	}
	writeJSON(w, http.StatusOK, body)
}

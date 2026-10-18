package workflow

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrLimit      = errors.New("workflow limit reached")
	ErrRunBusy    = errors.New("a run is already in progress")
	timeLayout    = time.RFC3339Nano
	defaultActor  = "system"
	maxListedRuns = 200
)

// Store persists workflows, runs and the audit trail. The database allows a
// single connection, so no method holds rows open while issuing another query.
type Store struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func newID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(raw[:])
}

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(value string) time.Time {
	t, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func parseNullTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	t := parseTime(value.String)
	return &t
}

func durationMS(start time.Time, finished *time.Time) *int64 {
	if finished == nil {
		return nil
	}
	ms := finished.Sub(start).Milliseconds()
	return &ms
}

// ---- workflows ----

func (s Store) CountWorkflows(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflows`).Scan(&count)
	return count, err
}

func (s Store) CreateWorkflow(ctx context.Context, def Definition, simulated bool, actor string) (Workflow, error) {
	count, err := s.CountWorkflows(ctx)
	if err != nil {
		return Workflow{}, err
	}
	if count >= MaxWorkflows {
		return Workflow{}, ErrLimit
	}
	id := newID("wf")
	now := s.now()
	body, err := json.Marshal(def)
	if err != nil {
		return Workflow{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO workflows(id,name,enabled,definition_json,simulated,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		id, def.Name, def.Enabled, string(body), simulated, formatTime(now), formatTime(now)); err != nil {
		return Workflow{}, err
	}
	if err := appendAudit(ctx, tx, now, actor, "workflow.created", "workflow", id, "succeeded", map[string]any{"name": def.Name, "trigger": def.Trigger.Type, "steps": len(def.Steps), "simulated": simulated}); err != nil {
		return Workflow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Workflow{}, err
	}
	return s.GetWorkflow(ctx, id)
}

func (s Store) UpdateWorkflow(ctx context.Context, id string, def Definition, actor string) (Workflow, error) {
	body, err := json.Marshal(def)
	if err != nil {
		return Workflow{}, err
	}
	now := s.now()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE workflows SET name=?,enabled=?,definition_json=?,updated_at=? WHERE id=?`, def.Name, def.Enabled, string(body), formatTime(now), id)
	if err != nil {
		return Workflow{}, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Workflow{}, ErrNotFound
	}
	if err := appendAudit(ctx, tx, now, actor, "workflow.updated", "workflow", id, "succeeded", map[string]any{"name": def.Name, "trigger": def.Trigger.Type, "steps": len(def.Steps), "enabled": def.Enabled}); err != nil {
		return Workflow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Workflow{}, err
	}
	return s.GetWorkflow(ctx, id)
}

func (s Store) SetEnabled(ctx context.Context, id string, enabled bool, actor string) (Workflow, error) {
	current, err := s.GetWorkflow(ctx, id)
	if err != nil {
		return Workflow{}, err
	}
	def := current.Definition
	def.Enabled = enabled
	body, err := json.Marshal(def)
	if err != nil {
		return Workflow{}, err
	}
	now := s.now()
	action := "workflow.disabled"
	if enabled {
		action = "workflow.enabled"
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET enabled=?,definition_json=?,updated_at=? WHERE id=?`, enabled, string(body), formatTime(now), id); err != nil {
		return Workflow{}, err
	}
	if err := appendAudit(ctx, tx, now, actor, action, "workflow", id, "succeeded", map[string]any{"name": def.Name}); err != nil {
		return Workflow{}, err
	}
	if err := tx.Commit(); err != nil {
		return Workflow{}, err
	}
	return s.GetWorkflow(ctx, id)
}

// RemoveSimulated deletes seeded demo workflows (and, by cascade, their runs)
// when the server switches to a live inventory source. Audit events remain.
func (s Store) RemoveSimulated(ctx context.Context) (int64, error) {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM workflows WHERE simulated=1`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type workflowRow struct {
	id, definition       string
	enabled, simulated   bool
	createdAt, updatedAt string
}

func (s Store) loadRows(ctx context.Context, where string, args ...any) ([]workflowRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,definition_json,enabled,simulated,created_at,updated_at FROM workflows `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []workflowRow
	for rows.Next() {
		var row workflowRow
		if err := rows.Scan(&row.id, &row.definition, &row.enabled, &row.simulated, &row.createdAt, &row.updatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s Store) hydrate(ctx context.Context, row workflowRow) (Workflow, error) {
	var def Definition
	if err := json.Unmarshal([]byte(row.definition), &def); err != nil {
		return Workflow{}, fmt.Errorf("decode workflow %s: %w", row.id, err)
	}
	def.Enabled = row.enabled
	if def.Steps == nil {
		def.Steps = []Step{}
	}
	wf := Workflow{ID: row.id, Definition: def, Simulated: row.simulated, CreatedAt: parseTime(row.createdAt), UpdatedAt: parseTime(row.updatedAt)}
	last, err := s.lastRun(ctx, row.id)
	if err != nil {
		return Workflow{}, err
	}
	wf.LastRun = last
	if def.Enabled && def.Trigger.Type == TriggerCron {
		if schedule, err := ParseCron(def.Trigger.Expression); err == nil {
			if next, ok := schedule.Next(s.now()); ok {
				wf.NextRunAt = &next
			}
		}
	}
	return wf, nil
}

func (s Store) ListWorkflows(ctx context.Context) ([]Workflow, error) {
	rows, err := s.loadRows(ctx, `ORDER BY created_at, rowid`)
	if err != nil {
		return nil, err
	}
	result := make([]Workflow, 0, len(rows))
	for _, row := range rows {
		wf, err := s.hydrate(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, wf)
	}
	return result, nil
}

func (s Store) GetWorkflow(ctx context.Context, id string) (Workflow, error) {
	rows, err := s.loadRows(ctx, `WHERE id=?`, id)
	if err != nil {
		return Workflow{}, err
	}
	if len(rows) == 0 {
		return Workflow{}, ErrNotFound
	}
	return s.hydrate(ctx, rows[0])
}

func (s Store) lastRun(ctx context.Context, workflowID string) (*RunSummary, error) {
	var summary RunSummary
	var started string
	var finished sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT id,state,trigger_source,started_at,finished_at FROM workflow_runs WHERE workflow_id=? ORDER BY started_at DESC, id DESC LIMIT 1`, workflowID).
		Scan(&summary.ID, &summary.State, &summary.Trigger, &started, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	summary.StartedAt = parseTime(started)
	summary.FinishedAt = parseNullTime(finished)
	summary.DurationMS = durationMS(summary.StartedAt, summary.FinishedAt)
	return &summary, nil
}

// ---- runs ----

// CreateRun records a queued run with all steps pending and appends the
// run-start audit event atomically. It refuses to start a second active run.
func (s Store) CreateRun(ctx context.Context, wf Workflow, trigger, actor string) (Run, error) {
	now := s.now()
	run := Run{ID: newID("run"), WorkflowID: wf.ID, WorkflowName: wf.Name, Trigger: trigger, Actor: actor, State: RunQueued, StartedAt: now, Simulated: wf.Simulated}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_runs WHERE workflow_id=? AND state IN ('queued','running')`, wf.ID).Scan(&active); err != nil {
		return Run{}, err
	}
	if active > 0 {
		return Run{}, ErrRunBusy
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_runs(id,workflow_id,workflow_name,trigger_source,actor,state,started_at,simulated) VALUES(?,?,?,?,?,?,?,?)`,
		run.ID, wf.ID, wf.Name, trigger, actor, RunQueued, formatTime(now), wf.Simulated); err != nil {
		return Run{}, err
	}
	for i, step := range wf.Steps {
		kind := step.Type
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_run_steps(run_id,position,step_id,kind,name,state) VALUES(?,?,?,?,?,?)`, run.ID, i, step.ID, kind, step.Name, StepPending); err != nil {
			return Run{}, err
		}
		run.Steps = append(run.Steps, RunStep{Position: i, StepID: step.ID, Kind: kind, Name: step.Name, State: StepPending})
	}
	if err := appendAudit(ctx, tx, now, actor, "workflow.run.started", "workflow", wf.ID, "succeeded", map[string]any{"run_id": run.ID, "trigger": trigger}); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s Store) MarkRunRunning(ctx context.Context, runID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE workflow_runs SET state=? WHERE id=?`, RunRunning, runID)
	return err
}

// AddOutcomeStep appends a notification step that runs after the ordered
// steps (success or failure notification).
func (s Store) AddOutcomeStep(ctx context.Context, runID string, position int, name string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO workflow_run_steps(run_id,position,step_id,kind,name,state) VALUES(?,?,?,?,?,?)`, runID, position, "outcome", StepNotify, name, StepPending)
	return err
}

func (s Store) UpdateStep(ctx context.Context, runID string, position int, state string, started, finished *time.Time, output, errorCode string) error {
	var startedValue, finishedValue any
	if started != nil {
		startedValue = formatTime(*started)
	}
	if finished != nil {
		finishedValue = formatTime(*finished)
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE workflow_run_steps SET state=?,started_at=COALESCE(?,started_at),finished_at=?,output=?,error_code=? WHERE run_id=? AND position=?`,
		state, startedValue, finishedValue, output, errorCode, runID, position)
	return err
}

func (s Store) FinishRun(ctx context.Context, run Run, state, errorCode, summary string) (Run, error) {
	finished := s.now()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return run, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_runs SET state=?,finished_at=?,error_code=?,summary=? WHERE id=?`, state, formatTime(finished), errorCode, summary, run.ID); err != nil {
		return run, err
	}
	outcome := "succeeded"
	if state != RunSucceeded {
		outcome = "failed"
	}
	if err := appendAudit(ctx, tx, finished, run.Actor, "workflow.run.finished", "workflow", run.WorkflowID, outcome, map[string]any{"run_id": run.ID, "state": state, "error_code": errorCode, "summary": summary}); err != nil {
		return run, err
	}
	if err := tx.Commit(); err != nil {
		return run, err
	}
	return s.GetRun(ctx, run.ID)
}

// RecoverInterrupted fails runs that were active when the process stopped.
func (s Store) RecoverInterrupted(ctx context.Context) (int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,workflow_id,actor FROM workflow_runs WHERE state IN ('queued','running')`)
	if err != nil {
		return 0, err
	}
	type pending struct{ id, workflowID, actor string }
	var list []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.workflowID, &p.actor); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, p)
	}
	rows.Close()
	now := s.now()
	for _, p := range list {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_runs SET state=?,finished_at=?,error_code='INTERRUPTED',summary='The server restarted while this run was active.' WHERE id=?`, RunFailed, formatTime(now), p.id); err != nil {
			tx.Rollback()
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_run_steps SET state=?,finished_at=? WHERE run_id=? AND state IN ('pending','running')`, StepSkipped, formatTime(now), p.id); err != nil {
			tx.Rollback()
			return 0, err
		}
		if err := appendAudit(ctx, tx, now, p.actor, "workflow.run.finished", "workflow", p.workflowID, "failed", map[string]any{"run_id": p.id, "state": RunFailed, "error_code": "INTERRUPTED"}); err != nil {
			tx.Rollback()
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
	}
	return len(list), nil
}

func scanRun(scan func(...any) error) (Run, error) {
	var run Run
	var started string
	var finished sql.NullString
	if err := scan(&run.ID, &run.WorkflowID, &run.WorkflowName, &run.Trigger, &run.Actor, &run.State, &started, &finished, &run.ErrorCode, &run.Summary, &run.Simulated); err != nil {
		return Run{}, err
	}
	run.StartedAt = parseTime(started)
	run.FinishedAt = parseNullTime(finished)
	run.DurationMS = durationMS(run.StartedAt, run.FinishedAt)
	return run, nil
}

const runColumns = `id,workflow_id,workflow_name,trigger_source,actor,state,started_at,finished_at,error_code,summary,simulated`

func (s Store) GetRun(ctx context.Context, id string) (Run, error) {
	run, err := scanRun(s.DB.QueryRowContext(ctx, `SELECT `+runColumns+` FROM workflow_runs WHERE id=?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT position,step_id,kind,name,state,started_at,finished_at,output,error_code FROM workflow_run_steps WHERE run_id=? ORDER BY position`, id)
	if err != nil {
		return Run{}, err
	}
	defer rows.Close()
	run.Steps = []RunStep{}
	for rows.Next() {
		var step RunStep
		var started, finished sql.NullString
		if err := rows.Scan(&step.Position, &step.StepID, &step.Kind, &step.Name, &step.State, &started, &finished, &step.Output, &step.ErrorCode); err != nil {
			return Run{}, err
		}
		step.StartedAt = parseNullTime(started)
		step.FinishedAt = parseNullTime(finished)
		run.Steps = append(run.Steps, step)
	}
	return run, rows.Err()
}

// ListRuns returns recent runs, newest first, without step detail. An empty
// workflowID lists runs across all workflows.
func (s Store) ListRuns(ctx context.Context, workflowID string, limit int) ([]Run, error) {
	if limit <= 0 || limit > maxListedRuns {
		limit = 25
	}
	query := `SELECT ` + runColumns + ` FROM workflow_runs `
	args := []any{}
	if workflowID != "" {
		query += `WHERE workflow_id=? `
		args = append(args, workflowID)
	}
	query += `ORDER BY started_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Run{}
	for rows.Next() {
		run, err := scanRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	return result, rows.Err()
}

// ---- summary ----

type Summary struct {
	Workflows        int        `json:"workflows"`
	Enabled          int        `json:"enabled"`
	Disabled         int        `json:"disabled"`
	ScheduledCount   int        `json:"scheduled_count"`
	NextScheduledAt  *time.Time `json:"next_scheduled_at"`
	RunsLast7Days    int        `json:"runs_last_7_days"`
	SucceededLast7   int        `json:"succeeded_last_7_days"`
	SuccessRate      *float64   `json:"success_rate"`
	FailedLast24h    int        `json:"failed_last_24h"`
	ActiveRuns       int        `json:"active_runs"`
	GeneratedAt      time.Time  `json:"generated_at"`
	SimulatedContent bool       `json:"simulated"`
}

func (s Store) Summary(ctx context.Context) (Summary, error) {
	now := s.now()
	result := Summary{GeneratedAt: now}
	workflows, err := s.ListWorkflows(ctx)
	if err != nil {
		return result, err
	}
	result.Workflows = len(workflows)
	for _, wf := range workflows {
		if wf.Simulated {
			result.SimulatedContent = true
		}
		if wf.Enabled {
			result.Enabled++
		} else {
			result.Disabled++
		}
		if wf.NextRunAt != nil {
			result.ScheduledCount++
			if result.NextScheduledAt == nil || wf.NextRunAt.Before(*result.NextScheduledAt) {
				next := *wf.NextRunAt
				result.NextScheduledAt = &next
			}
		}
	}
	weekAgo := formatTime(now.Add(-7 * 24 * time.Hour))
	dayAgo := formatTime(now.Add(-24 * time.Hour))
	var finished int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(state='succeeded'),0) FROM workflow_runs WHERE started_at>=? AND state IN ('succeeded','failed','cancelled')`, weekAgo).Scan(&finished, &result.SucceededLast7); err != nil {
		return result, err
	}
	result.RunsLast7Days = finished
	if finished > 0 {
		rate := float64(result.SucceededLast7) / float64(finished) * 100
		result.SuccessRate = &rate
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_runs WHERE started_at>=? AND state='failed'`, dayAgo).Scan(&result.FailedLast24h); err != nil {
		return result, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_runs WHERE state IN ('queued','running')`).Scan(&result.ActiveRuns); err != nil {
		return result, err
	}
	return result, nil
}

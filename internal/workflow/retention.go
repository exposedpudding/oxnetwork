package workflow

import (
	"context"
	"time"
)

// Run retention. Finished runs and their step results are the only unbounded
// growth in the automation tables, so they are trimmed. Two rules protect history:
// a workflow always keeps its newest KeepRunsPerWorkflow finished runs however old
// they are, and queued or running rows are never touched. The audit log is not
// pruned: it is append-only by design (database triggers refuse updates and
// deletes), and each trimming is itself recorded there.
const (
	DefaultRunRetentionDays = 90
	KeepRunsPerWorkflow     = 20
	pruneEvery              = 6 * time.Hour
)

// finishedBeyondKeep selects finished runs that start before ? and are not among the
// newest ? finished runs of their workflow. started_at is RFC3339Nano in UTC, which
// orders correctly as text except within the same second, so the cutoff can be off
// by under one second; that does not matter for a retention window of days.
const finishedBeyondKeep = `FROM workflow_runs WHERE state NOT IN ('queued','running') AND started_at < ? AND id NOT IN (
	SELECT id FROM (
		SELECT id, ROW_NUMBER() OVER (PARTITION BY workflow_id ORDER BY started_at DESC, id DESC) AS rank
		FROM workflow_runs WHERE state NOT IN ('queued','running')
	) WHERE rank <= ?)`

// PruneRuns deletes finished runs that started before cutoff, keeping the newest
// keep finished runs of every workflow. Steps are removed with their run. It returns
// the number of runs deleted and records one audit event when that is not zero.
func (s Store) PruneRuns(ctx context.Context, cutoff time.Time, keep int) (int64, error) {
	if keep < 0 {
		keep = 0
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	args := []any{formatTime(cutoff), keep}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_run_steps WHERE run_id IN (SELECT id `+finishedBeyondKeep+`)`, args...); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `DELETE `+finishedBeyondKeep, args...)
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if deleted > 0 {
		if err := appendAudit(ctx, tx, s.now(), "system", "workflow.runs.pruned", "workflow_runs", "all", "success",
			map[string]any{"deleted": deleted, "older_than": formatTime(cutoff), "keep_per_workflow": keep}); err != nil {
			return 0, err
		}
	}
	return deleted, tx.Commit()
}

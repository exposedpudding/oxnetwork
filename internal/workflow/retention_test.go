package workflow

import (
	"context"
	"testing"
	"time"
)

// finishedRunAt records a run that started and finished at the given time.
func (h *harness) finishedRunAt(t *testing.T, wf Workflow, at time.Time, state string) Run {
	t.Helper()
	ctx := context.Background()
	*h.clock = at
	run, err := h.store.CreateRun(ctx, wf, TriggerManual, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if state == RunQueued || state == RunRunning {
		return run
	}
	if _, err := h.store.FinishRun(ctx, run, state, "", "done"); err != nil {
		t.Fatal(err)
	}
	return run
}

func (h *harness) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := h.store.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPruneRunsKeepsNewestPerWorkflowAndActiveRuns(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	busy := h.workflowNamed(t, "Health check sweep")
	quiet := h.workflowNamed(t, "Check worker on demand")
	day := 24 * time.Hour

	// 25 old finished runs plus one recent run for one workflow; three old runs and one
	// old run that never finished for the other.
	for i := 25; i >= 1; i-- {
		h.finishedRunAt(t, busy, testNow.Add(-time.Duration(100+i)*day), RunSucceeded)
	}
	recent := h.finishedRunAt(t, busy, testNow.Add(-day), RunFailed)
	for i := 3; i >= 1; i-- {
		h.finishedRunAt(t, quiet, testNow.Add(-time.Duration(200+i)*day), RunSucceeded)
	}
	stuck := h.finishedRunAt(t, quiet, testNow.Add(-300*day), RunRunning)
	*h.clock = testNow

	auditBefore := h.count(t, `SELECT COUNT(*) FROM audit_events WHERE action='workflow.runs.pruned'`)
	deleted, err := h.store.PruneRuns(ctx, testNow.Add(-90*day), KeepRunsPerWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 6 {
		t.Fatalf("deleted %d, want 6 (26 finished runs minus the 20 newest)", deleted)
	}
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs WHERE workflow_id=?`, busy.ID); got != 20 {
		t.Fatalf("busy workflow keeps %d runs, want 20", got)
	}
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs WHERE id=?`, recent.ID); got != 1 {
		t.Fatal("the recent run must survive")
	}
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs WHERE workflow_id=?`, quiet.ID); got != 4 {
		t.Fatalf("quiet workflow lost runs it should keep: %d", got)
	}
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs WHERE id=?`, stuck.ID); got != 1 {
		t.Fatal("a queued or running run must never be pruned, however old")
	}
	if orphans := h.count(t, `SELECT COUNT(*) FROM workflow_run_steps WHERE run_id NOT IN (SELECT id FROM workflow_runs)`); orphans != 0 {
		t.Fatalf("%d step rows left without a run", orphans)
	}
	// Exactly the pruned runs' steps are gone: 20 kept runs of the busy workflow, plus the
	// quiet workflow's four runs (three finished, one still active).
	wantSteps := 20*len(busy.Steps) + 4*len(quiet.Steps)
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_run_steps`); got != wantSteps {
		t.Fatalf("%d step rows remain, want %d", got, wantSteps)
	}
	if got := h.count(t, `SELECT COUNT(*) FROM audit_events WHERE action='workflow.runs.pruned'`); got != auditBefore+1 {
		t.Fatalf("expected one prune audit event, got %d new", got-auditBefore)
	}

	// Nothing left to prune: no deletion and no new audit event.
	deleted, err = h.store.PruneRuns(ctx, testNow.Add(-90*day), KeepRunsPerWorkflow)
	if err != nil || deleted != 0 {
		t.Fatalf("second prune: %d %v", deleted, err)
	}
	if got := h.count(t, `SELECT COUNT(*) FROM audit_events WHERE action='workflow.runs.pruned'`); got != auditBefore+1 {
		t.Fatal("an empty prune must not write an audit event")
	}
}

func TestPruneNeverTouchesAuditHistory(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	wf := h.workflowNamed(t, "Health check sweep")
	for i := 30; i >= 1; i-- {
		h.finishedRunAt(t, wf, testNow.Add(-time.Duration(100+i)*24*time.Hour), RunSucceeded)
	}
	*h.clock = testNow
	before := h.count(t, `SELECT COUNT(*) FROM audit_events`)
	if _, err := h.store.PruneRuns(ctx, testNow.Add(-90*24*time.Hour), 5); err != nil {
		t.Fatal(err)
	}
	if after := h.count(t, `SELECT COUNT(*) FROM audit_events`); after != before+1 {
		t.Fatalf("audit rows went from %d to %d; pruning must only add its own record", before, after)
	}
}

func TestSchedulerPrunesOncePerInterval(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	wf := h.workflowNamed(t, "Health check sweep")
	day := 24 * time.Hour
	scheduler := &Scheduler{Runner: h.runner, Inventory: demoInventory(), RetentionDays: 7}
	old := func() {
		for i := 25; i >= 1; i-- {
			h.finishedRunAt(t, wf, testNow.Add(-time.Duration(20+i)*day), RunSucceeded)
		}
		*h.clock = testNow
	}

	old()
	scheduler.Tick(ctx, testNow)
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs`); got != KeepRunsPerWorkflow {
		t.Fatalf("first tick left %d runs, want %d", got, KeepRunsPerWorkflow)
	}
	// Older runs added after the first prune are left alone until the interval passes.
	h.finishedRunAt(t, wf, testNow.Add(-60*day), RunSucceeded)
	*h.clock = testNow
	scheduler.Tick(ctx, testNow.Add(time.Hour))
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs`); got != KeepRunsPerWorkflow+1 {
		t.Fatalf("a tick one hour later pruned again: %d runs", got)
	}
	scheduler.Tick(ctx, testNow.Add(7*time.Hour))
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs`); got != KeepRunsPerWorkflow {
		t.Fatalf("a tick after the interval left %d runs, want %d", got, KeepRunsPerWorkflow)
	}
}

func TestSchedulerRetriesFailedPruneOnNextTick(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	wf := h.workflowNamed(t, "Health check sweep")
	for i := 25; i >= 1; i-- {
		h.finishedRunAt(t, wf, testNow.Add(-time.Duration(20+i)*24*time.Hour), RunSucceeded)
	}
	*h.clock = testNow
	scheduler := &Scheduler{Runner: h.runner, Inventory: demoInventory(), RetentionDays: 7}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	scheduler.Tick(cancelled, testNow)
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs`); got != 25 {
		t.Fatalf("failed prune changed run count: %d", got)
	}
	scheduler.Tick(ctx, testNow.Add(time.Minute))
	if got := h.count(t, `SELECT COUNT(*) FROM workflow_runs`); got != KeepRunsPerWorkflow {
		t.Fatalf("successful retry left %d runs, want %d", got, KeepRunsPerWorkflow)
	}
}

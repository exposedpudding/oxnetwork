package workflow

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/integrations/demo"
)

var testNow = time.Date(2026, 10, 18, 12, 7, 0, 0, time.UTC)

type fakeInventory struct {
	snapshot func(now time.Time) domain.Snapshot
}

func (f fakeInventory) Snapshot(_ context.Context, now time.Time) (domain.Snapshot, error) {
	return f.snapshot(now), nil
}

func demoInventory() fakeInventory {
	return fakeInventory{snapshot: func(now time.Time) domain.Snapshot { return demo.Provider{}.Snapshot(now) }}
}

type harness struct {
	store  Store
	runner *Runner
	clock  *time.Time
}

func newHarness(t *testing.T, inventory Inventory) *harness {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "workflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	clock := testNow
	store := Store{DB: database, Now: func() time.Time { return clock }}
	return &harness{store: store, runner: &Runner{Store: store, Inventory: inventory, Hub: &Hub{}}, clock: &clock}
}

func (h *harness) workflowNamed(t *testing.T, name string) Workflow {
	t.Helper()
	list, err := h.store.ListWorkflows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, wf := range list {
		if wf.Name == name {
			return wf
		}
	}
	t.Fatalf("workflow %q not found", name)
	return Workflow{}
}

func (h *harness) runAndWait(t *testing.T, id, trigger string) Run {
	t.Helper()
	run, err := h.runner.Start(context.Background(), id, trigger, "tester")
	if err != nil {
		t.Fatal(err)
	}
	h.runner.Wait()
	finished, err := h.store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return finished
}

func TestParseCronAndMatches(t *testing.T) {
	for _, bad := range []string{"", "* * * *", "60 * * * *", "* 24 * * *", "*/0 * * * *", "a * * * *", "5-1 * * * *", "* * 0 * *", "* * * 13 *"} {
		if _, err := ParseCron(bad); err == nil {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
	every15, err := ParseCron("*/15 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) time.Time { return time.Date(2026, 10, 18, h, m, 30, 0, time.UTC) }
	if !every15.Matches(at(3, 0)) || !every15.Matches(at(3, 45)) || every15.Matches(at(3, 7)) {
		t.Fatal("*/15 matched the wrong minutes")
	}
	next, ok := every15.Next(at(12, 7))
	if !ok || !next.Equal(time.Date(2026, 10, 18, 12, 15, 0, 0, time.UTC)) {
		t.Fatalf("next = %v %v", next, ok)
	}
	// 2026-10-18 is a Sunday. Day 7 and day 0 are both Sunday.
	sunday, err := ParseCron("30 4 * * 7")
	if err != nil {
		t.Fatal(err)
	}
	if !sunday.Matches(time.Date(2026, 10, 18, 4, 30, 0, 0, time.UTC)) || sunday.Matches(time.Date(2026, 10, 19, 4, 30, 0, 0, time.UTC)) {
		t.Fatal("day-of-week 7 should mean Sunday")
	}
	// When both day fields are restricted, either may match.
	either, _ := ParseCron("0 0 1 * 1")
	if !either.Matches(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) || !either.Matches(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)) || either.Matches(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("restricted day-of-month and day-of-week should combine with OR")
	}
	rangeStep, err := ParseCron("0-30/10 9-17 * * 1-5")
	if err != nil || !rangeStep.Matches(time.Date(2026, 10, 19, 9, 20, 0, 0, time.UTC)) || rangeStep.Matches(time.Date(2026, 10, 19, 9, 25, 0, 0, time.UTC)) {
		t.Fatalf("range with step: %v", err)
	}
}

func TestDemoDefinitionsAreValid(t *testing.T) {
	options := BuildOptions(demo.Provider{}.Snapshot(testNow))
	for _, def := range DemoDefinitions() {
		def := def
		if err := Validate(&def, options); err != nil {
			t.Errorf("%s: %v", def.Name, err)
		}
	}
}

func TestValidateRejectsBadDefinitions(t *testing.T) {
	options := BuildOptions(demo.Provider{}.Snapshot(testNow))
	valid := func() Definition {
		return Definition{Name: "Check", Enabled: true, Trigger: Trigger{Type: TriggerManual}, Steps: []Step{{ID: "s1", Type: StepAction, Name: "Check", ActionType: "check_node_health", TargetID: "demo-worker"}}, OnFailure: OnFailure{Behavior: OnFailureStop}}
	}
	cases := map[string]struct {
		mutate func(*Definition)
		field  string
		code   string
	}{
		"empty name":      {func(d *Definition) { d.Name = "  " }, "name", "INVALID_WORKFLOW"},
		"bad cron":        {func(d *Definition) { d.Trigger = Trigger{Type: TriggerCron, Expression: "* * *"} }, "trigger", "INVALID_WORKFLOW"},
		"unknown trigger": {func(d *Definition) { d.Trigger = Trigger{Type: "webhook"} }, "trigger", "INVALID_WORKFLOW"},
		"health no target": {func(d *Definition) {
			d.Trigger = Trigger{Type: TriggerHealthCondition, TargetID: "nope", Condition: "stale"}
		}, "trigger", "INVALID_WORKFLOW"},
		"no steps":       {func(d *Definition) { d.Steps = nil }, "steps", "INVALID_WORKFLOW"},
		"unknown action": {func(d *Definition) { d.Steps[0].ActionType = "format_disk" }, "step.0.action", "INVALID_WORKFLOW"},
		"planned action": {func(d *Definition) { d.Steps[0].ActionType = "wake_node" }, "step.0.action", "ACTION_NOT_SUPPORTED"},
		"unknown target": {func(d *Definition) { d.Steps[0].TargetID = "ghost" }, "step.0.target", "INVALID_WORKFLOW"},
		"bad step id":    {func(d *Definition) { d.Steps[0].ID = "has space" }, "step.0.id", "INVALID_WORKFLOW"},
		"notify bad channel": {func(d *Definition) {
			d.Steps = append(d.Steps, Step{ID: "n", Type: StepNotify, Name: "n", Channel: "discord", Message: "hi"})
		}, "step.1.channel", "INVALID_WORKFLOW"},
		"notify no message": {func(d *Definition) {
			d.Steps = append(d.Steps, Step{ID: "n", Type: StepNotify, Name: "n", Channel: "activity"})
		}, "step.1.message", "INVALID_WORKFLOW"},
		"duplicate step ids": {func(d *Definition) { d.Steps = append(d.Steps, d.Steps[0]) }, "step.1.id", "INVALID_WORKFLOW"},
		"bad failure mode":   {func(d *Definition) { d.OnFailure.Behavior = "retry" }, "on_failure", "INVALID_WORKFLOW"},
		"too many steps": {func(d *Definition) {
			for i := 0; i < MaxSteps; i++ {
				d.Steps = append(d.Steps, Step{ID: "x" + strings.Repeat("a", i+1), Type: StepNotify, Name: "n", Channel: "activity", Message: "m"})
			}
		}, "steps", "INVALID_WORKFLOW"},
	}
	for name, tc := range cases {
		def := valid()
		tc.mutate(&def)
		err := Validate(&def, options)
		var validation *ValidationError
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		validation, _ = err.(*ValidationError)
		if validation == nil || validation.Fields[tc.field] == "" || validation.Code != tc.code {
			t.Errorf("%s: got %v (code %v), want field %s code %s", name, err, validation, tc.field, tc.code)
		}
	}
	good := valid()
	if err := Validate(&good, options); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
}

func TestSeedAndRemoveSimulated(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	if err := SeedDemo(ctx, h.store); err != nil {
		t.Fatal(err)
	}
	if err := SeedDemo(ctx, h.store); err != nil { // idempotent
		t.Fatal(err)
	}
	list, _ := h.store.ListWorkflows(ctx)
	if len(list) != 3 || !list[0].Simulated {
		t.Fatalf("seeded %d workflows: %+v", len(list), list)
	}
	sweep := h.workflowNamed(t, "Health check sweep")
	if sweep.NextRunAt == nil || !sweep.NextRunAt.Equal(time.Date(2026, 10, 18, 12, 15, 0, 0, time.UTC)) {
		t.Fatalf("next run: %v", sweep.NextRunAt)
	}
	if off := h.workflowNamed(t, "Stale worker alert"); off.Enabled || off.NextRunAt != nil {
		t.Fatalf("disabled workflow: %+v", off)
	}
	removed, err := h.store.RemoveSimulated(ctx)
	if err != nil || removed != 3 {
		t.Fatalf("removed %d: %v", removed, err)
	}
	events, _, _ := h.store.ListAudit(ctx, 50, 0, "", "")
	if len(events) != 3 { // creation events survive removal
		t.Fatalf("audit events after removal: %d", len(events))
	}
}

func TestRunnerFailureStopsAndNotifies(t *testing.T) {
	h := newHarness(t, demoInventory())
	_ = SeedDemo(context.Background(), h.store)
	wf := h.workflowNamed(t, "Check worker on demand")
	run := h.runAndWait(t, wf.ID, TriggerManual)
	// The demo worker report is 3 minutes old with a 90 second limit, so the
	// freshness check fails, the second step is skipped, and the failure
	// notification is recorded.
	if run.State != RunFailed || run.ErrorCode != "CHECK_FAILED" || run.FinishedAt == nil || run.DurationMS == nil {
		t.Fatalf("run: %+v", run)
	}
	want := []string{StepFailed, StepSkipped, StepSucceeded}
	if len(run.Steps) != len(want) {
		t.Fatalf("steps: %+v", run.Steps)
	}
	for i, state := range want {
		if run.Steps[i].State != state {
			t.Errorf("step %d state = %s, want %s (%+v)", i, run.Steps[i].State, state, run.Steps[i])
		}
	}
	if !strings.Contains(run.Steps[0].Output, "Demo Worker") || run.Steps[2].Name != "Failure notification" {
		t.Fatalf("unexpected step detail: %+v", run.Steps)
	}
	if !strings.Contains(run.Summary, "Step 1") {
		t.Fatalf("summary: %q", run.Summary)
	}
	last := h.workflowNamed(t, "Check worker on demand").LastRun
	if last == nil || last.ID != run.ID || last.State != RunFailed {
		t.Fatalf("last run: %+v", last)
	}
}

func TestRunnerContinueRunsRemainingStepsAndStillFails(t *testing.T) {
	h := newHarness(t, demoInventory())
	_ = SeedDemo(context.Background(), h.store)
	wf := h.workflowNamed(t, "Health check sweep")
	run := h.runAndWait(t, wf.ID, TriggerCron)
	// Gateway is healthy and fresh; worker is degraded and stale. Behaviour is
	// "continue", so both steps run, the run still fails, and the failure
	// notification is sent.
	if run.State != RunFailed || run.Trigger != TriggerCron {
		t.Fatalf("run: %+v", run)
	}
	if run.Steps[0].State != StepSucceeded || run.Steps[1].State != StepFailed || run.Steps[2].State != StepSucceeded || run.Steps[2].Name != "Failure notification" {
		t.Fatalf("steps: %+v", run.Steps)
	}
}

func TestRunnerSuccessSendsSuccessNotification(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	wf, err := h.store.CreateWorkflow(ctx, Definition{Name: "Gateway check", Enabled: true, Trigger: Trigger{Type: TriggerManual},
		Steps:     []Step{{ID: "g", Type: StepAction, Name: "Gateway health", ActionType: "check_node_health", TargetID: "demo-gateway"}},
		OnSuccess: OnSuccess{NotifyChannel: "activity"}, OnFailure: OnFailure{Behavior: OnFailureStop}}, false, "tester")
	if err != nil {
		t.Fatal(err)
	}
	run := h.runAndWait(t, wf.ID, TriggerManual)
	if run.State != RunSucceeded || run.ErrorCode != "" || run.Summary != "1 of 1 steps succeeded." || len(run.Steps) != 2 || run.Steps[1].Name != "Success notification" || run.Actor != "tester" {
		t.Fatalf("run: %+v", run)
	}
	events, _, _ := h.store.ListAudit(ctx, 50, 0, "workflow", wf.ID)
	var actions []string
	for _, e := range events {
		actions = append(actions, e.Action+":"+e.Outcome)
	}
	joined := strings.Join(actions, ",")
	for _, want := range []string{"workflow.created:succeeded", "workflow.run.started:succeeded", "workflow.notification:succeeded", "workflow.run.finished:succeeded"} {
		if !strings.Contains(joined, want) {
			t.Errorf("audit trail missing %s: %s", want, joined)
		}
	}
}

func TestRunnerReportsMissingNodeAndUnsupportedAction(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	// Bypass validation to simulate a stored definition that is no longer valid.
	wf, err := h.store.CreateWorkflow(ctx, Definition{Name: "Stale definition", Enabled: true, Trigger: Trigger{Type: TriggerManual},
		Steps: []Step{
			{ID: "a", Type: StepAction, Name: "Gone node", ActionType: "check_node_health", TargetID: "removed-node"},
			{ID: "b", Type: StepAction, Name: "Future action", ActionType: "wake_node", TargetID: "demo-worker"},
		}, OnFailure: OnFailure{Behavior: OnFailureContinue}}, false, "tester")
	if err != nil {
		t.Fatal(err)
	}
	run := h.runAndWait(t, wf.ID, TriggerManual)
	if run.State != RunFailed || run.Steps[0].ErrorCode != "NODE_NOT_FOUND" || run.Steps[1].ErrorCode != "ACTION_NOT_SUPPORTED" {
		t.Fatalf("run: %+v", run)
	}
}

func TestStartRejectsBusyDisabledAndMissing(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	if _, err := h.runner.Start(ctx, "wf_missing", TriggerManual, "t"); err != ErrNotFound {
		t.Fatalf("missing: %v", err)
	}
	off := h.workflowNamed(t, "Stale worker alert")
	if _, err := h.runner.Start(ctx, off.ID, TriggerManual, "t"); err != ErrDisabled {
		t.Fatalf("disabled: %v", err)
	}
	wf := h.workflowNamed(t, "Check worker on demand")
	if _, err := h.store.CreateRun(ctx, wf, TriggerManual, "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.runner.Start(ctx, wf.ID, TriggerManual, "t"); err != ErrRunBusy {
		t.Fatalf("busy: %v", err)
	}
	recovered, err := h.store.RecoverInterrupted(ctx)
	if err != nil || recovered != 1 {
		t.Fatalf("recovered %d: %v", recovered, err)
	}
	runs, _ := h.store.ListRuns(ctx, wf.ID, 10)
	if len(runs) != 1 || runs[0].State != RunFailed || runs[0].ErrorCode != "INTERRUPTED" {
		t.Fatalf("runs: %+v", runs)
	}
	if _, err := h.runner.Start(ctx, wf.ID, TriggerManual, "t"); err != nil {
		t.Fatalf("start after recovery: %v", err)
	}
	h.runner.Wait()
}

func TestAuditIsAppendOnly(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	if err := h.store.Audit(ctx, "tester", "test.event", "workflow", "wf_1", "succeeded", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.DB.ExecContext(ctx, `UPDATE audit_events SET actor='attacker'`); err == nil {
		t.Fatal("audit update must be rejected")
	}
	if _, err := h.store.DB.ExecContext(ctx, `DELETE FROM audit_events`); err == nil {
		t.Fatal("audit delete must be rejected")
	}
	events, more, err := h.store.ListAudit(ctx, 10, 0, "", "")
	if err != nil || more || len(events) != 1 || events[0].Actor != "tester" || events[0].Detail["k"] != "v" {
		t.Fatalf("events: %+v more=%v err=%v", events, more, err)
	}
}

func TestAuditPagination(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_ = h.store.Audit(ctx, "t", "test.event", "workflow", "wf_1", "succeeded", nil)
	}
	page1, more, _ := h.store.ListAudit(ctx, 2, 0, "", "")
	if len(page1) != 2 || !more {
		t.Fatalf("page 1: %d more=%v", len(page1), more)
	}
	page2, more, _ := h.store.ListAudit(ctx, 2, page1[1].ID, "", "")
	page3, more3, _ := h.store.ListAudit(ctx, 2, page2[1].ID, "", "")
	if len(page2) != 2 || !more || len(page3) != 1 || more3 || page3[0].ID >= page2[1].ID {
		t.Fatalf("pagination: %d %v %d %v", len(page2), more, len(page3), more3)
	}
}

func TestSummary(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	h.runAndWait(t, h.workflowNamed(t, "Check worker on demand").ID, TriggerManual) // fails
	h.runAndWait(t, h.workflowNamed(t, "Health check sweep").ID, TriggerCron)       // fails (worker stale)
	good, _ := h.store.CreateWorkflow(ctx, Definition{Name: "Gateway check", Enabled: true, Trigger: Trigger{Type: TriggerManual},
		Steps: []Step{{ID: "g", Type: StepAction, Name: "g", ActionType: "check_node_health", TargetID: "demo-gateway"}}, OnFailure: OnFailure{Behavior: OnFailureStop}}, false, "t")
	h.runAndWait(t, good.ID, TriggerManual)
	summary, err := h.store.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Workflows != 4 || summary.Enabled != 3 || summary.Disabled != 1 || summary.ScheduledCount != 1 || summary.NextScheduledAt == nil || !summary.SimulatedContent {
		t.Fatalf("workflow counts: %+v", summary)
	}
	if summary.RunsLast7Days != 3 || summary.SucceededLast7 != 1 || summary.FailedLast24h != 2 || summary.SuccessRate == nil || int(*summary.SuccessRate) != 33 || summary.ActiveRuns != 0 {
		t.Fatalf("run counts: %+v", summary)
	}
}

func TestSchedulerFiresCronOncePerMinuteAndSkipsBusy(t *testing.T) {
	h := newHarness(t, demoInventory())
	ctx := context.Background()
	_ = SeedDemo(ctx, h.store)
	scheduler := &Scheduler{Runner: h.runner, Inventory: demoInventory()}
	sweep := h.workflowNamed(t, "Health check sweep")

	scheduler.Tick(ctx, time.Date(2026, 10, 18, 12, 7, 5, 0, time.UTC)) // not a */15 minute
	h.runner.Wait()
	if runs, _ := h.store.ListRuns(ctx, sweep.ID, 10); len(runs) != 0 {
		t.Fatalf("unexpected runs at 12:07: %d", len(runs))
	}
	scheduler.Tick(ctx, time.Date(2026, 10, 18, 12, 15, 2, 0, time.UTC))
	h.runner.Wait()
	scheduler.Tick(ctx, time.Date(2026, 10, 18, 12, 15, 17, 0, time.UTC)) // same minute: no second run
	h.runner.Wait()
	runs, _ := h.store.ListRuns(ctx, sweep.ID, 10)
	if len(runs) != 1 || runs[0].Trigger != TriggerCron || runs[0].Actor != "scheduler" {
		t.Fatalf("runs at 12:15: %+v", runs)
	}
	// A still-active run causes the next trigger to be skipped and audited.
	if _, err := h.store.CreateRun(ctx, sweep, TriggerManual, "t"); err != nil {
		t.Fatal(err)
	}
	scheduler.Tick(ctx, time.Date(2026, 10, 18, 12, 30, 1, 0, time.UTC))
	h.runner.Wait()
	events, _, _ := h.store.ListAudit(ctx, 50, 0, "workflow", sweep.ID)
	skipped := false
	for _, e := range events {
		skipped = skipped || e.Action == "workflow.run.skipped"
	}
	if !skipped {
		t.Fatal("expected a workflow.run.skipped audit event")
	}
}

func TestSchedulerHealthConditionIsEdgeTriggered(t *testing.T) {
	stale := true
	inventory := fakeInventory{snapshot: func(now time.Time) domain.Snapshot {
		observed := now.Add(-10 * time.Second)
		if stale {
			observed = now.Add(-10 * time.Minute)
		}
		return domain.Snapshot{Nodes: []domain.Node{{ID: "demo-worker", Name: "Demo Worker", Health: domain.Health{State: "healthy", ObservedAt: observed, StaleAfterSeconds: 90, Source: "test"}}}}
	}}
	h := newHarness(t, inventory)
	ctx := context.Background()
	wf, err := h.store.CreateWorkflow(ctx, Definition{Name: "Stale alert", Enabled: true, Trigger: Trigger{Type: TriggerHealthCondition, TargetID: "demo-worker", Condition: "stale", ForSeconds: 60},
		Steps: []Step{{ID: "n", Type: StepNotify, Name: "Alert", Channel: "activity", Message: "stale"}}, OnFailure: OnFailure{Behavior: OnFailureStop}}, false, "t")
	if err != nil {
		t.Fatal(err)
	}
	scheduler := &Scheduler{Runner: h.runner, Inventory: inventory}
	count := func() int { h.runner.Wait(); runs, _ := h.store.ListRuns(ctx, wf.ID, 10); return len(runs) }
	scheduler.Tick(ctx, testNow)
	scheduler.Tick(ctx, testNow.Add(30*time.Second))
	if count() != 0 {
		t.Fatal("must not fire before the condition has held for 60 seconds")
	}
	scheduler.Tick(ctx, testNow.Add(61*time.Second))
	if count() != 1 {
		t.Fatal("should fire once the condition has held long enough")
	}
	scheduler.Tick(ctx, testNow.Add(2*time.Minute))
	if count() != 1 {
		t.Fatal("must not fire again while the condition persists")
	}
	stale = false
	scheduler.Tick(ctx, testNow.Add(3*time.Minute)) // condition clears and re-arms
	stale = true
	scheduler.Tick(ctx, testNow.Add(4*time.Minute))
	scheduler.Tick(ctx, testNow.Add(5*time.Minute+5*time.Second))
	if count() != 2 {
		t.Fatalf("should fire again after the condition cleared, got %d runs", count())
	}
}

func TestTriggerJSONShapes(t *testing.T) {
	cases := map[string]Trigger{
		`{"type":"manual"}`:                        {Type: TriggerManual},
		`{"type":"cron","expression":"0 * * * *"}`: {Type: TriggerCron, Expression: "0 * * * *"},
		`{"type":"health_condition","target_id":"n","condition":"stale","for_seconds":0}`: {Type: TriggerHealthCondition, TargetID: "n", Condition: "stale"},
	}
	for want, trigger := range cases {
		got, err := trigger.MarshalJSON()
		if err != nil || string(got) != want {
			t.Errorf("got %s (%v), want %s", got, err, want)
		}
	}
}

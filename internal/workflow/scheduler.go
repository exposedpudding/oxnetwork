package workflow

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

// Scheduler starts cron and health-condition workflows. Cron schedules are
// evaluated once per UTC minute; minutes missed while the server was down are
// not replayed. Health conditions are edge-triggered: a workflow fires once
// when its condition has held for the configured duration and re-arms only
// after the condition clears.
type Scheduler struct {
	Runner    *Runner
	Inventory Inventory
	Log       *slog.Logger
	Interval  time.Duration
	// RetentionDays is how long finished runs are kept (see PruneRuns). Zero means
	// DefaultRunRetentionDays.
	RetentionDays int

	lastPrune  time.Time
	lastMinute time.Time
	since      map[string]time.Time
	fired      map[string]bool
}

func (s *Scheduler) log(msg string, args ...any) {
	if s.Log != nil {
		s.Log.Info(msg, args...)
	}
}

// Run ticks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.Tick(ctx, now.UTC())
		}
	}
}

// prune trims old finished runs at most once every pruneEvery. A failure is logged and
// retried on the next interval; it never blocks scheduling.
func (s *Scheduler) prune(ctx context.Context, now time.Time) {
	if !s.lastPrune.IsZero() && now.Sub(s.lastPrune) < pruneEvery {
		return
	}
	s.lastPrune = now
	days := s.RetentionDays
	if days <= 0 {
		days = DefaultRunRetentionDays
	}
	deleted, err := s.Runner.Store.PruneRuns(ctx, now.AddDate(0, 0, -days), KeepRunsPerWorkflow)
	switch {
	case err != nil:
		s.log("run retention failed", "error", err)
	case deleted > 0:
		s.log("old workflow runs removed", "deleted", deleted, "retention_days", days)
	}
}

// Tick evaluates every enabled workflow once for the given instant.
func (s *Scheduler) Tick(ctx context.Context, now time.Time) {
	s.prune(ctx, now)
	workflows, err := s.Runner.Store.ListWorkflows(ctx)
	if err != nil {
		s.log("scheduler could not list workflows", "error", err)
		return
	}
	minute := now.Truncate(time.Minute)
	newMinute := !minute.Equal(s.lastMinute)
	if newMinute {
		s.lastMinute = minute
	}
	var snapshot *domain.Snapshot
	active := map[string]bool{}
	for _, wf := range workflows {
		if !wf.Enabled {
			continue
		}
		switch wf.Trigger.Type {
		case TriggerCron:
			if !newMinute {
				continue
			}
			schedule, err := ParseCron(wf.Trigger.Expression)
			if err != nil || !schedule.Matches(minute) {
				continue
			}
			s.start(ctx, wf.ID, TriggerCron)
		case TriggerHealthCondition:
			active[wf.ID] = true
			if snapshot == nil {
				loaded, err := s.Inventory.Snapshot(ctx, now)
				if err != nil {
					s.log("scheduler could not read inventory", "error", err)
					continue
				}
				snapshot = &loaded
			}
			s.evaluateHealth(ctx, wf, *snapshot, now)
		}
	}
	for id := range s.since {
		if !active[id] {
			delete(s.since, id)
			delete(s.fired, id)
		}
	}
}

func (s *Scheduler) evaluateHealth(ctx context.Context, wf Workflow, snapshot domain.Snapshot, now time.Time) {
	if s.since == nil {
		s.since, s.fired = map[string]time.Time{}, map[string]bool{}
	}
	holds := false
	for _, node := range snapshot.Nodes {
		if node.ID != wf.Trigger.TargetID {
			continue
		}
		switch wf.Trigger.Condition {
		case "unhealthy", "degraded":
			holds = node.Health.State == wf.Trigger.Condition
		case "stale":
			holds = node.Health.Freshness(now) == "stale"
		}
	}
	if !holds {
		delete(s.since, wf.ID)
		delete(s.fired, wf.ID)
		return
	}
	if _, ok := s.since[wf.ID]; !ok {
		s.since[wf.ID] = now
	}
	if s.fired[wf.ID] || now.Sub(s.since[wf.ID]) < time.Duration(wf.Trigger.ForSeconds)*time.Second {
		return
	}
	if s.start(ctx, wf.ID, TriggerHealthCondition) {
		s.fired[wf.ID] = true
	}
}

// start returns true when the trigger was consumed: either a run began or the
// workflow was already busy (recorded in the audit log).
func (s *Scheduler) start(ctx context.Context, workflowID, trigger string) bool {
	_, err := s.Runner.Start(ctx, workflowID, trigger, "scheduler")
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrRunBusy):
		_ = s.Runner.Store.Audit(ctx, "scheduler", "workflow.run.skipped", "workflow", workflowID, "skipped", map[string]any{"trigger": trigger, "reason": "a run is already in progress"})
		return true
	default:
		s.log("scheduled run could not start", "workflow", workflowID, "trigger", trigger, "error", err)
		return false
	}
}

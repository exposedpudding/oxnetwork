package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/domain"
)

var (
	ErrDisabled = errors.New("workflow is disabled")

	// Executable actions in v1: read-only checks against stored inventory.
	// Mutating actions stay listed as planned until Milestone 9.
	executableActions = []Option{
		{Value: "check_node_health", Label: "Check node health"},
		{Value: "check_node_freshness", Label: "Check node report freshness"},
	}
	plannedActions = []PlannedAction{
		{Value: "wake_node", Label: "Wake node (Wake-on-LAN)", Milestone: "Milestone 9"},
		{Value: "drain_node", Label: "Drain node", Milestone: "Milestone 9"},
		{Value: "restart_service", Label: "Restart service or container", Milestone: "Milestone 9"},
		{Value: "run_backup", Label: "Run backup", Milestone: "Milestone 9"},
	}
	executableChannels = []Option{{Value: "activity", Label: "Activity log"}}
)

// Inventory is the read-only view of nodes the runner and scheduler need.
type Inventory interface {
	Snapshot(ctx context.Context, now time.Time) (domain.Snapshot, error)
}

// BuildOptions lists what this server can execute plus the current nodes.
func BuildOptions(snapshot domain.Snapshot) Options {
	targets := make([]Target, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		targets = append(targets, Target{ID: node.ID, Label: node.Name})
	}
	return Options{
		ActionTypes:    append([]Option(nil), executableActions...),
		Targets:        targets,
		Channels:       append([]Option(nil), executableChannels...),
		PlannedActions: append([]PlannedAction(nil), plannedActions...),
	}
}

// Event is published to SSE subscribers when a run changes state.
type Event struct {
	Type       string `json:"type"`
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	State      string `json:"state"`
}

// Hub fans events out to SSE connections. Slow subscribers drop events; the
// payload is only a refetch hint.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	if h.subs == nil {
		h.subs = map[chan Event]struct{}{}
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

func (h *Hub) Publish(event Event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

// Runner executes workflows in the control-plane process.
type Runner struct {
	Store       Store
	Inventory   Inventory
	Hub         *Hub
	Log         *slog.Logger
	BaseContext context.Context
	StepTimeout time.Duration
	RunTimeout  time.Duration
	wg          sync.WaitGroup
}

func (r *Runner) base() context.Context {
	if r.BaseContext != nil {
		return r.BaseContext
	}
	return context.Background()
}

func (r *Runner) stepTimeout() time.Duration {
	if r.StepTimeout > 0 {
		return r.StepTimeout
	}
	return 30 * time.Second
}

func (r *Runner) runTimeout() time.Duration {
	if r.RunTimeout > 0 {
		return r.RunTimeout
	}
	return 5 * time.Minute
}

// Wait blocks until in-flight runs finish. Used by tests and shutdown.
func (r *Runner) Wait() { r.wg.Wait() }

// Start records a queued run and executes it in the background. It returns
// ErrNotFound, ErrDisabled or ErrRunBusy without side effects when the run
// cannot begin.
func (r *Runner) Start(ctx context.Context, workflowID, trigger, actor string) (Run, error) {
	wf, err := r.Store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return Run{}, err
	}
	if !wf.Enabled {
		return Run{}, ErrDisabled
	}
	run, err := r.Store.CreateRun(ctx, wf, trigger, actor)
	if err != nil {
		return Run{}, err
	}
	r.Hub.Publish(Event{Type: "workflow_run.updated", WorkflowID: wf.ID, RunID: run.ID, State: run.State})
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.execute(run, wf)
	}()
	return run, nil
}

func (r *Runner) log(msg string, args ...any) {
	if r.Log != nil {
		r.Log.Warn(msg, args...)
	}
}

func (r *Runner) execute(run Run, wf Workflow) {
	// Bookkeeping must complete even if the server is shutting down.
	store := context.WithoutCancel(r.base())
	ctx, cancel := context.WithTimeout(r.base(), r.runTimeout())
	defer cancel()
	publish := func(state string) {
		r.Hub.Publish(Event{Type: "workflow_run.updated", WorkflowID: wf.ID, RunID: run.ID, State: state})
	}
	if err := r.Store.MarkRunRunning(store, run.ID); err != nil {
		r.log("workflow run could not start", "run", run.ID, "error", err)
	}
	publish(RunRunning)

	var snapshot *domain.Snapshot
	getSnapshot := func(stepCtx context.Context) (domain.Snapshot, error) {
		if snapshot != nil {
			return *snapshot, nil
		}
		loaded, err := r.Inventory.Snapshot(stepCtx, r.Store.now())
		if err != nil {
			return domain.Snapshot{}, err
		}
		snapshot = &loaded
		return loaded, nil
	}

	failedSteps, succeeded := 0, 0
	firstCode, firstMessage := "", ""
	stop := false
	for i, step := range wf.Steps {
		if stop {
			_ = r.Store.UpdateStep(store, run.ID, i, StepSkipped, nil, nil, "Skipped because an earlier step failed.", "")
			continue
		}
		started := r.Store.now()
		_ = r.Store.UpdateStep(store, run.ID, i, StepRunning, &started, nil, "", "")
		stepCtx, stepCancel := context.WithTimeout(ctx, r.stepTimeout())
		output, code := r.executeStep(stepCtx, wf, step, getSnapshot)
		stepCancel()
		finished := r.Store.now()
		if code != "" {
			failedSteps++
			if firstCode == "" {
				firstCode, firstMessage = code, fmt.Sprintf("Step %d (%s) failed: %s", i+1, step.Name, output)
			}
			_ = r.Store.UpdateStep(store, run.ID, i, StepFailed, nil, &finished, output, code)
			if wf.OnFailure.Behavior != OnFailureContinue {
				stop = true
			}
		} else {
			succeeded++
			_ = r.Store.UpdateStep(store, run.ID, i, StepSucceeded, nil, &finished, output, "")
		}
		publish(RunRunning)
	}

	state := RunSucceeded
	summary := fmt.Sprintf("%d of %d steps succeeded.", succeeded, len(wf.Steps))
	if failedSteps > 0 {
		state = RunFailed
		summary = firstMessage
	}
	channel, label := wf.OnSuccess.NotifyChannel, "Success notification"
	message := fmt.Sprintf("Workflow %q succeeded.", wf.Name)
	if state == RunFailed {
		channel, label = wf.OnFailure.NotifyChannel, "Failure notification"
		message = fmt.Sprintf("Workflow %q failed. %s", wf.Name, firstMessage)
	}
	if channel != "" {
		position := len(wf.Steps)
		if err := r.Store.AddOutcomeStep(store, run.ID, position, label); err == nil {
			started := r.Store.now()
			output, code := r.notify(store, wf, run.ID, channel, message)
			finished := r.Store.now()
			outcome := StepSucceeded
			if code != "" {
				outcome = StepFailed
			}
			_ = r.Store.UpdateStep(store, run.ID, position, outcome, &started, &finished, output, code)
		}
	}
	if ctx.Err() != nil && state == RunSucceeded {
		state, firstCode, summary = RunFailed, "TIMEOUT", "The run exceeded its time limit."
	}
	if _, err := r.Store.FinishRun(store, run, state, firstCode, summary); err != nil {
		r.log("workflow run could not be finalised", "run", run.ID, "error", err)
	}
	publish(state)
}

// executeStep returns the step output and an error code; an empty code means
// success.
func (r *Runner) executeStep(ctx context.Context, wf Workflow, step Step, snapshot func(context.Context) (domain.Snapshot, error)) (string, string) {
	if err := ctx.Err(); err != nil {
		return "The step did not start before its time limit.", "TIMEOUT"
	}
	switch step.Type {
	case StepNotify:
		return r.notify(ctx, wf, "", step.Channel, step.Message)
	case StepAction:
		switch step.ActionType {
		case "check_node_health", "check_node_freshness":
			inventory, err := snapshot(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return "The step exceeded its time limit.", "TIMEOUT"
				}
				return "Inventory is unavailable.", "INVENTORY_UNAVAILABLE"
			}
			return checkNode(inventory, step.ActionType, step.TargetID, r.Store.now())
		default:
			return "This action is not available in this version.", "ACTION_NOT_SUPPORTED"
		}
	}
	return "Unknown step type.", "INVALID_STEP"
}

func checkNode(inventory domain.Snapshot, action, targetID string, now time.Time) (string, string) {
	for _, node := range inventory.Nodes {
		if node.ID != targetID {
			continue
		}
		freshness := node.Health.Freshness(now)
		age := "never observed"
		if !node.Health.ObservedAt.IsZero() {
			age = "observed " + now.Sub(node.Health.ObservedAt).Round(time.Second).String() + " ago"
		}
		detail := fmt.Sprintf("%s: %s, %s (%s).", node.Name, node.Health.State, freshness, age)
		if action == "check_node_freshness" {
			if freshness != "fresh" {
				return detail, "CHECK_FAILED"
			}
			return detail, ""
		}
		if freshness != "fresh" || node.Health.State != "healthy" {
			return detail, "CHECK_FAILED"
		}
		return detail, ""
	}
	return "The target node is not in inventory.", "NODE_NOT_FOUND"
}

// notify records a notification on the only supported channel, the activity
// log, as an audit event. runID is empty for ordered notify steps; the audit
// detail still names the workflow.
func (r *Runner) notify(ctx context.Context, wf Workflow, runID, channel, message string) (string, string) {
	if channel != "activity" {
		return "This notification channel is not available.", "CHANNEL_UNAVAILABLE"
	}
	detail := map[string]any{"workflow": wf.Name, "channel": channel, "message": message}
	if runID != "" {
		detail["run_id"] = runID
	}
	if err := r.Store.Audit(ctx, "workflow:"+wf.ID, "workflow.notification", "workflow", wf.ID, "succeeded", detail); err != nil {
		return "The notification could not be recorded.", "NOTIFY_FAILED"
	}
	return message, ""
}

package workflow

import "context"

// DemoDefinitions are the simulated workflows shown in demo mode. They only
// use read-only checks against the demo inventory.
func DemoDefinitions() []Definition {
	return []Definition{
		{
			Name:        "Health check sweep",
			Description: "Check the demo gateway and worker every 15 minutes",
			Enabled:     true,
			Trigger:     Trigger{Type: TriggerCron, Expression: "*/15 * * * *"},
			Steps: []Step{
				{ID: "check-gateway", Type: StepAction, Name: "Check gateway health", ActionType: "check_node_health", TargetID: "demo-gateway"},
				{ID: "check-worker", Type: StepAction, Name: "Check worker health", ActionType: "check_node_health", TargetID: "demo-worker"},
			},
			OnSuccess: OnSuccess{NotifyChannel: "activity"},
			OnFailure: OnFailure{Behavior: OnFailureContinue, NotifyChannel: "activity"},
		},
		{
			Name:        "Check worker on demand",
			Description: "Manually confirm the demo worker is reporting",
			Enabled:     true,
			Trigger:     Trigger{Type: TriggerManual},
			Steps: []Step{
				{ID: "worker-fresh", Type: StepAction, Name: "Check worker report freshness", ActionType: "check_node_freshness", TargetID: "demo-worker"},
				{ID: "worker-note", Type: StepNotify, Name: "Record result", Channel: "activity", Message: "Worker freshness check finished."},
			},
			OnSuccess: OnSuccess{},
			OnFailure: OnFailure{Behavior: OnFailureStop, NotifyChannel: "activity"},
		},
		{
			Name:        "Stale worker alert",
			Description: "Record an alert when the demo worker report goes stale",
			Enabled:     false,
			Trigger:     Trigger{Type: TriggerHealthCondition, TargetID: "demo-worker", Condition: "stale", ForSeconds: 60},
			Steps: []Step{
				{ID: "alert", Type: StepNotify, Name: "Record alert", Channel: "activity", Message: "The demo worker report is stale."},
			},
			OnSuccess: OnSuccess{},
			OnFailure: OnFailure{Behavior: OnFailureStop},
		},
	}
}

// SeedDemo creates the simulated workflows when no workflows exist yet.
func SeedDemo(ctx context.Context, store Store) error {
	count, err := store.CountWorkflows(ctx)
	if err != nil || count > 0 {
		return err
	}
	for _, def := range DemoDefinitions() {
		if _, err := store.CreateWorkflow(ctx, def, true, "system"); err != nil {
			return err
		}
	}
	return nil
}

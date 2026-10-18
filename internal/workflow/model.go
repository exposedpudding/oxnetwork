// Package workflow implements Milestone 4: workflow definitions, a runner for
// read-only actions, a trigger scheduler and an append-only audit trail.
// See docs/DECISIONS.md ADR-013.
package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	TriggerManual          = "manual"
	TriggerCron            = "cron"
	TriggerHealthCondition = "health_condition"

	StepAction = "action"
	StepNotify = "notify"

	RunQueued    = "queued"
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCancelled = "cancelled"

	StepPending   = "pending"
	StepRunning   = "running"
	StepSucceeded = "succeeded"
	StepFailed    = "failed"
	StepSkipped   = "skipped"

	OnFailureStop     = "stop"
	OnFailureContinue = "continue"

	MaxWorkflows   = 100
	MaxSteps       = 20
	MaxNameLength  = 80
	MaxDescription = 240
	MaxMessage     = 500
)

// Trigger is a tagged union serialised exactly as the frontend expects.
type Trigger struct {
	Type       string `json:"type"`
	Expression string `json:"expression,omitempty"`
	TargetID   string `json:"target_id,omitempty"`
	Condition  string `json:"condition,omitempty"`
	ForSeconds int    `json:"for_seconds"`
}

func (t Trigger) MarshalJSON() ([]byte, error) {
	switch t.Type {
	case TriggerCron:
		return json.Marshal(struct {
			Type       string `json:"type"`
			Expression string `json:"expression"`
		}{t.Type, t.Expression})
	case TriggerHealthCondition:
		return json.Marshal(struct {
			Type       string `json:"type"`
			TargetID   string `json:"target_id"`
			Condition  string `json:"condition"`
			ForSeconds int    `json:"for_seconds"`
		}{t.Type, t.TargetID, t.Condition, t.ForSeconds})
	default:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{t.Type})
	}
}

// Step is a tagged union: action steps use ActionType and TargetID, notify
// steps use Channel and Message.
type Step struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	ActionType string `json:"action_type,omitempty"`
	TargetID   string `json:"target_id,omitempty"`
	Channel    string `json:"channel,omitempty"`
	Message    string `json:"message,omitempty"`
}

type OnSuccess struct {
	NotifyChannel string `json:"notify_channel,omitempty"`
}

type OnFailure struct {
	Behavior      string `json:"behavior"`
	NotifyChannel string `json:"notify_channel,omitempty"`
}

// Definition is the user-editable part of a workflow.
type Definition struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	Trigger     Trigger   `json:"trigger"`
	Steps       []Step    `json:"steps"`
	OnSuccess   OnSuccess `json:"on_success"`
	OnFailure   OnFailure `json:"on_failure"`
}

// Workflow is a stored definition plus server-managed fields.
type Workflow struct {
	ID string `json:"id"`
	Definition
	Simulated bool        `json:"simulated"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	LastRun   *RunSummary `json:"last_run"`
	NextRunAt *time.Time  `json:"next_run_at"`
}

// RunSummary is the compact run view embedded in workflow lists.
type RunSummary struct {
	ID         string     `json:"id"`
	State      string     `json:"state"`
	Trigger    string     `json:"trigger"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	DurationMS *int64     `json:"duration_ms"`
}

type RunStep struct {
	Position   int        `json:"position"`
	StepID     string     `json:"step_id"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	State      string     `json:"state"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Output     string     `json:"output"`
	ErrorCode  string     `json:"error_code,omitempty"`
}

type Run struct {
	ID           string     `json:"id"`
	WorkflowID   string     `json:"workflow_id"`
	WorkflowName string     `json:"workflow_name"`
	Trigger      string     `json:"trigger"`
	Actor        string     `json:"actor"`
	State        string     `json:"state"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	DurationMS   *int64     `json:"duration_ms"`
	ErrorCode    string     `json:"error_code,omitempty"`
	Summary      string     `json:"summary"`
	Simulated    bool       `json:"simulated"`
	Steps        []RunStep  `json:"steps,omitempty"`
}

// Option lists back both validation and the editor drop-downs.
type Option struct {
	Value     string `json:"value"`
	Label     string `json:"label"`
	Dangerous bool   `json:"dangerous,omitempty"`
}

type Target struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type PlannedAction struct {
	Value     string `json:"value"`
	Label     string `json:"label"`
	Milestone string `json:"milestone"`
}

// Options is what the server can currently execute and notify.
type Options struct {
	ActionTypes    []Option        `json:"action_types"`
	Targets        []Target        `json:"targets"`
	Channels       []Option        `json:"channels"`
	PlannedActions []PlannedAction `json:"planned_actions"`
}

// ValidationError carries per-field messages using the same keys as the
// frontend editor ("name", "trigger", "steps", "step.0.action", ...).
type ValidationError struct {
	Code   string
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for key := range e.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+e.Fields[key])
	}
	return strings.Join(parts, "; ")
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Validate checks a definition against what the server can execute. A
// workflow that names an action the runner cannot perform is rejected rather
// than saved and left to fail at run time.
func Validate(d *Definition, opts Options) error {
	fields := map[string]string{}
	code := "INVALID_WORKFLOW"
	d.Name = strings.TrimSpace(d.Name)
	d.Description = strings.TrimSpace(d.Description)
	if d.Name == "" {
		fields["name"] = "Name is required."
	} else if len([]rune(d.Name)) > MaxNameLength {
		fields["name"] = fmt.Sprintf("Name must be %d characters or fewer.", MaxNameLength)
	}
	if len([]rune(d.Description)) > MaxDescription {
		fields["description"] = fmt.Sprintf("Description must be %d characters or fewer.", MaxDescription)
	}
	targets := map[string]bool{}
	for _, target := range opts.Targets {
		targets[target.ID] = true
	}
	actions := map[string]bool{}
	for _, action := range opts.ActionTypes {
		actions[action.Value] = true
	}
	planned := map[string]bool{}
	for _, action := range opts.PlannedActions {
		planned[action.Value] = true
	}
	channels := map[string]bool{}
	for _, channel := range opts.Channels {
		channels[channel.Value] = true
	}
	switch d.Trigger.Type {
	case TriggerManual:
		d.Trigger = Trigger{Type: TriggerManual}
	case TriggerCron:
		d.Trigger = Trigger{Type: TriggerCron, Expression: strings.TrimSpace(d.Trigger.Expression)}
		if _, err := ParseCron(d.Trigger.Expression); err != nil {
			fields["trigger"] = err.Error()
		}
	case TriggerHealthCondition:
		t := d.Trigger
		d.Trigger = Trigger{Type: TriggerHealthCondition, TargetID: t.TargetID, Condition: t.Condition, ForSeconds: t.ForSeconds}
		switch {
		case !targets[t.TargetID]:
			fields["trigger"] = "Choose a known node to watch."
		case t.Condition != "unhealthy" && t.Condition != "degraded" && t.Condition != "stale":
			fields["trigger"] = "Condition must be unhealthy, degraded or stale."
		case t.ForSeconds < 0 || t.ForSeconds > 86400:
			fields["trigger"] = "Duration must be between 0 and 86400 seconds."
		}
	default:
		fields["trigger"] = "Trigger type must be manual, cron or health_condition."
	}
	if len(d.Steps) == 0 {
		fields["steps"] = "Add at least one step."
	}
	if len(d.Steps) > MaxSteps {
		fields["steps"] = fmt.Sprintf("A workflow can have at most %d steps.", MaxSteps)
	}
	seen := map[string]bool{}
	for i := range d.Steps {
		s := &d.Steps[i]
		key := func(field string) string { return fmt.Sprintf("step.%d.%s", i, field) }
		s.Name = strings.TrimSpace(s.Name)
		if !identifierPattern.MatchString(s.ID) {
			fields[key("id")] = "Step id must be 1-64 letters, digits, dashes or underscores."
		} else if seen[s.ID] {
			fields[key("id")] = "Step ids must be unique."
		}
		seen[s.ID] = true
		if s.Name == "" {
			fields[key("name")] = "Step name is required."
		} else if len([]rune(s.Name)) > MaxNameLength {
			fields[key("name")] = fmt.Sprintf("Step name must be %d characters or fewer.", MaxNameLength)
		}
		switch s.Type {
		case StepAction:
			s.Channel, s.Message = "", ""
			switch {
			case planned[s.ActionType]:
				fields[key("action")] = "This action is not available yet."
				code = "ACTION_NOT_SUPPORTED"
			case !actions[s.ActionType]:
				fields[key("action")] = "Choose an action."
			}
			if !targets[s.TargetID] {
				fields[key("target")] = "Choose a known target."
			}
		case StepNotify:
			s.ActionType, s.TargetID = "", ""
			s.Message = strings.TrimSpace(s.Message)
			if !channels[s.Channel] {
				fields[key("channel")] = "Choose a channel."
			}
			if s.Message == "" {
				fields[key("message")] = "Message is required."
			} else if len([]rune(s.Message)) > MaxMessage {
				fields[key("message")] = fmt.Sprintf("Message must be %d characters or fewer.", MaxMessage)
			}
		default:
			fields[key("type")] = "Step type must be action or notify."
		}
	}
	if d.OnSuccess.NotifyChannel != "" && !channels[d.OnSuccess.NotifyChannel] {
		fields["on_success"] = "Unknown notification channel."
	}
	switch d.OnFailure.Behavior {
	case OnFailureStop, OnFailureContinue:
	case "":
		d.OnFailure.Behavior = OnFailureStop
	default:
		fields["on_failure"] = "Failure behaviour must be stop or continue."
	}
	if d.OnFailure.NotifyChannel != "" && !channels[d.OnFailure.NotifyChannel] {
		fields["on_failure"] = "Unknown notification channel."
	}
	if len(fields) > 0 {
		return &ValidationError{Code: code, Fields: fields}
	}
	return nil
}

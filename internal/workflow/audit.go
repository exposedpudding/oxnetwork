package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// AuditEvent is an immutable record of an action or workflow run (FR-005).
// Detail holds sanitised parameters only: never tokens or secrets.
type AuditEvent struct {
	ID         int64          `json:"id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Outcome    string         `json:"outcome"`
	Detail     map[string]any `json:"detail"`
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func appendAudit(ctx context.Context, db execer, at time.Time, actor, action, targetType, targetID, outcome string, detail map[string]any) error {
	if actor == "" {
		actor = defaultActor
	}
	if detail == nil {
		detail = map[string]any{}
	}
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO audit_events(occurred_at,actor,action,target_type,target_id,outcome,detail_json) VALUES(?,?,?,?,?,?,?)`,
		formatTime(at), actor, action, targetType, targetID, outcome, string(body))
	return err
}

// Audit appends a standalone event.
func (s Store) Audit(ctx context.Context, actor, action, targetType, targetID, outcome string, detail map[string]any) error {
	return appendAudit(ctx, s.DB, s.now(), actor, action, targetType, targetID, outcome, detail)
}

// ListAudit returns events newest first. beforeID, when positive, returns only
// events with a smaller id (cursor pagination). It fetches limit+1 rows so the
// caller can tell whether another page exists.
func (s Store) ListAudit(ctx context.Context, limit int, beforeID int64, targetType, targetID string) ([]AuditEvent, bool, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id,occurred_at,actor,action,target_type,target_id,outcome,detail_json FROM audit_events WHERE 1=1`
	args := []any{}
	if beforeID > 0 {
		query += ` AND id<?`
		args = append(args, beforeID)
	}
	if targetType != "" {
		query += ` AND target_type=?`
		args = append(args, targetType)
	}
	if targetID != "" {
		query += ` AND target_id=?`
		args = append(args, targetID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	events := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		var occurred, detail string
		if err := rows.Scan(&event.ID, &occurred, &event.Actor, &event.Action, &event.TargetType, &event.TargetID, &event.Outcome, &detail); err != nil {
			return nil, false, err
		}
		event.OccurredAt = parseTime(occurred)
		if err := json.Unmarshal([]byte(detail), &event.Detail); err != nil || event.Detail == nil {
			event.Detail = map[string]any{}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(events) > limit
	if more {
		events = events[:limit]
	}
	return events, more, nil
}

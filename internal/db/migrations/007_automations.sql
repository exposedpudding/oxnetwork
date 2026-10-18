CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL,
    definition_json TEXT NOT NULL,
    simulated INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_runs (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    workflow_name TEXT NOT NULL,
    trigger_source TEXT NOT NULL,
    actor TEXT NOT NULL,
    state TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    error_code TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    simulated INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS workflow_runs_workflow_started ON workflow_runs(workflow_id, started_at DESC);
CREATE INDEX IF NOT EXISTS workflow_runs_started ON workflow_runs(started_at DESC);

CREATE TABLE IF NOT EXISTS workflow_run_steps (
    run_id TEXT NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    step_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    state TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT,
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (run_id, position)
);

-- Migration 001 created an unused placeholder audit_events table with an
-- incompatible shape (TEXT id, single target column). Nothing ever wrote to it,
-- so keep it under a new name instead of dropping it.
ALTER TABLE audit_events RENAME TO audit_events_v1_unused;

CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    outcome TEXT NOT NULL,
    detail_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS audit_events_target ON audit_events(target_type, target_id, id DESC);

CREATE TRIGGER IF NOT EXISTS audit_events_no_update BEFORE UPDATE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit events are append-only');
END;

CREATE TRIGGER IF NOT EXISTS audit_events_no_delete BEFORE DELETE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit events are append-only');
END;

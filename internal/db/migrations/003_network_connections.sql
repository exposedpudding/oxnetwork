CREATE TABLE IF NOT EXISTS network_connections (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    observed_at TEXT NOT NULL,
    stale_after_seconds INTEGER NOT NULL,
    source_system TEXT NOT NULL,
    simulated INTEGER NOT NULL
);

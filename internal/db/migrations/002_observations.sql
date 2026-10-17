CREATE TABLE IF NOT EXISTS node_observations (
    node_id TEXT PRIMARY KEY REFERENCES inventory_nodes(id) ON DELETE CASCADE,
    health_state TEXT NOT NULL,
    observed_at TEXT NOT NULL,
    stale_after_seconds INTEGER NOT NULL,
    source TEXT NOT NULL,
    cpu_percent REAL,
    ram_percent REAL,
    vcpu INTEGER,
    ram_gb INTEGER,
    simulated INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS node_capabilities (
    node_id TEXT NOT NULL REFERENCES inventory_nodes(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    PRIMARY KEY (node_id, capability)
);

CREATE TABLE IF NOT EXISTS service_observations (
    service_id TEXT PRIMARY KEY REFERENCES inventory_services(id) ON DELETE CASCADE,
    health_state TEXT NOT NULL,
    observed_at TEXT NOT NULL,
    stale_after_seconds INTEGER NOT NULL,
    source TEXT NOT NULL,
    simulated INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS activity_events (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    detail TEXT NOT NULL,
    state TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    simulated INTEGER NOT NULL
);

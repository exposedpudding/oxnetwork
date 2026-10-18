CREATE TABLE host_observations (
    node_id TEXT PRIMARY KEY,
    received_at TEXT NOT NULL,
    cpu_percent REAL NOT NULL,
    ram_percent REAL NOT NULL,
    disk_percent REAL NOT NULL,
    vcpu INTEGER NOT NULL,
    ram_gb INTEGER NOT NULL
);

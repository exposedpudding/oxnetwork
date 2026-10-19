-- Container inventory reported by an opt-in Docker reporter (ADR-019). One report row
-- per node says when the node last reported (so "zero containers" differs from "never
-- reported"); node_containers holds that node's latest list and is replaced whole.
CREATE TABLE IF NOT EXISTS container_reports (
    node_id TEXT PRIMARY KEY,
    received_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS node_containers (
    node_id TEXT NOT NULL,
    container_id TEXT NOT NULL,
    name TEXT NOT NULL,
    image TEXT NOT NULL,
    state TEXT NOT NULL,
    status TEXT NOT NULL,
    health TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    ports_json TEXT NOT NULL DEFAULT '[]',
    compose_project TEXT NOT NULL DEFAULT '',
    compose_service TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (node_id, container_id)
);

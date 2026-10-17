ALTER TABLE inventory_nodes ADD COLUMN addresses_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE inventory_nodes ADD COLUMN last_seen_at TEXT;

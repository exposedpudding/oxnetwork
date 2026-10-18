package inventory

import (
	"context"
	"database/sql"
	"time"
)

// RefreshedAt reports when the inventory source last stored a snapshot: every
// successful refresh rewrites each node's updated_at. The second result is false
// when no node has ever been stored. Host-agent reports do not move this value.
func (r Repository) RefreshedAt(ctx context.Context) (time.Time, bool, error) {
	var latest sql.NullString
	if err := r.DB.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM inventory_nodes`).Scan(&latest); err != nil {
		return time.Time{}, false, err
	}
	if !latest.Valid || latest.String == "" {
		return time.Time{}, false, nil
	}
	at, err := time.Parse(time.RFC3339Nano, latest.String)
	if err != nil {
		return time.Time{}, false, err
	}
	return at.UTC(), true, nil
}

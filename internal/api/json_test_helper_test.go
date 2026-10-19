package api

import (
	"context"
	"encoding/json"
)

// jsonUnmarshal keeps the integration tests free of repeated imports.
func jsonUnmarshal(data []byte, target any) error { return json.Unmarshal(data, target) }

// nil2ctx returns a background context for tests that never cancel.
func nil2ctx() context.Context { return context.Background() }

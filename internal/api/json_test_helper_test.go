package api

import "encoding/json"

// jsonUnmarshal keeps the integration tests free of repeated imports.
func jsonUnmarshal(data []byte, target any) error { return json.Unmarshal(data, target) }

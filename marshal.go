package workflow

import (
	"encoding/json"
)

// Marshal create a single point of change if the encoding changes.
func Marshal[T any](t *T) ([]byte, error) {
	return json.Marshal(t)
}

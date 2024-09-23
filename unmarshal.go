package workflow

import "encoding/json"

// Unmarshal create a single point of change if the decoding changes.
func Unmarshal[T any](b []byte, t *T) error {
	return json.Unmarshal(b, t)
}

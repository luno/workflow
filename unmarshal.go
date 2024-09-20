package workflow

import "encoding/json"

// Unmarshal create a single point of change if the decoding changes.
func Unmarshal[T any](b []byte, t *T) error {
	err := json.Unmarshal(b, t)
	if err != nil {
		return err
	}

	return nil
}

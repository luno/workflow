package workflow

import "encoding/json"

func Unmarshal[T any](b []byte, t *T) error {
	err := json.Unmarshal(b, t)
	if err != nil {
		return err
	}

	return nil
}

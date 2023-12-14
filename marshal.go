package workflow

import (
	"encoding/json"
)

func Marshal[T any](t *T) ([]byte, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	return b, nil
}

package workflow

import (
	"context"
	"strings"
)

type RoleScheduler interface {
	Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error)
}

func makeRole(inputs ...string) string {
	joined := strings.Join(inputs, "-")
	lowered := strings.ToLower(joined)
	filled := strings.Replace(lowered, " ", "_", -1)
	return filled
}

package workflow

import (
	"context"
	"strings"
)

// RoleScheduler implementations should all be tested with adaptertest.TestRoleScheduler
type RoleScheduler interface {
	// Await must return a child context of the provided (parent) context. Await should block until the role is
	// assigned to the caller. Only one caller should be able to be assigned the role at any given time. The returned
	// context.CancelFunc is called after each process execution. Some process executions can be more long living and
	// others not but if any process errors the context.CancelFunc will be called after the specified error backoff
	// has finished.
	Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error)
}

func makeRole(inputs ...string) string {
	joined := strings.Join(inputs, "-")
	lowered := strings.ToLower(joined)
	filled := strings.Replace(lowered, " ", "_", -1)
	return filled
}

package rinkrolescheduler

import (
	"context"

	"github.com/luno/rink/v2"
)

func New(r *rink.Rink) *RoleScheduler {
	return &RoleScheduler{
		rink: r,
	}
}

type RoleScheduler struct {
	rink *rink.Rink
}

func (r *RoleScheduler) Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
	return r.rink.Roles.AwaitRoleContext(ctx, role)
}

func (r *RoleScheduler) Close() error {
	r.rink.Shutdown(context.Background())
	return nil
}

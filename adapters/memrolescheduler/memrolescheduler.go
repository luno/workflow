package memrolescheduler

import (
	"context"
	"sync"
)

type RoleScheduler struct {
	mu    sync.Mutex
	roles map[string]*sync.Mutex
}

func (r *RoleScheduler) Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	ctx, cancel := context.WithCancel(ctx)

	// Lock the main mutex whilst checking and potentially creating new role mutexes
	r.mu.Lock()
	mu, ok := r.roles[role]
	if !ok {
		mu = &sync.Mutex{}
		r.roles[role] = mu
	}
	r.mu.Unlock()

	mu.Lock()

	go func(role string) {
		<-ctx.Done()
		r.mu.Lock()
		r.roles[role].Unlock()
		r.mu.Unlock()
	}(role)

	return ctx, cancel, nil
}

func New() *RoleScheduler {
	return &RoleScheduler{
		roles: make(map[string]*sync.Mutex),
	}
}
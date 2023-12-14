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
	ctx2, cancel := context.WithCancel(ctx)

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
		for {
			select {
			case <-ctx2.Done():
				r.roles[role].Unlock()
				return
			case <-ctx.Done():
				r.roles[role].Unlock()
				return
			}
		}
	}(role)

	return ctx2, cancel, nil
}

func New() *RoleScheduler {
	return &RoleScheduler{
		roles: make(map[string]*sync.Mutex),
	}
}

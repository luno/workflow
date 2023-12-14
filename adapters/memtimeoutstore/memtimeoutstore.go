package memtimeoutstore

import (
	"context"
	"sync"
	"time"

	"k8s.io/utils/clock"

	"github.com/andrewwormald/workflow"
)

func New(opts ...Option) *Store {
	s := &Store{
		clock: clock.RealClock{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type Option func(s *Store)

func WithClock(c clock.Clock) Option {
	return func(s *Store) {
		s.clock = c
	}
}

var _ workflow.TimeoutStore = (*Store)(nil)

type Store struct {
	clock clock.Clock

	mu                 sync.Mutex
	timeoutIdIncrement int64
	timeouts           []*workflow.Timeout
}

func (s *Store) List(ctx context.Context, workflowName string) ([]workflow.Timeout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ls []workflow.Timeout
	for _, timeout := range s.timeouts {
		if timeout.WorkflowName != workflowName {
			continue
		}

		ls = append(ls, *timeout)
	}

	return ls, nil
}

func (s *Store) Create(ctx context.Context, workflowName, foreignID, runID string, status int, expireAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.timeouts = append(s.timeouts, &workflow.Timeout{
		ID:           s.timeoutIdIncrement,
		WorkflowName: workflowName,
		ForeignID:    foreignID,
		RunID:        runID,
		Status:       status,
		ExpireAt:     expireAt,
		CreatedAt:    s.clock.Now(),
	})
	s.timeoutIdIncrement++

	return nil
}

func (s *Store) Complete(ctx context.Context, workflowName, foreignID, runID string, status int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, timeout := range s.timeouts {
		if timeout.WorkflowName != workflowName {
			continue
		}

		if timeout.ForeignID != foreignID {
			continue
		}

		if timeout.RunID != runID {
			continue
		}

		if timeout.Status != status {
			continue
		}

		s.timeouts[i].Completed = true
		break
	}

	return nil
}

func (s *Store) Cancel(ctx context.Context, workflowName, foreignID, runID string, status int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var index int
	for i, timeout := range s.timeouts {
		if timeout.WorkflowName != workflowName {
			continue
		}

		if timeout.ForeignID != foreignID {
			continue
		}

		if timeout.RunID != runID {
			continue
		}

		if timeout.Status != status {
			continue
		}

		index = i
		break
	}

	left := s.timeouts[:index]
	right := s.timeouts[index+1 : len(s.timeouts)]
	s.timeouts = append(left, right...)
	return nil
}

func (s *Store) ListValid(ctx context.Context, workflowName string, status int, now time.Time) ([]workflow.Timeout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var valid []workflow.Timeout
	for _, timeout := range s.timeouts {
		if timeout.WorkflowName != workflowName {
			continue
		}

		if timeout.Status != status {
			continue
		}

		if timeout.Completed {
			continue
		}

		if timeout.ExpireAt.After(now) {
			continue
		}

		valid = append(valid, *timeout)
	}

	return valid, nil
}

package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/graph"
)

func TestUpdater(t *testing.T) {
	testCases := []struct {
		name             string
		lookup           lookupFunc
		current          testStatus
		update           Run[string, testStatus]
		transitions      []graph.Transition
		expectedRunState RunState
		expectedErr      error
	}{
		{
			name: "Golden path",
			lookup: func(ctx context.Context, id int64) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				Record: Record{
					RunState: RunStateRunning,
					Status:   int(statusMiddle),
				},
				Status: statusMiddle,
			},
			transitions: []graph.Transition{
				{
					From: int(statusStart),
					To:   int(statusMiddle),
				},
				{
					From: int(statusMiddle),
					To:   int(statusEnd),
				},
			},
			expectedRunState: RunStateRunning,
		},
		{
			name: "No transitions - error",
			lookup: func(ctx context.Context, id int64) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				Record: Record{
					RunState: RunStateRunning,
					Status:   int(statusMiddle),
				},
				Status: statusMiddle,
			},
			transitions: []graph.Transition{},
			expectedErr: errors.New("current status not predefined"),
		},
		{
			name: "Mark as completed",
			lookup: func(ctx context.Context, id int64) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				Record: Record{
					RunState: RunStateRunning,
					Status:   int(statusMiddle),
				},
				Status: statusMiddle,
			},
			transitions: []graph.Transition{
				{
					From: int(statusStart),
					To:   int(statusMiddle),
				},
			},
			expectedRunState: RunStateCompleted,
		},
		{
			name: "Return error on lookup",
			lookup: func(ctx context.Context, id int64) (*Record, error) {
				return nil, errors.New("lookup error")
			},
			expectedErr: errors.New("lookup error"),
		},
		{
			name: "Exit early if lookup record status has changed",
			lookup: func(ctx context.Context, id int64) (*Record, error) {
				return &Record{
					Status: int(statusEnd),
				}, nil
			},
			current:     statusMiddle,
			expectedErr: nil,
		},
		{
			name: "No valid transition available",
			lookup: func(ctx context.Context, id int64) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				Record: Record{
					RunState: RunStateRunning,
					Status:   int(statusMiddle),
				},
				Status: statusMiddle,
			},
			transitions: []graph.Transition{
				{
					From: int(statusStart),
					To:   int(statusEnd),
				},
			},
			expectedErr: errors.New("invalid transition attempted"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g := graph.New()
			for _, transition := range tc.transitions {
				g.AddTransition(transition.From, transition.To)
			}
			c := clock_testing.NewFakeClock(time.Now())

			store := func(ctx context.Context, r *Record, maker OutboxEventDataMaker) error {
				require.Equal(t, tc.expectedRunState, r.RunState)
				_, err := maker(1)
				jtest.RequireNil(t, err)
				return nil
			}

			updater := newUpdater[string, testStatus](tc.lookup, store, g, c)
			err := updater(ctx, tc.current, tc.update.Status, &tc.update)
			jtest.Require(t, tc.expectedErr, err)
		})
	}
}

package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/graph"
)

func TestUpdater(t *testing.T) {
	testErr := errors.New("lookup error")
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
			lookup: func(context.Context, string) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				TypedRecord: TypedRecord[string, testStatus]{
					Record: Record{
						RunState: RunStateRunning,
						Status:   int(statusMiddle),
					},
					Status: statusMiddle,
				},
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
			lookup: func(context.Context, string) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				TypedRecord: TypedRecord[string, testStatus]{
					Record: Record{
						RunState: RunStateRunning,
						Status:   int(statusMiddle),
					},
					Status: statusMiddle,
				},
			},
			transitions: []graph.Transition{},
			expectedErr: fmt.Errorf("current status not defined in graph: current=%s", statusStart),
		},
		{
			name: "Mark as completed",
			lookup: func(context.Context, string) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				TypedRecord: TypedRecord[string, testStatus]{
					Record: Record{
						RunState: RunStateRunning,
						Status:   int(statusMiddle),
					},
					Status: statusMiddle,
				},
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
			lookup: func(context.Context, string) (*Record, error) {
				return nil, testErr
			},
			expectedErr: testErr,
		},
		{
			name: "Record version has changed - error and retry",
			lookup: func(context.Context, string) (*Record, error) {
				return &Record{
					Meta: Meta{
						Version: 1,
					},
				}, nil
			},
			current:     statusMiddle,
			expectedErr: fmt.Errorf("record was modified since it was loaded: run_id=, expected_version=0, actual_version=1"),
		},
		{
			name: "No valid transition available",
			lookup: func(context.Context, string) (*Record, error) {
				return &Record{
					Status: int(statusStart),
				}, nil
			},
			current: statusStart,
			update: Run[string, testStatus]{
				TypedRecord: TypedRecord[string, testStatus]{
					Record: Record{
						RunState: RunStateRunning,
						Status:   int(statusMiddle),
					},
					Status: statusMiddle,
				},
			},
			transitions: []graph.Transition{
				{
					From: int(statusStart),
					To:   int(statusEnd),
				},
			},
			expectedErr: fmt.Errorf("current status not defined in graph: current=%s, next=%s", statusStart, statusMiddle),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			g := graph.New()
			for _, transition := range tc.transitions {
				g.AddTransition(transition.From, transition.To)
			}
			c := clock_testing.NewFakeClock(time.Now())

			store := func(ctx context.Context, r *Record) error {
				require.Equal(t, tc.expectedRunState, r.RunState)
				return nil
			}

			updater := newUpdater[string, testStatus](tc.lookup, store, g, c)
			err := updater(ctx, tc.current, tc.update.Status, &tc.update, 0)
			if err != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}

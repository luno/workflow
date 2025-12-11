package workflow

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/graph"
)

func TestProcessCallback(t *testing.T) {
	ctx := t.Context()
	w := &Workflow[string, testStatus]{
		name:        "example",
		ctx:         ctx,
		clock:       clock_testing.NewFakeClock(time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC)),
		statusGraph: graph.New(),
		logger:      &logger{},
		runPool: sync.Pool{
			New: func() interface{} {
				return &Run[string, testStatus]{}
			},
		},
	}

	w.statusGraph.AddTransition(int(statusStart), int(statusEnd))

	value := "data"
	b, err := Marshal(&value)
	require.NoError(t, err)

	current := &Record{
		WorkflowName: "example",
		ForeignID:    "32948623984623",
		RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
		RunState:     RunStateRunning,
		Status:       int(statusStart),
		Object:       b,
	}

	t.Run("Golden path callback - initiated", func(t *testing.T) {
		calls := map[string]int{
			"callbackFunc": 0,
			"updater":      0,
			"latestLookup": 0,
		}

		current := &Record{
			WorkflowName: "example",
			ForeignID:    "32948623984623",
			RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
			RunState:     RunStateInitiated,
			Status:       int(statusStart),
			Object:       b,
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus], reader io.Reader) (testStatus, error) {
			calls["callbackFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			calls["latestLookup"] += 1
			return current, nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, store, updater)
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"callbackFunc": 1,
			"latestLookup": 1,
			"updater":      1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Golden path callback", func(t *testing.T) {
		calls := map[string]int{
			"callbackFunc": 0,
			"updater":      0,
			"latestLookup": 0,
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus], reader io.Reader) (testStatus, error) {
			calls["callbackFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			calls["latestLookup"] += 1
			return current, nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, store, updater)
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"callbackFunc": 1,
			"latestLookup": 1,
			"updater":      1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Skip consume", func(t *testing.T) {
		calls := map[string]int{
			"callbackFunc": 0,
			"updater":      0,
			"latestLookup": 0,
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus], reader io.Reader) (testStatus, error) {
			calls["callbackFunc"] += 1
			return testStatus(skipTypeDefault), nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			return nil
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			calls["latestLookup"] += 1
			return current, nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, store, updater)
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"callbackFunc": 1,
			"latestLookup": 1,
			"updater":      0,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Return on lookup error", func(t *testing.T) {
		testErr := errors.New("test error")
		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			return nil, testErr
		}

		err := processCallback(ctx, w, testStatus(current.Status), nil, current.ForeignID, nil, latestLookup, nil, nil)
		require.Truef(t, errors.Is(err, testErr), "actual: %s", err.Error())
	})

	t.Run("Return on callbackFunc error", func(t *testing.T) {
		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			return current, nil
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus], reader io.Reader) (testStatus, error) {
			return 0, errors.New("test error")
		})

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, nil, nil)
		require.Equal(t, errors.New("test error"), err)
	})

	t.Run("Ignore if record is in different state", func(t *testing.T) {
		currentRecord := &Record{
			Status: int(statusMiddle),
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			return currentRecord, nil
		}

		err := processCallback(ctx, w, statusStart, nil, current.ForeignID, nil, latestLookup, nil, nil)
		require.NoError(t, err)
	})
}

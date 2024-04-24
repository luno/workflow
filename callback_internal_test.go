package workflow

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/graph"
)

func TestProcessCallback(t *testing.T) {
	ctx := context.Background()
	w := &Workflow[string, testStatus]{
		Name:        "example",
		ctx:         ctx,
		clock:       clock_testing.NewFakeClock(time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC)),
		statusGraph: graph.NewGraph(),
	}

	w.statusGraph.AddTransition(int(statusStart), int(statusEnd))

	value := "data"
	b, err := Marshal(&value)
	jtest.RequireNil(t, err)

	current := &WireRecord{
		ID:           1,
		WorkflowName: "example",
		ForeignID:    "32948623984623",
		RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
		RunState:     RunStateRunning,
		Status:       int(statusStart),
		Object:       b,
	}

	t.Run("Golden path callback", func(t *testing.T) {
		calls := map[string]int{
			"callbackFunc": 0,
			"updater":      0,
			"latestLookup": 0,
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus], reader io.Reader) (testStatus, error) {
			calls["callbackFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Record[string, testStatus]) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			calls["latestLookup"] += 1
			return current, nil
		}

		store := func(ctx context.Context, record *WireRecord, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, store, updater)
		jtest.RequireNil(t, err)

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

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus], reader io.Reader) (testStatus, error) {
			calls["callbackFunc"] += 1
			return testStatus(SkipTypeDefault), nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Record[string, testStatus]) error {
			calls["updater"] += 1
			return nil
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			calls["latestLookup"] += 1
			return current, nil
		}

		store := func(ctx context.Context, record *WireRecord, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, store, updater)
		jtest.RequireNil(t, err)

		expectedCalls := map[string]int{
			"callbackFunc": 1,
			"latestLookup": 1,
			"updater":      0,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Mark record as Running", func(t *testing.T) {
		currentRecord := &WireRecord{
			ID:           1,
			WorkflowName: "example",
			ForeignID:    "32948623984623",
			RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
			RunState:     RunStateInitiated,
			Status:       int(statusStart),
			Object:       b,
		}

		calls := map[string]int{
			"callbackFunc": 0,
			"updater":      0,
			"store":        0,
			"latestLookup": 0,
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus], reader io.Reader) (testStatus, error) {
			calls["callbackFunc"] += 1
			return statusEnd, nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Record[string, testStatus]) error {
			calls["updater"] += 1
			return nil
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			calls["latestLookup"] += 1
			return currentRecord, nil
		}

		store := func(ctx context.Context, record *WireRecord, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, store, updater)
		jtest.RequireNil(t, err)

		expectedCalls := map[string]int{
			"callbackFunc": 1,
			"updater":      1,
			"store":        1,
			"latestLookup": 1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Return on lookup error", func(t *testing.T) {
		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			return nil, errors.New("test error")
		}

		err := processCallback(ctx, w, testStatus(current.Status), nil, current.ForeignID, nil, latestLookup, nil, nil)
		jtest.Require(t, errors.New("failed to latest record for callback"), err)
	})

	t.Run("Return on callbackFunc error", func(t *testing.T) {
		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			return current, nil
		}

		callbackFn := CallbackFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus], reader io.Reader) (testStatus, error) {
			return 0, errors.New("test error")
		})

		err := processCallback(ctx, w, testStatus(current.Status), callbackFn, current.ForeignID, nil, latestLookup, nil, nil)
		jtest.Require(t, errors.New("test error"), err)
	})

	t.Run("Ignore if record is in different state", func(t *testing.T) {
		currentRecord := &WireRecord{
			Status: int(statusMiddle),
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			return currentRecord, nil
		}

		err := processCallback(ctx, w, statusStart, nil, current.ForeignID, nil, latestLookup, nil, nil)
		jtest.RequireNil(t, err)
	})

	t.Run("Return error if failed to update run state", func(t *testing.T) {
		calls := map[string]int{
			"store":        0,
			"latestLookup": 0,
		}

		currentRecord := &WireRecord{
			ID:           1,
			WorkflowName: "example",
			ForeignID:    "32948623984623",
			RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
			RunState:     RunStateInitiated,
			Status:       int(statusStart),
			Object:       b,
		}

		latestLookup := func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error) {
			calls["latestLookup"] += 1
			return currentRecord, nil
		}

		store := func(ctx context.Context, record *WireRecord, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return errors.New("test error")
		}

		err := processCallback(ctx, w, statusStart, nil, current.ForeignID, nil, latestLookup, store, nil)
		jtest.Require(t, errors.New("test error"), err)

		expectedCalls := map[string]int{
			"store":        1,
			"latestLookup": 1,
		}
		require.Equal(t, expectedCalls, calls)
	})
}

package workflow

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/errorcounter"
	internal_logger "github.com/luno/workflow/internal/logger"
)

func Test_stepConsumer(t *testing.T) {
	ctx := t.Context()
	counter := errorcounter.New()
	processName := "processName"
	testErr := errors.New("test error")
	w := &Workflow[string, testStatus]{
		name:         "example",
		ctx:          ctx,
		clock:        clock_testing.NewFakeClock(time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC)),
		errorCounter: counter,
		logger: &logger{
			inner: internal_logger.New(os.Stdout),
		},
	}
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

	t.Run("Golden path consume - Initiated", func(t *testing.T) {
		calls := map[string]int{
			"consumerFunc": 0,
			"updater":      0,
			"lookup":       0,
		}

		currentRecord := &Record{
			WorkflowName: "example",
			ForeignID:    "32948623984623",
			RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
			RunState:     RunStateInitiated,
			Status:       int(statusStart),
			Object:       b,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		lookup := func(ctx context.Context, runID string) (*Record, error) {
			calls["lookup"] += 1
			return currentRecord, nil
		}

		err := stepConsumer(
			w.Name(),
			"",
			consumer,
			testStatus(current.Status),
			lookup,
			store,
			w.logger,
			updater,
			0,
			w.errorCounter,
		)(ctx, &Event{})
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"updater":      1,
			"lookup":       1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Golden path consume - Running", func(t *testing.T) {
		calls := map[string]int{
			"consumerFunc": 0,
			"updater":      0,
			"lookup":       0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		lookup := func(ctx context.Context, runID string) (*Record, error) {
			calls["lookup"] += 1
			return current, nil
		}

		err := stepConsumer(
			w.Name(),
			"",
			consumer,
			testStatus(current.Status),
			lookup,
			store,
			w.logger,
			updater,
			0,
			w.errorCounter,
		)(ctx, &Event{})
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"lookup":       1,
			"updater":      1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Skip consume", func(t *testing.T) {
		calls := map[string]int{
			"consumerFunc": 0,
			"updater":      0,
			"lookup":       0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			return testStatus(skipTypeDefault), nil
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			return nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		lookup := func(ctx context.Context, runID string) (*Record, error) {
			calls["lookup"] += 1
			return current, nil
		}

		err := stepConsumer(
			w.Name(),
			"",
			consumer,
			testStatus(current.Status),
			lookup,
			store,
			w.logger,
			updater,
			0,
			w.errorCounter,
		)(ctx, &Event{})
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"updater":      0,
			"lookup":       1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Pause record after exceeding allowed error count", func(t *testing.T) {
		counter.Clear(testErr, processName, current.RunID)

		calls := map[string]int{
			"consumerFunc": 0,
			"updater":      0,
			"lookup":       0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			return 0, testErr
		})

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus], workingVersion uint) error {
			calls["updater"] += 1
			return nil
		}

		store := func(ctx context.Context, record *Record) error {
			calls["store"] += 1
			return nil
		}

		lookup := func(ctx context.Context, runID string) (*Record, error) {
			calls["lookup"] += 1
			return current, nil
		}

		consume := stepConsumer(
			w.Name(),
			"",
			consumer,
			testStatus(current.Status),
			lookup,
			store,
			w.logger,
			updater,
			3,
			w.errorCounter,
		)
		require.NoError(t, err)

		err := consume(ctx, &Event{})
		require.NotNil(t, err)

		err = consume(ctx, &Event{})
		require.NotNil(t, err)

		err = consume(ctx, &Event{})
		require.NoError(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 3,
			"lookup":       3,
			"updater":      0,
			"store":        1,
		}
		require.Equal(t, expectedCalls, calls)
	})
}

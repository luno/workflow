package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/errorcounter"
)

func TestConsume(t *testing.T) {
	ctx := context.Background()
	w := &Workflow[string, testStatus]{
		Name:         "example",
		ctx:          ctx,
		clock:        clock_testing.NewFakeClock(time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC)),
		errorCounter: errorcounter.New(),
	}

	value := "data"
	b, err := Marshal(&value)
	require.Nil(t, err)

	current := &Record{
		ID:           1,
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
			"ack":          0,
			"updater":      0,
		}

		currentRecord := &Record{
			ID:           1,
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

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus]) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		store := func(ctx context.Context, record *Record, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := consume(ctx, w, currentRecord, consumer, ack, store, updater, "processName", 0)
		require.Nil(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"ack":          1,
			"updater":      1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Golden path consume - Running", func(t *testing.T) {
		calls := map[string]int{
			"consumerFunc": 0,
			"ack":          0,
			"updater":      0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus]) error {
			calls["updater"] += 1
			require.Equal(t, "new data", *record.Object)
			return nil
		}

		store := func(ctx context.Context, record *Record, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := consume(ctx, w, current, consumer, ack, store, updater, "processName", 0)
		require.Nil(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"ack":          1,
			"updater":      1,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Skip consume", func(t *testing.T) {
		calls := map[string]int{
			"consumerFunc": 0,
			"ack":          0,
			"updater":      0,
			"storeAndEmit": 0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			return testStatus(SkipTypeDefault), nil
		})

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus]) error {
			calls["updater"] += 1
			return nil
		}

		store := func(ctx context.Context, record *Record, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := consume(ctx, w, current, consumer, ack, store, updater, "processName", 0)
		require.Nil(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"ack":          1,
			"updater":      0,
			"storeAndEmit": 0,
		}
		require.Equal(t, expectedCalls, calls)
	})

	t.Run("Pause record after exceeding allowed error count", func(t *testing.T) {
		t.Parallel()

		calls := map[string]int{
			"consumerFunc": 0,
			"ack":          0,
			"updater":      0,
			"store":        0,
		}

		testErr := errors.New("test error")

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			return 0, testErr
		})

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, current testStatus, next testStatus, record *Run[string, testStatus]) error {
			calls["updater"] += 1
			return nil
		}

		store := func(ctx context.Context, record *Record, maker OutboxEventDataMaker) error {
			calls["store"] += 1
			return nil
		}

		err := consume(ctx, w, current, consumer, ack, store, updater, "processName", 3)
		require.True(t, errors.Is(err, testErr))

		err = consume(ctx, w, current, consumer, ack, store, updater, "processName", 3)
		require.True(t, errors.Is(err, testErr))

		err = consume(ctx, w, current, consumer, ack, store, updater, "processName", 3)
		require.Nil(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 3,
			"ack":          1,
			"updater":      0,
			"store":        1,
		}
		require.Equal(t, expectedCalls, calls)
	})
}

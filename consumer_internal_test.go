package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"
)

func TestConsume(t *testing.T) {
	ctx := context.Background()
	w := &Workflow[string, testStatus]{
		Name:  "example",
		ctx:   ctx,
		clock: clock_testing.NewFakeClock(time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC)),
	}

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

	t.Run("Golden path consume", func(t *testing.T) {
		calls := map[string]int{
			"consumerFunc": 0,
			"ack":          0,
			"updater":      0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			*r.Object = "new data"
			return statusEnd, nil
		})

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, store RecordStore, graph map[int][]int, currentStatus int, next *WireRecord) error {
			calls["updater"] += 1

			val := "new data"
			expected, err := Marshal(&val)
			jtest.RequireNil(t, err)

			require.Equal(t, expected, next.Object)
			return nil
		}

		// Not expected to be called
		storeAndEmitter := storeAndEmitFunc(nil)

		err := consume(ctx, w, current, consumer, ack, updater, storeAndEmitter, "processName")
		jtest.RequireNil(t, err)

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

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			return testStatus(SkipTypeDefault), nil
		})

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, store RecordStore, graph map[int][]int, currentStatus int, next *WireRecord) error {
			calls["updater"] += 1
			return nil
		}

		storeAndEmitter := func(ctx context.Context, store RecordStore, wr *WireRecord, previousRunState RunState) error {
			calls["storeAndEmit"] += 1
			return nil
		}

		err := consume(ctx, w, current, consumer, ack, updater, storeAndEmitter, "processName")
		jtest.RequireNil(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"ack":          1,
			"updater":      0,
			"storeAndEmit": 0,
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
			"consumerFunc": 0,
			"ack":          0,
			"updater":      0,
			"storeAndEmit": 0,
		}

		consumer := ConsumerFunc[string, testStatus](func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
			calls["consumerFunc"] += 1
			return testStatus(SkipTypeDefault), nil
		})

		ack := func() error {
			calls["ack"] += 1
			return nil
		}

		updater := func(ctx context.Context, store RecordStore, graph map[int][]int, currentStatus int, next *WireRecord) error {
			calls["updater"] += 1
			return nil
		}

		storeAndEmitter := func(ctx context.Context, store RecordStore, wr *WireRecord, previousRunState RunState) error {
			calls["storeAndEmit"] += 1

			require.Equal(t, RunStateRunning, wr.RunState)
			return nil
		}

		err := consume(ctx, w, currentRecord, consumer, ack, updater, storeAndEmitter, "processName")
		jtest.RequireNil(t, err)

		expectedCalls := map[string]int{
			"consumerFunc": 1,
			"ack":          1,
			"updater":      0,
			"storeAndEmit": 1,
		}
		require.Equal(t, expectedCalls, calls)
	})
}

package workflow

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/clock"
	clock_testing "k8s.io/utils/clock/testing"

	internal_logger "github.com/luno/workflow/internal/logger"
)

func Test_runOnce(t *testing.T) {
	ctx := t.Context()

	t.Run("Returns non-nil error (context.Canceled) when parent ctx is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			nil,
			nil,
			nil,
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Returns awaitRole's context cancellation", func(t *testing.T) {
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				return nil, nil, context.Canceled
			},
			func(ctx context.Context) error {
				return nil
			},
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Retry awaitRole error that is not context.Canceled", func(t *testing.T) {
		testErr := errors.New("test error")
		buf := bytes.NewBuffer([]byte{})
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				return nil, nil, testErr
			},
			func(ctx context.Context) error {
				return nil
			},
			&logger{
				debugMode: false,
				inner:     internal_logger.New(buf),
			},
			clock.RealClock{},
			time.Minute,
		)
		require.NoError(t, err)
		require.Contains(t, buf.String(), `"msg":"run error [role=role-1], [process=process-1]: test error"`)
	})

	t.Run("Cancelled parent context during process execution retries and exits with context.Canceled ", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				ctx, cancel := context.WithCancel(ctx)
				return ctx, cancel, nil
			},
			func(ctx context.Context) error {
				// Cancel parent context and return context error
				cancel()
				return ctx.Err()
			},
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		// If the err is nil then it will be retried
		require.NoError(t, err)

		// Context has been cancelled previously in the last runOnce so we expect
		// this to immediately return context.Canceled and need any parameters.
		err = runOnce(
			ctx,
			"",
			"",
			"",
			nil,
			nil,
			nil,
			nil,
			nil,
			time.Minute,
		)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Retry process error that is not context.Canceled", func(t *testing.T) {
		testErr := errors.New("test error")
		buf := bytes.NewBuffer([]byte{})
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				ctx, cancel := context.WithCancel(ctx)
				return ctx, cancel, nil
			},
			func(ctx context.Context) error {
				return testErr
			},
			&logger{
				debugMode: false,
				inner:     internal_logger.New(buf),
			},
			clock.RealClock{},
			time.Millisecond,
		)

		require.NoError(t, err)
		require.Contains(t, buf.String(), `"msg":"run error [role=role-1], [process=process-1]: test error`)
	})

	t.Run("Updates process state", func(t *testing.T) {
		var stateChanges []string
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {
				stateChanges = append(stateChanges, s.String())
			},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				ctx, cancel := context.WithCancel(ctx)
				return ctx, cancel, nil
			},
			func(ctx context.Context) error {
				return nil
			},
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		require.NoError(t, err)

		expected := []string{StateIdle.String(), StateRunning.String()}
		require.Equal(t, expected, stateChanges)
	})
}

func TestWorkflow_RunStopRace(t *testing.T) {
	// This test verifies that calling Run() and Stop() concurrently doesn't cause a data race.
	// It specifically tests that w.cancel is properly protected by a mutex during concurrent
	// access from Run() (write) and Stop() (read).
	// Run with: go test -race
	
	ctx := context.Background()
	
	// Run multiple iterations to increase the chance of detecting the race condition
	for i := 0; i < 100; i++ {
		// Build a minimal workflow using the Builder
		b := NewBuilder[string, testStatus]("race-test")
		b.AddStep(statusStart, func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		}, statusEnd)
		
		wf := b.Build(
			&noopEventStreamer{},
			&noopRecordStore{},
			&noopScheduler{},
			WithoutOutbox(),
		)
		
		// Start Run in a goroutine - this will write to w.cancel inside once.Do
		done := make(chan struct{})
		go func() {
			wf.Run(ctx)
			close(done)
		}()
		
		// Immediately call Stop - this reads w.cancel
		// Without the mutex protection, this would race with the write in Run()
		wf.Stop()

		// Wait for Run to complete
		<-done

		// Call Stop again to ensure cleanup of any goroutines that were started.
		// This handles the case where the first Stop() call happened before Run()
		// had set the cancel function (so it returned early without stopping anything).
		wf.Stop()
	}
}

// noopScheduler implements RoleScheduler for testing
type noopScheduler struct{}

func (n *noopScheduler) Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
	return ctx, func() {}, nil
}

// noopEventStreamer implements EventStreamer for testing
type noopEventStreamer struct{}

func (n *noopEventStreamer) NewSender(ctx context.Context, topic string) (EventSender, error) {
	return &noopEventSender{}, nil
}

func (n *noopEventStreamer) NewReceiver(ctx context.Context, topic string, consumerName string, opts ...ReceiverOption) (EventReceiver, error) {
	// Return a receiver that blocks until context is cancelled
	return &noopEventReceiver{ctx: ctx}, nil
}

// noopEventSender implements EventSender for testing
type noopEventSender struct{}

func (n *noopEventSender) Send(ctx context.Context, foreignID string, statusType int, headers map[Header]string) error {
	return nil
}

func (n *noopEventSender) Close() error {
	return nil
}

// noopEventReceiver implements EventReceiver for testing
type noopEventReceiver struct {
	ctx context.Context
}

func (n *noopEventReceiver) Recv(ctx context.Context) (*Event, Ack, error) {
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

func (n *noopEventReceiver) Close() error {
	return nil
}

// noopRecordStore implements RecordStore for testing
type noopRecordStore struct{}

func (n *noopRecordStore) Store(ctx context.Context, record *Record) error {
	return nil
}

func (n *noopRecordStore) Latest(ctx context.Context, workflowName, foreignID string) (*Record, error) {
	return nil, errors.New("not found")
}

func (n *noopRecordStore) Lookup(ctx context.Context, id string) (*Record, error) {
	return nil, errors.New("not found")
}

func (n *noopRecordStore) List(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error) {
	return nil, nil
}

func (n *noopRecordStore) ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]OutboxEvent, error) {
	return nil, nil
}

func (n *noopRecordStore) DeleteOutboxEvent(ctx context.Context, id string) error {
	return nil
}

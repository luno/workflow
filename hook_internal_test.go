package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/clock"
)

func Test_runHooks(t *testing.T) {
	ctx := context.Background()
	clock := clock.RealClock{}

	t.Run("Skip event on invalid / missing HeaderRunState", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		err := runHooks[string, testStatus](
			ctx,
			"workflow_name",
			"process_name",
			consumerImpl{
				recv: func(ctx context.Context) (*Event, Ack, error) {
					return &Event{
							Headers: map[Header]string{},
						}, func() error {
							cancel()
							return nil
						}, nil
				},
				close: func() error {
					return nil
				},
			},
			RunStateCompleted,
			nil,
			nil,
			time.Minute,
			clock,
		)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("Skip event on event RunState and target RunState mismatch", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		err := runHooks[string, testStatus](
			ctx,
			"workflow_name",
			"process_name",
			consumerImpl{
				recv: func(ctx context.Context) (*Event, Ack, error) {
					return &Event{
							Headers: map[Header]string{
								HeaderRunState: "1",
							},
						}, func() error {
							cancel()
							return nil
						}, nil
				},
				close: func() error {
					return nil
				},
			},
			RunStateCompleted,
			nil, // Test will panic if the event is not skipped and this function is called.
			nil,
			time.Minute,
			clock,
		)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("Skip event on ErrRecordNotFound", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		err := runHooks[string, testStatus](
			ctx,
			"workflow_name",
			"process_name",
			consumerImpl{
				recv: func(ctx context.Context) (*Event, Ack, error) {
					return &Event{
							Headers: map[Header]string{
								HeaderRunState: "5",
							},
						}, func() error {
							cancel()
							return nil
						}, nil
				},
				close: func() error {
					return nil
				},
			},
			RunStateCompleted,
			func(ctx context.Context, id int64) (*Record, error) {
				// Ensures that the test only finished if this is called
				wg.Done()
				return nil, ErrRecordNotFound
			},
			nil,
			time.Minute,
			clock,
		)
		wg.Wait()
		require.Equal(t, context.Canceled, err)
	})

	t.Run("Return non-nil error from lookup", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		testErr := errors.New("test error")
		err := runHooks[string, testStatus](
			ctx,
			"workflow_name",
			"process_name",
			consumerImpl{
				recv: func(ctx context.Context) (*Event, Ack, error) {
					return &Event{
							Headers: map[Header]string{
								HeaderRunState: "5",
							},
						}, func() error {
							cancel()
							return nil
						}, nil
				},
				close: func() error {
					return nil
				},
			},
			RunStateCompleted,
			func(ctx context.Context, id int64) (*Record, error) {
				return nil, testErr
			},
			nil,
			time.Minute,
			clock,
		)
		require.Equal(t, testErr, err)
	})

	t.Run("Skip event on failure to unmarshal object", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		err := runHooks[string, testStatus](
			ctx,
			"workflow_name",
			"process_name",
			consumerImpl{
				recv: func(ctx context.Context) (*Event, Ack, error) {
					return &Event{
							Headers: map[Header]string{
								HeaderRunState: "5",
							},
						}, func() error {
							cancel()
							return nil
						}, nil
				},
				close: func() error {
					return nil
				},
			},
			RunStateCompleted,
			func(ctx context.Context, id int64) (*Record, error) {
				wg.Done()
				return &Record{
					Object: []byte("INVALID JSON"),
				}, nil
			},
			nil,
			time.Minute,
			clock,
		)
		wg.Wait()
		require.Equal(t, context.Canceled, err)
	})

	t.Run("Return non-error if hook errors", func(t *testing.T) {
		testErr := errors.New("test error")
		err := runHooks[string, testStatus](
			ctx,
			"workflow_name",
			"process_name",
			consumerImpl{
				recv: func(ctx context.Context) (*Event, Ack, error) {
					return &Event{
							Headers: map[Header]string{
								HeaderRunState: "5",
							},
						}, func() error {
							return nil
						}, nil
				},
				close: func() error {
					return nil
				},
			},
			RunStateCompleted,
			func(ctx context.Context, id int64) (*Record, error) {
				return &Record{
					Object: []byte(`"VALID JSON"`),
				}, nil
			},
			func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
				return testErr
			},
			time.Minute,
			clock,
		)
		require.Equal(t, testErr, err)
	})
}

type consumerImpl struct {
	recv  func(ctx context.Context) (*Event, Ack, error)
	close func() error
}

func (c consumerImpl) Recv(ctx context.Context) (*Event, Ack, error) {
	return c.recv(ctx)
}

func (c consumerImpl) Close() error {
	return c.close()
}

var _ Consumer = (*consumerImpl)(nil)

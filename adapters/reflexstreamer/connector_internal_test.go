package reflexstreamer

import (
	"context"
	"testing"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex"
	"github.com/luno/workflow"
)

func TestConnectorErrHandling(t *testing.T) {
	testErr := errors.New("test error")
	t.Run("Return test error from GetCursor from Make method call", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "", testErr
				},
			},
		}

		_, err := connector.Make(context.Background(), "")
		jtest.Require(t, testErr, err)
	})

	t.Run("Return test error from streamFn from Make method call", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return nil, testErr
			},
		}

		_, err := connector.Make(context.Background(), "")
		jtest.Require(t, testErr, err)
	})

	t.Run("Return test error from stream client Recv from Recv method call", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{
					recv: func() (*reflex.Event, error) {
						return nil, testErr
					},
				}, nil
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		_, _, err = consumer.Recv(ctx)
		jtest.Require(t, testErr, err)
	})

	t.Run("Return test error from SetCursor from Recv method call", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{
					recv: func() (*reflex.Event, error) {
						return nil, testErr
					},
				}, nil
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		_, _, err = consumer.Recv(ctx)
		jtest.Require(t, testErr, err)
	})

	t.Run("Return test error from translator from Recv method call", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{
					recv: func() (*reflex.Event, error) {
						return &reflex.Event{}, nil
					},
				}, nil
			},
			translator: func(e *reflex.Event) (*workflow.ConnectorEvent, error) {
				return &workflow.ConnectorEvent{}, testErr
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		_, _, err = consumer.Recv(ctx)
		jtest.Require(t, testErr, err)
	})

	t.Run("Return test error from SetCursor on ack", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
				setCursor: func(ctx context.Context, consumerName string, cursor string) error {
					return testErr
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{
					recv: func() (*reflex.Event, error) {
						return &reflex.Event{}, nil
					},
				}, nil
			},
			translator: func(e *reflex.Event) (*workflow.ConnectorEvent, error) {
				return &workflow.ConnectorEvent{}, nil
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		_, ack, err := consumer.Recv(ctx)
		jtest.RequireNil(t, err)

		err = ack()
		jtest.Require(t, testErr, err)
	})

	t.Run("Return context Cancelled error from Recv", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{}, nil
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err = consumer.Recv(ctx)
		jtest.Require(t, context.Canceled, err)
	})

	t.Run("Return non-nil flush error from Close", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
				flush: func(context.Context) error {
					return testErr
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{}, nil
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		err = consumer.Close()
		jtest.Require(t, testErr, err)
	})

	t.Run("Return non-nil closer error from Close", func(t *testing.T) {
		connector := connector{
			cursorStore: testCStore{
				getCursor: func(ctx context.Context, consumerName string) (string, error) {
					return "0", nil
				},
				flush: func(context.Context) error {
					return nil
				},
			},
			streamFn: func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
				return testStreamClient{
					close: func() error {
						return testErr
					},
				}, nil
			},
		}

		ctx := t.Context()
		consumer, err := connector.Make(ctx, "")
		jtest.RequireNil(t, err)

		err = consumer.Close()
		jtest.Require(t, testErr, err)
	})
}

type testCStore struct {
	getCursor func(ctx context.Context, consumerName string) (string, error)
	setCursor func(ctx context.Context, consumerName string, cursor string) error
	flush     func(ctx context.Context) error
}

func (c testCStore) GetCursor(ctx context.Context, consumerName string) (string, error) {
	return c.getCursor(ctx, consumerName)
}

func (c testCStore) SetCursor(ctx context.Context, consumerName string, cursor string) error {
	return c.setCursor(ctx, consumerName, cursor)
}

func (c testCStore) Flush(ctx context.Context) error {
	return c.flush(ctx)
}

var _ reflex.CursorStore = (*testCStore)(nil)

type testStreamClient struct {
	recv  func() (*reflex.Event, error)
	close func() error
}

func (t testStreamClient) Recv() (*reflex.Event, error) {
	return t.recv()
}

func (t testStreamClient) Close() error {
	return t.close()
}

var _ reflex.StreamClient = (*testStreamClient)(nil)

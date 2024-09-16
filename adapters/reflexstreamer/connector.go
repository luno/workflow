package reflexstreamer

import (
	"context"
	"io"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/workflow"
)

func NewConnector(streamFn reflex.StreamFunc, cursorStore reflex.CursorStore, t ReflexTranslator) *connector {
	return &connector{
		translator:  t,
		streamFn:    streamFn,
		cursorStore: cursorStore,
	}
}

type ReflexTranslator func(e *reflex.Event) (*workflow.ConnectorEvent, error)

type connector struct {
	translator  ReflexTranslator
	streamFn    reflex.StreamFunc
	cursorStore reflex.CursorStore
}

func (c *connector) Make(ctx context.Context, name string) (workflow.ConnectorConsumer, error) {
	cursor, err := c.cursorStore.GetCursor(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect cursor")
	}

	streamClient, err := c.streamFn(ctx, cursor)
	if err != nil {
		return nil, err
	}

	return &consumer{
		cursorName:   name,
		translator:   c.translator,
		cursorStore:  c.cursorStore,
		streamClient: streamClient,
	}, nil
}

type consumer struct {
	cursorName   string
	translator   ReflexTranslator
	cursorStore  reflex.CursorStore
	streamClient reflex.StreamClient
}

func (c consumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
	for ctx.Err() == nil {
		reflexEvent, err := c.streamClient.Recv()
		if err != nil {
			return nil, nil, err
		}

		event, err := c.translator(reflexEvent)
		if err != nil {
			return nil, nil, err
		}

		return event, func() error {
			// Increment cursor for consumer only if ack function is called.
			if err := c.cursorStore.SetCursor(ctx, c.cursorName, event.ID); err != nil {
				return errors.Wrap(err, "failed to set cursor", j.MKV{
					"cursor_name": c.cursorName,
					"event_id":    reflexEvent.ID,
					"event_fid":   reflexEvent.ForeignID,
				})
			}

			return nil
		}, nil
	}

	// If the loop breaks then ctx.Err is non-nil
	return nil, nil, ctx.Err()
}

func (c consumer) Close() error {
	// Provide new context for flushing of cursor values to underlying store
	err := c.cursorStore.Flush(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to flush cursor")
	}

	if closer, ok := c.streamClient.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

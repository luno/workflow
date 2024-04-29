package reflexstreamer

import (
	"context"
	"fmt"
	"io"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"

	"github.com/luno/workflow"
)

func NewConnector(streamFn reflex.StreamFunc, cursorStore reflex.CursorStore) *connector {
	return &connector{
		streamFn:    streamFn,
		cursorStore: cursorStore,
	}
}

type connector struct {
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
		cursorStore:  c.cursorStore,
		streamClient: streamClient,
	}, nil
}

type consumer struct {
	cursorName   string
	cursorStore  reflex.CursorStore
	streamClient reflex.StreamClient
}

var HeaderMeta = "meta"

func (c consumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
	for ctx.Err() == nil {
		reflexEvent, err := c.streamClient.Recv()
		if err != nil {
			return nil, nil, err
		}

		event := &workflow.ConnectorEvent{
			ID:        reflexEvent.ID,
			ForeignID: reflexEvent.ForeignID,
			Type:      fmt.Sprintf("%v", reflexEvent.Type.ReflexType()),
			Headers: map[string]string{
				HeaderMeta: string(reflexEvent.MetaData),
			},
			CreatedAt: reflexEvent.Timestamp,
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
		return errors.Wrap(err, "failed here")
	}

	if closer, ok := c.streamClient.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

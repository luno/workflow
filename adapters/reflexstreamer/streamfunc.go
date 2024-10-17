package reflexstreamer

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"strconv"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow"
)

// StreamFunc can take the single event source (rsql.EventsTableInt) for multiple workflows and stream events
// corresponding to a specific workflow. This is possible due to the way Workflow uses the reflex events table's metadata
// column.
func StreamFunc(dbc *sql.DB, table *rsql.EventsTableInt, workflowName string) reflex.StreamFunc {
	return func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
		cl, err := table.ToStream(dbc)(ctx, after, opts...)
		if err != nil {
			return nil, err
		}

		return &streamClient{
			ctx:          ctx,
			workflowName: workflowName,
			client:       cl,
		}, nil
	}
}

type streamClient struct {
	ctx          context.Context
	workflowName string
	client       reflex.StreamClient
	filter       func(e *reflex.Event, headers map[workflow.Header]string) bool
}

func (s *streamClient) Recv() (*reflex.Event, error) {
	for s.ctx.Err() == nil {
		reflexEvent, err := s.client.Recv()
		if err != nil {
			return nil, err
		}

		if closer, ok := s.client.(io.Closer); ok {
			defer closer.Close()
		}

		headers := make(map[workflow.Header]string)
		err = json.Unmarshal(reflexEvent.MetaData, &headers)
		if err != nil {
			return nil, err
		}

		if headers[workflow.HeaderWorkflowName] != s.workflowName {
			continue
		}

		if s.filter != nil && s.filter(reflexEvent, headers) {
			continue
		}

		return reflexEvent, nil
	}

	return nil, s.ctx.Err()
}

// OnComplete can take the single event source (e.g. rsql.EventsTableInt) for multiple workflows and stream only the
// events where the workflow run has reached workflow.RunStateCompleted for the provided workflow name.
func OnComplete(dbc *sql.DB, table *rsql.EventsTableInt, workflowName string) reflex.StreamFunc {
	onCompleteFilter := func(e *reflex.Event, headers map[workflow.Header]string) bool {
		runState, err := strconv.ParseInt(headers[workflow.HeaderRunState], 10, 64)
		if err != nil {
			// NoReturnErr: Skip invalid run state headers
			return true
		}

		if workflow.RunState(runState) != workflow.RunStateCompleted {
			return true
		}

		return false
	}
	return filteredStreamClient(dbc, table, workflowName, onCompleteFilter)
}

// eventFilter allows for custom specification of filtering events. Returning true will result in the event being
// filtered out.
type eventFilter func(e *reflex.Event, headers map[workflow.Header]string) bool

func filteredStreamClient(
	dbc *sql.DB,
	table *rsql.EventsTableInt,
	workflowName string,
	filter eventFilter,
) reflex.StreamFunc {
	return func(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
		cl, err := table.ToStream(dbc)(ctx, after, opts...)
		if err != nil {
			return nil, err
		}

		return &streamClient{
			ctx:          ctx,
			workflowName: workflowName,
			client:       cl,
			filter:       filter,
		}, nil
	}
}

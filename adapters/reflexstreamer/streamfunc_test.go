package reflexstreamer_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/reflexstreamer"
)

func TestStreamFunc(t *testing.T) {
	dbc := ConnectForTesting(t)
	eventsTable := rsql.NewEventsTable("workflow_events", rsql.WithEventMetadataField("metadata"))

	wf, store, ctx, cancel := createTestWorkflow(t, dbc, eventsTable)

	fid := "23847923847"
	_, err := wf.Trigger(ctx, fid)
	require.Nil(t, err)

	workflow.Require(t, wf, fid, statusEnd, "Started and Completed in a Workflow")

	spec := reflex.NewSpec(
		reflexstreamer.StreamFunc(dbc, eventsTable, "myWorkflow"),
		rpatterns.MemCursorStore(),
		reflex.NewConsumer("something", func(ctx context.Context, event *reflex.Event) error {
			wireRecord, err := store.Lookup(ctx, event.ForeignID)
			if err != nil {
				return err
			}

			var val string
			err = workflow.Unmarshal(wireRecord.Object, &val)
			if err != nil {
				return err
			}

			val += " as well as consumed in a reflex consumer 😱"

			require.Equal(t, "Started and Completed in a Workflow as well as consumed in a reflex consumer 😱", val)

			// End the test
			cancel()

			return nil
		}),
	)

	err = reflex.Run(ctx, spec)
	jtest.Require(t, context.Canceled, err)
}

func TestOnComplete(t *testing.T) {
	dbc := ConnectForTesting(t)
	eventsTable := rsql.NewEventsTable("workflow_events", rsql.WithEventMetadataField("metadata"))

	wf, store, ctx, cancel := createTestWorkflow(t, dbc, eventsTable)

	fid := "23847923847"
	_, err := wf.Trigger(ctx, fid)
	require.Nil(t, err)

	workflow.Require(t, wf, fid, statusEnd, "Started and Completed in a Workflow")

	spec := reflex.NewSpec(
		reflexstreamer.OnComplete(dbc, eventsTable, "myWorkflow"),
		rpatterns.MemCursorStore(),
		reflex.NewConsumer("something", func(ctx context.Context, event *reflex.Event) error {
			wireRecord, err := store.Lookup(ctx, event.ForeignID)
			if err != nil {
				return err
			}

			var val string
			err = workflow.Unmarshal(wireRecord.Object, &val)
			if err != nil {
				return err
			}

			require.Equal(t, "Started and Completed in a Workflow", val)
			require.Equal(t, workflow.RunStateCompleted, wireRecord.RunState)

			// End the test
			cancel()

			return nil
		}),
	)

	err = reflex.Run(ctx, spec)
	jtest.Require(t, context.Canceled, err)
}

func createTestWorkflow(t *testing.T, dbc *sql.DB, eventsTable *rsql.EventsTable) (*workflow.Workflow[string, status], workflow.RecordStore, context.Context, context.CancelFunc) {
	workflowName := "myWorkflow"
	b := workflow.NewBuilder[string, status](workflowName)
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			*r.Object = "Started and "
			return statusMiddle, nil
		},
		statusMiddle,
	)
	b.AddStep(
		statusMiddle,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			*r.Object += "Completed in a Workflow"
			return statusEnd, nil
		},
		statusEnd,
	)

	recordStore := memrecordstore.New()

	wf := b.Build(
		reflexstreamer.New(dbc, dbc, eventsTable, rpatterns.MemCursorStore()),
		recordStore,
		memrolescheduler.New(),
	)
	ctx, cancel := context.WithCancel(context.Background())

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	return wf, recordStore, ctx, cancel
}

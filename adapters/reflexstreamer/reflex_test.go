package reflexstreamer_test

import (
	"context"
	"testing"

	"github.com/luno/fate"
	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
	"github.com/luno/reflex/rsql"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/adapters/reflexstreamer"
)

func TestStreamer(t *testing.T) {
	eventsTable := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))
	dbc := ConnectForTesting(t)
	cTable := rsql.NewCursorsTable("cursors")
	constructor := reflexstreamer.New(dbc, dbc, eventsTable, cTable.ToStore(dbc))
	adaptertest.RunEventStreamerTest(t, constructor)
}

func TestStreamFunc(t *testing.T) {
	dbc := ConnectForTesting(t)
	eventsTable := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))

	workflowName := "myWorkflow"
	b := workflow.NewBuilder[string, status](workflowName)
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
			*r.Object = "Started and "
			return statusMiddle, nil
		},
		statusMiddle,
	)
	b.AddStep(
		statusMiddle,
		func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
			*r.Object += "Completed in a Workflow"
			return statusEnd, nil
		},
		statusEnd,
	)

	recordStore := memrecordstore.New()

	wf := b.Build(
		reflexstreamer.New(dbc, dbc, eventsTable, rpatterns.MemCursorStore()),
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
	)
	ctx, cancel := context.WithCancel(context.Background())

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "23847923847"
	_, err := wf.Trigger(ctx, fid, statusStart)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, fid, statusEnd, "Started and Completed in a Workflow")

	spec := reflex.NewSpec(
		reflexstreamer.StreamFunc(dbc, eventsTable, "myWorkflow"),
		rpatterns.MemCursorStore(),
		reflex.NewConsumer("something", func(ctx context.Context, fate fate.Fate, event *reflex.Event) error {
			wireRecord, err := recordStore.Lookup(ctx, event.ForeignIDInt())
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

type status int

var (
	statusUnknown status = 0
	statusStart   status = 1
	statusMiddle  status = 2
	statusEnd     status = 3
)

func (s status) String() string {
	switch s {
	case statusStart:
		return "Start"
	case statusMiddle:
		return "Middle"
	case statusEnd:
		return "End"
	default:
		return "Unknown"
	}
}

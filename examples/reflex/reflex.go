package reflex

import (
	"context"
	"database/sql"

	"github.com/luno/fate"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
	"github.com/luno/reflex/rsql"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/adapters/reflexstreamer"
	"github.com/luno/workflow/examples"
	"github.com/luno/workflow/examples/gettingstarted"
)

func ExampleWorkflow(db *sql.DB, table *rsql.EventsTableInt, cstore reflex.CursorStore) *workflow.Workflow[gettingstarted.GettingStarted, examples.Status] {
	return gettingstarted.Workflow(gettingstarted.Deps{
		EventStreamer: reflexstreamer.New(db, db, table, cstore),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
}

// ExampleReflexStreamFunc is a simplistic example of using it for a reflex consumer. To see full example
// go to examples/reflex/reflex_test.go.
func ExampleReflexStreamFunc(db *sql.DB, table *rsql.EventsTableInt, workflowName string) {
	consumerFunc := reflex.NewConsumer("my-consumer-name", func(ctx context.Context, fate fate.Fate, event *reflex.Event) error {
		return nil
	})

	_ = reflex.NewSpec(
		// Note that the table should not be used directly due to the reflex adapter's implementation using a single
		// event table for events of multiple state machines and requires the wrapper to filter for the correct workflow.
		reflexstreamer.StreamFunc(db, table, workflowName),
		rpatterns.MemCursorStore(),
		consumerFunc,
	)
}

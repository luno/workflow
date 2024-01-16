package reflex

import (
	"database/sql"

	"github.com/luno/reflex"
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
	return gettingstarted.WorkflowWithEnum(gettingstarted.Deps{
		EventStreamer: reflexstreamer.New(db, db, table, cstore),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
}

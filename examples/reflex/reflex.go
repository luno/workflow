package reflex

import (
	"database/sql"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	"github.com/andrewwormald/workflow/adapters/memrolescheduler"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
	"github.com/andrewwormald/workflow/adapters/reflexstreamer"
	"github.com/andrewwormald/workflow/examples"
	"github.com/andrewwormald/workflow/examples/gettingstarted"
)

func ExampleWorkflow(db *sql.DB, table *rsql.EventsTableInt, cstore reflex.CursorStore) *workflow.Workflow[gettingstarted.GettingStarted, examples.Status] {
	return gettingstarted.WorkflowWithEnum(gettingstarted.Deps{
		EventStreamer: reflexstreamer.New(db, db, table, cstore),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
}

package reflex_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/corverroos/truss"
	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex/rpatterns"
	"github.com/luno/reflex/rsql"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
	"github.com/luno/workflow/examples/gettingstarted"
	"github.com/luno/workflow/examples/reflex"
)

func TestExampleWorkflow(t *testing.T) {
	dbc := ConnectForTesting(t)

	table := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))
	wf := reflex.ExampleWorkflow(dbc, table, rpatterns.MemCursorStore())
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "82347982374982374"
	_, err := wf.Trigger(ctx, foreignID, examples.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, foreignID, examples.StatusReadTheDocs, gettingstarted.GettingStarted{
		ReadTheDocs: "✅",
	})

	workflow.Require(t, wf, foreignID, examples.StatusFollowedTheExample, gettingstarted.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})

	workflow.Require(t, wf, foreignID, examples.StatusCreatedAFunExample, gettingstarted.GettingStarted{
		ReadTheDocs:       "✅",
		FollowAnExample:   "✅",
		CreateAFunExample: "✅",
	})
}

var tables = []string{
	`
	create table my_events_table (
	  id bigint not null auto_increment,
	  foreign_id bigint not null,
	  timestamp datetime not null,
	  type bigint not null default 0,
	  metadata blob,
	  
  	  primary key (id)
	);
`,
}

// ConnectForTesting returns a database connection for a temp database with latest schema.
func ConnectForTesting(t *testing.T) *sql.DB {
	return truss.ConnectForTesting(t, tables...)
}

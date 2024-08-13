package example_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/corverroos/truss"
	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow"
	"github.com/luno/workflow/example/gettingstarted"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/reflexstreamer"
	rexample "github.com/luno/workflow/adapters/reflexstreamer/example"
)

func TestExampleWorkflow(t *testing.T) {
	dbc := ConnectForTesting(t)

	table := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))
	wf := rexample.ExampleWorkflow(dbc, table, rpatterns.MemCursorStore())

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	foreignID := "82347982374982374"
	_, err := wf.Trigger(ctx, foreignID, _example.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, foreignID, _example.StatusReadTheDocs, gettingstarted.GettingStarted{
		ReadTheDocs: "✅",
	})

	workflow.Require(t, wf, foreignID, _example.StatusFollowedTheExample, gettingstarted.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})

	workflow.Require(t, wf, foreignID, _example.StatusCreatedAFunExample, gettingstarted.GettingStarted{
		ReadTheDocs:       "✅",
		FollowAnExample:   "✅",
		CreateAFunExample: "✅",
	})
}

func TestExampleReflexConsumerStream(t *testing.T) {
	dbc := ConnectForTesting(t)

	table := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))
	wf := rexample.ExampleWorkflow(dbc, table, rpatterns.MemCursorStore())

	ctx, cancel := context.WithCancel(context.Background())
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	foreignID := "82347982374982374"
	_, err := wf.Trigger(ctx, foreignID, _example.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, foreignID, _example.StatusCreatedAFunExample, gettingstarted.GettingStarted{
		ReadTheDocs:       "✅",
		FollowAnExample:   "✅",
		CreateAFunExample: "✅",
	})

	spec := reflex.NewSpec(
		reflexstreamer.StreamFunc(dbc, table, wf.Name),
		rpatterns.MemCursorStore(),
		reflex.NewConsumer("something", func(ctx context.Context, event *reflex.Event) error {
			switch event.IDInt() {
			case 1:
				require.Equal(t, int(_example.StatusStarted), event.Type.ReflexType())
			case 2:
				require.Equal(t, int(_example.StatusReadTheDocs), event.Type.ReflexType())
			case 3:
				require.Equal(t, int(_example.StatusFollowedTheExample), event.Type.ReflexType())
			case 4:
				require.Equal(t, int(_example.StatusCreatedAFunExample), event.Type.ReflexType())
			}

			// End the test
			cancel()

			return nil
		}),
	)

	err = reflex.Run(ctx, spec)
	jtest.Require(t, context.Canceled, err)
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

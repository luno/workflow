package connectors_test

import (
	"context"
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/example"
	"github.com/luno/workflow/example/connectors"
	"github.com/luno/workflow/example/gettingstarted"
)

func TestConnectStreamParallelConsumer(t *testing.T) {
	foreignID := "andrewwormald"

	now := time.Date(2024, time.April, 9, 0, 0, 0, 0, time.UTC)
	events := []workflow.ConnectorEvent{
		{
			ID:        "1",
			ForeignID: foreignID,
			CreatedAt: now,
		},
	}

	w := connectors.Workflow(connectors.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
		Connector:     memstreamer.NewConnector(events),
	})

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	workflow.Require(t, w, foreignID, example.StatusFollowedTheExample, gettingstarted.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})
}

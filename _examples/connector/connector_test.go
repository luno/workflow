package connector_test

import (
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"

	"github.com/luno/workflow/_examples/connector"
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

	w := connector.Workflow(connector.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
		Connector:     memstreamer.NewConnector(events),
	})

	ctx := t.Context()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	workflow.Require(t, w, foreignID, connector.StatusFollowedTheExample, connector.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})
}

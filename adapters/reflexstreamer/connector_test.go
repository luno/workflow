package reflexstreamer_test

import (
	"context"
	"testing"

	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"

	"github.com/luno/workflow/adapters/reflexstreamer"
)

func TestConnector(t *testing.T) {
	adaptertest.RunConnectorTest(t, func(seedEvents []workflow.ConnectorEvent) workflow.ConnectorConstructor {
		eventsTable := rsql.NewEventsTable("external_events", rsql.WithEventMetadataField("metadata"))
		dbc := ConnectForTesting(t)
		cTable := rsql.NewCursorsTable("cursors")

		ctx := context.Background()

		for _, event := range seedEvents {
			notify, err := eventsTable.Insert(ctx, dbc, event.ForeignID, reflexstreamer.EventType(1))
			if err != nil {
				originalErr := err
				t.Fatal("failed to insert event", event.ForeignID, originalErr.Error())
			}

			notify()
		}

		return reflexstreamer.NewConnector(eventsTable.ToStream(dbc), cTable.ToStore(dbc), reflexstreamer.DefaultReflexTranslator)
	})
}

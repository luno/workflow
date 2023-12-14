package reflexstreamer_test

import (
	"testing"

	"github.com/luno/reflex/rpatterns"
	"github.com/luno/reflex/rsql"

	"github.com/andrewwormald/workflow/adapters/reflexstreamer"
	adapter "github.com/andrewwormald/workflow/adapters/testing"
)

func TestStreamer(t *testing.T) {
	eventsTable := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))
	dbc := ConnectForTesting(t)
	constructor := reflexstreamer.New(dbc, dbc, eventsTable, rpatterns.MemCursorStore())
	adapter.TestStreamer(t, constructor)
}

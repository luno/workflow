package reflexstreamer_test

import (
	"testing"

	"github.com/luno/reflex/rsql"

	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/reflexstreamer"
)

func TestStreamer(t *testing.T) {
	eventsTable := rsql.NewEventsTableInt("my_events_table", rsql.WithEventMetadataField("metadata"))
	dbc := ConnectForTesting(t)
	cTable := rsql.NewCursorsTable("cursors")
	constructor := reflexstreamer.New(dbc, dbc, eventsTable, cTable.ToStore(dbc))
	adaptertest.RunEventStreamerTest(t, constructor)
}

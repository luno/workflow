package reflexstreamer_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/reflexstreamer"
)

func TestStreamer(t *testing.T) {
	eventsTable := rsql.NewEventsTableInt("workflow_events", rsql.WithEventMetadataField("metadata"))
	dbc := ConnectForTesting(t)
	cTable := rsql.NewCursorsTable("cursors")
	constructor := reflexstreamer.New(dbc, dbc, eventsTable, cTable.ToStore(dbc))
	adaptertest.RunEventStreamerTest(t, constructor)
}

func TestStreamerGapFiller(t *testing.T) {
	eventsTable := rsql.NewEventsTableInt("workflow_events", rsql.WithEventMetadataField("metadata"))
	dbc := ConnectForTesting(t)
	cTable := rsql.NewCursorsTable("cursors")
	_, err := dbc.Exec("insert into workflow_events set id=1, foreign_id=98327498324, timestamp=now(3), type=1, metadata=?;", "{}")
	jtest.RequireNil(t, err)

	_, err = dbc.Exec("insert into workflow_events set id=3, foreign_id=98579438574, timestamp=now(3), type=2, metadata=?;", "{}")
	jtest.RequireNil(t, err)

	constructor := reflexstreamer.New(dbc, dbc, eventsTable, cTable.ToStore(dbc))
	consumer, err := constructor.NewConsumer(context.Background(), "", "")
	jtest.RequireNil(t, err)

	for range 2 {
		_, ack, err := consumer.Recv(context.Background())
		jtest.RequireNil(t, err)

		err = ack()
		jtest.RequireNil(t, err)
	}

	rows, err := dbc.Query("select `id`from workflow_events")
	jtest.RequireNil(t, err)
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		err := rows.Scan(&id)
		jtest.RequireNil(t, err)

		ids = append(ids, id)
	}

	require.Equal(t, []int64{1, 2, 3}, ids)
}

type status int

var (
	statusUnknown status = 0
	statusStart   status = 1
	statusMiddle  status = 2
	statusEnd     status = 3
)

func (s status) String() string {
	switch s {
	case statusStart:
		return "Start"
	case statusMiddle:
		return "Middle"
	case statusEnd:
		return "End"
	default:
		return "Unknown"
	}
}

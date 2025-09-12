package reflexstreamer_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/corverroos/truss"
	"github.com/luno/reflex"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/reflexstreamer"
)

var tables = []string{
	`
	create table workflow_events (
	  id bigint not null auto_increment,
	  foreign_id varchar(255) not null,
	  timestamp datetime not null,
	  type int not null default 0,
	  metadata blob,
	  
  	  primary key (id)
	);
`,
	`
create table cursors (
    id varchar(255) not null,
    last_event_id bigint not null,
    updated_at datetime(3) not null,

    primary key (id)
);
`,
	`
	create table external_events (
	  id bigint not null auto_increment,
	  foreign_id varchar(255) not null,
	  timestamp datetime not null,
	  type int not null default 0,
	  metadata blob,
	  
  	  primary key (id)
	);
`,
}

// ConnectForTesting returns a database connection for a temp database with latest schema.
func ConnectForTesting(t *testing.T) *sql.DB {
	return truss.ConnectForTesting(t, tables...)
}

func TestDefaultTranslators(t *testing.T) {
	now := time.Date(2024, time.April, 9, 0, 0, 0, 0, time.UTC)
	reflexEvent := reflex.Event{
		ID:        "1",
		Type:      reflexstreamer.EventType(1),
		ForeignID: "9",
		MetaData:  []byte("my meta"),
		Timestamp: now,
	}

	connectorEvent, err := reflexstreamer.DefaultReflexTranslator(&reflexEvent)
	require.NoError(t, err)

	translated, err := reflexstreamer.DefaultConnectorTranslator(connectorEvent)
	require.NoError(t, err)

	require.Equal(t, reflexEvent.ID, translated.ID)
	require.Equal(t, reflexEvent.Type.ReflexType(), translated.Type.ReflexType())
	require.Equal(t, reflexEvent.ForeignID, translated.ForeignID)
	require.Equal(t, reflexEvent.MetaData, translated.MetaData)
	require.Equal(t, reflexEvent.Timestamp, translated.Timestamp)
}

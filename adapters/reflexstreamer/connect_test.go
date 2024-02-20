package reflexstreamer_test

import (
	"database/sql"
	"testing"

	"github.com/corverroos/truss"
)

var tables = []string{
	`
	create table my_events_table (
	  id bigint not null auto_increment,
	  foreign_id bigint not null,
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
}

// ConnectForTesting returns a database connection for a temp database with latest schema.
func ConnectForTesting(t *testing.T) *sql.DB {
	return truss.ConnectForTesting(t, tables...)
}

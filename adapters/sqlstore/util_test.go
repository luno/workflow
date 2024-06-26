package sqlstore_test

import (
	"database/sql"
	"testing"

	"github.com/corverroos/truss"
	_ "github.com/go-sql-driver/mysql"
)

var migrations = []string{
	`
	create table workflow_records (
		id                     bigint not null auto_increment,
		workflow_name           varchar(255) not null,
		foreign_id             varchar(255) not null,
		run_id                 varchar(255) not null,
		run_state              int not null,
		status                 int not null,
		object                 blob not null,
		created_at             datetime(3) not null,
		updated_at             datetime(3) not null,

		primary key(id),

		index by_workflow_name_status (workflow_name, status),
    	index by_workflow_name_foreign_id_run_id_status (workflow_name, foreign_id, run_id, status)
	)`,
	`
	create table workflow_outbox (
		id                 bigint not null auto_increment,
		workflow_name       varchar(255) not null,
		data               blob,
		created_at         datetime(3) not null,
	
	
		primary key (id)
	);
	`,
}

func ConnectForTesting(t *testing.T) *sql.DB {
	return truss.ConnectForTesting(t, migrations...)
}

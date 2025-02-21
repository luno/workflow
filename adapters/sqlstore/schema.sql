-- Example of required schema. Name of the table can be different and must match name provided to sqlstore.new("{{table_name}}"...)
create table workflow_records (
    workflow_name          varchar(255) not null,
    foreign_id             varchar(255) not null,
    run_id                 varchar(255) not null,
    run_state              int not null,
    status                 int not null,
    object                 longblob not null,
    created_at             datetime(3) not null,
    updated_at             datetime(3) not null,

    primary key(run_id),

    index by_workflow_name_foreign_id_run_id_status (workflow_name, foreign_id, run_id, status),
    index by_workflow_name_foreign_id_created_at (workflow_name, foreign_id, created_at desc),
    index by_workflow_name_status_created_at (workflow_name, status, created_at desc),
    index by_run_id_created_at (run_id, created_at),
    index by_status_run_state_created_at (run_state, status, created_at),
    index by_run_state (run_state)
);

create table workflow_outbox (
    id                 varchar(255) not null,
    workflow_name      varchar(255) not null,
    data               blob,
    created_at         datetime(3) not null,

    primary key (id)
);
-- Example of required schema. Name of the table can be different and must match name provided to sqlstore.new("{{table_name}}"...)
create table workflow_records (
    workflow_name          varchar(255) not null,
    foreign_id             varchar(255) not null,
    run_id                 varchar(255) not null,
    run_state              int not null,
    status                 int not null,
    object                 JSON not null,
    created_at             datetime(3) not null,
    updated_at             datetime(3) not null,
    meta                   JSON not null,

    primary key(run_id),

    index by_workflow_name_foreign_id_status (workflow_name, foreign_id, status),
    index by_run_state (run_state),
    index by_created_at (created_at)
);

create table workflow_outbox (
    id                 varchar(255) not null,
    workflow_name      varchar(255) not null,
    data               blob,
    created_at         datetime(3) not null,

    primary key (id),

    index by_workflow_name (workflow_name)
);
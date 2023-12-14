-- Example of required schema. Name of the table can be different and must match name provided to sqlstore.new("{{table_name}}"...)
create table workflow_records (
    id                     bigint not null auto_increment,
    workflow_name           varchar(255) not null,
    foreign_id             varchar(255) not null,
    run_id                 varchar(255) not null,
    status                 int not null,
    object                 longblob not null,
    is_start               bool not null,
    is_end                 bool not null,
    created_at             datetime(3) not null,

    primary key(id),

    index by_workflow_name_status (workflow_name, status),
    index by_workflow_name_foreign_id_run_id (workflow_name, foreign_id, run_id),
    index by_workflow_name_foreign_id_run_id_status (workflow_name, foreign_id, run_id, status)
);

create table workflow_timeouts (
    id                     bigint not null auto_increment,
    workflow_name           varchar(255) not null,
    foreign_id             varchar(255) not null,
    run_id                 varchar(255) not null,
    status                 varchar(255) not null,
    completed              bool not null default false,
    expire_at              datetime(3) not null,
    created_at             datetime(3) not null,

    primary key(id),

    index by_completed_expire_at (completed, expire_at),
    index by_workflow_name_status (workflow_name, status)
);
-- Workflow SQL Example Schema
-- This schema is automatically loaded when docker-compose starts MariaDB

CREATE TABLE IF NOT EXISTS workflow_records (
    workflow_name          VARCHAR(255) NOT NULL,
    foreign_id             VARCHAR(255) NOT NULL,
    run_id                 VARCHAR(255) NOT NULL,
    run_state              INT NOT NULL,
    status                 INT NOT NULL,
    object                 LONGBLOB NOT NULL,
    created_at             DATETIME(3) NOT NULL,
    updated_at             DATETIME(3) NOT NULL,
    meta                   BLOB,

    PRIMARY KEY(run_id),

    INDEX by_workflow_name_foreign_id_status (workflow_name, foreign_id, status),
    INDEX by_run_state (run_state),
    INDEX by_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS workflow_outbox (
    id                 VARCHAR(255) NOT NULL,
    workflow_name      VARCHAR(255) NOT NULL,
    data               BLOB,
    created_at         DATETIME(3) NOT NULL,

    PRIMARY KEY (id),

    INDEX by_workflow_name (workflow_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

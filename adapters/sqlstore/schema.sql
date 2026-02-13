-- =============================================================================
-- Workflow Library - SQL Database Schema
-- =============================================================================
-- This schema supports any SQL database (MySQL, MariaDB, PostgreSQL, etc.)
-- 
-- For detailed setup instructions, see: docs/database-setup.md
--
-- Quick Start:
--   1. Create a database for workflows
--   2. Run this schema file to create tables
--   3. Configure sqlstore with matching table names
--
-- Example: sqlstore.New(db, "workflow_records", "workflow_outbox")
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Table: workflow_records
-- -----------------------------------------------------------------------------
-- Stores the state and data for each workflow run instance.
-- This is the primary table that tracks all workflow executions.
--
-- Table name can be customised and must match the name provided to sqlstore.New()
-- -----------------------------------------------------------------------------
create table workflow_records (
    -- Workflow identifier: name of the workflow definition (e.g., "order-processor")
    -- Used to group all runs of the same workflow type
    workflow_name          varchar(255) not null,

    -- Business identifier: your domain-specific ID (e.g., "order-12345", "user-email@example.com")
    -- Allows you to find workflow runs by business entity
    -- Multiple runs can share the same foreign_id (e.g., retry scenarios)
    foreign_id             varchar(255) not null,

    -- Unique run identifier: system-generated UUID for this specific execution
    -- Primary key ensuring each run instance is uniquely identifiable
    run_id                 varchar(255) not null,

    -- System lifecycle state: indicates workflow engine state (not business state)
    -- Values: 1=Initiated, 2=Running, 3=Completed, 4=Paused, 5=Cancelled, 6=Deleted, 7=DataDeleted
    -- This is managed by the workflow engine automatically
    run_state              int not null,

    -- Business status: your custom enum defining business process state
    -- Example: 1=Created, 2=PaymentProcessed, 3=Shipped, 4=Delivered
    -- This is what you define in your workflow steps
    status                 int not null,

    -- Workflow data: serialised workflow object (your custom type)
    -- Stores the complete state of your business entity
    -- Type depends on database: LONGBLOB (MySQL/MariaDB), BYTEA (PostgreSQL)
    object                 longblob not null,

    -- Audit: when this workflow run was first created
    -- Uses datetime(3) for millisecond precision
    created_at             datetime(3) not null,

    -- Audit: when this workflow run was last modified
    -- Updated on every state transition
    updated_at             datetime(3) not null,

    -- Optional metadata: arbitrary data for custom use cases
    -- Can store additional context, tags, or tracking information
    meta                   blob,

    -- Primary key on run_id for fast lookups by execution instance
    primary key(run_id),

    -- Composite index for common query pattern: "find runs for this workflow and business ID at specific status"
    -- Example query: WHERE workflow_name = 'orders' AND foreign_id = 'order-123' AND status = 2
    index by_workflow_name_foreign_id_status (workflow_name, foreign_id, status),

    -- Index for workflow engine to find runs by lifecycle state
    -- Used internally to process running/paused workflows
    index by_run_state (run_state),

    -- Index for time-based queries and cleanup operations
    -- Useful for archiving old records or time-based reporting
    index by_created_at (created_at)
);

-- -----------------------------------------------------------------------------
-- Table: workflow_outbox
-- -----------------------------------------------------------------------------
-- Implements the Transactional Outbox Pattern for reliable event publishing.
-- 
-- When a workflow step completes, changes are committed to workflow_records
-- and events are written to this outbox table in the same transaction.
-- A background process polls this table and publishes events to the event stream.
--
-- This ensures exactly-once processing: if the transaction fails, neither the
-- state change nor the event is persisted. If it succeeds, both are persisted
-- together, and the event will eventually be published and removed from outbox.
-- -----------------------------------------------------------------------------
create table workflow_outbox (
    -- Unique event identifier (UUID)
    -- Prevents duplicate event processing
    id                 varchar(255) not null,

    -- Workflow this event belongs to
    -- Allows querying outbox events for a specific workflow
    workflow_name      varchar(255) not null,

    -- Event payload: serialised event data to be published
    -- Contains the status transition and any metadata
    data               blob,

    -- When this event was created
    -- Used for ordering and monitoring event lag
    created_at         datetime(3) not null,

    primary key (id),

    -- Index to efficiently poll outbox events for a specific workflow
    -- The outbox processor queries: WHERE workflow_name = 'xxx' ORDER BY created_at LIMIT N
    index by_workflow_name (workflow_name)
);

-- =============================================================================
-- Performance Notes
-- =============================================================================
-- 1. Connection Pool: Configure appropriately for your workload
--    Example: db.SetMaxOpenConns(50), db.SetMaxIdleConns(10)
--
-- 2. Index Usage: The provided indexes cover common access patterns
--    Monitor slow queries and add custom indexes as needed
--
-- 3. Data Retention: Implement archival strategy for old workflow_records
--    Example: DELETE completed runs older than 30 days
--
-- 4. Outbox Cleanup: Events are automatically deleted after publishing
--    If events accumulate, check your event streamer configuration
--
-- 5. Character Set: Use utf8mb4 for full Unicode support (MySQL/MariaDB)
--    Example: CREATE TABLE ... DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
--
-- 6. Storage Engine: Use InnoDB (MySQL/MariaDB) for transaction support
--    Example: CREATE TABLE ... ENGINE=InnoDB
--
-- For detailed tuning guidance, see: docs/database-setup.md
-- =============================================================================

-- =============================================================================
-- PostgreSQL Adaptation
-- =============================================================================
-- For PostgreSQL, modify as follows:
--   - LONGBLOB → BYTEA
--   - BLOB → BYTEA
--   - DATETIME(3) → TIMESTAMP(3)
--   - INDEX syntax → CREATE INDEX idx_name ON table_name(columns)
--
-- See docs/database-setup.md for PostgreSQL-specific schema
-- =============================================================================
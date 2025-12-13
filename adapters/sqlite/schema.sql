-- SQLite schema for workflow adapter

-- Records table for RecordStore
CREATE TABLE IF NOT EXISTS workflow_records (
    workflow_name    TEXT NOT NULL,
    foreign_id       TEXT NOT NULL,
    run_id           TEXT NOT NULL PRIMARY KEY,
    run_state        INTEGER NOT NULL,
    status           INTEGER NOT NULL,
    object           BLOB NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    meta             BLOB
);

CREATE INDEX IF NOT EXISTS idx_workflow_name_foreign_id_status
    ON workflow_records (workflow_name, foreign_id, status);
CREATE INDEX IF NOT EXISTS idx_run_state
    ON workflow_records (run_state);
CREATE INDEX IF NOT EXISTS idx_created_at
    ON workflow_records (created_at);

-- Outbox table for transactional outbox pattern
CREATE TABLE IF NOT EXISTS workflow_outbox (
    id            TEXT NOT NULL PRIMARY KEY,
    workflow_name TEXT NOT NULL,
    data          BLOB,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_outbox_workflow_name
    ON workflow_outbox (workflow_name);

-- Timeout records table for TimeoutStore
CREATE TABLE IF NOT EXISTS workflow_timeouts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_name TEXT NOT NULL,
    foreign_id    TEXT NOT NULL,
    run_id        TEXT NOT NULL,
    status        INTEGER NOT NULL,
    completed     BOOLEAN NOT NULL DEFAULT FALSE,
    expire_at     DATETIME NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_timeout_workflow_name
    ON workflow_timeouts (workflow_name);
CREATE INDEX IF NOT EXISTS idx_timeout_expire_at
    ON workflow_timeouts (expire_at);
CREATE INDEX IF NOT EXISTS idx_timeout_status
    ON workflow_timeouts (status);

-- Events table for EventStreamer
CREATE TABLE IF NOT EXISTS workflow_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    topic      TEXT NOT NULL,
    foreign_id TEXT NOT NULL,
    type       INTEGER NOT NULL,
    headers    TEXT NOT NULL, -- JSON encoded headers
    data       BLOB,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_topic
    ON workflow_events (topic);
CREATE INDEX IF NOT EXISTS idx_events_topic_id
    ON workflow_events (topic, id);
CREATE INDEX IF NOT EXISTS idx_events_created_at
    ON workflow_events (created_at);

-- Consumer cursors table for EventStreamer
CREATE TABLE IF NOT EXISTS workflow_cursors (
    topic        TEXT NOT NULL,
    consumer     TEXT NOT NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (topic, consumer)
);
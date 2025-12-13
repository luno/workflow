package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open creates a new SQLite database connection with optimal settings for workflow usage.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure SQLite for optimal performance and reliability
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // Enable Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL", // Good balance of safety and performance
		"PRAGMA cache_size=10000",   // Increase cache size for better performance
		"PRAGMA foreign_keys=ON",    // Enable foreign key constraints
		"PRAGMA temp_store=MEMORY",  // Store temporary tables in memory
		"PRAGMA busy_timeout=5000",  // Wait up to 5 seconds for locks
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return db, nil
}

// InitSchema creates all required tables for the workflow adapters.
func InitSchema(db *sql.DB) error {
	schema := `
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
);`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	return nil
}

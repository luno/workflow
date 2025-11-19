package sqlite_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	_ "modernc.org/sqlite"

	"github.com/luno/workflow/adapters/sqlite"
)

func TestRecordStore(t *testing.T) {
	adaptertest.RunRecordStoreTest(t, func() workflow.RecordStore {
		db := connectForTesting(t)
		return sqlite.NewRecordStore(db)
	})
}

func connectForTesting(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Configure SQLite for better concurrency
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // Enable Write-Ahead Logging
		"PRAGMA synchronous=NORMAL", // Good balance of safety and performance
		"PRAGMA busy_timeout=10000", // Wait up to 10 seconds for locks
		"PRAGMA cache_size=10000",   // Increase cache size
		"PRAGMA temp_store=MEMORY",  // Store temporary tables in memory
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			t.Fatalf("failed to set pragma %s: %v", pragma, err)
		}
	}

	// Create schema
	schemaPath := filepath.Join(getPackageDir(t), "schema.sql")
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}

	if _, err := db.Exec(string(schemaSQL)); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func getPackageDir(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	return wd
}

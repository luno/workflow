package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/sqlite"
)

func TestSQLiteAdapterExample(t *testing.T) {
	// Create a temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "example.db")

	// Open SQLite database with optimized settings
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Initialize schema
	err = sqlite.InitSchema(db)
	require.NoError(t, err)

	// Create adapters
	recordStore := sqlite.NewRecordStore(db)
	timeoutStore := sqlite.NewTimeoutStore(db)
	eventStreamer := sqlite.NewEventStreamer(db)

	ctx := context.Background()

	// Example: RecordStore usage
	record := &workflow.Record{
		WorkflowName: "example_workflow",
		ForeignID:    "user-123",
		RunID:        "run-456",
		RunState:     workflow.RunStateInitiated,
		Status:       1,
		Object:       []byte(`{"data":"example"}`),
		Meta:         workflow.Meta{StatusDescription: "example status"},
	}

	err = recordStore.Store(ctx, record)
	require.NoError(t, err)

	// Lookup record
	retrieved, err := recordStore.Lookup(ctx, "run-456")
	require.NoError(t, err)
	require.Equal(t, "user-123", retrieved.ForeignID)

	// Example: TimeoutStore usage
	expireAt := time.Now().Add(1 * time.Hour)
	err = timeoutStore.Create(ctx, "example_workflow", "user-123", "run-456", 1, expireAt)
	require.NoError(t, err)

	timeouts, err := timeoutStore.List(ctx, "example_workflow")
	require.NoError(t, err)
	require.Len(t, timeouts, 1)

	// Example: EventStreamer usage (basic)
	sender, err := eventStreamer.NewSender(ctx, "example_topic")
	require.NoError(t, err)
	defer sender.Close()

	headers := map[workflow.Header]string{
		workflow.HeaderTopic:        "example_topic",
		workflow.HeaderWorkflowName: "example_workflow",
		workflow.HeaderForeignID:    "user-123",
	}

	err = sender.Send(ctx, "user-123", 1, headers)
	require.NoError(t, err)

	t.Log("SQLite adapters created successfully!")
	t.Log("Database file:", dbPath)
}

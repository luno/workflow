package testing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/andrewwormald/workflow"
)

func TestRecordStore(t *testing.T, factory func() workflow.RecordStore) {
	tests := []func(t *testing.T, store workflow.RecordStore){
		testStore_Latest,
		testStore_Lookup,
		testStore_Store,
	}

	for _, test := range tests {
		storeForTesting := factory()
		test(t, storeForTesting)
	}
}

func testStore_Latest(t *testing.T, store workflow.RecordStore) {
	t.Run("Latest", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		foreignID := "Andrew Wormald"
		runID := "LSDKLJFN-SKDFJB-WERLTBE"

		type example struct {
			name string
		}

		e := example{name: foreignID}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		createdAt := time.Now()

		wr := &workflow.WireRecord{
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusStarted),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		var counter int
		counterPtr := &counter
		eventEmitter := func(id int64) error {
			*counterPtr += 1
			return nil
		}

		err = store.Store(ctx, wr, eventEmitter)
		jtest.RequireNil(t, err)

		expected := workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusStarted),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		latest, err := store.Latest(ctx, workflowName, foreignID)
		jtest.RequireNil(t, err)

		recordIsEqual(t, expected, *latest)

		wr = latest
		wr.Status = int(statusEnd)
		wr.IsStart = false
		wr.IsEnd = true
		err = store.Store(ctx, wr, eventEmitter)
		jtest.RequireNil(t, err)

		require.Equal(t, 2, *counterPtr)

		expected = workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusEnd),
			Object:       b,
			IsStart:      false,
			IsEnd:        true,
			CreatedAt:    createdAt,
		}

		latest, err = store.Latest(ctx, workflowName, foreignID)
		jtest.RequireNil(t, err)
		recordIsEqual(t, expected, *latest)
	})
}

func testStore_Lookup(t *testing.T, store workflow.RecordStore) {
	t.Run("Lookup", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		foreignID := "Andrew Wormald"
		runID := "LSDKLJFN-SKDFJB-WERLTBE"

		type example struct {
			name string
		}

		e := example{name: foreignID}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		createdAt := time.Now()

		wr := &workflow.WireRecord{
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusStarted),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		eventEmitter := func(id int64) error {
			return nil
		}

		err = store.Store(ctx, wr, eventEmitter)
		jtest.RequireNil(t, err)

		expected := workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusStarted),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		latest, err := store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		recordIsEqual(t, expected, *latest)
	})
}

func testStore_Store(t *testing.T, store workflow.RecordStore) {
	t.Run("RecordStore", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		foreignID := "Andrew Wormald"
		runID := "LSDKLJFN-SKDFJB-WERLTBE"

		type example struct {
			name string
		}

		e := example{name: foreignID}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		createdAt := time.Now()

		wr := &workflow.WireRecord{
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusStarted),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		eventEmitter := func(id int64) error {
			return nil
		}

		err = store.Store(ctx, wr, eventEmitter)
		jtest.RequireNil(t, err)

		latest, err := store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		latest.Status = int(statusMiddle)

		err = store.Store(ctx, latest, eventEmitter)
		jtest.RequireNil(t, err)

		expected := workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusMiddle),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		recordIsEqual(t, expected, *latest)

		latest, err = store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		latest.Status = int(statusEnd)

		err = store.Store(ctx, latest, eventEmitter)
		jtest.RequireNil(t, err)

		expected = workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			Status:       int(statusEnd),
			IsStart:      true,
			IsEnd:        false,
			Object:       b,
			CreatedAt:    createdAt,
		}

		recordIsEqual(t, expected, *latest)
	})
}

func recordIsEqual(t *testing.T, a, b workflow.WireRecord) {
	require.Equal(t, a.ID, b.ID)
	require.Equal(t, a.WorkflowName, b.WorkflowName)
	require.Equal(t, a.ForeignID, b.ForeignID)
	require.Equal(t, a.RunID, b.RunID)
	require.Equal(t, a.Status, b.Status)
	require.Equal(t, a.Object, b.Object)
	require.Equal(t, a.IsStart, b.IsStart)
	require.Equal(t, a.IsEnd, b.IsEnd)
	require.WithinDuration(t, a.CreatedAt, b.CreatedAt, time.Second*10)
}

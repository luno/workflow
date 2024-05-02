package adaptertest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/luno/workflow"
	"github.com/luno/workflow/internal/outboxpb"
)

func RunRecordStoreTest(t *testing.T, factory func() workflow.RecordStore) {
	tests := []func(t *testing.T, store workflow.RecordStore){
		testLatest,
		testLookup,
		testStore,
		testListOutboxEvents,
		testDeleteOutboxEvent,
		testList,
	}

	for _, test := range tests {
		storeForTesting := factory()
		test(t, storeForTesting)
	}
}

func testLatest(t *testing.T, store workflow.RecordStore) {
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
			RunState:     workflow.RunStateInitiated,
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		err = store.Store(ctx, wr, maker)
		jtest.RequireNil(t, err)

		expected := workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusStarted),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		latest, err := store.Latest(ctx, workflowName, foreignID)
		jtest.RequireNil(t, err)

		recordIsEqual(t, expected, *latest)

		wr = latest
		wr.Status = int(statusEnd)
		wr.RunState = workflow.RunStateCompleted
		err = store.Store(ctx, wr, maker)
		jtest.RequireNil(t, err)

		expected = workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			RunState:     workflow.RunStateCompleted,
			Status:       int(statusEnd),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		latest, err = store.Latest(ctx, workflowName, foreignID)
		jtest.RequireNil(t, err)
		recordIsEqual(t, expected, *latest)
	})
}

func testLookup(t *testing.T, store workflow.RecordStore) {
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
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusStarted),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		err = store.Store(ctx, wr, maker)
		jtest.RequireNil(t, err)

		expected := workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusStarted),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		latest, err := store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		recordIsEqual(t, expected, *latest)
	})
}

func testStore(t *testing.T, store workflow.RecordStore) {
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
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusStarted),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		err = store.Store(ctx, wr, maker)
		jtest.RequireNil(t, err)

		latest, err := store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		latest.Status = int(statusMiddle)

		err = store.Store(ctx, latest, maker)
		jtest.RequireNil(t, err)

		expected := workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusMiddle),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		recordIsEqual(t, expected, *latest)

		latest, err = store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		latest.Status = int(statusEnd)

		err = store.Store(ctx, latest, maker)
		jtest.RequireNil(t, err)

		expected = workflow.WireRecord{
			ID:           1,
			WorkflowName: workflowName,
			ForeignID:    foreignID,
			RunID:        runID,
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusEnd),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		recordIsEqual(t, expected, *latest)
	})
}

func testListOutboxEvents(t *testing.T, store workflow.RecordStore) {
	t.Run("ListOutboxEvents", func(t *testing.T) {
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
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusStarted),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		maker := func(recordID int64) (workflow.OutboxEventData, error) {
			// Record ID would not have been set if it is a new record. Assign the recordID that the Store provides
			wr.ID = recordID
			return workflow.WireRecordToOutboxEventData(*wr, workflow.RunStateInitiated)
		}

		err = store.Store(ctx, wr, maker)
		jtest.RequireNil(t, err)

		ls, err := store.ListOutboxEvents(ctx, workflowName, 1000)
		jtest.RequireNil(t, err)

		require.Equal(t, 1, len(ls))

		require.Equal(t, int64(1), ls[0].ID)
		require.Equal(t, workflowName, ls[0].WorkflowName)

		var r outboxpb.OutboxRecord
		err = proto.Unmarshal(ls[0].Data, &r)
		jtest.RequireNil(t, err)

		require.Equal(t, int32(statusStarted), r.Type)
		require.Equal(t, "my_workflow-1", r.Headers[string(workflow.HeaderTopic)])
		require.Equal(t, "Andrew Wormald", r.Headers[string(workflow.HeaderForeignID)])
		require.Equal(t, "my_workflow", r.Headers[string(workflow.HeaderWorkflowName)])
	})
}

func testDeleteOutboxEvent(t *testing.T, store workflow.RecordStore) {
	t.Run("DeleteOutboxEvent", func(t *testing.T) {
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
			RunState:     workflow.RunStateInitiated,
			Status:       int(statusStarted),
			Object:       b,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		}

		maker := func(recordID int64) (workflow.OutboxEventData, error) {
			// Record ID would not have been set if it is a new record. Assign the recordID that the Store provides
			wr.ID = recordID
			return workflow.WireRecordToOutboxEventData(*wr, workflow.RunStateInitiated)
		}

		err = store.Store(ctx, wr, maker)
		jtest.RequireNil(t, err)

		latest, err := store.Lookup(ctx, 1)
		jtest.RequireNil(t, err)

		latest.Status = int(statusMiddle)

		ls, err := store.ListOutboxEvents(ctx, workflowName, 1000)
		jtest.RequireNil(t, err)

		err = store.DeleteOutboxEvent(ctx, ls[0].ID)
		jtest.RequireNil(t, err)

		ls, err = store.ListOutboxEvents(ctx, workflowName, 1000)
		jtest.RequireNil(t, err)

		require.Equal(t, 0, len(ls))
	})
}

func testList(t *testing.T, store workflow.RecordStore) {
	t.Run("List", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		type example struct {
			value string
		}

		e := example{value: "test"}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		seedCount := 1000
		for i := 0; i < seedCount; i++ {
			newRecord := &workflow.WireRecord{
				WorkflowName: workflowName,
				ForeignID:    fmt.Sprintf("%v", i),
				RunID:        "SLDKFLSD-FLSDKF-SLDKNFL",
				Object:       b,
			}

			err := store.Store(ctx, newRecord, maker)
			jtest.RequireNil(t, err)
		}

		ls, err := store.List(ctx, workflowName, 0, 53, workflow.OrderTypeAscending)
		jtest.RequireNil(t, err)
		require.Equal(t, 53, len(ls))

		ls2, err := store.List(ctx, workflowName, 53, 100, workflow.OrderTypeAscending)
		jtest.RequireNil(t, err)
		require.Equal(t, 100, len(ls2))

		// Make sure the last of the first page is not the same as the first of the next page
		require.NotEqual(t, ls[52].ID, ls2[0])

		ls3, err := store.List(ctx, workflowName, 153, seedCount-153, workflow.OrderTypeAscending)
		jtest.RequireNil(t, err)
		require.Equal(t, seedCount-153, len(ls3))

		// Make sure the last of the first page is not the same as the first of the next page
		require.NotEqual(t, ls3[152].ID, ls3[0])

		// Make sure that if 950 is the offset and we only have 1000 then only 1 item would be returned
		lastPageAsc, err := store.List(ctx, workflowName, 950, 1000, workflow.OrderTypeAscending)
		jtest.RequireNil(t, err)
		require.Equal(t, 50, len(lastPageAsc))
		require.Equal(t, int64(1000), lastPageAsc[len(lastPageAsc)-1].ID)

		lastPageDesc, err := store.List(ctx, workflowName, 950, 1000, workflow.OrderTypeDescending)
		jtest.RequireNil(t, err)
		require.Equal(t, 50, len(lastPageDesc))
		require.Equal(t, int64(1000), lastPageDesc[0].ID)
	})

	t.Run("List - FilterByForeignID", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		type example struct {
			value string
		}

		e := example{value: "test"}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		foreignIDs := []string{"MSDVUI-OBEWF-BYUIOW", "FRELBJK-SRGIUE-RGTJDSF"}
		for _, foreignID := range foreignIDs {
			for i := 0; i < 20; i++ {
				uid, err := uuid.NewUUID()
				jtest.RequireNil(t, err)

				newRecord := &workflow.WireRecord{
					WorkflowName: workflowName,
					Status:       int(statusMiddle),
					ForeignID:    foreignID,
					RunID:        uid.String(),
					Object:       b,
				}

				err = store.Store(ctx, newRecord, maker)
				jtest.RequireNil(t, err)
			}
		}

		ls, err := store.List(ctx, workflowName, 0, 100, workflow.OrderTypeAscending, workflow.FilterByForeignID(foreignIDs[0]))
		jtest.RequireNil(t, err)
		require.Equal(t, 20, len(ls))

		ls2, err := store.List(ctx, workflowName, 0, 100, workflow.OrderTypeAscending, workflow.FilterByForeignID(foreignIDs[1]))
		jtest.RequireNil(t, err)
		require.Equal(t, 20, len(ls2))

		ls3, err := store.List(ctx, workflowName, 0, 100, workflow.OrderTypeAscending, workflow.FilterByForeignID("random"))
		jtest.RequireNil(t, err)
		require.Equal(t, 0, len(ls3))
	})

	t.Run("List - FilterByRunState", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		type example struct {
			value string
		}

		e := example{value: "test"}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		config := map[workflow.RunState]int{
			workflow.RunStateInitiated:   10,
			workflow.RunStateRunning:     100,
			workflow.RunStateCompleted:   20,
			workflow.RunStatePaused:      3,
			workflow.RunStateCancelled:   1,
			workflow.RunStateDataDeleted: 15,
		}
		for runState, count := range config {
			for i := 0; i < count; i++ {
				uid, err := uuid.NewUUID()
				jtest.RequireNil(t, err)

				newRecord := &workflow.WireRecord{
					WorkflowName: workflowName,
					Status:       int(statusMiddle),
					ForeignID:    "1",
					RunState:     runState,
					RunID:        uid.String(),
					Object:       b,
				}

				err = store.Store(ctx, newRecord, maker)
				jtest.RequireNil(t, err)
			}
		}

		for runState, count := range config {
			ls, err := store.List(ctx, workflowName, 0, 100, workflow.OrderTypeAscending, workflow.FilterByRunState(runState))
			jtest.RequireNil(t, err)
			require.Equal(t, count, len(ls))

			for _, l := range ls {
				require.Equal(t, l.RunState, runState)
			}
		}
	})

	t.Run("List - FilterByStatus", func(t *testing.T) {
		ctx := context.Background()
		workflowName := "my_workflow"
		maker := func(recordID int64) (workflow.OutboxEventData, error) { return workflow.OutboxEventData{}, nil }

		type example struct {
			value string
		}

		e := example{value: "test"}
		b, err := json.Marshal(e)
		jtest.RequireNil(t, err)

		config := map[status]int{
			statusStarted: 10,
			statusMiddle:  100,
			statusEnd:     20,
		}
		for status, count := range config {
			for i := 0; i < count; i++ {
				uid, err := uuid.NewUUID()
				jtest.RequireNil(t, err)

				newRecord := &workflow.WireRecord{
					WorkflowName: workflowName,
					Status:       int(status),
					ForeignID:    "1",
					RunState:     workflow.RunStateCompleted,
					RunID:        uid.String(),
					Object:       b,
				}

				err = store.Store(ctx, newRecord, maker)
				jtest.RequireNil(t, err)
			}
		}

		for status, count := range config {
			ls, err := store.List(ctx, workflowName, 0, 100, workflow.OrderTypeAscending, workflow.FilterByStatus(status))
			jtest.RequireNil(t, err)
			require.Equal(t, count, len(ls))

			for _, l := range ls {
				require.Equal(t, l.Status, int(status))
			}
		}
	})
}

func recordIsEqual(t *testing.T, a, b workflow.WireRecord) {
	require.Equal(t, a.ID, b.ID)
	require.Equal(t, a.WorkflowName, b.WorkflowName)
	require.Equal(t, a.ForeignID, b.ForeignID)
	require.Equal(t, a.RunID, b.RunID)
	require.Equal(t, a.Status, b.Status)
	require.Equal(t, a.Object, b.Object)
	require.Equal(t, a.RunState, b.RunState)
	require.WithinDuration(t, a.CreatedAt, b.CreatedAt, time.Second*10)
	require.WithinDuration(t, a.UpdatedAt, b.UpdatedAt, time.Second*10)
}

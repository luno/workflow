package adaptertest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/luno/workflow"
	"github.com/luno/workflow/internal/outboxpb"
)

func RunRecordStoreTest(t *testing.T, factory func() workflow.RecordStore) {
	tests := []func(t *testing.T, factory func() workflow.RecordStore){
		testLatest,
		testLookup,
		testStore,
		testListOutboxEvents,
		testDeleteOutboxEvent,
		testList,
	}

	for _, test := range tests {
		test(t, factory)
	}
}

func testLatest(t *testing.T, factory func() workflow.RecordStore) {
	t.Run("Latest", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		expected := dummyWireRecord(t, "my_workflow")

		err := store.Store(ctx, expected)
		require.Nil(t, err)

		latest, err := store.Latest(ctx, expected.WorkflowName, expected.ForeignID)
		require.Nil(t, err)

		recordIsEqual(t, *expected, *latest)

		expected.Status = int(statusEnd)
		expected.RunState = workflow.RunStateCompleted
		err = store.Store(ctx, expected)
		require.Nil(t, err)

		latest, err = store.Latest(ctx, expected.WorkflowName, expected.ForeignID)
		require.Nil(t, err)
		recordIsEqual(t, *expected, *latest)
	})
}

func testLookup(t *testing.T, factory func() workflow.RecordStore) {
	t.Run("Lookup", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		expected := dummyWireRecord(t, "my_workflow")

		err := store.Store(ctx, expected)
		require.Nil(t, err)

		latest, err := store.Lookup(ctx, expected.RunID)
		require.Nil(t, err)

		recordIsEqual(t, *expected, *latest)
	})
}

func testStore(t *testing.T, factory func() workflow.RecordStore) {
	t.Run("RecordStore", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		expected := dummyWireRecord(t, "my_workflow")

		err := store.Store(ctx, expected)
		require.Nil(t, err)

		latest, err := store.Lookup(ctx, expected.RunID)
		require.Nil(t, err)

		latest.Status = int(statusMiddle)
		expected.Status = int(statusMiddle)

		err = store.Store(ctx, latest)
		require.Nil(t, err)

		recordIsEqual(t, *expected, *latest)

		latest, err = store.Lookup(ctx, expected.RunID)
		require.Nil(t, err)

		latest.Status = int(statusEnd)
		expected.Status = int(statusEnd)

		err = store.Store(ctx, latest)
		require.Nil(t, err)

		recordIsEqual(t, *expected, *latest)
	})
}

func testListOutboxEvents(t *testing.T, factory func() workflow.RecordStore) {
	t.Run("ListOutboxEvents", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		expected := dummyWireRecord(t, "my_workflow")

		err := store.Store(ctx, expected)
		require.Nil(t, err)

		ls, err := store.ListOutboxEvents(ctx, expected.WorkflowName, 1000)
		require.Nil(t, err)

		require.Equal(t, 1, len(ls))

		require.True(t, strings.Contains(ls[0].ID, "outbox-id-"))
		require.Equal(t, expected.WorkflowName, ls[0].WorkflowName)

		var r outboxpb.OutboxRecord
		err = proto.Unmarshal(ls[0].Data, &r)
		require.Nil(t, err)

		require.Equal(t, int32(statusStarted), r.Type)
		require.Equal(t, "my_workflow-1", r.Headers[string(workflow.HeaderTopic)])
		require.Equal(t, "Andrew Wormald", r.Headers[string(workflow.HeaderForeignID)])
		require.Equal(t, "my_workflow", r.Headers[string(workflow.HeaderWorkflowName)])
	})
}

func testDeleteOutboxEvent(t *testing.T, factory func() workflow.RecordStore) {
	t.Run("DeleteOutboxEvent", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		expected := dummyWireRecord(t, "my_workflow")

		err := store.Store(ctx, expected)
		require.Nil(t, err)

		latest, err := store.Lookup(ctx, expected.RunID)
		require.Nil(t, err)

		latest.Status = int(statusMiddle)

		ls, err := store.ListOutboxEvents(ctx, expected.WorkflowName, 1000)
		require.Nil(t, err)

		err = store.DeleteOutboxEvent(ctx, ls[0].ID)
		require.Nil(t, err)

		ls, err = store.ListOutboxEvents(ctx, expected.WorkflowName, 1000)
		require.Nil(t, err)

		require.Equal(t, 0, len(ls))
	})
}

func testList(t *testing.T, factory func() workflow.RecordStore) {
	workflowName := "my_workflow"
	secondWorkflowName := "my_second_workflow"

	t.Run("List with no workflow specified returns all", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		seedCount := 1000
		for i := 0; i < seedCount; i++ {
			name := workflowName
			if i > seedCount/2 {
				name = secondWorkflowName
			}
			err := store.Store(ctx, dummyWireRecord(t, name))
			require.Nil(t, err)
		}

		ls, err := store.List(ctx, "", 0, 53, workflow.OrderTypeAscending)
		require.Nil(t, err)
		require.Equal(t, 53, len(ls))

		ls2, err := store.List(ctx, "", 53, 100, workflow.OrderTypeAscending)
		require.Nil(t, err)
		require.Equal(t, 100, len(ls2))

		// Make sure the last of the first page is not the same as the first of the next page
		require.NotEqual(t, ls[52].RunID, ls2[0].RunID)

		ls3, err := store.List(ctx, "", 153, seedCount-153, workflow.OrderTypeAscending)
		require.Nil(t, err)
		require.Equal(t, seedCount-153, len(ls3))

		// Make sure the last of the first page is not the same as the first of the next page.
		require.NotEqual(t, ls3[152].RunID, ls3[0].RunID)

		// Make sure that if 950 is the offset, and we only have 1000 then only 50 items will be returned.
		lastPageAsc, err := store.List(ctx, "", 950, 1000, workflow.OrderTypeAscending)
		require.Nil(t, err)
		require.Equal(t, 50, len(lastPageAsc))

		lastPageDesc, err := store.List(ctx, "", 950, 1000, workflow.OrderTypeDescending)
		require.Nil(t, err)
		require.Equal(t, 50, len(lastPageDesc))
	})

	t.Run("List - WorkflowName", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		for i := 0; i < 100; i++ {
			newRecord := dummyWireRecord(t, workflowName)
			err := store.Store(ctx, newRecord)
			require.Nil(t, err)

			secondRecord := dummyWireRecord(t, secondWorkflowName)
			err = store.Store(ctx, secondRecord)
			require.Nil(t, err)
		}

		ls, err := store.List(
			ctx,
			workflowName,
			0,
			2000,
			workflow.OrderTypeAscending,
		)
		require.Nil(t, err)
		require.Equal(t, 100, len(ls))

		for _, l := range ls {
			require.Equal(t, l.WorkflowName, workflowName)
		}
	})

	t.Run("List - FilterByForeignID", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		foreignIDs := []string{"MSDVUI-OBEWF-BYUIOW", "FRELBJK-SRGIUE-RGTJDSF"}
		for _, foreignID := range foreignIDs {
			for i := 0; i < 20; i++ {
				wr := dummyWireRecord(t, workflowName)
				wr.ForeignID = foreignID

				err := store.Store(ctx, wr)
				require.Nil(t, err)
			}
		}

		ls, err := store.List(
			ctx,
			workflowName,
			0,
			2000,
			workflow.OrderTypeAscending,
			workflow.FilterByForeignID(foreignIDs[0]),
		)
		require.Nil(t, err)
		require.Equal(t, 20, len(ls))

		ls2, err := store.List(
			ctx,
			workflowName,
			0,
			100,
			workflow.OrderTypeAscending,
			workflow.FilterByForeignID(foreignIDs[1]),
		)
		require.Nil(t, err)
		require.Equal(t, 20, len(ls2))

		ls3, err := store.List(
			ctx,
			workflowName,
			0,
			100,
			workflow.OrderTypeAscending,
			workflow.FilterByForeignID("random"),
		)
		require.Nil(t, err)
		require.Equal(t, 0, len(ls3))
	})

	t.Run("List - FilterByRunState", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
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
				wr := dummyWireRecord(t, workflowName)
				wr.RunState = runState

				err := store.Store(ctx, wr)
				require.Nil(t, err)
			}
		}

		// Testing for one RunState at a time
		for runState, count := range config {
			ls, err := store.List(
				ctx,
				workflowName,
				0,
				2000,
				workflow.OrderTypeAscending,
				workflow.FilterByRunState(runState),
			)
			require.Nil(t, err)
			require.Equal(t, count, len(ls), fmt.Sprintf("Expected to have %v entries of %v", count, runState.String()))

			for _, l := range ls {
				require.Equal(t, l.RunState, runState)
			}
		}

		// Testing for multiple RunStates at a time
		ls, err := store.List(
			ctx,
			workflowName,
			0,
			2000,
			workflow.OrderTypeAscending,
			workflow.FilterByRunState(workflow.RunStateRunning, workflow.RunStatePaused),
		)
		require.Nil(t, err)
		require.Equal(
			t,
			103,
			len(ls),
			fmt.Sprintf(
				"Expected to have %v entries of %v and %v",
				103,
				workflow.RunStateRunning.String(),
				workflow.RunStatePaused.String(),
			),
		)

		for _, l := range ls {
			if l.RunState != workflow.RunStateRunning &&
				l.RunState != workflow.RunStatePaused {
				t.Fatalf("Unexpected run state: %v", l.RunState)
			}
		}
	})

	t.Run("List - FilterByStatus", func(t *testing.T) {
		store := factory()
		ctx := context.Background()
		config := map[status]int{
			statusStarted: 10,
			statusMiddle:  100,
			statusEnd:     20,
		}
		for status, count := range config {
			for i := 0; i < count; i++ {
				newRecord := dummyWireRecord(t, workflowName)
				newRecord.Status = int(status)

				err := store.Store(ctx, newRecord)
				require.Nil(t, err)
			}
		}

		for status, count := range config {
			ls, err := store.List(
				ctx,
				workflowName,
				0,
				100,
				workflow.OrderTypeAscending,
				workflow.FilterByStatus(status),
			)
			require.Nil(t, err)
			require.Equal(t, count, len(ls))

			for _, l := range ls {
				require.Equal(t, l.Status, int(status))
			}
		}
	})
}

func dummyWireRecord(t *testing.T, workflowName string) *workflow.Record {
	foreignID := "Andrew Wormald"
	runID, err := uuid.NewUUID()
	require.Nil(t, err)

	type example struct {
		name string
	}

	e := example{name: foreignID}
	b, err := json.Marshal(e)
	require.Nil(t, err)

	createdAt := time.Now()

	return &workflow.Record{
		WorkflowName: workflowName,
		ForeignID:    foreignID,
		RunID:        runID.String(),
		Status:       int(statusStarted),
		RunState:     workflow.RunStateInitiated,
		Object:       b,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
}

func recordIsEqual(t *testing.T, a, b workflow.Record) {
	require.Equal(t, a.WorkflowName, b.WorkflowName)
	require.Equal(t, a.ForeignID, b.ForeignID)
	require.Equal(t, a.RunID, b.RunID)
	require.Equal(t, a.Status, b.Status)
	require.Equal(t, a.Object, b.Object)
	require.Equal(t, a.RunState, b.RunState)
	require.WithinDuration(t, a.CreatedAt, b.CreatedAt, allowedTimeDeviation)
	require.WithinDuration(t, a.UpdatedAt, b.UpdatedAt, allowedTimeDeviation)
}

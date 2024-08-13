package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
)

func TriggerCallbackOn[Type any, Status StatusType, Payload any](t testing.TB, w *Workflow[Type, Status], foreignID, runID string, waitFor Status, p Payload) {
	if t == nil {
		panic("TriggerCallbackOn can only be used for testing")
	}

	ctx := context.TODO()

	_, err := w.Await(ctx, foreignID, runID, waitFor)
	jtest.RequireNil(t, err)

	b, err := json.Marshal(p)
	jtest.RequireNil(t, err)

	err = w.Callback(ctx, foreignID, waitFor, bytes.NewReader(b))
	jtest.RequireNil(t, err)
}

func AwaitTimeoutInsert[Type any, Status StatusType](t testing.TB, w *Workflow[Type, Status], foreignID, runID string, waitFor Status) {
	if t == nil {
		panic("AwaitTimeout can only be used for testing")
	}

	var found bool
	for !found {
		if w.ctx.Err() != nil {
			return
		}

		ls, err := w.timeoutStore.List(w.ctx, w.Name)
		jtest.RequireNil(t, err)

		for _, l := range ls {
			if l.Status != int(waitFor) {
				continue
			}

			if l.ForeignID != foreignID {
				continue
			}

			if l.RunID != runID {
				continue
			}

			found = true
			break
		}
	}
}

func Require[Type any, Status StatusType](t testing.TB, w *Workflow[Type, Status], foreignID string, waitFor Status, expected Type) {
	if t == nil {
		panic("Require can only be used for testing")
	}

	if !w.statusGraph.IsValid(int(waitFor)) {
		t.Error(fmt.Sprintf(`Status provided is not configured for workflow: "%v" (Workflow: %v)`, waitFor, w.Name))
		return
	}

	testingStore, ok := w.recordStore.(TestingRecordStore)
	if !ok {
		panic("Require function requires TestingRecordStore implementation for record store dependency")
	}

	var runID string
	for runID == "" {
		latest, err := w.recordStore.Latest(context.Background(), w.Name, foreignID)
		if errors.Is(err, ErrRecordNotFound) {
			continue
		} else {
			jtest.RequireNil(t, err)
		}

		runID = latest.RunID
	}

	var wr *WireRecord
	for wr == nil {
		offset := testingStore.SnapshotOffset(w.Name, foreignID, runID)
		snapshots := testingStore.Snapshots(w.Name, foreignID, runID)
		for i, r := range snapshots {
			if offset > i {
				continue
			}

			if r.Status == int(waitFor) {
				wr = r
			}

			testingStore.SetSnapshotOffset(w.Name, foreignID, runID, offset+1)
		}
	}

	var typ Type
	err := json.Unmarshal(wr.Object, &typ)
	jtest.RequireNil(t, err)

	actual := &Run[Type, Status]{
		WireRecord: *wr,
		Status:     Status(wr.Status),
		Object:     &typ,
	}

	require.Equal(t, expected, *actual.Object)
}

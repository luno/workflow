package workflow

import (
	"context"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/graph"
	"github.com/luno/workflow/internal/metrics"
)

type (
	lookupFunc func(ctx context.Context, id int64) (*WireRecord, error)
	storeFunc  func(ctx context.Context, record *WireRecord, maker OutboxEventDataMaker) error

	updater[Type any, Status StatusType] func(ctx context.Context, current Status, next Status, record *Record[Type, Status]) error
)

func newUpdater[Type any, Status StatusType](lookup lookupFunc, store storeFunc, graph *graph.Graph, clock clock.Clock) updater[Type, Status] {
	return func(ctx context.Context, current Status, next Status, record *Record[Type, Status]) error {
		object, err := Marshal(&record.Object)
		if err != nil {
			return err
		}

		runState := RunStateRunning
		isEnd := graph.IsTerminal(int(next))
		if isEnd {
			runState = RunStateCompleted
		}

		updatedRecord := &WireRecord{
			ID:           record.ID,
			WorkflowName: record.WorkflowName,
			ForeignID:    record.ForeignID,
			RunID:        record.RunID,
			RunState:     runState,
			Status:       int(next),
			Object:       object,
			CreatedAt:    record.CreatedAt,
			UpdatedAt:    clock.Now(),
		}

		latest, err := lookup(ctx, updatedRecord.ID)
		if err != nil {
			return err
		}

		// Ensure that the record still has the intended status. If not then another consumer will be processing this
		// record.
		if Status(latest.Status) != current {
			return nil
		}

		err = validateTransition(current, next, graph)
		if err != nil {
			return err
		}

		// Push run state changes for observability
		metrics.RunStateChanges.WithLabelValues(record.WorkflowName, record.RunState.String(), updatedRecord.RunState.String()).Inc()

		return store(ctx, updatedRecord, func(recordID int64) (OutboxEventData, error) {
			// Record ID would not have been set if it is a new record. Assign the recordID that the Store provides
			updatedRecord.ID = recordID
			return WireRecordToOutboxEventData(*updatedRecord, record.RunState)
		})
	}
}

func validateTransition[Status StatusType](current, next Status, graph *graph.Graph) error {
	// Lookup all available transitions from the current status
	nodes := graph.Transitions(int(current))
	if len(nodes) == 0 {
		return errors.New("current status not predefined", j.MKV{
			"current_status": current.String(),
		})
	}

	var found bool
	// Attempt to find the next status amongst the list of valid transitions
	for _, node := range nodes {
		if node == int(next) {
			found = true
			break
		}
	}

	// If no valid transition matches that of the next status then error.
	if !found {
		return errors.New("invalid transition attempted", j.MKV{
			"current_status": current.String(),
			"next_status":    next.String(),
		})
	}

	return nil
}

func updateWireRecord(ctx context.Context, store storeFunc, record *WireRecord, previousRunState RunState) error {
	// Push run state changes for observability
	metrics.RunStateChanges.WithLabelValues(record.WorkflowName, previousRunState.String(), record.RunState.String()).Inc()

	return store(ctx, record, func(recordID int64) (OutboxEventData, error) {
		// Record ID would not have been set if it is a new record. Assign the recordID that the Store provides
		record.ID = recordID
		return WireRecordToOutboxEventData(*record, previousRunState)
	})
}

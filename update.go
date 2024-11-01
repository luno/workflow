package workflow

import (
	"context"
	"fmt"

	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/graph"
	"github.com/luno/workflow/internal/metrics"
)

type (
	lookupFunc func(ctx context.Context, runID string) (*Record, error)
	storeFunc  func(ctx context.Context, record *Record) error

	updater[Type any, Status StatusType] func(ctx context.Context, current Status, next Status, run *Run[Type, Status]) error
)

func newUpdater[Type any, Status StatusType](lookup lookupFunc, store storeFunc, graph *graph.Graph, clock clock.Clock) updater[Type, Status] {
	return func(ctx context.Context, current Status, next Status, record *Run[Type, Status]) error {
		object, err := Marshal(&record.Object)
		if err != nil {
			return err
		}

		runState := RunStateRunning
		isEnd := graph.IsTerminal(int(next))
		if isEnd {
			runState = RunStateCompleted
		}

		updatedRecord := &Record{
			WorkflowName: record.WorkflowName,
			ForeignID:    record.ForeignID,
			RunID:        record.RunID,
			RunState:     runState,
			Status:       int(next),
			Object:       object,
			CreatedAt:    record.CreatedAt,
			UpdatedAt:    clock.Now(),
		}

		latest, err := lookup(ctx, updatedRecord.RunID)
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

		return store(ctx, updatedRecord)
	}
}

func validateTransition[Status StatusType](current, next Status, graph *graph.Graph) error {
	// Lookup all available transitions from the current status
	nodes := graph.Transitions(int(current))
	if len(nodes) == 0 {
		return fmt.Errorf("current status not defined in graph: current=%s", current)
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
		return fmt.Errorf("current status not defined in graph: current=%s, next=%s", current, next)
	}

	return nil
}

func updateRecord(ctx context.Context, store storeFunc, record *Record, previousRunState RunState) error {
	// Push run state changes for observability
	metrics.RunStateChanges.WithLabelValues(record.WorkflowName, previousRunState.String(), record.RunState.String()).Inc()

	return store(ctx, record)
}

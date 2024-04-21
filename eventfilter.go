package workflow

import "strconv"

// EventFilter can be passed to the event streaming implementation to allow specific consumers to have an
// earlier on filtering process. True is returned when the event should be skipped.
type EventFilter func(e *Event) bool

func FilterUsing(e *Event, filters ...EventFilter) bool {
	for _, filter := range filters {
		if mustFilterOut := filter(e); mustFilterOut {
			return true
		}
	}

	return false
}

func shardFilter(shard, totalShards int) EventFilter {
	return func(e *Event) bool {
		if totalShards > 1 {
			return e.ID%int64(totalShards) != int64(shard)-1
		}

		return false
	}
}

func runStateUpdatesFilter() EventFilter {
	return func(e *Event) bool {
		if e.Headers[HeaderPreviousRunState] == "" || e.Headers[HeaderRunState] == "" {
			return false
		}

		intValue, err := strconv.ParseInt(e.Headers[HeaderPreviousRunState], 10, 64)
		if err != nil {
			// NoReturnErr: Ignore failure to parse int from string
			return false
		}

		previous := RunState(intValue)

		intValue, err = strconv.ParseInt(e.Headers[HeaderRunState], 10, 64)
		if err != nil {
			// NoReturnErr: Ignore failure to parse int from string
			return false
		}

		current := RunState(intValue)

		if current.Stopped() {
			return true
		}

		if previous == RunStateInitiated && current == RunStateRunning {
			// Ignore all events generated from moving from Initiated to Running
			return true
		}

		return false
	}
}

func FilterByForeignID(foreignID string) EventFilter {
	return func(e *Event) bool {
		fid, ok := e.Headers[HeaderWorkflowForeignID]
		if !ok {
			return false
		}

		return fid != foreignID
	}
}

func FilterByRunID(runID string) EventFilter {
	return func(e *Event) bool {
		rID, ok := e.Headers[HeaderRunID]
		if !ok {
			return false
		}

		return rID != runID
	}
}

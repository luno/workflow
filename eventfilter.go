package workflow

import (
	"hash/fnv"
	"strconv"
)

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

// ConnectorEventFilter can be passed to the event streaming implementation to allow specific consumers to have an
// earlier on filtering process. True is returned when the event should be skipped.
type ConnectorEventFilter func(e *ConnectorEvent) bool

func FilterConnectorEventUsing(e *ConnectorEvent, filters ...ConnectorEventFilter) bool {
	for _, filter := range filters {
		if mustFilterOut := filter(e); mustFilterOut {
			return true
		}
	}

	return false
}

func shardConnectorEventFilter(shard, totalShards int) ConnectorEventFilter {
	hsh := fnv.New32()
	return func(e *ConnectorEvent) bool {
		if totalShards > 1 {
			hsh.Reset()
			_, err := hsh.Write([]byte(e.ID))
			if err != nil {
				return false
			}

			hash := hsh.Sum32()
			return hash%uint32(totalShards) == uint32(shard)-1
		}

		return false
	}
}

func shardFilter(shard, totalShards int) EventFilter {
	return func(e *Event) bool {
		if totalShards > 1 {
			return e.ID%int64(totalShards) != int64(shard)-1
		}

		return false
	}
}

func filterByForeignID(foreignID string) EventFilter {
	return func(e *Event) bool {
		fid, ok := e.Headers[HeaderForeignID]
		if !ok {
			return false
		}

		return fid != foreignID
	}
}

func filterByRunID(runID string) EventFilter {
	return func(e *Event) bool {
		rID, ok := e.Headers[HeaderRunID]
		if !ok {
			return false
		}

		return rID != runID
	}
}

func filterByRunState(runState RunState) EventFilter {
	return func(e *Event) bool {
		rs, ok := e.Headers[HeaderRunState]
		if !ok {
			return false
		}

		return rs != strconv.FormatInt(int64(runState), 10)
	}
}

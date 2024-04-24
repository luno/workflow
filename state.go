package workflow

import (
	"fmt"

	"github.com/luno/workflow/internal/metrics"
)

//go:generate stringer -type=State
type State int

const (
	StateUnknown  State = 0
	StateShutdown State = 1
	StateRunning  State = 2
	StateIdle     State = 3
)

var stateStrings = map[State]string{
	StateUnknown:  "Unknown",
	StateShutdown: "Shutdown",
	StateRunning:  "Running",
	StateIdle:     "Idle",
}

func (s State) String() string {
	if val, ok := stateStrings[s]; ok {
		return val
	}

	return fmt.Sprintf("State(%d)", s)
}

func (w *Workflow[Type, Status]) updateState(processName string, s State) {
	w.internalStateMu.Lock()
	defer w.internalStateMu.Unlock()

	metrics.ProcessStates.WithLabelValues(w.Name, processName).Set(float64(s))

	w.internalState[processName] = s
}

func (w *Workflow[Type, Status]) States() map[string]State {
	w.internalStateMu.Lock()
	defer w.internalStateMu.Unlock()

	states := make(map[string]State)
	for k, v := range w.internalState {
		states[k] = v
	}

	return states
}

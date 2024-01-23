package workflow

import "github.com/luno/workflow/internal/metrics"

type State int

const (
	StateUnknown  State = 0
	StateShutdown State = 1
	StateRunning  State = 2
	StateIdle     State = 3
)

func (s State) String() string {
	switch s {
	case StateShutdown:
		return "Shutdown"
	case StateRunning:
		return "Running"
	case StateIdle:
		return "Idle"
	default:
		return "Unknown"
	}
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

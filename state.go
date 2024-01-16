package workflow

import "github.com/luno/workflow/internal/metrics"

type State string

const (
	StateUnknown  State = ""
	StateIdle     State = "Idle"
	StateRunning  State = "Running"
	StateShutdown State = "Shutdown"
)

func (w *Workflow[Type, Status]) updateState(processName string, s State) {
	w.internalStateMu.Lock()
	defer w.internalStateMu.Unlock()

	switch s {
	case StateRunning:
		// Set the state to on for this process
		metrics.ProcessStates.WithLabelValues(w.Name, processName).Set(1)
	case StateShutdown:
		// Set the state to off for this process
		metrics.ProcessStates.WithLabelValues(w.Name, processName).Set(0.0)
	}

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

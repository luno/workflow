package workflow

import "github.com/luno/workflow/internal/metrics"

type State string

const (
	StateUnknown  State = ""
	StateShutdown State = "Shutdown"
	StateRunning  State = "Running"
	StateIdle     State = "Idle"
)

func (w *Workflow[Type, Status]) updateState(processName string, s State) {
	w.internalStateMu.Lock()
	defer w.internalStateMu.Unlock()

	switch s {
	case StateIdle:
		metrics.ProcessStates.WithLabelValues(w.Name, processName).Set(2)
	case StateRunning:
		metrics.ProcessStates.WithLabelValues(w.Name, processName).Set(1)
	case StateShutdown:
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

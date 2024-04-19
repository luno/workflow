package workflow

import (
	"fmt"

	"github.com/luno/workflow/internal/metrics"
)

type LifecycleState int

const (
	LifecycleStateUnknown  LifecycleState = 0
	LifecycleStateShutdown LifecycleState = 1
	LifecycleStateRunning  LifecycleState = 2
	LifecycleStateIdle     LifecycleState = 3
)

func (s LifecycleState) String() string {
	switch s {
	case LifecycleStateUnknown:
		return "Unknown"
	case LifecycleStateShutdown:
		return "Shutdown"
	case LifecycleStateRunning:
		return "Running"
	case LifecycleStateIdle:
		return "Idle"
	default:
		return fmt.Sprintf("LifecycleState(%d)", s)
	}
}

func (w *Workflow[Type, Status]) updateLifecycle(processName string, s LifecycleState) {
	w.processLifecycleMu.Lock()
	defer w.processLifecycleMu.Unlock()

	metrics.ProcessStates.WithLabelValues(w.Name, processName).Set(float64(s))

	w.processLifecycles[processName] = s
}

func (w *Workflow[Type, Status]) Lifecycles() map[string]LifecycleState {
	w.processLifecycleMu.Lock()
	defer w.processLifecycleMu.Unlock()

	states := make(map[string]LifecycleState)
	for k, v := range w.processLifecycles {
		states[k] = v
	}

	return states
}

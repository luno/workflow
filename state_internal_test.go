package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateState(t *testing.T) {
	t.Parallel()
	b := NewBuilder[string, testStatus]("example")
	b.AddStep(statusStart, func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
		return statusEnd, nil
	}, statusEnd)

	w := Workflow[string, testStatus]{
		internalState: make(map[string]State),
	}

	require.Equal(t, map[string]State{}, w.States())

	w.updateState("start-consumer-1-of-1", StateIdle)

	require.Equal(t, map[string]State{
		"start-consumer-1-of-1": StateIdle,
	}, w.States())

	w.updateState("start-consumer-1-of-1", StateRunning)

	require.Equal(t, map[string]State{
		"start-consumer-1-of-1": StateRunning,
	}, w.States())

	w.updateState("start-consumer-1-of-1", StateShutdown)

	require.Equal(t, map[string]State{
		"start-consumer-1-of-1": StateShutdown,
	}, w.States())
}

func TestStateString(t *testing.T) {
	states := []State{
		StateUnknown,
		StateShutdown,
		StateRunning,
		StateIdle,
	}

	for _, state := range states {
		require.Equal(t, stateStrings[state], state.String())
	}
}

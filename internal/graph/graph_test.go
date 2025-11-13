package graph_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/graph"
)

func TestGraph(t *testing.T) {
	g := graph.New()
	g.AddTransition(1, 2)
	require.True(t, g.IsTerminal(2))
	require.True(t, g.IsValid(1))
	require.True(t, g.IsValid(2))

	g.AddTransition(2, 3)
	require.False(t, g.IsTerminal(2))
	require.True(t, g.IsTerminal(3))
	require.True(t, g.IsValid(3))

	g.AddTransition(3, 4)
	require.False(t, g.IsTerminal(3))
	require.True(t, g.IsTerminal(4))
	require.True(t, g.IsValid(4))

	g.AddTransition(1, 5)
	require.True(t, g.IsTerminal(5))
	require.Equal(t, []int{2, 5}, g.Transitions(1))

	actual := g.Info()
	expected := graph.Info{
		StartingNodes: []int{1},
		TerminalNodes: []int{4, 5},
		Transitions: []graph.Transition{
			{
				From: 1,
				To:   2,
			},
			{
				From: 1,
				To:   5,
			},
			{
				From: 2,
				To:   3,
			},
			{
				From: 3,
				To:   4,
			},
		},
	}
	require.Equal(t, expected, actual)

	actualNodes := g.Nodes()
	expectedNodes := []int{1, 2, 3, 4, 5}
	require.Equal(t, expectedNodes, actualNodes)
}

func TestGraphReservedNodes(t *testing.T) {
	g := graph.New(-1, 99)
	// Ensure no panic on adding transitions that are not reserved nodes
	g.AddTransition(1, 2)
	require.True(t, g.IsTerminal(2))
	require.True(t, g.IsValid(1))
	require.True(t, g.IsValid(2))

	require.PanicsWithValue(t,
		"cannot use reserved node as 'from' node",
		func() {
			g.AddTransition(-1, 1)
		}, "Using reserved nodes should panic")

	require.PanicsWithValue(t,
		"cannot use reserved node as 'to' node",
		func() {
			g.AddTransition(1, -1)
		}, "Using reserved nodes should panic")

	require.PanicsWithValue(t,
		"cannot use reserved node as 'from' node",
		func() {
			g.AddTransition(99, 1)
		}, "Using reserved nodes should panic")

	require.PanicsWithValue(t,
		"cannot use reserved node as 'to' node",
		func() {
			g.AddTransition(1, 99)
		}, "Using reserved nodes should panic")
}

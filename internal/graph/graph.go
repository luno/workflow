package graph

func New() *Graph {
	return &Graph{
		graph:      make(map[int][]int),
		starting:   make(map[int]bool),
		terminal:   make(map[int]bool),
		validNodes: make(map[int]bool),
	}
}

type Graph struct {
	graph      map[int][]int
	nodeOrder  []int
	starting   map[int]bool
	terminal   map[int]bool
	validNodes map[int]bool
}

func (g *Graph) AddTransition(from int, to int) {
	if _, ok := g.validNodes[from]; !ok {
		g.nodeOrder = append(g.nodeOrder, from)
	}

	if _, ok := g.validNodes[to]; !ok {
		g.nodeOrder = append(g.nodeOrder, to)
	}

	// Nodes that are reached via another node are never considered starting nodes
	g.starting[to] = false

	// Only mark the origin node ("from") as a starting node if it's never been marked as false
	if _, ok := g.starting[from]; !ok {
		g.starting[from] = true
	}

	// If the toStatus has not been defined as a node that has edges then mark it as terminal
	if _, ok := g.graph[to]; !ok {
		g.terminal[to] = true
	}

	// When declaring a node with edges ensure that any previous marking as terminal is overridden
	if _, ok := g.terminal[from]; ok {
		g.terminal[from] = false
	}

	// Add transition from node to node.
	g.graph[from] = append(g.graph[from], to)

	// Update valid nodes
	g.validNodes[from] = true
	g.validNodes[to] = true
}

func (g *Graph) IsTerminal(node int) bool {
	return g.terminal[node]
}

func (g *Graph) Transitions(node int) []int {
	return g.graph[node]
}

func (g *Graph) IsValid(node int) bool {
	return g.validNodes[node]
}

type Transition struct {
	From int
	To   int
}

type Info struct {
	StartingNodes []int
	TerminalNodes []int
	Transitions   []Transition
}

func (g *Graph) Info() Info {
	var i Info
	for _, node := range g.nodeOrder {
		// Add transitions
		if transitions, ok := g.graph[node]; ok {
			for _, to := range transitions {
				i.Transitions = append(i.Transitions, Transition{
					From: node,
					To:   to,
				})
			}
		}

		// Add starting nodes
		if valid, ok := g.starting[node]; ok {
			if valid {
				i.StartingNodes = append(i.StartingNodes, node)
			}
		}

		// Add terminal nodes
		if valid, ok := g.terminal[node]; ok {
			if valid {
				i.TerminalNodes = append(i.TerminalNodes, node)
			}
		}
	}

	return i
}

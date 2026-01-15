package graph

// BFS performs breadth-first search starting from the given node.
// Returns all nodes reachable from start in BFS order.
// direction: "forward" follows edges, "reverse" follows reverse edges.
func (g *Graph) BFS(start string, direction string) []string {
	var getNeighbors func(string) []string
	if direction == "reverse" {
		getNeighbors = func(n string) []string { return g.ReverseEdges[n] }
	} else {
		getNeighbors = func(n string) []string { return g.Edges[n] }
	}

	visited := make(map[string]struct{})
	result := []string{}
	queue := []string{start}
	visited[start] = struct{}{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, neighbor := range getNeighbors(current) {
			if _, seen := visited[neighbor]; !seen {
				visited[neighbor] = struct{}{}
				queue = append(queue, neighbor)
			}
		}
	}

	return result
}

// DFS performs depth-first search starting from the given node.
// Returns all nodes reachable from start in DFS order.
// direction: "forward" follows edges, "reverse" follows reverse edges.
func (g *Graph) DFS(start string, direction string) []string {
	var getNeighbors func(string) []string
	if direction == "reverse" {
		getNeighbors = func(n string) []string { return g.ReverseEdges[n] }
	} else {
		getNeighbors = func(n string) []string { return g.Edges[n] }
	}

	visited := make(map[string]struct{})
	result := []string{}

	var dfs func(string)
	dfs = func(node string) {
		if _, seen := visited[node]; seen {
			return
		}
		visited[node] = struct{}{}
		result = append(result, node)

		for _, neighbor := range getNeighbors(node) {
			dfs(neighbor)
		}
	}

	dfs(start)
	return result
}

// TransitiveClosure returns all nodes reachable from start (excluding start itself).
func (g *Graph) TransitiveClosure(start string) []string {
	all := g.BFS(start, "forward")
	if len(all) > 0 && all[0] == start {
		return all[1:] // Exclude start node
	}
	return all
}

// ReverseTransitiveClosure returns all nodes that can reach start (excluding start itself).
func (g *Graph) ReverseTransitiveClosure(start string) []string {
	all := g.BFS(start, "reverse")
	if len(all) > 0 && all[0] == start {
		return all[1:] // Exclude start node
	}
	return all
}

// ShortestPath finds the shortest path from start to end using BFS.
// Returns the path as a slice of node IDs, or nil if no path exists.
// direction: "forward" or "reverse"
func (g *Graph) ShortestPath(start, end, direction string) []string {
	if start == end {
		return []string{start}
	}

	var getNeighbors func(string) []string
	if direction == "reverse" {
		getNeighbors = func(n string) []string { return g.ReverseEdges[n] }
	} else {
		getNeighbors = func(n string) []string { return g.Edges[n] }
	}

	visited := make(map[string]struct{})
	parent := make(map[string]string)
	queue := []string{start}
	visited[start] = struct{}{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range getNeighbors(current) {
			if _, seen := visited[neighbor]; seen {
				continue
			}
			visited[neighbor] = struct{}{}
			parent[neighbor] = current

			if neighbor == end {
				// Reconstruct path
				path := []string{end}
				for node := end; node != start; {
					node = parent[node]
					path = append([]string{node}, path...)
				}
				return path
			}

			queue = append(queue, neighbor)
		}
	}

	return nil // No path found
}

// FindCycles detects if the graph contains any cycles.
// Returns true if cycles exist, along with one example cycle.
func (g *Graph) FindCycles() (bool, []string) {
	// Use DFS with color marking
	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // finished
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	for node := range g.Edges {
		color[node] = white
	}

	var cycleStart, cycleEnd string
	hasCycle := false

	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray

		for _, neighbor := range g.Edges[node] {
			if color[neighbor] == gray {
				// Back edge found - cycle detected
				cycleStart = neighbor
				cycleEnd = node
				return true
			}
			if color[neighbor] == white {
				parent[neighbor] = node
				if dfs(neighbor) {
					return true
				}
			}
		}

		color[node] = black
		return false
	}

	for node := range g.Edges {
		if color[node] == white {
			if dfs(node) {
				hasCycle = true
				break
			}
		}
	}

	if !hasCycle {
		return false, nil
	}

	// Reconstruct one cycle
	cycle := []string{cycleStart}
	for node := cycleEnd; node != cycleStart; {
		cycle = append([]string{node}, cycle...)
		node = parent[node]
	}
	cycle = append(cycle, cycleStart) // Close the cycle

	return true, cycle
}

// TopologicalSort returns nodes in topological order.
// Returns nil if the graph has cycles.
func (g *Graph) TopologicalSort() []string {
	inDegree := make(map[string]int)
	for node := range g.Edges {
		inDegree[node] = 0
	}
	for _, targets := range g.Edges {
		for _, target := range targets {
			inDegree[target]++
		}
	}

	// Start with nodes that have no incoming edges
	queue := []string{}
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	result := []string{}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, target := range g.Edges[node] {
			inDegree[target]--
			if inDegree[target] == 0 {
				queue = append(queue, target)
			}
		}
	}

	// If we didn't visit all nodes, there's a cycle
	if len(result) != len(g.Edges) {
		return nil
	}

	return result
}

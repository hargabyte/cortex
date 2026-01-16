package graph

import (
	"strings"
)

// Path represents a sequence of nodes forming a path in the graph.
type Path []string

// AllPaths finds all paths from start to end using DFS with a maximum depth limit.
// Returns all unique paths found within the depth limit.
// If maxDepth <= 0, defaults to 10 to prevent infinite exploration.
func (g *Graph) AllPaths(start, end string, maxDepth int) []Path {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	if start == end {
		return []Path{{start}}
	}

	var paths []Path
	visited := make(map[string]bool)
	currentPath := []string{start}
	visited[start] = true

	g.findAllPathsDFS(start, end, visited, currentPath, maxDepth, &paths)

	return paths
}

// findAllPathsDFS is a helper function for AllPaths that performs DFS exploration.
func (g *Graph) findAllPathsDFS(current, end string, visited map[string]bool, currentPath []string, maxDepth int, paths *[]Path) {
	if len(currentPath) > maxDepth {
		return
	}

	for _, neighbor := range g.Edges[current] {
		if neighbor == end {
			// Found a path - make a copy and add to results
			newPath := make([]string, len(currentPath)+1)
			copy(newPath, currentPath)
			newPath[len(currentPath)] = end
			*paths = append(*paths, newPath)
			continue
		}

		if visited[neighbor] {
			continue
		}

		// Continue exploring
		visited[neighbor] = true
		g.findAllPathsDFS(neighbor, end, visited, append(currentPath, neighbor), maxDepth, paths)
		visited[neighbor] = false
	}
}

// AllPathsReverse finds all paths from start to end following reverse edges.
// Useful for finding all upstream callers.
func (g *Graph) AllPathsReverse(start, end string, maxDepth int) []Path {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	if start == end {
		return []Path{{start}}
	}

	var paths []Path
	visited := make(map[string]bool)
	currentPath := []string{start}
	visited[start] = true

	g.findAllPathsReverseDFS(start, end, visited, currentPath, maxDepth, &paths)

	return paths
}

// findAllPathsReverseDFS is a helper for AllPathsReverse.
func (g *Graph) findAllPathsReverseDFS(current, end string, visited map[string]bool, currentPath []string, maxDepth int, paths *[]Path) {
	if len(currentPath) > maxDepth {
		return
	}

	for _, neighbor := range g.ReverseEdges[current] {
		if neighbor == end {
			newPath := make([]string, len(currentPath)+1)
			copy(newPath, currentPath)
			newPath[len(currentPath)] = end
			*paths = append(*paths, newPath)
			continue
		}

		if visited[neighbor] {
			continue
		}

		visited[neighbor] = true
		g.findAllPathsReverseDFS(neighbor, end, visited, append(currentPath, neighbor), maxDepth, paths)
		visited[neighbor] = false
	}
}

// PathPattern represents a pattern for path matching.
// Supports:
//   - Exact node matches: "NodeA"
//   - Wildcards: "*" matches any single node
//   - Multi-wildcards: "**" matches zero or more nodes
type PathPattern struct {
	Elements []PatternElement
}

// PatternElement represents one element in a path pattern.
type PatternElement struct {
	Type  PatternType
	Value string // For exact matches, the node name or prefix
}

// PatternType indicates the type of pattern element.
type PatternType int

const (
	// PatternExact matches an exact node name or prefix
	PatternExact PatternType = iota
	// PatternWildcard matches any single node
	PatternWildcard
	// PatternMultiWildcard matches zero or more nodes
	PatternMultiWildcard
)

// ParsePattern parses a pattern string like "A -> * -> B" or "A -> ** -> B".
func ParsePattern(pattern string) *PathPattern {
	parts := strings.Split(pattern, "->")
	elements := make([]PatternElement, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var elem PatternElement
		switch part {
		case "**":
			elem = PatternElement{Type: PatternMultiWildcard}
		case "*":
			elem = PatternElement{Type: PatternWildcard}
		default:
			elem = PatternElement{Type: PatternExact, Value: part}
		}
		elements = append(elements, elem)
	}

	return &PathPattern{Elements: elements}
}

// PathMatch finds all paths that match a given pattern.
// The pattern uses "->" as separator and supports wildcards.
// Examples:
//   - "A -> B" - direct path from A to B
//   - "A -> * -> B" - path from A to B through exactly one intermediate node
//   - "A -> ** -> B" - path from A to B through any number of intermediate nodes
//   - "Auth* -> *" - paths from any node starting with "Auth" to any node
//
// nodeNameFunc is used to convert node IDs to human-readable names for matching.
// If nil, node IDs are used directly.
func (g *Graph) PathMatch(pattern string, maxDepth int, nodeNameFunc func(string) string) []Path {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	parsed := ParsePattern(pattern)
	if len(parsed.Elements) == 0 {
		return nil
	}

	if nodeNameFunc == nil {
		nodeNameFunc = func(id string) string { return id }
	}

	var results []Path

	// Find all starting nodes that match the first pattern element
	startNodes := g.matchNodes(parsed.Elements[0], nodeNameFunc)

	for _, start := range startNodes {
		// If pattern has only one element, that's the entire match
		if len(parsed.Elements) == 1 {
			results = append(results, Path{start})
			continue
		}

		// Find paths matching the rest of the pattern
		matches := g.matchPatternFromNode(start, parsed.Elements[1:], maxDepth, nodeNameFunc, make(map[string]bool))
		results = append(results, matches...)
	}

	return results
}

// matchNodes returns all nodes that match a pattern element.
func (g *Graph) matchNodes(elem PatternElement, nameFunc func(string) string) []string {
	var matches []string

	for node := range g.Edges {
		if g.nodeMatchesElement(node, elem, nameFunc) {
			matches = append(matches, node)
		}
	}

	return matches
}

// nodeMatchesElement checks if a node matches a pattern element.
func (g *Graph) nodeMatchesElement(nodeID string, elem PatternElement, nameFunc func(string) string) bool {
	switch elem.Type {
	case PatternWildcard, PatternMultiWildcard:
		return true
	case PatternExact:
		name := nameFunc(nodeID)
		// Support prefix matching with wildcard suffix
		if strings.HasSuffix(elem.Value, "*") {
			prefix := strings.TrimSuffix(elem.Value, "*")
			return strings.HasPrefix(name, prefix) || strings.HasPrefix(nodeID, prefix)
		}
		// Exact match (case-insensitive)
		return strings.EqualFold(name, elem.Value) || strings.EqualFold(nodeID, elem.Value)
	}
	return false
}

// matchPatternFromNode finds all paths from a node that match the remaining pattern.
func (g *Graph) matchPatternFromNode(current string, remaining []PatternElement, maxDepth int, nameFunc func(string) string, visited map[string]bool) []Path {
	if len(remaining) == 0 {
		return []Path{{current}}
	}

	if maxDepth <= 0 {
		return nil
	}

	visited[current] = true
	defer func() { visited[current] = false }()

	var results []Path
	nextElem := remaining[0]
	restPattern := remaining[1:]

	switch nextElem.Type {
	case PatternExact, PatternWildcard:
		// Match exactly one node
		for _, neighbor := range g.Edges[current] {
			if visited[neighbor] {
				continue
			}
			if !g.nodeMatchesElement(neighbor, nextElem, nameFunc) {
				continue
			}

			subPaths := g.matchPatternFromNode(neighbor, restPattern, maxDepth-1, nameFunc, visited)
			for _, subPath := range subPaths {
				fullPath := append(Path{current}, subPath...)
				results = append(results, fullPath)
			}
		}

	case PatternMultiWildcard:
		// Match zero or more nodes
		// First, try matching zero nodes (skip the wildcard)
		if len(restPattern) > 0 {
			// Try to match the next pattern element from current
			subPaths := g.matchPatternFromNode(current, restPattern, maxDepth, nameFunc, visited)
			results = append(results, subPaths...)
		} else {
			// End of pattern - current is a match
			results = append(results, Path{current})
		}

		// Then try expanding through any neighbor
		for _, neighbor := range g.Edges[current] {
			if visited[neighbor] {
				continue
			}

			// Continue with ** (can match more nodes)
			subPaths := g.matchPatternFromNode(neighbor, remaining, maxDepth-1, nameFunc, visited)
			for _, subPath := range subPaths {
				fullPath := append(Path{current}, subPath...)
				results = append(results, fullPath)
			}
		}
	}

	return results
}

// TracePath traces the call path between two entities and returns detailed path information.
// This is a convenience wrapper around ShortestPath that provides entity names.
func (g *Graph) TracePath(from, to string, direction string) Path {
	return g.ShortestPath(from, to, direction)
}

// TraceCallers returns all entities that call the given entity, up to maxDepth hops.
// Returns paths from each caller to the target entity.
func (g *Graph) TraceCallers(entityID string, maxDepth int) []Path {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	var paths []Path
	callers := g.ReverseEdges[entityID]

	for _, caller := range callers {
		// Add direct caller path
		paths = append(paths, Path{caller, entityID})

		// If maxDepth > 1, trace further upstream
		if maxDepth > 1 {
			upstreamPaths := g.TraceCallers(caller, maxDepth-1)
			for _, upstream := range upstreamPaths {
				// Extend upstream path to include our entity
				extended := make(Path, len(upstream)+1)
				copy(extended, upstream)
				extended[len(upstream)] = entityID
				paths = append(paths, extended)
			}
		}
	}

	return paths
}

// TraceCallees returns all entities that the given entity calls, up to maxDepth hops.
// Returns paths from the entity to each callee.
func (g *Graph) TraceCallees(entityID string, maxDepth int) []Path {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	var paths []Path
	callees := g.Edges[entityID]

	for _, callee := range callees {
		// Add direct callee path
		paths = append(paths, Path{entityID, callee})

		// If maxDepth > 1, trace further downstream
		if maxDepth > 1 {
			downstreamPaths := g.TraceCallees(callee, maxDepth-1)
			for _, downstream := range downstreamPaths {
				// Extend with our entity at the start
				extended := make(Path, len(downstream)+1)
				extended[0] = entityID
				copy(extended[1:], downstream)
				paths = append(paths, extended)
			}
		}
	}

	return paths
}

// CollectCallerChain collects all callers of an entity into a single chain.
// Returns entities in order from root callers to the target.
func (g *Graph) CollectCallerChain(entityID string, maxDepth int) []string {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	reachable := g.BFS(entityID, "reverse")
	// Limit to maxDepth
	if len(reachable) > maxDepth+1 {
		reachable = reachable[:maxDepth+1]
	}

	return reachable
}

// CollectCalleeChain collects all callees of an entity into a single chain.
// Returns entities in order from the entity to leaf callees.
func (g *Graph) CollectCalleeChain(entityID string, maxDepth int) []string {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	reachable := g.BFS(entityID, "forward")
	// Limit to maxDepth
	if len(reachable) > maxDepth+1 {
		reachable = reachable[:maxDepth+1]
	}

	return reachable
}

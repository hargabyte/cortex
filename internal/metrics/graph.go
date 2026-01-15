package metrics

import (
	"github.com/anthropics/cx/internal/bd"
)

// BuildDependencyGraph builds a graph from beads dependencies.
// Returns: map[entityID][]outgoingEntityIDs where an edge from A to B
// means A depends on B (A calls B, A uses type from B, etc.)
//
// This function reads .beads/issues.jsonl directly for O(1) performance
// instead of making O(N) subprocess calls to `bd dep list`.
func BuildDependencyGraph(client *bd.Client, entityIDs []string) (map[string][]string, error) {
	graph := make(map[string][]string)

	// Create a set of entity IDs for fast lookup
	entitySet := make(map[string]struct{}, len(entityIDs))
	for _, id := range entityIDs {
		entitySet[id] = struct{}{}
		graph[id] = []string{} // Initialize all entities with empty slice
	}

	// Read all dependencies from JSONL in one pass
	allDeps, err := client.ReadAllDependenciesFromJSONL()
	if err != nil {
		// Fall back to N+1 queries if JSONL read fails
		return buildDependencyGraphFallback(client, entityIDs)
	}

	// Build graph from pre-loaded dependencies
	for _, id := range entityIDs {
		deps, exists := allDeps[id]
		if !exists {
			continue
		}

		targets := make([]string, 0)
		for _, dep := range deps {
			// Include code dependency types
			if isCodeDependency(dep.DepType) {
				targets = append(targets, dep.ToID)
			}
		}
		graph[id] = targets
	}

	return graph, nil
}

// buildDependencyGraphFallback is the original O(N) implementation
// used as a fallback if JSONL reading fails.
func buildDependencyGraphFallback(client *bd.Client, entityIDs []string) (map[string][]string, error) {
	graph := make(map[string][]string)

	for _, id := range entityIDs {
		deps, err := client.ListDependencies(id)
		if err != nil {
			graph[id] = []string{}
			continue
		}

		targets := make([]string, 0)
		for _, dep := range deps {
			if isCodeDependency(dep.DepType) {
				targets = append(targets, dep.ToID)
			}
		}
		graph[id] = targets
	}

	return graph, nil
}

// isCodeDependency returns true if the dependency type represents a code relationship
func isCodeDependency(depType string) bool {
	switch depType {
	case "calls", "uses_type", "imports", "extends", "implements", "references":
		return true
	default:
		return false
	}
}

// BuildDependencyGraphFromDeps builds a graph directly from a list of dependencies.
// This is useful when you already have the dependencies loaded.
func BuildDependencyGraphFromDeps(deps []bd.Dependency) map[string][]string {
	graph := make(map[string][]string)

	for _, dep := range deps {
		if !isCodeDependency(dep.DepType) {
			continue
		}

		// Ensure both nodes exist in the graph
		if _, exists := graph[dep.FromID]; !exists {
			graph[dep.FromID] = []string{}
		}
		if _, exists := graph[dep.ToID]; !exists {
			graph[dep.ToID] = []string{}
		}

		// Add edge from source to target
		graph[dep.FromID] = append(graph[dep.FromID], dep.ToID)
	}

	return graph
}

// ComputeInOutDegree calculates in-degree and out-degree for each node.
// In-degree: number of edges pointing TO the node
// Out-degree: number of edges pointing FROM the node
func ComputeInOutDegree(graph map[string][]string) (inDegree, outDegree map[string]int) {
	inDegree = make(map[string]int)
	outDegree = make(map[string]int)

	// Collect all nodes first
	allNodes := make(map[string]struct{})
	for node, targets := range graph {
		allNodes[node] = struct{}{}
		for _, target := range targets {
			allNodes[target] = struct{}{}
		}
	}

	// Initialize degrees for all nodes
	for node := range allNodes {
		inDegree[node] = 0
		outDegree[node] = 0
	}

	// Compute degrees
	for node, targets := range graph {
		outDegree[node] = len(targets)
		for _, target := range targets {
			inDegree[target]++
		}
	}

	return
}

// ReverseGraph creates a reverse graph where all edges are inverted.
// If original graph has edge A -> B, reversed graph has edge B -> A.
func ReverseGraph(graph map[string][]string) map[string][]string {
	reversed := make(map[string][]string)

	// Collect all nodes first
	allNodes := make(map[string]struct{})
	for node, targets := range graph {
		allNodes[node] = struct{}{}
		for _, target := range targets {
			allNodes[target] = struct{}{}
		}
	}

	// Initialize empty slices for all nodes
	for node := range allNodes {
		reversed[node] = []string{}
	}

	// Build reversed edges
	for source, targets := range graph {
		for _, target := range targets {
			reversed[target] = append(reversed[target], source)
		}
	}

	return reversed
}

// FindRoots finds nodes with no incoming edges (in-degree = 0).
// These are entry points or independent components.
func FindRoots(graph map[string][]string) []string {
	inDegree, _ := ComputeInOutDegree(graph)

	var roots []string
	for node, degree := range inDegree {
		if degree == 0 {
			roots = append(roots, node)
		}
	}

	return roots
}

// FindLeaves finds nodes with no outgoing edges (out-degree = 0).
// These are end points or leaf dependencies.
func FindLeaves(graph map[string][]string) []string {
	_, outDegree := ComputeInOutDegree(graph)

	var leaves []string
	for node, degree := range outDegree {
		if degree == 0 {
			leaves = append(leaves, node)
		}
	}

	return leaves
}

// GraphStats contains statistics about a dependency graph
type GraphStats struct {
	NodeCount   int     `json:"node_count"`
	EdgeCount   int     `json:"edge_count"`
	AvgInDegree float64 `json:"avg_in_degree"`
	MaxInDegree int     `json:"max_in_degree"`
	MaxOutDegree int    `json:"max_out_degree"`
	RootCount   int     `json:"root_count"`
	LeafCount   int     `json:"leaf_count"`
	Density     float64 `json:"density"`
}

// ComputeGraphStats calculates statistics for a dependency graph.
func ComputeGraphStats(graph map[string][]string) GraphStats {
	inDegree, outDegree := ComputeInOutDegree(graph)

	nodeCount := len(inDegree)
	if nodeCount == 0 {
		return GraphStats{}
	}

	// Count edges
	edgeCount := 0
	for _, targets := range graph {
		edgeCount += len(targets)
	}

	// Compute max degrees and averages
	maxIn, maxOut := 0, 0
	rootCount, leafCount := 0, 0
	totalInDegree := 0

	for node, in := range inDegree {
		totalInDegree += in
		if in > maxIn {
			maxIn = in
		}
		if in == 0 {
			rootCount++
		}

		out := outDegree[node]
		if out > maxOut {
			maxOut = out
		}
		if out == 0 {
			leafCount++
		}
	}

	// Density = edges / (nodes * (nodes-1)) for directed graph
	var density float64
	if nodeCount > 1 {
		density = float64(edgeCount) / float64(nodeCount*(nodeCount-1))
	}

	return GraphStats{
		NodeCount:    nodeCount,
		EdgeCount:    edgeCount,
		AvgInDegree:  float64(totalInDegree) / float64(nodeCount),
		MaxInDegree:  maxIn,
		MaxOutDegree: maxOut,
		RootCount:    rootCount,
		LeafCount:    leafCount,
		Density:      density,
	}
}

// Subgraph extracts a subgraph containing only the specified nodes.
// Only edges between nodes in the subset are included.
func Subgraph(graph map[string][]string, nodes []string) map[string][]string {
	// Create set for fast lookup
	nodeSet := make(map[string]struct{})
	for _, n := range nodes {
		nodeSet[n] = struct{}{}
	}

	subgraph := make(map[string][]string)
	for _, node := range nodes {
		if targets, exists := graph[node]; exists {
			// Filter to only include targets in the subset
			filteredTargets := []string{}
			for _, target := range targets {
				if _, inSet := nodeSet[target]; inSet {
					filteredTargets = append(filteredTargets, target)
				}
			}
			subgraph[node] = filteredTargets
		} else {
			subgraph[node] = []string{}
		}
	}

	return subgraph
}

// TransitiveClosure computes the transitive closure of a node.
// Returns all nodes reachable from the given node via dependency paths.
func TransitiveClosure(graph map[string][]string, startNode string) []string {
	visited := make(map[string]struct{})
	var result []string

	// BFS traversal
	queue := []string{startNode}
	visited[startNode] = struct{}{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current != startNode {
			result = append(result, current)
		}

		for _, target := range graph[current] {
			if _, seen := visited[target]; !seen {
				visited[target] = struct{}{}
				queue = append(queue, target)
			}
		}
	}

	return result
}

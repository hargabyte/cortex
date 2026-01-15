package graph

import (
	"github.com/anthropics/cx/internal/store"
)

// Graph represents an in-memory dependency graph.
type Graph struct {
	// Adjacency list: node -> list of nodes it depends on
	Edges map[string][]string
	// Reverse adjacency: node -> list of nodes that depend on it
	ReverseEdges map[string][]string
}

// BuildFromStore loads the dependency graph from the store.
func BuildFromStore(s *store.Store) (*Graph, error) {
	deps, err := s.GetAllDependencies()
	if err != nil {
		return nil, err
	}

	g := &Graph{
		Edges:        make(map[string][]string),
		ReverseEdges: make(map[string][]string),
	}

	for _, dep := range deps {
		if !isCodeDependency(dep.DepType) {
			continue
		}

		// Initialize nodes if not present
		if _, ok := g.Edges[dep.FromID]; !ok {
			g.Edges[dep.FromID] = []string{}
		}
		if _, ok := g.Edges[dep.ToID]; !ok {
			g.Edges[dep.ToID] = []string{}
		}
		if _, ok := g.ReverseEdges[dep.FromID]; !ok {
			g.ReverseEdges[dep.FromID] = []string{}
		}
		if _, ok := g.ReverseEdges[dep.ToID]; !ok {
			g.ReverseEdges[dep.ToID] = []string{}
		}

		// Add forward and reverse edges
		g.Edges[dep.FromID] = append(g.Edges[dep.FromID], dep.ToID)
		g.ReverseEdges[dep.ToID] = append(g.ReverseEdges[dep.ToID], dep.FromID)
	}

	return g, nil
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

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	return len(g.Edges)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	count := 0
	for _, targets := range g.Edges {
		count += len(targets)
	}
	return count
}

// Nodes returns all node IDs in the graph.
func (g *Graph) Nodes() []string {
	nodes := make([]string, 0, len(g.Edges))
	for node := range g.Edges {
		nodes = append(nodes, node)
	}
	return nodes
}

// OutDegree returns the number of outgoing edges from a node.
func (g *Graph) OutDegree(node string) int {
	return len(g.Edges[node])
}

// InDegree returns the number of incoming edges to a node.
func (g *Graph) InDegree(node string) int {
	return len(g.ReverseEdges[node])
}

// Successors returns nodes that this node depends on.
func (g *Graph) Successors(node string) []string {
	return g.Edges[node]
}

// Predecessors returns nodes that depend on this node.
func (g *Graph) Predecessors(node string) []string {
	return g.ReverseEdges[node]
}

// Subgraph creates a new graph containing only the specified nodes.
func (g *Graph) Subgraph(nodes []string) *Graph {
	nodeSet := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = struct{}{}
	}

	sub := &Graph{
		Edges:        make(map[string][]string),
		ReverseEdges: make(map[string][]string),
	}

	for _, node := range nodes {
		sub.Edges[node] = []string{}
		sub.ReverseEdges[node] = []string{}

		for _, target := range g.Edges[node] {
			if _, ok := nodeSet[target]; ok {
				sub.Edges[node] = append(sub.Edges[node], target)
			}
		}
		for _, source := range g.ReverseEdges[node] {
			if _, ok := nodeSet[source]; ok {
				sub.ReverseEdges[node] = append(sub.ReverseEdges[node], source)
			}
		}
	}

	return sub
}

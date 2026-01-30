package graph

import (
	"reflect"
	"sort"
	"testing"
)

// newTestGraph creates a graph directly for testing without store dependency.
func newTestGraph() *Graph {
	return &Graph{
		Edges:        make(map[string][]string),
		ReverseEdges: make(map[string][]string),
	}
}

// addEdge adds an edge to the test graph.
func (g *Graph) addEdge(from, to string) {
	if _, ok := g.Edges[from]; !ok {
		g.Edges[from] = []string{}
	}
	if _, ok := g.Edges[to]; !ok {
		g.Edges[to] = []string{}
	}
	if _, ok := g.ReverseEdges[from]; !ok {
		g.ReverseEdges[from] = []string{}
	}
	if _, ok := g.ReverseEdges[to]; !ok {
		g.ReverseEdges[to] = []string{}
	}

	g.Edges[from] = append(g.Edges[from], to)
	g.ReverseEdges[to] = append(g.ReverseEdges[to], from)
}

func TestGraph_NodeCount(t *testing.T) {
	g := newTestGraph()

	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}

	g.addEdge("a", "b")
	g.addEdge("b", "c")

	if g.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NodeCount())
	}
}

func TestGraph_EdgeCount(t *testing.T) {
	g := newTestGraph()

	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}

	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "c")

	if g.EdgeCount() != 3 {
		t.Errorf("expected 3 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_Nodes(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	nodes := g.Nodes()
	sort.Strings(nodes)

	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(nodes, expected) {
		t.Errorf("expected nodes %v, got %v", expected, nodes)
	}
}

func TestGraph_OutDegree(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "c")

	tests := []struct {
		node string
		want int
	}{
		{"a", 2},
		{"b", 1},
		{"c", 0},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		got := g.OutDegree(tt.node)
		if got != tt.want {
			t.Errorf("OutDegree(%s) = %d, want %d", tt.node, got, tt.want)
		}
	}
}

func TestGraph_InDegree(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "c")

	tests := []struct {
		node string
		want int
	}{
		{"a", 0},
		{"b", 1},
		{"c", 2},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		got := g.InDegree(tt.node)
		if got != tt.want {
			t.Errorf("InDegree(%s) = %d, want %d", tt.node, got, tt.want)
		}
	}
}

func TestGraph_Successors(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("a", "c")

	successors := g.Successors("a")
	sort.Strings(successors)

	expected := []string{"b", "c"}
	if !reflect.DeepEqual(successors, expected) {
		t.Errorf("Successors(a) = %v, want %v", successors, expected)
	}

	// Node with no successors
	if len(g.Successors("c")) != 0 {
		t.Errorf("expected empty successors for c")
	}
}

func TestGraph_Predecessors(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "c")
	g.addEdge("b", "c")

	predecessors := g.Predecessors("c")
	sort.Strings(predecessors)

	expected := []string{"a", "b"}
	if !reflect.DeepEqual(predecessors, expected) {
		t.Errorf("Predecessors(c) = %v, want %v", predecessors, expected)
	}

	// Node with no predecessors
	if len(g.Predecessors("a")) != 0 {
		t.Errorf("expected empty predecessors for a")
	}
}

func TestGraph_Subgraph(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")
	g.addEdge("a", "d")

	// Create subgraph with only a, b, c
	sub := g.Subgraph([]string{"a", "b", "c"})

	if sub.NodeCount() != 3 {
		t.Errorf("expected 3 nodes in subgraph, got %d", sub.NodeCount())
	}

	// Edge a->b should exist
	if len(sub.Edges["a"]) != 1 || sub.Edges["a"][0] != "b" {
		t.Errorf("expected edge a->b in subgraph")
	}

	// Edge b->c should exist
	if len(sub.Edges["b"]) != 1 || sub.Edges["b"][0] != "c" {
		t.Errorf("expected edge b->c in subgraph")
	}

	// Edge a->d should NOT exist (d not in subgraph)
	for _, target := range sub.Edges["a"] {
		if target == "d" {
			t.Error("did not expect edge a->d in subgraph")
		}
	}

	// Edge c->d should NOT exist
	if len(sub.Edges["c"]) != 0 {
		t.Errorf("expected no outgoing edges from c in subgraph")
	}
}

func TestIsCodeDependency(t *testing.T) {
	tests := []struct {
		depType string
		want    bool
	}{
		{"calls", true},
		{"uses_type", true},
		{"imports", true},
		{"extends", true},
		{"implements", true},
		{"references", true},
		{"related", false},
		{"discovered-from", false},
		{"blocks", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isCodeDependency(tt.depType)
		if got != tt.want {
			t.Errorf("isCodeDependency(%q) = %v, want %v", tt.depType, got, tt.want)
		}
	}
}

// Traversal tests

func TestGraph_BFS_Forward(t *testing.T) {
	g := newTestGraph()
	// Create a tree: a -> b -> d
	//                  -> c -> e
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "d")
	g.addEdge("c", "e")

	result := g.BFS("a", "forward")

	// First should be "a"
	if len(result) == 0 || result[0] != "a" {
		t.Errorf("expected BFS to start with 'a', got %v", result)
	}

	// Should visit all 5 nodes
	if len(result) != 5 {
		t.Errorf("expected 5 nodes, got %d: %v", len(result), result)
	}

	// Level 1 (b, c) should come before level 2 (d, e)
	bIdx, cIdx, dIdx, eIdx := -1, -1, -1, -1
	for i, node := range result {
		switch node {
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		case "d":
			dIdx = i
		case "e":
			eIdx = i
		}
	}

	if bIdx > dIdx || cIdx > eIdx {
		t.Errorf("BFS order violated: b=%d, c=%d, d=%d, e=%d", bIdx, cIdx, dIdx, eIdx)
	}
}

func TestGraph_BFS_Reverse(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "c")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	// From d going reverse should reach c, then a and b
	result := g.BFS("d", "reverse")

	if len(result) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(result), result)
	}

	if result[0] != "d" {
		t.Errorf("expected to start with 'd', got %s", result[0])
	}
}

func TestGraph_DFS_Forward(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("a", "d")

	result := g.DFS("a", "forward")

	// Should visit all 4 nodes
	if len(result) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(result), result)
	}

	// First should be "a"
	if result[0] != "a" {
		t.Errorf("expected DFS to start with 'a', got %s", result[0])
	}
}

func TestGraph_DFS_Reverse(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	result := g.DFS("c", "reverse")

	// Should reach a, b, c
	if len(result) != 3 {
		t.Errorf("expected 3 nodes, got %d: %v", len(result), result)
	}
}

func TestGraph_TransitiveClosure(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	closure := g.TransitiveClosure("a")

	// Should include b, c, d but NOT a
	if len(closure) != 3 {
		t.Errorf("expected 3 nodes, got %d: %v", len(closure), closure)
	}

	for _, node := range closure {
		if node == "a" {
			t.Error("transitive closure should not include start node")
		}
	}
}

func TestGraph_ReverseTransitiveClosure(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	closure := g.ReverseTransitiveClosure("d")

	// Should include a, b, c but NOT d
	if len(closure) != 3 {
		t.Errorf("expected 3 nodes, got %d: %v", len(closure), closure)
	}

	for _, node := range closure {
		if node == "d" {
			t.Error("reverse transitive closure should not include start node")
		}
	}
}

func TestGraph_ShortestPath(t *testing.T) {
	g := newTestGraph()
	// Create: a -> b -> d
	//         a -> c -> d
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "d")
	g.addEdge("c", "d")

	path := g.ShortestPath("a", "d", "forward")

	// Should be length 3 (a -> b/c -> d)
	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(path), path)
	}

	if path[0] != "a" || path[len(path)-1] != "d" {
		t.Errorf("path should start with 'a' and end with 'd': %v", path)
	}
}

func TestGraph_ShortestPath_SameNode(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")

	path := g.ShortestPath("a", "a", "forward")

	if len(path) != 1 || path[0] != "a" {
		t.Errorf("expected [a] for same node path, got %v", path)
	}
}

func TestGraph_ShortestPath_NoPath(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.Edges["c"] = []string{} // isolated node
	g.ReverseEdges["c"] = []string{}

	path := g.ShortestPath("a", "c", "forward")

	if path != nil {
		t.Errorf("expected nil for no path, got %v", path)
	}
}

func TestGraph_ShortestPath_Reverse(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	// From c to a in reverse direction
	path := g.ShortestPath("c", "a", "reverse")

	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(path), path)
	}

	if path[0] != "c" || path[len(path)-1] != "a" {
		t.Errorf("path should start with 'c' and end with 'a': %v", path)
	}
}

func TestGraph_FindCycles_NoCycle(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("a", "c")

	hasCycle, cycle := g.FindCycles()

	if hasCycle {
		t.Errorf("expected no cycle, but found: %v", cycle)
	}
}

func TestGraph_FindCycles_WithCycle(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "a") // Creates cycle: a -> b -> c -> a

	hasCycle, cycle := g.FindCycles()

	if !hasCycle {
		t.Error("expected to find a cycle")
	}

	if len(cycle) < 2 {
		t.Errorf("cycle should have at least 2 nodes, got %v", cycle)
	}

	// The cycle detection algorithm returns a path that forms a cycle
	// The implementation may return [start, ..., end, start] format
	// Just verify it contains cycle nodes and has reasonable length
	cycleNodes := make(map[string]bool)
	for _, n := range cycle {
		cycleNodes[n] = true
	}

	// Should contain at least one of the cycle nodes
	foundCycleNode := cycleNodes["a"] || cycleNodes["b"] || cycleNodes["c"]
	if !foundCycleNode {
		t.Errorf("expected cycle to contain a, b, or c, got %v", cycle)
	}
}

func TestGraph_FindCycles_SelfLoop(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "a") // Self-loop

	hasCycle, cycle := g.FindCycles()

	if !hasCycle {
		t.Error("expected to find self-loop cycle")
	}

	if len(cycle) < 2 || cycle[0] != "a" {
		t.Errorf("expected self-loop on 'a', got %v", cycle)
	}
}

func TestGraph_TopologicalSort_DAG(t *testing.T) {
	g := newTestGraph()
	// Diamond: a -> b -> d
	//          a -> c -> d
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "d")
	g.addEdge("c", "d")

	result := g.TopologicalSort()

	if result == nil {
		t.Fatal("expected topological sort to succeed for DAG")
	}

	if len(result) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(result))
	}

	// Verify ordering: a must come before b, c; b, c must come before d
	indexOf := make(map[string]int)
	for i, node := range result {
		indexOf[node] = i
	}

	if indexOf["a"] > indexOf["b"] || indexOf["a"] > indexOf["c"] {
		t.Errorf("a should come before b and c: %v", result)
	}

	if indexOf["b"] > indexOf["d"] || indexOf["c"] > indexOf["d"] {
		t.Errorf("b and c should come before d: %v", result)
	}
}

func TestGraph_TopologicalSort_WithCycle(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "a") // Cycle

	result := g.TopologicalSort()

	if result != nil {
		t.Errorf("expected nil for graph with cycle, got %v", result)
	}
}

func TestGraph_TopologicalSort_Linear(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	result := g.TopologicalSort()

	if result == nil {
		t.Fatal("expected topological sort to succeed")
	}

	// For a linear chain, order should be exactly a, b, c, d
	expected := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestGraph_TopologicalSort_Empty(t *testing.T) {
	g := newTestGraph()

	result := g.TopologicalSort()

	if result == nil {
		t.Error("expected empty slice, not nil, for empty graph")
	}

	if len(result) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(result))
	}
}

func TestGraph_TopologicalSort_SingleNode(t *testing.T) {
	g := newTestGraph()
	g.Edges["a"] = []string{}
	g.ReverseEdges["a"] = []string{}

	result := g.TopologicalSort()

	if len(result) != 1 || result[0] != "a" {
		t.Errorf("expected [a], got %v", result)
	}
}

// Complex graph scenarios

func TestGraph_DisconnectedComponents(t *testing.T) {
	g := newTestGraph()
	// Component 1: a -> b
	g.addEdge("a", "b")
	// Component 2: c -> d
	g.addEdge("c", "d")

	// BFS from a should only reach a, b
	result := g.BFS("a", "forward")
	if len(result) != 2 {
		t.Errorf("expected 2 nodes from component 1, got %d", len(result))
	}

	// Total nodes should be 4
	if g.NodeCount() != 4 {
		t.Errorf("expected 4 total nodes, got %d", g.NodeCount())
	}
}

func TestGraph_DiamondDependency(t *testing.T) {
	g := newTestGraph()
	// Diamond: a -> b -> d
	//          a -> c -> d
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "d")
	g.addEdge("c", "d")

	// d has in-degree 2
	if g.InDegree("d") != 2 {
		t.Errorf("expected in-degree 2 for d, got %d", g.InDegree("d"))
	}

	// a has out-degree 2
	if g.OutDegree("a") != 2 {
		t.Errorf("expected out-degree 2 for a, got %d", g.OutDegree("a"))
	}

	// Transitive closure from a should include b, c, d
	closure := g.TransitiveClosure("a")
	if len(closure) != 3 {
		t.Errorf("expected 3 nodes in closure, got %d", len(closure))
	}
}

func TestGraph_LongChain(t *testing.T) {
	g := newTestGraph()
	// Create a long chain: n0 -> n1 -> n2 -> ... -> n9
	for i := 0; i < 9; i++ {
		from := string(rune('0' + i))
		to := string(rune('0' + i + 1))
		g.addEdge(from, to)
	}

	// Shortest path from 0 to 9
	path := g.ShortestPath("0", "9", "forward")
	if len(path) != 10 {
		t.Errorf("expected path length 10, got %d", len(path))
	}

	// Topological sort should produce linear order
	sorted := g.TopologicalSort()
	if sorted == nil {
		t.Fatal("topological sort should succeed")
	}

	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i] >= sorted[i+1] {
			t.Errorf("expected ascending order, got %v", sorted)
			break
		}
	}
}

func TestGraph_BFS_SingleNode(t *testing.T) {
	g := newTestGraph()
	g.Edges["alone"] = []string{}
	g.ReverseEdges["alone"] = []string{}

	result := g.BFS("alone", "forward")

	if len(result) != 1 || result[0] != "alone" {
		t.Errorf("expected [alone], got %v", result)
	}
}

func TestGraph_DFS_SingleNode(t *testing.T) {
	g := newTestGraph()
	g.Edges["alone"] = []string{}
	g.ReverseEdges["alone"] = []string{}

	result := g.DFS("alone", "forward")

	if len(result) != 1 || result[0] != "alone" {
		t.Errorf("expected [alone], got %v", result)
	}
}

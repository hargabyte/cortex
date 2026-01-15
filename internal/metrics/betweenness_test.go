package metrics

import (
	"math"
	"testing"
)

// floatEquals checks if two floats are approximately equal.
func floatEquals(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// TestComputeBetweenness_LinearGraph tests betweenness on a linear path A→B→C→D.
// In a linear graph, the middle nodes (B and C) should have the highest betweenness
// because all paths between endpoints must pass through them.
func TestComputeBetweenness_LinearGraph(t *testing.T) {
	// Linear graph: A - B - C - D (undirected)
	graph := map[string][]string{
		"A": {"B"},
		"B": {"A", "C"},
		"C": {"B", "D"},
		"D": {"C"},
	}

	bc := ComputeBetweenness(graph)

	// Verify all nodes have scores
	if len(bc) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(bc))
	}

	// Middle nodes (B and C) should have higher betweenness than endpoints
	if bc["B"] <= bc["A"] {
		t.Errorf("B should have higher betweenness than A: B=%f, A=%f", bc["B"], bc["A"])
	}
	if bc["C"] <= bc["D"] {
		t.Errorf("C should have higher betweenness than D: C=%f, D=%f", bc["C"], bc["D"])
	}

	// B and C should have equal betweenness (symmetric graph)
	if !floatEquals(bc["B"], bc["C"], 0.001) {
		t.Errorf("B and C should have equal betweenness: B=%f, C=%f", bc["B"], bc["C"])
	}

	// Endpoints should have zero betweenness (no paths pass through them)
	if bc["A"] != 0.0 {
		t.Errorf("A should have 0 betweenness, got %f", bc["A"])
	}
	if bc["D"] != 0.0 {
		t.Errorf("D should have 0 betweenness, got %f", bc["D"])
	}
}

// TestComputeBetweenness_StarGraph tests betweenness on a star/hub-and-spoke topology.
// The hub should have the highest betweenness since all paths between spokes pass through it.
func TestComputeBetweenness_StarGraph(t *testing.T) {
	// Star graph: Hub connected to A, B, C, D (undirected)
	//     A
	//     |
	// B - Hub - C
	//     |
	//     D
	graph := map[string][]string{
		"Hub": {"A", "B", "C", "D"},
		"A":   {"Hub"},
		"B":   {"Hub"},
		"C":   {"Hub"},
		"D":   {"Hub"},
	}

	bc := ComputeBetweenness(graph)

	// Verify all nodes have scores
	if len(bc) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(bc))
	}

	// Hub should have the highest betweenness
	for node, score := range bc {
		if node != "Hub" && score >= bc["Hub"] {
			t.Errorf("Hub should have highest betweenness, but %s has %f vs Hub's %f",
				node, score, bc["Hub"])
		}
	}

	// All spokes should have zero betweenness (no shortest paths pass through them)
	for _, spoke := range []string{"A", "B", "C", "D"} {
		if bc[spoke] != 0.0 {
			t.Errorf("%s should have 0 betweenness, got %f", spoke, bc[spoke])
		}
	}
}

// TestComputeBetweenness_Normalization verifies that normalized scores are in [0,1].
func TestComputeBetweenness_Normalization(t *testing.T) {
	// Test with various graph sizes
	graphs := []map[string][]string{
		// Linear graph
		{
			"A": {"B"},
			"B": {"A", "C"},
			"C": {"B", "D"},
			"D": {"C"},
		},
		// Complete graph K4 (every node connected to every other)
		{
			"A": {"B", "C", "D"},
			"B": {"A", "C", "D"},
			"C": {"A", "B", "D"},
			"D": {"A", "B", "C"},
		},
		// Larger linear graph
		{
			"A": {"B"},
			"B": {"A", "C"},
			"C": {"B", "D"},
			"D": {"C", "E"},
			"E": {"D", "F"},
			"F": {"E"},
		},
	}

	for i, graph := range graphs {
		bc := ComputeBetweenness(graph)

		for node, score := range bc {
			if score < 0.0 || score > 1.0 {
				t.Errorf("graph %d: node %s has score %f outside [0,1]", i, node, score)
			}
		}
	}
}

// TestComputeBetweenness_SmallGraphs tests edge cases with small graphs.
func TestComputeBetweenness_SmallGraphs(t *testing.T) {
	// Single node
	graph1 := map[string][]string{
		"A": {},
	}
	bc1 := ComputeBetweenness(graph1)
	if bc1["A"] != 0.0 {
		t.Errorf("single node should have 0 betweenness, got %f", bc1["A"])
	}

	// Two nodes
	graph2 := map[string][]string{
		"A": {"B"},
		"B": {"A"},
	}
	bc2 := ComputeBetweenness(graph2)
	if bc2["A"] != 0.0 || bc2["B"] != 0.0 {
		t.Errorf("two-node graph should have 0 betweenness for both: A=%f, B=%f",
			bc2["A"], bc2["B"])
	}

	// Empty graph
	graph3 := map[string][]string{}
	bc3 := ComputeBetweenness(graph3)
	if len(bc3) != 0 {
		t.Errorf("empty graph should return empty map, got %d entries", len(bc3))
	}
}

// TestComputeBetweenness_DisconnectedNodes tests graph with isolated nodes.
func TestComputeBetweenness_DisconnectedNodes(t *testing.T) {
	// Graph with isolated node Z
	graph := map[string][]string{
		"A": {"B"},
		"B": {"A", "C"},
		"C": {"B"},
		"Z": {}, // Isolated node
	}

	bc := ComputeBetweenness(graph)

	// Z should have 0 betweenness since no paths pass through it
	if bc["Z"] != 0.0 {
		t.Errorf("isolated node Z should have 0 betweenness, got %f", bc["Z"])
	}

	// B should still have highest betweenness among connected nodes
	if bc["B"] <= bc["A"] || bc["B"] <= bc["C"] {
		t.Errorf("B should have highest betweenness: A=%f, B=%f, C=%f", bc["A"], bc["B"], bc["C"])
	}
}

// TestFindBottlenecks tests the FindBottlenecks function.
func TestFindBottlenecks(t *testing.T) {
	bc := map[string]float64{
		"A":   0.0,
		"B":   0.5,
		"C":   0.5,
		"D":   0.0,
		"Hub": 0.8,
	}

	// Threshold 0.5 should include B, C, and Hub
	bottlenecks := FindBottlenecks(bc, 0.5)
	if len(bottlenecks) != 3 {
		t.Errorf("expected 3 bottlenecks at threshold 0.5, got %d", len(bottlenecks))
	}

	// Threshold 0.6 should only include Hub
	bottlenecks = FindBottlenecks(bc, 0.6)
	if len(bottlenecks) != 1 || bottlenecks[0] != "Hub" {
		t.Errorf("expected only Hub at threshold 0.6, got %v", bottlenecks)
	}

	// Threshold 1.0 should return empty
	bottlenecks = FindBottlenecks(bc, 1.0)
	if len(bottlenecks) != 0 {
		t.Errorf("expected 0 bottlenecks at threshold 1.0, got %d", len(bottlenecks))
	}
}

// TestGetTopByBetweenness tests the GetTopByBetweenness function.
func TestGetTopByBetweenness(t *testing.T) {
	bc := map[string]float64{
		"A":   0.1,
		"B":   0.5,
		"C":   0.3,
		"D":   0.2,
		"Hub": 0.8,
	}

	// Get top 3
	top3 := GetTopByBetweenness(bc, 3)
	if len(top3) != 3 {
		t.Errorf("expected 3 results, got %d", len(top3))
	}

	// Verify order (descending)
	expected := []string{"Hub", "B", "C"}
	for i, ns := range top3 {
		if ns.Node != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], ns.Node)
		}
	}

	// Get more than available
	topAll := GetTopByBetweenness(bc, 10)
	if len(topAll) != 5 {
		t.Errorf("expected 5 results (all nodes), got %d", len(topAll))
	}

	// Get 0
	top0 := GetTopByBetweenness(bc, 0)
	if len(top0) != 0 {
		t.Errorf("expected 0 results, got %d", len(top0))
	}
}

// TestComputeBetweenness_DirectedGraph tests betweenness with a directed graph.
func TestComputeBetweenness_DirectedGraph(t *testing.T) {
	// Directed graph: A -> B -> C -> D
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"D"},
		"D": {},
	}

	bc := ComputeBetweenness(graph)

	// In a directed linear graph, middle nodes should still have higher betweenness
	if bc["B"] <= bc["A"] {
		t.Errorf("B should have higher betweenness than A: B=%f, A=%f", bc["B"], bc["A"])
	}
	if bc["C"] <= bc["D"] {
		t.Errorf("C should have higher betweenness than D: C=%f, D=%f", bc["C"], bc["D"])
	}
}

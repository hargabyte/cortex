package metrics

import (
	"reflect"
	"sort"
	"testing"

	"github.com/anthropics/cx/internal/bd"
)

func TestComputeInOutDegree(t *testing.T) {
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {},
	}

	inDegree, outDegree := ComputeInOutDegree(graph)

	// Check out-degrees
	expectedOut := map[string]int{"A": 2, "B": 1, "C": 0}
	for node, expected := range expectedOut {
		if outDegree[node] != expected {
			t.Errorf("outDegree[%s] = %d, expected %d", node, outDegree[node], expected)
		}
	}

	// Check in-degrees
	expectedIn := map[string]int{"A": 0, "B": 1, "C": 2}
	for node, expected := range expectedIn {
		if inDegree[node] != expected {
			t.Errorf("inDegree[%s] = %d, expected %d", node, inDegree[node], expected)
		}
	}
}

func TestComputeInOutDegree_WithImplicitNodes(t *testing.T) {
	// D is only referenced as a target, not as a key
	graph := map[string][]string{
		"A": {"B", "D"},
		"B": {"D"},
	}

	inDegree, outDegree := ComputeInOutDegree(graph)

	// D should be discovered from targets
	if inDegree["D"] != 2 {
		t.Errorf("inDegree[D] = %d, expected 2", inDegree["D"])
	}
	if outDegree["D"] != 0 {
		t.Errorf("outDegree[D] = %d, expected 0", outDegree["D"])
	}
}

func TestReverseGraph(t *testing.T) {
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {},
	}

	reversed := ReverseGraph(graph)

	// Check reversed edges
	expected := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A", "B"},
	}

	for node, expectedTargets := range expected {
		actualTargets := reversed[node]
		sort.Strings(actualTargets)
		sort.Strings(expectedTargets)
		if !reflect.DeepEqual(actualTargets, expectedTargets) {
			t.Errorf("reversed[%s] = %v, expected %v", node, actualTargets, expectedTargets)
		}
	}
}

func TestFindRoots(t *testing.T) {
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
		"D": {"C"}, // D is also a root (no incoming)
	}

	roots := FindRoots(graph)
	sort.Strings(roots)

	expected := []string{"A", "D"}
	if !reflect.DeepEqual(roots, expected) {
		t.Errorf("FindRoots = %v, expected %v", roots, expected)
	}
}

func TestFindLeaves(t *testing.T) {
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C", "D"},
		"C": {},
		"D": {},
	}

	leaves := FindLeaves(graph)
	sort.Strings(leaves)

	expected := []string{"C", "D"}
	if !reflect.DeepEqual(leaves, expected) {
		t.Errorf("FindLeaves = %v, expected %v", leaves, expected)
	}
}

func TestComputeGraphStats(t *testing.T) {
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {},
	}

	stats := ComputeGraphStats(graph)

	if stats.NodeCount != 3 {
		t.Errorf("NodeCount = %d, expected 3", stats.NodeCount)
	}
	if stats.EdgeCount != 3 {
		t.Errorf("EdgeCount = %d, expected 3", stats.EdgeCount)
	}
	if stats.MaxOutDegree != 2 {
		t.Errorf("MaxOutDegree = %d, expected 2", stats.MaxOutDegree)
	}
	if stats.MaxInDegree != 2 {
		t.Errorf("MaxInDegree = %d, expected 2", stats.MaxInDegree)
	}
	if stats.RootCount != 1 {
		t.Errorf("RootCount = %d, expected 1", stats.RootCount)
	}
	if stats.LeafCount != 1 {
		t.Errorf("LeafCount = %d, expected 1", stats.LeafCount)
	}

	// Density = 3 / (3 * 2) = 0.5
	expectedDensity := 0.5
	if !floatEquals(stats.Density, expectedDensity, 0.001) {
		t.Errorf("Density = %f, expected %f", stats.Density, expectedDensity)
	}
}

func TestComputeGraphStats_Empty(t *testing.T) {
	graph := make(map[string][]string)
	stats := ComputeGraphStats(graph)

	if stats.NodeCount != 0 {
		t.Errorf("NodeCount for empty graph = %d, expected 0", stats.NodeCount)
	}
}

func TestSubgraph(t *testing.T) {
	graph := map[string][]string{
		"A": {"B", "C", "D"},
		"B": {"C", "D"},
		"C": {"D"},
		"D": {},
	}

	// Extract subgraph with only A, B, C
	subgraph := Subgraph(graph, []string{"A", "B", "C"})

	// D should be excluded from targets
	expected := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {},
	}

	for node, expectedTargets := range expected {
		actualTargets := subgraph[node]
		sort.Strings(actualTargets)
		sort.Strings(expectedTargets)
		if !reflect.DeepEqual(actualTargets, expectedTargets) {
			t.Errorf("subgraph[%s] = %v, expected %v", node, actualTargets, expectedTargets)
		}
	}
}

func TestTransitiveClosure(t *testing.T) {
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C", "D"},
		"C": {"E"},
		"D": {},
		"E": {},
		"F": {"A"}, // F is not reachable from A
	}

	closure := TransitiveClosure(graph, "A")
	sort.Strings(closure)

	expected := []string{"B", "C", "D", "E"}
	if !reflect.DeepEqual(closure, expected) {
		t.Errorf("TransitiveClosure(A) = %v, expected %v", closure, expected)
	}
}

func TestTransitiveClosure_Cycle(t *testing.T) {
	// Graph with a cycle
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"A"},
	}

	closure := TransitiveClosure(graph, "A")
	sort.Strings(closure)

	// Should include all nodes except the start node
	expected := []string{"B", "C"}
	if !reflect.DeepEqual(closure, expected) {
		t.Errorf("TransitiveClosure(A) with cycle = %v, expected %v", closure, expected)
	}
}

func TestTransitiveClosure_NoOutgoing(t *testing.T) {
	graph := map[string][]string{
		"A": {},
		"B": {"A"},
	}

	closure := TransitiveClosure(graph, "A")

	if len(closure) != 0 {
		t.Errorf("TransitiveClosure(A) with no outgoing = %v, expected empty", closure)
	}
}

func TestBuildDependencyGraphFromDeps(t *testing.T) {
	deps := []bd.Dependency{
		{FromID: "A", ToID: "B", DepType: "calls"},
		{FromID: "A", ToID: "C", DepType: "uses_type"},
		{FromID: "B", ToID: "C", DepType: "imports"},
		{FromID: "D", ToID: "E", DepType: "blocks"}, // Not a code dep, should be ignored
	}

	graph := BuildDependencyGraphFromDeps(deps)

	// Check that code deps are included
	if !containsStr(graph["A"], "B") {
		t.Error("expected A -> B (calls)")
	}
	if !containsStr(graph["A"], "C") {
		t.Error("expected A -> C (uses_type)")
	}
	if !containsStr(graph["B"], "C") {
		t.Error("expected B -> C (imports)")
	}

	// Check that non-code deps are excluded
	if containsStr(graph["D"], "E") {
		t.Error("did not expect D -> E (blocks is not a code dep)")
	}
}

func TestIsCodeDependency(t *testing.T) {
	codeDeps := []string{"calls", "uses_type", "imports", "extends", "implements", "references"}
	nonCodeDeps := []string{"blocks", "blocked_by", "related", "parent", "child"}

	for _, dep := range codeDeps {
		if !isCodeDependency(dep) {
			t.Errorf("isCodeDependency(%s) = false, expected true", dep)
		}
	}

	for _, dep := range nonCodeDeps {
		if isCodeDependency(dep) {
			t.Errorf("isCodeDependency(%s) = true, expected false", dep)
		}
	}
}

// containsStr checks if a slice contains a value
func containsStr(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

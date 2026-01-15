package metrics

import (
	"testing"
)

func TestComputePageRank_EmptyGraph(t *testing.T) {
	graph := make(map[string][]string)
	scores := ComputePageRank(graph, DefaultPageRankConfig())

	if scores != nil {
		t.Errorf("expected nil for empty graph, got %v", scores)
	}
}

func TestComputePageRank_SingleNode(t *testing.T) {
	graph := map[string][]string{
		"A": {},
	}
	scores := ComputePageRank(graph, DefaultPageRankConfig())

	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}

	// Single node should have score of 1.0
	if !floatEquals(scores["A"], 1.0, 0.0001) {
		t.Errorf("expected score 1.0 for single node, got %f", scores["A"])
	}
}

func TestComputePageRank_SimpleThreeNodeGraph(t *testing.T) {
	// A -> B -> C (linear chain)
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	}

	scores := ComputePageRank(graph, DefaultPageRankConfig())

	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}

	// C should have highest score (receives from B and dangling contribution from C)
	// B should have middle score (receives from A)
	// A should have lowest score (no incoming links)
	if scores["C"] <= scores["B"] {
		t.Errorf("expected C > B, got C=%f, B=%f", scores["C"], scores["B"])
	}
	if scores["B"] <= scores["A"] {
		t.Errorf("expected B > A, got B=%f, A=%f", scores["B"], scores["A"])
	}

	// Scores should sum to approximately 1.0
	sum := scores["A"] + scores["B"] + scores["C"]
	if !floatEquals(sum, 1.0, 0.001) {
		t.Errorf("expected sum ~1.0, got %f", sum)
	}
}

func TestComputePageRank_CyclicGraph(t *testing.T) {
	// A -> B -> C -> A (cycle)
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"A"},
	}

	scores := ComputePageRank(graph, DefaultPageRankConfig())

	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}

	// In a symmetric cycle, all nodes should have equal scores
	avgScore := 1.0 / 3.0
	tolerance := 0.001

	for node, score := range scores {
		if !floatEquals(score, avgScore, tolerance) {
			t.Errorf("expected node %s to have score ~%f, got %f", node, avgScore, score)
		}
	}
}

func TestComputePageRank_StarGraph(t *testing.T) {
	// Hub with multiple incoming links
	// A -> D, B -> D, C -> D
	graph := map[string][]string{
		"A": {"D"},
		"B": {"D"},
		"C": {"D"},
		"D": {},
	}

	scores := ComputePageRank(graph, DefaultPageRankConfig())

	if len(scores) != 4 {
		t.Fatalf("expected 4 scores, got %d", len(scores))
	}

	// D should have highest score (3 incoming links)
	if scores["D"] <= scores["A"] || scores["D"] <= scores["B"] || scores["D"] <= scores["C"] {
		t.Errorf("expected D to have highest score, got A=%f, B=%f, C=%f, D=%f",
			scores["A"], scores["B"], scores["C"], scores["D"])
	}
}

func TestComputePageRank_Convergence(t *testing.T) {
	// Larger graph to test convergence
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"C", "D"},
		"C": {"D", "E"},
		"D": {"E", "A"},
		"E": {"A"},
	}

	config := DefaultPageRankConfig()
	result := ComputePageRankWithInfo(graph, config)

	if !result.Converged {
		t.Errorf("expected convergence within %d iterations, took %d with delta %f",
			config.MaxIterations, result.Iterations, result.FinalDelta)
	}

	if result.Iterations == 0 {
		t.Error("expected at least 1 iteration")
	}
}

func TestComputePageRank_CustomConfig(t *testing.T) {
	graph := map[string][]string{
		"A": {"B"},
		"B": {"A"},
	}

	// Very strict tolerance
	config := PageRankConfig{
		Damping:       0.85,
		MaxIterations: 1000,
		Tolerance:     0.000001,
	}

	result := ComputePageRankWithInfo(graph, config)

	// Should converge with more iterations due to stricter tolerance
	if !result.Converged {
		t.Logf("did not converge with strict tolerance in %d iterations, delta=%f",
			result.Iterations, result.FinalDelta)
	}
}

func TestComputePageRank_DifferentDamping(t *testing.T) {
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	}

	// Low damping (more random jumps)
	lowDampingConfig := PageRankConfig{
		Damping:       0.50,
		MaxIterations: 100,
		Tolerance:     0.0001,
	}
	lowDampingScores := ComputePageRank(graph, lowDampingConfig)

	// High damping (more link following)
	highDampingConfig := PageRankConfig{
		Damping:       0.95,
		MaxIterations: 100,
		Tolerance:     0.0001,
	}
	highDampingScores := ComputePageRank(graph, highDampingConfig)

	// With higher damping, the difference between C (sink) and A (source) should be larger
	lowDiff := lowDampingScores["C"] - lowDampingScores["A"]
	highDiff := highDampingScores["C"] - highDampingScores["A"]

	if highDiff <= lowDiff {
		t.Errorf("expected higher damping to increase score difference, got low=%f, high=%f",
			lowDiff, highDiff)
	}
}

func TestDefaultPageRankConfig(t *testing.T) {
	config := DefaultPageRankConfig()

	if config.Damping != 0.85 {
		t.Errorf("expected default damping 0.85, got %f", config.Damping)
	}
	if config.MaxIterations != 100 {
		t.Errorf("expected default max iterations 100, got %d", config.MaxIterations)
	}
	if config.Tolerance != 0.0001 {
		t.Errorf("expected default tolerance 0.0001, got %f", config.Tolerance)
	}
}

func TestNormalizeScores(t *testing.T) {
	scores := map[string]float64{
		"A": 0.2,
		"B": 0.3,
		"C": 0.5,
	}

	normalized := NormalizeScores(scores)

	sum := 0.0
	for _, score := range normalized {
		sum += score
	}

	if !floatEquals(sum, 1.0, 0.0001) {
		t.Errorf("expected normalized sum to be 1.0, got %f", sum)
	}
}

func TestNormalizeScores_Empty(t *testing.T) {
	scores := map[string]float64{}
	normalized := NormalizeScores(scores)

	if len(normalized) != 0 {
		t.Errorf("expected empty map, got %v", normalized)
	}
}

func TestTopN(t *testing.T) {
	scores := map[string]float64{
		"A": 0.1,
		"B": 0.4,
		"C": 0.2,
		"D": 0.3,
	}

	top2 := TopN(scores, 2)

	if len(top2) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top2))
	}

	if top2[0].Node != "B" || !floatEquals(top2[0].Score, 0.4, 0.0001) {
		t.Errorf("expected top node B with score 0.4, got %s with %f", top2[0].Node, top2[0].Score)
	}

	if top2[1].Node != "D" || !floatEquals(top2[1].Score, 0.3, 0.0001) {
		t.Errorf("expected second node D with score 0.3, got %s with %f", top2[1].Node, top2[1].Score)
	}
}

func TestTopN_MoreThanAvailable(t *testing.T) {
	scores := map[string]float64{
		"A": 0.1,
		"B": 0.2,
	}

	top10 := TopN(scores, 10)

	if len(top10) != 2 {
		t.Errorf("expected 2 results when requesting more than available, got %d", len(top10))
	}
}

func TestTopN_Zero(t *testing.T) {
	scores := map[string]float64{
		"A": 0.1,
	}

	top0 := TopN(scores, 0)

	if top0 != nil {
		t.Errorf("expected nil for n=0, got %v", top0)
	}
}

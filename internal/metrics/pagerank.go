package metrics

import "math"

// PageRankConfig holds algorithm parameters for PageRank computation.
type PageRankConfig struct {
	// Damping is the damping factor (probability of following a link).
	// Standard value is 0.85. Higher values favor link structure more.
	Damping float64

	// MaxIterations is the maximum number of iterations before stopping.
	// Default is 100.
	MaxIterations int

	// Tolerance is the convergence threshold.
	// Iteration stops when max change between iterations < Tolerance.
	// Default is 0.0001.
	Tolerance float64
}

// DefaultPageRankConfig returns the default PageRank configuration.
func DefaultPageRankConfig() PageRankConfig {
	return PageRankConfig{
		Damping:       0.85,
		MaxIterations: 100,
		Tolerance:     0.0001,
	}
}

// PageRankResult contains the PageRank computation results.
type PageRankResult struct {
	// Scores maps node IDs to their PageRank scores
	Scores map[string]float64

	// Iterations is the number of iterations performed
	Iterations int

	// Converged indicates whether the algorithm converged within MaxIterations
	Converged bool

	// FinalDelta is the maximum change in the last iteration
	FinalDelta float64
}

// ComputePageRank calculates PageRank for all nodes in the graph.
// The graph is represented as map[nodeID][]outgoingNodeIDs.
// Returns a map of node IDs to their PageRank scores.
func ComputePageRank(graph map[string][]string, config PageRankConfig) map[string]float64 {
	result := ComputePageRankWithInfo(graph, config)
	return result.Scores
}

// ComputePageRankWithInfo calculates PageRank and returns detailed results.
// The graph is represented as map[nodeID][]outgoingNodeIDs.
func ComputePageRankWithInfo(graph map[string][]string, config PageRankConfig) PageRankResult {
	n := len(graph)
	if n == 0 {
		return PageRankResult{
			Scores:     nil,
			Iterations: 0,
			Converged:  true,
			FinalDelta: 0,
		}
	}

	// Collect all nodes (including those only appearing as targets)
	allNodes := collectAllNodes(graph)
	n = len(allNodes)

	// Initialize all nodes with 1/N
	pr := make(map[string]float64)
	for node := range allNodes {
		pr[node] = 1.0 / float64(n)
	}

	// Build reverse index for efficient lookup of incoming links
	// incomingLinks[node] = list of (source, outDegree) pairs
	incomingLinks := buildIncomingLinks(graph, allNodes)

	// Iterative computation
	result := PageRankResult{
		Scores:     pr,
		Iterations: 0,
		Converged:  false,
		FinalDelta: 1.0,
	}

	for iter := 0; iter < config.MaxIterations; iter++ {
		newPR := make(map[string]float64)
		maxDelta := 0.0

		// Handle dangling nodes (nodes with no outgoing links)
		// Their PageRank is distributed evenly to all nodes
		danglingSum := 0.0
		for node := range allNodes {
			if len(graph[node]) == 0 {
				danglingSum += pr[node]
			}
		}
		danglingContribution := config.Damping * danglingSum / float64(n)

		for node := range allNodes {
			// Base score (random teleportation)
			newPR[node] = (1.0-config.Damping)/float64(n) + danglingContribution

			// Sum contributions from nodes that link TO this node
			for _, incoming := range incomingLinks[node] {
				newPR[node] += config.Damping * pr[incoming.source] / float64(incoming.outDegree)
			}

			delta := abs(newPR[node] - pr[node])
			if delta > maxDelta {
				maxDelta = delta
			}
		}

		pr = newPR
		result.Iterations = iter + 1
		result.FinalDelta = maxDelta

		// Check convergence
		if maxDelta < config.Tolerance {
			result.Converged = true
			break
		}
	}

	result.Scores = pr
	return result
}

// incomingLink represents a link from source with its out-degree
type incomingLink struct {
	source    string
	outDegree int
}

// collectAllNodes returns a set of all nodes in the graph
// including nodes that only appear as targets
func collectAllNodes(graph map[string][]string) map[string]struct{} {
	allNodes := make(map[string]struct{})

	for node, targets := range graph {
		allNodes[node] = struct{}{}
		for _, target := range targets {
			allNodes[target] = struct{}{}
		}
	}

	return allNodes
}

// buildIncomingLinks builds a reverse index of incoming links for each node
func buildIncomingLinks(graph map[string][]string, allNodes map[string]struct{}) map[string][]incomingLink {
	incoming := make(map[string][]incomingLink)

	// Initialize empty slices for all nodes
	for node := range allNodes {
		incoming[node] = []incomingLink{}
	}

	// Build the reverse index
	for source, targets := range graph {
		outDegree := len(targets)
		if outDegree == 0 {
			continue
		}
		for _, target := range targets {
			incoming[target] = append(incoming[target], incomingLink{
				source:    source,
				outDegree: outDegree,
			})
		}
	}

	return incoming
}

// abs returns the absolute value of x
func abs(x float64) float64 {
	return math.Abs(x)
}

// NormalizeScores normalizes PageRank scores so they sum to 1.0
func NormalizeScores(scores map[string]float64) map[string]float64 {
	if len(scores) == 0 {
		return scores
	}

	sum := 0.0
	for _, score := range scores {
		sum += score
	}

	if sum == 0 {
		return scores
	}

	normalized := make(map[string]float64)
	for node, score := range scores {
		normalized[node] = score / sum
	}

	return normalized
}

// TopN returns the top N nodes by PageRank score
func TopN(scores map[string]float64, n int) []NodeScore {
	if n <= 0 || len(scores) == 0 {
		return nil
	}

	// Convert to slice for sorting
	result := make([]NodeScore, 0, len(scores))
	for node, score := range scores {
		result = append(result, NodeScore{Node: node, Score: score})
	}

	// Sort by score descending
	sortByScoreDesc(result)

	// Return top N
	if n > len(result) {
		n = len(result)
	}
	return result[:n]
}

// NodeScore pairs a node ID with its score
type NodeScore struct {
	Node  string  `json:"node"`
	Score float64 `json:"score"`
}

// sortByScoreDesc sorts NodeScore slice by score in descending order
func sortByScoreDesc(scores []NodeScore) {
	// Simple insertion sort (good for small arrays, no external deps)
	for i := 1; i < len(scores); i++ {
		key := scores[i]
		j := i - 1
		for j >= 0 && scores[j].Score < key.Score {
			scores[j+1] = scores[j]
			j--
		}
		scores[j+1] = key
	}
}

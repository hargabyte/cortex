// Package metrics provides graph centrality metrics for code analysis.
package metrics

import (
	"sort"
)

// Note: NodeScore type is defined in pagerank.go and reused here for consistency.

// ComputeBetweenness calculates betweenness centrality using Brandes algorithm.
// graph: map[nodeID][]neighborIDs (undirected or directed edges)
// Returns: map[nodeID]betweenness score (normalized 0-1)
//
// Betweenness centrality measures how often a node lies on the shortest path
// between other nodes. Nodes with high betweenness are "bottlenecks" in the graph.
//
// Formula:
//
//	BC(v) = Σ σ(s,t|v) / σ(s,t)  for all s≠v≠t
//
// Where:
//
//	σ(s,t) = number of shortest paths from s to t
//	σ(s,t|v) = number of those paths passing through v
func ComputeBetweenness(graph map[string][]string) map[string]float64 {
	n := len(graph)

	// Initialize betweenness scores
	bc := make(map[string]float64)
	for node := range graph {
		bc[node] = 0.0
	}

	if n < 3 {
		// Need at least 3 nodes for betweenness to be meaningful
		return bc
	}

	// Brandes algorithm: BFS from each source
	for source := range graph {
		// Single-source shortest paths
		stack := make([]string, 0)
		pred := make(map[string][]string)  // predecessors on shortest paths
		sigma := make(map[string]float64)  // number of shortest paths
		dist := make(map[string]int)       // distance from source

		for node := range graph {
			pred[node] = make([]string, 0)
			sigma[node] = 0.0
			dist[node] = -1
		}

		sigma[source] = 1.0
		dist[source] = 0

		// BFS
		queue := []string{source}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			stack = append(stack, v)

			for _, w := range graph[v] {
				// w found for first time?
				if dist[w] < 0 {
					dist[w] = dist[v] + 1
					queue = append(queue, w)
				}
				// shortest path to w via v?
				if dist[w] == dist[v]+1 {
					sigma[w] += sigma[v]
					pred[w] = append(pred[w], v)
				}
			}
		}

		// Accumulation
		delta := make(map[string]float64)
		for node := range graph {
			delta[node] = 0.0
		}

		// Stack returns vertices in order of non-increasing distance from source
		for len(stack) > 0 {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			for _, v := range pred[w] {
				delta[v] += (sigma[v] / sigma[w]) * (1 + delta[w])
			}
			if w != source {
				bc[w] += delta[w]
			}
		}
	}

	// Normalize by (n-1)(n-2) for directed graph
	normFactor := float64((n - 1) * (n - 2))
	if normFactor > 0 {
		for node := range bc {
			bc[node] /= normFactor
		}
	}

	return bc
}

// FindBottlenecks returns nodes with betweenness above threshold.
// Bottleneck nodes are critical points in the graph that many paths pass through.
func FindBottlenecks(bc map[string]float64, threshold float64) []string {
	bottlenecks := make([]string, 0)
	for node, score := range bc {
		if score >= threshold {
			bottlenecks = append(bottlenecks, node)
		}
	}
	return bottlenecks
}

// GetTopByBetweenness returns top N nodes by betweenness score.
// Results are sorted in descending order by score.
// Uses the shared NodeScore type from pagerank.go.
func GetTopByBetweenness(bc map[string]float64, n int) []NodeScore {
	scores := make([]NodeScore, 0, len(bc))
	for node, score := range bc {
		scores = append(scores, NodeScore{Node: node, Score: score})
	}

	// Sort descending by score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	if n > len(scores) {
		n = len(scores)
	}

	return scores[:n]
}

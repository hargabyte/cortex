// Package metrics provides importance scoring algorithms for code entities.
// It includes PageRank-based importance calculation and entity classification.
package metrics

import "time"

// Metrics holds computed importance scores for an entity.
type Metrics struct {
	EntityID    string    `json:"entity_id"`
	PageRank    float64   `json:"pagerank"`
	InDegree    int       `json:"in_degree"`
	OutDegree   int       `json:"out_degree"`
	Betweenness float64   `json:"betweenness"`
	ComputedAt  time.Time `json:"computed_at"`
}

// Importance represents the classification level based on PageRank score.
type Importance string

const (
	// Critical importance: PageRank >= 0.50
	Critical Importance = "critical"
	// High importance: PageRank >= 0.30
	High Importance = "high"
	// Medium importance: PageRank >= 0.10
	Medium Importance = "medium"
	// Low importance: PageRank < 0.10
	Low Importance = "low"
)

// ClassifyImportance returns the importance level based on PageRank score.
// Thresholds:
//   - Critical: pr >= 0.50
//   - High: pr >= 0.30
//   - Medium: pr >= 0.10
//   - Low: pr < 0.10
func ClassifyImportance(pr float64) Importance {
	switch {
	case pr >= 0.50:
		return Critical
	case pr >= 0.30:
		return High
	case pr >= 0.10:
		return Medium
	default:
		return Low
	}
}

// IsKeystone returns true if an entity is a keystone component.
// A keystone is defined as having PageRank >= 0.30 AND at least 5 dependents.
// Keystones are critical architectural components that many other parts depend on.
func IsKeystone(pr float64, deps int) bool {
	return pr >= 0.30 && deps >= 5
}

// IsBottleneck returns true if an entity is a bottleneck.
// A bottleneck is defined as having betweenness centrality >= 0.20.
// Bottlenecks are entities that many paths flow through.
func IsBottleneck(betweenness float64) bool {
	return betweenness >= 0.20
}

// ImportanceThresholds contains configurable thresholds for importance classification.
type ImportanceThresholds struct {
	Critical    float64 // Default: 0.50
	High        float64 // Default: 0.30
	Medium      float64 // Default: 0.10
	KeystonePR  float64 // Default: 0.30
	KeystoneDep int     // Default: 5
	Bottleneck  float64 // Default: 0.20
}

// DefaultThresholds returns the default importance thresholds.
func DefaultThresholds() ImportanceThresholds {
	return ImportanceThresholds{
		Critical:    0.50,
		High:        0.30,
		Medium:      0.10,
		KeystonePR:  0.30,
		KeystoneDep: 5,
		Bottleneck:  0.20,
	}
}

// ClassifyWithThresholds returns the importance level using custom thresholds.
func ClassifyWithThresholds(pr float64, t ImportanceThresholds) Importance {
	switch {
	case pr >= t.Critical:
		return Critical
	case pr >= t.High:
		return High
	case pr >= t.Medium:
		return Medium
	default:
		return Low
	}
}

// IsKeystoneWithThresholds returns true if entity is a keystone using custom thresholds.
func IsKeystoneWithThresholds(pr float64, deps int, t ImportanceThresholds) bool {
	return pr >= t.KeystonePR && deps >= t.KeystoneDep
}

// IsBottleneckWithThreshold returns true if entity is a bottleneck using custom threshold.
func IsBottleneckWithThreshold(betweenness float64, threshold float64) bool {
	return betweenness >= threshold
}

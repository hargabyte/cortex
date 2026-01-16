package coverage

import (
	"fmt"
	"sort"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
)

// PriorityTier represents the priority level for a coverage gap
type PriorityTier string

const (
	// PriorityCritical is for keystones with very low coverage (<25%)
	PriorityCritical PriorityTier = "critical"
	// PriorityHigh is for keystones with low coverage (25-50%) or bottlenecks with <25%
	PriorityHigh PriorityTier = "high"
	// PriorityMedium is for normal entities with many callers and low coverage
	PriorityMedium PriorityTier = "medium"
	// PriorityLow is for remaining entities with coverage gaps
	PriorityLow PriorityTier = "low"
)

// CoverageGap represents an entity with insufficient test coverage,
// enriched with priority information based on importance metrics.
type CoverageGap struct {
	// Entity is the code entity with the coverage gap
	Entity *store.Entity `json:"entity"`

	// Metrics contains importance metrics (PageRank, InDegree, etc.)
	Metrics *store.Metrics `json:"metrics"`

	// Coverage contains the coverage data for this entity
	Coverage *EntityCoverage `json:"coverage"`

	// PriorityTier categorizes the gap by priority (critical, high, medium, low)
	PriorityTier PriorityTier `json:"priority_tier"`

	// RiskScore is a computed score combining coverage gap and importance
	// Formula: (1 - coverage/100) * pagerank * in_degree
	RiskScore float64 `json:"risk_score"`

	// IsKeystone indicates if this entity is a keystone (high PageRank)
	IsKeystone bool `json:"is_keystone"`

	// IsBottleneck indicates if this entity is a bottleneck (high betweenness)
	IsBottleneck bool `json:"is_bottleneck"`

	// CallerCount is the number of callers (in_degree) for easy reference
	CallerCount int `json:"caller_count"`
}

// GapsReportOptions configures the coverage gaps report generation.
type GapsReportOptions struct {
	// KeystonesOnly filters to show only keystone entities
	KeystonesOnly bool

	// Threshold is the coverage percentage below which entities are included
	// Default is 75 (include entities with <75% coverage)
	Threshold int

	// ByPriority groups the output by priority tier
	ByPriority bool

	// TopN limits results to the top N gaps by risk score (0 = no limit)
	TopN int
}

// GapsReport contains the generated coverage gaps report.
type GapsReport struct {
	// Gaps is the list of coverage gaps, sorted by risk score descending
	Gaps []CoverageGap `json:"gaps"`

	// ByPriority groups gaps by their priority tier
	ByPriority map[PriorityTier][]CoverageGap `json:"by_priority,omitempty"`

	// Summary contains aggregate statistics
	Summary GapsSummary `json:"summary"`
}

// GapsSummary provides aggregate statistics about coverage gaps.
type GapsSummary struct {
	// TotalGaps is the count of entities with coverage gaps
	TotalGaps int `json:"total_gaps"`

	// CriticalCount is gaps in the critical tier
	CriticalCount int `json:"critical_count"`

	// HighCount is gaps in the high tier
	HighCount int `json:"high_count"`

	// MediumCount is gaps in the medium tier
	MediumCount int `json:"medium_count"`

	// LowCount is gaps in the low tier
	LowCount int `json:"low_count"`

	// TotalKeystones is the total number of keystone entities in the codebase
	TotalKeystones int `json:"total_keystones"`

	// KeystonesWithGaps is keystones that have coverage gaps
	KeystonesWithGaps int `json:"keystones_with_gaps"`

	// Recommendation is an actionable suggestion based on the gaps
	Recommendation string `json:"recommendation"`
}

// DefaultGapsReportOptions returns the default options for gaps report generation.
func DefaultGapsReportOptions() GapsReportOptions {
	return GapsReportOptions{
		KeystonesOnly: false,
		Threshold:     75,
		ByPriority:    false,
		TopN:          0,
	}
}

// GenerateGapsReport analyzes coverage data and generates a prioritized report
// of coverage gaps based on entity importance.
func GenerateGapsReport(s *store.Store, cfg *config.Config, opts GapsReportOptions) (*GapsReport, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Get all active entities
	entities, err := s.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities found - run 'cx scan' first")
	}

	// Collect gaps
	var gaps []CoverageGap
	totalKeystones := 0
	keystonesWithGaps := 0

	for _, entity := range entities {
		// Get metrics for this entity
		metrics, err := s.GetMetrics(entity.ID)
		if err != nil || metrics == nil {
			// Skip entities without metrics
			continue
		}

		// Check if this is a keystone or bottleneck
		isKeystone := metrics.PageRank >= cfg.Metrics.KeystoneThreshold
		isBottleneck := metrics.Betweenness >= cfg.Metrics.BottleneckThreshold

		if isKeystone {
			totalKeystones++
		}

		// Get coverage data
		cov, err := GetEntityCoverage(s, entity.ID)
		if err != nil {
			// No coverage data - treat as 0% coverage
			cov = &EntityCoverage{
				EntityID:        entity.ID,
				CoveragePercent: 0,
				CoveredLines:    []int{},
				UncoveredLines:  []int{},
			}
		}

		// Skip if coverage meets threshold
		if cov.CoveragePercent >= float64(opts.Threshold) {
			continue
		}

		// Skip if keystones-only mode and not a keystone
		if opts.KeystonesOnly && !isKeystone {
			continue
		}

		if isKeystone {
			keystonesWithGaps++
		}

		// Calculate risk score: combines coverage gap with importance
		// Higher risk = lower coverage + higher PageRank + more callers
		riskScore := calculateRiskScore(cov.CoveragePercent, metrics.PageRank, metrics.InDegree)

		// Determine priority tier
		priorityTier := categorizePriority(metrics, cov, cfg)

		gaps = append(gaps, CoverageGap{
			Entity:       entity,
			Metrics:      metrics,
			Coverage:     cov,
			PriorityTier: priorityTier,
			RiskScore:    riskScore,
			IsKeystone:   isKeystone,
			IsBottleneck: isBottleneck,
			CallerCount:  metrics.InDegree,
		})
	}

	// Sort by risk score descending
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].RiskScore > gaps[j].RiskScore
	})

	// Apply TopN limit if specified
	if opts.TopN > 0 && len(gaps) > opts.TopN {
		gaps = gaps[:opts.TopN]
	}

	// Build report
	report := &GapsReport{
		Gaps: gaps,
		Summary: GapsSummary{
			TotalGaps:         len(gaps),
			TotalKeystones:    totalKeystones,
			KeystonesWithGaps: keystonesWithGaps,
		},
	}

	// Group by priority if requested
	if opts.ByPriority || true { // Always compute for summary
		report.ByPriority = groupGapsByPriorityTier(gaps)
		report.Summary.CriticalCount = len(report.ByPriority[PriorityCritical])
		report.Summary.HighCount = len(report.ByPriority[PriorityHigh])
		report.Summary.MediumCount = len(report.ByPriority[PriorityMedium])
		report.Summary.LowCount = len(report.ByPriority[PriorityLow])
	}

	// Generate recommendation
	report.Summary.Recommendation = generateGapsRecommendation(report.Summary)

	// Clear ByPriority if not requested (was computed just for counts)
	if !opts.ByPriority {
		report.ByPriority = nil
	}

	return report, nil
}

// calculateRiskScore computes a risk score for a coverage gap.
// The formula weights coverage gap by entity importance and connectivity.
func calculateRiskScore(coveragePercent float64, pageRank float64, inDegree int) float64 {
	// Coverage gap factor: 0% coverage = 1.0, 100% coverage = 0.0
	coverageGap := 1 - coveragePercent/100.0

	// Importance factor: PageRank indicates overall importance
	// InDegree indicates how many other entities depend on this one

	// Combined score: more coverage gap + higher importance = higher risk
	return coverageGap * pageRank * float64(inDegree+1) // +1 to avoid zero for leaf nodes
}

// categorizePriority determines the priority tier for a coverage gap
// based on entity importance (keystone/bottleneck status) and coverage level.
func categorizePriority(m *store.Metrics, cov *EntityCoverage, cfg *config.Config) PriorityTier {
	isKeystone := m.PageRank >= cfg.Metrics.KeystoneThreshold
	isBottleneck := m.Betweenness >= cfg.Metrics.BottleneckThreshold
	hasManyCalls := m.InDegree >= 5 // Entities called by 5+ others

	// Critical: Keystones with very low coverage
	if isKeystone && cov.CoveragePercent < 25 {
		return PriorityCritical
	}

	// High: Keystones with low coverage OR bottlenecks with very low coverage
	if isKeystone && cov.CoveragePercent < 50 {
		return PriorityHigh
	}
	if isBottleneck && cov.CoveragePercent < 25 {
		return PriorityHigh
	}

	// Medium: Entities with many callers and low coverage, or bottlenecks with low coverage
	if hasManyCalls && cov.CoveragePercent < 25 {
		return PriorityMedium
	}
	if isBottleneck && cov.CoveragePercent < 50 {
		return PriorityMedium
	}
	if isKeystone { // Keystones with 50%+ coverage but still below threshold
		return PriorityMedium
	}

	// Low: Everything else
	return PriorityLow
}

// groupGapsByPriorityTier groups gaps by their priority tier.
func groupGapsByPriorityTier(gaps []CoverageGap) map[PriorityTier][]CoverageGap {
	result := make(map[PriorityTier][]CoverageGap)

	for _, gap := range gaps {
		result[gap.PriorityTier] = append(result[gap.PriorityTier], gap)
	}

	return result
}

// generateGapsRecommendation produces an actionable recommendation based on gap counts.
func generateGapsRecommendation(summary GapsSummary) string {
	if summary.CriticalCount > 0 {
		return "URGENT: Address critical gaps (undertested keystones) before next release"
	}
	if summary.HighCount > 0 {
		return "Address high-priority gaps in the near term to reduce risk"
	}
	if summary.MediumCount > 0 {
		return "Plan to improve coverage for medium-priority entities"
	}
	if summary.LowCount > 0 {
		return "Continue monitoring coverage for remaining entities"
	}
	return "No coverage gaps found - maintain current test practices"
}

// GetGapsByTier returns gaps filtered to a specific priority tier.
func (r *GapsReport) GetGapsByTier(tier PriorityTier) []CoverageGap {
	if r.ByPriority != nil {
		return r.ByPriority[tier]
	}

	// Filter from main gaps list if ByPriority wasn't computed
	var result []CoverageGap
	for _, gap := range r.Gaps {
		if gap.PriorityTier == tier {
			result = append(result, gap)
		}
	}
	return result
}

// GetKeystoneGaps returns only gaps for keystone entities.
func (r *GapsReport) GetKeystoneGaps() []CoverageGap {
	var result []CoverageGap
	for _, gap := range r.Gaps {
		if gap.IsKeystone {
			result = append(result, gap)
		}
	}
	return result
}

// HasCriticalGaps returns true if there are any critical priority gaps.
func (r *GapsReport) HasCriticalGaps() bool {
	return r.Summary.CriticalCount > 0
}

// HasHighPriorityGaps returns true if there are critical or high priority gaps.
func (r *GapsReport) HasHighPriorityGaps() bool {
	return r.Summary.CriticalCount > 0 || r.Summary.HighCount > 0
}

// FormatGapLocation returns a formatted location string for a gap.
func (g *CoverageGap) FormatGapLocation() string {
	if g.Entity.LineEnd != nil {
		return fmt.Sprintf("%s:%d-%d", g.Entity.FilePath, g.Entity.LineStart, *g.Entity.LineEnd)
	}
	return fmt.Sprintf("%s:%d", g.Entity.FilePath, g.Entity.LineStart)
}

// FormatCoverage returns a formatted coverage percentage string.
func (g *CoverageGap) FormatCoverage() string {
	return fmt.Sprintf("%.1f%%", g.Coverage.CoveragePercent)
}

// FormatRiskScore returns a formatted risk score string.
func (g *CoverageGap) FormatRiskScore() string {
	return fmt.Sprintf("%.3f", g.RiskScore)
}

// ImportanceLabel returns a human-readable importance label.
func (g *CoverageGap) ImportanceLabel() string {
	if g.IsKeystone {
		return "keystone"
	}
	if g.IsBottleneck {
		return "bottleneck"
	}
	if g.CallerCount == 0 {
		return "leaf"
	}
	return "normal"
}

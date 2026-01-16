// Package coverage provides test impact analysis and coverage mapping.
// This file implements test impact analysis to show which tests cover entities
// and identify coverage gaps.
package coverage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
)

// ImpactAnalysis represents the result of analyzing test coverage impact for a target.
type ImpactAnalysis struct {
	// Target is the file path or entity ID being analyzed
	Target string `json:"target" yaml:"target"`
	// TargetType is "file" or "entity"
	TargetType string `json:"target_type" yaml:"target_type"`
	// Entities is the list of entities in the target with their coverage info
	Entities []EntityImpact `json:"entities" yaml:"entities"`
	// Summary contains aggregate statistics
	Summary *ImpactSummary `json:"summary" yaml:"summary"`
	// Recommendations for improving coverage
	Recommendations []string `json:"recommendations,omitempty" yaml:"recommendations,omitempty"`
}

// EntityImpact represents coverage impact for a single entity.
type EntityImpact struct {
	// EntityID is the unique entity identifier
	EntityID string `json:"entity_id" yaml:"entity_id"`
	// Name is the entity name
	Name string `json:"name" yaml:"name"`
	// EntityType is function, method, etc.
	EntityType string `json:"entity_type" yaml:"entity_type"`
	// FilePath is the file containing the entity
	FilePath string `json:"file_path" yaml:"file_path"`
	// LineStart is the starting line
	LineStart int `json:"line_start" yaml:"line_start"`
	// LineEnd is the ending line
	LineEnd int `json:"line_end,omitempty" yaml:"line_end,omitempty"`
	// CoveragePercent is the coverage percentage (0-100)
	CoveragePercent float64 `json:"coverage_percent" yaml:"coverage_percent"`
	// CoveringTests lists tests that cover this entity
	CoveringTests []CoveringTest `json:"covering_tests,omitempty" yaml:"covering_tests,omitempty"`
	// Importance indicates the entity's importance level
	Importance string `json:"importance" yaml:"importance"`
	// Recommendation suggests what to do about this entity
	Recommendation string `json:"recommendation,omitempty" yaml:"recommendation,omitempty"`
}

// CoveringTest represents a test that covers an entity.
type CoveringTest struct {
	// TestName is the test function name
	TestName string `json:"test_name" yaml:"test_name"`
	// TestFile is the test file path
	TestFile string `json:"test_file" yaml:"test_file"`
}

// ImpactSummary contains aggregate statistics for the analysis.
type ImpactSummary struct {
	// TotalEntities is the count of entities analyzed
	TotalEntities int `json:"total_entities" yaml:"total_entities"`
	// CoveredEntities is the count with some coverage
	CoveredEntities int `json:"covered_entities" yaml:"covered_entities"`
	// UncoveredEntities is the count with 0% coverage
	UncoveredEntities int `json:"uncovered_entities" yaml:"uncovered_entities"`
	// AverageCoverage is the mean coverage percentage
	AverageCoverage float64 `json:"average_coverage" yaml:"average_coverage"`
	// TotalTests is the count of unique tests covering the target
	TotalTests int `json:"total_tests" yaml:"total_tests"`
	// KeystoneGaps is the count of keystones with inadequate coverage
	KeystoneGaps int `json:"keystone_gaps,omitempty" yaml:"keystone_gaps,omitempty"`
}

// UncoveredEntitiesByFile groups uncovered entities by their file path.
type UncoveredEntitiesByFile struct {
	// Files maps file paths to uncovered entities
	Files map[string][]UncoveredEntity `json:"files" yaml:"files"`
	// TotalUncovered is the total count of uncovered entities
	TotalUncovered int `json:"total_uncovered" yaml:"total_uncovered"`
	// TotalFiles is the count of files with uncovered entities
	TotalFiles int `json:"total_files" yaml:"total_files"`
}

// UncoveredEntity represents an entity with 0% coverage.
type UncoveredEntity struct {
	// EntityID is the unique entity identifier
	EntityID string `json:"entity_id" yaml:"entity_id"`
	// Name is the entity name
	Name string `json:"name" yaml:"name"`
	// EntityType is function, method, etc.
	EntityType string `json:"entity_type" yaml:"entity_type"`
	// LineStart is the starting line
	LineStart int `json:"line_start" yaml:"line_start"`
	// Importance indicates the entity's importance level
	Importance string `json:"importance" yaml:"importance"`
	// Priority is the priority for adding tests (1=highest)
	Priority int `json:"priority" yaml:"priority"`
}

// CoverageRecommendation represents a recommendation for improving coverage.
type CoverageRecommendation struct {
	// EntityID is the entity to test
	EntityID string `json:"entity_id" yaml:"entity_id"`
	// Name is the entity name
	Name string `json:"name" yaml:"name"`
	// FilePath is the file containing the entity
	FilePath string `json:"file_path" yaml:"file_path"`
	// CurrentCoverage is the current coverage percentage
	CurrentCoverage float64 `json:"current_coverage" yaml:"current_coverage"`
	// Importance is the entity importance level
	Importance string `json:"importance" yaml:"importance"`
	// Priority is the priority for adding tests (1=highest)
	Priority int `json:"priority" yaml:"priority"`
	// Reason explains why this entity should be tested
	Reason string `json:"reason" yaml:"reason"`
}

// AnalyzeImpact analyzes test coverage impact for a file or entity.
func AnalyzeImpact(s *store.Store, target string, cfg *config.Config) (*ImpactAnalysis, error) {
	// Determine if target is a file or entity
	var entities []*store.Entity
	var targetType string
	var err error

	// Try as entity ID first
	entity, err := s.GetEntity(target)
	if err == nil {
		targetType = "entity"
		entities = []*store.Entity{entity}
	} else {
		// Try as file path
		targetType = "file"
		entities, err = s.QueryEntities(store.EntityFilter{
			FilePath: target,
			Status:   "active",
		})
		if err != nil {
			return nil, fmt.Errorf("query entities for %s: %w", target, err)
		}
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities found for target: %s", target)
	}

	// Analyze each entity
	var entityImpacts []EntityImpact
	var totalCoverage float64
	coveredCount := 0
	uncoveredCount := 0
	keystoneGaps := 0
	testSet := make(map[string]bool)

	for _, e := range entities {
		impact, err := analyzeEntityImpact(s, e, cfg)
		if err != nil {
			continue // Skip entities we can't analyze
		}

		entityImpacts = append(entityImpacts, *impact)
		totalCoverage += impact.CoveragePercent

		if impact.CoveragePercent > 0 {
			coveredCount++
		} else {
			uncoveredCount++
		}

		// Track unique tests
		for _, t := range impact.CoveringTests {
			testSet[t.TestFile+"::"+t.TestName] = true
		}

		// Count keystone gaps
		if impact.Importance == "keystone" && impact.CoveragePercent < 50 {
			keystoneGaps++
		}
	}

	// Calculate average coverage
	avgCoverage := 0.0
	if len(entityImpacts) > 0 {
		avgCoverage = totalCoverage / float64(len(entityImpacts))
	}

	// Sort entities by coverage (lowest first) then by importance
	sort.Slice(entityImpacts, func(i, j int) bool {
		// Keystones with low coverage come first
		if entityImpacts[i].Importance == "keystone" && entityImpacts[j].Importance != "keystone" {
			return true
		}
		if entityImpacts[j].Importance == "keystone" && entityImpacts[i].Importance != "keystone" {
			return false
		}
		// Then sort by coverage
		return entityImpacts[i].CoveragePercent < entityImpacts[j].CoveragePercent
	})

	// Generate recommendations
	recommendations := generateImpactRecommendations(entityImpacts, keystoneGaps, avgCoverage)

	return &ImpactAnalysis{
		Target:     target,
		TargetType: targetType,
		Entities:   entityImpacts,
		Summary: &ImpactSummary{
			TotalEntities:     len(entityImpacts),
			CoveredEntities:   coveredCount,
			UncoveredEntities: uncoveredCount,
			AverageCoverage:   avgCoverage,
			TotalTests:        len(testSet),
			KeystoneGaps:      keystoneGaps,
		},
		Recommendations: recommendations,
	}, nil
}

// analyzeEntityImpact analyzes coverage for a single entity.
func analyzeEntityImpact(s *store.Store, e *store.Entity, cfg *config.Config) (*EntityImpact, error) {
	// Get coverage data
	cov, err := GetEntityCoverage(s, e.ID)
	coveragePercent := 0.0
	if err == nil && cov != nil {
		coveragePercent = cov.CoveragePercent
	}

	// Get tests covering this entity
	tests, err := GetTestsForEntity(s, e.ID)
	coveringTests := make([]CoveringTest, 0, len(tests))
	if err == nil {
		for _, t := range tests {
			coveringTests = append(coveringTests, CoveringTest{
				TestName: t.TestName,
				TestFile: t.TestFile,
			})
		}
	}

	// Determine importance
	importance := "normal"
	m, err := s.GetMetrics(e.ID)
	if err == nil && m != nil {
		if cfg != nil && m.PageRank >= cfg.Metrics.KeystoneThreshold {
			importance = "keystone"
		} else if m.InDegree >= 5 || m.PageRank >= 0.01 {
			importance = "bottleneck"
		}
	}

	// Generate recommendation based on coverage and importance
	recommendation := generateEntityRecommendation(coveragePercent, importance, len(coveringTests))

	lineEnd := 0
	if e.LineEnd != nil {
		lineEnd = *e.LineEnd
	}

	return &EntityImpact{
		EntityID:        e.ID,
		Name:            e.Name,
		EntityType:      e.EntityType,
		FilePath:        e.FilePath,
		LineStart:       e.LineStart,
		LineEnd:         lineEnd,
		CoveragePercent: coveragePercent,
		CoveringTests:   coveringTests,
		Importance:      importance,
		Recommendation:  recommendation,
	}, nil
}

// GetCoveringTests returns all tests that cover a specific entity.
func GetCoveringTests(s *store.Store, entityID string) ([]CoveringTest, error) {
	tests, err := GetTestsForEntity(s, entityID)
	if err != nil {
		return nil, fmt.Errorf("get tests for entity %s: %w", entityID, err)
	}

	coveringTests := make([]CoveringTest, 0, len(tests))
	for _, t := range tests {
		coveringTests = append(coveringTests, CoveringTest{
			TestName: t.TestName,
			TestFile: t.TestFile,
		})
	}

	return coveringTests, nil
}

// GetUncoveredEntities returns all entities with 0% coverage, grouped by file.
func GetUncoveredEntities(s *store.Store, cfg *config.Config) (*UncoveredEntitiesByFile, error) {
	// Get all active entities
	entities, err := s.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}

	// Group uncovered entities by file
	fileMap := make(map[string][]UncoveredEntity)
	totalUncovered := 0

	for _, e := range entities {
		// Get coverage
		cov, err := GetEntityCoverage(s, e.ID)
		if err != nil || cov == nil {
			// No coverage data - treat as uncovered
		} else if cov.CoveragePercent > 0 {
			continue // Has some coverage
		}

		// Determine importance
		importance := "normal"
		priority := 3
		m, err := s.GetMetrics(e.ID)
		if err == nil && m != nil {
			if cfg != nil && m.PageRank >= cfg.Metrics.KeystoneThreshold {
				importance = "keystone"
				priority = 1
			} else if m.InDegree >= 5 || m.PageRank >= 0.01 {
				importance = "bottleneck"
				priority = 2
			}
		}

		uncovered := UncoveredEntity{
			EntityID:   e.ID,
			Name:       e.Name,
			EntityType: e.EntityType,
			LineStart:  e.LineStart,
			Importance: importance,
			Priority:   priority,
		}

		fileMap[e.FilePath] = append(fileMap[e.FilePath], uncovered)
		totalUncovered++
	}

	// Sort entities within each file by priority
	for filePath := range fileMap {
		sort.Slice(fileMap[filePath], func(i, j int) bool {
			return fileMap[filePath][i].Priority < fileMap[filePath][j].Priority
		})
	}

	return &UncoveredEntitiesByFile{
		Files:          fileMap,
		TotalUncovered: totalUncovered,
		TotalFiles:     len(fileMap),
	}, nil
}

// GenerateCoverageRecommendations generates prioritized recommendations for what to test next.
func GenerateCoverageRecommendations(s *store.Store, cfg *config.Config, limit int) ([]CoverageRecommendation, error) {
	// Get all active entities
	entities, err := s.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}

	type entityScore struct {
		entity     *store.Entity
		metrics    *store.Metrics
		coverage   *EntityCoverage
		score      float64
		importance string
	}

	var scored []entityScore

	for _, e := range entities {
		// Get metrics
		m, err := s.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue // Skip entities without metrics
		}

		// Get coverage
		cov, _ := GetEntityCoverage(s, e.ID)
		coveragePercent := 0.0
		if cov != nil {
			coveragePercent = cov.CoveragePercent
		}

		// Skip already well-covered entities
		if coveragePercent >= 75 {
			continue
		}

		// Determine importance
		importance := "normal"
		if cfg != nil && m.PageRank >= cfg.Metrics.KeystoneThreshold {
			importance = "keystone"
		} else if m.InDegree >= 5 || m.PageRank >= 0.01 {
			importance = "bottleneck"
		}

		// Calculate score: higher = more important to test
		// Score = importance_weight * (1 - coverage/100) * pagerank * in_degree
		importanceWeight := 1.0
		if importance == "keystone" {
			importanceWeight = 3.0
		} else if importance == "bottleneck" {
			importanceWeight = 2.0
		}

		score := importanceWeight * (1 - coveragePercent/100.0) * m.PageRank * float64(m.InDegree+1)

		scored = append(scored, entityScore{
			entity:     e,
			metrics:    m,
			coverage:   cov,
			score:      score,
			importance: importance,
		})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Limit results
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}

	// Build recommendations
	recommendations := make([]CoverageRecommendation, 0, len(scored))
	for i, es := range scored {
		coveragePercent := 0.0
		if es.coverage != nil {
			coveragePercent = es.coverage.CoveragePercent
		}

		reason := generateRecommendationReason(es.importance, coveragePercent, es.metrics)

		recommendations = append(recommendations, CoverageRecommendation{
			EntityID:        es.entity.ID,
			Name:            es.entity.Name,
			FilePath:        es.entity.FilePath,
			CurrentCoverage: coveragePercent,
			Importance:      es.importance,
			Priority:        i + 1,
			Reason:          reason,
		})
	}

	return recommendations, nil
}

// generateEntityRecommendation generates a recommendation for a single entity.
func generateEntityRecommendation(coverage float64, importance string, testCount int) string {
	if coverage == 0 {
		if importance == "keystone" {
			return "CRITICAL: Keystone entity with no test coverage - add tests immediately"
		}
		if importance == "bottleneck" {
			return "HIGH: Bottleneck entity with no coverage - prioritize testing"
		}
		return "Add test coverage for this entity"
	}

	if coverage < 50 {
		if importance == "keystone" {
			return fmt.Sprintf("WARNING: Keystone with only %.0f%% coverage - increase test coverage", coverage)
		}
		if importance == "bottleneck" {
			return fmt.Sprintf("Consider increasing coverage from %.0f%%", coverage)
		}
	}

	if coverage >= 80 {
		return "" // Good coverage, no recommendation needed
	}

	return ""
}

// generateImpactRecommendations generates overall recommendations for the impact analysis.
func generateImpactRecommendations(entities []EntityImpact, keystoneGaps int, avgCoverage float64) []string {
	var recommendations []string

	if keystoneGaps > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("CRITICAL: %d keystone(s) have inadequate coverage (<50%%) - prioritize testing these", keystoneGaps))
	}

	if avgCoverage < 50 {
		recommendations = append(recommendations,
			fmt.Sprintf("Overall coverage is low (%.1f%%) - consider adding more tests", avgCoverage))
	}

	// Find entities needing attention
	var uncoveredKeystones []string
	var uncoveredBottlenecks []string
	for _, e := range entities {
		if e.CoveragePercent == 0 {
			if e.Importance == "keystone" {
				uncoveredKeystones = append(uncoveredKeystones, e.Name)
			} else if e.Importance == "bottleneck" {
				uncoveredBottlenecks = append(uncoveredBottlenecks, e.Name)
			}
		}
	}

	if len(uncoveredKeystones) > 0 {
		names := uncoveredKeystones
		if len(names) > 3 {
			names = names[:3]
		}
		recommendations = append(recommendations,
			fmt.Sprintf("Uncovered keystones: %s", strings.Join(names, ", ")))
	}

	if len(uncoveredBottlenecks) > 0 {
		names := uncoveredBottlenecks
		if len(names) > 3 {
			names = names[:3]
		}
		recommendations = append(recommendations,
			fmt.Sprintf("Uncovered bottlenecks: %s", strings.Join(names, ", ")))
	}

	if len(recommendations) == 0 && avgCoverage >= 75 {
		recommendations = append(recommendations, "Good coverage! Continue maintaining test quality.")
	}

	return recommendations
}

// generateRecommendationReason generates a reason for why an entity should be tested.
func generateRecommendationReason(importance string, coverage float64, m *store.Metrics) string {
	var parts []string

	if importance == "keystone" {
		parts = append(parts, "keystone entity (high PageRank)")
	} else if importance == "bottleneck" {
		parts = append(parts, "bottleneck entity")
	}

	if coverage == 0 {
		parts = append(parts, "no test coverage")
	} else {
		parts = append(parts, fmt.Sprintf("only %.0f%% coverage", coverage))
	}

	if m != nil {
		if m.InDegree >= 10 {
			parts = append(parts, fmt.Sprintf("called by %d other entities", m.InDegree))
		} else if m.InDegree >= 5 {
			parts = append(parts, fmt.Sprintf("called by %d entities", m.InDegree))
		}
	}

	return strings.Join(parts, ", ")
}

package coverage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/anthropics/cx/internal/store"
)

// EntityCoverage represents coverage information for a single entity.
type EntityCoverage struct {
	EntityID        string    `json:"entity_id"`
	CoveragePercent float64   `json:"coverage_percent"`
	CoveredLines    []int     `json:"covered_lines"`
	UncoveredLines  []int     `json:"uncovered_lines"`
	LastRun         time.Time `json:"last_run"`
}

// Mapper maps coverage blocks to code entities in the cx store.
type Mapper struct {
	store        *store.Store
	coverageData *CoverageData
	basePath     string // Base path for normalizing file paths
}

// NewMapper creates a new coverage mapper.
func NewMapper(s *store.Store, coverageData *CoverageData, basePath string) *Mapper {
	return &Mapper{
		store:        s,
		coverageData: coverageData,
		basePath:     basePath,
	}
}

// MapCoverageToEntities maps all coverage blocks to entities and returns entity coverage data.
// It finds entities that overlap with coverage blocks and calculates coverage percentages.
func (m *Mapper) MapCoverageToEntities() ([]EntityCoverage, error) {
	entityCoverageMap := make(map[string]*EntityCoverage)

	// Get all coverage blocks grouped by file
	fileBlocks := m.coverageData.GetFilesWithCoverage()

	// For each file with coverage data
	for filePath, blocks := range fileBlocks {
		// Normalize file path to match cx database
		normalizedPath := m.normalizeFilePath(filePath)

		// Get all entities in this file
		entities, err := m.store.QueryEntities(store.EntityFilter{
			FilePath: normalizedPath,
			Status:   "active",
		})
		if err != nil {
			return nil, fmt.Errorf("query entities for file %s: %w", normalizedPath, err)
		}

		// For each entity, calculate coverage
		for i := range entities {
			coverage := m.calculateEntityCoverage(entities[i], blocks)
			if coverage != nil {
				entityCoverageMap[entities[i].ID] = coverage
			}
		}
	}

	// Convert map to slice
	result := make([]EntityCoverage, 0, len(entityCoverageMap))
	for _, coverage := range entityCoverageMap {
		result = append(result, *coverage)
	}

	return result, nil
}

// calculateEntityCoverage calculates coverage for a single entity based on overlapping blocks.
func (m *Mapper) calculateEntityCoverage(entity *store.Entity, blocks []CoverageBlock) *EntityCoverage {
	if entity.LineEnd == nil {
		// Can't calculate coverage for entities without end line
		return nil
	}

	entityStart := entity.LineStart
	entityEnd := *entity.LineEnd

	// Find all blocks that overlap with this entity's line range
	var overlappingBlocks []CoverageBlock
	for _, block := range blocks {
		if m.blocksOverlap(entityStart, entityEnd, block.StartLine, block.EndLine) {
			overlappingBlocks = append(overlappingBlocks, block)
		}
	}

	if len(overlappingBlocks) == 0 {
		// No coverage data for this entity
		return nil
	}

	// Build line-by-line coverage map
	lineCoverage := make(map[int]bool)
	for _, block := range overlappingBlocks {
		// Mark lines in this block's range
		start := max(block.StartLine, entityStart)
		end := min(block.EndLine, entityEnd)

		for line := start; line <= end; line++ {
			// If block was covered, mark line as covered
			if block.IsCovered() {
				lineCoverage[line] = true
			} else {
				// Only mark uncovered if not already marked as covered
				if _, exists := lineCoverage[line]; !exists {
					lineCoverage[line] = false
				}
			}
		}
	}

	// Calculate covered and uncovered lines
	var coveredLines, uncoveredLines []int
	_ = entityEnd - entityStart + 1 // totalLines available for future use

	for line := entityStart; line <= entityEnd; line++ {
		if covered, exists := lineCoverage[line]; exists {
			if covered {
				coveredLines = append(coveredLines, line)
			} else {
				uncoveredLines = append(uncoveredLines, line)
			}
		}
	}

	// Calculate coverage percentage
	// Only count lines that have coverage data
	linesWithData := len(coveredLines) + len(uncoveredLines)
	var coveragePercent float64
	if linesWithData > 0 {
		coveragePercent = float64(len(coveredLines)) / float64(linesWithData) * 100.0
	}

	return &EntityCoverage{
		EntityID:        entity.ID,
		CoveragePercent: coveragePercent,
		CoveredLines:    coveredLines,
		UncoveredLines:  uncoveredLines,
		LastRun:         time.Now(),
	}
}

// blocksOverlap checks if two line ranges overlap.
func (m *Mapper) blocksOverlap(start1, end1, start2, end2 int) bool {
	return start1 <= end2 && start2 <= end1
}

// normalizeFilePath normalizes a coverage file path to match the cx database format.
// Coverage files use module-qualified paths, we need to convert to relative paths.
func (m *Mapper) normalizeFilePath(coveragePath string) string {
	// Coverage paths often look like: github.com/user/repo/internal/pkg/file.go
	// We need to extract the relative path portion

	// Try to find where the actual project path starts
	// Look for common project structure indicators
	indicators := []string{
		"/internal/",
		"/pkg/",
		"/cmd/",
		"/src/",
	}

	for _, indicator := range indicators {
		if idx := findLastIndex(coveragePath, indicator); idx != -1 {
			// Found an indicator, extract from there
			relPath := coveragePath[idx+1:] // +1 to skip the leading slash
			return relPath
		}
	}

	// If no indicator found, try to match against base path
	if m.basePath != "" {
		// Clean both paths
		cleanCoverage := filepath.Clean(coveragePath)
		cleanBase := filepath.Clean(m.basePath)

		// Try to extract relative path
		if rel, err := filepath.Rel(cleanBase, cleanCoverage); err == nil {
			return rel
		}
	}

	// Last resort: return as-is
	return coveragePath
}

// findLastIndex finds the last occurrence of substr in s.
func findLastIndex(s, substr string) int {
	idx := -1
	offset := 0
	for {
		i := findIndex(s[offset:], substr)
		if i == -1 {
			break
		}
		idx = offset + i
		offset = idx + 1
	}
	return idx
}

// findIndex is a simple string search function.
func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// StoreCoverage persists entity coverage data to the database.
func StoreCoverage(s *store.Store, coverages []EntityCoverage) error {
	for _, cov := range coverages {
		// Marshal line arrays to JSON
		coveredJSON, err := json.Marshal(cov.CoveredLines)
		if err != nil {
			return fmt.Errorf("marshal covered lines for %s: %w", cov.EntityID, err)
		}

		uncoveredJSON, err := json.Marshal(cov.UncoveredLines)
		if err != nil {
			return fmt.Errorf("marshal uncovered lines for %s: %w", cov.EntityID, err)
		}

		// Insert or replace coverage data
		_, err = s.DB().Exec(`
			INSERT OR REPLACE INTO entity_coverage (
				entity_id, coverage_percent, covered_lines, uncovered_lines, last_run
			) VALUES (?, ?, ?, ?, ?)
		`, cov.EntityID, cov.CoveragePercent, string(coveredJSON), string(uncoveredJSON), cov.LastRun.Format(time.RFC3339))

		if err != nil {
			return fmt.Errorf("store coverage for %s: %w", cov.EntityID, err)
		}
	}

	return nil
}

// GetEntityCoverage retrieves coverage data for a specific entity.
func GetEntityCoverage(s *store.Store, entityID string) (*EntityCoverage, error) {
	var cov EntityCoverage
	var coveredJSON, uncoveredJSON, lastRunStr string

	err := s.DB().QueryRow(`
		SELECT entity_id, coverage_percent, covered_lines, uncovered_lines, last_run
		FROM entity_coverage
		WHERE entity_id = ?
	`, entityID).Scan(&cov.EntityID, &cov.CoveragePercent, &coveredJSON, &uncoveredJSON, &lastRunStr)

	if err != nil {
		return nil, err
	}

	// Unmarshal JSON arrays
	if err := json.Unmarshal([]byte(coveredJSON), &cov.CoveredLines); err != nil {
		return nil, fmt.Errorf("unmarshal covered lines: %w", err)
	}

	if err := json.Unmarshal([]byte(uncoveredJSON), &cov.UncoveredLines); err != nil {
		return nil, fmt.Errorf("unmarshal uncovered lines: %w", err)
	}

	// Parse timestamp
	lastRun, err := time.Parse(time.RFC3339, lastRunStr)
	if err != nil {
		return nil, fmt.Errorf("parse last_run: %w", err)
	}
	cov.LastRun = lastRun

	return &cov, nil
}

// GetCoverageStats returns overall coverage statistics.
func GetCoverageStats(s *store.Store) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total entities with coverage
	var totalWithCoverage int
	err := s.DB().QueryRow(`
		SELECT COUNT(*) FROM entity_coverage
	`).Scan(&totalWithCoverage)
	if err != nil {
		return nil, err
	}

	// Calculate average coverage
	var avgCoverage float64
	err = s.DB().QueryRow(`
		SELECT AVG(coverage_percent) FROM entity_coverage
	`).Scan(&avgCoverage)
	if err != nil {
		return nil, err
	}

	// Count entities by coverage ranges
	var fullyCovered, partiallyCovered, notCovered int
	err = s.DB().QueryRow(`
		SELECT
			SUM(CASE WHEN coverage_percent = 100 THEN 1 ELSE 0 END) as fully_covered,
			SUM(CASE WHEN coverage_percent > 0 AND coverage_percent < 100 THEN 1 ELSE 0 END) as partially_covered,
			SUM(CASE WHEN coverage_percent = 0 THEN 1 ELSE 0 END) as not_covered
		FROM entity_coverage
	`).Scan(&fullyCovered, &partiallyCovered, &notCovered)
	if err != nil {
		return nil, err
	}

	// Get last run time
	var lastRunStr string
	err = s.DB().QueryRow(`
		SELECT MAX(last_run) FROM entity_coverage
	`).Scan(&lastRunStr)
	if err == nil && lastRunStr != "" {
		if lastRun, err := time.Parse(time.RFC3339, lastRunStr); err == nil {
			stats["last_run"] = lastRun.Format(time.RFC3339)
		}
	}

	stats["total_entities_with_coverage"] = totalWithCoverage
	stats["average_coverage_percent"] = avgCoverage
	stats["fully_covered"] = fullyCovered
	stats["partially_covered"] = partiallyCovered
	stats["not_covered"] = notCovered

	return stats, nil
}

// TestInfo represents information about a test that covers an entity.
type TestInfo struct {
	TestFile string
	TestName string
}

// GetTestsForEntity retrieves all tests that cover a specific entity.
func GetTestsForEntity(s *store.Store, entityID string) ([]TestInfo, error) {
	rows, err := s.DB().Query(`
		SELECT test_file, test_name
		FROM test_entity_map
		WHERE entity_id = ?
		ORDER BY test_file, test_name
	`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query test_entity_map: %w", err)
	}
	defer rows.Close()

	var tests []TestInfo
	for rows.Next() {
		var test TestInfo
		if err := rows.Scan(&test.TestFile, &test.TestName); err != nil {
			return nil, fmt.Errorf("scan test row: %w", err)
		}
		tests = append(tests, test)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate test rows: %w", err)
	}

	return tests, nil
}

// StoreTestEntityMappings populates the test_entity_map table from per-test GOCOVERDIR data.
// For each test in the GOCOVERDIRData, it maps the coverage to entities and stores which
// tests cover which entities. Returns the number of mappings stored.
func StoreTestEntityMappings(s *store.Store, gocoverData *GOCOVERDIRData, basePath string) (int, error) {
	if !gocoverData.HasPerTestAttribution() {
		return 0, nil
	}

	// Clear existing test mappings (we're replacing them)
	_, err := s.DB().Exec(`DELETE FROM test_entity_map`)
	if err != nil {
		return 0, fmt.Errorf("clear test_entity_map: %w", err)
	}

	totalMappings := 0

	// For each test, map its coverage to entities
	for testName, coverageData := range gocoverData.PerTest {
		// Create mapper for this test's coverage
		mapper := NewMapper(s, coverageData, basePath)
		entityCoverages, err := mapper.MapCoverageToEntities()
		if err != nil {
			// Log warning but continue with other tests
			continue
		}

		// For each entity that this test covers (has any covered lines)
		for _, cov := range entityCoverages {
			if len(cov.CoveredLines) == 0 {
				continue // Test didn't actually cover this entity
			}

			// Derive test file from test name (conventionally TestXxx is in xxx_test.go)
			// But since we don't have that info from GOCOVERDIR, use a placeholder
			testFile := deriveTestFile(testName)

			// Insert the mapping
			_, err := s.DB().Exec(`
				INSERT OR IGNORE INTO test_entity_map (test_file, test_name, entity_id)
				VALUES (?, ?, ?)
			`, testFile, testName, cov.EntityID)
			if err != nil {
				return totalMappings, fmt.Errorf("insert test mapping for %s -> %s: %w", testName, cov.EntityID, err)
			}
			totalMappings++
		}
	}

	return totalMappings, nil
}

// deriveTestFile attempts to derive a test file path from a test name.
// Go test names follow the pattern TestXxx, so we try to construct a reasonable path.
// If we can't determine it, we use the test name as a placeholder.
func deriveTestFile(testName string) string {
	// For now, use a placeholder since GOCOVERDIR doesn't provide test file info
	// A more sophisticated approach would scan *_test.go files to find the test
	return testName + "_test.go"
}

// GetEntitiesForTest retrieves all entities covered by a specific test.
func GetEntitiesForTest(s *store.Store, testName string) ([]string, error) {
	rows, err := s.DB().Query(`
		SELECT entity_id
		FROM test_entity_map
		WHERE test_name = ?
		ORDER BY entity_id
	`, testName)
	if err != nil {
		return nil, fmt.Errorf("query test_entity_map: %w", err)
	}
	defer rows.Close()

	var entityIDs []string
	for rows.Next() {
		var entityID string
		if err := rows.Scan(&entityID); err != nil {
			return nil, fmt.Errorf("scan entity row: %w", err)
		}
		entityIDs = append(entityIDs, entityID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity rows: %w", err)
	}

	return entityIDs, nil
}

// GetAllTestMappings retrieves all test→entity mappings from the database.
func GetAllTestMappings(s *store.Store) (map[string][]string, error) {
	rows, err := s.DB().Query(`
		SELECT test_name, entity_id
		FROM test_entity_map
		ORDER BY test_name, entity_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query test_entity_map: %w", err)
	}
	defer rows.Close()

	mappings := make(map[string][]string)
	for rows.Next() {
		var testName, entityID string
		if err := rows.Scan(&testName, &entityID); err != nil {
			return nil, fmt.Errorf("scan mapping row: %w", err)
		}
		mappings[testName] = append(mappings[testName], entityID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mapping rows: %w", err)
	}

	return mappings, nil
}

package coverage

import (
	"path/filepath"
	"testing"

	"github.com/anthropics/cx/internal/store"
)

func TestMapperIntegration(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	// Open store (will create database and schema)
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	// Verify that coverage tables exist by trying to query them
	var count int
	err = storeDB.DB().QueryRow("SELECT COUNT(*) FROM entity_coverage").Scan(&count)
	if err != nil {
		t.Fatalf("entity_coverage table does not exist: %v", err)
	}

	err = storeDB.DB().QueryRow("SELECT COUNT(*) FROM test_entity_map").Scan(&count)
	if err != nil {
		t.Fatalf("test_entity_map table does not exist: %v", err)
	}
}

func TestStoreCoverageAndRetrieve(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	// Create a test entity first
	entity := &store.Entity{
		ID:         "test-entity-1",
		Name:       "TestFunction",
		EntityType: "function",
		FilePath:   "test.go",
		LineStart:  10,
		Status:     "active",
		Language:   "go",
	}
	endLine := 20
	entity.LineEnd = &endLine

	if err := storeDB.CreateEntity(entity); err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	// Create coverage data
	coverages := []EntityCoverage{
		{
			EntityID:        "test-entity-1",
			CoveragePercent: 75.0,
			CoveredLines:    []int{10, 11, 12, 15},
			UncoveredLines:  []int{13, 14},
		},
	}

	// Store coverage
	if err := StoreCoverage(storeDB, coverages); err != nil {
		t.Fatalf("failed to store coverage: %v", err)
	}

	// Retrieve coverage
	retrieved, err := GetEntityCoverage(storeDB, "test-entity-1")
	if err != nil {
		t.Fatalf("failed to retrieve coverage: %v", err)
	}

	// Verify data
	if retrieved.EntityID != "test-entity-1" {
		t.Errorf("expected entity_id 'test-entity-1', got '%s'", retrieved.EntityID)
	}
	if retrieved.CoveragePercent != 75.0 {
		t.Errorf("expected coverage 75.0, got %.2f", retrieved.CoveragePercent)
	}
	if len(retrieved.CoveredLines) != 4 {
		t.Errorf("expected 4 covered lines, got %d", len(retrieved.CoveredLines))
	}
	if len(retrieved.UncoveredLines) != 2 {
		t.Errorf("expected 2 uncovered lines, got %d", len(retrieved.UncoveredLines))
	}
}

func TestGetCoverageStatsWithData(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	// Create test entities
	for i := 1; i <= 3; i++ {
		entity := &store.Entity{
			ID:         "test-entity-" + string(rune('0'+i)),
			Name:       "TestFunction",
			EntityType: "function",
			FilePath:   "test.go",
			LineStart:  i * 10,
			Status:     "active",
			Language:   "go",
		}
		endLine := i*10 + 10
		entity.LineEnd = &endLine

		if err := storeDB.CreateEntity(entity); err != nil {
			t.Fatalf("failed to create entity %d: %v", i, err)
		}
	}

	// Create coverage data: fully covered, partially covered, not covered
	coverages := []EntityCoverage{
		{
			EntityID:        "test-entity-1",
			CoveragePercent: 100.0,
			CoveredLines:    []int{10, 11, 12, 13},
			UncoveredLines:  []int{},
		},
		{
			EntityID:        "test-entity-2",
			CoveragePercent: 50.0,
			CoveredLines:    []int{20, 21},
			UncoveredLines:  []int{22, 23},
		},
		{
			EntityID:        "test-entity-3",
			CoveragePercent: 0.0,
			CoveredLines:    []int{},
			UncoveredLines:  []int{30, 31, 32, 33},
		},
	}

	// Store coverage
	if err := StoreCoverage(storeDB, coverages); err != nil {
		t.Fatalf("failed to store coverage: %v", err)
	}

	// Get stats
	stats, err := GetCoverageStats(storeDB)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	// Verify stats
	if stats["total_entities_with_coverage"] != 3 {
		t.Errorf("expected 3 entities, got %v", stats["total_entities_with_coverage"])
	}

	if stats["fully_covered"] != 1 {
		t.Errorf("expected 1 fully covered, got %v", stats["fully_covered"])
	}

	if stats["partially_covered"] != 1 {
		t.Errorf("expected 1 partially covered, got %v", stats["partially_covered"])
	}

	if stats["not_covered"] != 1 {
		t.Errorf("expected 1 not covered, got %v", stats["not_covered"])
	}

	avgCov := stats["average_coverage_percent"].(float64)
	expectedAvg := (100.0 + 50.0 + 0.0) / 3.0
	if avgCov < expectedAvg-0.1 || avgCov > expectedAvg+0.1 {
		t.Errorf("expected average coverage ~%.2f, got %.2f", expectedAvg, avgCov)
	}
}

func TestCalculateEntityCoverage(t *testing.T) {
	// Create a mock coverage data and mapper
	tmpDir := t.TempDir()
	cxDir := filepath.Join(tmpDir, ".cx")
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	data := &CoverageData{
		Mode: "set",
		Blocks: []CoverageBlock{
			{FilePath: "test.go", StartLine: 10, EndLine: 15, Count: 1}, // Covered
			{FilePath: "test.go", StartLine: 20, EndLine: 25, Count: 0}, // Not covered
		},
	}

	mapper := NewMapper(storeDB, data, tmpDir)

	// Test entity that overlaps with covered block
	entity1 := &store.Entity{
		ID:        "test-1",
		Name:      "Test1",
		FilePath:  "test.go",
		LineStart: 12,
	}
	end1 := 17
	entity1.LineEnd = &end1

	blocks := data.GetCoverageForFile("test.go")
	coverage1 := mapper.calculateEntityCoverage(entity1, blocks)

	if coverage1 == nil {
		t.Fatal("expected coverage data, got nil")
	}

	// Should have some covered lines (12-15) and some without data (16-17)
	if len(coverage1.CoveredLines) == 0 {
		t.Error("expected some covered lines")
	}

	// Test entity that overlaps with uncovered block
	entity2 := &store.Entity{
		ID:        "test-2",
		Name:      "Test2",
		FilePath:  "test.go",
		LineStart: 22,
	}
	end2 := 27
	entity2.LineEnd = &end2

	coverage2 := mapper.calculateEntityCoverage(entity2, blocks)

	if coverage2 == nil {
		t.Fatal("expected coverage data, got nil")
	}

	// Should have uncovered lines (22-25)
	if len(coverage2.UncoveredLines) == 0 {
		t.Error("expected some uncovered lines")
	}

	if coverage2.CoveragePercent > 0 {
		t.Errorf("expected 0%% coverage for uncovered block, got %.2f%%", coverage2.CoveragePercent)
	}
}

func TestNormalizeFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	mapper := NewMapper(nil, nil, tmpDir)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with internal",
			input:    "github.com/user/project/internal/auth/login.go",
			expected: "internal/auth/login.go",
		},
		{
			name:     "path with pkg",
			input:    "github.com/user/project/pkg/utils/helper.go",
			expected: "pkg/utils/helper.go",
		},
		{
			name:     "path with cmd",
			input:    "github.com/user/project/cmd/server/main.go",
			expected: "cmd/server/main.go",
		},
		{
			name:     "path without indicators",
			input:    "simple/path.go",
			expected: "simple/path.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.normalizeFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

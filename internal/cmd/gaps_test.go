package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/store"
)

func TestGapsCommand(t *testing.T) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "cx-gaps-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cx directory
	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("failed to create .cx dir: %v", err)
	}

	// Create test config
	if _, err := config.SaveDefault(tmpDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	// Create test entities
	entities := []*store.Entity{
		{
			ID:         "sa-fn-test1-HighImportance",
			Name:       "HighImportance",
			EntityType: "function",
			FilePath:   "test/high.go",
			LineStart:  10,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "sa-fn-test2-MediumImportance",
			Name:       "MediumImportance",
			EntityType: "function",
			FilePath:   "test/medium.go",
			LineStart:  20,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "sa-fn-test3-LowImportance",
			Name:       "LowImportance",
			EntityType: "function",
			FilePath:   "test/low.go",
			LineStart:  30,
			Visibility: "private",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	for _, e := range entities {
		if err := storeDB.CreateEntity(e); err != nil {
			t.Fatalf("failed to insert entity: %v", err)
		}
	}

	// Create metrics for entities
	metrics := []*store.Metrics{
		{
			EntityID:    "sa-fn-test1-HighImportance",
			PageRank:    0.35, // Keystone
			InDegree:    10,
			OutDegree:   2,
			Betweenness: 0.15,
			ComputedAt:  time.Now(),
		},
		{
			EntityID:    "sa-fn-test2-MediumImportance",
			PageRank:    0.15, // Normal
			InDegree:    3,
			OutDegree:   1,
			Betweenness: 0.05,
			ComputedAt:  time.Now(),
		},
		{
			EntityID:    "sa-fn-test3-LowImportance",
			PageRank:    0.02, // Leaf
			InDegree:    0,
			OutDegree:   5,
			Betweenness: 0.00,
			ComputedAt:  time.Now(),
		},
	}

	if err := storeDB.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("failed to save metrics: %v", err)
	}

	// Create coverage data
	coverages := []coverage.EntityCoverage{
		{
			EntityID:        "sa-fn-test1-HighImportance",
			CoveragePercent: 20.0, // Critical - keystone with low coverage
			CoveredLines:    []int{10, 11},
			UncoveredLines:  []int{12, 13, 14, 15, 16, 17, 18, 19},
			LastRun:         time.Now(),
		},
		{
			EntityID:        "sa-fn-test2-MediumImportance",
			CoveragePercent: 45.0, // Medium - normal entity with low coverage
			CoveredLines:    []int{20, 21, 22, 23, 24},
			UncoveredLines:  []int{25, 26, 27, 28, 29, 30},
			LastRun:         time.Now(),
		},
		{
			EntityID:        "sa-fn-test3-LowImportance",
			CoveragePercent: 100.0, // No gap - full coverage
			CoveredLines:    []int{30, 31, 32, 33, 34},
			UncoveredLines:  []int{},
			LastRun:         time.Now(),
		},
	}

	if err := coverage.StoreCoverage(storeDB, coverages); err != nil {
		t.Fatalf("failed to store coverage: %v", err)
	}

	// Change to temp directory for test
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	t.Run("basic_gaps_detection", func(t *testing.T) {
		// Test basic gaps detection
		cmd := gapsCmd
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("gaps command failed: %v", err)
		}

		// If we got here, basic execution succeeded
	})

	t.Run("keystones_only_filter", func(t *testing.T) {
		// Test keystones-only filter
		cmd := gapsCmd
		cmd.SetArgs([]string{"--keystones-only"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("gaps --keystones-only failed: %v", err)
		}
	})

	t.Run("threshold_filter", func(t *testing.T) {
		// Test threshold filter
		cmd := gapsCmd
		cmd.SetArgs([]string{"--threshold", "50"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("gaps --threshold failed: %v", err)
		}
	})

	t.Run("risk_categorization", func(t *testing.T) {
		// Test risk categorization logic
		cfg := config.DefaultConfig()

		// Test critical (keystone with <25% coverage)
		risk := categorizeRisk(metrics[0], &coverages[0], cfg)
		if risk != "CRITICAL" {
			t.Errorf("expected CRITICAL risk for keystone with 20%% coverage, got %s", risk)
		}

		// Test medium (normal entity with <25% coverage)
		risk = categorizeRisk(metrics[1], &coverages[1], cfg)
		if risk != "LOW" { // 45% coverage is not <25%, so it's LOW
			t.Errorf("expected LOW risk for normal entity with 45%% coverage, got %s", risk)
		}
	})
}

func TestCategorizeRisk(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name             string
		pagerank         float64
		coverage         float64
		expectedCategory string
	}{
		{
			name:             "keystone_low_coverage",
			pagerank:         0.35,
			coverage:         20.0,
			expectedCategory: "CRITICAL",
		},
		{
			name:             "keystone_medium_coverage",
			pagerank:         0.35,
			coverage:         40.0,
			expectedCategory: "HIGH",
		},
		{
			name:             "keystone_high_coverage",
			pagerank:         0.35,
			coverage:         60.0,
			expectedCategory: "MEDIUM",
		},
		{
			name:             "normal_low_coverage",
			pagerank:         0.15,
			coverage:         20.0,
			expectedCategory: "MEDIUM",
		},
		{
			name:             "normal_medium_coverage",
			pagerank:         0.15,
			coverage:         40.0,
			expectedCategory: "LOW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &store.Metrics{
				EntityID:    "test",
				PageRank:    tt.pagerank,
				InDegree:    5,
				OutDegree:   2,
				Betweenness: 0.1,
				ComputedAt:  time.Now(),
			}

			cov := &coverage.EntityCoverage{
				EntityID:        "test",
				CoveragePercent: tt.coverage,
				CoveredLines:    []int{1, 2, 3},
				UncoveredLines:  []int{4, 5, 6},
				LastRun:         time.Now(),
			}

			category := categorizeRisk(m, cov, cfg)
			if category != tt.expectedCategory {
				t.Errorf("categorizeRisk() = %s, want %s", category, tt.expectedCategory)
			}
		})
	}
}

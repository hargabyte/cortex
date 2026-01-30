package coverage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
)

func TestGenerateGapsReport(t *testing.T) {
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

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	// Create test entities
	entities := []*store.Entity{
		{
			ID:         "sa-fn-test1-KeystoneFunc",
			Name:       "KeystoneFunc",
			EntityType: "function",
			FilePath:   "internal/core/critical.go",
			LineStart:  10,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "sa-fn-test2-BottleneckFunc",
			Name:       "BottleneckFunc",
			EntityType: "function",
			FilePath:   "internal/core/bottleneck.go",
			LineStart:  20,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "sa-fn-test3-NormalFunc",
			Name:       "NormalFunc",
			EntityType: "function",
			FilePath:   "internal/core/normal.go",
			LineStart:  30,
			Visibility: "public",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "sa-fn-test4-LeafFunc",
			Name:       "LeafFunc",
			EntityType: "function",
			FilePath:   "internal/core/leaf.go",
			LineStart:  40,
			Visibility: "private",
			Language:   "go",
			Status:     "active",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         "sa-fn-test5-WellTestedFunc",
			Name:       "WellTestedFunc",
			EntityType: "function",
			FilePath:   "internal/core/tested.go",
			LineStart:  50,
			Visibility: "public",
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
			EntityID:    "sa-fn-test1-KeystoneFunc",
			PageRank:    0.45, // Keystone (high PageRank)
			InDegree:    15,
			OutDegree:   3,
			Betweenness: 0.15,
			ComputedAt:  time.Now(),
		},
		{
			EntityID:    "sa-fn-test2-BottleneckFunc",
			PageRank:    0.18, // Not keystone, but high betweenness
			InDegree:    8,
			OutDegree:   4,
			Betweenness: 0.35, // Bottleneck (high betweenness)
			ComputedAt:  time.Now(),
		},
		{
			EntityID:    "sa-fn-test3-NormalFunc",
			PageRank:    0.12,
			InDegree:    6, // Many callers
			OutDegree:   2,
			Betweenness: 0.08,
			ComputedAt:  time.Now(),
		},
		{
			EntityID:    "sa-fn-test4-LeafFunc",
			PageRank:    0.02,
			InDegree:    0, // Leaf - no callers
			OutDegree:   5,
			Betweenness: 0.00,
			ComputedAt:  time.Now(),
		},
		{
			EntityID:    "sa-fn-test5-WellTestedFunc",
			PageRank:    0.40, // Keystone
			InDegree:    12,
			OutDegree:   2,
			Betweenness: 0.10,
			ComputedAt:  time.Now(),
		},
	}

	if err := storeDB.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("failed to save metrics: %v", err)
	}

	// Create coverage data
	coverages := []EntityCoverage{
		{
			EntityID:        "sa-fn-test1-KeystoneFunc",
			CoveragePercent: 15.0, // Critical - keystone with <25% coverage
			CoveredLines:    []int{10, 11},
			UncoveredLines:  []int{12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23},
			LastRun:         time.Now(),
		},
		{
			EntityID:        "sa-fn-test2-BottleneckFunc",
			CoveragePercent: 22.0, // High - bottleneck with <25% coverage
			CoveredLines:    []int{20, 21, 22},
			UncoveredLines:  []int{23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34},
			LastRun:         time.Now(),
		},
		{
			EntityID:        "sa-fn-test3-NormalFunc",
			CoveragePercent: 20.0, // Medium - many callers with <25% coverage
			CoveredLines:    []int{30, 31},
			UncoveredLines:  []int{32, 33, 34, 35, 36, 37, 38, 39},
			LastRun:         time.Now(),
		},
		{
			EntityID:        "sa-fn-test4-LeafFunc",
			CoveragePercent: 40.0, // Low - leaf with low coverage
			CoveredLines:    []int{40, 41, 42, 43},
			UncoveredLines:  []int{44, 45, 46, 47, 48, 49},
			LastRun:         time.Now(),
		},
		{
			EntityID:        "sa-fn-test5-WellTestedFunc",
			CoveragePercent: 95.0, // Well tested - should not appear in gaps
			CoveredLines:    []int{50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
			UncoveredLines:  []int{60},
			LastRun:         time.Now(),
		},
	}

	if err := StoreCoverage(storeDB, coverages); err != nil {
		t.Fatalf("failed to store coverage: %v", err)
	}

	cfg := config.DefaultConfig()

	t.Run("basic_gaps_report", func(t *testing.T) {
		opts := DefaultGapsReportOptions()
		report, err := GenerateGapsReport(storeDB, cfg, opts)
		if err != nil {
			t.Fatalf("GenerateGapsReport failed: %v", err)
		}

		// Should find 4 gaps (all except WellTestedFunc which has 95% coverage)
		if len(report.Gaps) != 4 {
			t.Errorf("expected 4 gaps, got %d", len(report.Gaps))
		}

		// Check summary counts
		if report.Summary.TotalGaps != 4 {
			t.Errorf("expected TotalGaps=4, got %d", report.Summary.TotalGaps)
		}

		// Check keystones count
		if report.Summary.TotalKeystones != 2 {
			t.Errorf("expected TotalKeystones=2, got %d", report.Summary.TotalKeystones)
		}
	})

	t.Run("keystones_only_filter", func(t *testing.T) {
		opts := GapsReportOptions{
			KeystonesOnly: true,
			Threshold:     75,
		}
		report, err := GenerateGapsReport(storeDB, cfg, opts)
		if err != nil {
			t.Fatalf("GenerateGapsReport failed: %v", err)
		}

		// Should find 1 keystone gap (KeystoneFunc has 15% coverage)
		if len(report.Gaps) != 1 {
			t.Errorf("expected 1 keystone gap, got %d", len(report.Gaps))
		}

		if len(report.Gaps) > 0 && report.Gaps[0].Entity.Name != "KeystoneFunc" {
			t.Errorf("expected KeystoneFunc, got %s", report.Gaps[0].Entity.Name)
		}
	})

	t.Run("threshold_filter", func(t *testing.T) {
		opts := GapsReportOptions{
			Threshold: 30, // Only show <30% coverage
		}
		report, err := GenerateGapsReport(storeDB, cfg, opts)
		if err != nil {
			t.Fatalf("GenerateGapsReport failed: %v", err)
		}

		// Should find 3 gaps (KeystoneFunc=15%, BottleneckFunc=22%, NormalFunc=20%)
		// LeafFunc=40% and WellTestedFunc=95% should be excluded
		if len(report.Gaps) != 3 {
			t.Errorf("expected 3 gaps with threshold=30, got %d", len(report.Gaps))
		}
	})

	t.Run("by_priority_grouping", func(t *testing.T) {
		opts := GapsReportOptions{
			ByPriority: true,
			Threshold:  75,
		}
		report, err := GenerateGapsReport(storeDB, cfg, opts)
		if err != nil {
			t.Fatalf("GenerateGapsReport failed: %v", err)
		}

		if report.ByPriority == nil {
			t.Fatal("expected ByPriority to be populated")
		}

		// Check critical gaps (keystones with <25% coverage)
		criticalGaps := report.GetGapsByTier(PriorityCritical)
		if len(criticalGaps) != 1 {
			t.Errorf("expected 1 critical gap, got %d", len(criticalGaps))
		}
		if len(criticalGaps) > 0 && criticalGaps[0].Entity.Name != "KeystoneFunc" {
			t.Errorf("expected KeystoneFunc in critical, got %s", criticalGaps[0].Entity.Name)
		}

		// Check high gaps (bottlenecks with <25% coverage)
		highGaps := report.GetGapsByTier(PriorityHigh)
		if len(highGaps) != 1 {
			t.Errorf("expected 1 high gap, got %d", len(highGaps))
		}
		if len(highGaps) > 0 && highGaps[0].Entity.Name != "BottleneckFunc" {
			t.Errorf("expected BottleneckFunc in high, got %s", highGaps[0].Entity.Name)
		}
	})

	t.Run("risk_score_ordering", func(t *testing.T) {
		opts := DefaultGapsReportOptions()
		report, err := GenerateGapsReport(storeDB, cfg, opts)
		if err != nil {
			t.Fatalf("GenerateGapsReport failed: %v", err)
		}

		// Gaps should be sorted by risk score descending
		for i := 1; i < len(report.Gaps); i++ {
			if report.Gaps[i].RiskScore > report.Gaps[i-1].RiskScore {
				t.Errorf("gaps not sorted by risk score: %f > %f",
					report.Gaps[i].RiskScore, report.Gaps[i-1].RiskScore)
			}
		}
	})

	t.Run("recommendation_generation", func(t *testing.T) {
		opts := DefaultGapsReportOptions()
		report, err := GenerateGapsReport(storeDB, cfg, opts)
		if err != nil {
			t.Fatalf("GenerateGapsReport failed: %v", err)
		}

		// Should have a recommendation since there are critical gaps
		if report.Summary.Recommendation == "" {
			t.Error("expected recommendation to be set")
		}

		// With critical gaps, should mention urgent
		if report.Summary.CriticalCount > 0 && report.Summary.Recommendation != "URGENT: Address critical gaps (undertested keystones) before next release" {
			t.Errorf("unexpected recommendation: %s", report.Summary.Recommendation)
		}
	})
}

func TestCategorizePriority(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name         string
		pagerank     float64
		betweenness  float64
		inDegree     int
		coverage     float64
		expectedTier PriorityTier
	}{
		{
			name:         "keystone_very_low_coverage",
			pagerank:     0.40,
			betweenness:  0.10,
			inDegree:     10,
			coverage:     15.0,
			expectedTier: PriorityCritical,
		},
		{
			name:         "keystone_low_coverage",
			pagerank:     0.35,
			betweenness:  0.10,
			inDegree:     8,
			coverage:     35.0,
			expectedTier: PriorityHigh,
		},
		{
			name:         "bottleneck_very_low_coverage",
			pagerank:     0.15,
			betweenness:  0.30,
			inDegree:     5,
			coverage:     20.0,
			expectedTier: PriorityHigh,
		},
		{
			name:         "many_callers_low_coverage",
			pagerank:     0.10,
			betweenness:  0.05,
			inDegree:     8, // >= 5 callers
			coverage:     15.0,
			expectedTier: PriorityMedium,
		},
		{
			name:         "normal_low_coverage",
			pagerank:     0.10,
			betweenness:  0.05,
			inDegree:     2,
			coverage:     40.0,
			expectedTier: PriorityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &store.Metrics{
				EntityID:    "test",
				PageRank:    tt.pagerank,
				Betweenness: tt.betweenness,
				InDegree:    tt.inDegree,
				OutDegree:   2,
				ComputedAt:  time.Now(),
			}

			cov := &EntityCoverage{
				EntityID:        "test",
				CoveragePercent: tt.coverage,
				CoveredLines:    []int{1, 2, 3},
				UncoveredLines:  []int{4, 5, 6},
				LastRun:         time.Now(),
			}

			tier := categorizePriority(m, cov, cfg)
			if tier != tt.expectedTier {
				t.Errorf("categorizePriority() = %s, want %s", tier, tt.expectedTier)
			}
		})
	}
}

func TestCalculateRiskScore(t *testing.T) {
	tests := []struct {
		name            string
		coveragePercent float64
		pageRank        float64
		inDegree        int
		expectedMin     float64
		expectedMax     float64
	}{
		{
			name:            "zero_coverage_high_importance",
			coveragePercent: 0,
			pageRank:        0.5,
			inDegree:        10,
			expectedMin:     5.0, // (1-0) * 0.5 * (10+1) = 5.5
			expectedMax:     6.0,
		},
		{
			name:            "full_coverage",
			coveragePercent: 100,
			pageRank:        0.5,
			inDegree:        10,
			expectedMin:     0,
			expectedMax:     0.01, // Should be ~0
		},
		{
			name:            "half_coverage",
			coveragePercent: 50,
			pageRank:        0.4,
			inDegree:        5,
			expectedMin:     1.0, // (1-0.5) * 0.4 * (5+1) = 1.2
			expectedMax:     1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateRiskScore(tt.coveragePercent, tt.pageRank, tt.inDegree)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("calculateRiskScore() = %f, expected between %f and %f",
					score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestGapsReportHelperMethods(t *testing.T) {
	// Create a mock report
	report := &GapsReport{
		Gaps: []CoverageGap{
			{
				Entity: &store.Entity{
					ID:        "test1",
					Name:      "Critical1",
					FilePath:  "test.go",
					LineStart: 10,
				},
				Metrics: &store.Metrics{
					PageRank: 0.4,
					InDegree: 5,
				},
				Coverage: &EntityCoverage{
					CoveragePercent: 10,
				},
				PriorityTier: PriorityCritical,
				IsKeystone:   true,
				CallerCount:  5,
			},
			{
				Entity: &store.Entity{
					ID:        "test2",
					Name:      "High1",
					FilePath:  "test2.go",
					LineStart: 20,
				},
				Metrics: &store.Metrics{
					PageRank: 0.2,
					InDegree: 3,
				},
				Coverage: &EntityCoverage{
					CoveragePercent: 30,
				},
				PriorityTier: PriorityHigh,
				IsBottleneck: true,
				CallerCount:  3,
			},
		},
		Summary: GapsSummary{
			TotalGaps:      2,
			CriticalCount:  1,
			HighCount:      1,
			TotalKeystones: 1,
		},
	}

	t.Run("HasCriticalGaps", func(t *testing.T) {
		if !report.HasCriticalGaps() {
			t.Error("expected HasCriticalGaps() to return true")
		}
	})

	t.Run("HasHighPriorityGaps", func(t *testing.T) {
		if !report.HasHighPriorityGaps() {
			t.Error("expected HasHighPriorityGaps() to return true")
		}
	})

	t.Run("GetKeystoneGaps", func(t *testing.T) {
		keystoneGaps := report.GetKeystoneGaps()
		if len(keystoneGaps) != 1 {
			t.Errorf("expected 1 keystone gap, got %d", len(keystoneGaps))
		}
		if keystoneGaps[0].Entity.Name != "Critical1" {
			t.Errorf("expected Critical1, got %s", keystoneGaps[0].Entity.Name)
		}
	})

	t.Run("CoverageGap_FormatMethods", func(t *testing.T) {
		gap := report.Gaps[0]

		location := gap.FormatGapLocation()
		if location != "test.go:10" {
			t.Errorf("expected 'test.go:10', got '%s'", location)
		}

		cov := gap.FormatCoverage()
		if cov != "10.0%" {
			t.Errorf("expected '10.0%%', got '%s'", cov)
		}

		importance := gap.ImportanceLabel()
		if importance != "keystone" {
			t.Errorf("expected 'keystone', got '%s'", importance)
		}
	})
}

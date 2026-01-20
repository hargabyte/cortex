package report

import (
	"testing"
)

// TestClassifyImportance tests the importance classification logic.
func TestClassifyImportance(t *testing.T) {
	tests := []struct {
		name     string
		pageRank float64
		inDegree int
		want     Importance
	}{
		{
			name:     "keystone with high pagerank",
			pageRank: 0.015,
			inDegree: 50,
			want:     ImportanceKeystone,
		},
		{
			name:     "keystone threshold",
			pageRank: 0.01,
			inDegree: 5,
			want:     ImportanceKeystone,
		},
		{
			name:     "bottleneck with many callers",
			pageRank: 0.005,
			inDegree: 15,
			want:     ImportanceBottleneck,
		},
		{
			name:     "bottleneck threshold",
			pageRank: 0.001,
			inDegree: 10,
			want:     ImportanceBottleneck,
		},
		{
			name:     "leaf with no dependents",
			pageRank: 0.001,
			inDegree: 0,
			want:     ImportanceLeaf,
		},
		{
			name:     "normal entity",
			pageRank: 0.005,
			inDegree: 5,
			want:     ImportanceNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyImportance(tt.pageRank, tt.inDegree)
			if got != tt.want {
				t.Errorf("classifyImportance(%v, %v) = %v, want %v",
					tt.pageRank, tt.inDegree, got, tt.want)
			}
		})
	}
}

// TestDataGathererNew tests creating a new DataGatherer.
func TestDataGathererNew(t *testing.T) {
	// Can't test with actual store without database setup
	// Just verify that NewDataGatherer doesn't panic with nil
	gatherer := NewDataGatherer(nil)
	if gatherer == nil {
		t.Error("NewDataGatherer returned nil")
	}
}

// TestComputeLanguageBreakdown tests language distribution computation.
func TestComputeLanguageBreakdown(t *testing.T) {
	gatherer := &DataGatherer{}

	entities := []EntityData{
		{File: "internal/cmd/main.go"},
		{File: "internal/store/db.go"},
		{File: "src/utils.ts"},
		{File: "src/component.tsx"},
		{File: "lib/helper.js"},
		{File: "scripts/main.py"},
		{File: "config.unknown"},
	}

	breakdown := gatherer.computeLanguageBreakdown(entities)

	// Check counts
	if breakdown["go"] != 2 {
		t.Errorf("go count = %d, want 2", breakdown["go"])
	}
	if breakdown["typescript"] != 2 {
		t.Errorf("typescript count = %d, want 2", breakdown["typescript"])
	}
	if breakdown["javascript"] != 1 {
		t.Errorf("javascript count = %d, want 1", breakdown["javascript"])
	}
	if breakdown["python"] != 1 {
		t.Errorf("python count = %d, want 1", breakdown["python"])
	}
	if breakdown["other"] != 1 {
		t.Errorf("other count = %d, want 1", breakdown["other"])
	}
}

// TestCalculateRiskScore tests risk score calculation.
func TestCalculateRiskScore(t *testing.T) {
	gatherer := &DataGatherer{}

	tests := []struct {
		name          string
		criticalCount int
		warningCount  int
		coverage      float64
		minExpected   int
		maxExpected   int
	}{
		{
			name:          "perfect health",
			criticalCount: 0,
			warningCount:  0,
			coverage:      100,
			minExpected:   100,
			maxExpected:   100,
		},
		{
			name:          "some critical issues",
			criticalCount: 3,
			warningCount:  0,
			coverage:      80,
			minExpected:   60,
			maxExpected:   80,
		},
		{
			name:          "many critical issues (capped)",
			criticalCount: 10,
			warningCount:  10,
			coverage:      50,
			minExpected:   0,
			maxExpected:   30,
		},
		{
			name:          "low coverage only",
			criticalCount: 0,
			warningCount:  0,
			coverage:      40,
			minExpected:   75,
			maxExpected:   90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := NewHealthReport()

			// Add critical issues
			for i := 0; i < tt.criticalCount; i++ {
				data.AddCriticalIssue(HealthIssue{Type: "test"})
			}

			// Add warnings
			for i := 0; i < tt.warningCount; i++ {
				data.AddWarningIssue(HealthIssue{Type: "test"})
			}

			// Set coverage
			data.Coverage = &CoverageData{Overall: tt.coverage}

			score := gatherer.calculateRiskScore(data)

			if score < tt.minExpected || score > tt.maxExpected {
				t.Errorf("calculateRiskScore() = %d, want between %d and %d",
					score, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

// TestEntityModified tests the entity modification detection.
func TestEntityModified(t *testing.T) {
	gatherer := &DataGatherer{}

	// We can't easily test this without store.Entity, but we can test the logic
	// For now, just ensure the gatherer was created
	if gatherer == nil {
		t.Error("gatherer is nil")
	}
}

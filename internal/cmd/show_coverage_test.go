package cmd

import (
	"testing"

	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
)

func TestBuildCoverageOutput(t *testing.T) {
	// Create a mock entity
	entity := &store.Entity{
		ID:        "test-entity-1",
		Name:      "TestFunction",
		FilePath:  "test.go",
		LineStart: 10,
	}
	lineEnd := 50
	entity.LineEnd = &lineEnd

	// Create mock coverage data
	coverageData := &coverage.EntityCoverage{
		EntityID:        "test-entity-1",
		CoveragePercent: 78.5,
		CoveredLines:    []int{10, 11, 12, 20, 21, 22, 30},
		UncoveredLines:  []int{15, 16, 17, 40, 41},
	}

	// Create mock test info (no tests for this simple case)
	tests := []coverage.TestInfo{}

	// Build coverage output
	result := buildCoverageOutput(coverageData, tests, entity, nil)

	// Verify the result
	if result == nil {
		t.Fatal("Expected coverage output, got nil")
	}

	if result.Tested {
		t.Error("Expected Tested to be false when no tests provided")
	}

	if result.Percent != 78.5 {
		t.Errorf("Expected Percent to be 78.5, got %f", result.Percent)
	}

	if len(result.UncoveredLines) != 5 {
		t.Errorf("Expected 5 uncovered lines, got %d", len(result.UncoveredLines))
	}

	if len(result.TestedBy) != 0 {
		t.Errorf("Expected 0 tests, got %d", len(result.TestedBy))
	}
}

func TestBuildCoverageOutputWithTests(t *testing.T) {
	// Create a mock entity
	entity := &store.Entity{
		ID:        "test-entity-1",
		Name:      "TestFunction",
		FilePath:  "test.go",
		LineStart: 10,
	}
	lineEnd := 50
	entity.LineEnd = &lineEnd

	// Create mock coverage data
	coverageData := &coverage.EntityCoverage{
		EntityID:        "test-entity-1",
		CoveragePercent: 100.0,
		CoveredLines:    []int{10, 11, 12, 13, 14, 15},
		UncoveredLines:  []int{},
	}

	// Create mock test info
	tests := []coverage.TestInfo{
		{
			TestFile: "test_test.go",
			TestName: "TestExample",
		},
	}

	// Build coverage output (without store, so location will be file path only)
	result := buildCoverageOutput(coverageData, tests, entity, nil)

	// Verify the result
	if result == nil {
		t.Fatal("Expected coverage output, got nil")
	}

	if !result.Tested {
		t.Error("Expected Tested to be true when tests are provided")
	}

	if result.Percent != 100.0 {
		t.Errorf("Expected Percent to be 100.0, got %f", result.Percent)
	}

	if len(result.UncoveredLines) != 0 {
		t.Errorf("Expected 0 uncovered lines, got %d", len(result.UncoveredLines))
	}

	if len(result.TestedBy) != 1 {
		t.Fatalf("Expected 1 test, got %d", len(result.TestedBy))
	}

	// Check that the test is in the map
	testEntry, exists := result.TestedBy["TestExample"]
	if !exists {
		t.Error("Expected TestExample to be in TestedBy map")
	}

	if testEntry == nil {
		t.Fatal("Expected test entry to be non-nil")
	}

	if testEntry.Location != "test_test.go" {
		t.Errorf("Expected location to be 'test_test.go', got '%s'", testEntry.Location)
	}
}

func TestGroupLinesIntoRanges(t *testing.T) {
	tests := []struct {
		name     string
		lines    []int
		expected [][]int
	}{
		{
			name:     "consecutive lines",
			lines:    []int{1, 2, 3, 4, 5},
			expected: [][]int{{1, 5}},
		},
		{
			name:     "single line",
			lines:    []int{10},
			expected: [][]int{{10, 10}},
		},
		{
			name:     "multiple ranges",
			lines:    []int{1, 2, 3, 10, 11, 20},
			expected: [][]int{{1, 3}, {10, 11}, {20, 20}},
		},
		{
			name:     "empty",
			lines:    []int{},
			expected: nil,
		},
		{
			name:     "unsorted input",
			lines:    []int{5, 1, 3, 2, 4},
			expected: [][]int{{1, 5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupLinesIntoRanges(tt.lines)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d ranges, got %d", len(tt.expected), len(result))
				return
			}

			for i, expectedRange := range tt.expected {
				if len(result[i]) != 2 || result[i][0] != expectedRange[0] || result[i][1] != expectedRange[1] {
					t.Errorf("Range %d: expected %v, got %v", i, expectedRange, result[i])
				}
			}
		})
	}
}

func TestCoverageOutputSchema(t *testing.T) {
	// Test that the Coverage struct matches the expected schema
	cov := &output.Coverage{
		Tested:  true,
		Percent: 85.5,
		TestedBy: map[string]*output.TestEntry{
			"TestOne": {
				Location:    "file.go:10-20",
				CoversLines: [][]int{{10, 15}, {18, 20}},
			},
		},
		UncoveredLines: []int{16, 17},
	}

	// Verify structure
	if !cov.Tested {
		t.Error("Expected Tested to be true")
	}

	if cov.Percent != 85.5 {
		t.Error("Expected Percent to be 85.5")
	}

	if len(cov.TestedBy) != 1 {
		t.Error("Expected 1 test entry")
	}

	testEntry := cov.TestedBy["TestOne"]
	if testEntry == nil {
		t.Fatal("Expected test entry to exist")
	}

	if testEntry.Location != "file.go:10-20" {
		t.Error("Expected location to match")
	}

	if len(testEntry.CoversLines) != 2 {
		t.Error("Expected 2 coverage ranges")
	}

	if len(cov.UncoveredLines) != 2 {
		t.Error("Expected 2 uncovered lines")
	}
}

package diff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/cx/internal/store"
)

func TestParseWithoutStore(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test Go file
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() string {
	return "hello"
}

func goodbye(name string) (string, error) {
	return "goodbye " + name, nil
}

type User struct {
	Name string
	Age  int
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create analyzer (nil store is OK for ParseWithoutStore)
	analyzer := NewInlineDriftAnalyzer(nil, tmpDir)

	// Parse the file
	entities, err := analyzer.ParseWithoutStore(testFile)
	if err != nil {
		t.Fatalf("ParseWithoutStore failed: %v", err)
	}

	// Verify we got the expected entities
	if len(entities) == 0 {
		t.Fatal("expected at least one entity")
	}

	// Check for expected entities
	foundHello := false
	foundGoodbye := false
	foundUser := false

	for _, e := range entities {
		switch e.Name {
		case "hello":
			foundHello = true
			if e.Kind != "function" {
				t.Errorf("hello should be a function, got %s", e.Kind)
			}
			if e.SigHash == "" {
				t.Error("hello should have a signature hash")
			}
		case "goodbye":
			foundGoodbye = true
			if e.Kind != "function" {
				t.Errorf("goodbye should be a function, got %s", e.Kind)
			}
			if len(e.Params) != 1 {
				t.Errorf("goodbye should have 1 param, got %d", len(e.Params))
			}
		case "User":
			foundUser = true
			if e.Kind != "type" {
				t.Errorf("User should be a type, got %s", e.Kind)
			}
		}
	}

	if !foundHello {
		t.Error("did not find hello function")
	}
	if !foundGoodbye {
		t.Error("did not find goodbye function")
	}
	if !foundUser {
		t.Error("did not find User type")
	}
}

func TestParseWithoutStore_UnsupportedLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an unsupported file type
	testFile := filepath.Join(tmpDir, "test.xyz")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	analyzer := NewInlineDriftAnalyzer(nil, tmpDir)

	_, err := analyzer.ParseWithoutStore(testFile)
	if err == nil {
		t.Error("expected error for unsupported file type")
	}
}

func TestParseWithoutStore_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	analyzer := NewInlineDriftAnalyzer(nil, tmpDir)

	_, err := analyzer.ParseWithoutStore(filepath.Join(tmpDir, "nonexistent.go"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestInlineDriftReport_BuildRecommendations(t *testing.T) {
	analyzer := NewInlineDriftAnalyzer(nil, ".")

	tests := []struct {
		name     string
		report   *InlineDriftReport
		wantRecs int
	}{
		{
			name: "clean report",
			report: &InlineDriftReport{
				Summary: &InlineDriftSummary{
					Status: "clean",
				},
			},
			wantRecs: 1, // "No drift detected - safe to proceed"
		},
		{
			name: "signature changes",
			report: &InlineDriftReport{
				Summary: &InlineDriftSummary{
					Status:               "drifted",
					SignatureChanges:     2,
					BreakingChanges:      2,
					TotalAffectedCallers: 5,
				},
			},
			wantRecs: 4, // WARNING, run cx scan, review callers, run cx scan
		},
		{
			name: "body changes only",
			report: &InlineDriftReport{
				Summary: &InlineDriftSummary{
					Status:      "drifted",
					BodyChanges: 3,
				},
			},
			wantRecs: 2, // body changes message, run cx scan
		},
		{
			name: "new entities",
			report: &InlineDriftReport{
				Summary: &InlineDriftSummary{
					Status:      "drifted",
					NewEntities: 2,
				},
			},
			wantRecs: 2, // run cx scan for new entities, run cx scan
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := analyzer.buildRecommendations(tt.report)
			if len(recs) < 1 {
				t.Error("expected at least one recommendation")
			}
		})
	}
}

func TestEntityDrift_Types(t *testing.T) {
	// Test that drift types are properly defined
	driftTypes := []DriftType{
		DriftSignature,
		DriftBody,
		DriftMissing,
		DriftNew,
		DriftFileMissing,
	}

	for _, dt := range driftTypes {
		if dt == "" {
			t.Error("drift type should not be empty")
		}
	}
}

func TestCompareSignatures_NoStore(t *testing.T) {
	// This test verifies the compare function handles missing store gracefully
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() string {
	return "hello"
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Without a real store, we can't do full comparison, but we can test parsing
	analyzer := NewInlineDriftAnalyzer(nil, tmpDir)

	entities, err := analyzer.ParseWithoutStore(testFile)
	if err != nil {
		t.Fatalf("ParseWithoutStore failed: %v", err)
	}

	if len(entities) == 0 {
		t.Fatal("expected at least one entity")
	}
}

func TestAnalyzeFile_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock store for testing
	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("failed to create .cx dir: %v", err)
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	analyzer := NewInlineDriftAnalyzer(storeDB, tmpDir)

	// Try to analyze a non-existent file
	report, err := analyzer.AnalyzeFile("nonexistent.go")
	if err != nil {
		t.Fatalf("AnalyzeFile should not fail for missing file: %v", err)
	}

	// Should have a warning about the missing file
	if len(report.Warnings) == 0 {
		t.Error("expected warning for missing file")
	}
}

func TestAnalyzeFile_WithValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test Go file
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func hello() string {
	return "hello"
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create a mock store for testing
	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("failed to create .cx dir: %v", err)
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer storeDB.Close()

	analyzer := NewInlineDriftAnalyzer(storeDB, tmpDir)

	// Analyze the file (should find new entities since store is empty)
	report, err := analyzer.AnalyzeFile("test.go")
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Should have recommendations
	if len(report.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}

	// Summary should be populated
	if report.Summary == nil {
		t.Fatal("expected summary to be populated")
	}

	if report.Summary.Target != "test.go" {
		t.Errorf("expected target to be test.go, got %s", report.Summary.Target)
	}
}

func TestFormatStoredLocation(t *testing.T) {
	lineEnd := 10
	tests := []struct {
		name     string
		entity   *store.Entity
		expected string
	}{
		{
			name: "single line",
			entity: &store.Entity{
				FilePath:  "test.go",
				LineStart: 5,
			},
			expected: "test.go:5",
		},
		{
			name: "multi line",
			entity: &store.Entity{
				FilePath:  "test.go",
				LineStart: 5,
				LineEnd:   &lineEnd,
			},
			expected: "test.go:5-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStoredLocation(tt.entity)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

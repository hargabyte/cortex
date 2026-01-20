package report

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNewOverviewReport(t *testing.T) {
	before := time.Now()
	report := NewOverviewReport()
	after := time.Now()

	if report == nil {
		t.Fatal("NewOverviewReport() returned nil")
	}

	// Check report header
	if report.Report.Type != ReportTypeOverview {
		t.Errorf("Report.Type = %v, want %v", report.Report.Type, ReportTypeOverview)
	}

	if report.Report.GeneratedAt.Before(before) || report.Report.GeneratedAt.After(after) {
		t.Errorf("GeneratedAt %v not in range [%v, %v]", report.Report.GeneratedAt, before, after)
	}

	// Check empty collections
	if len(report.Keystones) != 0 {
		t.Errorf("Keystones length = %d, want 0", len(report.Keystones))
	}

	if len(report.Modules) != 0 {
		t.Errorf("Modules length = %d, want 0", len(report.Modules))
	}

	if len(report.Diagrams) != 0 {
		t.Errorf("Diagrams length = %d, want 0", len(report.Diagrams))
	}

	if report.Health != nil {
		t.Errorf("Health = %v, want nil", report.Health)
	}
}

func TestOverviewReportData_YAMLMarshal(t *testing.T) {
	report := NewOverviewReport()
	report.Metadata.TotalEntities = 3648
	report.Metadata.ActiveEntities = 3608
	report.Metadata.ArchivedEntities = 40
	report.Metadata.LanguageBreakdown = map[string]int{
		"go":         2500,
		"typescript": 1000,
		"python":     148,
	}

	report.Statistics.ByType = map[string]int{
		"function": 1500,
		"method":   800,
		"type":     400,
		"constant": 200,
		"variable": 100,
	}

	report.Statistics.ByLanguage = map[string]int{
		"go":         2500,
		"typescript": 1000,
		"python":     148,
	}

	report.Keystones = []EntityData{
		{
			ID:         "sa-fn-abc123",
			Name:       "Store.GetEntity",
			Type:       "method",
			File:       "internal/store/entity.go",
			Lines:      [2]int{45, 89},
			Importance: ImportanceKeystone,
			PageRank:   0.0456,
			Coverage:   95.0,
			InDegree:   89,
		},
		{
			ID:         "sa-fn-def456",
			Name:       "Scanner.ScanFile",
			Type:       "method",
			File:       "internal/scanner/scanner.go",
			Lines:      [2]int{12, 56},
			Importance: ImportanceKeystone,
			PageRank:   0.0234,
			Coverage:   88.0,
			InDegree:   45,
		},
	}

	report.Modules = []ModuleData{
		{
			Path:      "internal/store",
			Entities:  234,
			Functions: 120,
			Types:     45,
			Coverage:  89.0,
		},
		{
			Path:      "internal/scanner",
			Entities:  156,
			Functions: 89,
			Types:     23,
			Coverage:  76.0,
		},
	}

	report.Diagrams = map[string]DiagramData{
		"architecture": {
			Title: "System Architecture",
			D2:    "direction: right\n# D2 code here",
		},
	}

	report.Health = &HealthSummary{
		CoverageOverall:      78.5,
		UntestedKeystones:    3,
		CircularDependencies: 0,
		DeadCodeCandidates:   12,
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal YAML: %v", err)
	}

	if len(yamlData) == 0 {
		t.Fatal("YAML output is empty")
	}

	// Unmarshal back
	var unmarshaled OverviewReportData
	err = yaml.Unmarshal(yamlData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify round-trip
	if unmarshaled.Report.Type != ReportTypeOverview {
		t.Errorf("Unmarshaled Report.Type = %v, want %v", unmarshaled.Report.Type, ReportTypeOverview)
	}

	if unmarshaled.Metadata.TotalEntities != 3648 {
		t.Errorf("Unmarshaled TotalEntities = %d, want 3648", unmarshaled.Metadata.TotalEntities)
	}

	if len(unmarshaled.Keystones) != 2 {
		t.Errorf("Unmarshaled Keystones length = %d, want 2", len(unmarshaled.Keystones))
	}

	if len(unmarshaled.Modules) != 2 {
		t.Errorf("Unmarshaled Modules length = %d, want 2", len(unmarshaled.Modules))
	}

	if unmarshaled.Health == nil {
		t.Fatal("Unmarshaled Health is nil")
	}

	if unmarshaled.Health.CoverageOverall != 78.5 {
		t.Errorf("Unmarshaled CoverageOverall = %f, want 78.5", unmarshaled.Health.CoverageOverall)
	}
}

func TestOverviewReportData_JSONMarshal(t *testing.T) {
	report := NewOverviewReport()
	report.Metadata.TotalEntities = 3648
	report.Metadata.ActiveEntities = 3608

	report.Statistics.ByType = map[string]int{
		"function": 1500,
		"method":   800,
	}

	report.Keystones = []EntityData{
		{
			ID:         "sa-fn-abc123",
			Name:       "Store.GetEntity",
			Type:       "method",
			File:       "internal/store/entity.go",
			Lines:      [2]int{45, 89},
			Importance: ImportanceKeystone,
			PageRank:   0.0456,
			Coverage:   95.0,
			InDegree:   89,
		},
	}

	report.Modules = []ModuleData{
		{
			Path:      "internal/store",
			Entities:  234,
			Functions: 120,
			Types:     45,
			Coverage:  89.0,
		},
	}

	report.Diagrams = map[string]DiagramData{
		"architecture": {
			Title: "System Architecture",
			D2:    "direction: right",
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Fatal("JSON output is empty")
	}

	// Unmarshal back
	var unmarshaled OverviewReportData
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify round-trip
	if unmarshaled.Report.Type != ReportTypeOverview {
		t.Errorf("Unmarshaled Report.Type = %v, want %v", unmarshaled.Report.Type, ReportTypeOverview)
	}

	if unmarshaled.Metadata.TotalEntities != 3648 {
		t.Errorf("Unmarshaled TotalEntities = %d, want 3648", unmarshaled.Metadata.TotalEntities)
	}

	if len(unmarshaled.Keystones) != 1 {
		t.Errorf("Unmarshaled Keystones length = %d, want 1", len(unmarshaled.Keystones))
	}

	if len(unmarshaled.Modules) != 1 {
		t.Errorf("Unmarshaled Modules length = %d, want 1", len(unmarshaled.Modules))
	}
}

func TestOverviewReportData_FieldOrdering(t *testing.T) {
	report := NewOverviewReport()
	report.Metadata.TotalEntities = 1000

	// Marshal to YAML
	yamlData, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal YAML: %v", err)
	}

	yamlStr := string(yamlData)

	// Check that report comes before metadata, metadata before statistics, etc.
	// This verifies YAML field ordering matches struct definition
	reportIdx := findStringIndex(yamlStr, "report:")
	metadataIdx := findStringIndex(yamlStr, "metadata:")
	statisticsIdx := findStringIndex(yamlStr, "statistics:")
	keystonesIdx := findStringIndex(yamlStr, "keystones:")
	modulesIdx := findStringIndex(yamlStr, "modules:")
	diagramsIdx := findStringIndex(yamlStr, "diagrams:")

	if reportIdx == -1 || metadataIdx == -1 || statisticsIdx == -1 {
		t.Fatal("YAML missing expected sections")
	}

	if reportIdx >= metadataIdx {
		t.Error("Field ordering: report should come before metadata")
	}

	if metadataIdx >= statisticsIdx {
		t.Error("Field ordering: metadata should come before statistics")
	}

	if statisticsIdx >= keystonesIdx {
		t.Error("Field ordering: statistics should come before keystones")
	}

	if keystonesIdx >= modulesIdx {
		t.Error("Field ordering: keystones should come before modules")
	}

	if modulesIdx >= diagramsIdx {
		t.Error("Field ordering: modules should come before diagrams")
	}
}

// findStringIndex finds the index of a substring in a string, or -1 if not found
func findStringIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

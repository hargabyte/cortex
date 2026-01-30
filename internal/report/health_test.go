package report

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewHealthReport(t *testing.T) {
	report := NewHealthReport()

	if report == nil {
		t.Fatal("NewHealthReport returned nil")
	}

	if report.Report.Type != ReportTypeHealth {
		t.Errorf("Expected report type %s, got %s", ReportTypeHealth, report.Report.Type)
	}

	if report.Report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}

	if report.RiskScore != 0 {
		t.Errorf("Initial RiskScore should be 0, got %d", report.RiskScore)
	}

	if report.Issues.Critical == nil || len(report.Issues.Critical) != 0 {
		t.Error("Critical issues should be initialized as empty slice")
	}

	if report.Issues.Warning == nil || len(report.Issues.Warning) != 0 {
		t.Error("Warning issues should be initialized as empty slice")
	}

	if report.Issues.Info == nil || len(report.Issues.Info) != 0 {
		t.Error("Info issues should be initialized as empty slice")
	}

	if report.Diagrams == nil {
		t.Error("Diagrams should be initialized as non-nil map")
	}
}

func TestHealthReportAddCriticalIssue(t *testing.T) {
	report := NewHealthReport()

	issue := HealthIssue{
		Type:           IssueTypeUntestedKeystone,
		Entity:         "AuthService",
		File:           "internal/auth/service.go",
		Coverage:       0,
		Importance:     ImportanceKeystone,
		Recommendation: "Add unit tests for AuthService",
	}

	report.AddCriticalIssue(issue)

	if len(report.Issues.Critical) != 1 {
		t.Errorf("Expected 1 critical issue, got %d", len(report.Issues.Critical))
	}

	if report.Issues.Critical[0].Entity != "AuthService" {
		t.Errorf("Expected entity AuthService, got %s", report.Issues.Critical[0].Entity)
	}
}

func TestHealthReportAddWarningIssue(t *testing.T) {
	report := NewHealthReport()

	issue := HealthIssue{
		Type:           IssueTypeLowCoverageBottle,
		Entity:         "CacheService",
		Coverage:       45.0,
		Importance:     ImportanceBottleneck,
		Recommendation: "Increase test coverage for frequently-used code",
	}

	report.AddWarningIssue(issue)

	if len(report.Issues.Warning) != 1 {
		t.Errorf("Expected 1 warning issue, got %d", len(report.Issues.Warning))
	}

	if report.Issues.Warning[0].Coverage != 45.0 {
		t.Errorf("Expected coverage 45.0, got %f", report.Issues.Warning[0].Coverage)
	}
}

func TestHealthReportAddInfoIssue(t *testing.T) {
	report := NewHealthReport()

	issue := HealthIssue{
		Type:           IssueTypeDeadCodeCandidate,
		Entity:         "UnusedHelper",
		File:           "internal/util/helpers.go",
		InDegree:       0,
		Recommendation: "Consider removing if truly unused",
	}

	report.AddInfoIssue(issue)

	if len(report.Issues.Info) != 1 {
		t.Errorf("Expected 1 info issue, got %d", len(report.Issues.Info))
	}

	if report.Issues.Info[0].InDegree != 0 {
		t.Errorf("Expected InDegree 0, got %d", report.Issues.Info[0].InDegree)
	}
}

func TestHealthReportSetCoverage(t *testing.T) {
	report := NewHealthReport()

	coverage := &CoverageData{
		Overall: 78.5,
		ByEntity: map[string]float64{
			"LoginUser":     85.5,
			"ValidateToken": 92.0,
			"SessionCache":  45.0,
		},
		ByImportance: map[string]float64{
			"keystone":   85.0,
			"bottleneck": 72.0,
			"normal":     78.0,
			"leaf":       65.0,
		},
	}

	report.SetCoverage(coverage)

	if report.Coverage == nil {
		t.Fatal("Coverage should not be nil after SetCoverage")
	}

	if report.Coverage.Overall != 78.5 {
		t.Errorf("Expected overall coverage 78.5, got %f", report.Coverage.Overall)
	}
}

func TestHealthReportSetComplexity(t *testing.T) {
	report := NewHealthReport()

	complexity := &ComplexityAnalysis{
		Hotspots: []ComplexityHotspot{
			{
				Entity:     "ComplexParser",
				OutDegree:  45,
				Lines:      234,
				Cyclomatic: 23,
			},
		},
	}

	report.SetComplexity(complexity)

	if report.Complexity == nil {
		t.Fatal("Complexity should not be nil after SetComplexity")
	}

	if len(report.Complexity.Hotspots) != 1 {
		t.Errorf("Expected 1 hotspot, got %d", len(report.Complexity.Hotspots))
	}

	if report.Complexity.Hotspots[0].Entity != "ComplexParser" {
		t.Errorf("Expected entity ComplexParser, got %s", report.Complexity.Hotspots[0].Entity)
	}
}

func TestHealthReportAddDiagram(t *testing.T) {
	report := NewHealthReport()

	diagram := DiagramData{
		Title: "Risk Heat Map",
		D2:    "direction: right\nNode1 -> Node2",
	}

	report.AddDiagram("risk_map", diagram)

	if len(report.Diagrams) != 1 {
		t.Errorf("Expected 1 diagram, got %d", len(report.Diagrams))
	}

	if d, ok := report.Diagrams["risk_map"]; !ok {
		t.Error("Expected diagram with key 'risk_map' not found")
	} else if d.Title != "Risk Heat Map" {
		t.Errorf("Expected title 'Risk Heat Map', got '%s'", d.Title)
	}
}

func TestHealthReportIssueCounts(t *testing.T) {
	report := NewHealthReport()

	// Add various issues
	report.AddCriticalIssue(HealthIssue{Type: IssueTypeUntestedKeystone})
	report.AddCriticalIssue(HealthIssue{Type: IssueTypeUntestedKeystone})

	report.AddWarningIssue(HealthIssue{Type: IssueTypeLowCoverageBottle})

	report.AddInfoIssue(HealthIssue{Type: IssueTypeDeadCodeCandidate})
	report.AddInfoIssue(HealthIssue{Type: IssueTypeDeadCodeCandidate})
	report.AddInfoIssue(HealthIssue{Type: IssueTypeDeadCodeCandidate})

	if report.CriticalIssueCount() != 2 {
		t.Errorf("Expected 2 critical issues, got %d", report.CriticalIssueCount())
	}

	if report.WarningIssueCount() != 1 {
		t.Errorf("Expected 1 warning issue, got %d", report.WarningIssueCount())
	}

	if report.InfoIssueCount() != 3 {
		t.Errorf("Expected 3 info issues, got %d", report.InfoIssueCount())
	}

	if report.TotalIssues() != 6 {
		t.Errorf("Expected 6 total issues, got %d", report.TotalIssues())
	}
}

func TestHealthReportYAMLMarshal(t *testing.T) {
	report := NewHealthReport()
	report.RiskScore = 72
	report.Report.Query = ""

	// Add some issues
	report.AddCriticalIssue(HealthIssue{
		Type:           IssueTypeUntestedKeystone,
		Entity:         "CriticalFunction",
		File:           "internal/core/critical.go",
		Coverage:       0.0,
		Importance:     ImportanceKeystone,
		Recommendation: "Add tests for this high-importance function",
	})

	report.AddWarningIssue(HealthIssue{
		Type:           IssueTypeLowCoverageBottle,
		Entity:         "SessionCache.Get",
		Coverage:       45.0,
		Importance:     ImportanceBottleneck,
		Recommendation: "Increase coverage for frequently-used code",
	})

	report.AddDiagram("risk_map", DiagramData{
		Title: "Risk Heat Map",
		D2:    "direction: right\nNode1 -> Node2",
	})

	// Marshal to YAML
	data, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal report to YAML: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("YAML output is empty")
	}

	// Verify some key fields are in the output
	yamlStr := string(data)
	if !contains(yamlStr, "report:") {
		t.Error("YAML output should contain 'report:' field")
	}
	if !contains(yamlStr, "risk_score:") {
		t.Error("YAML output should contain 'risk_score:' field")
	}
	if !contains(yamlStr, "issues:") {
		t.Error("YAML output should contain 'issues:' field")
	}
	if !contains(yamlStr, "diagrams:") {
		t.Error("YAML output should contain 'diagrams:' field")
	}
}

func TestHealthReportYAMLUnmarshal(t *testing.T) {
	yamlData := `
report:
  type: health
  generated_at: "2026-01-20T15:30:00Z"
risk_score: 72
issues:
  critical:
    - type: untested_keystone
      entity: CriticalFunction
      file: internal/core/critical.go
      coverage: 0
      importance: keystone
      recommendation: "Add tests for this high-importance function"
  warning:
    - type: low_coverage_bottleneck
      entity: SessionCache.Get
      coverage: 45
      importance: bottleneck
      recommendation: "Increase coverage for frequently-used code"
coverage:
  overall: 78.5
diagrams:
  risk_map:
    title: "Risk Heat Map"
    d2: "direction: right"
`

	var report HealthReportData
	err := yaml.Unmarshal([]byte(yamlData), &report)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if report.Report.Type != ReportTypeHealth {
		t.Errorf("Expected report type %s, got %s", ReportTypeHealth, report.Report.Type)
	}

	if report.RiskScore != 72 {
		t.Errorf("Expected risk score 72, got %d", report.RiskScore)
	}

	if len(report.Issues.Critical) != 1 {
		t.Errorf("Expected 1 critical issue, got %d", len(report.Issues.Critical))
	}

	if report.Issues.Critical[0].Entity != "CriticalFunction" {
		t.Errorf("Expected entity CriticalFunction, got %s", report.Issues.Critical[0].Entity)
	}

	if len(report.Issues.Warning) != 1 {
		t.Errorf("Expected 1 warning issue, got %d", len(report.Issues.Warning))
	}

	if report.Coverage == nil {
		t.Error("Coverage should not be nil")
	} else if report.Coverage.Overall != 78.5 {
		t.Errorf("Expected coverage 78.5, got %f", report.Coverage.Overall)
	}

	if len(report.Diagrams) != 1 {
		t.Errorf("Expected 1 diagram, got %d", len(report.Diagrams))
	}
}

func TestHealthReportJSONMarshal(t *testing.T) {
	report := NewHealthReport()
	report.RiskScore = 72

	report.AddCriticalIssue(HealthIssue{
		Type:           IssueTypeUntestedKeystone,
		Entity:         "CriticalFunction",
		File:           "internal/core/critical.go",
		Coverage:       0.0,
		Importance:     ImportanceKeystone,
		Recommendation: "Add tests for this high-importance function",
	})

	report.SetCoverage(&CoverageData{
		Overall: 78.5,
		ByEntity: map[string]float64{
			"CriticalFunction": 0,
		},
	})

	// Marshal to JSON
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal report to JSON: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("JSON output is empty")
	}

	// Verify it's valid JSON by unmarshaling back
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON back: %v", err)
	}

	// Verify some key fields
	if _, ok := result["report"]; !ok {
		t.Error("JSON output should contain 'report' field")
	}
	if _, ok := result["risk_score"]; !ok {
		t.Error("JSON output should contain 'risk_score' field")
	}
	if _, ok := result["issues"]; !ok {
		t.Error("JSON output should contain 'issues' field")
	}
}

func TestHealthReportJSONUnmarshal(t *testing.T) {
	jsonData := `{
		"report": {
			"type": "health",
			"generated_at": "2026-01-20T15:30:00Z"
		},
		"risk_score": 72,
		"issues": {
			"critical": [
				{
					"type": "untested_keystone",
					"entity": "CriticalFunction",
					"file": "internal/core/critical.go",
					"coverage": 0,
					"importance": "keystone",
					"recommendation": "Add tests for this high-importance function"
				}
			],
			"warning": [],
			"info": []
		},
		"coverage": {
			"overall": 78.5
		},
		"diagrams": {
			"risk_map": {
				"title": "Risk Heat Map",
				"d2": "direction: right"
			}
		}
	}`

	var report HealthReportData
	err := json.Unmarshal([]byte(jsonData), &report)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if report.Report.Type != ReportTypeHealth {
		t.Errorf("Expected report type %s, got %s", ReportTypeHealth, report.Report.Type)
	}

	if report.RiskScore != 72 {
		t.Errorf("Expected risk score 72, got %d", report.RiskScore)
	}

	if len(report.Issues.Critical) != 1 {
		t.Errorf("Expected 1 critical issue, got %d", len(report.Issues.Critical))
	}

	if report.Coverage == nil {
		t.Error("Coverage should not be nil")
	} else if report.Coverage.Overall != 78.5 {
		t.Errorf("Expected coverage 78.5, got %f", report.Coverage.Overall)
	}
}

func TestComplexityAnalysis(t *testing.T) {
	hotspots := []ComplexityHotspot{
		{
			Entity:     "ComplexParser",
			OutDegree:  45,
			Lines:      234,
			Cyclomatic: 23,
		},
		{
			Entity:     "DataTransformer",
			OutDegree:  32,
			Lines:      189,
			Cyclomatic: 18,
		},
	}

	analysis := &ComplexityAnalysis{
		Hotspots: hotspots,
	}

	if len(analysis.Hotspots) != 2 {
		t.Errorf("Expected 2 hotspots, got %d", len(analysis.Hotspots))
	}

	if analysis.Hotspots[0].Entity != "ComplexParser" {
		t.Errorf("Expected entity ComplexParser, got %s", analysis.Hotspots[0].Entity)
	}

	if analysis.Hotspots[1].Lines != 189 {
		t.Errorf("Expected 189 lines, got %d", analysis.Hotspots[1].Lines)
	}
}

package report

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNewFeatureReport(t *testing.T) {
	query := "authentication"
	report := NewFeatureReport(query)

	if report == nil {
		t.Fatal("NewFeatureReport returned nil")
	}

	if report.Report.Type != ReportTypeFeature {
		t.Errorf("Report.Type = %v, want %v", report.Report.Type, ReportTypeFeature)
	}

	if report.Report.Query != query {
		t.Errorf("Report.Query = %v, want %v", report.Report.Query, query)
	}

	if report.Report.GeneratedAt.IsZero() {
		t.Error("Report.GeneratedAt should not be zero")
	}

	if len(report.Entities) != 0 {
		t.Errorf("Entities should be empty initially, got %d", len(report.Entities))
	}

	if len(report.Dependencies) != 0 {
		t.Errorf("Dependencies should be empty initially, got %d", len(report.Dependencies))
	}

	if report.Diagrams == nil {
		t.Error("Diagrams should be initialized to empty map, not nil")
	}

	if len(report.Diagrams) != 0 {
		t.Errorf("Diagrams should be empty initially, got %d", len(report.Diagrams))
	}
}

func TestFeatureReportData_YAML_Marshaling(t *testing.T) {
	report := &FeatureReportData{
		Report: ReportHeader{
			Type:        ReportTypeFeature,
			GeneratedAt: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC),
			Query:       "authentication",
		},
		Metadata: MetadataData{
			TotalEntitiesSearched: 3648,
			MatchesFound:          15,
			SearchMethod:          "hybrid",
			EntityCount:           15,
		},
		Entities: []EntityData{
			{
				ID:             "sa-fn-abc123-45-LoginUser",
				Name:           "LoginUser",
				Type:           "function",
				File:           "internal/auth/login.go",
				Lines:          [2]int{45, 89},
				Signature:      "func LoginUser(ctx context.Context, email, password string) (*User, error)",
				Importance:     ImportanceKeystone,
				PageRank:       0.0234,
				Coverage:       85.5,
				DocComment:     "LoginUser authenticates a user by email and password",
				RelevanceScore: 0.95,
			},
		},
		Dependencies: []DependencyData{
			{
				From: "LoginUser",
				To:   "ValidateToken",
				Type: DepTypeCalls,
			},
		},
		Diagrams: map[string]DiagramData{
			"call_flow": {
				Title: "Authentication Call Flow",
				D2: `direction: down
1.Request: {
  shape: oval
  label: "HTTP Request"
}
1.Request -> 2.Handler`,
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(data)

	// Verify expected structure
	expectedFields := []string{
		"report:",
		"type: feature",
		"query: authentication",
		"metadata:",
		"entities:",
		"dependencies:",
		"diagrams:",
		"entity_type: function",
		"importance: keystone",
		"type: calls",
	}

	for _, field := range expectedFields {
		if !contains(yamlStr, field) {
			t.Errorf("YAML output missing expected field %q\nGot:\n%s", field, yamlStr)
		}
	}

	// Unmarshal back
	var decoded FeatureReportData
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.Report.Type != ReportTypeFeature {
		t.Errorf("Decoded Type = %v, want %v", decoded.Report.Type, ReportTypeFeature)
	}

	if decoded.Report.Query != "authentication" {
		t.Errorf("Decoded Query = %v, want %v", decoded.Report.Query, "authentication")
	}

	if decoded.Metadata.MatchesFound != 15 {
		t.Errorf("Decoded MatchesFound = %v, want 15", decoded.Metadata.MatchesFound)
	}

	if len(decoded.Entities) != 1 {
		t.Fatalf("Decoded Entities length = %d, want 1", len(decoded.Entities))
	}

	if decoded.Entities[0].Name != "LoginUser" {
		t.Errorf("Decoded Entity Name = %v, want LoginUser", decoded.Entities[0].Name)
	}

	if len(decoded.Dependencies) != 1 {
		t.Fatalf("Decoded Dependencies length = %d, want 1", len(decoded.Dependencies))
	}

	if decoded.Dependencies[0].From != "LoginUser" {
		t.Errorf("Decoded Dependency From = %v, want LoginUser", decoded.Dependencies[0].From)
	}

	if len(decoded.Diagrams) != 1 {
		t.Fatalf("Decoded Diagrams length = %d, want 1", len(decoded.Diagrams))
	}

	if diagram, ok := decoded.Diagrams["call_flow"]; ok {
		if diagram.Title != "Authentication Call Flow" {
			t.Errorf("Decoded Diagram Title = %v, want Authentication Call Flow", diagram.Title)
		}
	} else {
		t.Error("Decoded Diagrams missing 'call_flow' key")
	}
}

func TestFeatureReportData_JSON_Marshaling(t *testing.T) {
	report := &FeatureReportData{
		Report: ReportHeader{
			Type:        ReportTypeFeature,
			GeneratedAt: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC),
			Query:       "payment processing",
		},
		Metadata: MetadataData{
			TotalEntitiesSearched: 5000,
			MatchesFound:          23,
			SearchMethod:          "fts",
		},
		Entities: []EntityData{
			{
				ID:         "sa-fn-def456-12-ProcessPayment",
				Name:       "ProcessPayment",
				Type:       "function",
				File:       "internal/payment/process.go",
				Lines:      [2]int{10, 95},
				Importance: ImportanceKeystone,
				Coverage:   92.0,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonStr := string(data)

	// Verify expected structure
	expectedFields := []string{
		`"type":"feature"`,
		`"query":"payment processing"`,
		`"total_entities_searched":5000`,
		`"matches_found":23`,
		`"entities"`,
		`"dependencies"`,
		`"diagrams"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON output missing expected field %q\nGot:\n%s", field, jsonStr)
		}
	}

	// Unmarshal back
	var decoded FeatureReportData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Report.Type != ReportTypeFeature {
		t.Errorf("Decoded Type = %v, want %v", decoded.Report.Type, ReportTypeFeature)
	}

	if decoded.Report.Query != "payment processing" {
		t.Errorf("Decoded Query = %v, want %v", decoded.Report.Query, "payment processing")
	}

	if len(decoded.Entities) != 1 {
		t.Fatalf("Decoded Entities length = %d, want 1", len(decoded.Entities))
	}

	if decoded.Entities[0].Name != "ProcessPayment" {
		t.Errorf("Decoded Entity Name = %v, want ProcessPayment", decoded.Entities[0].Name)
	}
}

func TestFeatureReportData_EntityCount(t *testing.T) {
	report := NewFeatureReport("test")
	report.Entities = []EntityData{
		{ID: "1", Name: "Entity1"},
		{ID: "2", Name: "Entity2"},
		{ID: "3", Name: "Entity3"},
	}

	if count := report.EntityCount(); count != 3 {
		t.Errorf("EntityCount() = %d, want 3", count)
	}
}

func TestFeatureReportData_DependencyCount(t *testing.T) {
	report := NewFeatureReport("test")
	report.Dependencies = []DependencyData{
		{From: "A", To: "B", Type: DepTypeCalls},
		{From: "B", To: "C", Type: DepTypeCalls},
	}

	if count := report.DependencyCount(); count != 2 {
		t.Errorf("DependencyCount() = %d, want 2", count)
	}
}

func TestFeatureReportData_DiagramCount(t *testing.T) {
	report := NewFeatureReport("test")
	report.Diagrams["diagram1"] = DiagramData{Title: "Diagram 1"}
	report.Diagrams["diagram2"] = DiagramData{Title: "Diagram 2"}

	if count := report.DiagramCount(); count != 2 {
		t.Errorf("DiagramCount() = %d, want 2", count)
	}
}

func TestFeatureReportData_TestCount(t *testing.T) {
	report := NewFeatureReport("test")
	report.Tests = []TestData{
		{Name: "Test1", File: "test1.go"},
		{Name: "Test2", File: "test2.go"},
		{Name: "Test3", File: "test3.go"},
	}

	if count := report.TestCount(); count != 3 {
		t.Errorf("TestCount() = %d, want 3", count)
	}
}

func TestFeatureReportData_GetDiagram(t *testing.T) {
	report := NewFeatureReport("test")
	diagram := DiagramData{Title: "Test Diagram"}
	report.Diagrams["test"] = diagram

	got, ok := report.GetDiagram("test")
	if !ok {
		t.Error("GetDiagram() should return true for existing diagram")
	}
	if got.Title != diagram.Title {
		t.Errorf("GetDiagram() returned title %q, want %q", got.Title, diagram.Title)
	}

	_, ok = report.GetDiagram("nonexistent")
	if ok {
		t.Error("GetDiagram() should return false for nonexistent diagram")
	}
}

func TestFeatureReportData_AddDiagram(t *testing.T) {
	report := NewFeatureReport("test")

	diagram1 := DiagramData{Title: "Diagram 1"}
	report.AddDiagram("diagram1", diagram1)

	if len(report.Diagrams) != 1 {
		t.Errorf("After AddDiagram(), count = %d, want 1", len(report.Diagrams))
	}

	if got, ok := report.GetDiagram("diagram1"); !ok || got.Title != "Diagram 1" {
		t.Error("AddDiagram() did not add diagram correctly")
	}

	// Test replacement
	diagram2 := DiagramData{Title: "Diagram 1 Updated"}
	report.AddDiagram("diagram1", diagram2)

	if len(report.Diagrams) != 1 {
		t.Errorf("After replacing diagram, count = %d, want 1", len(report.Diagrams))
	}

	if got, ok := report.GetDiagram("diagram1"); !ok || got.Title != "Diagram 1 Updated" {
		t.Error("AddDiagram() did not replace diagram correctly")
	}
}

func TestFeatureReportData_SetCoverage_HasCoverage(t *testing.T) {
	report := NewFeatureReport("test")

	if report.HasCoverage() {
		t.Error("HasCoverage() should return false initially")
	}

	coverage := &CoverageData{Overall: 85.5}
	report.SetCoverage(coverage)

	if !report.HasCoverage() {
		t.Error("HasCoverage() should return true after SetCoverage()")
	}

	if report.Coverage != coverage {
		t.Error("SetCoverage() did not set Coverage field correctly")
	}
}

func TestFeatureReportData_GetOverallCoverage(t *testing.T) {
	report := NewFeatureReport("test")

	// No coverage
	if cov := report.GetOverallCoverage(); cov != -1 {
		t.Errorf("GetOverallCoverage() with no coverage = %f, want -1", cov)
	}

	// With coverage
	coverage := &CoverageData{Overall: 78.5}
	report.SetCoverage(coverage)

	if cov := report.GetOverallCoverage(); cov != 78.5 {
		t.Errorf("GetOverallCoverage() = %f, want 78.5", cov)
	}
}

func TestFeatureReportData_GetKeystoneEntities(t *testing.T) {
	report := NewFeatureReport("test")
	report.Entities = []EntityData{
		{Name: "Entity1", Importance: ImportanceKeystone},
		{Name: "Entity2", Importance: ImportanceBottleneck},
		{Name: "Entity3", Importance: ImportanceKeystone},
		{Name: "Entity4", Importance: ImportanceNormal},
	}

	keystones := report.GetKeystoneEntities()
	if len(keystones) != 2 {
		t.Errorf("GetKeystoneEntities() returned %d entities, want 2", len(keystones))
	}

	for _, e := range keystones {
		if e.Importance != ImportanceKeystone {
			t.Errorf("GetKeystoneEntities() returned entity with importance %v", e.Importance)
		}
	}
}

func TestFeatureReportData_GetBottleneckEntities(t *testing.T) {
	report := NewFeatureReport("test")
	report.Entities = []EntityData{
		{Name: "Entity1", Importance: ImportanceKeystone},
		{Name: "Entity2", Importance: ImportanceBottleneck},
		{Name: "Entity3", Importance: ImportanceBottleneck},
		{Name: "Entity4", Importance: ImportanceLeaf},
	}

	bottlenecks := report.GetBottleneckEntities()
	if len(bottlenecks) != 2 {
		t.Errorf("GetBottleneckEntities() returned %d entities, want 2", len(bottlenecks))
	}

	for _, e := range bottlenecks {
		if e.Importance != ImportanceBottleneck {
			t.Errorf("GetBottleneckEntities() returned entity with importance %v", e.Importance)
		}
	}
}

func TestFeatureReportData_GetLowCoverageEntities(t *testing.T) {
	report := NewFeatureReport("test")
	report.Entities = []EntityData{
		{Name: "Entity1", Coverage: 95.0},
		{Name: "Entity2", Coverage: 50.0},
		{Name: "Entity3", Coverage: 75.0},
		{Name: "Entity4", Coverage: -1}, // No coverage data
	}

	low := report.GetLowCoverageEntities(80.0)
	if len(low) != 2 {
		t.Errorf("GetLowCoverageEntities(80.0) returned %d entities, want 2", len(low))
	}

	for _, e := range low {
		if e.Coverage >= 80.0 {
			t.Errorf("GetLowCoverageEntities() returned entity with coverage %f", e.Coverage)
		}
	}
}

func TestFeatureReportData_GetEntitiesByFile(t *testing.T) {
	report := NewFeatureReport("test")
	report.Entities = []EntityData{
		{Name: "Entity1", File: "auth.go"},
		{Name: "Entity2", File: "auth.go"},
		{Name: "Entity3", File: "store.go"},
		{Name: "Entity4", File: "cache.go"},
	}

	byFile := report.GetEntitiesByFile()
	if len(byFile) != 3 {
		t.Errorf("GetEntitiesByFile() returned %d files, want 3", len(byFile))
	}

	if len(byFile["auth.go"]) != 2 {
		t.Errorf("GetEntitiesByFile()[auth.go] returned %d entities, want 2", len(byFile["auth.go"]))
	}

	if len(byFile["store.go"]) != 1 {
		t.Errorf("GetEntitiesByFile()[store.go] returned %d entities, want 1", len(byFile["store.go"]))
	}
}

func TestFeatureReportData_GetDependenciesForEntity(t *testing.T) {
	report := NewFeatureReport("test")
	report.Dependencies = []DependencyData{
		{From: "LoginUser", To: "ValidateToken", Type: DepTypeCalls},
		{From: "LoginUser", To: "UserStore.GetByEmail", Type: DepTypeCalls},
		{From: "ValidateToken", To: "SessionCache.Get", Type: DepTypeCalls},
		{From: "OtherFunc", To: "ValidateToken", Type: DepTypeCalls},
	}

	deps := report.GetDependenciesForEntity("LoginUser")
	if len(deps) != 2 {
		t.Errorf("GetDependenciesForEntity('LoginUser') returned %d deps, want 2", len(deps))
	}

	for _, d := range deps {
		if d.From != "LoginUser" {
			t.Errorf("GetDependenciesForEntity() returned dependency from %q", d.From)
		}
	}
}

func TestFeatureReportData_GetDependentsOfEntity(t *testing.T) {
	report := NewFeatureReport("test")
	report.Dependencies = []DependencyData{
		{From: "LoginUser", To: "ValidateToken", Type: DepTypeCalls},
		{From: "CheckAuth", To: "ValidateToken", Type: DepTypeCalls},
		{From: "ValidateToken", To: "SessionCache.Get", Type: DepTypeCalls},
		{From: "OtherFunc", To: "SomethingElse", Type: DepTypeCalls},
	}

	dependents := report.GetDependentsOfEntity("ValidateToken")
	if len(dependents) != 2 {
		t.Errorf("GetDependentsOfEntity('ValidateToken') returned %d dependents, want 2", len(dependents))
	}

	for _, d := range dependents {
		if d.To != "ValidateToken" {
			t.Errorf("GetDependentsOfEntity() returned dependency to %q", d.To)
		}
	}
}

func TestFeatureReportData_OptionalFields(t *testing.T) {
	// Test that optional fields (Tests, Coverage) are omitted when empty/nil
	report := &FeatureReportData{
		Report: ReportHeader{
			Type:        ReportTypeFeature,
			GeneratedAt: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC),
			Query:       "test",
		},
		Metadata: MetadataData{},
		Entities: []EntityData{},
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(yamlData)
	if contains(yamlStr, "tests:") {
		t.Errorf("YAML should omit empty tests field:\n%s", yamlStr)
	}
	if contains(yamlStr, "coverage:") {
		t.Errorf("YAML should omit nil coverage field:\n%s", yamlStr)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonStr := string(jsonData)
	if contains(jsonStr, `"tests"`) {
		t.Errorf("JSON should omit empty tests field:\n%s", jsonStr)
	}
	if contains(jsonStr, `"coverage"`) {
		t.Errorf("JSON should omit nil coverage field:\n%s", jsonStr)
	}
}

func TestFeatureReportData_CompleteReport(t *testing.T) {
	// Test a complete report with all fields populated
	report := NewFeatureReport("authentication")
	report.Metadata.TotalEntitiesSearched = 3648
	report.Metadata.MatchesFound = 15
	report.Metadata.SearchMethod = "hybrid"
	report.Metadata.EntityCount = 15

	report.Entities = []EntityData{
		{
			ID:             "sa-fn-abc123-45-LoginUser",
			Name:           "LoginUser",
			Type:           "function",
			File:           "internal/auth/login.go",
			Lines:          [2]int{45, 89},
			Importance:     ImportanceKeystone,
			PageRank:       0.0234,
			Coverage:       85.5,
			RelevanceScore: 0.95,
		},
		{
			ID:             "sa-fn-def456-12-ValidateToken",
			Name:           "ValidateToken",
			Type:           "function",
			File:           "internal/auth/token.go",
			Lines:          [2]int{12, 45},
			Importance:     ImportanceBottleneck,
			PageRank:       0.0189,
			Coverage:       92.0,
			RelevanceScore: 0.88,
		},
	}

	report.Dependencies = []DependencyData{
		{From: "LoginUser", To: "ValidateToken", Type: DepTypeCalls},
		{From: "LoginUser", To: "UserStore.GetByEmail", Type: DepTypeCalls},
	}

	report.Diagrams["call_flow"] = DiagramData{
		Title: "Authentication Call Flow",
		D2:    "direction: down\n1.Request -> 2.Handler",
	}

	report.Tests = []TestData{
		{Name: "TestLoginUser_Success", File: "internal/auth/login_test.go", Lines: [2]int{15, 45}},
		{Name: "TestLoginUser_InvalidPassword", File: "internal/auth/login_test.go", Lines: [2]int{47, 78}},
	}

	report.SetCoverage(&CoverageData{
		Overall: 78.5,
		ByEntity: map[string]float64{
			"LoginUser":     85.5,
			"ValidateToken": 92.0,
		},
	})

	// Marshal to YAML and back
	yamlData, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded FeatureReportData
	if err := yaml.Unmarshal(yamlData, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	// Verify all data is preserved
	if decoded.EntityCount() != 2 {
		t.Errorf("After round-trip, EntityCount = %d, want 2", decoded.EntityCount())
	}
	if decoded.DependencyCount() != 2 {
		t.Errorf("After round-trip, DependencyCount = %d, want 2", decoded.DependencyCount())
	}
	if decoded.DiagramCount() != 1 {
		t.Errorf("After round-trip, DiagramCount = %d, want 1", decoded.DiagramCount())
	}
	if decoded.TestCount() != 2 {
		t.Errorf("After round-trip, TestCount = %d, want 2", decoded.TestCount())
	}
	if !decoded.HasCoverage() {
		t.Error("After round-trip, HasCoverage should be true")
	}
	if decoded.GetOverallCoverage() != 78.5 {
		t.Errorf("After round-trip, GetOverallCoverage = %f, want 78.5", decoded.GetOverallCoverage())
	}
}

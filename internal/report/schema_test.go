package report

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestReportType_String(t *testing.T) {
	tests := []struct {
		rt   ReportType
		want string
	}{
		{ReportTypeFeature, "feature"},
		{ReportTypeOverview, "overview"},
		{ReportTypeChanges, "changes"},
		{ReportTypeHealth, "health"},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			if got := tt.rt.String(); got != tt.want {
				t.Errorf("ReportType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseReportType(t *testing.T) {
	tests := []struct {
		input   string
		want    ReportType
		wantErr bool
	}{
		{"feature", ReportTypeFeature, false},
		{"Feature", ReportTypeFeature, false},
		{"FEATURE", ReportTypeFeature, false},
		{"  feature  ", ReportTypeFeature, false},
		{"overview", ReportTypeOverview, false},
		{"changes", ReportTypeChanges, false},
		{"health", ReportTypeHealth, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseReportType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReportType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseReportType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateReportType(t *testing.T) {
	tests := []struct {
		rt   ReportType
		want bool
	}{
		{ReportTypeFeature, true},
		{ReportTypeOverview, true},
		{ReportTypeChanges, true},
		{ReportTypeHealth, true},
		{ReportType("invalid"), false},
		{ReportType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			if got := ValidateReportType(tt.rt); got != tt.want {
				t.Errorf("ValidateReportType(%v) = %v, want %v", tt.rt, got, tt.want)
			}
		})
	}
}

func TestReportHeader_YAML(t *testing.T) {
	header := ReportHeader{
		Type:        ReportTypeFeature,
		GeneratedAt: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC),
		Query:       "authentication",
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&header)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	// Verify expected fields are present
	yamlStr := string(data)
	expectedFields := []string{
		"type: feature",
		"generated_at:",
		"query: authentication",
	}
	for _, field := range expectedFields {
		if !contains(yamlStr, field) {
			t.Errorf("YAML output missing expected field %q\nGot:\n%s", field, yamlStr)
		}
	}

	// Unmarshal back
	var decoded ReportHeader
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.Type != header.Type {
		t.Errorf("Decoded Type = %v, want %v", decoded.Type, header.Type)
	}
	if decoded.Query != header.Query {
		t.Errorf("Decoded Query = %v, want %v", decoded.Query, header.Query)
	}
}

func TestReportHeader_JSON(t *testing.T) {
	header := ReportHeader{
		Type:        ReportTypeChanges,
		GeneratedAt: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC),
		FromRef:     "HEAD~50",
		ToRef:       "HEAD",
	}

	// Marshal to JSON
	data, err := json.Marshal(&header)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify expected fields are present
	jsonStr := string(data)
	expectedFields := []string{
		`"type":"changes"`,
		`"from_ref":"HEAD~50"`,
		`"to_ref":"HEAD"`,
	}
	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON output missing expected field %q\nGot:\n%s", field, jsonStr)
		}
	}

	// Unmarshal back
	var decoded ReportHeader
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Type != header.Type {
		t.Errorf("Decoded Type = %v, want %v", decoded.Type, header.Type)
	}
	if decoded.FromRef != header.FromRef {
		t.Errorf("Decoded FromRef = %v, want %v", decoded.FromRef, header.FromRef)
	}
}

func TestEntityData_YAML(t *testing.T) {
	entity := EntityData{
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
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&entity)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	// Verify entity_type field name (not "type")
	yamlStr := string(data)
	if !contains(yamlStr, "entity_type: function") {
		t.Errorf("YAML should use 'entity_type' field name, got:\n%s", yamlStr)
	}
	if !contains(yamlStr, "importance: keystone") {
		t.Errorf("YAML should contain importance field, got:\n%s", yamlStr)
	}

	// Unmarshal back
	var decoded EntityData
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.ID != entity.ID {
		t.Errorf("Decoded ID = %v, want %v", decoded.ID, entity.ID)
	}
	if decoded.Lines != entity.Lines {
		t.Errorf("Decoded Lines = %v, want %v", decoded.Lines, entity.Lines)
	}
	if decoded.Importance != entity.Importance {
		t.Errorf("Decoded Importance = %v, want %v", decoded.Importance, entity.Importance)
	}
}

func TestEntityData_JSON(t *testing.T) {
	entity := EntityData{
		ID:         "sa-fn-abc123-45-LoginUser",
		Name:       "LoginUser",
		Type:       "function",
		File:       "internal/auth/login.go",
		Lines:      [2]int{45, 89},
		Importance: ImportanceBottleneck,
	}

	// Marshal to JSON
	data, err := json.Marshal(&entity)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify entity_type field name
	jsonStr := string(data)
	if !contains(jsonStr, `"entity_type":"function"`) {
		t.Errorf("JSON should use 'entity_type' field name, got:\n%s", jsonStr)
	}

	// Unmarshal back
	var decoded EntityData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Type != entity.Type {
		t.Errorf("Decoded Type = %v, want %v", decoded.Type, entity.Type)
	}
}

func TestDependencyData_Marshal(t *testing.T) {
	dep := DependencyData{
		From: "LoginUser",
		To:   "ValidateToken",
		Type: DepTypeCalls,
	}

	// Test YAML
	yamlData, err := yaml.Marshal(&dep)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if !contains(string(yamlData), "type: calls") {
		t.Errorf("YAML missing type field: %s", yamlData)
	}

	// Test JSON
	jsonData, err := json.Marshal(&dep)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !contains(string(jsonData), `"type":"calls"`) {
		t.Errorf("JSON missing type field: %s", jsonData)
	}
}

func TestDiagramData_Marshal(t *testing.T) {
	diagram := DiagramData{
		Title: "Authentication Call Flow",
		D2: `direction: down
1.Request: {
  shape: oval
  label: "HTTP Request"
}
2.Handler: {
  label: "LoginHandler"
}
1.Request -> 2.Handler`,
	}

	// Test YAML preserves multiline
	yamlData, err := yaml.Marshal(&diagram)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded DiagramData
	if err := yaml.Unmarshal(yamlData, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.Title != diagram.Title {
		t.Errorf("Decoded Title = %v, want %v", decoded.Title, diagram.Title)
	}
	if decoded.D2 != diagram.D2 {
		t.Errorf("Decoded D2 content mismatch\nGot:\n%s\nWant:\n%s", decoded.D2, diagram.D2)
	}
}

func TestMetadataData_Marshal(t *testing.T) {
	meta := MetadataData{
		EntityCount: 1234,
		LanguageBreakdown: map[string]int{
			"go":         800,
			"typescript": 400,
		},
		CoverageAvailable: true,
		SearchMethod:      "hybrid",
	}

	// Test YAML
	yamlData, err := yaml.Marshal(&meta)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded MetadataData
	if err := yaml.Unmarshal(yamlData, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.EntityCount != meta.EntityCount {
		t.Errorf("Decoded EntityCount = %v, want %v", decoded.EntityCount, meta.EntityCount)
	}
	if decoded.LanguageBreakdown["go"] != 800 {
		t.Errorf("Decoded LanguageBreakdown[go] = %v, want 800", decoded.LanguageBreakdown["go"])
	}

	// Test JSON
	jsonData, err := json.Marshal(&meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decodedJSON MetadataData
	if err := json.Unmarshal(jsonData, &decodedJSON); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decodedJSON.CoverageAvailable != meta.CoverageAvailable {
		t.Errorf("Decoded CoverageAvailable = %v, want %v", decodedJSON.CoverageAvailable, meta.CoverageAvailable)
	}
}

func TestCoverageData_Marshal(t *testing.T) {
	coverage := CoverageData{
		Overall: 78.5,
		ByEntity: map[string]float64{
			"LoginUser":     85.5,
			"ValidateToken": 92.0,
		},
		Gaps: []CoverageGap{
			{
				Entity:     "SessionCache.Get",
				Coverage:   45.0,
				Importance: ImportanceBottleneck,
				Risk:       RiskHigh,
			},
		},
	}

	// Test YAML
	yamlData, err := yaml.Marshal(&coverage)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded CoverageData
	if err := yaml.Unmarshal(yamlData, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.Overall != coverage.Overall {
		t.Errorf("Decoded Overall = %v, want %v", decoded.Overall, coverage.Overall)
	}
	if len(decoded.Gaps) != 1 {
		t.Fatalf("Decoded Gaps length = %v, want 1", len(decoded.Gaps))
	}
	if decoded.Gaps[0].Risk != RiskHigh {
		t.Errorf("Decoded Gaps[0].Risk = %v, want %v", decoded.Gaps[0].Risk, RiskHigh)
	}
}

func TestTestData_Marshal(t *testing.T) {
	testData := TestData{
		Name:   "TestLoginUser_Success",
		File:   "internal/auth/login_test.go",
		Lines:  [2]int{15, 45},
		Covers: []string{"LoginUser", "ValidateCredentials"},
	}

	// Test YAML
	yamlData, err := yaml.Marshal(&testData)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded TestData
	if err := yaml.Unmarshal(yamlData, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if decoded.Name != testData.Name {
		t.Errorf("Decoded Name = %v, want %v", decoded.Name, testData.Name)
	}
	if len(decoded.Covers) != 2 {
		t.Errorf("Decoded Covers length = %v, want 2", len(decoded.Covers))
	}
}

func TestHealthIssues_Marshal(t *testing.T) {
	issues := HealthIssues{
		Critical: []HealthIssue{
			{
				Type:           IssueTypeUntestedKeystone,
				Entity:         "CriticalFunction",
				File:           "internal/core/critical.go",
				PageRank:       0.0456,
				Coverage:       0.0,
				Recommendation: "Add tests for this high-importance function",
			},
		},
		Warning: []HealthIssue{
			{
				Type:           IssueTypeCircularDependency,
				Entities:       []string{"A", "B", "C"},
				Cycle:          "A -> B -> C -> A",
				Recommendation: "Break cycle by extracting shared interface",
			},
		},
	}

	// Test YAML
	yamlData, err := yaml.Marshal(&issues)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var decoded HealthIssues
	if err := yaml.Unmarshal(yamlData, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if len(decoded.Critical) != 1 {
		t.Fatalf("Decoded Critical length = %v, want 1", len(decoded.Critical))
	}
	if decoded.Critical[0].Type != IssueTypeUntestedKeystone {
		t.Errorf("Decoded Critical[0].Type = %v, want %v", decoded.Critical[0].Type, IssueTypeUntestedKeystone)
	}
	if len(decoded.Warning) != 1 {
		t.Fatalf("Decoded Warning length = %v, want 1", len(decoded.Warning))
	}
	if decoded.Warning[0].Cycle != "A -> B -> C -> A" {
		t.Errorf("Decoded Warning[0].Cycle = %v, want 'A -> B -> C -> A'", decoded.Warning[0].Cycle)
	}
}

func TestChangedEntity_Marshal(t *testing.T) {
	// Test added entity
	added := ChangedEntity{
		ID:      "sa-fn-new123",
		Name:    "GenerateReport",
		Type:    "function",
		File:    "internal/report/generate.go",
		Lines:   [2]int{1, 45},
		AddedIn: "abc1234",
	}

	yamlData, err := yaml.Marshal(&added)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if !contains(string(yamlData), "added_in: abc1234") {
		t.Errorf("YAML missing added_in field: %s", yamlData)
	}

	// Test deleted entity
	deleted := ChangedEntity{
		ID:        "sa-fn-del789",
		Name:      "OldSearch",
		Type:      "function",
		WasFile:   "internal/store/search_old.go",
		DeletedIn: "def5678",
	}

	yamlData, err = yaml.Marshal(&deleted)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if !contains(string(yamlData), "was_file:") {
		t.Errorf("YAML missing was_file field: %s", yamlData)
	}
	if !contains(string(yamlData), "deleted_in: def5678") {
		t.Errorf("YAML missing deleted_in field: %s", yamlData)
	}
}

func TestImportance_String(t *testing.T) {
	tests := []struct {
		i    Importance
		want string
	}{
		{ImportanceKeystone, "keystone"},
		{ImportanceBottleneck, "bottleneck"},
		{ImportanceNormal, "normal"},
		{ImportanceLeaf, "leaf"},
	}

	for _, tt := range tests {
		t.Run(string(tt.i), func(t *testing.T) {
			if got := tt.i.String(); got != tt.want {
				t.Errorf("Importance.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOmitEmptyFields(t *testing.T) {
	// Test that optional fields are omitted when empty
	header := ReportHeader{
		Type:        ReportTypeOverview,
		GeneratedAt: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC),
		// Query, FromRef, ToRef are empty
	}

	yamlData, err := yaml.Marshal(&header)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(yamlData)
	if contains(yamlStr, "query:") {
		t.Errorf("YAML should omit empty query field:\n%s", yamlStr)
	}
	if contains(yamlStr, "from_ref:") {
		t.Errorf("YAML should omit empty from_ref field:\n%s", yamlStr)
	}

	jsonData, err := json.Marshal(&header)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonStr := string(jsonData)
	if contains(jsonStr, "query") {
		t.Errorf("JSON should omit empty query field:\n%s", jsonStr)
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

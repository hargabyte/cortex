package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestEntityOutputYAML tests YAML marshaling of EntityOutput
func TestEntityOutputYAML(t *testing.T) {
	entity := &EntityOutput{
		Type:       "function",
		Location:   "internal/auth/login.go:45-89",
		Signature:  "(email: string, password: string) -> (*User, error)",
		Visibility: "public",
		Dependencies: &Dependencies{
			Calls:    []string{"ValidateEmail", "HashPassword"},
			CalledBy: []CalledByEntry{{Name: "HandleLogin"}, {Name: "HandleRegister"}},
		},
		Metrics: &Metrics{
			PageRank:   0.0234,
			InDegree:   5,
			OutDegree:  3,
			Importance: "keystone",
		},
	}

	data, err := yaml.Marshal(entity)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	// Verify we can unmarshal back
	var decoded EntityOutput
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	// Verify key fields
	if decoded.Type != "function" {
		t.Errorf("expected type=function, got %s", decoded.Type)
	}
	if decoded.Location != "internal/auth/login.go:45-89" {
		t.Errorf("expected location=internal/auth/login.go:45-89, got %s", decoded.Location)
	}
	if decoded.Metrics.InDegree != 5 {
		t.Errorf("expected in_degree=5, got %d", decoded.Metrics.InDegree)
	}
}

// TestEntityOutputJSON tests JSON marshaling of EntityOutput
func TestEntityOutputJSON(t *testing.T) {
	entity := &EntityOutput{
		Type:       "struct",
		Location:   "internal/user/service.go:12-45",
		Visibility: "public",
		Fields: map[string]string{
			"repo":    "UserRepository",
			"cache":   "Cache",
			"timeout": "time.Duration",
		},
		Methods: []string{"CreateUser", "GetUser", "UpdateUser", "DeleteUser"},
	}

	data, err := json.Marshal(entity)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	// Verify we can unmarshal back
	var decoded EntityOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify key fields
	if decoded.Type != "struct" {
		t.Errorf("expected type=struct, got %s", decoded.Type)
	}
	if len(decoded.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(decoded.Fields))
	}
	if len(decoded.Methods) != 4 {
		t.Errorf("expected 4 methods, got %d", len(decoded.Methods))
	}
}

// TestListOutputYAML tests YAML marshaling of ListOutput
func TestListOutputYAML(t *testing.T) {
	list := &ListOutput{
		Results: map[string]*EntityOutput{
			"LoginUser": {
				Type:      "function",
				Location:  "internal/auth/login.go:45-89",
				Signature: "(email: string, password: string) -> (*User, error)",
			},
			"UserService": {
				Type:     "struct",
				Location: "internal/user/service.go:12-45",
				Methods:  []string{"CreateUser", "GetUser"},
			},
		},
		Count: 2,
	}

	data, err := yaml.Marshal(list)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	// Verify we can unmarshal back
	var decoded ListOutput
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	// Verify structure
	if decoded.Count != 2 {
		t.Errorf("expected count=2, got %d", decoded.Count)
	}
	if len(decoded.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(decoded.Results))
	}
	if _, ok := decoded.Results["LoginUser"]; !ok {
		t.Errorf("expected LoginUser in results")
	}
}

// TestGraphOutputYAML tests YAML marshaling of GraphOutput
func TestGraphOutputYAML(t *testing.T) {
	graph := &GraphOutput{
		Graph: &GraphMetadata{
			Root:      "LoginUser",
			Direction: "both",
			Depth:     2,
		},
		Nodes: map[string]*GraphNode{
			"LoginUser": {
				Type:     "function",
				Location: "internal/auth/login.go:45-89",
				Depth:    0,
			},
			"ValidateEmail": {
				Type:     "function",
				Location: "internal/validation/email.go:10-25",
				Depth:    1,
			},
		},
		Edges: [][]string{
			{"LoginUser", "ValidateEmail", "calls"},
			{"HandleLogin", "LoginUser", "calls"},
		},
	}

	data, err := yaml.Marshal(graph)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	// Verify we can unmarshal back
	var decoded GraphOutput
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	// Verify structure
	if decoded.Graph.Root != "LoginUser" {
		t.Errorf("expected root=LoginUser, got %s", decoded.Graph.Root)
	}
	if len(decoded.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(decoded.Nodes))
	}
	if len(decoded.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(decoded.Edges))
	}
}

// TestImpactOutputYAML tests YAML marshaling of ImpactOutput
func TestImpactOutputYAML(t *testing.T) {
	impact := &ImpactOutput{
		Impact: &ImpactMetadata{
			Target: "internal/auth/login.go",
			Depth:  3,
		},
		Summary: &ImpactSummary{
			FilesAffected:    8,
			EntitiesAffected: 23,
			RiskLevel:        "medium",
		},
		Affected: map[string]*AffectedEntity{
			"LoginUser": {
				Type:     "function",
				Location: "internal/auth/login.go:45-89",
				Impact:   "direct",
				Reason:   "file was changed",
			},
			"HandleLogin": {
				Type:       "function",
				Location:   "internal/handlers/auth.go:50-80",
				Impact:     "caller",
				Importance: "keystone",
				Reason:     "calls changed entity",
			},
		},
		Recommendations: []string{
			"Review HandleLogin - keystone caller of changed code",
			"Update tests in internal/auth/login_test.go",
		},
	}

	data, err := yaml.Marshal(impact)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	// Verify we can unmarshal back
	var decoded ImpactOutput
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	// Verify structure
	if decoded.Summary.RiskLevel != "medium" {
		t.Errorf("expected risk_level=medium, got %s", decoded.Summary.RiskLevel)
	}
	if len(decoded.Affected) != 2 {
		t.Errorf("expected 2 affected entities, got %d", len(decoded.Affected))
	}
}

// TestContextOutputYAML tests YAML marshaling of ContextOutput
func TestContextOutputYAML(t *testing.T) {
	context := &ContextOutput{
		Context: &ContextMetadata{
			Target:     "add rate limiting to API",
			Budget:     8000,
			TokensUsed: 6234,
		},
		EntryPoints: map[string]*EntryPoint{
			"Router": {
				Type:     "function",
				Location: "internal/handlers/api.go:15-40",
				Note:     "24 HTTP endpoints registered",
			},
		},
		Relevant: map[string]*RelevantEntity{
			"AuthMiddleware": {
				Type:      "function",
				Location:  "internal/middleware/auth.go:20-60",
				Relevance: "high",
				Reason:    "Middleware chain - rate limiting typically added here",
				Code:      "func AuthMiddleware(next http.Handler) http.Handler { ... }",
			},
		},
		Excluded: map[string]string{
			"UserService": "Token budget - lower relevance (3 functions)",
		},
	}

	data, err := yaml.Marshal(context)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	// Verify we can unmarshal back
	var decoded ContextOutput
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	// Verify structure
	if decoded.Context.TokensUsed != 6234 {
		t.Errorf("expected tokens_used=6234, got %d", decoded.Context.TokensUsed)
	}
	if len(decoded.EntryPoints) != 1 {
		t.Errorf("expected 1 entry point, got %d", len(decoded.EntryPoints))
	}
	if len(decoded.Relevant) != 1 {
		t.Errorf("expected 1 relevant entity, got %d", len(decoded.Relevant))
	}
}

// TestSparseFormat tests sparse density output format
func TestSparseFormat(t *testing.T) {
	// In sparse mode, entities should be represented as simple one-liners
	// This is more of a formatting test - the schema supports it via omitempty tags
	entity := &EntityOutput{
		Type:     "function",
		Location: "internal/auth/login.go:45-89",
		// All other fields omitted (sparse mode)
	}

	data, err := yaml.Marshal(entity)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	// Verify the YAML is minimal
	yamlStr := string(data)
	if len(yamlStr) > 100 {
		t.Errorf("sparse format should be minimal, got %d bytes", len(yamlStr))
	}
}

// TestYAMLFormatterEntity tests YAML formatter for single entity output
func TestYAMLFormatterEntity(t *testing.T) {
	formatter := NewYAMLFormatter()
	entity := &EntityOutput{
		Type:       "function",
		Location:   "internal/auth/login.go:45-89",
		Signature:  "(email: string, password: string) -> (*User, error)",
		Visibility: "public",
		Dependencies: &Dependencies{
			Calls:    []string{"ValidateEmail", "HashPassword"},
			CalledBy: []CalledByEntry{{Name: "HandleLogin"}},
		},
	}

	// Test formatting
	yaml, err := formatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify YAML contains expected fields
	if !contains(yaml, "type:") {
		t.Error("YAML should contain 'type:' field")
	}
	if !contains(yaml, "location:") {
		t.Error("YAML should contain 'location:' field")
	}
	if !contains(yaml, "signature:") {
		t.Error("YAML should contain 'signature:' field")
	}
	if !contains(yaml, "calls:") {
		t.Error("YAML should contain 'calls:' field")
	}
}

// TestJSONFormatterEntity tests JSON formatter for single entity output
func TestJSONFormatterEntity(t *testing.T) {
	formatter := NewJSONFormatter()
	entity := &EntityOutput{
		Type:       "function",
		Location:   "internal/auth/login.go:45-89",
		Signature:  "(email: string, password: string) -> (*User, error)",
		Visibility: "public",
	}

	// Test formatting
	jsonStr, err := formatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify JSON can be parsed
	var decoded EntityOutput
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify fields
	if decoded.Type != "function" {
		t.Errorf("expected type=function, got %s", decoded.Type)
	}
	if decoded.Signature != "(email: string, password: string) -> (*User, error)" {
		t.Errorf("expected signature to match, got %s", decoded.Signature)
	}
}

// TestYAMLFormatterListOutput tests YAML formatter for list output
func TestYAMLFormatterListOutput(t *testing.T) {
	formatter := NewYAMLFormatter()
	list := &ListOutput{
		Results: map[string]*EntityOutput{
			"LoginUser": {
				Type:      "function",
				Location:  "internal/auth/login.go:45-89",
				Signature: "(email: string, password: string) -> (*User, error)",
			},
			"ValidateEmail": {
				Type:     "function",
				Location: "internal/validation/email.go:10-25",
			},
		},
		Count: 2,
	}

	yaml, err := formatter.Format(list, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify YAML structure
	if !contains(yaml, "results:") {
		t.Error("YAML should contain 'results:' field")
	}
	if !contains(yaml, "LoginUser") {
		t.Error("YAML should contain 'LoginUser' entry")
	}
	if !contains(yaml, "count:") {
		t.Error("YAML should contain 'count:' field")
	}
}

// TestCGFFormatterEntity tests CGF formatter for single entity output
func TestCGFFormatterEntity(t *testing.T) {
	formatter := NewCGFFormatter()
	entity := &EntityOutput{
		Type:       "function",
		Location:   "internal/auth/login.go:45-89",
		Signature:  "(email: string, password: string) -> (*User, error)",
		Visibility: "public",
		Dependencies: &Dependencies{
			Calls:    []string{"ValidateEmail", "HashPassword"},
			CalledBy: []CalledByEntry{{Name: "HandleLogin"}},
		},
	}

	// Test formatting (should emit deprecation warning)
	cgf, err := formatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify CGF header
	if !contains(cgf, "#cgf v1") {
		t.Error("CGF should contain header '#cgf v1'")
	}
	if !contains(cgf, "d=medium") {
		t.Error("CGF should contain density marker 'd=medium'")
	}
}

// TestYAMLFormatterGraphOutput tests YAML formatter for graph output
func TestYAMLFormatterGraphOutput(t *testing.T) {
	formatter := NewYAMLFormatter()
	graph := &GraphOutput{
		Graph: &GraphMetadata{
			Root:      "LoginUser",
			Direction: "both",
			Depth:     2,
		},
		Nodes: map[string]*GraphNode{
			"LoginUser": {
				Type:     "function",
				Location: "internal/auth/login.go:45-89",
				Depth:    0,
			},
			"ValidateEmail": {
				Type:     "function",
				Location: "internal/validation/email.go:10-25",
				Depth:    1,
			},
		},
		Edges: [][]string{
			{"LoginUser", "ValidateEmail", "calls"},
		},
	}

	yaml, err := formatter.Format(graph, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify YAML structure
	if !contains(yaml, "graph:") {
		t.Error("YAML should contain 'graph:' field")
	}
	if !contains(yaml, "nodes:") {
		t.Error("YAML should contain 'nodes:' field")
	}
	if !contains(yaml, "edges:") {
		t.Error("YAML should contain 'edges:' field")
	}
}

// TestYAMLFormatterImpactOutput tests YAML formatter for impact output
func TestYAMLFormatterImpactOutput(t *testing.T) {
	formatter := NewYAMLFormatter()
	impact := &ImpactOutput{
		Impact: &ImpactMetadata{
			Target: "internal/auth/login.go",
			Depth:  3,
		},
		Summary: &ImpactSummary{
			FilesAffected:    8,
			EntitiesAffected: 23,
			RiskLevel:        "medium",
		},
		Affected: map[string]*AffectedEntity{
			"LoginUser": {
				Type:     "function",
				Location: "internal/auth/login.go:45-89",
				Impact:   "direct",
				Reason:   "file was changed",
			},
		},
	}

	yaml, err := formatter.Format(impact, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify YAML structure
	if !contains(yaml, "impact:") {
		t.Error("YAML should contain 'impact:' field")
	}
	if !contains(yaml, "summary:") {
		t.Error("YAML should contain 'summary:' field")
	}
	if !contains(yaml, "affected:") {
		t.Error("YAML should contain 'affected:' field")
	}
}

// TestYAMLFormatterContextOutput tests YAML formatter for context output
func TestYAMLFormatterContextOutput(t *testing.T) {
	formatter := NewYAMLFormatter()
	ctx := &ContextOutput{
		Context: &ContextMetadata{
			Target:     "add rate limiting to API",
			Budget:     8000,
			TokensUsed: 6234,
		},
		EntryPoints: map[string]*EntryPoint{
			"Router": {
				Type:     "function",
				Location: "internal/handlers/api.go:15-40",
				Note:     "24 HTTP endpoints registered",
			},
		},
		Relevant: map[string]*RelevantEntity{
			"AuthMiddleware": {
				Type:      "function",
				Location:  "internal/middleware/auth.go:20-60",
				Relevance: "high",
				Reason:    "Middleware chain - rate limiting typically added here",
			},
		},
	}

	yaml, err := formatter.Format(ctx, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Verify YAML structure
	if !contains(yaml, "context:") {
		t.Error("YAML should contain 'context:' field")
	}
	if !contains(yaml, "entry_points:") {
		t.Error("YAML should contain 'entry_points:' field")
	}
	if !contains(yaml, "relevant:") {
		t.Error("YAML should contain 'relevant:' field")
	}
}

// TestDensityFiltering tests that different density levels are respected
func TestDensityFiltering(t *testing.T) {
	formatter := NewYAMLFormatter()
	entity := &EntityOutput{
		Type:       "function",
		Location:   "internal/auth/login.go:45-89",
		Signature:  "(email: string, password: string) -> (*User, error)",
		Visibility: "public",
		Dependencies: &Dependencies{
			Calls: []string{"ValidateEmail", "HashPassword"},
		},
		Metrics: &Metrics{
			PageRank:   0.85,
			InDegree:   5,
			OutDegree:  2,
			Importance: "keystone",
		},
		Hashes: &Hashes{
			Signature: "abcd1234",
			Body:      "efgh5678",
		},
	}

	// Test sparse mode - should include minimal fields
	sparseCGF, err := NewCGFFormatter().Format(entity, DensitySparse)
	if err != nil {
		t.Fatalf("failed to format sparse: %v", err)
	}
	if !contains(sparseCGF, "sparse") {
		t.Error("CGF sparse should include 'd=sparse'")
	}

	// Test dense mode - should include metrics
	denseYAML, err := formatter.Format(entity, DensityDense)
	if err != nil {
		t.Fatalf("failed to format dense: %v", err)
	}
	if !contains(denseYAML, "metrics:") {
		t.Error("Dense YAML should include metrics")
	}
}

// TestFormatterWithWriter tests FormatToWriter method
func TestFormatterWithWriter(t *testing.T) {
	formatter := NewYAMLFormatter()
	entity := &EntityOutput{
		Type:     "function",
		Location: "internal/auth/login.go:45-89",
	}

	var buf bytes.Buffer
	if err := formatter.FormatToWriter(&buf, entity, DensityMedium); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	yaml := buf.String()
	if yaml == "" {
		t.Error("FormatToWriter should produce output")
	}
	if !contains(yaml, "type:") {
		t.Error("Output should contain type field")
	}
}

// TestJSONFormatterWithWriter tests JSON FormatToWriter
func TestJSONFormatterWithWriter(t *testing.T) {
	formatter := NewJSONFormatter()
	entity := &EntityOutput{
		Type:     "struct",
		Location: "internal/user/model.go:10-30",
		Fields: map[string]string{
			"ID":   "string",
			"Name": "string",
		},
	}

	var buf bytes.Buffer
	if err := formatter.FormatToWriter(&buf, entity, DensityMedium); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	jsonStr := buf.String()
	var decoded EntityOutput
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Type != "struct" {
		t.Errorf("expected type=struct, got %s", decoded.Type)
	}
	if len(decoded.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(decoded.Fields))
	}
}

// TestCGFDeprecationWarning tests that CGF formatter warns about deprecation
func TestCGFDeprecationWarning(t *testing.T) {
	formatter := NewCGFFormatter()
	entity := &EntityOutput{
		Type:     "function",
		Location: "internal/auth/login.go:45-89",
	}

	// First call should emit warning
	_, err := formatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}

	// Second call should not warn again (warnedOnce=true)
	_, err = formatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format: %v", err)
	}
}

// TestUnsupportedTypeInCGFFormatter tests CGF formatter with unsupported type
func TestUnsupportedTypeInCGFFormatter(t *testing.T) {
	formatter := NewCGFFormatter()

	// Try to format an unsupported type
	_, err := formatter.Format("not a valid output type", DensityMedium)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

// TestYAMLJSONConsistency tests that YAML and JSON produce same data structure
func TestYAMLJSONConsistency(t *testing.T) {
	entity := &EntityOutput{
		Type:       "function",
		Location:   "internal/auth/login.go:45-89",
		Signature:  "(email: string, password: string) -> (*User, error)",
		Visibility: "public",
		Dependencies: &Dependencies{
			Calls:    []string{"ValidateEmail", "HashPassword"},
			CalledBy: []CalledByEntry{{Name: "HandleLogin", Location: "handlers/auth.go:50"}},
		},
	}

	yamlFormatter := NewYAMLFormatter()
	jsonFormatter := NewJSONFormatter()

	yamlStr, err := yamlFormatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format YAML: %v", err)
	}

	jsonStr, err := jsonFormatter.Format(entity, DensityMedium)
	if err != nil {
		t.Fatalf("failed to format JSON: %v", err)
	}

	// Parse both formats
	var yamlEntity, jsonEntity EntityOutput
	if err := yaml.Unmarshal([]byte(yamlStr), &yamlEntity); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}
	if err := json.Unmarshal([]byte(jsonStr), &jsonEntity); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify fields match
	if yamlEntity.Type != jsonEntity.Type {
		t.Errorf("type mismatch: YAML=%s, JSON=%s", yamlEntity.Type, jsonEntity.Type)
	}
	if yamlEntity.Signature != jsonEntity.Signature {
		t.Errorf("signature mismatch: YAML=%s, JSON=%s", yamlEntity.Signature, jsonEntity.Signature)
	}
	if len(yamlEntity.Dependencies.Calls) != len(jsonEntity.Dependencies.Calls) {
		t.Errorf("calls count mismatch")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

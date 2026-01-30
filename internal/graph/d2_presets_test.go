package graph

import (
	"strings"
	"testing"
)

func TestArchitecturePreset(t *testing.T) {
	cfg := ArchitecturePreset()

	if cfg.Type != DiagramArchitecture {
		t.Errorf("expected DiagramArchitecture, got %v", cfg.Type)
	}
	// ELK is used instead of TALA (ELK handles containers well and is bundled with D2)
	if cfg.Layout != "elk" {
		t.Errorf("expected elk layout, got %s", cfg.Layout)
	}
	if cfg.Direction != "right" {
		t.Errorf("expected right direction, got %s", cfg.Direction)
	}
	if cfg.MaxNodes != 50 {
		t.Errorf("expected MaxNodes 50, got %d", cfg.MaxNodes)
	}
	if !cfg.ShowLabels {
		t.Error("expected ShowLabels true")
	}
	if !cfg.ShowIcons {
		t.Error("expected ShowIcons true")
	}
	if !cfg.Collapse {
		t.Error("expected Collapse true")
	}
}

func TestCallFlowPreset(t *testing.T) {
	cfg := CallFlowPreset()

	if cfg.Type != DiagramCallFlow {
		t.Errorf("expected DiagramCallFlow, got %v", cfg.Type)
	}
	if cfg.Layout != "elk" {
		t.Errorf("expected elk layout, got %s", cfg.Layout)
	}
	if cfg.Direction != "down" {
		t.Errorf("expected down direction, got %s", cfg.Direction)
	}
	if cfg.Collapse {
		t.Error("expected Collapse false for call flow")
	}
}

func TestCoveragePreset(t *testing.T) {
	cfg := CoveragePreset()

	if cfg.Type != DiagramCoverage {
		t.Errorf("expected DiagramCoverage, got %v", cfg.Type)
	}
	if cfg.ShowLabels {
		t.Error("expected ShowLabels false for coverage")
	}
	if !cfg.ShowIcons {
		t.Error("expected ShowIcons true for status icons")
	}
}

func TestDependencyPreset(t *testing.T) {
	cfg := DependencyPreset()

	if cfg.Type != DiagramDeps {
		t.Errorf("expected DiagramDeps, got %v", cfg.Type)
	}
	if cfg.MaxNodes != 30 {
		t.Errorf("expected MaxNodes 30, got %d", cfg.MaxNodes)
	}
}

func TestGetPreset(t *testing.T) {
	tests := []struct {
		preset   DiagramPreset
		wantType DiagramType
	}{
		{PresetArchitecture, DiagramArchitecture},
		{PresetCallFlow, DiagramCallFlow},
		{PresetCoverage, DiagramCoverage},
		{PresetDependency, DiagramDeps},
		{"unknown", DiagramDeps}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			cfg := GetPreset(tt.preset)
			if cfg.Type != tt.wantType {
				t.Errorf("expected type %v, got %v", tt.wantType, cfg.Type)
			}
		})
	}
}

func TestExtractModuleFromPath(t *testing.T) {
	tests := []struct {
		filePath string
		want     string
	}{
		{"internal/store/entity.go", "internal/store"},
		{"cmd/main.go", "cmd"},
		{"main.go", "_root"},
		{"", "_root"},
		{"src/components/Button.tsx", "src/components"},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := extractModuleFromPath(tt.filePath)
			if got != tt.want {
				t.Errorf("extractModuleFromPath(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestInferLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		want     string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"script.js", "javascript"},
		{"script.jsx", "javascript"},
		{"main.py", "python"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"App.cs", "csharp"},
		{"index.php", "php"},
		{"app.rb", "ruby"},
		{"Main.kt", "kotlin"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := inferLanguage(tt.filePath)
			if got != tt.want {
				t.Errorf("inferLanguage(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestInferLayer(t *testing.T) {
	// Tests for Cortex-specific layer inference
	tests := []struct {
		entityType string
		module     string
		want       string
	}{
		// Parser layer
		{"function", "internal/parser", "parser"},
		{"function", "internal/extract", "parser"},
		{"function", "internal/resolve", "parser"},
		{"function", "internal/semdiff", "parser"},
		// Store layer
		{"function", "internal/store", "store"},
		{"function", "internal/cache", "store"},
		// Graph layer
		{"function", "internal/graph", "graph"},
		{"function", "internal/metrics", "graph"},
		{"function", "internal/embeddings", "graph"},
		// Output layer
		{"function", "internal/output", "output"},
		{"function", "internal/report", "output"},
		{"function", "internal/coverage", "output"},
		// API layer
		{"function", "internal/cmd", "api"},
		{"function", "internal/daemon", "api"},
		{"function", "internal/mcp", "api"},
		{"function", "internal/bd", "api"},
		// Core layer (default)
		{"function", "internal/config", "core"},
		{"function", "internal/context", "core"},
		{"function", "internal/diff", "core"},
		{"function", "internal/foo", "core"},
	}

	for _, tt := range tests {
		t.Run(tt.module+"_"+tt.entityType, func(t *testing.T) {
			got := inferLayer(tt.entityType, tt.module)
			if got != tt.want {
				t.Errorf("inferLayer(%q, %q) = %q, want %q", tt.entityType, tt.module, got, tt.want)
			}
		})
	}
}

func TestClassifyImportanceByRank(t *testing.T) {
	tests := []struct {
		pageRank float64
		want     string
	}{
		{0.02, "keystone"},
		{0.01, "keystone"},
		{0.008, "bottleneck"},
		{0.005, "bottleneck"},
		{0.003, "normal"},
		{0.001, "normal"},
		{0.0005, "leaf"},
		{0.0, "leaf"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := classifyImportanceByRank(tt.pageRank)
			if got != tt.want {
				t.Errorf("classifyImportanceByRank(%v) = %q, want %q", tt.pageRank, got, tt.want)
			}
		})
	}
}

func TestArchitecturePresetGeneratesDiagram(t *testing.T) {
	// Create entities with module grouping
	entities := []DiagramEntity{
		{ID: "fn1", Name: "HandleRequest", Type: "function", Module: "internal/api", Importance: "keystone"},
		{ID: "fn2", Name: "GetUser", Type: "function", Module: "internal/store", Importance: "bottleneck"},
		{ID: "fn3", Name: "CreateUser", Type: "function", Module: "internal/store", Importance: "normal"},
		{ID: "type1", Name: "User", Type: "struct", Module: "internal/model", Importance: "normal"},
	}

	deps := []DiagramEdge{
		{From: "fn1", To: "fn2", Type: "calls"},
		{From: "fn2", To: "type1", Type: "uses_type"},
	}

	cfg := ArchitecturePreset()
	cfg.Title = "Test Architecture"
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Check for module containers
	if !strings.Contains(result, "internal-api:") && !strings.Contains(result, `"internal/api":`) {
		t.Error("expected internal/api module container in output")
	}
	if !strings.Contains(result, "internal-store:") && !strings.Contains(result, `"internal/store":`) {
		t.Error("expected internal/store module container in output")
	}

	// Check for title
	if !strings.Contains(result, `label: "Test Architecture"`) {
		t.Error("expected title in output")
	}

	// Check for TALA layout (or ELK fallback)
	if !strings.Contains(result, "layout-engine: tala") && !strings.Contains(result, "layout-engine: elk") {
		t.Error("expected layout engine configuration")
	}

	// Check for connections section
	if !strings.Contains(result, "# Flow Connections") {
		t.Error("expected flow connections section")
	}
}

func TestCallFlowPresetGeneratesDiagram(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "fn1", Name: "Main", Type: "function"},
		{ID: "fn2", Name: "Process", Type: "function"},
		{ID: "fn3", Name: "Cleanup", Type: "function"},
	}

	deps := []DiagramEdge{
		{From: "fn1", To: "fn2", Type: "calls", Label: "process data"},
		{From: "fn2", To: "fn3", Type: "calls", Label: "cleanup"},
	}

	cfg := CallFlowPreset()
	cfg.Title = "Call Flow"
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Check for downward direction
	if !strings.Contains(result, "direction: down") {
		t.Error("expected down direction for call flow")
	}

	// Check for call flow section
	if !strings.Contains(result, "# Call Flow") {
		t.Error("expected call flow section")
	}

	// Check for flow section
	if !strings.Contains(result, "# Flow") {
		t.Error("expected flow section")
	}
}

func TestCoveragePresetGeneratesDiagram(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "fn1", Name: "WellTested", Type: "function", Coverage: 95, Importance: "keystone"},
		{ID: "fn2", Name: "PartiallyTested", Type: "function", Coverage: 60, Importance: "normal"},
		{ID: "fn3", Name: "Untested", Type: "function", Coverage: 0, Importance: "bottleneck"},
	}

	deps := []DiagramEdge{}

	cfg := CoveragePreset()
	cfg.Title = "Coverage Analysis"
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Check for coverage analysis section
	if !strings.Contains(result, "# Coverage Analysis") {
		t.Error("expected coverage analysis section")
	}

	// Check for legend
	if !strings.Contains(result, "Coverage Legend") {
		t.Error("expected coverage legend")
	}

	// Check for coverage percentage in labels
	if !strings.Contains(result, "coverage") {
		t.Error("expected coverage percentages in labels")
	}
}

func TestCallFlowPresetWithMultipleEntities(t *testing.T) {
	// Simulate a call flow: main -> process -> validate -> save
	entities := []DiagramEntity{
		{ID: "fn-main", Name: "Main", Type: "function", Importance: "keystone"},
		{ID: "fn-process", Name: "ProcessRequest", Type: "function", Importance: "normal"},
		{ID: "fn-validate", Name: "ValidateInput", Type: "function", Importance: "normal"},
		{ID: "fn-save", Name: "SaveToDatabase", Type: "function", Importance: "normal"},
	}

	deps := []DiagramEdge{
		{From: "fn-main", To: "fn-process", Type: "calls"},
		{From: "fn-process", To: "fn-validate", Type: "calls"},
		{From: "fn-process", To: "fn-save", Type: "calls"},
	}

	cfg := CallFlowPreset()
	cfg.Title = "Request Processing Flow"
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Verify direction is down for call flow
	if !strings.Contains(result, "direction: down") {
		t.Error("expected downward direction for call flow")
	}

	// Verify title is included
	if !strings.Contains(result, `label: "Request Processing Flow"`) {
		t.Error("expected title in output")
	}

	// Verify all entities are present
	for _, e := range entities {
		if !strings.Contains(result, e.Name) {
			t.Errorf("expected entity %s in output", e.Name)
		}
	}

	// Verify call flow edges use arrow syntax
	if !strings.Contains(result, "->") {
		t.Error("expected arrow edges in call flow")
	}

	// Verify layout is elk
	if !strings.Contains(result, "layout-engine: elk") {
		t.Error("expected ELK layout engine for call flow")
	}
}

func TestCallFlowEdgeStyling(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "fn1", Name: "Caller", Type: "function"},
		{ID: "fn2", Name: "Callee", Type: "function"},
	}

	deps := []DiagramEdge{
		{From: "fn1", To: "fn2", Type: "calls", Label: "invoke"},
	}

	cfg := CallFlowPreset()
	cfg.ShowLabels = true
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Verify edge has proper structure
	if !strings.Contains(result, "fn1 -> fn2") {
		t.Error("expected fn1 -> fn2 edge")
	}

	// Verify label is included
	if !strings.Contains(result, "invoke") {
		t.Error("expected label 'invoke' on edge")
	}
}

func TestCallFlowEntityOrdering(t *testing.T) {
	// Test that buildEntityOrder correctly orders entities based on dependencies
	entities := []DiagramEntity{
		{ID: "leaf1", Name: "Leaf1", Type: "function"},
		{ID: "root", Name: "Root", Type: "function"},
		{ID: "leaf2", Name: "Leaf2", Type: "function"},
		{ID: "middle", Name: "Middle", Type: "function"},
	}

	deps := []DiagramEdge{
		{From: "root", To: "middle", Type: "calls"},
		{From: "middle", To: "leaf1", Type: "calls"},
		{From: "middle", To: "leaf2", Type: "calls"},
	}

	ordered := buildEntityOrder(entities, deps)

	// Root should come before middle
	rootIdx, middleIdx := -1, -1
	for i, e := range ordered {
		if e.ID == "root" {
			rootIdx = i
		}
		if e.ID == "middle" {
			middleIdx = i
		}
	}

	if rootIdx == -1 || middleIdx == -1 {
		t.Error("expected both root and middle in ordered entities")
	}

	if rootIdx > middleIdx {
		t.Error("expected root to come before middle in call flow order")
	}
}

func TestCallFlowWithCycles(t *testing.T) {
	// Test handling of cyclic dependencies (e.g., A calls B, B calls A)
	entities := []DiagramEntity{
		{ID: "fn-a", Name: "FuncA", Type: "function"},
		{ID: "fn-b", Name: "FuncB", Type: "function"},
	}

	deps := []DiagramEdge{
		{From: "fn-a", To: "fn-b", Type: "calls"},
		{From: "fn-b", To: "fn-a", Type: "calls"},
	}

	cfg := CallFlowPreset()
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Should not panic and should include both entities
	if !strings.Contains(result, "FuncA") || !strings.Contains(result, "FuncB") {
		t.Error("expected both functions in output despite cycle")
	}

	// Both edges should be present
	if !strings.Contains(result, "fn-a -> fn-b") {
		t.Error("expected fn-a -> fn-b edge")
	}
	if !strings.Contains(result, "fn-b -> fn-a") {
		t.Error("expected fn-b -> fn-a edge")
	}
}

func TestCallFlowEmptyDeps(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "fn1", Name: "StandaloneFunc", Type: "function"},
	}

	deps := []DiagramEdge{}

	cfg := CallFlowPreset()
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Should generate valid output with single entity
	if !strings.Contains(result, "StandaloneFunc") {
		t.Error("expected standalone function in output")
	}

	// Should still have proper sections
	if !strings.Contains(result, "# Call Flow") {
		t.Error("expected call flow section")
	}
}

func TestCallFlowWithModuleInfo(t *testing.T) {
	// Call flow entities can have module info but it's not used for grouping
	entities := []DiagramEntity{
		{ID: "fn1", Name: "Handler", Type: "function", Module: "internal/api"},
		{ID: "fn2", Name: "Service", Type: "function", Module: "internal/service"},
		{ID: "fn3", Name: "Query", Type: "function", Module: "internal/store"},
	}

	deps := []DiagramEdge{
		{From: "fn1", To: "fn2", Type: "calls"},
		{From: "fn2", To: "fn3", Type: "calls"},
	}

	cfg := CallFlowPreset()
	gen := NewD2Generator(cfg)
	result := gen.Generate(entities, deps)

	// Call flow should NOT group by module (unlike architecture diagrams)
	// Entities should be listed directly, not in containers
	if strings.Contains(result, "internal-api:") || strings.Contains(result, `"internal/api":`) {
		t.Error("call flow should not group entities into module containers")
	}

	// All entities should be present at top level
	if !strings.Contains(result, "fn1:") {
		t.Error("expected fn1 entity at top level")
	}
}

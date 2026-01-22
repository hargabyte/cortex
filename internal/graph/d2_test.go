package graph

import (
	"strings"
	"testing"
)

func TestNewD2Generator(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		gen := NewD2Generator(nil)
		if gen.config == nil {
			t.Fatal("expected config to be set")
		}
		if gen.config.Type != DiagramDeps {
			t.Errorf("expected default type DiagramDeps, got %v", gen.config.Type)
		}
		if gen.config.Theme != "default" {
			t.Errorf("expected default theme, got %s", gen.config.Theme)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &DiagramConfig{
			Type:      DiagramArchitecture,
			Theme:     "dark",
			Direction: "down",
		}
		gen := NewD2Generator(cfg)
		if gen.config.Type != DiagramArchitecture {
			t.Errorf("expected type DiagramArchitecture, got %v", gen.config.Type)
		}
	})
}

func TestD2Generator_SetConfig(t *testing.T) {
	gen := NewD2Generator(nil)
	newCfg := &DiagramConfig{Type: DiagramCallFlow}
	gen.SetConfig(newCfg)
	if gen.config.Type != DiagramCallFlow {
		t.Errorf("expected config to be updated")
	}
}

func TestD2Generator_GetConfig(t *testing.T) {
	cfg := &DiagramConfig{Type: DiagramCoverage}
	gen := NewD2Generator(cfg)
	got := gen.GetConfig()
	if got.Type != DiagramCoverage {
		t.Errorf("expected to get same config")
	}
}

func TestD2Generator_Generate_ThemeConfig(t *testing.T) {
	tests := []struct {
		name     string
		theme    string
		wantID   string
		wantLang string
	}{
		{"default theme", "default", "theme-id: 8", "layout-engine: elk"},        // Colorblind Clear
		{"dark theme", "dark", "theme-id: 200", "layout-engine: elk"},            // Dark Mauve
		{"neutral theme", "neutral", "theme-id: 0", "layout-engine: elk"},        // Neutral Default
		{"invalid theme falls back to default", "invalid", "theme-id: 8", "layout-engine: elk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewD2Generator(&DiagramConfig{
				Theme:     tt.theme,
				Direction: "right",
			})
			result := gen.Generate(nil, nil)

			if !strings.Contains(result, tt.wantID) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantID, result)
			}
			if !strings.Contains(result, tt.wantLang) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantLang, result)
			}
		})
	}
}

func TestD2Generator_Generate_Title(t *testing.T) {
	gen := NewD2Generator(&DiagramConfig{
		Title:     "My Diagram",
		Direction: "right",
	})
	result := gen.Generate(nil, nil)

	if !strings.Contains(result, `label: "My Diagram"`) {
		t.Errorf("expected title in output, got:\n%s", result)
	}
	if !strings.Contains(result, "near: top-center") {
		t.Errorf("expected title position in output")
	}
}

func TestD2Generator_Generate_Direction(t *testing.T) {
	tests := []string{"right", "down", "left", "up"}
	for _, dir := range tests {
		t.Run(dir, func(t *testing.T) {
			gen := NewD2Generator(&DiagramConfig{Direction: dir})
			result := gen.Generate(nil, nil)
			if !strings.Contains(result, "direction: "+dir) {
				t.Errorf("expected direction %s in output", dir)
			}
		})
	}
}

func TestD2Generator_Generate_DependencyDiagram(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "func1", Name: "Function1", Type: "function", Importance: "keystone"},
		{ID: "func2", Name: "Function2", Type: "function", Importance: "normal"},
	}
	deps := []DiagramEdge{
		{From: "func1", To: "func2", Type: "calls", Label: "invokes"},
	}

	gen := NewD2Generator(&DiagramConfig{
		Type:       DiagramDeps,
		Direction:  "right",
		ShowLabels: true,
		ShowIcons:  true,
	})
	result := gen.Generate(entities, deps)

	// Check nodes section
	if !strings.Contains(result, "# Nodes") {
		t.Error("expected nodes section")
	}
	if !strings.Contains(result, "func1:") {
		t.Error("expected func1 node")
	}
	if !strings.Contains(result, "func2:") {
		t.Error("expected func2 node")
	}

	// Check edges section
	if !strings.Contains(result, "# Edges") {
		t.Error("expected edges section")
	}
	if !strings.Contains(result, "func1 -> func2") {
		t.Error("expected edge from func1 to func2")
	}

	// Check styling applied
	if !strings.Contains(result, "shape:") {
		t.Error("expected shape in node")
	}
	if !strings.Contains(result, "fill:") {
		t.Error("expected fill color in style")
	}
}

func TestD2Generator_Generate_ArchitectureDiagram(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "cmd.report", Name: "report", Type: "function", Module: "internal/cmd"},
		{ID: "cmd.show", Name: "show", Type: "function", Module: "internal/cmd"},
		{ID: "store.Get", Name: "Get", Type: "method", Module: "internal/store"},
	}
	deps := []DiagramEdge{
		{From: "cmd.report", To: "store.Get", Type: "calls"},
	}

	gen := NewD2Generator(&DiagramConfig{
		Type:       DiagramArchitecture,
		Direction:  "right",
		ShowLabels: true,
	})
	result := gen.Generate(entities, deps)

	// Check module containers
	if !strings.Contains(result, "# Module Layers") {
		t.Error("expected module layers section")
	}

	// Check container styling
	if !strings.Contains(result, "border-radius: 8") {
		t.Error("expected container border radius")
	}

	// Check connections section
	if !strings.Contains(result, "# Flow Connections") {
		t.Error("expected flow connections section")
	}
}

func TestD2Generator_Generate_CallFlowDiagram(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "handler", Name: "Handler", Type: "http"},
		{ID: "service", Name: "Service", Type: "function"},
		{ID: "store", Name: "Store", Type: "database"},
	}
	deps := []DiagramEdge{
		{From: "handler", To: "service", Type: "calls", Label: "process()"},
		{From: "service", To: "store", Type: "calls", Label: "query()"},
	}

	gen := NewD2Generator(&DiagramConfig{
		Type:       DiagramCallFlow,
		Direction:  "down",
		ShowLabels: true,
	})
	result := gen.Generate(entities, deps)

	// Check call flow section
	if !strings.Contains(result, "# Call Flow") {
		t.Error("expected call flow section")
	}
	if !strings.Contains(result, "# Flow") {
		t.Error("expected flow section")
	}

	// Check edge styling
	if !strings.Contains(result, "stroke:") {
		t.Error("expected stroke in edge styling")
	}
}

func TestD2Generator_Generate_CoverageDiagram(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "func1", Name: "HighCoverage", Type: "function", Coverage: 95, Importance: "normal"},
		{ID: "func2", Name: "MedCoverage", Type: "function", Coverage: 65, Importance: "normal"},
		{ID: "func3", Name: "LowCoverage", Type: "function", Coverage: 30, Importance: "keystone"},
		{ID: "func4", Name: "NoCoverage", Type: "function", Coverage: 0, Importance: "normal"},
	}
	deps := []DiagramEdge{
		{From: "func1", To: "func2", Type: "calls"},
	}

	gen := NewD2Generator(&DiagramConfig{
		Type:       DiagramCoverage,
		Direction:  "right",
		ShowLabels: true,
		ShowIcons:  true,
	})
	result := gen.Generate(entities, deps)

	// Check coverage analysis section
	if !strings.Contains(result, "# Coverage Analysis") {
		t.Error("expected coverage analysis section")
	}

	// Check legend
	if !strings.Contains(result, "Coverage Legend") {
		t.Error("expected coverage legend")
	}
	if !strings.Contains(result, ">80%") {
		t.Error("expected high coverage legend")
	}

	// Check coverage percentages in labels
	if !strings.Contains(result, "95% coverage") {
		t.Error("expected coverage percentage in label")
	}

	// Check important entity gets emphasized styling (stroke-width/shadow)
	// Note: Warning icons are disabled due to Terrastruct service returning 403
	if !strings.Contains(result, "stroke-width: 3") {
		t.Error("expected keystone entity to have emphasized stroke")
	}
}

func TestD2Generator_NodeStyling(t *testing.T) {
	tests := []struct {
		name       string
		entity     DiagramEntity
		wantShape  string
		wantInStyle string
	}{
		{
			name:       "function type",
			entity:     DiagramEntity{ID: "fn", Type: "function"},
			wantShape:  "shape: rectangle",
			wantInStyle: "fill:",
		},
		{
			name:       "database type",
			entity:     DiagramEntity{ID: "db", Type: "database"},
			wantShape:  "shape: cylinder",
			wantInStyle: "fill:",
		},
		{
			name:       "keystone importance",
			entity:     DiagramEntity{ID: "key", Type: "function", Importance: "keystone"},
			wantInStyle: "shadow: true",
		},
		{
			// Icons are disabled due to Terrastruct service returning 403
			// Language icon would normally appear here; testing that entity renders without icon
			name:       "with language (icon disabled)",
			entity:     DiagramEntity{ID: "fn", Type: "function", Language: "go"},
			wantInStyle: "fill:", // Still has styling even without icon
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewD2Generator(&DiagramConfig{
				Type:      DiagramDeps,
				ShowIcons: true,
			})
			result := gen.Generate([]DiagramEntity{tt.entity}, nil)

			if tt.wantShape != "" && !strings.Contains(result, tt.wantShape) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantShape, result)
			}
			if tt.wantInStyle != "" && !strings.Contains(result, tt.wantInStyle) {
				t.Errorf("expected %q in style, got:\n%s", tt.wantInStyle, result)
			}
		})
	}
}

func TestD2Generator_EdgeStyling(t *testing.T) {
	tests := []struct {
		name      string
		depType   string
		wantArrow string
	}{
		{"calls edge", "calls", "->"},
		{"implements edge", "implements", "->"},
		{"data flow edge", "data_flow", "->"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := []DiagramEntity{
				{ID: "a", Type: "function"},
				{ID: "b", Type: "function"},
			}
			deps := []DiagramEdge{
				{From: "a", To: "b", Type: tt.depType},
			}

			gen := NewD2Generator(&DiagramConfig{Type: DiagramDeps})
			result := gen.Generate(entities, deps)

			if !strings.Contains(result, "a "+tt.wantArrow+" b") {
				t.Errorf("expected arrow %q in edge, got:\n%s", tt.wantArrow, result)
			}
		})
	}
}

func TestD2Generator_SanitizeID(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "internal/cmd.runFind", Type: "function"},
		{ID: "store.Store.Get", Type: "method"},
	}

	gen := NewD2Generator(&DiagramConfig{Type: DiagramDeps})
	result := gen.Generate(entities, nil)

	// IDs with special chars should be quoted
	if !strings.Contains(result, `"internal/cmd.runFind"`) {
		t.Error("expected quoted ID for path with slash")
	}
	if !strings.Contains(result, `"store.Store.Get"`) {
		t.Error("expected quoted ID for dotted path")
	}
}

func TestD2Generator_WithIcons(t *testing.T) {
	// Note: Icons are disabled due to Terrastruct service returning 403 (as of 2026-01).
	// The icon maps are empty, so no icons are generated even when ShowIcons is true.
	// When icons are re-enabled, update these tests to check for actual icon output.

	t.Run("icons enabled", func(t *testing.T) {
		gen := NewD2Generator(&DiagramConfig{
			Type:      DiagramDeps,
			ShowIcons: true,
		})
		entities := []DiagramEntity{
			{ID: "fn", Type: "function", Language: "go"},
		}
		result := gen.Generate(entities, nil)

		// Icons are currently disabled; verify output still renders correctly
		if !strings.Contains(result, "fn:") {
			t.Error("expected entity to render")
		}
	})

	t.Run("icons disabled", func(t *testing.T) {
		gen := NewD2Generator(&DiagramConfig{
			Type:      DiagramDeps,
			ShowIcons: false,
		})
		entities := []DiagramEntity{
			{ID: "fn", Type: "function", Language: "go"},
		}
		result := gen.Generate(entities, nil)

		if strings.Contains(result, "icon:") {
			t.Error("expected no icon when ShowIcons is false")
		}
	})
}

func TestD2Generator_EdgeLabels(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "a", Type: "function"},
		{ID: "b", Type: "function"},
	}
	deps := []DiagramEdge{
		{From: "a", To: "b", Type: "calls", Label: "invoke"},
	}

	t.Run("labels enabled", func(t *testing.T) {
		gen := NewD2Generator(&DiagramConfig{
			Type:       DiagramDeps,
			ShowLabels: true,
		})
		result := gen.Generate(entities, deps)

		if !strings.Contains(result, ": invoke") {
			t.Error("expected label when ShowLabels is true")
		}
	})

	t.Run("labels disabled", func(t *testing.T) {
		gen := NewD2Generator(&DiagramConfig{
			Type:       DiagramDeps,
			ShowLabels: false,
		})
		result := gen.Generate(entities, deps)

		if strings.Contains(result, ": invoke") {
			t.Error("expected no label when ShowLabels is false")
		}
	})
}

func TestExtractModuleDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"internal/cmd", "cmd"},
		{"internal/store/entity", "entity"},
		{"single", "single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractModuleDisplayName(tt.input)
			if got != tt.want {
				t.Errorf("extractModuleDisplayName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetermineModuleLayer(t *testing.T) {
	tests := []struct {
		name     string
		entities []DiagramEntity
		want     string
	}{
		{
			name: "http handlers -> api layer",
			entities: []DiagramEntity{
				{Type: "http"},
				{Type: "handler"},
			},
			want: "api",
		},
		{
			name: "database -> data layer",
			entities: []DiagramEntity{
				{Type: "database"},
				{Type: "storage"},
			},
			want: "data",
		},
		{
			name: "explicit layer set",
			entities: []DiagramEntity{
				{Type: "function", Layer: "domain"},
			},
			want: "domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineModuleLayer(tt.entities)
			if got != tt.want {
				t.Errorf("determineModuleLayer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildEntityOrder(t *testing.T) {
	entities := []DiagramEntity{
		{ID: "c"},
		{ID: "a"},
		{ID: "b"},
	}
	deps := []DiagramEdge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
	}

	result := buildEntityOrder(entities, deps)

	// a should come first (no incoming edges)
	if result[0].ID != "a" {
		t.Errorf("expected 'a' first in topological order, got %v", result[0].ID)
	}
}

func TestSanitizeD2ID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with_underscore", "with_underscore"},
		{"with-dash", "with-dash"},
		{"with.dot", `"with.dot"`},
		{"path/slash", `"path/slash"`},
		{"with spaces", `"with spaces"`},
		{`with"quotes`, `"with\"quotes"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeD2ID(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeD2ID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractNodeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"package.Function", "Function"},
		{"internal/cmd.runFind", "runFind"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractNodeName(tt.input)
			if got != tt.want {
				t.Errorf("extractNodeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Test legacy API compatibility
func TestGenerateD2_Legacy(t *testing.T) {
	// Just test that the default options function works
	opts := DefaultD2Options()
	if opts.MaxNodes != 30 {
		t.Errorf("expected default MaxNodes 30, got %d", opts.MaxNodes)
	}
	if opts.Direction != "right" {
		t.Errorf("expected default direction 'right', got %s", opts.Direction)
	}
	if !opts.ShowLabels {
		t.Error("expected ShowLabels to be true by default")
	}
	if !opts.Collapse {
		t.Error("expected Collapse to be true by default")
	}
}

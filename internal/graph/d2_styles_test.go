package graph

import (
	"strings"
	"testing"
)

func TestGetD2NodeStyle(t *testing.T) {
	tests := []struct {
		name       string
		entityType string
		importance string
		coverage   float64
		language   string
		wantShape  string
		wantShadow bool
	}{
		{
			name:       "function normal",
			entityType: "function",
			importance: "normal",
			coverage:   -1, // no coverage
			language:   "",
			wantShape:  "rectangle",
			wantShadow: false,
		},
		{
			name:       "keystone function",
			entityType: "function",
			importance: "keystone",
			coverage:   -1,
			language:   "",
			wantShape:  "rectangle",
			wantShadow: true,
		},
		{
			name:       "interface type",
			entityType: "interface",
			importance: "normal",
			coverage:   -1,
			language:   "",
			wantShape:  "diamond",
			wantShadow: false,
		},
		{
			name:       "database type",
			entityType: "database",
			importance: "normal",
			coverage:   -1,
			language:   "",
			wantShape:  "cylinder",
			wantShadow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := GetD2NodeStyle(tt.entityType, tt.importance, tt.coverage, tt.language)

			if style.Shape != tt.wantShape {
				t.Errorf("Shape = %v, want %v", style.Shape, tt.wantShape)
			}
			if style.Shadow != tt.wantShadow {
				t.Errorf("Shadow = %v, want %v", style.Shadow, tt.wantShadow)
			}
		})
	}
}

func TestGetCoverageColor(t *testing.T) {
	tests := []struct {
		coverage   float64
		wantStroke string
	}{
		{95, "#4caf50"},  // high
		{80, "#4caf50"},  // high
		{79, "#fbc02d"},  // medium
		{50, "#fbc02d"},  // medium
		{49, "#f44336"},  // low
		{1, "#f44336"},   // low
		{0, "#9e9e9e"},   // none
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			color := GetCoverageColor(tt.coverage)
			if color.Stroke != tt.wantStroke {
				t.Errorf("GetCoverageColor(%v).Stroke = %v, want %v", tt.coverage, color.Stroke, tt.wantStroke)
			}
		})
	}
}

func TestGetCoverageLevel(t *testing.T) {
	tests := []struct {
		coverage float64
		want     string
	}{
		{100, "high"},
		{80, "high"},
		{79.9, "medium"},
		{50, "medium"},
		{49.9, "low"},
		{1, "low"},
		{0, "none"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetCoverageLevel(tt.coverage)
			if got != tt.want {
				t.Errorf("GetCoverageLevel(%v) = %v, want %v", tt.coverage, got, tt.want)
			}
		})
	}
}

func TestGetD2EdgeStyle(t *testing.T) {
	tests := []struct {
		depType   string
		wantArrow string
	}{
		{"calls", "->"},
		{"implements", "->"},
		{"data_flow", "->"},
		{"unknown", "->"},
	}

	for _, tt := range tests {
		t.Run(tt.depType, func(t *testing.T) {
			style := GetD2EdgeStyle(tt.depType)
			if style.Arrow != tt.wantArrow {
				t.Errorf("Arrow = %v, want %v", style.Arrow, tt.wantArrow)
			}
		})
	}
}

func TestGetD2Icon(t *testing.T) {
	tests := []struct {
		entityType string
		wantPrefix string
	}{
		{"function", "https://icons.terrastruct.com/"},
		{"method", "https://icons.terrastruct.com/"},
		{"database", "https://icons.terrastruct.com/"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.entityType, func(t *testing.T) {
			icon := GetD2Icon(tt.entityType)
			if tt.wantPrefix == "" {
				if icon != "" {
					t.Errorf("Expected empty icon for %v, got %v", tt.entityType, icon)
				}
			} else {
				if !strings.HasPrefix(string(icon), tt.wantPrefix) {
					t.Errorf("Icon for %v = %v, want prefix %v", tt.entityType, icon, tt.wantPrefix)
				}
			}
		})
	}
}

func TestGetD2LanguageIcon(t *testing.T) {
	tests := []struct {
		language   string
		wantSuffix string
	}{
		{"go", "go.svg"},
		{"typescript", "typescript.svg"},
		{"python", "python.svg"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			icon := GetD2LanguageIcon(tt.language)
			if tt.wantSuffix == "" {
				if icon != "" {
					t.Errorf("Expected empty icon for %v, got %v", tt.language, icon)
				}
			} else {
				if !strings.HasSuffix(string(icon), tt.wantSuffix) {
					t.Errorf("Icon for %v = %v, want suffix %v", tt.language, icon, tt.wantSuffix)
				}
			}
		})
	}
}

func TestD2StyleToString(t *testing.T) {
	style := D2NodeStyle{
		Fill:        "#e3f2fd",
		Stroke:      "#1976d2",
		StrokeWidth: 2,
		Shadow:      true,
	}

	result := D2StyleToString(style)

	if !strings.Contains(result, "fill: \"#e3f2fd\"") {
		t.Error("Missing fill in style string")
	}
	if !strings.Contains(result, "stroke: \"#1976d2\"") {
		t.Error("Missing stroke in style string")
	}
	if !strings.Contains(result, "stroke-width: 2") {
		t.Error("Missing stroke-width in style string")
	}
	if !strings.Contains(result, "shadow: true") {
		t.Error("Missing shadow in style string")
	}
}

func TestD2EdgeStyleToString(t *testing.T) {
	style := D2EdgeStyleDef{
		StrokeColor: "#424242",
		StrokeWidth: 2,
		StrokeDash:  3,
		Animated:    true,
	}

	result := D2EdgeStyleToString(style)

	if !strings.Contains(result, "stroke: \"#424242\"") {
		t.Error("Missing stroke in edge style string")
	}
	if !strings.Contains(result, "stroke-width: 2") {
		t.Error("Missing stroke-width in edge style string")
	}
	if !strings.Contains(result, "stroke-dash: 3") {
		t.Error("Missing stroke-dash in edge style string")
	}
	if !strings.Contains(result, "animated: true") {
		t.Error("Missing animated in edge style string")
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{123, "123"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := itoa(tt.input)
			if got != tt.want {
				t.Errorf("itoa(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFtoa(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{1.0, "1.0"},
		{0.0, "0.0"},
		{0.9, "0.9"},
		{0.5, "0.5"},
		{0.1, "0.1"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := ftoa(tt.input)
			if got != tt.want {
				t.Errorf("ftoa(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestD2EntityColors(t *testing.T) {
	// Ensure all expected entity types have colors
	expectedTypes := []string{"function", "method", "type", "struct", "interface", "constant", "database", "http", "test"}

	for _, entityType := range expectedTypes {
		if _, ok := D2EntityColors[entityType]; !ok {
			t.Errorf("Missing color for entity type: %v", entityType)
		}
	}
}

func TestD2ImportanceColors(t *testing.T) {
	// Ensure all expected importance levels have colors
	expectedLevels := []string{"keystone", "bottleneck", "high-fan-in", "high-fan-out", "normal", "leaf"}

	for _, level := range expectedLevels {
		if _, ok := D2ImportanceColors[level]; !ok {
			t.Errorf("Missing color for importance level: %v", level)
		}
	}
}

func TestD2CoverageColors(t *testing.T) {
	// Ensure all expected coverage levels have colors
	expectedLevels := []string{"high", "medium", "low", "none"}

	for _, level := range expectedLevels {
		if _, ok := D2CoverageColors[level]; !ok {
			t.Errorf("Missing color for coverage level: %v", level)
		}
	}
}

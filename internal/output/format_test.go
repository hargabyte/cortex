package output

import (
	"testing"
)

// TestGetFormatterYAML tests that GetFormatter returns a YAML formatter
func TestGetFormatterYAML(t *testing.T) {
	formatter, err := GetFormatter(FormatYAML)
	if err != nil {
		t.Fatalf("GetFormatter(FormatYAML) failed: %v", err)
	}

	_, ok := formatter.(*YAMLFormatter)
	if !ok {
		t.Errorf("expected *YAMLFormatter, got %T", formatter)
	}
}

// TestGetFormatterJSON tests that GetFormatter returns a JSON formatter
func TestGetFormatterJSON(t *testing.T) {
	formatter, err := GetFormatter(FormatJSON)
	if err != nil {
		t.Fatalf("GetFormatter(FormatJSON) failed: %v", err)
	}

	_, ok := formatter.(*JSONFormatter)
	if !ok {
		t.Errorf("expected *JSONFormatter, got %T", formatter)
	}
}

// TestGetFormatterCGF tests that GetFormatter returns a CGF formatter
func TestGetFormatterCGF(t *testing.T) {
	formatter, err := GetFormatter(FormatCGF)
	if err != nil {
		t.Fatalf("GetFormatter(FormatCGF) failed: %v", err)
	}

	_, ok := formatter.(*CGFFormatter)
	if !ok {
		t.Errorf("expected *CGFFormatter, got %T", formatter)
	}
}

// TestGetFormatterInvalid tests that GetFormatter returns error for invalid format
func TestGetFormatterInvalid(t *testing.T) {
	_, err := GetFormatter(Format("invalid"))
	if err == nil {
		t.Error("GetFormatter should return error for invalid format")
	}
}

// TestFormatString tests the String() method
func TestFormatString(t *testing.T) {
	tests := []struct {
		format   Format
		expected string
	}{
		{FormatYAML, "yaml"},
		{FormatJSON, "json"},
		{FormatCGF, "cgf"},
	}

	for _, tt := range tests {
		if got := tt.format.String(); got != tt.expected {
			t.Errorf("Format(%s).String() = %s, want %s", tt.format, got, tt.expected)
		}
	}
}

// TestDensityString tests the String() method for Density
func TestDensityString(t *testing.T) {
	tests := []struct {
		density  Density
		expected string
	}{
		{DensitySparse, "sparse"},
		{DensityMedium, "medium"},
		{DensityDense, "dense"},
		{DensitySmart, "smart"},
	}

	for _, tt := range tests {
		if got := tt.density.String(); got != tt.expected {
			t.Errorf("Density(%s).String() = %s, want %s", tt.density, got, tt.expected)
		}
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
		wantErr  bool
	}{
		{"yaml", FormatYAML, false},
		{"YAML", FormatYAML, false},
		{"json", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{"cgf", FormatCGF, false},
		{"CGF", FormatCGF, false},
		{"  yaml  ", FormatYAML, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFormat(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatIsDeprecated(t *testing.T) {
	tests := []struct {
		format   Format
		expected bool
	}{
		{FormatYAML, false},
		{FormatJSON, false},
		{FormatCGF, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got := tt.format.IsDeprecated()
			if got != tt.expected {
				t.Errorf("Format(%s).IsDeprecated() = %v, want %v", tt.format, got, tt.expected)
			}
		})
	}
}

func TestFormatDeprecationWarning(t *testing.T) {
	// YAML should have no warning
	if warning := FormatYAML.DeprecationWarning(); warning != "" {
		t.Errorf("FormatYAML should have no deprecation warning, got: %s", warning)
	}

	// JSON should have no warning
	if warning := FormatJSON.DeprecationWarning(); warning != "" {
		t.Errorf("FormatJSON should have no deprecation warning, got: %s", warning)
	}

	// CGF should have a warning
	if warning := FormatCGF.DeprecationWarning(); warning == "" {
		t.Error("FormatCGF should have a deprecation warning")
	}
}

func TestParseDensity(t *testing.T) {
	tests := []struct {
		input    string
		expected Density
		wantErr  bool
	}{
		{"sparse", DensitySparse, false},
		{"SPARSE", DensitySparse, false},
		{"medium", DensityMedium, false},
		{"MEDIUM", DensityMedium, false},
		{"dense", DensityDense, false},
		{"DENSE", DensityDense, false},
		{"smart", DensitySmart, false},
		{"SMART", DensitySmart, false},
		{"  medium  ", DensityMedium, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDensity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDensity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseDensity(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDensityIncludesSignature(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, false},
		{DensityMedium, true},
		{DensityDense, true},
		{DensitySmart, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := tt.density.IncludesSignature()
			if got != tt.expected {
				t.Errorf("Density(%s).IncludesSignature() = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestDensityIncludesEdges(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, false},
		{DensityMedium, true},
		{DensityDense, true},
		{DensitySmart, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := tt.density.IncludesEdges()
			if got != tt.expected {
				t.Errorf("Density(%s).IncludesEdges() = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestDensityIncludesMetrics(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, false},
		{DensityMedium, false},
		{DensityDense, true},
		{DensitySmart, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := tt.density.IncludesMetrics()
			if got != tt.expected {
				t.Errorf("Density(%s).IncludesMetrics() = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestDensityIncludesHashes(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, false},
		{DensityMedium, false},
		{DensityDense, true},
		{DensitySmart, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := tt.density.IncludesHashes()
			if got != tt.expected {
				t.Errorf("Density(%s).IncludesHashes() = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestDensityIncludesTimestamps(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, false},
		{DensityMedium, false},
		{DensityDense, true},
		{DensitySmart, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := tt.density.IncludesTimestamps()
			if got != tt.expected {
				t.Errorf("Density(%s).IncludesTimestamps() = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestDensityIncludesExtendedContext(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, false},
		{DensityMedium, false},
		{DensityDense, true},
		{DensitySmart, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := tt.density.IncludesExtendedContext()
			if got != tt.expected {
				t.Errorf("Density(%s).IncludesExtendedContext() = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestGetEffectiveDensity(t *testing.T) {
	tests := []struct {
		name     string
		density  Density
		inDegree int
		expected Density
	}{
		// Non-smart modes return same density
		{"sparse stays sparse", DensitySparse, 5, DensitySparse},
		{"medium stays medium", DensityMedium, 5, DensityMedium},
		{"dense stays dense", DensityDense, 5, DensityDense},

		// Smart mode adaptively changes
		{"smart: keystone (in_degree >= 10)", DensitySmart, 12, DensityDense},
		{"smart: keystone boundary (in_degree = 10)", DensitySmart, 10, DensityDense},
		{"smart: normal (in_degree >= 3)", DensitySmart, 5, DensityMedium},
		{"smart: normal boundary (in_degree = 3)", DensitySmart, 3, DensityMedium},
		{"smart: leaf (in_degree < 3)", DensitySmart, 2, DensitySparse},
		{"smart: leaf (in_degree = 0)", DensitySmart, 0, DensitySparse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEffectiveDensity(tt.density, tt.inDegree)
			if got != tt.expected {
				t.Errorf("GetEffectiveDensity(%s, %d) = %v, want %v", tt.density, tt.inDegree, got, tt.expected)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		format   Format
		expected bool
	}{
		{FormatYAML, true},
		{FormatJSON, true},
		{FormatCGF, true},
		{Format("invalid"), false},
		{Format(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got := ValidateFormat(tt.format)
			if got != tt.expected {
				t.Errorf("ValidateFormat(%s) = %v, want %v", tt.format, got, tt.expected)
			}
		})
	}
}

func TestValidateDensity(t *testing.T) {
	tests := []struct {
		density  Density
		expected bool
	}{
		{DensitySparse, true},
		{DensityMedium, true},
		{DensityDense, true},
		{DensitySmart, true},
		{Density("invalid"), false},
		{Density(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.density), func(t *testing.T) {
			got := ValidateDensity(tt.density)
			if got != tt.expected {
				t.Errorf("ValidateDensity(%s) = %v, want %v", tt.density, got, tt.expected)
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultFormat != FormatYAML {
		t.Errorf("DefaultFormat should be YAML, got %s", DefaultFormat)
	}

	if DefaultDensity != DensityMedium {
		t.Errorf("DefaultDensity should be medium, got %s", DefaultDensity)
	}
}

// Package output provides format and density types for CX 2.0.
package output

import (
	"fmt"
	"strings"
)

// Format represents the output format type.
type Format string

const (
	// FormatYAML is the default self-documenting YAML output
	FormatYAML Format = "yaml"

	// FormatJSON is the JSON output format
	FormatJSON Format = "json"

	// FormatCGF is the deprecated compact CGF format
	// This format is maintained for backward compatibility but will be removed in v2.0
	FormatCGF Format = "cgf"
)

// ParseFormat parses a format string into a Format value.
// Accepts: "yaml", "json", "cgf" (case-insensitive)
// Returns an error for invalid format values.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yaml":
		return FormatYAML, nil
	case "json":
		return FormatJSON, nil
	case "cgf":
		return FormatCGF, nil
	default:
		return "", fmt.Errorf("invalid format: %q (expected yaml, json, or cgf)", s)
	}
}

// String returns the string representation of the format.
func (f Format) String() string {
	return string(f)
}

// IsDeprecated returns true if this format is deprecated.
func (f Format) IsDeprecated() bool {
	return f == FormatCGF
}

// DeprecationWarning returns the deprecation warning message for this format.
// Returns empty string if format is not deprecated.
func (f Format) DeprecationWarning() string {
	if f == FormatCGF {
		return "Warning: CGF format is deprecated and will be removed in v2.0.\nUse --format=yaml (default) or --format=json."
	}
	return ""
}

// Density represents the level of detail in output.
// Different density levels optimize for different use cases:
//   - Sparse: Minimal tokens for "find where" queries
//   - Medium: Balanced detail for most use cases (default)
//   - Dense: Full detail including hashes, metrics, timestamps
//   - Smart: Importance-based density (keystones expanded, leaves compact)
type Density string

const (
	// DensitySparse provides minimal one-line format
	// Example: LoginUser: function @ internal/auth/login.go:45-89
	DensitySparse Density = "sparse"

	// DensityMedium provides balanced detail (default)
	// Includes: type, location, signature, visibility, basic dependencies
	DensityMedium Density = "medium"

	// DensityDense provides full detail
	// Includes: everything in medium + hashes, metrics, timestamps, all edges
	DensityDense Density = "dense"

	// DensitySmart provides importance-based density
	// Keystones (high in_degree) get dense format
	// Normal entities get medium format
	// Leaves (in_degree=0) get sparse/flow format
	DensitySmart Density = "smart"
)

// ParseDensity parses a density string into a Density value.
// Accepts: "sparse", "medium", "dense", "smart" (case-insensitive)
// Returns an error for invalid density values.
func ParseDensity(s string) (Density, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sparse":
		return DensitySparse, nil
	case "medium":
		return DensityMedium, nil
	case "dense":
		return DensityDense, nil
	case "smart":
		return DensitySmart, nil
	default:
		return "", fmt.Errorf("invalid density: %q (expected sparse, medium, dense, or smart)", s)
	}
}

// String returns the string representation of the density.
func (d Density) String() string {
	return string(d)
}

// IncludesSignature returns true if this density level includes signatures.
func (d Density) IncludesSignature() bool {
	return d == DensityMedium || d == DensityDense || d == DensitySmart
}

// IncludesEdges returns true if this density level includes edge information.
func (d Density) IncludesEdges() bool {
	return d == DensityMedium || d == DensityDense || d == DensitySmart
}

// IncludesMetrics returns true if this density level includes metrics.
func (d Density) IncludesMetrics() bool {
	return d == DensityDense
}

// IncludesHashes returns true if this density level includes hashes.
func (d Density) IncludesHashes() bool {
	return d == DensityDense
}

// IncludesTimestamps returns true if this density level includes timestamps.
func (d Density) IncludesTimestamps() bool {
	return d == DensityDense
}

// IncludesExtendedContext returns true if this density level includes extended context.
// Extended context includes "why" annotations for dependencies in dense mode.
func (d Density) IncludesExtendedContext() bool {
	return d == DensityDense
}

// DefaultFormat is the default output format when none is specified.
const DefaultFormat = FormatYAML

// DefaultDensity is the default density level when none is specified.
const DefaultDensity = DensityMedium

// ValidateFormat checks if a format value is valid.
func ValidateFormat(f Format) bool {
	switch f {
	case FormatYAML, FormatJSON, FormatCGF:
		return true
	default:
		return false
	}
}

// ValidateDensity checks if a density value is valid.
func ValidateDensity(d Density) bool {
	switch d {
	case DensitySparse, DensityMedium, DensityDense, DensitySmart:
		return true
	default:
		return false
	}
}

// GetEffectiveDensity returns the effective density for a given entity
// when using smart density mode. For other modes, returns the same density.
//
// Smart mode rules:
//   - in_degree >= 10: Dense (keystones - many things depend on this)
//   - in_degree >= 3: Medium (normal entities)
//   - in_degree < 3: Sparse (leaves - fewer dependencies)
func GetEffectiveDensity(d Density, inDegree int) Density {
	if d != DensitySmart {
		return d
	}

	// Smart density based on importance
	if inDegree >= 10 {
		return DensityDense
	} else if inDegree >= 3 {
		return DensityMedium
	}
	return DensitySparse
}

// Package output provides density level handling for CGF output.
// DEPRECATED: CGF format is deprecated and will be removed in v2.0.
package output

import (
	"fmt"
	"strings"
)

// CGFDensity represents the level of detail in CGF output (DEPRECATED)
// DEPRECATED: Use output.Density instead.
type CGFDensity string

const (
	// CGFSparse provides routing only - minimal tokens for "find where"
	// Includes: marker, location, name
	// Example: F src/auth/login.go:45 LoginUser
	CGFSparse CGFDensity = "sparse"

	// CGFMedium provides signatures + direct edges - default for task context
	// Includes: marker, location, name, signature, direct edges
	// Example: F src/auth/login.go:45-89 LoginUser(email:str,pass:str)->(*User,err)
	CGFMedium CGFDensity = "medium"

	// CGFDense provides full context for editing
	// Includes: everything in medium + comments, type refs, all edges, metrics
	// Example: F src/auth/login.go:45-89 LoginUser(email:str,pass:str)->(*User,err)
	//            importance=medium pr=0.12 deps=3 betw=0.08
	CGFDense CGFDensity = "dense"
)

// ParseCGFDensity parses a density string into a CGFDensity value.
// Accepts: "sparse", "medium", "dense" (case-insensitive)
// Returns an error for invalid density values.
// DEPRECATED: Use output.ParseDensity instead.
func ParseCGFDensity(s string) (CGFDensity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sparse":
		return CGFSparse, nil
	case "medium":
		return CGFMedium, nil
	case "dense":
		return CGFDense, nil
	default:
		return "", fmt.Errorf("invalid density: %q (expected sparse, medium, or dense)", s)
	}
}

// ValidateCGFDensity checks if a density value is valid
// DEPRECATED: Use output.ValidateDensity instead.
func ValidateCGFDensity(d CGFDensity) bool {
	switch d {
	case CGFSparse, CGFMedium, CGFDense:
		return true
	default:
		return false
	}
}

// String returns the string representation of the density
func (d CGFDensity) String() string {
	return string(d)
}

// IncludesSignature returns true if this density level includes signatures
func (d CGFDensity) IncludesSignature() bool {
	return d == CGFMedium || d == CGFDense
}

// IncludesEdges returns true if this density level includes edge information
func (d CGFDensity) IncludesEdges() bool {
	return d == CGFMedium || d == CGFDense
}

// IncludesMetrics returns true if this density level includes metrics
func (d CGFDensity) IncludesMetrics() bool {
	return d == CGFDense
}

// IncludesComments returns true if this density level includes comments/annotations
func (d CGFDensity) IncludesComments() bool {
	return d == CGFDense
}

// CGFDefaultDensity is the default density level used when none is specified
// DEPRECATED: Use output.DefaultDensity instead.
const CGFDefaultDensity = CGFMedium

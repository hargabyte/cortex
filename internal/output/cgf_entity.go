// Package output provides entity types and formatting for CGF output.
// DEPRECATED: CGF format is deprecated and will be removed in v2.0.
package output

import (
	"fmt"
	"strings"
)

// CGFEntityType represents the type marker for a code entity (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFEntityType string

const (
	// CGFModule represents a module or directory
	CGFModule CGFEntityType = "M"

	// CGFFunction represents a function or method
	CGFFunction CGFEntityType = "F"

	// CGFType represents a type, interface, class, or struct
	CGFType CGFEntityType = "T"

	// CGFConstant represents a constant or variable
	CGFConstant CGFEntityType = "C"

	// CGFEnum represents an enumeration
	CGFEnum CGFEntityType = "E"

	// CGFImport represents an import statement
	CGFImport CGFEntityType = "I"

	// CGFWork represents a work item (bead)
	CGFWork CGFEntityType = "W"

	// CGFExternal represents an external reference (stdlib, other repos)
	CGFExternal CGFEntityType = "X"
)

// String returns the string representation of the entity type
func (t CGFEntityType) String() string {
	return string(t)
}

// CGFEntityTypeFromString parses a string into a CGFEntityType
// DEPRECATED: CGF format is deprecated.
func CGFEntityTypeFromString(s string) (CGFEntityType, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "M":
		return CGFModule, nil
	case "F":
		return CGFFunction, nil
	case "T":
		return CGFType, nil
	case "C":
		return CGFConstant, nil
	case "E":
		return CGFEnum, nil
	case "I":
		return CGFImport, nil
	case "W":
		return CGFWork, nil
	case "X":
		return CGFExternal, nil
	default:
		return "", fmt.Errorf("invalid entity type: %q", s)
	}
}

// CGFEntity represents a code entity for CGF output (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFEntity struct {
	// Type is the entity type marker (M, F, T, C, E, I, W, X)
	Type CGFEntityType

	// Location is the file:line or file:start-end reference
	// Examples: "src/auth/login.go:45", "src/auth/login.go:45-89"
	Location string

	// Name is the identifier/name of the entity
	Name string

	// Signature is the type signature for functions/types (density >= medium)
	// Examples: "(email:str,pass:str)->(*User,err)", "{email:str,pass:str}"
	Signature string

	// Metrics contains optional importance metrics (density == dense)
	Metrics *CGFEntityMetrics
}

// CGFEntityMetrics contains importance and graph metrics for an entity (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFEntityMetrics struct {
	// Importance is the computed importance level: critical, high, medium, low
	Importance string

	// PageRank is the PageRank score (0.0 to 1.0)
	// critical >= 0.50, high >= 0.30, medium >= 0.10, low < 0.10
	PageRank float64

	// Deps is the count of entities that depend on this one
	Deps int

	// Betweenness is the betweenness centrality score (0.0 to 1.0)
	Betweenness float64
}

// WriteEntity writes an entity line in CGF format.
//
// Sparse format:
//
//	F src/auth/login.go:45 LoginUser
//
// Medium format:
//
//	F src/auth/login.go:45-89 LoginUser(email:str,pass:str)->(*User,err)
//
// Dense format:
//
//	F src/auth/login.go:45-89 LoginUser(email:str,pass:str)->(*User,err)
//	   importance=medium pr=0.12 deps=3 betw=0.08
func (w *CGFWriter) WriteEntity(e *CGFEntity) error {
	// Build the main entity line
	var parts []string
	parts = append(parts, string(e.Type))
	parts = append(parts, e.Location)

	// Name and signature based on density
	if w.density.IncludesSignature() && e.Signature != "" {
		parts = append(parts, e.Name+" "+e.Signature)
	} else {
		parts = append(parts, e.Name)
	}

	// Write the main line
	line := strings.Join(parts, " ")
	if err := w.writeLineRaw("%s", line); err != nil {
		return err
	}

	// Write metrics on a new indented line (dense only)
	if w.density.IncludesMetrics() && e.Metrics != nil {
		w.Indent()
		if err := w.writeMetrics(e.Metrics); err != nil {
			w.Dedent()
			return err
		}
		w.Dedent()
	}

	return nil
}

// writeMetrics writes entity metrics as an indented line
func (w *CGFWriter) writeMetrics(m *CGFEntityMetrics) error {
	var parts []string

	if m.Importance != "" {
		parts = append(parts, fmt.Sprintf("importance=%s", m.Importance))
	}
	if m.PageRank > 0 {
		parts = append(parts, fmt.Sprintf("pr=%.2f", m.PageRank))
	}
	if m.Deps > 0 {
		parts = append(parts, fmt.Sprintf("deps=%d", m.Deps))
	}
	if m.Betweenness > 0 {
		parts = append(parts, fmt.Sprintf("betw=%.2f", m.Betweenness))
	}

	if len(parts) > 0 {
		return w.writeLineRaw("%s", strings.Join(parts, " "))
	}
	return nil
}

// WriteExternalRef writes an external reference in CGF format.
// Format: X @<source>:<name>
// Examples:
//
//	X @stdlib:fmt.Println
//	X @github.com/pkg/errors:Wrap
//	X @internal/shared:Logger
func (w *CGFWriter) WriteExternalRef(source, name string) error {
	return w.writeLineRaw("X @%s:%s", source, name)
}

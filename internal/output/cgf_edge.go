// Package output provides edge types and formatting for CGF output.
// DEPRECATED: CGF format is deprecated and will be removed in v2.0.
package output

import (
	"fmt"
	"strings"
)

// CGFEdgeType represents the type marker for a relationship edge (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFEdgeType string

const (
	// CGFCalls represents an outgoing call/invocation
	// Format: >target
	CGFCalls CGFEdgeType = ">"

	// CGFCalledBy represents an incoming call (this entity is called by target)
	// Format: <caller
	CGFCalledBy CGFEdgeType = "<"

	// CGFUsesType represents a type usage relationship
	// Format: @TypeName
	CGFUsesType CGFEdgeType = "@"

	// CGFImplements represents an implements/extends relationship
	// Format: ^Interface
	CGFImplements CGFEdgeType = "^"

	// CGFRelated represents a soft/related relationship
	// Format: ~target
	CGFRelated CGFEdgeType = "~"

	// CGFBlocks represents a blocking dependency relationship
	// Format: !blocker
	CGFBlocks CGFEdgeType = "!"

	// CGFTagged represents a tag/label relationship
	// Format: #tag
	CGFTagged CGFEdgeType = "#"
)

// String returns the string representation of the edge type
func (t CGFEdgeType) String() string {
	return string(t)
}

// CGFEdgeTypeFromString parses a string into a CGFEdgeType
// DEPRECATED: CGF format is deprecated.
func CGFEdgeTypeFromString(s string) (CGFEdgeType, error) {
	switch s {
	case ">":
		return CGFCalls, nil
	case "<":
		return CGFCalledBy, nil
	case "@":
		return CGFUsesType, nil
	case "^":
		return CGFImplements, nil
	case "~":
		return CGFRelated, nil
	case "!":
		return CGFBlocks, nil
	case "#":
		return CGFTagged, nil
	default:
		return "", fmt.Errorf("invalid edge type: %q", s)
	}
}

// CGFEdgeFlag represents a modifier flag for edges (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFEdgeFlag string

const (
	// CGFCycle indicates the edge is part of a dependency cycle
	CGFCycle CGFEdgeFlag = "*"

	// CGFConditional indicates the call is conditional/optional
	CGFConditional CGFEdgeFlag = "?"

	// CGFMultiple indicates multiple calls to the target
	CGFMultiple CGFEdgeFlag = "+"

	// CGFDeprecated indicates a deprecated path
	CGFDeprecated CGFEdgeFlag = "-"
)

// String returns the string representation of the edge flag
func (f CGFEdgeFlag) String() string {
	return string(f)
}

// CGFEdgeFlagFromString parses a string into a CGFEdgeFlag
// DEPRECATED: CGF format is deprecated.
func CGFEdgeFlagFromString(s string) (CGFEdgeFlag, error) {
	switch s {
	case "*":
		return CGFCycle, nil
	case "?":
		return CGFConditional, nil
	case "+":
		return CGFMultiple, nil
	case "-":
		return CGFDeprecated, nil
	default:
		return "", fmt.Errorf("invalid edge flag: %q", s)
	}
}

// CGFEdge represents a relationship between entities (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFEdge struct {
	// Type is the edge type marker (>, <, @, ^, ~, !, #)
	Type CGFEdgeType

	// Target is the location or ID of the target entity
	// Examples: "src/auth/jwt.go:12", "bd-a7c", "User"
	Target string

	// TargetName is the optional human-readable name of the target
	// Example: "Validate", "CheckMFA"
	TargetName string

	// Flags are optional modifiers for the edge
	Flags []CGFEdgeFlag

	// Annotation is an optional comment/explanation for the edge
	// Written after semicolon: ; why this call matters
	Annotation string
}

// WriteEdge writes an edge line in CGF format.
// Edges must be written indented under their parent entity.
//
// Basic format:
//
//	>target
//	<caller
//	@TypeName
//
// With flags:
//
//	>*target    (cycle)
//	>?target    (conditional)
//	>+target    (multiple)
//	>-target    (deprecated)
//
// With name:
//
//	>src/auth/jwt.go:12 Validate
//
// With annotation:
//
//	>?src/auth/mfa.go:34 CheckMFA ; if MFA enabled
func (w *CGFWriter) WriteEdge(e *CGFEdge) error {
	var sb strings.Builder

	// Write edge marker
	sb.WriteString(string(e.Type))

	// Write flags
	for _, flag := range e.Flags {
		sb.WriteString(string(flag))
	}

	// Write target
	sb.WriteString(" ")
	sb.WriteString(e.Target)

	// Write target name if present
	if e.TargetName != "" {
		sb.WriteString(" ")
		sb.WriteString(e.TargetName)
	}

	// Write annotation if present (dense mode or always if specified)
	if e.Annotation != "" && w.density.IncludesComments() {
		sb.WriteString(" ; ")
		sb.WriteString(e.Annotation)
	}

	return w.writeLineRaw("%s", sb.String())
}

// WriteEdgeWithAnnotation writes an edge with annotation regardless of density.
// Use this when the annotation is critical information, not just a comment.
func (w *CGFWriter) WriteEdgeWithAnnotation(e *CGFEdge) error {
	var sb strings.Builder

	// Write edge marker
	sb.WriteString(string(e.Type))

	// Write flags
	for _, flag := range e.Flags {
		sb.WriteString(string(flag))
	}

	// Write target
	sb.WriteString(" ")
	sb.WriteString(e.Target)

	// Write target name if present
	if e.TargetName != "" {
		sb.WriteString(" ")
		sb.WriteString(e.TargetName)
	}

	// Always write annotation if present
	if e.Annotation != "" {
		sb.WriteString(" ; ")
		sb.WriteString(e.Annotation)
	}

	return w.writeLineRaw("%s", sb.String())
}

// WriteSimpleEdge is a convenience method for writing a simple edge.
// For edges without flags or annotations.
func (w *CGFWriter) WriteSimpleEdge(edgeType CGFEdgeType, target string) error {
	return w.WriteEdge(&CGFEdge{
		Type:   edgeType,
		Target: target,
	})
}

// WriteCallEdge is a convenience method for writing a call edge.
// Format: >target [name]
func (w *CGFWriter) WriteCallEdge(target, name string) error {
	return w.WriteEdge(&CGFEdge{
		Type:       CGFCalls,
		Target:     target,
		TargetName: name,
	})
}

// WriteCalledByEdge is a convenience method for writing a called-by edge.
// Format: <caller [name]
func (w *CGFWriter) WriteCalledByEdge(caller, name string) error {
	return w.WriteEdge(&CGFEdge{
		Type:       CGFCalledBy,
		Target:     caller,
		TargetName: name,
	})
}

// WriteUsesTypeEdge is a convenience method for writing a uses-type edge.
// Format: @TypeName
func (w *CGFWriter) WriteUsesTypeEdge(typeName string) error {
	return w.WriteEdge(&CGFEdge{
		Type:   CGFUsesType,
		Target: typeName,
	})
}

// WriteImplementsEdge is a convenience method for writing an implements edge.
// Format: ^Interface
func (w *CGFWriter) WriteImplementsEdge(iface string) error {
	return w.WriteEdge(&CGFEdge{
		Type:   CGFImplements,
		Target: iface,
	})
}

// WriteTagEdge is a convenience method for writing a tag edge.
// Format: #tag
func (w *CGFWriter) WriteTagEdge(tag string) error {
	return w.WriteEdge(&CGFEdge{
		Type:   CGFTagged,
		Target: tag,
	})
}

// WriteBlocksEdge is a convenience method for writing a blocks edge.
// Format: !blocker-id
func (w *CGFWriter) WriteBlocksEdge(blockerID string) error {
	return w.WriteEdge(&CGFEdge{
		Type:   CGFBlocks,
		Target: blockerID,
	})
}

// WriteRelatedEdge is a convenience method for writing a related edge.
// Format: ~target
func (w *CGFWriter) WriteRelatedEdge(target string) error {
	return w.WriteEdge(&CGFEdge{
		Type:   CGFRelated,
		Target: target,
	})
}

// CompactEdges writes multiple edges on a single line (medium density style).
// Format: >target1 >target2 <?target3
func (w *CGFWriter) CompactEdges(edges []*CGFEdge) error {
	if len(edges) == 0 {
		return nil
	}

	var parts []string
	for _, e := range edges {
		var sb strings.Builder
		sb.WriteString(string(e.Type))
		for _, flag := range e.Flags {
			sb.WriteString(string(flag))
		}
		sb.WriteString(e.Target)
		parts = append(parts, sb.String())
	}

	return w.writeLineRaw("%s", strings.Join(parts, " "))
}

package graph

// EntityShape defines diagram shapes for different entity types.
// Both D2 and Mermaid have native shape support.
type EntityShape struct {
	D2Shape      string // D2 shape name (rectangle, hexagon, diamond, etc.)
	MermaidShape string // Mermaid shape syntax ([], {{}}, {()}, etc.)
}

// EntityShapes maps entity types to their diagram shapes.
// Entity types come from store.Entity.EntityType and store.Entity.Kind.
var EntityShapes = map[string]EntityShape{
	// Functions and methods - rectangles (most common)
	"function": {D2Shape: "rectangle", MermaidShape: "[]"},
	"method":   {D2Shape: "rectangle", MermaidShape: "[]"},

	// Types - hexagons to distinguish from functions
	"struct":    {D2Shape: "hexagon", MermaidShape: "{{}}"},
	"class":     {D2Shape: "hexagon", MermaidShape: "{{}}"},
	"type":      {D2Shape: "hexagon", MermaidShape: "{{}}"},
	"interface": {D2Shape: "diamond", MermaidShape: "{()}"},

	// Data - ovals and parallelograms
	"constant": {D2Shape: "oval", MermaidShape: "([])"},
	"variable": {D2Shape: "parallelogram", MermaidShape: "[/\\]"},
	"enum":     {D2Shape: "oval", MermaidShape: "([])"},

	// Modules/packages - cylinders (container-like)
	"package": {D2Shape: "cylinder", MermaidShape: "[()]"},
	"module":  {D2Shape: "cylinder", MermaidShape: "[()]"},

	// Default fallback
	"default": {D2Shape: "rectangle", MermaidShape: "[]"},
}

// EdgeStyle defines diagram edge styles for different dependency types.
type EdgeStyle struct {
	D2Style      string // D2 edge syntax (->, --, etc.)
	MermaidStyle string // Mermaid edge syntax (-->, ---, etc.)
	D2Arrowhead  string // D2 arrowhead style (optional)
}

// EdgeStyles maps dependency types to their diagram edge styles.
// Dependency types come from store.Dependency.DepType.
var EdgeStyles = map[string]EdgeStyle{
	// Direct calls - solid arrow
	"calls": {D2Style: "->", MermaidStyle: "-->", D2Arrowhead: ""},

	// Type usage - dashed arrow
	"uses_type": {D2Style: "->", MermaidStyle: "-.->", D2Arrowhead: ""},

	// Implementation - dotted with open arrow
	"implements": {D2Style: "->", MermaidStyle: "-.->", D2Arrowhead: ""},

	// Inheritance - solid with diamond
	"extends": {D2Style: "->", MermaidStyle: "-->>", D2Arrowhead: "diamond"},

	// Imports - dashed
	"imports": {D2Style: "->", MermaidStyle: "-.->", D2Arrowhead: ""},

	// References - dotted
	"references": {D2Style: "->", MermaidStyle: "-.->", D2Arrowhead: ""},

	// Default fallback
	"default": {D2Style: "->", MermaidStyle: "-->", D2Arrowhead: ""},
}

// ImportanceStyle defines visual emphasis for entity importance levels.
type ImportanceStyle struct {
	D2Class     string // D2 class name for styling
	D2Style     string // D2 inline style (stroke-width, etc.)
	MermaidNote string // Mermaid styling note
}

// ImportanceStyles maps importance levels to visual styles.
// Importance is determined by metrics (PageRank, betweenness, etc.)
var ImportanceStyles = map[string]ImportanceStyle{
	// Keystone - critical entities, bold emphasis
	"keystone": {
		D2Class:     "keystone",
		D2Style:     "stroke-width: 3",
		MermaidNote: ":::keystone",
	},

	// Bottleneck - high betweenness centrality
	"bottleneck": {
		D2Class:     "bottleneck",
		D2Style:     "stroke-width: 2; stroke: orange",
		MermaidNote: ":::bottleneck",
	},

	// High fan-in - many dependents
	"high-fan-in": {
		D2Class:     "high-fan-in",
		D2Style:     "stroke-width: 2",
		MermaidNote: ":::high-fan-in",
	},

	// High fan-out - many dependencies
	"high-fan-out": {
		D2Class:     "high-fan-out",
		D2Style:     "stroke-width: 2",
		MermaidNote: ":::high-fan-out",
	},

	// Normal - default styling
	"normal": {
		D2Class:     "",
		D2Style:     "",
		MermaidNote: "",
	},
}

// GetEntityShape returns the shape for an entity type, with fallback to default.
func GetEntityShape(entityType string) EntityShape {
	if shape, ok := EntityShapes[entityType]; ok {
		return shape
	}
	return EntityShapes["default"]
}

// GetEdgeStyle returns the style for an edge type, with fallback to default.
func GetEdgeStyle(edgeType string) EdgeStyle {
	if style, ok := EdgeStyles[edgeType]; ok {
		return style
	}
	return EdgeStyles["default"]
}

// GetImportanceStyle returns the style for an importance level, with fallback to normal.
func GetImportanceStyle(importance string) ImportanceStyle {
	if style, ok := ImportanceStyles[importance]; ok {
		return style
	}
	return ImportanceStyles["normal"]
}

// DiagramOptions contains common options for diagram generation.
type DiagramOptions struct {
	MaxNodes  int    // Maximum nodes before auto-collapsing (default: 30)
	Direction string // Layout direction: "right"/"LR" or "down"/"TD"
	Collapse  bool   // Auto-collapse to modules when > MaxNodes
	Title     string // Optional diagram title
}

// DefaultDiagramOptions returns sensible defaults for diagram generation.
func DefaultDiagramOptions() *DiagramOptions {
	return &DiagramOptions{
		MaxNodes:  30,
		Direction: "right",
		Collapse:  true,
		Title:     "",
	}
}

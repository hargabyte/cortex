package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/output"
)

// DiagramType specifies the type of diagram to generate.
type DiagramType string

const (
	DiagramArchitecture DiagramType = "architecture" // Module containers with layered entities
	DiagramCallFlow     DiagramType = "call_flow"    // Sequential function call flow
	DiagramDeps         DiagramType = "dependency"   // Entity dependency graph
	DiagramCoverage     DiagramType = "coverage"     // Coverage heatmap overlay
)

// DiagramConfig configures diagram generation.
type DiagramConfig struct {
	Type       DiagramType // Type of diagram to generate
	Theme      string      // Theme name: "default", "light", "dark", "neutral"
	Layout     string      // Layout engine: "elk", "dagre", "tala"
	Direction  string      // Layout direction: "right", "down", "left", "up"
	MaxNodes   int         // Maximum nodes before auto-collapsing
	Collapse   bool        // Auto-collapse to modules when > MaxNodes
	ShowLabels bool        // Show edge labels
	ShowIcons  bool        // Include icons on nodes
	Title      string      // Optional diagram title
}

// DefaultDiagramConfig returns sensible defaults for diagram generation.
func DefaultDiagramConfig() *DiagramConfig {
	return &DiagramConfig{
		Type:       DiagramDeps,
		Theme:      "default",
		Layout:     "elk",
		Direction:  "right",
		MaxNodes:   30,
		Collapse:   true,
		ShowLabels: true,
		ShowIcons:  true,
		Title:      "",
	}
}

// DiagramEntity represents an entity to include in the diagram.
type DiagramEntity struct {
	ID          string  // Unique identifier
	Name        string  // Display name
	Type        string  // Entity type: function, method, type, struct, interface, etc.
	Importance  string  // Importance: keystone, bottleneck, high-fan-in, high-fan-out, normal, leaf
	Coverage    float64 // Test coverage percentage (0-100), -1 if unknown
	Language    string  // Programming language
	Module      string  // Module/package for grouping
	Layer       string  // Architectural layer: api, service, data, domain
	ChangeState string  // Change state: added, modified, deleted, unchanged (empty = not applicable)
}

// DiagramEdge represents a connection between entities.
type DiagramEdge struct {
	From  string // Source entity ID
	To    string // Target entity ID
	Type  string // Dependency type: calls, uses_type, implements, extends, data_flow, imports
	Label string // Optional edge label
}

// DiagramGenerator defines the interface for diagram generation.
type DiagramGenerator interface {
	// Generate creates a diagram from entities and their dependencies.
	Generate(entities []DiagramEntity, deps []DiagramEdge) string

	// SetConfig updates the generator configuration.
	SetConfig(config *DiagramConfig)

	// GetConfig returns the current configuration.
	GetConfig() *DiagramConfig
}

// D2Generator generates D2 diagrams using the visual design system.
type D2Generator struct {
	config *DiagramConfig
}

// NewD2Generator creates a new D2 diagram generator.
func NewD2Generator(config *DiagramConfig) *D2Generator {
	if config == nil {
		config = DefaultDiagramConfig()
	}
	return &D2Generator{config: config}
}

// SetConfig updates the generator configuration.
func (g *D2Generator) SetConfig(config *DiagramConfig) {
	if config != nil {
		g.config = config
	}
}

// GetConfig returns the current configuration.
func (g *D2Generator) GetConfig() *DiagramConfig {
	return g.config
}

// Generate creates a D2 diagram from entities and dependencies.
func (g *D2Generator) Generate(entities []DiagramEntity, deps []DiagramEdge) string {
	var sb strings.Builder

	// Write theme configuration
	g.writeThemeConfig(&sb)

	// Write title if provided
	if g.config.Title != "" {
		sb.WriteString(fmt.Sprintf("\ntitle: {\n  label: %q\n  near: top-center\n}\n", g.config.Title))
	}

	// Write direction
	sb.WriteString(fmt.Sprintf("\ndirection: %s\n", g.config.Direction))

	sb.WriteString("\n")

	// Generate based on diagram type
	switch g.config.Type {
	case DiagramArchitecture:
		g.generateArchitecture(&sb, entities, deps)
	case DiagramCallFlow:
		g.generateCallFlow(&sb, entities, deps)
	case DiagramCoverage:
		g.generateCoverage(&sb, entities, deps)
	default:
		g.generateDependency(&sb, entities, deps)
	}

	return sb.String()
}

// writeThemeConfig writes the D2 vars block with theme configuration.
func (g *D2Generator) writeThemeConfig(sb *strings.Builder) {
	theme, ok := D2Themes[g.config.Theme]
	if !ok {
		theme = D2Themes["default"]
	}

	layout := g.config.Layout
	if layout == "" {
		layout = theme.LayoutEngine
	}

	sb.WriteString("vars: {\n")
	sb.WriteString("  d2-config: {\n")
	sb.WriteString(fmt.Sprintf("    theme-id: %d\n", theme.ID))
	sb.WriteString(fmt.Sprintf("    layout-engine: %s\n", layout))
	sb.WriteString("  }\n")
	sb.WriteString("}\n")
}

// generateDependency generates a standard dependency diagram.
func (g *D2Generator) generateDependency(sb *strings.Builder, entities []DiagramEntity, deps []DiagramEdge) {
	// Build entity map for quick lookup
	entityMap := make(map[string]DiagramEntity)
	for _, e := range entities {
		entityMap[e.ID] = e
	}

	// Sort entities by ID for deterministic output
	sortedEntities := make([]DiagramEntity, len(entities))
	copy(sortedEntities, entities)
	sort.Slice(sortedEntities, func(i, j int) bool {
		return sortedEntities[i].ID < sortedEntities[j].ID
	})

	// Write nodes
	sb.WriteString("# Nodes\n")
	for _, entity := range sortedEntities {
		g.writeNode(sb, entity)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Write edges
	sb.WriteString("# Edges\n")
	for _, dep := range deps {
		g.writeEdge(sb, dep)
		sb.WriteString("\n")
	}
}

// generateArchitecture generates an architecture diagram with module grouping.
func (g *D2Generator) generateArchitecture(sb *strings.Builder, entities []DiagramEntity, deps []DiagramEdge) {
	// Group entities by module
	modules := make(map[string][]DiagramEntity)
	for _, e := range entities {
		module := e.Module
		if module == "" {
			module = "_root"
		}
		modules[module] = append(modules[module], e)
	}

	// Sort module names
	moduleNames := make([]string, 0, len(modules))
	for name := range modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	// Write module containers
	sb.WriteString("# Modules\n")
	for _, moduleName := range moduleNames {
		moduleEntities := modules[moduleName]

		// Sort entities within module
		sort.Slice(moduleEntities, func(i, j int) bool {
			return moduleEntities[i].ID < moduleEntities[j].ID
		})

		if moduleName == "_root" {
			// Root entities without container
			for _, entity := range moduleEntities {
				g.writeNode(sb, entity)
				sb.WriteString("\n")
			}
		} else {
			// Create module container
			safeModuleName := sanitizeD2ID(moduleName)
			displayName := extractModuleDisplayName(moduleName)

			// Determine layer styling
			layer := determineModuleLayer(moduleEntities)
			layerColor := GetD2LayerColor(layer)

			sb.WriteString(fmt.Sprintf("%s: {\n", safeModuleName))
			sb.WriteString(fmt.Sprintf("  label: %q\n", displayName))
			sb.WriteString("  style: {\n")
			sb.WriteString(fmt.Sprintf("    fill: %q\n", layerColor.Fill))
			sb.WriteString(fmt.Sprintf("    stroke: %q\n", layerColor.Stroke))
			sb.WriteString("    border-radius: 8\n")
			sb.WriteString("  }\n")

			// Write entities within container
			for _, entity := range moduleEntities {
				g.writeNodeInContainer(sb, entity, "  ")
				sb.WriteString("\n")
			}

			sb.WriteString("}\n\n")
		}
	}

	// Write edges with module-qualified paths
	sb.WriteString("# Connections\n")
	for _, dep := range deps {
		g.writeEdgeArchitecture(sb, dep, entities)
		sb.WriteString("\n")
	}
}

// generateCallFlow generates a sequential call flow diagram.
func (g *D2Generator) generateCallFlow(sb *strings.Builder, entities []DiagramEntity, deps []DiagramEdge) {
	// For call flow, use vertical direction by default
	// Entities are shown in a sequence

	// Sort entities by their appearance in dependencies to get flow order
	entityOrder := buildEntityOrder(entities, deps)

	sb.WriteString("# Call Flow\n")
	for _, entity := range entityOrder {
		g.writeNode(sb, entity)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Write edges - for call flow, all edges are styled as calls
	sb.WriteString("# Flow\n")
	for _, dep := range deps {
		g.writeEdgeCallFlow(sb, dep)
		sb.WriteString("\n")
	}
}

// generateCoverage generates a coverage heatmap diagram.
func (g *D2Generator) generateCoverage(sb *strings.Builder, entities []DiagramEntity, deps []DiagramEdge) {
	// Sort entities by coverage (lowest first to highlight risk)
	sortedEntities := make([]DiagramEntity, len(entities))
	copy(sortedEntities, entities)
	sort.Slice(sortedEntities, func(i, j int) bool {
		return sortedEntities[i].Coverage < sortedEntities[j].Coverage
	})

	// Group by coverage level
	sb.WriteString("# Coverage Analysis\n")

	// Write legend
	g.writeCoverageLegend(sb)
	sb.WriteString("\n")

	// Write nodes with coverage styling
	for _, entity := range sortedEntities {
		g.writeNodeCoverage(sb, entity)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Write edges
	sb.WriteString("# Dependencies\n")
	for _, dep := range deps {
		g.writeEdge(sb, dep)
		sb.WriteString("\n")
	}
}

// writeNode writes a single entity node with full design system styling.
func (g *D2Generator) writeNode(sb *strings.Builder, entity DiagramEntity) {
	safeID := sanitizeD2ID(entity.ID)
	displayName := entity.Name
	if displayName == "" {
		displayName = extractNodeName(entity.ID)
	}

	// Get complete styling from design system
	coverage := entity.Coverage
	if coverage < 0 {
		coverage = -1 // Mark as unknown
	}
	style := GetD2NodeStyle(entity.Type, entity.Importance, coverage, entity.Language)

	// Apply change state styling if present (for before/after diagrams)
	if entity.ChangeState != "" {
		ApplyChangeStateStyle(&style, entity.ChangeState)
	}

	sb.WriteString(fmt.Sprintf("%s: {\n", safeID))
	sb.WriteString(fmt.Sprintf("  label: %q\n", displayName))
	sb.WriteString(fmt.Sprintf("  shape: %s\n", style.Shape))

	// Add icon if enabled and available
	if g.config.ShowIcons && style.Icon != "" {
		sb.WriteString(fmt.Sprintf("  icon: %s\n", style.Icon))
	}

	// Write style block
	styleStr := D2StyleToString(style)
	if styleStr != "" {
		sb.WriteString("  ")
		sb.WriteString(styleStr)
		sb.WriteString("\n")
	}

	sb.WriteString("}")
}

// writeNodeInContainer writes a node inside a container (with indentation).
func (g *D2Generator) writeNodeInContainer(sb *strings.Builder, entity DiagramEntity, indent string) {
	// Use just the entity name as ID within container
	safeID := sanitizeD2ID(extractNodeName(entity.ID))
	displayName := entity.Name
	if displayName == "" {
		displayName = extractNodeName(entity.ID)
	}

	coverage := entity.Coverage
	if coverage < 0 {
		coverage = -1
	}
	style := GetD2NodeStyle(entity.Type, entity.Importance, coverage, entity.Language)

	// Apply change state styling if present (for before/after diagrams)
	if entity.ChangeState != "" {
		ApplyChangeStateStyle(&style, entity.ChangeState)
	}

	sb.WriteString(fmt.Sprintf("%s%s: {\n", indent, safeID))
	sb.WriteString(fmt.Sprintf("%s  label: %q\n", indent, displayName))
	sb.WriteString(fmt.Sprintf("%s  shape: %s\n", indent, style.Shape))

	if g.config.ShowIcons && style.Icon != "" {
		sb.WriteString(fmt.Sprintf("%s  icon: %s\n", indent, style.Icon))
	}

	// Inline style
	sb.WriteString(fmt.Sprintf("%s  style: {\n", indent))
	if style.Fill != "" {
		sb.WriteString(fmt.Sprintf("%s    fill: %q\n", indent, style.Fill))
	}
	if style.Stroke != "" {
		sb.WriteString(fmt.Sprintf("%s    stroke: %q\n", indent, style.Stroke))
	}
	if style.StrokeWidth > 0 {
		sb.WriteString(fmt.Sprintf("%s    stroke-width: %d\n", indent, style.StrokeWidth))
	}
	if style.StrokeDash > 0 {
		sb.WriteString(fmt.Sprintf("%s    stroke-dash: %d\n", indent, style.StrokeDash))
	}
	if style.BorderRadius > 0 {
		sb.WriteString(fmt.Sprintf("%s    border-radius: %d\n", indent, style.BorderRadius))
	}
	if style.Shadow {
		sb.WriteString(fmt.Sprintf("%s    shadow: true\n", indent))
	}
	sb.WriteString(fmt.Sprintf("%s  }\n", indent))

	sb.WriteString(fmt.Sprintf("%s}", indent))
}

// writeNodeCoverage writes a node with coverage-specific styling.
func (g *D2Generator) writeNodeCoverage(sb *strings.Builder, entity DiagramEntity) {
	safeID := sanitizeD2ID(entity.ID)
	displayName := entity.Name
	if displayName == "" {
		displayName = extractNodeName(entity.ID)
	}

	// Add coverage to label
	coverageLabel := displayName
	if entity.Coverage >= 0 {
		coverageLabel = fmt.Sprintf("%s\\n%.0f%% coverage", displayName, entity.Coverage)
	} else {
		coverageLabel = fmt.Sprintf("%s\\nNo coverage data", displayName)
	}

	// Get coverage color
	coverageColor := GetCoverageColor(entity.Coverage)
	coverageLevel := GetCoverageLevel(entity.Coverage)

	sb.WriteString(fmt.Sprintf("%s: {\n", safeID))
	sb.WriteString(fmt.Sprintf("  label: %q\n", coverageLabel))
	sb.WriteString("  shape: rectangle\n")

	// Add warning icon for low/no coverage on important entities
	if g.config.ShowIcons {
		if entity.Coverage < 50 && (entity.Importance == "keystone" || entity.Importance == "bottleneck") {
			sb.WriteString(fmt.Sprintf("  icon: %s\n", D2StatusIcons["warning"]))
		} else if entity.Coverage >= 80 {
			sb.WriteString(fmt.Sprintf("  icon: %s\n", D2StatusIcons["success"]))
		}
	}

	sb.WriteString("  style: {\n")
	sb.WriteString(fmt.Sprintf("    fill: %q\n", coverageColor.Fill))
	sb.WriteString(fmt.Sprintf("    stroke: %q\n", coverageColor.Stroke))
	sb.WriteString("    border-radius: 4\n")
	if coverageLevel == "none" {
		sb.WriteString("    stroke-dash: 3\n")
	}
	// Emphasize important entities
	if entity.Importance == "keystone" {
		sb.WriteString("    stroke-width: 3\n")
		sb.WriteString("    shadow: true\n")
	} else if entity.Importance == "bottleneck" || entity.Importance == "high-fan-in" {
		sb.WriteString("    stroke-width: 2\n")
	}
	sb.WriteString("  }\n")

	sb.WriteString("}")
}

// writeEdge writes a dependency edge with design system styling.
func (g *D2Generator) writeEdge(sb *strings.Builder, dep DiagramEdge) {
	safeFrom := sanitizeD2ID(dep.From)
	safeTo := sanitizeD2ID(dep.To)

	edgeStyle := GetD2EdgeStyle(dep.Type)

	// Build edge line
	var edge strings.Builder
	edge.WriteString(safeFrom)
	edge.WriteString(" ")
	edge.WriteString(edgeStyle.Arrow)
	edge.WriteString(" ")
	edge.WriteString(safeTo)

	// Add label if configured and provided
	if g.config.ShowLabels && dep.Label != "" {
		edge.WriteString(": ")
		edge.WriteString(dep.Label)
	}

	// Add style block for non-default edges
	styleStr := D2EdgeStyleToString(edgeStyle)
	if styleStr != "" {
		edge.WriteString(" {\n  ")
		edge.WriteString(styleStr)
		edge.WriteString("\n}")
	}

	sb.WriteString(edge.String())
}

// writeEdgeArchitecture writes an edge for architecture diagrams with module paths.
func (g *D2Generator) writeEdgeArchitecture(sb *strings.Builder, dep DiagramEdge, entities []DiagramEntity) {
	// Find the entities to get their modules
	var fromModule, toModule string
	for _, e := range entities {
		if e.ID == dep.From {
			fromModule = e.Module
		}
		if e.ID == dep.To {
			toModule = e.Module
		}
	}

	// Build qualified paths
	var fromPath, toPath string
	if fromModule != "" && fromModule != "_root" {
		fromPath = sanitizeD2ID(fromModule) + "." + sanitizeD2ID(extractNodeName(dep.From))
	} else {
		fromPath = sanitizeD2ID(dep.From)
	}
	if toModule != "" && toModule != "_root" {
		toPath = sanitizeD2ID(toModule) + "." + sanitizeD2ID(extractNodeName(dep.To))
	} else {
		toPath = sanitizeD2ID(dep.To)
	}

	edgeStyle := GetD2EdgeStyle(dep.Type)

	var edge strings.Builder
	edge.WriteString(fromPath)
	edge.WriteString(" ")
	edge.WriteString(edgeStyle.Arrow)
	edge.WriteString(" ")
	edge.WriteString(toPath)

	if g.config.ShowLabels && dep.Label != "" {
		edge.WriteString(": ")
		edge.WriteString(dep.Label)
	}

	styleStr := D2EdgeStyleToString(edgeStyle)
	if styleStr != "" {
		edge.WriteString(" {\n  ")
		edge.WriteString(styleStr)
		edge.WriteString("\n}")
	}

	sb.WriteString(edge.String())
}

// writeEdgeCallFlow writes an edge styled for call flow diagrams.
func (g *D2Generator) writeEdgeCallFlow(sb *strings.Builder, dep DiagramEdge) {
	safeFrom := sanitizeD2ID(dep.From)
	safeTo := sanitizeD2ID(dep.To)

	// Use call styling for call flow
	edgeStyle := D2EdgeStyles["calls"]

	var edge strings.Builder
	edge.WriteString(safeFrom)
	edge.WriteString(" -> ")
	edge.WriteString(safeTo)

	if dep.Label != "" {
		edge.WriteString(": ")
		edge.WriteString(dep.Label)
	}

	// Call flow edges get consistent styling
	edge.WriteString(" {\n")
	edge.WriteString("  style: {\n")
	edge.WriteString(fmt.Sprintf("    stroke: %q\n", edgeStyle.StrokeColor))
	edge.WriteString("    stroke-width: 1\n")
	edge.WriteString("  }\n")
	edge.WriteString("}")

	sb.WriteString(edge.String())
}

// writeCoverageLegend writes a legend for coverage diagrams.
func (g *D2Generator) writeCoverageLegend(sb *strings.Builder) {
	sb.WriteString("legend: {\n")
	sb.WriteString("  label: \"Coverage Legend\"\n")
	sb.WriteString("  style: {\n")
	sb.WriteString("    fill: \"#fafafa\"\n")
	sb.WriteString("    stroke: \"#e0e0e0\"\n")
	sb.WriteString("    border-radius: 8\n")
	sb.WriteString("  }\n")
	sb.WriteString("\n")

	// High coverage
	highColor := D2CoverageColors["high"]
	sb.WriteString("  high: {\n")
	sb.WriteString("    label: \">80%\"\n")
	sb.WriteString("    style: {\n")
	sb.WriteString(fmt.Sprintf("      fill: %q\n", highColor.Fill))
	sb.WriteString(fmt.Sprintf("      stroke: %q\n", highColor.Stroke))
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")

	// Medium coverage
	medColor := D2CoverageColors["medium"]
	sb.WriteString("  medium: {\n")
	sb.WriteString("    label: \"50-80%\"\n")
	sb.WriteString("    style: {\n")
	sb.WriteString(fmt.Sprintf("      fill: %q\n", medColor.Fill))
	sb.WriteString(fmt.Sprintf("      stroke: %q\n", medColor.Stroke))
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")

	// Low coverage
	lowColor := D2CoverageColors["low"]
	sb.WriteString("  low: {\n")
	sb.WriteString("    label: \"<50%\"\n")
	sb.WriteString("    style: {\n")
	sb.WriteString(fmt.Sprintf("      fill: %q\n", lowColor.Fill))
	sb.WriteString(fmt.Sprintf("      stroke: %q\n", lowColor.Stroke))
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")

	// No coverage
	noneColor := D2CoverageColors["none"]
	sb.WriteString("  none: {\n")
	sb.WriteString("    label: \"0%\"\n")
	sb.WriteString("    style: {\n")
	sb.WriteString(fmt.Sprintf("      fill: %q\n", noneColor.Fill))
	sb.WriteString(fmt.Sprintf("      stroke: %q\n", noneColor.Stroke))
	sb.WriteString("      stroke-dash: 3\n")
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")

	sb.WriteString("}\n")
}

// Helper functions

// extractModuleDisplayName extracts a display name from a module path.
func extractModuleDisplayName(module string) string {
	parts := strings.Split(module, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return module
}

// determineModuleLayer determines the architectural layer of a module based on its entities.
func determineModuleLayer(entities []DiagramEntity) string {
	layers := make(map[string]int)
	for _, e := range entities {
		if e.Layer != "" {
			layers[e.Layer]++
		} else if e.Type == "http" || e.Type == "handler" {
			layers["api"]++
		} else if e.Type == "database" || e.Type == "storage" {
			layers["data"]++
		} else if strings.Contains(e.Type, "struct") || strings.Contains(e.Type, "type") {
			layers["domain"]++
		} else {
			layers["service"]++
		}
	}

	// Return the most common layer
	maxCount := 0
	maxLayer := "default"
	for layer, count := range layers {
		if count > maxCount {
			maxCount = count
			maxLayer = layer
		}
	}
	return maxLayer
}

// buildEntityOrder orders entities based on their appearance in the call flow.
func buildEntityOrder(entities []DiagramEntity, deps []DiagramEdge) []DiagramEntity {
	// Build adjacency list
	outgoing := make(map[string][]string)
	incoming := make(map[string]int)
	entityMap := make(map[string]DiagramEntity)

	for _, e := range entities {
		entityMap[e.ID] = e
		incoming[e.ID] = 0
	}

	for _, dep := range deps {
		outgoing[dep.From] = append(outgoing[dep.From], dep.To)
		incoming[dep.To]++
	}

	// Find root nodes (no incoming)
	var roots []string
	for id, count := range incoming {
		if count == 0 {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)

	// Topological sort
	var result []DiagramEntity
	visited := make(map[string]bool)

	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		if entity, ok := entityMap[id]; ok {
			result = append(result, entity)
		}
		targets := outgoing[id]
		sort.Strings(targets)
		for _, target := range targets {
			visit(target)
		}
	}

	for _, root := range roots {
		visit(root)
	}

	// Add any unvisited entities
	for _, e := range entities {
		if !visited[e.ID] {
			result = append(result, e)
		}
	}

	return result
}

// sanitizeD2ID makes an ID safe for D2 by quoting if necessary.
func sanitizeD2ID(id string) string {
	// Check if ID contains special characters that need quoting
	needsQuoting := false
	for _, c := range id {
		if !isAlphanumeric(c) && c != '_' && c != '-' {
			needsQuoting = true
			break
		}
	}

	if needsQuoting {
		// Escape any quotes in the ID and wrap in quotes
		escaped := strings.ReplaceAll(id, "\"", "\\\"")
		return fmt.Sprintf("\"%s\"", escaped)
	}
	return id
}

// isAlphanumeric returns true if the rune is a letter or digit.
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// extractNodeName extracts a short display name from a full entity ID.
func extractNodeName(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return id
}

// =============================================================================
// LEGACY API COMPATIBILITY
// =============================================================================
// These functions maintain backwards compatibility with the old API.

// D2Options configures D2 diagram generation (legacy).
type D2Options struct {
	MaxNodes   int
	Direction  string
	ShowLabels bool
	Collapse   bool
	Title      string
}

// DefaultD2Options returns sensible defaults for D2 generation (legacy).
func DefaultD2Options() *D2Options {
	return &D2Options{
		MaxNodes:   30,
		Direction:  "right",
		ShowLabels: true,
		Collapse:   true,
		Title:      "",
	}
}

// GenerateD2 generates a D2 diagram from graph nodes and edges (legacy API).
// Takes nodes map and edges slice from GraphOutput.
// Returns D2 diagram as string.
func GenerateD2(nodes map[string]*output.GraphNode, edges [][]string, opts *D2Options) string {
	if opts == nil {
		opts = DefaultD2Options()
	}

	// Convert to new format
	entities := make([]DiagramEntity, 0, len(nodes))
	for id, node := range nodes {
		entities = append(entities, DiagramEntity{
			ID:         id,
			Name:       extractNodeName(id),
			Type:       node.Type,
			Importance: "normal",
			Coverage:   -1, // Unknown
			Language:   "",
			Module:     "",
		})
	}

	deps := make([]DiagramEdge, 0, len(edges))
	for _, edge := range edges {
		if len(edge) < 2 {
			continue
		}
		depType := "calls"
		if len(edge) >= 3 {
			depType = edge[2]
		}
		deps = append(deps, DiagramEdge{
			From: edge[0],
			To:   edge[1],
			Type: depType,
		})
	}

	// Create generator with legacy options
	config := &DiagramConfig{
		Type:       DiagramDeps,
		Theme:      "default",
		Layout:     "elk",
		Direction:  opts.Direction,
		MaxNodes:   opts.MaxNodes,
		Collapse:   opts.Collapse,
		ShowLabels: opts.ShowLabels,
		ShowIcons:  true,
		Title:      opts.Title,
	}

	gen := NewD2Generator(config)
	return gen.Generate(entities, deps)
}

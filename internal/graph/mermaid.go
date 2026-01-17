package graph

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/output"
)

// MermaidOptions configures Mermaid diagram generation.
type MermaidOptions struct {
	MaxNodes  int    // Maximum nodes before auto-collapsing (default: 30)
	Direction string // Layout direction: "TD" (top-down) or "LR" (left-right)
	ChartType string // Chart type: "flowchart", "classDiagram", "pie"
	Collapse  bool   // Auto-collapse to modules when > MaxNodes
	Title     string // Optional diagram title
}

// DefaultMermaidOptions returns sensible defaults for Mermaid diagram generation.
func DefaultMermaidOptions() *MermaidOptions {
	return &MermaidOptions{
		MaxNodes:  30,
		Direction: "LR",
		ChartType: "flowchart",
		Collapse:  true,
		Title:     "",
	}
}

// GenerateMermaid generates a Mermaid diagram from graph nodes and edges.
// Nodes are keyed by ID with GraphNode values containing type and location info.
// Edges are tuples of [from, to, edgeType].
func GenerateMermaid(nodes map[string]*output.GraphNode, edges [][]string, opts *MermaidOptions) string {
	if opts == nil {
		opts = DefaultMermaidOptions()
	}

	// Validate options
	if opts.MaxNodes <= 0 {
		opts.MaxNodes = 30
	}
	if opts.Direction != "TD" && opts.Direction != "LR" {
		opts.Direction = "LR"
	}
	if opts.ChartType == "" {
		opts.ChartType = "flowchart"
	}

	var sb strings.Builder

	// Start diagram
	sb.WriteString(fmt.Sprintf("%s %s\n", opts.ChartType, opts.Direction))

	// Add title if specified
	if opts.Title != "" {
		sb.WriteString(fmt.Sprintf("    subgraph title[\"%s\"]\n", escapeMermaidString(opts.Title)))
		sb.WriteString("    end\n")
	}

	// Check if we need to collapse to modules
	if opts.Collapse && len(nodes) > opts.MaxNodes {
		return generateCollapsedMermaid(nodes, edges, opts, &sb)
	}

	// Generate node declarations
	nodeIDs := sortedNodeIDs(nodes)
	for _, id := range nodeIDs {
		node := nodes[id]
		name := extractNodeName(id)
		entityType := node.Type
		nodeStr := generateMermaidNode(sanitizeMermaidID(id), name, entityType)
		sb.WriteString(fmt.Sprintf("    %s\n", nodeStr))
	}

	// Generate edge declarations
	for _, edge := range edges {
		if len(edge) < 2 {
			continue
		}
		from := edge[0]
		to := edge[1]
		edgeType := ""
		if len(edge) >= 3 {
			edgeType = edge[2]
		}
		edgeStr := generateMermaidEdge(sanitizeMermaidID(from), sanitizeMermaidID(to), edgeType)
		sb.WriteString(fmt.Sprintf("    %s\n", edgeStr))
	}

	return sb.String()
}

// generateCollapsedMermaid generates a module-level view when there are too many nodes.
func generateCollapsedMermaid(nodes map[string]*output.GraphNode, edges [][]string, opts *MermaidOptions, sb *strings.Builder) string {
	// Group nodes by module (extracted from location path)
	modules := make(map[string][]string)
	nodeToModule := make(map[string]string)

	for id, node := range nodes {
		module := extractModule(node.Location)
		if module == "" {
			module = "root"
		}
		modules[module] = append(modules[module], id)
		nodeToModule[id] = module
	}

	// Generate module nodes
	sortedModules := make([]string, 0, len(modules))
	for m := range modules {
		sortedModules = append(sortedModules, m)
	}
	sort.Strings(sortedModules)

	for _, module := range sortedModules {
		count := len(modules[module])
		label := fmt.Sprintf("%s (%d)", module, count)
		nodeStr := generateMermaidNode(sanitizeMermaidID(module), label, "module")
		sb.WriteString(fmt.Sprintf("    %s\n", nodeStr))
	}

	// Generate module-level edges (deduplicated)
	moduleEdges := make(map[string]bool)
	for _, edge := range edges {
		if len(edge) < 2 {
			continue
		}
		fromModule := nodeToModule[edge[0]]
		toModule := nodeToModule[edge[1]]
		if fromModule == "" || toModule == "" || fromModule == toModule {
			continue
		}
		edgeKey := fmt.Sprintf("%s->%s", fromModule, toModule)
		if !moduleEdges[edgeKey] {
			moduleEdges[edgeKey] = true
			edgeStr := generateMermaidEdge(sanitizeMermaidID(fromModule), sanitizeMermaidID(toModule), "")
			sb.WriteString(fmt.Sprintf("    %s\n", edgeStr))
		}
	}

	return sb.String()
}

// generateMermaidNode creates a Mermaid node declaration with appropriate shape.
func generateMermaidNode(id, name string, entityType string) string {
	shape := GetEntityShape(entityType)
	mermaidShape := shape.MermaidShape

	// Parse the shape to extract prefix and suffix
	// Shapes are like: "[]", "{{}}", "{()}", "([])", "[/\\]", "[()]"
	escapedName := escapeMermaidString(name)

	switch mermaidShape {
	case "[]":
		return fmt.Sprintf("%s[\"%s\"]", id, escapedName)
	case "{{}}":
		return fmt.Sprintf("%s{{\"%s\"}}", id, escapedName)
	case "{()}":
		return fmt.Sprintf("%s{(\"%s\")}", id, escapedName)
	case "([])":
		return fmt.Sprintf("%s([\"%s\"])", id, escapedName)
	case "[/\\]":
		return fmt.Sprintf("%s[/\"%s\"\\]", id, escapedName)
	case "[()]":
		return fmt.Sprintf("%s[(\"%s\")]", id, escapedName)
	default:
		// Default to rectangle
		return fmt.Sprintf("%s[\"%s\"]", id, escapedName)
	}
}

// generateMermaidEdge creates a Mermaid edge declaration with appropriate style.
func generateMermaidEdge(from, to, edgeType string) string {
	style := GetEdgeStyle(edgeType)
	return fmt.Sprintf("%s %s %s", from, style.MermaidStyle, to)
}

// sanitizeMermaidID converts an ID to be valid in Mermaid.
// Mermaid IDs can contain alphanumeric chars and underscores.
var mermaidIDRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeMermaidID(id string) string {
	// Replace invalid characters with underscores
	sanitized := mermaidIDRegex.ReplaceAllString(id, "_")

	// Ensure it starts with a letter or underscore (not a digit)
	if len(sanitized) > 0 && sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "_" + sanitized
	}

	// Handle empty strings
	if sanitized == "" {
		sanitized = "_empty"
	}

	return sanitized
}

// escapeMermaidString escapes special characters in Mermaid string content.
func escapeMermaidString(s string) string {
	// Escape quotes and other special characters
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	return s
}

// extractModule extracts the module/package path from a location string.
// Location format is typically "path/to/file.go:line-line"
func extractModule(location string) string {
	// Remove line number part
	colonIdx := strings.LastIndex(location, ":")
	if colonIdx > 0 {
		location = location[:colonIdx]
	}

	// Extract directory path
	slashIdx := strings.LastIndex(location, "/")
	if slashIdx > 0 {
		return location[:slashIdx]
	}

	return location
}

// sortedNodeIDs returns node IDs sorted alphabetically for deterministic output.
func sortedNodeIDs(nodes map[string]*output.GraphNode) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// GeneratePieChart generates a Mermaid pie chart from statistics.
// The stats map contains category names as keys and counts as values.
func GeneratePieChart(stats map[string]int, title string) string {
	var sb strings.Builder

	// Start pie chart
	if title != "" {
		sb.WriteString(fmt.Sprintf("pie title %s\n", escapeMermaidString(title)))
	} else {
		sb.WriteString("pie\n")
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Generate pie slices
	for _, key := range keys {
		value := stats[key]
		sb.WriteString(fmt.Sprintf("    \"%s\" : %d\n", escapeMermaidString(key), value))
	}

	return sb.String()
}

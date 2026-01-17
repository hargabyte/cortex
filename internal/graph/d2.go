package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/output"
)

// D2Options configures D2 diagram generation.
type D2Options struct {
	MaxNodes   int    // Maximum nodes before auto-collapsing (default: 30)
	Direction  string // Layout direction: "right" or "down"
	ShowLabels bool   // Whether to show edge labels
	Collapse   bool   // Auto-collapse to modules when > MaxNodes
	Title      string // Optional diagram title
}

// DefaultD2Options returns sensible defaults for D2 generation.
func DefaultD2Options() *D2Options {
	return &D2Options{
		MaxNodes:   30,
		Direction:  "right",
		ShowLabels: true,
		Collapse:   true,
		Title:      "",
	}
}

// GenerateD2 generates a D2 diagram from graph nodes and edges.
// Takes nodes map and edges slice from GraphOutput.
// Returns D2 diagram as string.
func GenerateD2(nodes map[string]*output.GraphNode, edges [][]string, opts *D2Options) string {
	if opts == nil {
		opts = DefaultD2Options()
	}

	var sb strings.Builder

	// Write direction
	sb.WriteString(fmt.Sprintf("direction: %s\n", opts.Direction))

	// Write title if provided
	if opts.Title != "" {
		sb.WriteString(fmt.Sprintf("title: {\n  label: %s\n  near: top-center\n}\n", opts.Title))
	}

	sb.WriteString("\n")

	// Sort node IDs for deterministic output
	nodeIDs := make([]string, 0, len(nodes))
	for id := range nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Write nodes
	sb.WriteString("# Nodes\n")
	for _, id := range nodeIDs {
		node := nodes[id]
		// Extract importance from the node if available (not directly in GraphNode, so we default to "normal")
		importance := "normal"
		nodeD2 := generateD2Node(id, extractNodeName(id), node.Type, importance)
		sb.WriteString(nodeD2)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Write edges
	sb.WriteString("# Edges\n")
	for _, edge := range edges {
		if len(edge) < 2 {
			continue
		}
		from := edge[0]
		to := edge[1]
		edgeType := "calls"
		if len(edge) >= 3 {
			edgeType = edge[2]
		}
		edgeD2 := generateD2Edge(from, to, edgeType, opts.ShowLabels)
		sb.WriteString(edgeD2)
		sb.WriteString("\n")
	}

	return sb.String()
}

// generateD2Node generates a D2 node definition.
func generateD2Node(id, name string, entityType string, importance string) string {
	shape := GetEntityShape(entityType)
	impStyle := GetImportanceStyle(importance)

	var sb strings.Builder

	// Sanitize ID for D2 (replace special chars with underscores)
	safeID := sanitizeD2ID(id)

	sb.WriteString(fmt.Sprintf("%s: {\n", safeID))
	sb.WriteString(fmt.Sprintf("  label: \"%s\"\n", name))
	sb.WriteString(fmt.Sprintf("  shape: %s\n", shape.D2Shape))

	// Add importance styling if not normal
	if impStyle.D2Style != "" {
		sb.WriteString("  style: {\n")
		sb.WriteString(fmt.Sprintf("    %s\n", impStyle.D2Style))
		sb.WriteString("  }\n")
	}

	sb.WriteString("}")

	return sb.String()
}

// generateD2Edge generates a D2 edge definition.
func generateD2Edge(from, to, edgeType string, showLabel bool) string {
	style := GetEdgeStyle(edgeType)

	safeFrom := sanitizeD2ID(from)
	safeTo := sanitizeD2ID(to)

	if showLabel {
		return fmt.Sprintf("%s %s %s: %s", safeFrom, style.D2Style, safeTo, edgeType)
	}
	return fmt.Sprintf("%s %s %s", safeFrom, style.D2Style, safeTo)
}

// sanitizeD2ID makes an ID safe for D2 by quoting if necessary.
// D2 IDs with special characters need to be quoted.
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
// Entity IDs are often in the format "package.Type.Method" or similar.
func extractNodeName(id string) string {
	// Try to extract just the last part of the ID for display
	// This handles cases like "internal/cmd.runFind" -> "runFind"
	// or "store.Store.GetEntity" -> "GetEntity"
	parts := strings.Split(id, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return id
}

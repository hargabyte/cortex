// Package output provides formatters for different output formats.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Formatter is the interface for formatting entity output in different formats.
type Formatter interface {
	// Format formats an entity according to the specified density level.
	// Returns the formatted string or an error.
	Format(entity interface{}, density Density) (string, error)

	// FormatToWriter writes formatted output directly to a writer.
	FormatToWriter(w io.Writer, entity interface{}, density Density) error
}

// YAMLFormatter formats entities as YAML output.
type YAMLFormatter struct{}

// NewYAMLFormatter creates a new YAML formatter.
func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

// Format formats an entity as YAML.
func (f *YAMLFormatter) Format(entity interface{}, density Density) (string, error) {
	var buf bytes.Buffer
	if err := f.FormatToWriter(&buf, entity, density); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FormatToWriter writes YAML output to a writer.
func (f *YAMLFormatter) FormatToWriter(w io.Writer, entity interface{}, density Density) error {
	// Apply density filtering to the entity before marshaling
	filtered := applyDensityFilter(entity, density)

	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	defer encoder.Close()

	return encoder.Encode(filtered)
}

// JSONFormatter formats entities as JSON output.
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format formats an entity as JSON.
func (f *JSONFormatter) Format(entity interface{}, density Density) (string, error) {
	var buf bytes.Buffer
	if err := f.FormatToWriter(&buf, entity, density); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FormatToWriter writes JSON output to a writer.
func (f *JSONFormatter) FormatToWriter(w io.Writer, entity interface{}, density Density) error {
	// Apply density filtering to the entity before marshaling
	filtered := applyDensityFilter(entity, density)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(filtered)
}

// CGFFormatter formats entities in the deprecated CGF (Cortex Graph Format).
// This formatter is maintained for backward compatibility but emits a deprecation warning.
type CGFFormatter struct {
	warnedOnce bool
}

// NewCGFFormatter creates a new CGF formatter.
func NewCGFFormatter() *CGFFormatter {
	return &CGFFormatter{}
}

// Format formats an entity as CGF (deprecated).
func (f *CGFFormatter) Format(entity interface{}, density Density) (string, error) {
	f.emitDeprecationWarning()

	var buf bytes.Buffer
	if err := f.FormatToWriter(&buf, entity, density); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FormatToWriter writes CGF output to a writer (deprecated).
func (f *CGFFormatter) FormatToWriter(w io.Writer, entity interface{}, density Density) error {
	f.emitDeprecationWarning()

	// Map output density to CGF density
	cgfDensity := mapDensityToCGF(density)

	// Format the entity based on its type
	switch v := entity.(type) {
	case *EntityOutput:
		return f.writeEntityOutput(w, v, cgfDensity)
	case *ListOutput:
		return f.writeListOutput(w, v, cgfDensity)
	case *GraphOutput:
		return f.writeGraphOutput(w, v, cgfDensity)
	case *ImpactOutput:
		return f.writeImpactOutput(w, v, cgfDensity)
	case *ContextOutput:
		return f.writeContextOutput(w, v, cgfDensity)
	default:
		return fmt.Errorf("CGF formatter does not support type %T", entity)
	}
}

// emitDeprecationWarning prints a deprecation warning to stderr once per formatter instance.
func (f *CGFFormatter) emitDeprecationWarning() {
	if !f.warnedOnce {
		fmt.Fprintln(os.Stderr, "Warning: CGF format is deprecated and will be removed in v2.0.")
		fmt.Fprintln(os.Stderr, "Use --format=yaml (default) or --format=json.")
		f.warnedOnce = true
	}
}

// mapDensityToCGF maps output.Density to CGF density levels.
func mapDensityToCGF(d Density) string {
	switch d {
	case DensitySparse:
		return "sparse"
	case DensityMedium, DensitySmart:
		return "medium"
	case DensityDense:
		return "dense"
	default:
		return "medium"
	}
}

// writeEntityOutput writes an EntityOutput in CGF format.
func (f *CGFFormatter) writeEntityOutput(w io.Writer, entity *EntityOutput, density string) error {
	// Write CGF header
	fmt.Fprintf(w, "#cgf v1 d=%s\n", density)

	// Write entity marker and basic info
	marker := mapTypeToCGFMarker(entity.Type)
	fmt.Fprintf(w, "%s %s %s", marker, entity.Location, extractEntityName(entity))

	// Add signature for medium/dense
	if density != "sparse" && entity.Signature != "" {
		fmt.Fprintf(w, " %s", entity.Signature)
	}
	fmt.Fprintln(w)

	// Write edges for medium/dense
	if density != "sparse" && entity.Dependencies != nil {
		f.writeEntityEdges(w, entity.Dependencies)
	}

	return nil
}

// writeListOutput writes a ListOutput in CGF format.
func (f *CGFFormatter) writeListOutput(w io.Writer, list *ListOutput, density string) error {
	// Write CGF header
	fmt.Fprintf(w, "#cgf v1 d=%s\n", density)
	fmt.Fprintf(w, "; === RESULTS (count=%d) ===\n\n", list.Count)

	// Write each entity
	for name, entity := range list.Results {
		marker := mapTypeToCGFMarker(entity.Type)
		fmt.Fprintf(w, "%s %s %s", marker, entity.Location, name)

		if density != "sparse" && entity.Signature != "" {
			fmt.Fprintf(w, " %s", entity.Signature)
		}
		fmt.Fprintln(w)

		// Write edges for medium/dense
		if density != "sparse" && entity.Dependencies != nil {
			f.writeEntityEdges(w, entity.Dependencies)
		}
	}

	return nil
}

// writeGraphOutput writes a GraphOutput in CGF format.
func (f *CGFFormatter) writeGraphOutput(w io.Writer, graph *GraphOutput, density string) error {
	// Write CGF header
	fmt.Fprintf(w, "#cgf v1 d=%s\n", density)
	if graph.Graph != nil {
		fmt.Fprintf(w, "; === GRAPH: %s (direction=%s depth=%d) ===\n\n",
			graph.Graph.Root, graph.Graph.Direction, graph.Graph.Depth)
	}

	// Write nodes
	for name, node := range graph.Nodes {
		marker := mapTypeToCGFMarker(node.Type)
		fmt.Fprintf(w, "%s %s %s", marker, node.Location, name)

		if density != "sparse" && node.Signature != "" {
			fmt.Fprintf(w, " %s", node.Signature)
		}
		fmt.Fprintln(w)
	}

	// Write edges section
	if len(graph.Edges) > 0 {
		fmt.Fprintln(w, "\n; === EDGES ===")
		for _, edge := range graph.Edges {
			if len(edge) >= 3 {
				fmt.Fprintf(w, "%s %s %s\n", edge[0], mapEdgeTypeToCGF(edge[2]), edge[1])
			}
		}
	}

	return nil
}

// writeImpactOutput writes an ImpactOutput in CGF format.
func (f *CGFFormatter) writeImpactOutput(w io.Writer, impact *ImpactOutput, density string) error {
	// Write CGF header
	fmt.Fprintf(w, "#cgf v1 d=%s\n", density)
	if impact.Impact != nil {
		fmt.Fprintf(w, "; === IMPACT: %s ===\n", impact.Impact.Target)
	}

	// Write summary
	if impact.Summary != nil {
		fmt.Fprintf(w, "; Risk: %s | Files: %d | Entities: %d\n\n",
			impact.Summary.RiskLevel,
			impact.Summary.FilesAffected,
			impact.Summary.EntitiesAffected)
	}

	// Write affected entities
	for name, entity := range impact.Affected {
		marker := mapTypeToCGFMarker(entity.Type)
		fmt.Fprintf(w, "%s %s %s", marker, entity.Location, name)

		if density != "sparse" {
			fmt.Fprintf(w, " ; impact=%s", entity.Impact)
			if entity.Reason != "" {
				fmt.Fprintf(w, " (%s)", entity.Reason)
			}
		}
		fmt.Fprintln(w)
	}

	// Write recommendations
	if len(impact.Recommendations) > 0 {
		fmt.Fprintln(w, "\n; === RECOMMENDATIONS ===")
		for _, rec := range impact.Recommendations {
			fmt.Fprintf(w, "; - %s\n", rec)
		}
	}

	return nil
}

// writeContextOutput writes a ContextOutput in CGF format.
func (f *CGFFormatter) writeContextOutput(w io.Writer, ctx *ContextOutput, density string) error {
	// Write CGF header
	fmt.Fprintf(w, "#cgf v1 d=%s\n", density)
	if ctx.Context != nil {
		fmt.Fprintf(w, "; === CONTEXT: %s ===\n", ctx.Context.Target)
		fmt.Fprintf(w, "; Budget: %d tokens | Used: %d tokens\n\n",
			ctx.Context.Budget, ctx.Context.TokensUsed)
	}

	// Write entry points
	if len(ctx.EntryPoints) > 0 {
		fmt.Fprintln(w, "; === ENTRY POINTS ===")
		for name, ep := range ctx.EntryPoints {
			marker := mapTypeToCGFMarker(ep.Type)
			fmt.Fprintf(w, "%s %s %s", marker, ep.Location, name)
			if density != "sparse" && ep.Note != "" {
				fmt.Fprintf(w, " ; %s", ep.Note)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	// Write relevant entities
	fmt.Fprintln(w, "; === RELEVANT ===")
	for name, entity := range ctx.Relevant {
		marker := mapTypeToCGFMarker(entity.Type)
		fmt.Fprintf(w, "%s %s %s", marker, entity.Location, name)

		if density != "sparse" {
			fmt.Fprintf(w, " ; relevance=%s", entity.Relevance)
			if entity.Reason != "" {
				fmt.Fprintf(w, " (%s)", entity.Reason)
			}
		}
		fmt.Fprintln(w)

		// Include code for dense mode
		if density == "dense" && entity.Code != "" {
			// Indent the code
			lines := strings.Split(entity.Code, "\n")
			for _, line := range lines {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
	}

	return nil
}

// writeEntityEdges writes dependency edges in CGF format.
func (f *CGFFormatter) writeEntityEdges(w io.Writer, deps *Dependencies) {
	if deps == nil {
		return
	}

	// Write calls (outgoing edges)
	for _, call := range deps.Calls {
		fmt.Fprintf(w, "  >%s\n", call)
	}

	// Write called-by (incoming edges)
	for _, caller := range deps.CalledBy {
		name := caller.Name
		if name == "" {
			continue
		}
		fmt.Fprintf(w, "  <%s", name)
		if caller.Location != "" {
			fmt.Fprintf(w, " @ %s", caller.Location)
		}
		fmt.Fprintln(w)
	}

	// Write uses-types
	for _, typeName := range deps.UsesTypes {
		fmt.Fprintf(w, "  @%s\n", typeName)
	}
}

// mapTypeToCGFMarker maps entity type strings to CGF markers.
func mapTypeToCGFMarker(t string) string {
	switch strings.ToLower(t) {
	case "function", "method":
		return "F"
	case "struct", "interface", "type":
		return "T"
	case "constant", "variable":
		return "C"
	case "module", "package":
		return "M"
	default:
		return "F" // default to function
	}
}

// mapEdgeTypeToCGF maps edge type strings to CGF edge symbols.
func mapEdgeTypeToCGF(t string) string {
	switch strings.ToLower(t) {
	case "calls":
		return ">"
	case "called_by":
		return "<"
	case "uses":
		return "@"
	case "implements":
		return "^"
	default:
		return "~"
	}
}

// extractEntityName extracts entity name from various contexts.
// This is a helper to get entity name when it's the YAML key.
func extractEntityName(entity *EntityOutput) string {
	// In the YAML output, the entity name is the key, not part of EntityOutput.
	// For CGF we need to extract it from context or use a placeholder.
	// This will be called from contexts where name is available separately.
	return "[entity]"
}

// applyDensityFilter filters entity fields based on density level.
func applyDensityFilter(entity interface{}, density Density) interface{} {
	// For now, return entity as-is.
	// The YAML/JSON tags already include omitempty which handles sparse output.
	// Future enhancement: actively nil out fields based on density.

	// Smart density: adjust based on importance
	// This would require access to metrics to determine effective density

	return entity
}

// GetFormatter returns a formatter for the specified format.
func GetFormatter(format Format) (Formatter, error) {
	switch format {
	case FormatYAML:
		return NewYAMLFormatter(), nil
	case FormatJSON:
		return NewJSONFormatter(), nil
	case FormatCGF:
		return NewCGFFormatter(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

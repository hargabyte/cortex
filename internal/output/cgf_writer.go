// Package output provides writers and types for outputting the Cortex Graph Format (CGF).
// CGF is a token-minimal format for AI agent code navigation, optimized for context
// window efficiency (50-80% smaller than JSON/YAML).
//
// DEPRECATED: CGF format is deprecated and will be removed in v2.0.
// Use YAML (default) or JSON formats instead.
//
// The format supports three density levels:
//   - sparse: routing only, minimal tokens for "find where"
//   - medium: signatures + direct edges, default for task context
//   - dense: full context for editing, includes comments, type refs, all edges
package output

import (
	"fmt"
	"io"
	"strings"
)

const (
	// CGFVersion is the current CGF specification version
	CGFVersion = 1

	// CGFIndentWidth is the number of spaces per indent level
	CGFIndentWidth = 2
)

// CGFWriter outputs CGF format to an io.Writer (DEPRECATED)
// DEPRECATED: Use YAMLFormatter or JSONFormatter instead.
type CGFWriter struct {
	w       io.Writer
	density CGFDensity
	indent  int
}

// NewCGFWriter creates a CGF writer with the specified density level (DEPRECATED)
// DEPRECATED: Use output.GetFormatter(output.FormatYAML) instead.
func NewCGFWriter(w io.Writer, density CGFDensity) *CGFWriter {
	return &CGFWriter{
		w:       w,
		density: density,
		indent:  0,
	}
}

// CGFHeaderOption configures the CGF header
type CGFHeaderOption func(*cgfHeaderConfig)

type cgfHeaderConfig struct {
	scope   string
	task    string
	hops    int
	density CGFDensity
}

// WithScope sets the scope filter for the header
func WithScope(scope string) CGFHeaderOption {
	return func(c *cgfHeaderConfig) {
		c.scope = scope
	}
}

// WithTask sets the task context bead ID for the header
func WithTask(task string) CGFHeaderOption {
	return func(c *cgfHeaderConfig) {
		c.task = task
	}
}

// WithHops sets the max hops for graph expansion in the header
func WithHops(hops int) CGFHeaderOption {
	return func(c *cgfHeaderConfig) {
		c.hops = hops
	}
}

// WriteHeader writes the CGF header line.
// Format: #cgf v1 [options]
// Options include: d=<density> s=<scope> t=<task> h=<hops>
func (w *CGFWriter) WriteHeader(opts ...CGFHeaderOption) error {
	cfg := &cgfHeaderConfig{
		density: w.density,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("#cgf v%d", CGFVersion))
	parts = append(parts, fmt.Sprintf("d=%s", cfg.density))

	if cfg.scope != "" {
		parts = append(parts, fmt.Sprintf("s=%s", cfg.scope))
	}
	if cfg.task != "" {
		parts = append(parts, fmt.Sprintf("t=%s", cfg.task))
	}
	if cfg.hops > 0 {
		parts = append(parts, fmt.Sprintf("h=%d", cfg.hops))
	}

	_, err := fmt.Fprintln(w.w, strings.Join(parts, " "))
	return err
}

// WriteSection writes a section marker line.
// Format: ; === SECTION NAME ===
func (w *CGFWriter) WriteSection(name string) error {
	_, err := fmt.Fprintf(w.w, "; === %s ===\n", strings.ToUpper(name))
	return err
}

// WriteComment writes a comment line.
// Format: ; <text>
func (w *CGFWriter) WriteComment(text string) error {
	_, err := fmt.Fprintf(w.w, "%s; %s\n", w.indentString(), text)
	return err
}

// WriteBlankLine writes an empty line
func (w *CGFWriter) WriteBlankLine() error {
	_, err := fmt.Fprintln(w.w)
	return err
}

// Indent increases the indentation level by one
func (w *CGFWriter) Indent() {
	w.indent++
}

// Dedent decreases the indentation level by one
func (w *CGFWriter) Dedent() {
	if w.indent > 0 {
		w.indent--
	}
}

// SetIndent sets the indentation level directly
func (w *CGFWriter) SetIndent(level int) {
	if level >= 0 {
		w.indent = level
	}
}

// IndentLevel returns the current indentation level
func (w *CGFWriter) IndentLevel() int {
	return w.indent
}

// Density returns the writer's density level
func (w *CGFWriter) Density() CGFDensity {
	return w.density
}

// indentString returns the current indent as a string of spaces
func (w *CGFWriter) indentString() string {
	return strings.Repeat(" ", w.indent*CGFIndentWidth)
}

// writeLineRaw writes a raw line with current indentation
func (w *CGFWriter) writeLineRaw(format string, args ...interface{}) error {
	line := fmt.Sprintf(format, args...)
	_, err := fmt.Fprintf(w.w, "%s%s\n", w.indentString(), line)
	return err
}

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// renderCmd represents the cx render command
var renderCmd = &cobra.Command{
	Use:   "render [file]",
	Short: "Render D2 diagrams to images",
	Long: `Render D2 diagram code to SVG or PNG images.

This command invokes the D2 CLI to render diagrams. It supports:

1. Rendering standalone .d2 files to images
2. Rendering D2 code from stdin
3. Finding D2 code blocks in HTML files and embedding rendered SVGs inline

Input Formats:
  - .d2 file:  Renders directly to image
  - stdin:     Read D2 code from stdin (use - as file argument)
  - .html file with --embed: Finds D2 code blocks and replaces with SVG

Output Formats:
  - svg (default): Scalable vector graphics
  - png: Raster image

Examples:
  # Render a D2 file to SVG (output: diagram.svg)
  cx render diagram.d2

  # Render to PNG with custom output
  cx render diagram.d2 --format png --output diagram.png

  # Render D2 from stdin
  echo "x -> y" | cx render -

  # Render D2 from stdin to specific file
  cat flow.d2 | cx render - --output flow.svg

  # Embed rendered diagrams in HTML file
  cx render report.html --embed

  # Process HTML and write to new file
  cx render report.html --embed --output report-rendered.html`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRender,
}

var (
	renderFormat string // --format svg|png
	renderOutput string // -o/--output file path
	renderEmbed  bool   // --embed for HTML processing
	renderTheme  int    // --theme D2 theme ID
	renderLayout string // --layout engine (elk, dagre, tala)
)

func init() {
	rootCmd.AddCommand(renderCmd)

	renderCmd.Flags().StringVarP(&renderFormat, "format", "f", "svg", "Output format: svg or png")
	renderCmd.Flags().StringVarP(&renderOutput, "output", "o", "", "Output file path (default: input file with new extension)")
	renderCmd.Flags().BoolVar(&renderEmbed, "embed", false, "Find and embed D2 code blocks in HTML files")
	renderCmd.Flags().IntVar(&renderTheme, "theme", 0, "D2 theme ID (0=default)")
	renderCmd.Flags().StringVar(&renderLayout, "layout", "", "Layout engine: elk, dagre, tala (default: from D2 file or elk)")
}

func runRender(cmd *cobra.Command, args []string) error {
	// Validate format
	if renderFormat != "svg" && renderFormat != "png" {
		return fmt.Errorf("invalid format: %s (must be svg or png)", renderFormat)
	}

	// Check that D2 is available
	if err := checkD2Available(); err != nil {
		return err
	}

	// Determine input source
	var inputFile string
	var fromStdin bool

	if len(args) == 0 || args[0] == "-" {
		fromStdin = true
	} else {
		inputFile = args[0]
	}

	if fromStdin {
		return renderFromStdin(cmd)
	}

	// Check if file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", inputFile)
	}

	// Handle HTML embedding mode
	if renderEmbed {
		return renderEmbedHTML(inputFile)
	}

	// Standard D2 file rendering
	return renderD2File(inputFile)
}

// checkD2Available verifies that the D2 CLI is installed and accessible.
func checkD2Available() error {
	_, err := exec.LookPath("d2")
	if err != nil {
		return fmt.Errorf("d2 CLI not found in PATH - install from https://d2lang.com")
	}
	return nil
}

// renderFromStdin reads D2 code from stdin and renders it.
func renderFromStdin(cmd *cobra.Command) error {
	// Read all input from stdin
	d2Code, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}

	if len(bytes.TrimSpace(d2Code)) == 0 {
		return fmt.Errorf("no D2 code provided on stdin")
	}

	// Determine output destination
	var outputPath string
	if renderOutput != "" {
		outputPath = renderOutput
	} else {
		// Default to stdout for stdin input when no output specified
		svg, err := renderD2ToImage(d2Code, renderFormat)
		if err != nil {
			return err
		}
		fmt.Print(string(svg))
		return nil
	}

	// Render and write to file
	result, err := renderD2ToImage(d2Code, renderFormat)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, result, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Rendered to %s\n", outputPath)
	return nil
}

// renderD2File renders a .d2 file to an image file.
func renderD2File(inputFile string) error {
	// Read the D2 file
	d2Code, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Determine output path
	outputPath := renderOutput
	if outputPath == "" {
		// Default: same name with new extension
		ext := filepath.Ext(inputFile)
		outputPath = strings.TrimSuffix(inputFile, ext) + "." + renderFormat
	}

	// Render
	result, err := renderD2ToImage(d2Code, renderFormat)
	if err != nil {
		return err
	}

	// Write output
	if err := os.WriteFile(outputPath, result, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Rendered %s -> %s\n", inputFile, outputPath)
	return nil
}

// renderD2ToImage invokes the D2 CLI to render D2 code to an image.
func renderD2ToImage(d2Code []byte, format string) ([]byte, error) {
	// Create temp file for D2 input
	tmpDir, err := os.MkdirTemp("", "cx-render-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.d2")
	outputPath := filepath.Join(tmpDir, "output."+format)

	if err := os.WriteFile(inputPath, d2Code, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	// Build D2 command
	args := []string{inputPath, outputPath}

	// Add theme if specified
	if renderTheme > 0 {
		args = append(args, "--theme", fmt.Sprintf("%d", renderTheme))
	}

	// Add layout if specified
	if renderLayout != "" {
		args = append(args, "--layout", renderLayout)
	}

	cmd := exec.Command("d2", args...)

	// Capture stderr for error messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("d2 render failed: %s", errMsg)
		}
		return nil, fmt.Errorf("d2 render failed: %w", err)
	}

	// Read the output
	result, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rendered output: %w", err)
	}

	return result, nil
}

// HTML D2 block patterns
var (
	// Match ```d2 ... ``` code blocks
	d2CodeBlockPattern = regexp.MustCompile("(?s)```d2\\s*\n(.*?)```")

	// Match <pre class="d2">...</pre> blocks
	d2PrePattern = regexp.MustCompile(`(?s)<pre[^>]*class="[^"]*d2[^"]*"[^>]*>(.*?)</pre>`)

	// Match <!-- d2 -->...<!-- /d2 --> comment blocks
	d2CommentPattern = regexp.MustCompile(`(?s)<!--\s*d2\s*-->\s*(.*?)\s*<!--\s*/d2\s*-->`)
)

// renderEmbedHTML finds D2 code blocks in an HTML file and replaces them with rendered SVGs.
func renderEmbedHTML(inputFile string) error {
	// Read the HTML file
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	html := string(content)
	modified := false
	var renderErrors []string

	// Process ```d2 ... ``` blocks
	html, mod, errs := processD2Blocks(html, d2CodeBlockPattern, func(match, d2Code string) string {
		svg, err := renderD2ToSVGForEmbed(d2Code)
		if err != nil {
			return match // Keep original on error
		}
		return wrapSVGInDiv(svg, "d2-diagram")
	})
	modified = modified || mod
	renderErrors = append(renderErrors, errs...)

	// Process <pre class="d2">...</pre> blocks
	html, mod, errs = processD2Blocks(html, d2PrePattern, func(match, d2Code string) string {
		// Unescape HTML entities in the code
		d2Code = unescapeHTML(d2Code)
		svg, err := renderD2ToSVGForEmbed(d2Code)
		if err != nil {
			return match
		}
		return wrapSVGInDiv(svg, "d2-diagram")
	})
	modified = modified || mod
	renderErrors = append(renderErrors, errs...)

	// Process <!-- d2 -->...<!-- /d2 --> blocks
	html, mod, errs = processD2Blocks(html, d2CommentPattern, func(match, d2Code string) string {
		svg, err := renderD2ToSVGForEmbed(d2Code)
		if err != nil {
			return match
		}
		return wrapSVGInDiv(svg, "d2-diagram")
	})
	modified = modified || mod
	renderErrors = append(renderErrors, errs...)

	if !modified {
		fmt.Fprintln(os.Stderr, "No D2 code blocks found in file")
		return nil
	}

	// Determine output path
	outputPath := renderOutput
	if outputPath == "" {
		// Default: overwrite the input file
		outputPath = inputFile
	}

	// Write output
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Report results
	if len(renderErrors) > 0 {
		fmt.Fprintf(os.Stderr, "Rendered D2 blocks with %d errors:\n", len(renderErrors))
		for _, e := range renderErrors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Rendered D2 blocks in %s\n", outputPath)
	}

	return nil
}

// processD2Blocks finds and replaces D2 code blocks matching the given pattern.
func processD2Blocks(html string, pattern *regexp.Regexp, replace func(match, d2Code string) string) (string, bool, []string) {
	modified := false
	var errors []string

	result := pattern.ReplaceAllStringFunc(html, func(match string) string {
		// Extract the D2 code from the match
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		d2Code := strings.TrimSpace(submatches[1])
		if d2Code == "" {
			return match
		}

		replacement := replace(match, d2Code)
		if replacement != match {
			modified = true
		} else {
			errors = append(errors, fmt.Sprintf("failed to render block: %s...", truncate(d2Code, 50)))
		}

		return replacement
	})

	return result, modified, errors
}

// renderD2ToSVGForEmbed renders D2 code to SVG for HTML embedding.
func renderD2ToSVGForEmbed(d2Code string) (string, error) {
	svg, err := renderD2ToImage([]byte(d2Code), "svg")
	if err != nil {
		return "", err
	}

	// Clean up the SVG for embedding
	svgStr := string(svg)

	// Remove XML declaration if present
	svgStr = regexp.MustCompile(`<\?xml[^?]*\?>`).ReplaceAllString(svgStr, "")

	// Remove DOCTYPE if present
	svgStr = regexp.MustCompile(`<!DOCTYPE[^>]*>`).ReplaceAllString(svgStr, "")

	return strings.TrimSpace(svgStr), nil
}

// wrapSVGInDiv wraps an SVG in a div with the given class.
func wrapSVGInDiv(svg, class string) string {
	return fmt.Sprintf(`<div class="%s">
%s
</div>`, class, svg)
}

// unescapeHTML unescapes common HTML entities.
func unescapeHTML(s string) string {
	replacements := map[string]string{
		"&lt;":   "<",
		"&gt;":   ">",
		"&amp;":  "&",
		"&quot;": "\"",
		"&#39;":  "'",
		"&apos;": "'",
	}

	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}

	return s
}

// truncate truncates a string to max length with ellipsis.
func truncate(s string, max int) string {
	// Remove newlines for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

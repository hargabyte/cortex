package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckD2Available(t *testing.T) {
	// This test verifies the D2 check function works
	err := checkD2Available()

	// D2 should be installed in the test environment
	// If not, skip the test
	if err != nil {
		t.Skip("D2 CLI not available, skipping render tests")
	}
}

func TestRenderD2ToImage(t *testing.T) {
	// Skip if D2 not available
	if _, err := exec.LookPath("d2"); err != nil {
		t.Skip("D2 CLI not available, skipping render tests")
	}

	tests := []struct {
		name    string
		d2Code  string
		format  string
		wantErr bool
	}{
		{
			name:    "simple arrow",
			d2Code:  "x -> y",
			format:  "svg",
			wantErr: false,
		},
		{
			name:    "multiple nodes",
			d2Code:  "a -> b -> c\nb -> d",
			format:  "svg",
			wantErr: false,
		},
		{
			name: "with labels",
			d2Code: `client: Client
server: Server
client -> server: request`,
			format:  "svg",
			wantErr: false,
		},
		{
			name:    "invalid d2",
			d2Code:  "this is not valid d2 { { {",
			format:  "svg",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderD2ToImage([]byte(tt.d2Code), tt.format)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) == 0 {
				t.Error("expected non-empty result")
				return
			}

			// Verify SVG output
			if tt.format == "svg" {
				resultStr := string(result)
				if !strings.Contains(resultStr, "<svg") {
					t.Error("SVG output does not contain <svg tag")
				}
			}
		})
	}
}

func TestRenderD2File(t *testing.T) {
	// Skip if D2 not available
	if _, err := exec.LookPath("d2"); err != nil {
		t.Skip("D2 CLI not available, skipping render tests")
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-render-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test D2 file
	d2File := filepath.Join(tmpDir, "test.d2")
	d2Content := `title: Test Diagram

a: Component A
b: Component B
a -> b: calls`

	if err := os.WriteFile(d2File, []byte(d2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test rendering
	renderOutput = filepath.Join(tmpDir, "test.svg")
	renderFormat = "svg"
	defer func() {
		renderOutput = ""
		renderFormat = "svg"
	}()

	err = renderD2File(d2File)
	if err != nil {
		t.Errorf("renderD2File failed: %v", err)
		return
	}

	// Verify output file exists
	if _, err := os.Stat(renderOutput); os.IsNotExist(err) {
		t.Error("output file was not created")
		return
	}

	// Verify content
	content, err := os.ReadFile(renderOutput)
	if err != nil {
		t.Error(err)
		return
	}

	if !strings.Contains(string(content), "<svg") {
		t.Error("output does not contain SVG")
	}
}

func TestRenderEmbedHTML(t *testing.T) {
	// Skip if D2 not available
	if _, err := exec.LookPath("d2"); err != nil {
		t.Skip("D2 CLI not available, skipping render tests")
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-render-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		htmlContent string
		wantSVG     bool
	}{
		{
			name: "markdown code block",
			htmlContent: `<!DOCTYPE html>
<html>
<body>
<h1>Test</h1>
` + "```d2\n" + `
x -> y
` + "```" + `
</body>
</html>`,
			wantSVG: true,
		},
		{
			name: "pre tag with class",
			htmlContent: `<!DOCTYPE html>
<html>
<body>
<pre class="d2">
a -> b
</pre>
</body>
</html>`,
			wantSVG: true,
		},
		{
			name: "comment markers",
			htmlContent: `<!DOCTYPE html>
<html>
<body>
<!-- d2 -->
foo -> bar
<!-- /d2 -->
</body>
</html>`,
			wantSVG: true,
		},
		{
			name: "no d2 blocks",
			htmlContent: `<!DOCTYPE html>
<html>
<body>
<p>Just some text</p>
</body>
</html>`,
			wantSVG: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create input file
			inputFile := filepath.Join(tmpDir, "input.html")
			outputFile := filepath.Join(tmpDir, "output.html")

			if err := os.WriteFile(inputFile, []byte(tt.htmlContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Set output path
			renderOutput = outputFile
			defer func() { renderOutput = "" }()

			err := renderEmbedHTML(inputFile)
			if err != nil {
				t.Errorf("renderEmbedHTML failed: %v", err)
				return
			}

			if !tt.wantSVG {
				// No SVG expected, check that output wasn't created or is unchanged
				return
			}

			// Read output
			content, err := os.ReadFile(outputFile)
			if err != nil {
				t.Errorf("failed to read output: %v", err)
				return
			}

			result := string(content)

			// Should contain SVG
			if !strings.Contains(result, "<svg") {
				t.Error("output does not contain embedded SVG")
			}

			// Should contain wrapper div
			if !strings.Contains(result, `class="d2-diagram"`) {
				t.Error("output does not contain d2-diagram wrapper")
			}
		})
	}
}

func TestUnescapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "a &lt; b",
			expected: "a < b",
		},
		{
			input:    "a &gt; b",
			expected: "a > b",
		},
		{
			input:    "a &amp; b",
			expected: "a & b",
		},
		{
			input:    "&quot;hello&quot;",
			expected: "\"hello\"",
		},
		{
			input:    "no entities",
			expected: "no entities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := unescapeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("unescapeHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{
			input:    "short",
			max:      10,
			expected: "short",
		},
		{
			input:    "this is a long string",
			max:      10,
			expected: "this is...",
		},
		{
			input:    "line1\nline2\nline3",
			max:      20,
			expected: "line1 line2 line3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 10)], func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

func TestD2CodeBlockPatterns(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pattern string
		matches int
	}{
		{
			name:    "markdown block",
			input:   "```d2\nx -> y\n```",
			pattern: "markdown",
			matches: 1,
		},
		{
			name:    "pre tag",
			input:   `<pre class="d2">x -> y</pre>`,
			pattern: "pre",
			matches: 1,
		},
		{
			name:    "comment block",
			input:   "<!-- d2 -->\nx -> y\n<!-- /d2 -->",
			pattern: "comment",
			matches: 1,
		},
		{
			name:    "multiple markdown blocks",
			input:   "```d2\na -> b\n```\ntext\n```d2\nc -> d\n```",
			pattern: "markdown",
			matches: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern = d2CodeBlockPattern
			switch tt.pattern {
			case "pre":
				pattern = d2PrePattern
			case "comment":
				pattern = d2CommentPattern
			}

			matches := pattern.FindAllString(tt.input, -1)
			if len(matches) != tt.matches {
				t.Errorf("expected %d matches, got %d", tt.matches, len(matches))
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

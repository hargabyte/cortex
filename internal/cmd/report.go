package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/report"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// reportCmd is the parent command for all report subcommands
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate structured data for AI-powered reports",
	Long: `Generate structured YAML/JSON data for AI-powered codebase reports.

The report commands output data that AI agents can use to generate
publication-quality reports with narratives and visualizations.

Report Types:
  overview   System-level summary with architecture diagram
  feature    Deep-dive into a specific feature using semantic search
  changes    What changed between two points in time (Dolt time-travel)
  health     Risk analysis and recommendations

The --data flag outputs structured YAML (default) or JSON that includes:
  - Entity information (name, type, location, importance, coverage)
  - Dependency relationships (calls, uses_type, implements)
  - D2 diagram code for visualizations
  - Coverage and health metrics

Diagram Themes (--theme):
  default           High-contrast, accessibility-focused (recommended)
  vanilla-nitro     Warm cream and brown tones, professional
  mixed-berry       Cool blue-purple berry palette
  grape-soda        Vibrant purple/violet shades
  earth-tones       Natural browns and greens, organic feel
  orange-creamsicle Warm orange and cream, energetic
  shirley-temple    Playful pink and red tones
  everglade-green   Forest greens, nature-inspired
  terminal          Green-on-black retro terminal aesthetic
  dark              Dark purple/mauve for dark mode
  dark-flagship     Dark with branded accent colors
  neutral           Minimal grayscale, no color distraction

Skill Setup:
  Use --init-skill to generate a Claude Code skill for interactive reports:
  cx report --init-skill > ~/.claude/commands/report.md

Examples:
  cx report overview --data                    # System overview data
  cx report feature "authentication" --data    # Feature deep-dive
  cx report changes --since HEAD~50 --data     # What changed
  cx report health --data                      # Risk analysis

  # Output to file with custom theme
  cx report overview --data --theme earth-tones -o overview.yaml

  # JSON format with dark theme
  cx report feature "auth" --data --format json --theme dark

  # Interactive playground mode (includes layer/filter metadata)
  cx report overview --data --playground -o overview-playground.yaml`,
	RunE: runReportRoot,
}

// Report flags
var (
	reportData       bool   // --data flag to output structured data
	reportOutput     string // -o/--output file path
	reportInitSkill  bool   // --init-skill flag to output skill template
	reportTheme      string // --theme flag for diagram theme
	reportPlayground bool   // --playground flag for interactive playground mode
	reportHTML       bool   // --html flag to generate visual HTML playground
)

// reportOverviewCmd generates overview report data
var reportOverviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Generate system overview report data",
	Long: `Generate structured data for a system-level overview report.

Includes:
  - Statistics (entity counts by type and language)
  - Keystones (high-importance entities)
  - Module structure
  - Architecture diagram (D2 code)
  - Health summary

Examples:
  cx report overview --data                    # YAML to stdout
  cx report overview --data -o overview.yaml   # Write to file
  cx report overview --data --format json      # JSON output`,
	RunE: runReportOverview,
}

// reportFeatureCmd generates feature report data
var reportFeatureCmd = &cobra.Command{
	Use:   "feature <query>",
	Short: "Generate feature report data using semantic search",
	Long: `Generate structured data for a feature deep-dive report.

Uses hybrid search (FTS + embeddings) to find entities related to
the specified feature query.

Includes:
  - Matched entities with relevance scores
  - Dependencies and call relationships
  - Call flow diagram (D2 code)
  - Associated tests
  - Coverage analysis

Examples:
  cx report feature "authentication" --data
  cx report feature "payment processing" --data
  cx report feature "error handling" --data -o errors.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runReportFeature,
}

// reportChangesCmd generates changes report data
var reportChangesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Generate change report data using Dolt time-travel",
	Long: `Generate structured data showing what changed between two points in time.

Uses Dolt's time-travel capabilities to compare the codebase at
different commits, tags, or dates.

Includes:
  - Added, modified, and deleted entities
  - Before/after architecture diagrams (D2 code)
  - Impact analysis (affected dependents)

Examples:
  cx report changes --since HEAD~50 --data
  cx report changes --since v1.0 --until v2.0 --data
  cx report changes --since 2026-01-01 --data`,
	RunE: runReportChanges,
}

// Changes report flags
var (
	changesSince string // --since ref
	changesUntil string // --until ref (defaults to HEAD)
)

// reportHealthCmd generates health report data
var reportHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Generate health report data with risk analysis",
	Long: `Generate structured data for a codebase health report.

Analyzes the codebase for potential issues and risks.

Includes:
  - Risk score (0-100, higher = healthier)
  - Issues by severity (critical, warning, info)
  - Coverage gaps for important code
  - Complexity hotspots
  - Risk heat map diagram (D2 code)

Examples:
  cx report health --data
  cx report health --data -o health.yaml
  cx report health --data --format json`,
	RunE: runReportHealth,
}

func init() {
	rootCmd.AddCommand(reportCmd)

	// Add subcommands
	reportCmd.AddCommand(reportOverviewCmd)
	reportCmd.AddCommand(reportFeatureCmd)
	reportCmd.AddCommand(reportChangesCmd)
	reportCmd.AddCommand(reportHealthCmd)

	// Report-level flags (inherited by subcommands)
	reportCmd.PersistentFlags().BoolVar(&reportData, "data", false, "Output structured data for AI consumption")
	reportCmd.PersistentFlags().StringVarP(&reportOutput, "output", "o", "", "Output file path (default: stdout)")
	reportCmd.PersistentFlags().StringVar(&reportTheme, "theme", "", "Diagram color theme (use --themes to list)")
	reportCmd.PersistentFlags().BoolVar(&reportPlayground, "playground", false, "Include playground metadata for interactive HTML generation")
	reportCmd.PersistentFlags().BoolVar(&reportHTML, "html", false, "Generate visual HTML playground (requires --playground)")

	// Skill template flag (local to report command, not inherited)
	reportCmd.Flags().BoolVar(&reportInitSkill, "init-skill", false, "Output skill template for interactive report generation")

	// Changes-specific flags
	reportChangesCmd.Flags().StringVar(&changesSince, "since", "", "Starting reference (commit, tag, date)")
	reportChangesCmd.Flags().StringVar(&changesUntil, "until", "HEAD", "Ending reference (default: HEAD)")
	reportChangesCmd.MarkFlagRequired("since")
}

// runReportRoot handles the root report command
func runReportRoot(cmd *cobra.Command, args []string) error {
	if reportInitSkill {
		fmt.Print(reportSkillTemplate)
		return nil
	}
	return cmd.Help()
}

// runReportOverview generates overview report data
func runReportOverview(cmd *cobra.Command, args []string) error {
	if !reportData {
		return fmt.Errorf("--data flag is required (reports are designed for AI consumption)")
	}

	// Validate theme if provided
	if reportTheme != "" && !graph.ValidateTheme(reportTheme) {
		return fmt.Errorf("invalid theme %q. Available themes:\n%s", reportTheme, formatAvailableThemes())
	}

	// Open store
	store, err := openStore()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	// Create report structure
	data := report.NewOverviewReport()

	// Gather data from store
	gatherer := report.NewDataGatherer(store)
	gatherer.SetTheme(reportTheme)
	gatherer.SetPlaygroundMode(reportPlayground)
	if err := gatherer.GatherOverviewData(data); err != nil {
		return fmt.Errorf("gather overview data: %w", err)
	}

	return outputReportData(data)
}

// runReportFeature generates feature report data
func runReportFeature(cmd *cobra.Command, args []string) error {
	if !reportData {
		return fmt.Errorf("--data flag is required (reports are designed for AI consumption)")
	}

	// Validate theme if provided
	if reportTheme != "" && !graph.ValidateTheme(reportTheme) {
		return fmt.Errorf("invalid theme %q. Available themes:\n%s", reportTheme, formatAvailableThemes())
	}

	query := args[0]

	// Open store
	store, err := openStore()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	// Create report structure
	data := report.NewFeatureReport(query)

	// Gather data from store using FTS search
	gatherer := report.NewDataGatherer(store)
	gatherer.SetTheme(reportTheme)
	gatherer.SetPlaygroundMode(reportPlayground)
	if err := gatherer.GatherFeatureData(data, query); err != nil {
		return fmt.Errorf("gather feature data: %w", err)
	}

	return outputReportData(data)
}

// runReportChanges generates changes report data
func runReportChanges(cmd *cobra.Command, args []string) error {
	if !reportData {
		return fmt.Errorf("--data flag is required (reports are designed for AI consumption)")
	}

	// Validate theme if provided
	if reportTheme != "" && !graph.ValidateTheme(reportTheme) {
		return fmt.Errorf("invalid theme %q. Available themes:\n%s", reportTheme, formatAvailableThemes())
	}

	// Open store
	store, err := openStore()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	// Create report structure
	data := report.NewChangesReport(changesSince, changesUntil)

	// Gather data from Dolt time-travel queries
	gatherer := report.NewDataGatherer(store)
	gatherer.SetTheme(reportTheme)
	gatherer.SetPlaygroundMode(reportPlayground)
	if err := gatherer.GatherChangesData(data, changesSince, changesUntil); err != nil {
		return fmt.Errorf("gather changes data: %w", err)
	}

	return outputReportData(data)
}

// runReportHealth generates health report data
func runReportHealth(cmd *cobra.Command, args []string) error {
	if !reportData {
		return fmt.Errorf("--data flag is required (reports are designed for AI consumption)")
	}

	// Validate theme if provided (health reports may have diagrams in the future)
	if reportTheme != "" && !graph.ValidateTheme(reportTheme) {
		return fmt.Errorf("invalid theme %q. Available themes:\n%s", reportTheme, formatAvailableThemes())
	}

	// Open store
	store, err := openStore()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	// Create report structure
	data := report.NewHealthReport()

	// Gather health data from store
	gatherer := report.NewDataGatherer(store)
	gatherer.SetTheme(reportTheme)
	gatherer.SetPlaygroundMode(reportPlayground)
	if err := gatherer.GatherHealthData(data); err != nil {
		return fmt.Errorf("gather health data: %w", err)
	}

	return outputReportData(data)
}

// formatAvailableThemes returns a formatted string of available themes for error messages.
func formatAvailableThemes() string {
	themes := graph.GetAvailableThemes()
	var sb strings.Builder
	for _, theme := range themes {
		desc := graph.D2ThemeDescription[theme]
		sb.WriteString(fmt.Sprintf("  %-17s %s\n", theme, desc))
	}
	return sb.String()
}

// outputReportData outputs the report data in the requested format
func outputReportData(data interface{}) error {
	// If HTML playground mode, generate visual HTML
	if reportHTML && reportPlayground {
		return outputPlaygroundHTML(data)
	}

	// Determine output format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}

	// Determine output destination
	var out *os.File
	if reportOutput != "" {
		out, err = os.Create(reportOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	// Output based on format
	switch format {
	case output.FormatYAML:
		enc := yaml.NewEncoder(out)
		enc.SetIndent(2)
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("failed to encode YAML: %w", err)
		}
		return enc.Close()

	case output.FormatJSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)

	default:
		return fmt.Errorf("unsupported format for reports: %s (use yaml or json)", format)
	}
}

// outputPlaygroundHTML generates a visual HTML playground with embedded diagram
func outputPlaygroundHTML(data interface{}) error {
	// Check for coverage data and warn if missing
	if overviewData, ok := data.(*report.OverviewReportData); ok {
		hasCoverage := false
		for _, k := range overviewData.Keystones {
			if k.Coverage >= 0 {
				hasCoverage = true
				break
			}
		}
		if !hasCoverage {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "⚠️  No coverage data available for heatmap feature.")
			fmt.Fprintln(os.Stderr, "   To enable coverage heatmap, run:")
			fmt.Fprintln(os.Stderr, "     go test -coverprofile=coverage.out ./...")
			fmt.Fprintln(os.Stderr, "     cx coverage import coverage.out")
			fmt.Fprintln(os.Stderr, "   Then regenerate this playground. (Tests may take 2-3 minutes)")
			fmt.Fprintln(os.Stderr, "")
		}
	}

	// Convert data to JSON for embedding
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Load SVGs for each preset
	presets := []string{"full", "core", "store", "parser"}
	svgMap := make(map[string]string)

	// Map preset names to SVG file names
	svgFiles := map[string]string{
		"full":   "architecture.svg",
		"core":   "architecture_core.svg",
		"store":  "architecture_store.svg",
		"parser": "architecture_parser.svg",
	}

	placeholder := `<svg viewBox="0 0 800 400" style="background:#f8f8f8">
		<text x="400" y="180" text-anchor="middle" fill="#666" font-size="16">Diagram not rendered yet.</text>
		<text x="400" y="210" text-anchor="middle" fill="#999" font-size="14">Run the render command to generate SVGs.</text>
	</svg>`

	for _, preset := range presets {
		filename := svgFiles[preset]
		paths := []string{
			"reports/" + filename,
			".cx/cortex/reports/" + filename,
		}
		svgContent := ""
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err == nil {
				svgContent = string(content)
				break
			}
		}
		if svgContent == "" {
			svgContent = placeholder
		}
		svgMap[preset] = svgContent
	}

	// Generate the HTML with all SVGs
	html := generatePlaygroundHTML(string(jsonData), svgMap)

	// Determine output destination
	var out *os.File
	if reportOutput != "" {
		out, err = os.Create(reportOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	_, err = out.WriteString(html)
	return err
}

// generatePlaygroundHTML creates the visual playground HTML with multiple SVG presets
func generatePlaygroundHTML(jsonData string, svgMap map[string]string) string {
	// Build SVG containers for each preset
	svgContainers := ""
	for _, preset := range []string{"full", "core", "store", "parser"} {
		display := "none"
		if preset == "full" {
			display = "block"
		}
		svgContainers += fmt.Sprintf(`<div class="svg-preset" id="svg-%s" style="display:%s">%s</div>`, preset, display, svgMap[preset])
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cortex Architecture - Interactive Playground</title>
  <style>
    :root { --bg: #f5f5f5; --sidebar-bg: #fff; --text: #333; --text-muted: #666; --accent: #3498db; --border: #e0e0e0; --issue: #e74c3c; --question: #f39c12; --idea: #9b59b6; --comment: #2ecc71; }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg); color: var(--text); display: flex; height: 100vh; overflow: hidden; }
    .sidebar { width: 300px; background: var(--sidebar-bg); border-right: 1px solid var(--border); display: flex; flex-direction: column; overflow-y: auto; }
    .sidebar-header { padding: 1.25rem; border-bottom: 1px solid var(--border); }
    .sidebar-header h1 { font-size: 1.1rem; margin-bottom: 0.25rem; }
    .sidebar-header p { font-size: 0.8rem; color: var(--text-muted); }
    .info-box { background: #e3f2fd; border-left: 3px solid var(--accent); padding: 0.75rem; margin: 1rem; font-size: 0.8rem; color: #1565c0; border-radius: 0 4px 4px 0; }
    .section { padding: 1rem 1.25rem; border-bottom: 1px solid var(--border); }
    .section h3 { font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-muted); margin-bottom: 0.75rem; }
    .preset-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem; }
    .preset-btn { padding: 0.5rem; border: 1px solid var(--border); border-radius: 6px; background: #fff; cursor: pointer; font-size: 0.8rem; transition: all 0.15s; }
    .preset-btn:hover { border-color: var(--accent); background: #f0f7ff; }
    .preset-btn.active { background: var(--accent); color: white; border-color: var(--accent); }
    .toggle-row { display: flex; align-items: center; gap: 0.75rem; margin-top: 0.5rem; }
    .toggle-row label { font-size: 0.8rem; display: flex; align-items: center; gap: 0.3rem; cursor: pointer; }
    .feedback-section { flex: 1; padding: 1rem 1.25rem; overflow-y: auto; min-height: 120px; }
    .feedback-item { padding: 0.6rem; border-radius: 6px; margin-bottom: 0.5rem; font-size: 0.8rem; border-left: 3px solid; }
    .feedback-item.issue { background: #fdecea; border-color: var(--issue); }
    .feedback-item.question { background: #fef9e7; border-color: var(--question); }
    .feedback-item.idea { background: #f5eef8; border-color: var(--idea); }
    .feedback-item.comment { background: #eafaf1; border-color: var(--comment); }
    .feedback-entity { font-weight: 600; font-size: 0.75rem; margin-bottom: 0.2rem; }
    .feedback-type { font-size: 0.65rem; text-transform: uppercase; opacity: 0.7; }
    .prompt-section { padding: 1rem 1.25rem; border-top: 1px solid var(--border); background: #fafafa; }
    .prompt-section h3 { font-size: 0.7rem; text-transform: uppercase; color: var(--text-muted); margin-bottom: 0.5rem; }
    .prompt-section textarea { width: 100%%; height: 120px; border: 1px solid var(--border); border-radius: 6px; padding: 0.5rem; font-family: monospace; font-size: 0.7rem; resize: vertical; margin-bottom: 0.5rem; }
    .copy-btn { width: 100%%; padding: 0.6rem; background: var(--accent); color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
    .copy-btn:hover { background: #2980b9; }
    .canvas { flex: 1; position: relative; overflow: hidden; background: white; }
    .canvas-toolbar { position: absolute; top: 1rem; right: 1rem; display: flex; gap: 0.5rem; z-index: 10; }
    .toolbar-btn { height: 36px; padding: 0 12px; border: 1px solid var(--border); border-radius: 6px; background: white; cursor: pointer; font-size: 0.8rem; display: flex; align-items: center; gap: 0.3rem; }
    .toolbar-btn:hover { background: #f0f0f0; }
    .toolbar-btn.active { background: var(--accent); color: white; border-color: var(--accent); }
    .svg-container { width: 100%%; height: 100%%; overflow: auto; cursor: grab; }
    .svg-container:active { cursor: grabbing; }
    .svg-preset { width: 100%%; height: 100%%; }
    .svg-preset svg { width: 100%%; height: auto; }
    /* Tooltip */
    .tooltip { position: fixed; background: rgba(0,0,0,0.85); color: white; padding: 8px 12px; border-radius: 6px; font-size: 0.75rem; pointer-events: none; z-index: 1000; max-width: 280px; display: none; }
    .tooltip-name { font-weight: 600; margin-bottom: 4px; }
    .tooltip-info { opacity: 0.8; font-size: 0.7rem; }
    /* Selection ring animation */
    @keyframes pulse-ring { 0%% { transform: scale(1); opacity: 0.8; } 100%% { transform: scale(1.5); opacity: 0; } }
    .selection-ring { position: absolute; border: 3px solid var(--accent); border-radius: 50%%; pointer-events: none; animation: pulse-ring 0.6s ease-out forwards; }
    .node-selected { stroke: var(--accent) !important; stroke-width: 3px !important; filter: drop-shadow(0 0 6px var(--accent)) !important; }
    /* Entity panel */
    .entity-panel { position: absolute; bottom: 0; left: 0; right: 0; background: white; border-top: 1px solid var(--border); padding: 1.25rem; transform: translateY(100%%); transition: transform 0.3s ease; box-shadow: 0 -4px 20px rgba(0,0,0,0.1); }
    .entity-panel.visible { transform: translateY(0); }
    .entity-panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .entity-panel h2 { font-size: 1.1rem; color: var(--accent); }
    .close-btn { background: none; border: none; font-size: 1.5rem; cursor: pointer; color: var(--text-muted); }
    .entity-details { display: flex; gap: 1.5rem; flex-wrap: wrap; margin-bottom: 1rem; }
    .detail-item { font-size: 0.8rem; }
    .detail-item span { color: var(--text-muted); }
    .detail-item code { background: #f0f0f0; padding: 2px 6px; border-radius: 3px; font-size: 0.75rem; }
    .feedback-buttons { display: flex; gap: 0.5rem; flex-wrap: wrap; }
    .fb-btn { padding: 0.5rem 0.75rem; border: none; border-radius: 6px; cursor: pointer; font-size: 0.8rem; color: white; transition: opacity 0.15s; }
    .fb-btn:hover { opacity: 0.85; }
    .fb-btn.issue { background: var(--issue); }
    .fb-btn.question { background: var(--question); }
    .fb-btn.idea { background: var(--idea); }
    .fb-btn.comment { background: var(--comment); }
  </style>
</head>
<body>
  <div class="tooltip" id="tooltip"><div class="tooltip-name"></div><div class="tooltip-info"></div></div>
  <div class="sidebar">
    <div class="sidebar-header"><h1>🔬 Cortex Playground</h1><p>Interactive Architecture Analysis</p></div>
    <div class="section">
      <h3>View Presets</h3>
      <div class="preset-grid">
        <button class="preset-btn active" onclick="applyPreset('full', this)">Full System</button>
        <button class="preset-btn" onclick="applyPreset('core', this)">Core Only</button>
        <button class="preset-btn" onclick="applyPreset('store', this)">Data Flow</button>
        <button class="preset-btn" onclick="applyPreset('parser', this)">Parser</button>
      </div>
    </div>
    <div class="feedback-section">
      <h3>Feedback (<span id="feedback-count">0</span>)</h3>
      <div id="feedback-list"><p style="font-size:0.8rem;color:#999;margin-top:0.5rem;">Click a component, then add feedback using the buttons below.</p></div>
    </div>
    <div class="prompt-section">
      <h3>Generated Prompt</h3>
      <textarea id="prompt-output" readonly placeholder="Your feedback will appear here as a structured prompt..."></textarea>
      <button class="copy-btn" onclick="copyPrompt()">📋 Copy Prompt to Clipboard</button>
    </div>
  </div>
  <div class="canvas">
    <div class="canvas-toolbar">
      <button class="toolbar-btn" onclick="zoomIn()">+</button>
      <button class="toolbar-btn" onclick="zoomOut()">−</button>
      <button class="toolbar-btn" onclick="resetZoom()">⟲</button>
      <button class="toolbar-btn" id="heatmap-btn" onclick="toggleHeatmap()">🌡️ Coverage</button>
    </div>
    <div class="svg-container" id="svg-container">%s</div>
    <div class="entity-panel" id="entity-panel">
      <div class="entity-panel-header">
        <h2 id="panel-title">Component</h2>
        <button class="close-btn" onclick="closePanel()">×</button>
      </div>
      <div class="entity-details" id="entity-details"></div>
      <div class="feedback-buttons">
        <button class="fb-btn issue" onclick="addFeedback('issue')">🔴 Issue</button>
        <button class="fb-btn question" onclick="addFeedback('question')">❓ Question</button>
        <button class="fb-btn idea" onclick="addFeedback('idea')">💡 Idea</button>
        <button class="fb-btn comment" onclick="addFeedback('comment')">💬 Comment</button>
      </div>
    </div>
  </div>
  <script>
    const reportData = %s;
    const state = { 
      currentPreset: 'full', 
      zoom: 1, 
      feedback: [], 
      selectedNode: null,
      selectedNodes: [],      // Multi-select support
      lastSelectedIndex: -1,  // For Shift+click range selection
      heatmapEnabled: false
    };
    
    window.onload = function() { 
      makeSVGInteractive(); 
      generatePrompt(); 
    };
    
    function applyPreset(preset, btn) {
      document.querySelectorAll('.preset-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      document.querySelectorAll('.svg-preset').forEach(div => div.style.display = 'none');
      const target = document.getElementById('svg-' + preset);
      if (target) target.style.display = 'block';
      state.currentPreset = preset;
      makeSVGInteractive();
      if (state.heatmapEnabled) applyHeatmap();
      generatePrompt();
    }
    
    function makeSVGInteractive() {
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return;
      const svg = container.querySelector('svg');
      if (!svg) return;
      
      const tooltip = document.getElementById('tooltip');
      const groups = svg.querySelectorAll('g');
      
      groups.forEach(g => {
        const rect = g.querySelector('rect');
        const text = g.querySelector('text');
        if (rect && text) {
          g.style.cursor = 'pointer';
          g.onclick = (e) => { e.stopPropagation(); selectNode(text.textContent, g, e); };
          g.onmouseenter = (e) => { 
            rect.style.filter = 'brightness(1.1)'; 
            showTooltip(e, text.textContent);
          };
          g.onmousemove = (e) => { moveTooltip(e); };
          g.onmouseleave = () => { 
            rect.style.filter = ''; 
            hideTooltip();
          };
        }
      });
    }
    
    function showTooltip(e, name) {
      const tooltip = document.getElementById('tooltip');
      const entity = reportData.keystones?.find(k => k.name === name);
      tooltip.querySelector('.tooltip-name').textContent = name;
      tooltip.querySelector('.tooltip-info').innerHTML = entity 
        ? entity.entity_type + ' • ' + (entity.file || 'N/A') + '<br>PageRank: ' + (entity.pagerank?.toFixed(4) || 'N/A')
        : 'Component';
      tooltip.style.display = 'block';
      moveTooltip(e);
    }
    
    function moveTooltip(e) {
      const tooltip = document.getElementById('tooltip');
      tooltip.style.left = (e.clientX + 15) + 'px';
      tooltip.style.top = (e.clientY + 15) + 'px';
    }
    
    function hideTooltip() {
      document.getElementById('tooltip').style.display = 'none';
    }
    
    function selectNode(name, element, event) {
      const allNodes = getAllNodeElements();
      const nodeIndex = allNodes.findIndex(n => n.name === name);
      
      // Multi-select with Ctrl+click
      if (event.ctrlKey || event.metaKey) {
        const existingIndex = state.selectedNodes.findIndex(n => n.name === name);
        if (existingIndex >= 0) {
          // Deselect if already selected
          state.selectedNodes.splice(existingIndex, 1);
          element.querySelector('rect')?.classList.remove('node-selected');
        } else {
          // Add to selection
          state.selectedNodes.push({ name, element });
          element.querySelector('rect')?.classList.add('node-selected');
        }
        state.lastSelectedIndex = nodeIndex;
      }
      // Range select with Shift+click
      else if (event.shiftKey && state.lastSelectedIndex >= 0) {
        const start = Math.min(state.lastSelectedIndex, nodeIndex);
        const end = Math.max(state.lastSelectedIndex, nodeIndex);
        // Clear previous selection
        clearAllSelections();
        state.selectedNodes = [];
        // Select range
        for (let i = start; i <= end; i++) {
          if (allNodes[i]) {
            state.selectedNodes.push(allNodes[i]);
            allNodes[i].element.querySelector('rect')?.classList.add('node-selected');
          }
        }
      }
      // Normal click - single select
      else {
        clearAllSelections();
        state.selectedNodes = [{ name, element }];
        element.querySelector('rect')?.classList.add('node-selected');
        state.lastSelectedIndex = nodeIndex;
      }
      
      // Always set selectedNode to most recent for feedback
      state.selectedNode = { name, element };
      
      // Show selection ring animation
      const rect = element.getBoundingClientRect();
      const ring = document.createElement('div');
      ring.className = 'selection-ring';
      ring.style.left = (rect.left + rect.width/2 - 30) + 'px';
      ring.style.top = (rect.top + rect.height/2 - 30) + 'px';
      ring.style.width = '60px';
      ring.style.height = '60px';
      document.body.appendChild(ring);
      setTimeout(() => ring.remove(), 600);
      
      // Update panel with selection info
      updateSelectionPanel();
    }
    
    function getAllNodeElements() {
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return [];
      const svg = container.querySelector('svg');
      if (!svg) return [];
      const nodes = [];
      svg.querySelectorAll('g').forEach(g => {
        const text = g.querySelector('text');
        if (text) nodes.push({ name: text.textContent, element: g });
      });
      return nodes;
    }
    
    function clearAllSelections() {
      state.selectedNodes.forEach(n => {
        n.element.querySelector('rect')?.classList.remove('node-selected');
      });
    }
    
    function updateSelectionPanel() {
      const count = state.selectedNodes.length;
      if (count === 0) {
        document.getElementById('entity-panel').classList.remove('visible');
        return;
      }
      
      if (count === 1) {
        const name = state.selectedNodes[0].name;
        document.getElementById('panel-title').textContent = name;
        const entity = reportData.keystones?.find(k => k.name === name) || { name };
        document.getElementById('entity-details').innerHTML = 
          '<div class="detail-item"><span>Type:</span> '+(entity.entity_type||'component')+'</div>'+
          '<div class="detail-item"><span>File:</span> <code>'+(entity.file||'N/A')+'</code></div>'+
          '<div class="detail-item"><span>PageRank:</span> '+(entity.pagerank?.toFixed(4)||'N/A')+'</div>'+
          '<div class="detail-item"><span>In-degree:</span> '+(entity.in_degree||0)+'</div>';
      } else {
        document.getElementById('panel-title').textContent = count + ' entities selected';
        const names = state.selectedNodes.map(n => n.name);
        document.getElementById('entity-details').innerHTML = 
          '<div class="detail-item"><span>Selected:</span></div>'+
          '<div style="max-height:150px;overflow-y:auto;font-size:0.85rem;">'+
          names.map(n => '<div style="padding:2px 0;border-bottom:1px solid #eee;">• '+n+'</div>').join('')+
          '</div>'+
          '<div style="margin-top:10px;font-size:0.8rem;color:#666;">'+
          '<strong>Tip:</strong> Ctrl+click to toggle, Shift+click for range</div>';
      }
      document.getElementById('entity-panel').classList.add('visible');
      generatePrompt();
    }
    
    function closePanel() { 
      document.getElementById('entity-panel').classList.remove('visible'); 
      clearAllSelections();
      state.selectedNode = null;
      state.selectedNodes = [];
      generatePrompt();
    }
    
    function addFeedback(type) {
      if (!state.selectedNode) return;
      const prompts = {
        issue: 'Describe the issue with ' + state.selectedNode.name + ':',
        question: 'What question do you have about ' + state.selectedNode.name + '?',
        idea: 'Describe your improvement idea for ' + state.selectedNode.name + ':',
        comment: 'Add a comment about ' + state.selectedNode.name + ':'
      };
      const text = prompt(prompts[type]);
      if (text) { 
        state.feedback.push({ entity: state.selectedNode.name, type, text }); 
        updateFeedbackList(); 
        generatePrompt(); 
      }
    }
    
    function updateFeedbackList() {
      const list = document.getElementById('feedback-list');
      document.getElementById('feedback-count').textContent = state.feedback.length;
      if (state.feedback.length === 0) { 
        list.innerHTML = '<p style="font-size:0.8rem;color:#999;margin-top:0.5rem;">Click a component, then add feedback using the buttons below.</p>'; 
        return; 
      }
      const icons = { issue: '🔴', question: '❓', idea: '💡', comment: '💬' };
      list.innerHTML = state.feedback.map((f, i) => 
        '<div class="feedback-item '+f.type+'">'+
        '<div class="feedback-type">'+icons[f.type]+' '+f.type+'</div>'+
        '<div class="feedback-entity">'+f.entity+'</div>'+
        '<div>'+f.text+'</div></div>'
      ).join('');
    }
    
    function toggleHeatmap() {
      // Check if we have real coverage data
      const hasCoverage = (reportData.keystones || []).some(k => k.coverage >= 0);
      if (!hasCoverage) {
        const prompt = 'Please generate coverage data for the Cortex playground. Run the test suite with coverage profiling and import it into the database, then regenerate the playground HTML.';
        if (confirm('No coverage data available.\\n\\nWould you like to copy a prompt to ask Claude to generate coverage data?\\n\\n(This may take 2-3 minutes to run tests)')) {
          navigator.clipboard.writeText(prompt).then(() => {
            alert('Prompt copied! Paste it to Claude to generate coverage data.');
          }).catch(() => {
            alert('Copy this prompt to Claude:\\n\\n' + prompt);
          });
        }
        return;
      }
      
      state.heatmapEnabled = !state.heatmapEnabled;
      document.getElementById('heatmap-btn').classList.toggle('active', state.heatmapEnabled);
      if (state.heatmapEnabled) applyHeatmap();
      else removeHeatmap();
    }
    
    function applyHeatmap() {
      // Build coverage map from keystones
      const coverageMap = {};
      (reportData.keystones || []).forEach(k => {
        if (k.coverage >= 0) coverageMap[k.name] = k.coverage;
      });
      
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return;
      
      // Find text elements and color their parent rects
      const textElems = container.querySelectorAll('text');
      textElems.forEach(text => {
        const name = text.textContent?.trim();
        const coverage = coverageMap[name];
        if (coverage !== undefined) {
          const parent = text.closest('g');
          if (parent) {
            const rect = parent.querySelector('rect');
            if (rect) {
              rect.dataset.originalFill = rect.getAttribute('fill');
              // Color by coverage: red < 50, yellow 50-80, green 80+
              let color;
              if (coverage < 50) color = '#e74c3c';
              else if (coverage < 80) color = '#f39c12';
              else color = '#2ecc71';
              rect.setAttribute('fill', color);
            }
          }
        }
      });
    }
    
    function removeHeatmap() {
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return;
      const rects = container.querySelectorAll('rect[data-original-fill]');
      rects.forEach(rect => {
        rect.setAttribute('fill', rect.dataset.originalFill);
      });
    }
    
    function generatePrompt() {
      let p = '# Architecture Analysis Request\n\n';
      p += '**View:** ' + state.currentPreset + '\n';
      p += '**Generated:** ' + (reportData.report?.generated_at || new Date().toISOString()) + '\n\n';
      
      // Show selected entities if any
      if (state.selectedNodes.length > 0) {
        p += '## 🎯 Selected Entities (' + state.selectedNodes.length + ')\n\n';
        state.selectedNodes.forEach(n => {
          const entity = reportData.keystones?.find(k => k.name === n.name);
          if (entity) {
            p += '- **' + n.name + '** (' + entity.entity_type + ') - ' + (entity.file || 'N/A');
            if (entity.pagerank) p += ' [PR: ' + entity.pagerank.toFixed(3) + ']';
            p += '\n';
          } else {
            p += '- **' + n.name + '**\n';
          }
        });
        p += '\n';
      }
      
      if (state.feedback.length > 0) {
        const grouped = { issue: [], question: [], idea: [], comment: [] };
        state.feedback.forEach(f => grouped[f.type].push(f));
        
        if (grouped.issue.length) {
          p += '## 🔴 Issues\n\n';
          grouped.issue.forEach(f => p += '**' + f.entity + ':** ' + f.text + '\n\n');
        }
        if (grouped.question.length) {
          p += '## ❓ Questions\n\n';
          grouped.question.forEach(f => p += '**' + f.entity + ':** ' + f.text + '\n\n');
        }
        if (grouped.idea.length) {
          p += '## 💡 Ideas\n\n';
          grouped.idea.forEach(f => p += '**' + f.entity + ':** ' + f.text + '\n\n');
        }
        if (grouped.comment.length) {
          p += '## 💬 Comments\n\n';
          grouped.comment.forEach(f => p += '**' + f.entity + ':** ' + f.text + '\n\n');
        }
      }
      
      p += '## Context: Top Components\n\n';
      (reportData.keystones || []).slice(0, 5).forEach(k => {
        p += '- **' + k.name + '** (' + k.entity_type + ') - ' + (k.file || 'N/A') + '\n';
      });
      
      document.getElementById('prompt-output').value = p;
    }
    
    function copyPrompt() { 
      const t = document.getElementById('prompt-output'); 
      t.select(); 
      document.execCommand('copy'); 
      event.target.textContent = '✓ Copied!'; 
      setTimeout(() => event.target.textContent = '📋 Copy Prompt to Clipboard', 2000); 
    }
    
    function zoomIn() { state.zoom *= 1.2; applyZoom(); }
    function zoomOut() { state.zoom /= 1.2; applyZoom(); }
    function resetZoom() { state.zoom = 1; applyZoom(); }
    function applyZoom() { 
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return;
      const svg = container.querySelector('svg'); 
      if (svg) svg.style.transform = 'scale(' + state.zoom + ')'; 
    }
  </script>
</body>
</html>`, svgContainers, jsonData)
}


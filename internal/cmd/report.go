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
    :root { --bg: #f5f5f5; --sidebar-bg: #fff; --text: #333; --text-muted: #666; --accent: #3498db; --border: #e0e0e0; }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg); color: var(--text); display: flex; height: 100vh; overflow: hidden; }
    .sidebar { width: 280px; background: var(--sidebar-bg); border-right: 1px solid var(--border); display: flex; flex-direction: column; overflow-y: auto; }
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
    .toggle-group { display: flex; flex-direction: column; gap: 0.5rem; }
    .toggle-item { display: flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; cursor: pointer; }
    .toggle-item input { margin: 0; }
    .toggle-color { width: 14px; height: 14px; border-radius: 3px; }
    .legend { margin-top: 0.5rem; }
    .legend-item { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; margin-bottom: 0.4rem; }
    .legend-line { width: 30px; height: 2px; }
    .legend-line.solid { background: #3498db; }
    .legend-line.dashed { background: repeating-linear-gradient(90deg, #2ecc71, #2ecc71 4px, transparent 4px, transparent 8px); }
    .legend-line.dotted { background: repeating-linear-gradient(90deg, #e74c3c, #e74c3c 2px, transparent 2px, transparent 5px); }
    .comments-section { flex: 1; padding: 1rem 1.25rem; overflow-y: auto; }
    .comment-item { background: #f8f8f8; padding: 0.75rem; border-radius: 6px; margin-bottom: 0.5rem; font-size: 0.85rem; }
    .comment-entity { font-weight: 600; color: var(--accent); margin-bottom: 0.25rem; }
    .prompt-section { padding: 1rem 1.25rem; border-top: 1px solid var(--border); background: #fafafa; }
    .prompt-section textarea { width: 100%%; height: 80px; border: 1px solid var(--border); border-radius: 6px; padding: 0.5rem; font-family: monospace; font-size: 0.75rem; resize: none; margin-bottom: 0.5rem; }
    .copy-btn { width: 100%%; padding: 0.6rem; background: var(--accent); color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
    .copy-btn:hover { background: #2980b9; }
    .canvas { flex: 1; position: relative; overflow: hidden; background: white; }
    .canvas-toolbar { position: absolute; top: 1rem; right: 1rem; display: flex; gap: 0.5rem; z-index: 10; }
    .zoom-btn { width: 36px; height: 36px; border: 1px solid var(--border); border-radius: 6px; background: white; cursor: pointer; font-size: 1.2rem; display: flex; align-items: center; justify-content: center; }
    .zoom-btn:hover { background: #f0f0f0; }
    .svg-container { width: 100%%; height: 100%%; overflow: auto; cursor: grab; }
    .svg-container:active { cursor: grabbing; }
    .svg-container svg { display: block; min-width: 100%%; min-height: 100%%; }
    .svg-preset { width: 100%%; height: 100%%; }
    .svg-preset svg { width: 100%%; height: auto; }
    .entity-panel { position: absolute; bottom: 0; left: 0; right: 0; background: white; border-top: 1px solid var(--border); padding: 1.5rem; transform: translateY(100%%); transition: transform 0.3s ease; box-shadow: 0 -4px 20px rgba(0,0,0,0.1); }
    .entity-panel.visible { transform: translateY(0); }
    .entity-panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .entity-panel h2 { font-size: 1.2rem; color: var(--accent); }
    .close-btn { background: none; border: none; font-size: 1.5rem; cursor: pointer; color: var(--text-muted); }
    .entity-details { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; }
    .detail-card { background: #f8f8f8; padding: 0.75rem; border-radius: 6px; }
    .detail-label { font-size: 0.7rem; text-transform: uppercase; color: var(--text-muted); margin-bottom: 0.25rem; }
    .detail-value { font-weight: 500; font-size: 0.9rem; }
    .detail-value code { background: #e8e8e8; padding: 0.1rem 0.3rem; border-radius: 3px; font-size: 0.8rem; }
    .comment-input-btn { margin-top: 1rem; padding: 0.5rem 1rem; background: #2ecc71; color: white; border: none; border-radius: 6px; cursor: pointer; }
  </style>
</head>
<body>
  <div class="sidebar">
    <div class="sidebar-header"><h1>Cortex Architecture</h1><p>Interactive Playground</p></div>
    <div class="info-box">Click presets to switch between filtered views. Click components to add comments.</div>
    <div class="section"><h3>View Presets</h3><div class="preset-grid" id="presets">
      <button class="preset-btn active" data-preset="full" onclick="applyPreset('full', this)">Full System</button>
      <button class="preset-btn" data-preset="core" onclick="applyPreset('core', this)">Core Only</button>
      <button class="preset-btn" data-preset="store" onclick="applyPreset('store', this)">Data Flow</button>
      <button class="preset-btn" data-preset="parser" onclick="applyPreset('parser', this)">Parser</button>
    </div></div>
    <div class="section"><h3>Connection Types</h3><div class="legend"><div class="legend-item"><div class="legend-line solid"></div><span>Data Flow</span></div><div class="legend-item"><div class="legend-line dashed"></div><span>Type Dependencies</span></div><div class="legend-item"><div class="legend-line dotted"></div><span>Implements</span></div></div></div>
    <div class="comments-section"><h3>Comments (<span id="comment-count">0</span>)</h3><div id="comments-list"><p style="font-size:0.8rem;color:#999">Click a component to add comments</p></div></div>
    <div class="prompt-section"><textarea id="prompt-output" readonly placeholder="Your observations will appear here..."></textarea><button class="copy-btn" onclick="copyPrompt()">Copy Prompt</button></div>
  </div>
  <div class="canvas">
    <div class="canvas-toolbar"><button class="zoom-btn" onclick="zoomIn()">+</button><button class="zoom-btn" onclick="zoomOut()">−</button><button class="zoom-btn" onclick="resetZoom()">⟲</button></div>
    <div class="svg-container" id="svg-container">%s</div>
    <div class="entity-panel" id="entity-panel"><div class="entity-panel-header"><h2 id="panel-title">Component Details</h2><button class="close-btn" onclick="closePanel()">×</button></div><div class="entity-details" id="entity-details"></div><button class="comment-input-btn" onclick="addComment()">💬 Add Comment</button></div>
  </div>
  <script>
    const reportData = %s;
    const state = { currentPreset: 'full', zoom: 1, comments: [], selectedNode: null };
    
    window.onload = function() { 
      makeSVGInteractive(); 
      generatePrompt(); 
    };
    
    function applyPreset(preset, btn) {
      // Update button states
      document.querySelectorAll('.preset-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      
      // Hide all SVG presets, show selected one
      document.querySelectorAll('.svg-preset').forEach(div => div.style.display = 'none');
      const target = document.getElementById('svg-' + preset);
      if (target) target.style.display = 'block';
      
      state.currentPreset = preset;
      makeSVGInteractive();
      generatePrompt();
    }
    
    function makeSVGInteractive() {
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return;
      const svg = container.querySelector('svg');
      if (!svg) return;
      
      const groups = svg.querySelectorAll('g');
      groups.forEach(g => {
        const rect = g.querySelector('rect');
        const text = g.querySelector('text');
        if (rect && text) {
          g.style.cursor = 'pointer';
          g.onclick = (e) => { e.stopPropagation(); selectNode(text.textContent, g); };
          g.onmouseenter = () => { rect.style.filter = 'brightness(1.1)'; };
          g.onmouseleave = () => { rect.style.filter = ''; };
        }
      });
    }
    
    function selectNode(name, element) {
      state.selectedNode = { name, element };
      document.getElementById('panel-title').textContent = name;
      const entity = reportData.keystones?.find(k => k.name === name) || { name };
      document.getElementById('entity-details').innerHTML = 
        '<div class="detail-card"><div class="detail-label">Name</div><div class="detail-value">'+(entity.name||name)+'</div></div>'+
        '<div class="detail-card"><div class="detail-label">Type</div><div class="detail-value">'+(entity.entity_type||'component')+'</div></div>'+
        '<div class="detail-card"><div class="detail-label">File</div><div class="detail-value"><code>'+(entity.file||'N/A')+'</code></div></div>'+
        '<div class="detail-card"><div class="detail-label">PageRank</div><div class="detail-value">'+(entity.pagerank?.toFixed(4)||'N/A')+'</div></div>';
      document.getElementById('entity-panel').classList.add('visible');
    }
    
    function closePanel() { document.getElementById('entity-panel').classList.remove('visible'); state.selectedNode = null; }
    
    function addComment() {
      if (!state.selectedNode) return;
      const text = prompt('Add comment for '+state.selectedNode.name+':');
      if (text) { state.comments.push({ entity: state.selectedNode.name, text }); updateCommentsList(); generatePrompt(); }
    }
    
    function updateCommentsList() {
      const list = document.getElementById('comments-list');
      document.getElementById('comment-count').textContent = state.comments.length;
      if (state.comments.length === 0) { list.innerHTML = '<p style="font-size:0.8rem;color:#999">Click a component to add comments</p>'; return; }
      list.innerHTML = state.comments.map(c => '<div class="comment-item"><div class="comment-entity">'+c.entity+'</div><div>'+c.text+'</div></div>').join('');
    }
    
    function generatePrompt() {
      let p = '# Cortex Architecture Analysis\n\n**Preset:** '+state.currentPreset+'\n**Generated:** '+(reportData.report?.generated_at||'Unknown')+'\n\n';
      if (state.comments.length > 0) { p += '## Observations\n\n'; state.comments.forEach(c => { p += '### '+c.entity+'\n'+c.text+'\n\n'; }); }
      p += '## Top Components\n'; (reportData.keystones||[]).slice(0,5).forEach(k => { p += '- **'+k.name+'** ('+k.entity_type+')\n'; });
      document.getElementById('prompt-output').value = p;
    }
    
    function copyPrompt() { const t = document.getElementById('prompt-output'); t.select(); document.execCommand('copy'); event.target.textContent = '✓ Copied!'; setTimeout(() => event.target.textContent = 'Copy Prompt', 2000); }
    function zoomIn() { state.zoom *= 1.2; applyZoom(); }
    function zoomOut() { state.zoom /= 1.2; applyZoom(); }
    function resetZoom() { state.zoom = 1; applyZoom(); }
    function applyZoom() { 
      const container = document.getElementById('svg-' + state.currentPreset);
      if (!container) return;
      const svg = container.querySelector('svg'); 
      if (svg) svg.style.transform = 'scale('+state.zoom+')'; 
    }
  </script>
</body>
</html>`, svgContainers, jsonData)
}


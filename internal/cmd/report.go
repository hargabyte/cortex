package cmd

import (
	"encoding/json"
	"fmt"
	"os"

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

Examples:
  cx report overview --data                    # System overview data
  cx report feature "authentication" --data    # Feature deep-dive
  cx report changes --since HEAD~50 --data     # What changed
  cx report health --data                      # Risk analysis

  # Output to file
  cx report overview --data -o overview.yaml

  # JSON format
  cx report feature "auth" --data --format json`,
}

// Report flags
var (
	reportData   bool   // --data flag to output structured data
	reportOutput string // -o/--output file path
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

	// Changes-specific flags
	reportChangesCmd.Flags().StringVar(&changesSince, "since", "", "Starting reference (commit, tag, date)")
	reportChangesCmd.Flags().StringVar(&changesUntil, "until", "HEAD", "Ending reference (default: HEAD)")
	reportChangesCmd.MarkFlagRequired("since")
}

// runReportOverview generates overview report data
func runReportOverview(cmd *cobra.Command, args []string) error {
	if !reportData {
		return fmt.Errorf("--data flag is required (reports are designed for AI consumption)")
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
	if err := gatherer.GatherHealthData(data); err != nil {
		return fmt.Errorf("gather health data: %w", err)
	}

	return outputReportData(data)
}

// outputReportData outputs the report data in the requested format
func outputReportData(data interface{}) error {
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


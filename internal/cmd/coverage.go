package cmd

import (
	"fmt"
	"os"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// coverageCmd represents the coverage command
var coverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Import and analyze test coverage data",
	Long: `Import Go test coverage data and map it to code entities.

The coverage command helps you understand which entities are tested and which are not.
It parses coverage.out files from 'go test -coverprofile' and maps coverage blocks
to entities in the cx database.

Subcommands:
  import  Import coverage data from coverage.out file
  status  Show overall coverage statistics

Examples:
  # Generate and import coverage
  go test -coverprofile=coverage.out ./...
  cx coverage import coverage.out

  # Show coverage statistics
  cx coverage status

  # Import with custom base path
  cx coverage import coverage.out --base-path=/path/to/project`,
}

// importCmd represents the coverage import subcommand
var importCmd = &cobra.Command{
	Use:   "import <coverage.out | directory>",
	Short: "Import coverage data from coverage.out file or directory",
	Long: `Parse Go coverage data and map coverage blocks to entities.

Supported formats:
  1. coverage.out file (from go test -coverprofile)
  2. Per-test coverage directory (TestName.out files) - RECOMMENDED for test-impact
  3. GOCOVERDIR directory (from go build -cover binaries)

This command:
  1. Auto-detects the input format (file vs directory type)
  2. Parses the coverage data
  3. Maps coverage blocks to entities by line ranges
  4. Calculates coverage percentage per entity
  5. Stores coverage data in the database
  6. Populates test→entity mappings (for per-test directories)

Examples:
  # Traditional coverage.out (aggregate only, no per-test attribution)
  go test -coverprofile=coverage.out ./...
  cx coverage import coverage.out

  # Per-test coverage directory (RECOMMENDED for cx test-impact)
  # Creates one coverage file per test, enabling test→entity mapping
  mkdir -p .coverage
  for test in $(go test -list '.*' ./... 2>/dev/null | grep '^Test'); do
    go test -coverprofile=.coverage/${test}.out -run "^${test}$" ./... 2>/dev/null
  done
  cx coverage import .coverage/

  # Quick per-test script (add to Makefile)
  # make coverage-per-test && cx coverage import .coverage/

The per-test mode populates test_entity_map, enabling 'cx test-impact' and
showing 'tested_by' in 'cx show --coverage'.`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

// statusCmd represents the coverage status subcommand
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show coverage statistics",
	Long: `Display overall test coverage statistics from the database.

Shows:
  - Total entities with coverage data
  - Average coverage percentage
  - Breakdown by coverage levels (full, partial, none)
  - Last coverage import time

Run 'cx coverage import' first to import coverage data.`,
	RunE: runStatus,
}

var (
	coverageBasePath string
)

func init() {
	rootCmd.AddCommand(coverageCmd)
	coverageCmd.AddCommand(importCmd)
	coverageCmd.AddCommand(statusCmd)

	// Import-specific flags
	importCmd.Flags().StringVar(&coverageBasePath, "base-path", "", "Base path for normalizing file paths (default: current directory)")
}

func runImport(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	// Open store
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Determine base path for normalization
	basePath := coverageBasePath
	if basePath == "" {
		basePath = "."
	}

	// Auto-detect format:
	// 1. Per-test coverage directory (TestFoo.out, TestBar.out files)
	// 2. GOCOVERDIR (covmeta.*/covcounters.* files)
	// 3. Traditional coverage.out file
	if coverage.IsPerTestCoverageDir(inputPath) {
		return runImportPerTestDir(storeDB, inputPath, basePath)
	}
	if coverage.IsGOCOVERDIR(inputPath) {
		return runImportGOCOVERDIR(storeDB, inputPath, basePath)
	}

	// Traditional coverage.out file
	return runImportCoverageOut(storeDB, inputPath, basePath)
}

// runImportCoverageOut handles traditional coverage.out file import
func runImportCoverageOut(storeDB *store.Store, coverageFile, basePath string) error {
	// Parse coverage file
	fmt.Fprintf(os.Stderr, "Parsing coverage file: %s\n", coverageFile)
	coverageData, err := coverage.ParseCoverageFile(coverageFile)
	if err != nil {
		return fmt.Errorf("failed to parse coverage file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Found %d coverage blocks (mode: %s)\n", len(coverageData.Blocks), coverageData.Mode)

	// Map coverage to entities
	fmt.Fprintf(os.Stderr, "Mapping coverage to entities...\n")
	mapper := coverage.NewMapper(storeDB, coverageData, basePath)
	entityCoverages, err := mapper.MapCoverageToEntities()
	if err != nil {
		return fmt.Errorf("failed to map coverage: %w", err)
	}

	if len(entityCoverages) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: No entities matched coverage data. Check file path normalization.\n")
		return nil
	}

	// Store coverage data
	fmt.Fprintf(os.Stderr, "Storing coverage for %d entities...\n", len(entityCoverages))
	if err := coverage.StoreCoverage(storeDB, entityCoverages); err != nil {
		return fmt.Errorf("failed to store coverage: %w", err)
	}

	// Display summary
	printCoverageSummary(entityCoverages, false, 0)

	return nil
}

// runImportPerTestDir handles a directory of per-test coverage.out files
func runImportPerTestDir(storeDB *store.Store, dirPath, basePath string) error {
	fmt.Fprintf(os.Stderr, "Parsing per-test coverage directory: %s\n", dirPath)

	perTestData, err := coverage.ParsePerTestCoverageDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to parse per-test coverage: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Found %d tests with coverage data\n", len(perTestData.PerTest))
	if perTestData.Aggregate != nil {
		fmt.Fprintf(os.Stderr, "Merged %d coverage blocks (mode: %s)\n",
			len(perTestData.Aggregate.Blocks), perTestData.Aggregate.Mode)
	}

	// Map aggregate coverage to entities
	fmt.Fprintf(os.Stderr, "Mapping coverage to entities...\n")
	mapper := coverage.NewMapper(storeDB, perTestData.Aggregate, basePath)
	entityCoverages, err := mapper.MapCoverageToEntities()
	if err != nil {
		return fmt.Errorf("failed to map coverage: %w", err)
	}

	if len(entityCoverages) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: No entities matched coverage data. Check file path normalization.\n")
		return nil
	}

	// Store aggregate coverage data
	fmt.Fprintf(os.Stderr, "Storing coverage for %d entities...\n", len(entityCoverages))
	if err := coverage.StoreCoverage(storeDB, entityCoverages); err != nil {
		return fmt.Errorf("failed to store coverage: %w", err)
	}

	// Populate test_entity_map with per-test attribution
	fmt.Fprintf(os.Stderr, "Mapping per-test coverage to entities...\n")
	testMappings, err := coverage.StoreTestEntityMappings(storeDB, perTestData, basePath)
	if err != nil {
		return fmt.Errorf("failed to store test mappings: %w", err)
	}

	// Display summary
	printCoverageSummary(entityCoverages, true, testMappings)

	return nil
}

// runImportGOCOVERDIR handles Go 1.20+ GOCOVERDIR import with per-test attribution
func runImportGOCOVERDIR(storeDB *store.Store, dirPath, basePath string) error {
	fmt.Fprintf(os.Stderr, "Parsing GOCOVERDIR: %s\n", dirPath)

	gocoverData, err := coverage.ParseGOCOVERDIR(dirPath)
	if err != nil {
		return fmt.Errorf("failed to parse GOCOVERDIR: %w", err)
	}

	// Report what we found
	if gocoverData.HasPerTestAttribution() {
		fmt.Fprintf(os.Stderr, "Found %d tests with per-test coverage\n", len(gocoverData.PerTest))
	}
	if gocoverData.Aggregate != nil {
		fmt.Fprintf(os.Stderr, "Found %d coverage blocks (mode: %s)\n",
			len(gocoverData.Aggregate.Blocks), gocoverData.Aggregate.Mode)
	}

	// Map aggregate coverage to entities
	fmt.Fprintf(os.Stderr, "Mapping coverage to entities...\n")
	mapper := coverage.NewMapper(storeDB, gocoverData.Aggregate, basePath)
	entityCoverages, err := mapper.MapCoverageToEntities()
	if err != nil {
		return fmt.Errorf("failed to map coverage: %w", err)
	}

	if len(entityCoverages) == 0 {
		fmt.Fprintf(os.Stderr, "Warning: No entities matched coverage data. Check file path normalization.\n")
		return nil
	}

	// Store aggregate coverage data
	fmt.Fprintf(os.Stderr, "Storing coverage for %d entities...\n", len(entityCoverages))
	if err := coverage.StoreCoverage(storeDB, entityCoverages); err != nil {
		return fmt.Errorf("failed to store coverage: %w", err)
	}

	// If we have per-test attribution, populate test_entity_map
	var testMappings int
	if gocoverData.HasPerTestAttribution() {
		fmt.Fprintf(os.Stderr, "Mapping per-test coverage to entities...\n")
		testMappings, err = coverage.StoreTestEntityMappings(storeDB, gocoverData, basePath)
		if err != nil {
			return fmt.Errorf("failed to store test mappings: %w", err)
		}
	}

	// Display summary
	printCoverageSummary(entityCoverages, gocoverData.HasPerTestAttribution(), testMappings)

	return nil
}

// printCoverageSummary prints a summary of imported coverage
func printCoverageSummary(entityCoverages []coverage.EntityCoverage, hasTestAttribution bool, testMappings int) {
	var totalCovered, totalUncovered int
	for _, cov := range entityCoverages {
		totalCovered += len(cov.CoveredLines)
		totalUncovered += len(cov.UncoveredLines)
	}

	totalLines := totalCovered + totalUncovered
	var overallPercent float64
	if totalLines > 0 {
		overallPercent = float64(totalCovered) / float64(totalLines) * 100.0
	}

	fmt.Fprintf(os.Stderr, "\nCoverage imported successfully:\n")
	fmt.Fprintf(os.Stderr, "  Entities with coverage: %d\n", len(entityCoverages))
	fmt.Fprintf(os.Stderr, "  Lines covered: %d / %d (%.1f%%)\n", totalCovered, totalLines, overallPercent)

	if hasTestAttribution {
		fmt.Fprintf(os.Stderr, "  Test→entity mappings: %d\n", testMappings)
		fmt.Fprintf(os.Stderr, "\nPer-test attribution enabled. Use 'cx test-impact' and 'cx show --coverage' to see which tests cover your code.\n")
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Open store
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Get coverage statistics
	stats, err := coverage.GetCoverageStats(storeDB)
	if err != nil {
		return fmt.Errorf("failed to get coverage stats: %w", err)
	}

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Get formatter
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	// Format and output stats
	return formatter.FormatToWriter(cmd.OutOrStdout(), stats, output.DensityMedium)
}

package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// testCmd represents the test command - consolidated from test-impact and coverage
var testCmd = &cobra.Command{
	Use:   "test [file-or-path]",
	Short: "Smart test selection, coverage analysis, and test data management",
	Long: `Smart test selection based on code changes and coverage analysis.

This command consolidates test-related functionality:
  - Smart test selection (formerly test-impact)
  - Coverage gap analysis (--gaps)
  - Coverage status (--coverage)
  - Coverage import (test coverage import)

Smart Test Selection:
Given a change, identify exactly which tests need to run. Uses coverage data
and the call graph to find:
  - Direct tests: Tests that directly call changed entities
  - Indirect tests: Tests that call callers of changed entities
  - Integration tests: Tests in files with "integration" in the path

Coverage Analysis:
Use --gaps to find coverage gaps weighted by code importance, or --coverage
to show overall coverage statistics.

Examples:
  # Smart test selection (default uses --diff)
  cx test                              # Tests affected by uncommitted changes
  cx test --diff                       # Same as above, explicit
  cx test internal/auth/login.go       # Tests for specific file
  cx test --commit HEAD~1              # Tests affected by specific commit

  # Running tests
  cx test --run                        # Actually run the selected tests
  cx test --output-command             # Just output the go test command

  # Coverage analysis
  cx test --gaps                       # Show coverage gaps (see 'cx gaps')
  cx test --gaps --keystones-only      # Only keystone gaps
  cx test --coverage                   # Show coverage summary

  # Import coverage data
  cx test coverage import coverage.out
  cx test coverage import .coverage/   # Per-test coverage directory

Flags:
  --diff            Use git diff HEAD to find changed files (default behavior)
  --commit          Use specific commit to find changed files
  --file            Specify a file path directly
  --depth           Depth for indirect test discovery (default: 2)
  --output-command  Only output the go test command (for scripts)
  --run             Actually run the selected tests
  --gaps            Show coverage gaps (calls gaps logic)
  --coverage        Show coverage summary (calls coverage status logic)
  --keystones-only  With --gaps: only show keystones
  --threshold       With --gaps: coverage threshold (default: 75)`,
	RunE: runTest,
}

// testCoverageCmd is a subcommand for coverage import
var testCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Coverage data management",
	Long:  `Manage test coverage data. Use 'cx test coverage import' to import coverage files.`,
}

// testDiscoverCmd discovers test functions in the codebase
var testDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover test functions in the codebase",
	Long: `Scan the codebase for test files and discover test functions.

This command:
  1. Walks the project directory looking for test files
  2. Parses test files to identify test functions
  3. Stores discovered tests as entities in the database
  4. Links tests to coverage data if available

Supported patterns:
  Go:         func Test*, Benchmark*, Example*, Fuzz* in *_test.go
  TypeScript: it(), test(), describe() in *.test.ts, *.spec.ts
  JavaScript: it(), test(), describe() in *.test.js, *.spec.js
  Python:     test_* in test_*.py, *_test.py
  Rust:       #[test] in tests/ or *_test.rs
  Java:       @Test in *Test.java

Examples:
  cx test discover              # Discover all tests in project
  cx test discover --language go  # Only Go tests`,
	RunE: runTestDiscover,
}

// testListCmd lists discovered test functions
var testListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered test functions",
	Long: `List test functions discovered in the codebase.

Use 'cx test discover' first to scan for tests.

Examples:
  cx test list                  # List all discovered tests
  cx test list --language go    # Only Go tests
  cx test list --for-entity <id>  # Tests covering a specific entity`,
	RunE: runTestList,
}

// testCoverageImportCmd imports coverage data
var testCoverageImportCmd = &cobra.Command{
	Use:   "import <coverage.out | directory>",
	Short: "Import coverage data from coverage.out file or directory",
	Long: `Parse Go coverage data and map coverage blocks to entities.

Supported formats:
  1. coverage.out file (from go test -coverprofile)
  2. Per-test coverage directory (TestName.out files) - RECOMMENDED for test selection
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
  cx test coverage import coverage.out

  # Per-test coverage directory (RECOMMENDED for smart test selection)
  mkdir -p .coverage
  for test in $(go test -list '.*' ./... 2>/dev/null | grep '^Test'); do
    go test -coverprofile=.coverage/${test}.out -run "^${test}$" ./... 2>/dev/null
  done
  cx test coverage import .coverage/

The per-test mode populates test_entity_map, enabling smart test selection
and showing 'tested_by' in 'cx show --coverage'.`,
	Args: cobra.ExactArgs(1),
	RunE: runTestCoverageImport,
}

var (
	// Test selection flags
	testDiff          bool
	testCommit        string
	testFile          string
	testDepth         int
	testOutputCommand bool
	testRun           bool

	// Coverage analysis flags
	testShowGaps      bool
	testShowCoverage  bool
	testKeystonesOnly bool
	testThreshold     int

	// Coverage import flags
	testCoverageBasePath string

	// Test discover/list flags
	testDiscoverLanguage string
	testListLanguage     string
	testListForEntity    string
)

func init() {
	rootCmd.AddCommand(testCmd)

	// Test selection flags
	testCmd.Flags().BoolVar(&testDiff, "diff", false, "Use git diff HEAD to find changed files")
	testCmd.Flags().StringVar(&testCommit, "commit", "", "Use specific commit to find changed files")
	testCmd.Flags().StringVar(&testFile, "file", "", "Specify a file path directly")
	testCmd.Flags().IntVar(&testDepth, "depth", 2, "Depth for indirect test discovery")
	testCmd.Flags().BoolVar(&testOutputCommand, "output-command", false, "Only output the go test command")
	testCmd.Flags().BoolVar(&testRun, "run", false, "Actually run the selected tests")

	// Coverage analysis flags
	testCmd.Flags().BoolVar(&testShowGaps, "gaps", false, "Show coverage gaps (calls gaps logic)")
	testCmd.Flags().BoolVar(&testShowCoverage, "coverage", false, "Show coverage summary")
	testCmd.Flags().BoolVar(&testKeystonesOnly, "keystones-only", false, "With --gaps: only show keystones")
	testCmd.Flags().IntVar(&testThreshold, "threshold", 75, "With --gaps: coverage threshold percentage")

	// Add coverage subcommand
	testCmd.AddCommand(testCoverageCmd)
	testCoverageCmd.AddCommand(testCoverageImportCmd)

	// Coverage import flags
	testCoverageImportCmd.Flags().StringVar(&testCoverageBasePath, "base-path", "", "Base path for normalizing file paths (default: current directory)")

	// Add discover subcommand
	testCmd.AddCommand(testDiscoverCmd)
	testDiscoverCmd.Flags().StringVar(&testDiscoverLanguage, "language", "", "Filter by language (go, typescript, javascript, python, rust, java)")

	// Add list subcommand
	testCmd.AddCommand(testListCmd)
	testListCmd.Flags().StringVar(&testListLanguage, "language", "", "Filter by language")
	testListCmd.Flags().StringVar(&testListForEntity, "for-entity", "", "Show tests covering this entity ID")
}

func runTest(cmd *cobra.Command, args []string) error {
	// Handle --coverage flag (show coverage status)
	if testShowCoverage {
		return runTestCoverageStatus(cmd)
	}

	// Handle --gaps flag (show coverage gaps)
	if testShowGaps {
		return runTestGaps(cmd)
	}

	// Otherwise, run smart test selection
	return runTestSelection(cmd, args)
}

// runTestCoverageStatus shows coverage statistics (delegates to coverage status logic)
func runTestCoverageStatus(cmd *cobra.Command) error {
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

// runTestGaps shows coverage gaps (delegates to gaps logic)
func runTestGaps(cmd *cobra.Command) error {
	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Open store
	storeDB, err := openStore()
	if err != nil {
		return err
	}
	defer storeDB.Close()

	// Get all active entities
	entities, err := storeDB.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		return fmt.Errorf("no entities found - run 'cx scan' first")
	}

	// Check if metrics exist
	hasMetrics := false
	for _, e := range entities {
		if m, _ := storeDB.GetMetrics(e.ID); m != nil {
			hasMetrics = true
			break
		}
	}

	if !hasMetrics {
		return fmt.Errorf("no metrics found - run 'cx rank' first to compute importance")
	}

	// Check if coverage data exists
	hasCoverage := false
	for _, e := range entities {
		if cov, _ := coverage.GetEntityCoverage(storeDB, e.ID); cov != nil {
			hasCoverage = true
			break
		}
	}

	if !hasCoverage {
		return fmt.Errorf("no coverage data found - run 'cx test coverage import' first")
	}

	// Build list of gaps
	var gaps []coverageGap
	keystoneCount := 0

	for _, e := range entities {
		// Get metrics
		m, err := storeDB.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue
		}

		// Count keystones
		if m.PageRank >= cfg.Metrics.KeystoneThreshold {
			keystoneCount++
		}

		// Get coverage
		cov, err := coverage.GetEntityCoverage(storeDB, e.ID)
		if err != nil {
			// No coverage data for this entity - treat as 0% coverage
			cov = &coverage.EntityCoverage{
				EntityID:        e.ID,
				CoveragePercent: 0,
				CoveredLines:    []int{},
				UncoveredLines:  []int{},
			}
		}

		// Check if below threshold
		if cov.CoveragePercent >= float64(testThreshold) {
			continue
		}

		// Skip if keystones-only mode and not a keystone
		if testKeystonesOnly && m.PageRank < cfg.Metrics.KeystoneThreshold {
			continue
		}

		// Calculate risk score
		riskScore := (1 - cov.CoveragePercent/100.0) * m.PageRank * float64(m.InDegree)

		// Determine risk category
		riskCategory := categorizeRisk(m, cov, cfg)

		gaps = append(gaps, coverageGap{
			entity:       e,
			metrics:      m,
			coverage:     cov,
			riskScore:    riskScore,
			riskCategory: riskCategory,
		})
	}

	if len(gaps) == 0 {
		fmt.Fprintf(os.Stderr, "No coverage gaps found! All entities meet the threshold.\n")
		return nil
	}

	// Sort by risk score descending
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].riskScore > gaps[j].riskScore
	})

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Group by risk category
	gapsByRisk := groupGapsByRisk(gaps)

	// Build output structure
	outputData := buildGapsOutput(gapsByRisk, keystoneCount)

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), outputData, output.DensityMedium)
}

// runTestSelection performs smart test selection (formerly test-impact)
func runTestSelection(cmd *cobra.Command, args []string) error {
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

	// Determine changed files
	var changedFiles []string
	var changedFileMap map[string][]int // file -> changed line numbers

	// Determine mode: explicit flags, args, or default to --diff
	useFile := testFile != ""
	useCommit := testCommit != ""
	useDiff := testDiff
	useArgs := len(args) > 0

	// If no flags and no args specified, default to --diff behavior
	if !useFile && !useCommit && !useDiff && !useArgs {
		useDiff = true
	}

	if useFile {
		// Use --file flag
		target := testFile
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", target, err)
		}

		if info.IsDir() {
			err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
					changedFiles = append(changedFiles, path)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk directory: %w", err)
			}
		} else {
			changedFiles = []string{target}
		}
		changedFileMap = make(map[string][]int)
	} else if useDiff {
		// Use git diff HEAD
		changedFiles, changedFileMap, err = getTestChangedFilesFromDiff("HEAD")
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}
	} else if useCommit {
		// Use specific commit
		changedFiles, changedFileMap, err = getTestChangedFilesFromCommit(testCommit)
		if err != nil {
			return fmt.Errorf("failed to get commit diff: %w", err)
		}
	} else if useArgs {
		// Use provided file/path
		target := args[0]
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", target, err)
		}

		if info.IsDir() {
			err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
					changedFiles = append(changedFiles, path)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk directory: %w", err)
			}
		} else {
			changedFiles = []string{target}
		}
		changedFileMap = make(map[string][]int)
	}

	if len(changedFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No changed files found\n")
		return nil
	}

	// Build graph for traversal
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Find changed entities
	changedEntityIDs := make(map[string]bool)
	for _, filePath := range changedFiles {
		entities, err := storeDB.QueryEntities(store.EntityFilter{
			FilePath: filePath,
			Status:   "active",
		})
		if err != nil {
			continue
		}

		// Filter by changed lines if we have line info
		lineNums, hasLineInfo := changedFileMap[filePath]
		for _, e := range entities {
			if hasLineInfo {
				// Check if entity overlaps with changed lines
				if testEntityOverlapsLines(e, lineNums) {
					changedEntityIDs[e.ID] = true
				}
			} else {
				// No line info, include all entities in file
				changedEntityIDs[e.ID] = true
			}
		}
	}

	if len(changedEntityIDs) == 0 {
		fmt.Fprintf(os.Stderr, "No entities found in changed files\n")
		return nil
	}

	// Find tests that cover changed entities (direct)
	directTests := make(map[string]*testSelectionInfo)
	for entityID := range changedEntityIDs {
		tests, err := getTestsForEntityID(storeDB, entityID)
		if err != nil {
			continue
		}
		for _, t := range tests {
			key := t.testFile + "::" + t.testName
			if _, exists := directTests[key]; !exists {
				t.impactType = "direct"
				directTests[key] = t
			}
		}
	}

	// Find tests that cover callers of changed entities (indirect)
	indirectTests := make(map[string]*testSelectionInfo)
	if testDepth > 0 {
		// BFS to find callers up to depth
		visited := make(map[string]int)
		queue := make([]string, 0, len(changedEntityIDs))
		for entityID := range changedEntityIDs {
			queue = append(queue, entityID)
			visited[entityID] = 0
		}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			currentDepth := visited[current]

			if currentDepth >= testDepth {
				continue
			}

			// Get callers (predecessors in the graph)
			callers := g.Predecessors(current)
			for _, caller := range callers {
				if _, seen := visited[caller]; seen {
					continue
				}
				visited[caller] = currentDepth + 1

				// Find tests for this caller
				tests, err := getTestsForEntityID(storeDB, caller)
				if err != nil {
					continue
				}
				for _, t := range tests {
					key := t.testFile + "::" + t.testName
					// Skip if already in direct tests
					if _, inDirect := directTests[key]; !inDirect {
						if _, exists := indirectTests[key]; !exists {
							t.impactType = "indirect"
							indirectTests[key] = t
						}
					}
				}

				queue = append(queue, caller)
			}
		}
	}

	// Find integration tests
	integrationTests := make(map[string]*testSelectionInfo)
	integrationTestFiles, err := findTestIntegrationFiles(storeDB)
	if err == nil {
		for _, testFile := range integrationTestFiles {
			if isTestIntegrationRelevant(testFile, changedFiles) {
				tests, err := getTestsInFileByPath(storeDB, testFile)
				if err != nil {
					continue
				}
				for _, t := range tests {
					key := t.testFile + "::" + t.testName
					if _, inDirect := directTests[key]; !inDirect {
						if _, inIndirect := indirectTests[key]; !inIndirect {
							if _, exists := integrationTests[key]; !exists {
								t.impactType = "integration"
								integrationTests[key] = t
							}
						}
					}
				}
			}
		}
	}

	// Get total test count
	totalTests, err := getTestTotalCount(storeDB)
	if err != nil {
		totalTests = 0
	}

	affectedTestCount := len(directTests) + len(indirectTests) + len(integrationTests)

	// Generate go test command
	testCommand := generateTestRunCommand(directTests, indirectTests, integrationTests)

	// If --run, execute the tests
	if testRun {
		if testCommand == "# No tests affected" {
			fmt.Fprintf(os.Stderr, "No tests to run\n")
			return nil
		}
		fmt.Fprintf(os.Stderr, "Running: %s\n", testCommand)
		execCmd := exec.Command("sh", "-c", testCommand)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		return execCmd.Run()
	}

	// If --output-command, just print the command
	if testOutputCommand {
		fmt.Println(testCommand)
		return nil
	}

	// Build output structure
	testOutput := &TestOutput{
		AffectedTests: &TestAffectedTests{
			Direct:      convertTestSelectionToList(directTests),
			Indirect:    convertTestSelectionToList(indirectTests),
			Integration: convertTestSelectionToList(integrationTests),
		},
		Summary: &TestSummary{
			TotalTests:    totalTests,
			AffectedTests: affectedTestCount,
			TimeSaved:     estimateTestTimeSaved(totalTests, affectedTestCount),
		},
		Command: testCommand,
	}

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), testOutput, density)
}

// runTestCoverageImport imports coverage data (delegates to coverage import logic)
func runTestCoverageImport(cmd *cobra.Command, args []string) error {
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
	basePath := testCoverageBasePath
	if basePath == "" {
		basePath = "."
	}

	// Auto-detect format
	if coverage.IsPerTestCoverageDir(inputPath) {
		return runTestImportPerTestDir(storeDB, inputPath, basePath)
	}
	if coverage.IsGOCOVERDIR(inputPath) {
		return runTestImportGOCOVERDIR(storeDB, inputPath, basePath)
	}

	// Traditional coverage.out file
	return runTestImportCoverageOut(storeDB, inputPath, basePath)
}

// runTestImportCoverageOut handles traditional coverage.out file import
func runTestImportCoverageOut(storeDB *store.Store, coverageFile, basePath string) error {
	fmt.Fprintf(os.Stderr, "Parsing coverage file: %s\n", coverageFile)
	coverageData, err := coverage.ParseCoverageFile(coverageFile)
	if err != nil {
		return fmt.Errorf("failed to parse coverage file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Found %d coverage blocks (mode: %s)\n", len(coverageData.Blocks), coverageData.Mode)

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

	fmt.Fprintf(os.Stderr, "Storing coverage for %d entities...\n", len(entityCoverages))
	if err := coverage.StoreCoverage(storeDB, entityCoverages); err != nil {
		return fmt.Errorf("failed to store coverage: %w", err)
	}

	printTestCoverageSummary(entityCoverages, false, 0)
	return nil
}

// runTestImportPerTestDir handles a directory of per-test coverage.out files
func runTestImportPerTestDir(storeDB *store.Store, dirPath, basePath string) error {
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

	fmt.Fprintf(os.Stderr, "Storing coverage for %d entities...\n", len(entityCoverages))
	if err := coverage.StoreCoverage(storeDB, entityCoverages); err != nil {
		return fmt.Errorf("failed to store coverage: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Mapping per-test coverage to entities...\n")
	testMappings, err := coverage.StoreTestEntityMappings(storeDB, perTestData, basePath)
	if err != nil {
		return fmt.Errorf("failed to store test mappings: %w", err)
	}

	printTestCoverageSummary(entityCoverages, true, testMappings)
	return nil
}

// runTestImportGOCOVERDIR handles Go 1.20+ GOCOVERDIR import
func runTestImportGOCOVERDIR(storeDB *store.Store, dirPath, basePath string) error {
	fmt.Fprintf(os.Stderr, "Parsing GOCOVERDIR: %s\n", dirPath)

	gocoverData, err := coverage.ParseGOCOVERDIR(dirPath)
	if err != nil {
		return fmt.Errorf("failed to parse GOCOVERDIR: %w", err)
	}

	if gocoverData.HasPerTestAttribution() {
		fmt.Fprintf(os.Stderr, "Found %d tests with per-test coverage\n", len(gocoverData.PerTest))
	}
	if gocoverData.Aggregate != nil {
		fmt.Fprintf(os.Stderr, "Found %d coverage blocks (mode: %s)\n",
			len(gocoverData.Aggregate.Blocks), gocoverData.Aggregate.Mode)
	}

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

	fmt.Fprintf(os.Stderr, "Storing coverage for %d entities...\n", len(entityCoverages))
	if err := coverage.StoreCoverage(storeDB, entityCoverages); err != nil {
		return fmt.Errorf("failed to store coverage: %w", err)
	}

	var testMappings int
	if gocoverData.HasPerTestAttribution() {
		fmt.Fprintf(os.Stderr, "Mapping per-test coverage to entities...\n")
		testMappings, err = coverage.StoreTestEntityMappings(storeDB, gocoverData, basePath)
		if err != nil {
			return fmt.Errorf("failed to store test mappings: %w", err)
		}
	}

	printTestCoverageSummary(entityCoverages, gocoverData.HasPerTestAttribution(), testMappings)
	return nil
}

// printTestCoverageSummary prints a summary of imported coverage
func printTestCoverageSummary(entityCoverages []coverage.EntityCoverage, hasTestAttribution bool, testMappings int) {
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
		fmt.Fprintf(os.Stderr, "\nPer-test attribution enabled. Use 'cx test' and 'cx show --coverage' to see which tests cover your code.\n")
	}
}

// TestOutput represents the output structure for cx test
type TestOutput struct {
	AffectedTests *TestAffectedTests `yaml:"affected_tests" json:"affected_tests"`
	Summary       *TestSummary       `yaml:"summary" json:"summary"`
	Command       string             `yaml:"command" json:"command"`
}

// TestAffectedTests groups tests by impact type
type TestAffectedTests struct {
	Direct      []string `yaml:"direct,omitempty" json:"direct,omitempty"`
	Indirect    []string `yaml:"indirect,omitempty" json:"indirect,omitempty"`
	Integration []string `yaml:"integration,omitempty" json:"integration,omitempty"`
}

// TestSummary contains statistics about test selection
type TestSummary struct {
	TotalTests    int    `yaml:"total_tests" json:"total_tests"`
	AffectedTests int    `yaml:"affected_tests" json:"affected_tests"`
	TimeSaved     string `yaml:"time_saved" json:"time_saved"`
}

// testSelectionInfo holds information about a test for selection
type testSelectionInfo struct {
	testFile   string
	testName   string
	impactType string // direct, indirect, integration
}

// getTestChangedFilesFromDiff gets changed files from git diff
func getTestChangedFilesFromDiff(ref string) ([]string, map[string][]int, error) {
	cmd := exec.Command("git", "diff", ref, "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git diff failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasSuffix(line, ".go") && !strings.HasSuffix(line, "_test.go") {
			files = append(files, line)
		}
	}

	lineMap := make(map[string][]int)
	for _, file := range files {
		lines, err := getTestChangedLinesInFile(ref, file)
		if err == nil && len(lines) > 0 {
			lineMap[file] = lines
		}
	}

	return files, lineMap, nil
}

// getTestChangedFilesFromCommit gets changed files from a specific commit
func getTestChangedFilesFromCommit(commit string) ([]string, map[string][]int, error) {
	cmd := exec.Command("git", "diff", commit+"^", commit, "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git diff failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasSuffix(line, ".go") && !strings.HasSuffix(line, "_test.go") {
			files = append(files, line)
		}
	}

	lineMap := make(map[string][]int)
	for _, file := range files {
		execCmd := exec.Command("git", "diff", commit+"^", commit, "--unified=0", file)
		output, err := execCmd.Output()
		if err == nil {
			lines := parseTestChangedLinesFromDiff(string(output))
			if len(lines) > 0 {
				lineMap[file] = lines
			}
		}
	}

	return files, lineMap, nil
}

// getTestChangedLinesInFile gets the changed line numbers in a file
func getTestChangedLinesInFile(ref string, file string) ([]int, error) {
	cmd := exec.Command("git", "diff", ref, "--unified=0", file)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseTestChangedLinesFromDiff(string(output)), nil
}

// parseTestChangedLinesFromDiff parses changed line numbers from git diff output
func parseTestChangedLinesFromDiff(diffOutput string) []int {
	var lines []int

	for _, line := range strings.Split(diffOutput, "\n") {
		if strings.HasPrefix(line, "@@") {
			parts := strings.Split(line, " ")
			for _, part := range parts {
				if strings.HasPrefix(part, "+") {
					part = strings.TrimPrefix(part, "+")
					if idx := strings.Index(part, ","); idx != -1 {
						var start, count int
						fmt.Sscanf(part[:idx], "%d", &start)
						fmt.Sscanf(part[idx+1:], "%d", &count)
						for i := start; i < start+count; i++ {
							lines = append(lines, i)
						}
					} else {
						var lineNum int
						fmt.Sscanf(part, "%d", &lineNum)
						lines = append(lines, lineNum)
					}
				}
			}
		}
	}

	return lines
}

// testEntityOverlapsLines checks if an entity overlaps with any changed lines
func testEntityOverlapsLines(e *store.Entity, changedLines []int) bool {
	if e.LineEnd == nil {
		for _, line := range changedLines {
			if line == e.LineStart {
				return true
			}
		}
		return false
	}

	for _, line := range changedLines {
		if line >= e.LineStart && line <= *e.LineEnd {
			return true
		}
	}
	return false
}

// getTestsForEntityID gets tests that cover a specific entity
func getTestsForEntityID(s *store.Store, entityID string) ([]*testSelectionInfo, error) {
	rows, err := s.DB().Query(`
		SELECT test_file, test_name
		FROM test_entity_map
		WHERE entity_id = ?
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*testSelectionInfo
	for rows.Next() {
		var t testSelectionInfo
		if err := rows.Scan(&t.testFile, &t.testName); err != nil {
			continue
		}
		tests = append(tests, &t)
	}

	return tests, rows.Err()
}

// getTestsInFileByPath gets all tests in a specific test file
func getTestsInFileByPath(s *store.Store, testFile string) ([]*testSelectionInfo, error) {
	rows, err := s.DB().Query(`
		SELECT DISTINCT test_file, test_name
		FROM test_entity_map
		WHERE test_file = ?
	`, testFile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*testSelectionInfo
	for rows.Next() {
		var t testSelectionInfo
		if err := rows.Scan(&t.testFile, &t.testName); err != nil {
			continue
		}
		tests = append(tests, &t)
	}

	return tests, rows.Err()
}

// findTestIntegrationFiles finds test files that might be integration tests
func findTestIntegrationFiles(s *store.Store) ([]string, error) {
	rows, err := s.DB().Query(`
		SELECT DISTINCT test_file
		FROM test_entity_map
		WHERE test_file LIKE '%integration%' OR test_file LIKE '%e2e%'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var file string
		if err := rows.Scan(&file); err != nil {
			continue
		}
		files = append(files, file)
	}

	return files, rows.Err()
}

// isTestIntegrationRelevant checks if an integration test might be relevant
func isTestIntegrationRelevant(testFile string, changedFiles []string) bool {
	testDir := filepath.Dir(testFile)

	for _, changedFile := range changedFiles {
		changedDir := filepath.Dir(changedFile)

		if strings.HasPrefix(testDir, changedDir) || strings.HasPrefix(changedDir, testDir) {
			return true
		}

		testParts := strings.Split(testDir, string(filepath.Separator))
		changedParts := strings.Split(changedDir, string(filepath.Separator))

		if len(testParts) >= 2 && len(changedParts) >= 2 {
			if testParts[0] == changedParts[0] && testParts[1] == changedParts[1] {
				return true
			}
		}
	}

	return false
}

// getTestTotalCount gets the total number of tests in the project
func getTestTotalCount(s *store.Store) (int, error) {
	var count int
	err := s.DB().QueryRow(`
		SELECT COUNT(DISTINCT test_file || '::' || test_name)
		FROM test_entity_map
	`).Scan(&count)

	if err == sql.ErrNoRows {
		return 0, nil
	}

	return count, err
}

// generateTestRunCommand generates a go test command to run affected tests
func generateTestRunCommand(direct, indirect, integration map[string]*testSelectionInfo) string {
	if len(direct) == 0 && len(indirect) == 0 && len(integration) == 0 {
		return "# No tests affected"
	}

	testNames := make(map[string]bool)
	for _, t := range direct {
		testNames[t.testName] = true
	}
	for _, t := range indirect {
		testNames[t.testName] = true
	}
	for _, t := range integration {
		testNames[t.testName] = true
	}

	names := make([]string, 0, len(testNames))
	for name := range testNames {
		names = append(names, name)
	}
	sort.Strings(names)

	pattern := "^(" + strings.Join(names, "|") + ")$"

	var buf bytes.Buffer
	buf.WriteString("go test -run '")
	buf.WriteString(pattern)
	buf.WriteString("' ./...")

	return buf.String()
}

// convertTestSelectionToList converts test map to sorted list of test identifiers
func convertTestSelectionToList(tests map[string]*testSelectionInfo) []string {
	if len(tests) == 0 {
		return nil
	}

	list := make([]string, 0, len(tests))
	for _, t := range tests {
		identifier := fmt.Sprintf("%s @ %s", t.testName, t.testFile)
		list = append(list, identifier)
	}

	sort.Strings(list)
	return list
}

// estimateTestTimeSaved estimates time saved by running fewer tests
func estimateTestTimeSaved(totalTests, affectedTests int) string {
	if totalTests == 0 || affectedTests >= totalTests {
		return "No time saved (all tests need to run)"
	}

	const avgTestTimeSeconds = 1.5

	totalTime := float64(totalTests) * avgTestTimeSeconds
	affectedTime := float64(affectedTests) * avgTestTimeSeconds
	savedSeconds := totalTime - affectedTime

	if savedSeconds < 60 {
		return fmt.Sprintf("Run %d tests (~%.0fs) instead of %d tests (~%.0fs)",
			affectedTests, affectedTime, totalTests, totalTime)
	}

	totalMinutes := totalTime / 60
	affectedMinutes := affectedTime / 60

	return fmt.Sprintf("Run %d tests (~%.1fmin) instead of %d tests (~%.1fmin)",
		affectedTests, affectedMinutes, totalTests, totalMinutes)
}

// runTestDiscover discovers and stores test functions
func runTestDiscover(cmd *cobra.Command, args []string) error {
	// Get current working directory as base path
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

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

	// Create test discovery instance
	discovery := coverage.NewTestDiscovery(storeDB, cwd)

	fmt.Fprintf(os.Stderr, "Discovering test functions...\n")

	// Discover tests
	tests, err := discovery.DiscoverTests()
	if err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	// Filter by language if specified
	if testDiscoverLanguage != "" {
		var filtered []coverage.DiscoveredTest
		for _, t := range tests {
			if t.Language == testDiscoverLanguage {
				filtered = append(filtered, t)
			}
		}
		tests = filtered
	}

	if len(tests) == 0 {
		fmt.Fprintf(os.Stderr, "No test functions found\n")
		return nil
	}

	// Store discovered tests
	fmt.Fprintf(os.Stderr, "Found %d test functions, storing...\n", len(tests))
	if err := discovery.StoreDiscoveredTests(tests); err != nil {
		return fmt.Errorf("store tests: %w", err)
	}

	// Build output
	testOutput := &TestDiscoverOutput{
		TotalTests: len(tests),
		ByLanguage: make(map[string]int),
		ByType:     make(map[string]int),
	}

	for _, t := range tests {
		testOutput.ByLanguage[t.Language]++
		testOutput.ByType[string(t.TestType)]++
	}

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), testOutput, output.DensityMedium)
}

// TestDiscoverOutput represents the output of test discovery
type TestDiscoverOutput struct {
	TotalTests int            `yaml:"total_tests" json:"total_tests"`
	ByLanguage map[string]int `yaml:"by_language" json:"by_language"`
	ByType     map[string]int `yaml:"by_type" json:"by_type"`
}

// runTestList lists discovered test functions
func runTestList(cmd *cobra.Command, args []string) error {
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

	// If --for-entity is specified, show tests that cover that entity
	if testListForEntity != "" {
		return runTestListForEntity(cmd, storeDB, testListForEntity)
	}

	// Get discovered tests
	tests, err := coverage.GetDiscoveredTests(storeDB, testListLanguage)
	if err != nil {
		return fmt.Errorf("get tests: %w", err)
	}

	if len(tests) == 0 {
		fmt.Fprintf(os.Stderr, "No discovered tests found. Run 'cx test discover' first.\n")
		return nil
	}

	// Build output
	testListOutput := &TestListOutput{
		Tests: make([]TestListItem, 0, len(tests)),
	}

	for _, t := range tests {
		testListOutput.Tests = append(testListOutput.Tests, TestListItem{
			Name:     t.Name,
			FilePath: t.FilePath,
			Line:     t.StartLine,
			Language: t.Language,
			Type:     string(t.TestType),
		})
	}

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), testListOutput, density)
}

// runTestListForEntity shows tests that cover a specific entity
func runTestListForEntity(cmd *cobra.Command, storeDB *store.Store, entityID string) error {
	tests, err := coverage.GetTestsForEntity(storeDB, entityID)
	if err != nil {
		return fmt.Errorf("get tests for entity: %w", err)
	}

	if len(tests) == 0 {
		fmt.Fprintf(os.Stderr, "No tests found covering entity %s\n", entityID)
		return nil
	}

	// Build output
	testOutput := &TestsForEntityOutput{
		EntityID: entityID,
		Tests:    make([]TestListItem, 0, len(tests)),
	}

	for _, t := range tests {
		testOutput.Tests = append(testOutput.Tests, TestListItem{
			Name:     t.TestName,
			FilePath: t.TestFile,
		})
	}

	// Parse format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), testOutput, output.DensityMedium)
}

// TestListOutput represents the output of listing tests
type TestListOutput struct {
	Tests []TestListItem `yaml:"tests" json:"tests"`
}

// TestListItem represents a single test in the list
type TestListItem struct {
	Name     string `yaml:"name" json:"name"`
	FilePath string `yaml:"file,omitempty" json:"file,omitempty"`
	Line     int    `yaml:"line,omitempty" json:"line,omitempty"`
	Language string `yaml:"language,omitempty" json:"language,omitempty"`
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
}

// TestsForEntityOutput represents tests covering a specific entity
type TestsForEntityOutput struct {
	EntityID string         `yaml:"entity_id" json:"entity_id"`
	Tests    []TestListItem `yaml:"tests" json:"tests"`
}

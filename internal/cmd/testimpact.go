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
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// testImpactCmd represents the test-impact command
var testImpactCmd = &cobra.Command{
	Use:   "test-impact [file-or-path]",
	Short: "Smart test selection - identify which tests need to run",
	Long: `Given a change, identify exactly which tests need to run.

This command analyzes code changes and uses the dependency graph to determine
which tests are affected. It uses coverage data and the call graph to find:
- Direct tests: Tests that directly call changed entities
- Indirect tests: Tests that call callers of changed entities
- Integration tests: Tests in files with "integration" in the path

The output includes a command to run only the affected tests, saving time
in CI/CD pipelines by skipping unrelated tests.

Smart test selection requires:
  1. Coverage data: Run 'cx coverage import coverage.out' first
  2. Test mapping: Populated test_entity_map table (from Phase 1)

Examples:
  # What tests cover this file?
  cx test-impact internal/auth/login.go

  # What tests affected by uncommitted changes?
  cx test-impact --diff

  # What tests affected by this commit?
  cx test-impact --commit HEAD~1

  # Just output the go test command
  cx test-impact --diff --output-command

  # Specify depth for indirect tests
  cx test-impact internal/auth/login.go --depth 3

Output Format:
  YAML/JSON structure with:
  - affected_tests: Map of test names grouped by impact level
    - direct: Tests that directly call changed entities
    - indirect: Tests that call callers of changed entities (transitive)
    - integration: Integration tests that may be affected
  - summary: Statistics about test selection
    - total_tests: Total number of tests in project
    - affected_tests: Number of tests that need to run
    - time_saved: Estimated time savings
  - command: Shell command to run affected tests

Flags:
  --diff          Use git diff HEAD to find changed files
  --commit        Use specific commit to find changed files
  --depth         Depth for indirect test discovery (default: 2)
  --output-command  Only output the go test command (for scripts)`,
	RunE: runTestImpact,
}

var (
	testImpactDiff          bool
	testImpactCommit        string
	testImpactDepth         int
	testImpactOutputCommand bool
)

func init() {
	rootCmd.AddCommand(testImpactCmd)

	testImpactCmd.Flags().BoolVar(&testImpactDiff, "diff", false, "Use git diff HEAD to find changed files")
	testImpactCmd.Flags().StringVar(&testImpactCommit, "commit", "", "Use specific commit to find changed files")
	testImpactCmd.Flags().IntVar(&testImpactDepth, "depth", 2, "Depth for indirect test discovery")
	testImpactCmd.Flags().BoolVar(&testImpactOutputCommand, "output-command", false, "Only output the go test command")

	// Deprecate this command in favor of cx test
	DeprecateCommand(testImpactCmd, DeprecationInfo{
		OldCommand: "cx test-impact",
		NewCommand: "cx test",
	})
}

// testInfo holds information about a test
type testInfo struct {
	testFile   string
	testName   string
	impactType string // direct, indirect, integration
}

func runTestImpact(cmd *cobra.Command, args []string) error {
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

	if testImpactDiff {
		// Use git diff HEAD
		changedFiles, changedFileMap, err = getChangedFilesFromDiff("HEAD")
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}
	} else if testImpactCommit != "" {
		// Use specific commit
		changedFiles, changedFileMap, err = getChangedFilesFromCommit(testImpactCommit)
		if err != nil {
			return fmt.Errorf("failed to get commit diff: %w", err)
		}
	} else if len(args) > 0 {
		// Use provided file/path
		target := args[0]

		// Check if it's a file or directory
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", target, err)
		}

		if info.IsDir() {
			// Find all Go files in directory
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
	} else {
		return fmt.Errorf("must provide file/path or use --diff/--commit")
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
				if entityOverlapsLines(e, lineNums) {
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
	directTests := make(map[string]*testInfo)
	for entityID := range changedEntityIDs {
		tests, err := getTestsForEntity(storeDB, entityID)
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
	indirectTests := make(map[string]*testInfo)
	if testImpactDepth > 0 {
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

			if currentDepth >= testImpactDepth {
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
				tests, err := getTestsForEntity(storeDB, caller)
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

	// Find integration tests (heuristic: tests in files with "integration" in path)
	integrationTests := make(map[string]*testInfo)
	integrationTestFiles, err := findIntegrationTestFiles(storeDB)
	if err == nil {
		for _, testFile := range integrationTestFiles {
			// Check if this test file might be relevant
			// (Simple heuristic: if any changed file is in a related package)
			if isIntegrationTestRelevant(testFile, changedFiles) {
				tests, err := getTestsInFile(storeDB, testFile)
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
	totalTests, err := getTotalTestCount(storeDB)
	if err != nil {
		totalTests = 0
	}

	affectedTestCount := len(directTests) + len(indirectTests) + len(integrationTests)

	// Generate go test command
	testCommand := generateTestCommand(directTests, indirectTests, integrationTests)

	// If --output-command, just print the command
	if testImpactOutputCommand {
		fmt.Println(testCommand)
		return nil
	}

	// Build output structure
	testImpactOutput := &TestImpactOutput{
		AffectedTests: &AffectedTests{
			Direct:      convertTestsToList(directTests),
			Indirect:    convertTestsToList(indirectTests),
			Integration: convertTestsToList(integrationTests),
		},
		Summary: &TestImpactSummary{
			TotalTests:     totalTests,
			AffectedTests:  affectedTestCount,
			TimeSaved:      estimateTimeSaved(totalTests, affectedTestCount),
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

	return formatter.FormatToWriter(cmd.OutOrStdout(), testImpactOutput, density)
}

// TestImpactOutput represents the output structure for cx test-impact
type TestImpactOutput struct {
	AffectedTests *AffectedTests      `yaml:"affected_tests" json:"affected_tests"`
	Summary       *TestImpactSummary  `yaml:"summary" json:"summary"`
	Command       string              `yaml:"command" json:"command"`
}

// AffectedTests groups tests by impact type
type AffectedTests struct {
	Direct      []string `yaml:"direct,omitempty" json:"direct,omitempty"`
	Indirect    []string `yaml:"indirect,omitempty" json:"indirect,omitempty"`
	Integration []string `yaml:"integration,omitempty" json:"integration,omitempty"`
}

// TestImpactSummary contains statistics about test selection
type TestImpactSummary struct {
	TotalTests    int    `yaml:"total_tests" json:"total_tests"`
	AffectedTests int    `yaml:"affected_tests" json:"affected_tests"`
	TimeSaved     string `yaml:"time_saved" json:"time_saved"`
}

// getChangedFilesFromDiff gets changed files from git diff
func getChangedFilesFromDiff(ref string) ([]string, map[string][]int, error) {
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

	// Get changed line numbers using git diff with unified diff format
	lineMap := make(map[string][]int)
	for _, file := range files {
		lines, err := getChangedLinesInFile(ref, file)
		if err == nil && len(lines) > 0 {
			lineMap[file] = lines
		}
	}

	return files, lineMap, nil
}

// getChangedFilesFromCommit gets changed files from a specific commit
func getChangedFilesFromCommit(commit string) ([]string, map[string][]int, error) {
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

	// Get changed line numbers
	lineMap := make(map[string][]int)
	for _, file := range files {
		cmd := exec.Command("git", "diff", commit+"^", commit, "--unified=0", file)
		output, err := cmd.Output()
		if err == nil {
			lines := parseChangedLinesFromDiff(string(output))
			if len(lines) > 0 {
				lineMap[file] = lines
			}
		}
	}

	return files, lineMap, nil
}

// getChangedLinesInFile gets the changed line numbers in a file
func getChangedLinesInFile(ref string, file string) ([]int, error) {
	cmd := exec.Command("git", "diff", ref, "--unified=0", file)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseChangedLinesFromDiff(string(output)), nil
}

// parseChangedLinesFromDiff parses changed line numbers from git diff output
func parseChangedLinesFromDiff(diffOutput string) []int {
	var lines []int

	// Parse diff hunks: @@ -start,count +start,count @@
	for _, line := range strings.Split(diffOutput, "\n") {
		if strings.HasPrefix(line, "@@") {
			// Extract the +start,count part
			parts := strings.Split(line, " ")
			for _, part := range parts {
				if strings.HasPrefix(part, "+") {
					// Parse +start,count or +start
					part = strings.TrimPrefix(part, "+")
					if idx := strings.Index(part, ","); idx != -1 {
						// Has count: +start,count
						var start, count int
						fmt.Sscanf(part[:idx], "%d", &start)
						fmt.Sscanf(part[idx+1:], "%d", &count)
						for i := start; i < start+count; i++ {
							lines = append(lines, i)
						}
					} else {
						// Just line number: +start
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

// entityOverlapsLines checks if an entity overlaps with any changed lines
func entityOverlapsLines(e *store.Entity, changedLines []int) bool {
	if e.LineEnd == nil {
		// Entity without end line - check if start line is in changed lines
		for _, line := range changedLines {
			if line == e.LineStart {
				return true
			}
		}
		return false
	}

	// Check if any changed line falls within entity range
	for _, line := range changedLines {
		if line >= e.LineStart && line <= *e.LineEnd {
			return true
		}
	}
	return false
}

// getTestsForEntity gets tests that cover a specific entity
func getTestsForEntity(s *store.Store, entityID string) ([]*testInfo, error) {
	rows, err := s.DB().Query(`
		SELECT test_file, test_name
		FROM test_entity_map
		WHERE entity_id = ?
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*testInfo
	for rows.Next() {
		var t testInfo
		if err := rows.Scan(&t.testFile, &t.testName); err != nil {
			continue
		}
		tests = append(tests, &t)
	}

	return tests, rows.Err()
}

// getTestsInFile gets all tests in a specific test file
func getTestsInFile(s *store.Store, testFile string) ([]*testInfo, error) {
	rows, err := s.DB().Query(`
		SELECT DISTINCT test_file, test_name
		FROM test_entity_map
		WHERE test_file = ?
	`, testFile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*testInfo
	for rows.Next() {
		var t testInfo
		if err := rows.Scan(&t.testFile, &t.testName); err != nil {
			continue
		}
		tests = append(tests, &t)
	}

	return tests, rows.Err()
}

// findIntegrationTestFiles finds test files that might be integration tests
func findIntegrationTestFiles(s *store.Store) ([]string, error) {
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

// isIntegrationTestRelevant checks if an integration test might be relevant to changed files
func isIntegrationTestRelevant(testFile string, changedFiles []string) bool {
	// Extract package from test file
	testDir := filepath.Dir(testFile)

	// Check if any changed file is in a related package
	for _, changedFile := range changedFiles {
		changedDir := filepath.Dir(changedFile)

		// Simple heuristic: if directories have common prefix, consider relevant
		if strings.HasPrefix(testDir, changedDir) || strings.HasPrefix(changedDir, testDir) {
			return true
		}

		// Also check if they share a common parent directory
		testParts := strings.Split(testDir, string(filepath.Separator))
		changedParts := strings.Split(changedDir, string(filepath.Separator))

		// If they share first 2 path components, consider related
		if len(testParts) >= 2 && len(changedParts) >= 2 {
			if testParts[0] == changedParts[0] && testParts[1] == changedParts[1] {
				return true
			}
		}
	}

	return false
}

// getTotalTestCount gets the total number of tests in the project
func getTotalTestCount(s *store.Store) (int, error) {
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

// generateTestCommand generates a go test command to run affected tests
func generateTestCommand(direct, indirect, integration map[string]*testInfo) string {
	if len(direct) == 0 && len(indirect) == 0 && len(integration) == 0 {
		return "# No tests affected"
	}

	// Collect unique test names
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

	// Convert to sorted list
	names := make([]string, 0, len(testNames))
	for name := range testNames {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build regex pattern for -run flag
	// Pattern: ^(Test1|Test2|Test3)$
	pattern := "^(" + strings.Join(names, "|") + ")$"

	// Build command
	var buf bytes.Buffer
	buf.WriteString("go test -run '")
	buf.WriteString(pattern)
	buf.WriteString("' ./...")

	return buf.String()
}

// convertTestsToList converts test map to sorted list of test identifiers
func convertTestsToList(tests map[string]*testInfo) []string {
	if len(tests) == 0 {
		return nil
	}

	list := make([]string, 0, len(tests))
	for _, t := range tests {
		// Format: TestName @ file:line
		// We don't have line numbers in test_entity_map, so just use file
		identifier := fmt.Sprintf("%s @ %s", t.testName, t.testFile)
		list = append(list, identifier)
	}

	sort.Strings(list)
	return list
}

// estimateTimeSaved estimates time saved by running fewer tests
func estimateTimeSaved(totalTests, affectedTests int) string {
	if totalTests == 0 || affectedTests >= totalTests {
		return "No time saved (all tests need to run)"
	}

	// Rough estimate: 1.5 seconds per test on average
	const avgTestTimeSeconds = 1.5

	totalTime := float64(totalTests) * avgTestTimeSeconds
	affectedTime := float64(affectedTests) * avgTestTimeSeconds
	savedSeconds := totalTime - affectedTime

	// Format output
	if savedSeconds < 60 {
		return fmt.Sprintf("Run %d tests (~%.0fs) instead of %d tests (~%.0fs)",
			affectedTests, affectedTime, totalTests, totalTime)
	}

	totalMinutes := totalTime / 60
	affectedMinutes := affectedTime / 60

	return fmt.Sprintf("Run %d tests (~%.1fmin) instead of %d tests (~%.1fmin)",
		affectedTests, affectedMinutes, totalTests, totalMinutes)
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// guardCmd represents the guard command
var guardCmd = &cobra.Command{
	Use:   "guard",
	Short: "Pre-commit hook to catch problems before commit",
	Long: `Pre-commit guard that checks staged changes for quality issues.

This command is designed to run as a git pre-commit hook. It analyzes
staged files and warns about potential problems before they're committed.

Checks performed:
  1. Coverage regression - Did coverage decrease for modified keystones?
  2. New untested code - Are there new entities with 0% coverage?
  3. Breaking changes - Are there signature changes with unchecked callers?
  4. Graph drift - Is the cx database out of sync with code?

Exit codes:
  0 = pass (no errors, warnings allowed if --fail-on-warnings is false)
  1 = warnings only (pass by default, fail if --fail-on-warnings is true)
  2 = errors (always fails)

Configuration (.cx/config.yaml):
  guard:
    fail_on_coverage_regression: true
    min_coverage_for_keystones: 50
    fail_on_warnings: false

Examples:
  cx guard                    # Check staged changes (default)
  cx guard --staged           # Explicit: check staged changes
  cx guard --all              # Check all modified files (staged + unstaged)
  cx guard --fail-on-warnings # Fail on warnings too

Hook installation:
  echo 'cx guard --staged' >> .git/hooks/pre-commit
  chmod +x .git/hooks/pre-commit`,
	RunE: runGuard,
}

var (
	guardStaged         bool
	guardAll            bool
	guardFailOnWarnings bool
	guardMinCoverage    float64
)

func init() {
	rootCmd.AddCommand(guardCmd)

	guardCmd.Flags().BoolVar(&guardStaged, "staged", true, "Check only staged files (default)")
	guardCmd.Flags().BoolVar(&guardAll, "all", false, "Check all modified files (staged + unstaged)")
	guardCmd.Flags().BoolVar(&guardFailOnWarnings, "fail-on-warnings", false, "Exit with error code on warnings")
	guardCmd.Flags().Float64Var(&guardMinCoverage, "min-coverage", 50.0, "Minimum coverage threshold for keystones (%)")
}

// GuardOutput represents the guard check results
type GuardOutput struct {
	Summary         *GuardSummary `yaml:"summary" json:"summary"`
	Errors          []GuardIssue  `yaml:"errors,omitempty" json:"errors,omitempty"`
	Warnings        []GuardIssue  `yaml:"warnings,omitempty" json:"warnings,omitempty"`
	FilesChecked    []string      `yaml:"files_checked" json:"files_checked"`
	Recommendations []string      `yaml:"recommendations,omitempty" json:"recommendations,omitempty"`
}

// GuardSummary contains aggregate statistics
type GuardSummary struct {
	FilesChecked     int    `yaml:"files_checked" json:"files_checked"`
	EntitiesAffected int    `yaml:"entities_affected" json:"entities_affected"`
	ErrorCount       int    `yaml:"error_count" json:"error_count"`
	WarningCount     int    `yaml:"warning_count" json:"warning_count"`
	DriftDetected    bool   `yaml:"drift_detected" json:"drift_detected"`
	CoverageIssues   int    `yaml:"coverage_issues" json:"coverage_issues"`
	SignatureChanges int    `yaml:"signature_changes" json:"signature_changes"`
	PassStatus       string `yaml:"pass_status" json:"pass_status"` // pass, warnings, fail
}

// GuardIssue represents a single error or warning
type GuardIssue struct {
	Type       string `yaml:"type" json:"type"` // coverage_regression, untested_code, signature_change, drift
	Entity     string `yaml:"entity" json:"entity"`
	File       string `yaml:"file" json:"file"`
	Message    string `yaml:"message" json:"message"`
	Suggestion string `yaml:"suggestion,omitempty" json:"suggestion,omitempty"`
}

// guardEntity holds entity info during guard analysis
type guardEntity struct {
	entity      *store.Entity
	metrics     *store.Metrics
	coverage    *coverage.EntityCoverage
	drifted     bool
	driftType   string
	isNew       bool
	sigChanged  bool
	callerCount int
}

func runGuard(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Apply config defaults if flags weren't explicitly set
	if !cmd.Flags().Changed("min-coverage") {
		guardMinCoverage = cfg.Guard.MinCoverageForKeystones
	}
	if !cmd.Flags().Changed("fail-on-warnings") {
		guardFailOnWarnings = cfg.Guard.FailOnWarnings
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

	// Build graph for analysis
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Get files to check
	var files []string
	if guardAll {
		files, err = getModifiedFiles()
	} else {
		files, err = getStagedFiles()
	}
	if err != nil {
		return fmt.Errorf("failed to get files: %w", err)
	}

	if len(files) == 0 {
		if !quiet {
			fmt.Println("cx guard: No files to check")
		}
		return nil
	}

	// Filter to source files
	sourceFiles := filterSourceFiles(files)
	if len(sourceFiles) == 0 {
		if !quiet {
			fmt.Println("cx guard: No source files to check")
		}
		return nil
	}

	// Analyze files
	baseDir, _ := os.Getwd()
	guardOutput := analyzeFiles(sourceFiles, storeDB, g, cfg, baseDir)

	// Determine exit status
	exitCode := 0
	if guardOutput.Summary.ErrorCount > 0 {
		exitCode = 2
		guardOutput.Summary.PassStatus = "fail"
	} else if guardOutput.Summary.WarningCount > 0 {
		if guardFailOnWarnings {
			exitCode = 1
			guardOutput.Summary.PassStatus = "fail"
		} else {
			guardOutput.Summary.PassStatus = "warnings"
		}
	} else {
		guardOutput.Summary.PassStatus = "pass"
	}

	// Output results
	if quiet && exitCode == 0 {
		return nil
	}

	// Parse format
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

	// For non-YAML formats or when there are issues, output full structure
	if format != output.FormatYAML || guardOutput.Summary.ErrorCount > 0 || guardOutput.Summary.WarningCount > 0 {
		if err := formatter.FormatToWriter(cmd.OutOrStdout(), guardOutput, density); err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}
	} else if !quiet {
		// Simple success message for clean runs
		fmt.Printf("cx guard: %d files checked, no issues found\n", len(sourceFiles))
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// getStagedFiles returns list of staged files from git
func getStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// getModifiedFiles returns all modified files (staged + unstaged)
func getModifiedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=ACMR", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// If HEAD doesn't exist (new repo), try without HEAD
		cmd = exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACMR")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// filterSourceFiles keeps only supported source files
func filterSourceFiles(files []string) []string {
	var result []string
	for _, f := range files {
		ext := filepath.Ext(f)
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".java", ".rs", ".py",
			".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".cs", ".php",
			".kt", ".kts", ".rb", ".rake":
			result = append(result, f)
		}
	}
	return result
}

// analyzeFiles performs guard analysis on the given files
func analyzeFiles(files []string, storeDB *store.Store, g *graph.Graph, cfg *config.Config, baseDir string) *GuardOutput {
	output := &GuardOutput{
		Summary: &GuardSummary{
			FilesChecked: len(files),
		},
		FilesChecked: files,
	}

	// Collect all entities from modified files
	entities := make(map[string]*guardEntity)
	driftCount := 0
	signatureChanges := 0
	coverageIssues := 0

	for _, filePath := range files {
		absPath := filePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(baseDir, filePath)
		}

		// Read current file content
		content, err := os.ReadFile(absPath)
		if err != nil {
			continue // File might be deleted
		}

		// Get entities from store for this file
		storedEntities, err := storeDB.QueryEntities(store.EntityFilter{
			FilePath: filePath,
			Status:   "active",
		})
		if err != nil {
			continue
		}

		// Parse current file
		lang := detectLanguageForGuard(filePath)
		if lang == "" {
			continue
		}

		p, err := parser.NewParser(lang)
		if err != nil {
			continue
		}

		parseResult, err := p.Parse(content)
		p.Close()
		if err != nil {
			continue
		}

		// Extract current entities
		ext := extract.NewExtractor(parseResult)
		currentEntities, err := ext.ExtractAll()
		if err != nil {
			continue
		}

		// Build lookup by name for current entities
		currentLookup := make(map[string]*extract.Entity)
		for i := range currentEntities {
			ce := &currentEntities[i]
			currentLookup[ce.Name] = ce
		}

		// Check stored entities against current
		for _, storedEnt := range storedEntities {
			if storedEnt.EntityType == "import" {
				continue
			}

			ge := &guardEntity{
				entity: storedEnt,
			}

			// Get metrics
			ge.metrics, _ = storeDB.GetMetrics(storedEnt.ID)

			// Get coverage
			cov, _ := coverage.GetEntityCoverage(storeDB, storedEnt.ID)
			ge.coverage = cov

			// Check for drift
			currentEnt, found := currentLookup[storedEnt.Name]
			if !found {
				ge.drifted = true
				ge.driftType = "missing"
				driftCount++
			} else {
				// Check signature change
				if storedEnt.SigHash != "" && currentEnt.SigHash != "" {
					if storedEnt.SigHash != currentEnt.SigHash {
						ge.sigChanged = true
						ge.driftType = "signature"
						signatureChanges++

						// Count callers
						ge.callerCount = len(g.Predecessors(storedEnt.ID))
					}
				}

				// Check body change (drift)
				if storedEnt.BodyHash != "" && currentEnt.BodyHash != "" {
					if storedEnt.BodyHash != currentEnt.BodyHash && !ge.sigChanged {
						ge.drifted = true
						ge.driftType = "body"
						driftCount++
					}
				}
			}

			entities[storedEnt.ID] = ge
		}

		// Check for new entities (in current but not in store)
		for _, currentEnt := range currentEntities {
			if currentEnt.Kind == extract.ImportEntity {
				continue
			}

			entityID := currentEnt.GenerateEntityID()
			if _, exists := entities[entityID]; exists {
				continue
			}

			// Check if this is a new entity
			_, err := storeDB.GetEntity(entityID)
			if err != nil {
				// New entity
				ge := &guardEntity{
					entity: &store.Entity{
						ID:         entityID,
						Name:       currentEnt.Name,
						EntityType: string(currentEnt.Kind),
						FilePath:   filePath,
						LineStart:  int(currentEnt.StartLine),
					},
					isNew: true,
				}
				entities[entityID] = ge
			}
		}
	}

	output.Summary.EntitiesAffected = len(entities)
	output.Summary.SignatureChanges = signatureChanges

	// Generate errors and warnings
	for _, ge := range entities {
		// Compute keystone threshold dynamically
		isKeystone := ge.metrics != nil && isKeystoneEntity(ge.metrics, entities)

		// ERROR: Coverage regression on keystone
		if isKeystone && ge.coverage != nil && ge.coverage.CoveragePercent < guardMinCoverage {
			output.Errors = append(output.Errors, GuardIssue{
				Type:       "coverage_regression",
				Entity:     ge.entity.Name,
				File:       ge.entity.FilePath,
				Message:    fmt.Sprintf("Keystone %s coverage is %.1f%% (below %.0f%% threshold)", ge.entity.Name, ge.coverage.CoveragePercent, guardMinCoverage),
				Suggestion: "Add tests before committing",
			})
			coverageIssues++
		}

		// WARNING: New untested entity
		if ge.isNew {
			output.Warnings = append(output.Warnings, GuardIssue{
				Type:       "untested_code",
				Entity:     ge.entity.Name,
				File:       ge.entity.FilePath,
				Message:    fmt.Sprintf("New entity %s has no test coverage", ge.entity.Name),
				Suggestion: "Consider adding tests",
			})
		}

		// WARNING: Signature change with callers
		if ge.sigChanged && ge.callerCount > 0 {
			output.Warnings = append(output.Warnings, GuardIssue{
				Type:       "signature_change",
				Entity:     ge.entity.Name,
				File:       ge.entity.FilePath,
				Message:    fmt.Sprintf("%s signature changed, %d callers may be affected", ge.entity.Name, ge.callerCount),
				Suggestion: "Verify all callers handle the new signature",
			})
		}

		// WARNING: Graph drift
		if ge.drifted && ge.driftType == "body" {
			output.Warnings = append(output.Warnings, GuardIssue{
				Type:       "drift",
				Entity:     ge.entity.Name,
				File:       ge.entity.FilePath,
				Message:    fmt.Sprintf("%s has drifted since last scan", ge.entity.Name),
				Suggestion: "Run 'cx scan' to update the graph",
			})
		}
	}

	output.Summary.DriftDetected = driftCount > 0
	output.Summary.CoverageIssues = coverageIssues
	output.Summary.ErrorCount = len(output.Errors)
	output.Summary.WarningCount = len(output.Warnings)

	// Sort issues by file then entity
	sort.Slice(output.Errors, func(i, j int) bool {
		if output.Errors[i].File != output.Errors[j].File {
			return output.Errors[i].File < output.Errors[j].File
		}
		return output.Errors[i].Entity < output.Errors[j].Entity
	})
	sort.Slice(output.Warnings, func(i, j int) bool {
		if output.Warnings[i].File != output.Warnings[j].File {
			return output.Warnings[i].File < output.Warnings[j].File
		}
		return output.Warnings[i].Entity < output.Warnings[j].Entity
	})

	// Build recommendations
	if output.Summary.ErrorCount > 0 {
		output.Recommendations = append(output.Recommendations, "Fix errors before committing")
	}
	if driftCount > 0 {
		output.Recommendations = append(output.Recommendations, "Run 'cx scan' to update the code graph")
	}
	if coverageIssues > 0 {
		output.Recommendations = append(output.Recommendations, "Add tests for undertested keystones")
	}
	if signatureChanges > 0 {
		output.Recommendations = append(output.Recommendations, "Review callers of changed signatures")
	}

	return output
}

// detectLanguageForGuard detects the parser language from a file path
func detectLanguageForGuard(path string) parser.Language {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return parser.Go
	case ".ts", ".tsx":
		return parser.TypeScript
	case ".js", ".jsx", ".mjs", ".cjs":
		return parser.JavaScript
	case ".java":
		return parser.Java
	case ".rs":
		return parser.Rust
	case ".py":
		return parser.Python
	case ".c", ".h":
		return parser.C
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return parser.Cpp
	case ".cs":
		return parser.CSharp
	case ".php":
		return parser.PHP
	case ".kt", ".kts":
		return parser.Kotlin
	case ".rb", ".rake":
		return parser.Ruby
	default:
		return "" // Unknown
	}
}

// isKeystoneEntity determines if an entity is a keystone based on dynamic threshold
func isKeystoneEntity(metrics *store.Metrics, allEntities map[string]*guardEntity) bool {
	if metrics == nil || metrics.PageRank == 0 {
		return false
	}

	// Collect all PageRank values
	var pageranks []float64
	for _, ge := range allEntities {
		if ge.metrics != nil && ge.metrics.PageRank > 0 {
			pageranks = append(pageranks, ge.metrics.PageRank)
		}
	}

	if len(pageranks) == 0 {
		return false
	}

	// Sort descending
	sort.Slice(pageranks, func(i, j int) bool {
		return pageranks[i] > pageranks[j]
	})

	// Top 5% or minimum of 10
	topN := len(pageranks) / 20
	if topN < 10 {
		topN = 10
	}
	if topN > len(pageranks) {
		topN = len(pageranks)
	}

	threshold := pageranks[topN-1]
	return metrics.PageRank >= threshold
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/integration"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/semdiff"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check [file-or-entity]",
	Short: "Pre-flight safety check before modifying code",
	Long: `Comprehensive safety assessment before modifying a file or entity.

This is the unified safety command that consolidates impact, coverage gaps,
drift verification, and change detection into a single interface.

Modes:
  cx check <file>           Full safety assessment (default: impact + coverage + drift)
  cx check <file> --quick   Just blast radius (impact analysis only)
  cx check --coverage       Just coverage gaps (no target required)
  cx check --drift          Just staleness check (no target required)
  cx check --changes        What changed since last scan (no target required)

The full check performs four analyses:
  1. Impact Analysis - What entities are affected by changes here?
  2. Coverage Gaps - Are affected keystones adequately tested?
  3. Drift Detection - Has the code changed since last scan?
  4. Change Detection - What has changed since last scan?

Output Structure (full check):
  safety_assessment:
    target:              File or entity being checked
    risk_level:          Overall risk (low, medium, high, critical)
    impact_radius:       Number of entities affected
    keystone_count:      Number of keystone entities affected
    coverage_gaps:       Undertested keystones in the blast radius
    drift_detected:      Whether code has drifted since scan

  warnings:              List of actionable warnings
  recommendations:       Suggested actions before proceeding
  affected_keystones:    Details of keystone entities at risk

Risk Levels:
  critical:  Multiple undertested keystones affected, or drift detected
  high:      Keystones affected with coverage gaps
  medium:    Multiple entities affected, adequate coverage
  low:       Isolated changes with good test coverage

Examples:
  cx check src/auth/jwt.go              # Full safety assessment
  cx check src/auth/jwt.go --quick      # Just blast radius (was: cx impact)
  cx check --coverage                   # Coverage gaps report (was: cx gaps)
  cx check --coverage --keystones-only  # Only keystone coverage gaps
  cx check --drift                      # Staleness check (was: cx verify)
  cx check --drift --strict             # Exit non-zero on drift (for CI)
  cx check --changes                    # What changed since scan (was: cx diff)
  cx check --depth 5 src/core/          # Deeper transitive analysis
  cx check --format json src/api.go     # JSON output for tooling
  cx check --create-task src/auth/      # Create beads task for findings`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCheck,
}

var (
	checkDepth      int
	checkCreateTask bool
	// Mode flags - when set, run only that specific analysis
	checkQuick    bool // Just impact analysis (blast radius)
	checkCoverage bool // Just coverage gaps
	checkDrift    bool // Just drift/verify check
	checkChanges  bool // Just diff (what changed since scan)
	// Coverage-specific flags (from gaps command)
	checkKeystonesOnly bool
	checkThreshold     int
	// Drift-specific flags (from verify command)
	checkStrict bool
	checkFix    bool
	checkDryRun bool
	// Diff-specific flags (from diff command)
	checkFile     string
	checkDetailed bool
	checkSemantic bool
	// Impact-specific flags
	checkImpactThreshold float64
)

func init() {
	rootCmd.AddCommand(checkCmd)

	// General flags
	checkCmd.Flags().IntVar(&checkDepth, "depth", 3, "Transitive impact depth")
	checkCmd.Flags().BoolVar(&checkCreateTask, "create-task", false, "Create a beads task for safety findings")

	// Mode flags
	checkCmd.Flags().BoolVar(&checkQuick, "quick", false, "Quick mode: just blast radius/impact analysis")
	checkCmd.Flags().BoolVar(&checkCoverage, "coverage", false, "Coverage mode: show coverage gaps (was: cx gaps)")
	checkCmd.Flags().BoolVar(&checkDrift, "drift", false, "Drift mode: check staleness (was: cx verify)")
	checkCmd.Flags().BoolVar(&checkChanges, "changes", false, "Changes mode: what changed since scan (was: cx diff)")

	// Coverage-specific flags
	checkCmd.Flags().BoolVar(&checkKeystonesOnly, "keystones-only", false, "Only show keystones with gaps (--coverage mode)")
	checkCmd.Flags().IntVar(&checkThreshold, "threshold", 75, "Coverage threshold percentage (--coverage mode)")

	// Drift-specific flags
	checkCmd.Flags().BoolVar(&checkStrict, "strict", false, "Exit non-zero on any drift (--drift mode, for CI)")
	checkCmd.Flags().BoolVar(&checkFix, "fix", false, "Update hashes for drifted entities (--drift mode)")
	checkCmd.Flags().BoolVar(&checkDryRun, "dry-run", false, "Show what --fix would do without making changes")

	// Diff-specific flags
	checkCmd.Flags().StringVar(&checkFile, "file", "", "Show changes for specific file/directory only (--changes mode)")
	checkCmd.Flags().BoolVar(&checkDetailed, "detailed", false, "Show hash changes for modified entities (--changes mode)")
	checkCmd.Flags().BoolVar(&checkSemantic, "semantic", false, "Show semantic analysis (--changes mode)")

	// Impact-specific flags
	checkCmd.Flags().Float64Var(&checkImpactThreshold, "impact-threshold", 0, "Min importance threshold for impact analysis")
}

// CheckOutput represents the safety check results
type CheckOutput struct {
	SafetyAssessment  *SafetyAssessment `yaml:"safety_assessment" json:"safety_assessment"`
	Warnings          []string          `yaml:"warnings,omitempty" json:"warnings,omitempty"`
	Recommendations   []string          `yaml:"recommendations" json:"recommendations"`
	AffectedKeystones []KeystoneInfo    `yaml:"affected_keystones,omitempty" json:"affected_keystones,omitempty"`
}

// SafetyAssessment contains the aggregate safety metrics
type SafetyAssessment struct {
	Target        string `yaml:"target" json:"target"`
	RiskLevel     string `yaml:"risk_level" json:"risk_level"`
	ImpactRadius  int    `yaml:"impact_radius" json:"impact_radius"`
	FilesAffected int    `yaml:"files_affected" json:"files_affected"`
	KeystoneCount int    `yaml:"keystone_count" json:"keystone_count"`
	CoverageGaps  int    `yaml:"coverage_gaps" json:"coverage_gaps"`
	DriftDetected bool   `yaml:"drift_detected" json:"drift_detected"`
	DriftedCount  int    `yaml:"drifted_count,omitempty" json:"drifted_count,omitempty"`
}

// KeystoneInfo contains details about an affected keystone
type KeystoneInfo struct {
	Name        string  `yaml:"name" json:"name"`
	Type        string  `yaml:"type" json:"type"`
	Location    string  `yaml:"location" json:"location"`
	PageRank    float64 `yaml:"pagerank" json:"pagerank"`
	Coverage    string  `yaml:"coverage" json:"coverage"`
	Impact      string  `yaml:"impact" json:"impact"`
	CoverageGap bool    `yaml:"coverage_gap" json:"coverage_gap"`
}

// checkEntity holds entity info for check analysis
type checkEntity struct {
	entity    *store.Entity
	metrics   *store.Metrics
	coverage  *coverage.EntityCoverage
	depth     int
	direct    bool
	drifted   bool
	driftType string
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Count how many mode flags are set
	modeCount := 0
	if checkCoverage {
		modeCount++
	}
	if checkDrift {
		modeCount++
	}
	if checkChanges {
		modeCount++
	}

	// Validate: only one mode flag at a time (quick is different - it's a modifier)
	if modeCount > 1 {
		return fmt.Errorf("only one of --coverage, --drift, or --changes can be specified")
	}

	// Route to the appropriate mode handler
	if checkCoverage {
		return runCheckCoverage(cmd, args)
	}
	if checkDrift {
		return runCheckDrift(cmd, args)
	}
	if checkChanges {
		return runCheckChanges(cmd, args)
	}

	// Default mode: full check or quick (impact-only) - requires a target
	if len(args) == 0 {
		return fmt.Errorf("target file or entity required (or use --coverage, --drift, or --changes)")
	}

	target := args[0]

	if checkQuick {
		return runCheckQuick(cmd, target)
	}

	return runCheckFull(cmd, target)
}

// runCheckFull performs the full safety assessment (impact + coverage + drift)
func runCheckFull(cmd *cobra.Command, target string) error {
	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultConfig()
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

	// Build graph for traversal
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// === PHASE 1: Impact Analysis ===
	directEntities, err := findDirectEntities(target, storeDB)
	if err != nil {
		return err
	}

	if len(directEntities) == 0 {
		return fmt.Errorf("no entities found matching: %s", target)
	}

	// Find all affected entities via BFS
	affected := findAffectedEntities(directEntities, g, storeDB, cfg, checkDepth)

	// === PHASE 2: Coverage Gap Detection ===
	enrichWithCoverage(affected, storeDB)

	// === PHASE 3: Drift Detection ===
	baseDir, _ := os.Getwd()
	driftCount := detectDrift(affected, storeDB, baseDir)

	// === Build Output ===
	checkOutput := buildCheckOutput(target, affected, cfg, driftCount)

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

	if err := formatter.FormatToWriter(cmd.OutOrStdout(), checkOutput, density); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Create beads task if requested
	if checkCreateTask && len(checkOutput.Warnings) > 0 {
		if err := createCheckTask(target, checkOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create task: %v\n", err)
		}
	}

	return nil
}

// runCheckQuick performs just impact analysis (blast radius) - equivalent to old cx impact
func runCheckQuick(cmd *cobra.Command, target string) error {
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

	// Build graph for traversal
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Find direct entities
	var directEntries []*impactEntry

	if isFilePath(target) {
		entities, err := storeDB.QueryEntities(store.EntityFilter{
			FilePath: target,
			Status:   "active",
		})
		if err == nil {
			for _, e := range entities {
				m, _ := storeDB.GetMetrics(e.ID)
				var pr float64
				var deps int
				if m != nil {
					pr = m.PageRank
					deps = m.InDegree
				}
				directEntries = append(directEntries, &impactEntry{
					entityID:   e.ID,
					name:       e.Name,
					location:   formatStoreLocation(e),
					entityType: mapStoreTypeToCGF(e.EntityType),
					pagerank:   pr,
					deps:       deps,
					importance: computeImportanceLevel(pr),
				})
			}
		}
	} else {
		entity, err := storeDB.GetEntity(target)
		if err == nil && entity != nil {
			m, _ := storeDB.GetMetrics(entity.ID)
			var pr float64
			var deps int
			if m != nil {
				pr = m.PageRank
				deps = m.InDegree
			}
			directEntries = append(directEntries, &impactEntry{
				entityID:   entity.ID,
				name:       entity.Name,
				location:   formatStoreLocation(entity),
				entityType: mapStoreTypeToCGF(entity.EntityType),
				pagerank:   pr,
				deps:       deps,
				importance: computeImportanceLevel(pr),
			})
		} else {
			entities, err := storeDB.QueryEntities(store.EntityFilter{
				Name:   target,
				Status: "active",
				Limit:  10,
			})
			if err == nil {
				for _, e := range entities {
					m, _ := storeDB.GetMetrics(e.ID)
					var pr float64
					var deps int
					if m != nil {
						pr = m.PageRank
						deps = m.InDegree
					}
					directEntries = append(directEntries, &impactEntry{
						entityID:   e.ID,
						name:       e.Name,
						location:   formatStoreLocation(e),
						entityType: mapStoreTypeToCGF(e.EntityType),
						pagerank:   pr,
						deps:       deps,
						importance: computeImportanceLevel(pr),
					})
				}
			}
		}
	}

	if len(directEntries) == 0 {
		return fmt.Errorf("no entities found matching: %s", target)
	}

	// Find affected entities using graph traversal
	affected := make(map[string]*impactEntry)
	recommendations := []string{}

	// Add direct entities
	for _, entry := range directEntries {
		entry.direct = true
		entry.depth = 0
		affected[entry.entityID] = entry

		if entry.pagerank >= 0.30 {
			recommendations = append(recommendations, fmt.Sprintf("Review %s - keystone entity (pr=%.2f, %d direct dependents)",
				entry.name, entry.pagerank, entry.deps))
		}
	}

	// BFS to find transitively affected
	for _, direct := range directEntries {
		depth := 1
		visited := make(map[string]int)
		visited[direct.entityID] = 0

		queue := []string{direct.entityID}
		for len(queue) > 0 && depth <= checkDepth {
			levelSize := len(queue)
			for i := 0; i < levelSize; i++ {
				current := queue[0]
				queue = queue[1:]

				preds := g.Predecessors(current)
				for _, pred := range preds {
					if _, seen := visited[pred]; seen {
						continue
					}
					visited[pred] = depth

					if depth <= checkDepth {
						queue = append(queue, pred)
					}

					if _, inAffected := affected[pred]; inAffected {
						continue
					}

					callerEntity, err := storeDB.GetEntity(pred)
					if err != nil {
						continue
					}

					m, _ := storeDB.GetMetrics(pred)
					var pr float64
					var deps int
					if m != nil {
						pr = m.PageRank
						deps = m.InDegree
					}

					if pr >= checkImpactThreshold {
						entry := &impactEntry{
							entityID:   pred,
							name:       callerEntity.Name,
							location:   formatStoreLocation(callerEntity),
							entityType: mapStoreTypeToCGF(callerEntity.EntityType),
							depth:      depth,
							direct:     false,
							pagerank:   pr,
							deps:       deps,
							importance: computeImportanceLevel(pr),
						}
						affected[pred] = entry

						if pr >= 0.30 {
							recommendations = append(recommendations, fmt.Sprintf("Review %s - keystone caller (pr=%.2f, %d dependents)",
								callerEntity.Name, pr, deps))
						}
					}
				}
			}
			depth++
		}
	}

	// Limit results
	maxResults := 100
	if len(affected) > maxResults {
		sortedEntries := make([]*impactEntry, 0, len(affected))
		for _, e := range affected {
			sortedEntries = append(sortedEntries, e)
		}
		sort.Slice(sortedEntries, func(i, j int) bool {
			if sortedEntries[i].direct != sortedEntries[j].direct {
				return sortedEntries[i].direct
			}
			return sortedEntries[i].pagerank > sortedEntries[j].pagerank
		})

		affected = make(map[string]*impactEntry)
		for i := 0; i < maxResults && i < len(sortedEntries); i++ {
			affected[sortedEntries[i].entityID] = sortedEntries[i]
		}
	}

	// Build output
	impactOutput := buildImpactOutput(target, affected)

	// Add recommendations
	if len(recommendations) > 0 {
		seen := make(map[string]bool)
		for _, rec := range recommendations {
			if !seen[rec] {
				impactOutput.Recommendations = append(impactOutput.Recommendations, rec)
				seen[rec] = true
			}
		}
	}

	// Format output
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), impactOutput, density)
}

// runCheckCoverage shows coverage gaps - equivalent to old cx gaps
func runCheckCoverage(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("no coverage data found - run 'cx coverage import' first")
	}

	// Build list of gaps
	var gaps []coverageGap
	keystoneCount := 0

	for _, e := range entities {
		m, err := storeDB.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue
		}

		if m.PageRank >= cfg.Metrics.KeystoneThreshold {
			keystoneCount++
		}

		cov, err := coverage.GetEntityCoverage(storeDB, e.ID)
		if err != nil {
			cov = &coverage.EntityCoverage{
				EntityID:        e.ID,
				CoveragePercent: 0,
				CoveredLines:    []int{},
				UncoveredLines:  []int{},
			}
		}

		if cov.CoveragePercent >= float64(checkThreshold) {
			continue
		}

		if checkKeystonesOnly && m.PageRank < cfg.Metrics.KeystoneThreshold {
			continue
		}

		riskScore := (1 - cov.CoveragePercent/100.0) * m.PageRank * float64(m.InDegree)
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

	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].riskScore > gaps[j].riskScore
	})

	if checkCreateTask {
		return printTaskCommands(gaps, cfg)
	}

	// Format output
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	gapsByRisk := groupGapsByRisk(gaps)
	outputData := buildGapsOutput(gapsByRisk, keystoneCount)

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), outputData, output.DensityMedium)
}

// runCheckDrift performs staleness verification - equivalent to old cx verify
func runCheckDrift(cmd *cobra.Command, args []string) error {
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

	// Get entities
	filter := store.EntityFilter{Status: "active"}
	entities, err := storeDB.QueryEntities(filter)
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		verifyOut := &VerifyOutput{
			Verification: &VerificationData{
				Status:          "passed",
				EntitiesChecked: 0,
				Valid:           0,
				Drifted:         0,
				Missing:         0,
				Issues:          []VerifyIssue{},
			},
		}

		if !quiet {
			formatter, err := output.GetFormatter(output.FormatYAML)
			if err != nil {
				return fmt.Errorf("failed to get formatter: %w", err)
			}
			return formatter.FormatToWriter(cmd.OutOrStdout(), verifyOut, output.DensityMedium)
		}
		return nil
	}

	result := &verifyResult{}
	byFile := groupByFileStore(entities)
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}

	for filePath, fileEntities := range byFile {
		absPath := filePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(baseDir, filePath)
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			for _, entity := range fileEntities {
				e := entity
				result.missing = append(result.missing, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "file deleted",
					detail: fmt.Sprintf("File not found: %s", filePath),
				})
			}
			continue
		}

		p, err := parser.NewParser(parser.Go)
		if err != nil {
			for _, entity := range fileEntities {
				e := entity
				result.drifted = append(result.drifted, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "parse error",
					detail: fmt.Sprintf("Cannot create parser: %v", err),
				})
			}
			continue
		}

		parseResult, err := p.Parse(content)
		p.Close()

		if err != nil {
			for _, entity := range fileEntities {
				e := entity
				result.drifted = append(result.drifted, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "parse error",
					detail: fmt.Sprintf("Parse failed: %v", err),
				})
			}
			continue
		}

		extractor := extract.NewExtractor(parseResult)
		currentEntities, err := extractor.ExtractAll()
		if err != nil {
			for _, entity := range fileEntities {
				e := entity
				result.drifted = append(result.drifted, verifyEntry{
					entity: e,
					file:   filePath,
					line:   fmt.Sprintf("%d", e.LineStart),
					reason: "extract error",
					detail: fmt.Sprintf("Extraction failed: %v", err),
				})
			}
			continue
		}

		currentMap := buildEntityLookup(currentEntities)

		for _, stored := range fileEntities {
			s := stored
			if s.EntityType == "import" {
				continue
			}

			current := findMatchingEntityByName(stored.Name, currentMap)
			if current == nil {
				result.missing = append(result.missing, verifyEntry{
					entity: s,
					file:   filePath,
					line:   fmt.Sprintf("%d", s.LineStart),
					reason: "symbol not found",
					detail: "Symbol no longer exists in file",
				})
				continue
			}

			entry := verifyEntry{
				entity:  s,
				file:    filePath,
				line:    fmt.Sprintf("%d", s.LineStart),
				oldSig:  s.SigHash,
				newSig:  current.SigHash,
				oldBody: s.BodyHash,
				newBody: current.BodyHash,
			}

			if s.SigHash == "" && s.BodyHash == "" {
				entry.reason = "no stored hashes"
				entry.detail = "Entity has no stored hashes"
				result.drifted = append(result.drifted, entry)
			} else if s.SigHash != current.SigHash {
				entry.reason = "sig_hash changed"
				entry.detail = "Parameters or return types changed"
				result.drifted = append(result.drifted, entry)
			} else if s.BodyHash != current.BodyHash {
				entry.reason = "body_hash changed"
				entry.detail = "Logic changed"
				result.drifted = append(result.drifted, entry)
			} else {
				result.valid = append(result.valid, entry)
			}
		}

		parseResult.Close()
	}

	// Fix drifted if requested
	if (checkFix || checkDryRun) && len(result.drifted) > 0 {
		if checkDryRun && !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "\n[dry-run] Would fix %d drifted entities:\n", len(result.drifted))
		}
		for _, entry := range result.drifted {
			if entry.newSig != "" && entry.newBody != "" {
				if checkDryRun && !quiet {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run]   - %s (%s)\n",
						entry.entity.Name, entry.entity.ID)
				} else if !checkDryRun {
					entry.entity.SigHash = entry.newSig
					entry.entity.BodyHash = entry.newBody
					if err := storeDB.UpdateEntity(entry.entity); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to update %s: %v\n",
							entry.entity.ID, err)
					}
				}
			}
		}
		if checkDryRun && !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] No changes made\n\n")
		}
	}

	// Build issues list
	issues := []VerifyIssue{}
	for _, entry := range result.drifted {
		issue := VerifyIssue{
			Entity:   entry.entity.Name,
			Type:     "drifted",
			Location: formatStoreLocation(entry.entity),
			Reason:   entry.reason,
			Detail:   entry.detail,
		}
		if entry.reason == "sig_hash changed" && entry.oldSig != "" && entry.newSig != "" {
			issue.HashType = "signature"
			issue.Expected = entry.oldSig
			issue.Actual = entry.newSig
		} else if entry.reason == "body_hash changed" && entry.oldBody != "" && entry.newBody != "" {
			issue.HashType = "body"
			issue.Expected = entry.oldBody
			issue.Actual = entry.newBody
		}
		issues = append(issues, issue)
	}

	for _, entry := range result.missing {
		issue := VerifyIssue{
			Entity:   entry.entity.Name,
			Type:     "missing",
			Location: formatStoreLocation(entry.entity),
			Reason:   entry.reason,
			Detail:   entry.detail,
		}
		issues = append(issues, issue)
	}

	status := "passed"
	if len(result.drifted) > 0 || len(result.missing) > 0 {
		status = "failed"
	}

	actions := []string{}
	if len(result.drifted) > 0 || len(result.missing) > 0 {
		if checkFix {
			actions = append(actions, "Fixed drifted entities")
		} else {
			actions = append(actions, "Run `cx check --drift --fix` to update hashes for drifted entities")
		}
		actions = append(actions, "Run `cx scan --force` to re-scan the codebase")
	}

	verifyOut := &VerifyOutput{
		Verification: &VerificationData{
			Status:          status,
			EntitiesChecked: len(result.valid) + len(result.drifted) + len(result.missing),
			Valid:           len(result.valid),
			Drifted:         len(result.drifted),
			Missing:         len(result.missing),
			Issues:          issues,
			Actions:         actions,
		},
	}

	if !quiet {
		formatter, err := output.GetFormatter(output.FormatYAML)
		if err != nil {
			return fmt.Errorf("failed to get formatter: %w", err)
		}
		if err := formatter.FormatToWriter(cmd.OutOrStdout(), verifyOut, output.DensityMedium); err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}
	}

	// Create beads task if requested
	if checkCreateTask && (len(result.drifted) > 0 || len(result.missing) > 0) {
		if !integration.BeadsAvailable() {
			return fmt.Errorf("--create-task requires beads integration (bd CLI and .beads/ directory)")
		}

		title := fmt.Sprintf("Verify: %d drifted, %d missing entities need attention",
			len(result.drifted), len(result.missing))

		var desc strings.Builder
		desc.WriteString("## Verification Results\n\n")
		desc.WriteString(fmt.Sprintf("- **Drifted**: %d entities\n", len(result.drifted)))
		desc.WriteString(fmt.Sprintf("- **Missing**: %d entities\n", len(result.missing)))
		desc.WriteString(fmt.Sprintf("- **Valid**: %d entities\n\n", len(result.valid)))

		if len(result.drifted) > 0 {
			desc.WriteString("### Drifted Entities\n")
			for i, e := range result.drifted {
				if i >= 10 {
					desc.WriteString(fmt.Sprintf("... and %d more\n", len(result.drifted)-10))
					break
				}
				desc.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", e.entity.Name, e.file, e.reason))
			}
			desc.WriteString("\n")
		}

		if len(result.missing) > 0 {
			desc.WriteString("### Missing Entities\n")
			for i, e := range result.missing {
				if i >= 10 {
					desc.WriteString(fmt.Sprintf("... and %d more\n", len(result.missing)-10))
					break
				}
				desc.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", e.entity.Name, e.file, e.reason))
			}
			desc.WriteString("\n")
		}

		desc.WriteString("### Suggested Actions\n")
		desc.WriteString("- Run `cx scan --force` to update the database\n")
		desc.WriteString("- Run `cx check --drift --fix` to update hashes for drifted entities\n")

		priority := 2
		if len(result.missing) > 10 || len(result.drifted) > 20 {
			priority = 1
		}

		opts := integration.CreateBeadOptions{
			Title:       title,
			Description: desc.String(),
			Type:        "task",
			Priority:    priority,
			Labels:      []string{"cx:verify", "cx:stale"},
		}

		beadID, err := integration.CreateBead(opts)
		if err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}

		if !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "\n# Created task: %s\n", beadID)
		}
	}

	if checkStrict && (len(result.drifted) > 0 || len(result.missing) > 0) {
		return fmt.Errorf("verification failed: %d drifted, %d missing",
			len(result.drifted), len(result.missing))
	}

	return nil
}

// runCheckChanges shows what changed since last scan - equivalent to old cx diff
func runCheckChanges(cmd *cobra.Command, args []string) error {
	// Find .cx directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx init && cx scan' first")
	}
	projectRoot := filepath.Dir(cxDir)

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Use semantic diff if requested
	if checkSemantic {
		cfg, err := config.Load(projectRoot)
		if err != nil {
			cfg = config.DefaultConfig()
		}
		analyzer := semdiff.NewAnalyzer(storeDB, projectRoot, cfg)
		result, err := analyzer.Analyze(checkFile)
		if err != nil {
			return err
		}

		format, err := output.ParseFormat(outputFormat)
		if err != nil {
			return err
		}
		density, err := output.ParseDensity(outputDensity)
		if err != nil {
			return err
		}
		formatter, err := output.GetFormatter(format)
		if err != nil {
			return err
		}
		return formatter.FormatToWriter(cmd.OutOrStdout(), result, density)
	}

	// Get all file entries from last scan
	fileEntries, err := storeDB.GetAllFileEntries()
	if err != nil {
		return fmt.Errorf("failed to get file entries: %w", err)
	}

	if len(fileEntries) == 0 {
		return fmt.Errorf("no scan data found: run 'cx scan' first")
	}

	// Find the most recent scan time
	var lastScanTime time.Time
	for _, entry := range fileEntries {
		if entry.ScannedAt.After(lastScanTime) {
			lastScanTime = entry.ScannedAt
		}
	}

	// Build map of stored file hashes
	storedHashes := make(map[string]string)
	for _, entry := range fileEntries {
		if checkFile != "" {
			if entry.FilePath != checkFile && !hasPrefix(entry.FilePath, checkFile) {
				continue
			}
		}
		storedHashes[entry.FilePath] = entry.ScanHash
	}

	// Track changes
	var added []DiffEntity
	var modified []DiffEntity
	var removed []DiffEntity
	changedFiles := make(map[string]bool)

	// Check stored files for modifications and deletions
	for filePath, storedHash := range storedHashes {
		absPath := filepath.Join(projectRoot, filePath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			entities, _ := storeDB.QueryEntities(store.EntityFilter{FilePath: filePath, Status: "active"})
			for _, e := range entities {
				removed = append(removed, DiffEntity{
					Name:     e.Name,
					Type:     e.EntityType,
					Location: formatStoreLocation(e),
					Change:   "file_deleted",
				})
			}
			changedFiles[filePath] = true
			continue
		}

		currentHash := extract.ComputeFileHash(content)
		if currentHash != storedHash {
			changedFiles[filePath] = true
			storedEntities, _ := storeDB.QueryEntities(store.EntityFilter{FilePath: filePath, Status: "active"})
			for _, e := range storedEntities {
				diffEntity := DiffEntity{
					Name:     e.Name,
					Type:     e.EntityType,
					Location: formatStoreLocation(e),
					Change:   "file_modified",
				}
				if checkDetailed {
					diffEntity.OldHash = e.SigHash
				}
				modified = append(modified, diffEntity)
			}
		}
	}

	// Check for new files
	err = filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == ".cx" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".js" && ext != ".py" && ext != ".rs" && ext != ".java" {
			return nil
		}

		relPath, _ := filepath.Rel(projectRoot, path)
		if checkFile != "" {
			if relPath != checkFile && !hasPrefix(relPath, checkFile) {
				return nil
			}
		}

		if _, exists := storedHashes[relPath]; !exists {
			changedFiles[relPath] = true
			added = append(added, DiffEntity{
				Name:     relPath,
				Type:     "file",
				Location: relPath,
				Change:   "new_file",
			})
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	// Build output
	diffOutput := &DiffOutput{
		Summary: DiffSummary{
			FilesChanged:     len(changedFiles),
			EntitiesAdded:    len(added),
			EntitiesModified: len(modified),
			EntitiesRemoved:  len(removed),
			LastScan:         lastScanTime.Format(time.RFC3339),
		},
		Added:    added,
		Modified: modified,
		Removed:  removed,
	}

	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return err
	}
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return err
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), diffOutput, density)
}

// findDirectEntities finds entities that match the target (file or entity name)
func findDirectEntities(target string, storeDB *store.Store) ([]*checkEntity, error) {
	var results []*checkEntity

	if isFilePath(target) {
		entities, err := storeDB.QueryEntities(store.EntityFilter{
			FilePath: target,
			Status:   "active",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query entities: %w", err)
		}

		for _, e := range entities {
			m, _ := storeDB.GetMetrics(e.ID)
			results = append(results, &checkEntity{
				entity:  e,
				metrics: m,
				direct:  true,
				depth:   0,
			})
		}
	} else {
		// Try direct entity lookup first
		entity, err := storeDB.GetEntity(target)
		if err == nil && entity != nil {
			m, _ := storeDB.GetMetrics(entity.ID)
			results = append(results, &checkEntity{
				entity:  entity,
				metrics: m,
				direct:  true,
				depth:   0,
			})
		} else {
			// Try name search
			entities, err := storeDB.QueryEntities(store.EntityFilter{
				Name:   target,
				Status: "active",
				Limit:  10,
			})
			if err == nil {
				for _, e := range entities {
					m, _ := storeDB.GetMetrics(e.ID)
					results = append(results, &checkEntity{
						entity:  e,
						metrics: m,
						direct:  true,
						depth:   0,
					})
				}
			}
		}
	}

	return results, nil
}

// findAffectedEntities performs BFS to find all transitively affected entities
func findAffectedEntities(direct []*checkEntity, g *graph.Graph, storeDB *store.Store, cfg *config.Config, maxDepth int) map[string]*checkEntity {
	affected := make(map[string]*checkEntity)

	// Add direct entities
	for _, e := range direct {
		affected[e.entity.ID] = e
	}

	// BFS from each direct entity
	for _, directEnt := range direct {
		visited := make(map[string]int)
		visited[directEnt.entity.ID] = 0

		queue := []string{directEnt.entity.ID}
		depth := 1

		for len(queue) > 0 && depth <= maxDepth {
			levelSize := len(queue)
			for i := 0; i < levelSize; i++ {
				current := queue[0]
				queue = queue[1:]

				// Get predecessors (callers)
				preds := g.Predecessors(current)
				for _, pred := range preds {
					if _, seen := visited[pred]; seen {
						continue
					}
					visited[pred] = depth

					if depth <= maxDepth {
						queue = append(queue, pred)
					}

					// Skip if already tracked
					if _, exists := affected[pred]; exists {
						continue
					}

					callerEntity, err := storeDB.GetEntity(pred)
					if err != nil {
						continue
					}

					m, _ := storeDB.GetMetrics(pred)
					affected[pred] = &checkEntity{
						entity:  callerEntity,
						metrics: m,
						depth:   depth,
						direct:  false,
					}
				}
			}
			depth++
		}
	}

	return affected
}

// enrichWithCoverage adds coverage data to affected entities
func enrichWithCoverage(affected map[string]*checkEntity, storeDB *store.Store) {
	for _, e := range affected {
		cov, err := coverage.GetEntityCoverage(storeDB, e.entity.ID)
		if err == nil && cov != nil {
			e.coverage = cov
		}
	}
}

// detectDrift checks for code drift in affected entities
func detectDrift(affected map[string]*checkEntity, storeDB *store.Store, baseDir string) int {
	driftCount := 0

	// Group by file for efficient parsing
	byFile := make(map[string][]*checkEntity)
	for _, e := range affected {
		byFile[e.entity.FilePath] = append(byFile[e.entity.FilePath], e)
	}

	for filePath, entities := range byFile {
		absPath := filePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(baseDir, filePath)
		}

		// Read file
		content, err := os.ReadFile(absPath)
		if err != nil {
			// File might be deleted
			for _, e := range entities {
				e.drifted = true
				e.driftType = "file_missing"
				driftCount++
			}
			continue
		}

		// Detect language from file extension
		lang := detectLanguageForCheck(filePath)
		if lang == "" {
			continue // Unsupported language
		}

		// Parse file
		p, err := parser.NewParser(lang)
		if err != nil {
			continue
		}

		parseResult, err := p.Parse(content)
		p.Close()
		if err != nil {
			continue
		}

		// Extract entities
		ext := extract.NewExtractor(parseResult)
		currentEntities, err := ext.ExtractAll()
		if err != nil {
			continue
		}

		// Build lookup by name
		lookup := make(map[string]*extract.Entity)
		for i := range currentEntities {
			ce := &currentEntities[i]
			lookup[ce.Name] = ce
		}

		// Check each entity for drift
		for _, e := range entities {
			if e.entity.EntityType == "import" {
				continue // Skip imports
			}

			current, found := lookup[e.entity.Name]
			if !found {
				e.drifted = true
				e.driftType = "missing"
				driftCount++
				continue
			}

			// Compare hashes
			if e.entity.SigHash != "" && current.SigHash != "" {
				if e.entity.SigHash != current.SigHash {
					e.drifted = true
					e.driftType = "signature"
					driftCount++
					continue
				}
			}

			if e.entity.BodyHash != "" && current.BodyHash != "" {
				if e.entity.BodyHash != current.BodyHash {
					e.drifted = true
					e.driftType = "body"
					driftCount++
				}
			}
		}
	}

	return driftCount
}

// detectLanguageForCheck detects the parser language from a file path
func detectLanguageForCheck(path string) parser.Language {
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
	default:
		return "" // Unknown
	}
}

// buildCheckOutput creates the final output structure
func buildCheckOutput(target string, affected map[string]*checkEntity, cfg *config.Config, driftCount int) *CheckOutput {
	// Identify keystones using top-N approach (top 5% of entities by PageRank within affected set)
	// This avoids issues with absolute thresholds that don't match actual PageRank distributions
	keystoneThreshold := computeDynamicKeystoneThreshold(affected)

	// Count keystones and coverage gaps
	keystoneCount := 0
	coverageGaps := 0
	var keystones []KeystoneInfo

	for _, e := range affected {
		if e.metrics == nil {
			continue
		}

		isKeystone := e.metrics.PageRank >= keystoneThreshold
		if isKeystone {
			keystoneCount++

			// Check for coverage gap
			hasCoverageGap := false
			coverageStr := "unknown"
			if e.coverage != nil {
				coverageStr = fmt.Sprintf("%.1f%%", e.coverage.CoveragePercent)
				if e.coverage.CoveragePercent < 50 {
					hasCoverageGap = true
					coverageGaps++
				}
			} else {
				hasCoverageGap = true
				coverageGaps++
			}

			impactType := "indirect"
			if e.direct {
				impactType = "direct"
			} else if e.depth == 1 {
				impactType = "caller"
			}

			keystones = append(keystones, KeystoneInfo{
				Name:        e.entity.Name,
				Type:        mapStoreEntityTypeToString(e.entity.EntityType),
				Location:    formatStoreLocation(e.entity),
				PageRank:    e.metrics.PageRank,
				Coverage:    coverageStr,
				Impact:      impactType,
				CoverageGap: hasCoverageGap,
			})
		}
	}

	// Sort keystones by PageRank descending
	sort.Slice(keystones, func(i, j int) bool {
		return keystones[i].PageRank > keystones[j].PageRank
	})

	// Limit to top 10 keystones
	if len(keystones) > 10 {
		keystones = keystones[:10]
	}

	// Count affected files
	files := make(map[string]bool)
	for _, e := range affected {
		files[e.entity.FilePath] = true
	}

	// Determine risk level
	riskLevel := computeCheckRiskLevel(len(affected), keystoneCount, coverageGaps, driftCount)

	// Build warnings
	var warnings []string
	if driftCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d entities have drifted since last scan - run 'cx scan' to update", driftCount))
	}
	if coverageGaps > 0 {
		warnings = append(warnings, fmt.Sprintf("%d keystone entities have inadequate test coverage (<50%%)", coverageGaps))
	}
	for _, k := range keystones {
		if k.CoverageGap {
			warnings = append(warnings, fmt.Sprintf("Keystone '%s' has low coverage (%s) - add tests before modifying", k.Name, k.Coverage))
		}
	}

	// Build recommendations
	recommendations := buildRecommendations(riskLevel, driftCount, coverageGaps, keystoneCount)

	return &CheckOutput{
		SafetyAssessment: &SafetyAssessment{
			Target:        target,
			RiskLevel:     riskLevel,
			ImpactRadius:  len(affected),
			FilesAffected: len(files),
			KeystoneCount: keystoneCount,
			CoverageGaps:  coverageGaps,
			DriftDetected: driftCount > 0,
			DriftedCount:  driftCount,
		},
		Warnings:          warnings,
		Recommendations:   recommendations,
		AffectedKeystones: keystones,
	}
}

// computeDynamicKeystoneThreshold calculates a threshold based on the actual PageRank distribution
// Uses top 5% of entities or minimum of top 10, whichever identifies more keystones
func computeDynamicKeystoneThreshold(affected map[string]*checkEntity) float64 {
	// Collect all PageRank values
	var pageranks []float64
	for _, e := range affected {
		if e.metrics != nil && e.metrics.PageRank > 0 {
			pageranks = append(pageranks, e.metrics.PageRank)
		}
	}

	if len(pageranks) == 0 {
		return 1.0 // No keystones if no metrics
	}

	// Sort descending
	sort.Slice(pageranks, func(i, j int) bool {
		return pageranks[i] > pageranks[j]
	})

	// Take top 5% or minimum of 10 entities
	topN := len(pageranks) / 20 // 5%
	if topN < 10 {
		topN = 10
	}
	if topN > len(pageranks) {
		topN = len(pageranks)
	}

	// Threshold is the PageRank of the Nth entity
	return pageranks[topN-1]
}

// computeCheckRiskLevel determines overall risk level
func computeCheckRiskLevel(impactRadius, keystoneCount, coverageGaps, driftCount int) string {
	// Critical: drift + coverage gaps on keystones
	if driftCount > 0 && coverageGaps > 0 {
		return "critical"
	}

	// Critical: multiple undertested keystones
	if coverageGaps >= 3 {
		return "critical"
	}

	// High: any coverage gaps on keystones
	if coverageGaps > 0 {
		return "high"
	}

	// High: drift detected
	if driftCount > 0 {
		return "high"
	}

	// Medium: multiple keystones affected
	if keystoneCount >= 3 {
		return "medium"
	}

	// Medium: large impact radius
	if impactRadius >= 20 {
		return "medium"
	}

	return "low"
}

// buildRecommendations generates actionable recommendations
func buildRecommendations(riskLevel string, driftCount, coverageGaps, keystoneCount int) []string {
	var recs []string

	switch riskLevel {
	case "critical":
		recs = append(recs, "STOP: Address safety issues before proceeding")
		if driftCount > 0 {
			recs = append(recs, "Run 'cx scan' to update the code graph")
		}
		if coverageGaps > 0 {
			recs = append(recs, "Add tests for undertested keystones before making changes")
		}
		recs = append(recs, "Consider breaking this change into smaller, safer increments")

	case "high":
		recs = append(recs, "Proceed with caution")
		if coverageGaps > 0 {
			recs = append(recs, "Add tests for affected keystones before or alongside changes")
		}
		if driftCount > 0 {
			recs = append(recs, "Run 'cx scan' to ensure graph accuracy")
		}
		recs = append(recs, "Request thorough code review for this change")

	case "medium":
		recs = append(recs, "Proceed with standard review process")
		if keystoneCount > 0 {
			recs = append(recs, "Pay attention to keystone entities in review")
		}
		recs = append(recs, "Run tests after making changes")

	case "low":
		recs = append(recs, "Safe to proceed")
		recs = append(recs, "Run relevant tests after making changes")
	}

	return recs
}

// createCheckTask creates a beads task for safety findings
func createCheckTask(target string, checkOut *CheckOutput) error {
	if !integration.BeadsAvailable() {
		return fmt.Errorf("beads integration not available (bd CLI and .beads/ directory required)")
	}

	// Build description
	var desc strings.Builder
	desc.WriteString("## Safety Check Results\n\n")
	desc.WriteString(fmt.Sprintf("**Target:** `%s`\n", target))
	desc.WriteString(fmt.Sprintf("**Risk Level:** %s\n", strings.ToUpper(checkOut.SafetyAssessment.RiskLevel)))
	desc.WriteString(fmt.Sprintf("**Impact Radius:** %d entities\n", checkOut.SafetyAssessment.ImpactRadius))
	desc.WriteString(fmt.Sprintf("**Keystones Affected:** %d\n", checkOut.SafetyAssessment.KeystoneCount))
	desc.WriteString(fmt.Sprintf("**Coverage Gaps:** %d\n\n", checkOut.SafetyAssessment.CoverageGaps))

	if len(checkOut.Warnings) > 0 {
		desc.WriteString("### Warnings\n")
		for _, w := range checkOut.Warnings {
			desc.WriteString(fmt.Sprintf("- %s\n", w))
		}
		desc.WriteString("\n")
	}

	if len(checkOut.Recommendations) > 0 {
		desc.WriteString("### Recommendations\n")
		for _, r := range checkOut.Recommendations {
			desc.WriteString(fmt.Sprintf("- %s\n", r))
		}
		desc.WriteString("\n")
	}

	if len(checkOut.AffectedKeystones) > 0 {
		desc.WriteString("### Keystones at Risk\n")
		for _, k := range checkOut.AffectedKeystones {
			gap := ""
			if k.CoverageGap {
				gap = " [GAP]"
			}
			desc.WriteString(fmt.Sprintf("- `%s` (%s) - %s%s\n", k.Name, k.Coverage, k.Impact, gap))
		}
	}

	// Determine priority based on risk
	priority := 2
	switch checkOut.SafetyAssessment.RiskLevel {
	case "critical":
		priority = 0
	case "high":
		priority = 1
	}

	title := fmt.Sprintf("Safety Check: %s [%s risk]", filepath.Base(target), strings.ToUpper(checkOut.SafetyAssessment.RiskLevel))

	opts := integration.CreateBeadOptions{
		Title:       title,
		Description: desc.String(),
		Type:        "task",
		Priority:    priority,
		Labels:      []string{"cx:check", "cx:safety"},
	}

	beadID, err := integration.CreateBead(opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "\n# Created task: %s\n", beadID)
	return nil
}

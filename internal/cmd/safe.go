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
	"github.com/anthropics/cx/internal/diff"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/integration"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/semdiff"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// safeCmd represents the safe command (was: check)
var safeCmd = &cobra.Command{
	Use:   "safe [file-or-entity]",
	Short: "Pre-flight safety check before modifying code",
	Long: `Comprehensive safety assessment before modifying a file or entity.

This is the unified safety command that consolidates impact, coverage gaps,
drift verification, and change detection into a single interface.

Modes:
  cx safe <file>           Full safety assessment (default: impact + coverage + drift)
  cx safe <file> --quick   Just blast radius (impact analysis only)
  cx safe --coverage       Just coverage gaps (no target required)
  cx safe --drift          Just staleness check (no target required)
  cx safe --changes        What changed since last scan (no target required)

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
  cx safe src/auth/jwt.go              # Full safety assessment
  cx safe src/auth/jwt.go --quick      # Just blast radius (was: cx impact)
  cx safe --coverage                   # Coverage gaps report (was: cx gaps)
  cx safe --coverage --keystones-only  # Only keystone coverage gaps
  cx safe --drift                      # Staleness check (was: cx verify)
  cx safe --drift --strict             # Exit non-zero on drift (for CI)
  cx safe --changes                    # What changed since scan (was: cx diff)
  cx safe --depth 5 src/core/          # Deeper transitive analysis
  cx safe --format json src/api.go     # JSON output for tooling
  cx safe --create-task src/auth/      # Create beads task for findings`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSafe,
}

var (
	safeDepth      int
	safeCreateTask bool
	// Mode flags - when set, run only that specific analysis
	safeQuick    bool // Just impact analysis (blast radius)
	safeCoverage bool // Just coverage gaps
	safeDrift    bool // Just drift/verify check
	safeChanges  bool // Just diff (what changed since scan)
	// Coverage-specific flags (from gaps command)
	safeKeystonesOnly bool
	safeThreshold     int
	// Drift-specific flags (from verify command)
	safeStrict bool
	safeFix    bool
	safeDryRun bool
	// Diff-specific flags (from diff command)
	safeFile     string
	safeDetailed bool
	safeSemantic bool
	// Impact-specific flags
	safeImpactThreshold float64
	// Inline drift flag
	safeInline bool
)

func init() {
	rootCmd.AddCommand(safeCmd)

	// General flags
	safeCmd.Flags().IntVar(&safeDepth, "depth", 3, "Transitive impact depth")
	safeCmd.Flags().BoolVar(&safeCreateTask, "create-task", false, "Create a beads task for safety findings")

	// Mode flags
	safeCmd.Flags().BoolVar(&safeQuick, "quick", false, "Quick mode: just blast radius/impact analysis")
	safeCmd.Flags().BoolVar(&safeCoverage, "coverage", false, "Coverage mode: show coverage gaps (was: cx gaps)")
	safeCmd.Flags().BoolVar(&safeDrift, "drift", false, "Drift mode: check staleness (was: cx verify)")
	safeCmd.Flags().BoolVar(&safeChanges, "changes", false, "Changes mode: what changed since scan (was: cx diff)")

	// Coverage-specific flags
	safeCmd.Flags().BoolVar(&safeKeystonesOnly, "keystones-only", false, "Only show keystones with gaps (--coverage mode)")
	safeCmd.Flags().IntVar(&safeThreshold, "threshold", 75, "Coverage threshold percentage (--coverage mode)")

	// Drift-specific flags
	safeCmd.Flags().BoolVar(&safeStrict, "strict", false, "Exit non-zero on any drift (--drift mode, for CI)")
	safeCmd.Flags().BoolVar(&safeFix, "fix", false, "Update hashes for drifted entities (--drift mode)")
	safeCmd.Flags().BoolVar(&safeDryRun, "dry-run", false, "Show what --fix would do without making changes")

	// Diff-specific flags
	safeCmd.Flags().StringVar(&safeFile, "file", "", "Show changes for specific file/directory only (--changes mode)")
	safeCmd.Flags().BoolVar(&safeDetailed, "detailed", false, "Show hash changes for modified entities (--changes mode)")
	safeCmd.Flags().BoolVar(&safeSemantic, "semantic", false, "Show semantic analysis (--changes mode)")

	// Impact-specific flags
	safeCmd.Flags().Float64Var(&safeImpactThreshold, "impact-threshold", 0, "Min importance threshold for impact analysis")

	// Inline drift flag
	safeCmd.Flags().BoolVar(&safeInline, "inline", false, "Inline mode: quick drift check for a specific file")
}

// SafeOutput represents the safety check results
type SafeOutput struct {
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

// safeEntity holds entity info for safe analysis
type safeEntity struct {
	entity    *store.Entity
	metrics   *store.Metrics
	coverage  *coverage.EntityCoverage
	depth     int
	direct    bool
	drifted   bool
	driftType string
	tags      []string // Entity tags for warning generation
}

func runSafe(cmd *cobra.Command, args []string) error {
	// Count how many mode flags are set
	modeCount := 0
	if safeCoverage {
		modeCount++
	}
	if safeDrift {
		modeCount++
	}
	if safeChanges {
		modeCount++
	}
	if safeInline {
		modeCount++
	}

	// Validate: only one mode flag at a time (quick is different - it's a modifier)
	if modeCount > 1 {
		return fmt.Errorf("only one of --coverage, --drift, --changes, or --inline can be specified")
	}

	// Route to the appropriate mode handler
	if safeCoverage {
		return runSafeCoverage(cmd, args)
	}
	if safeDrift {
		return runSafeDrift(cmd, args)
	}
	if safeChanges {
		return runSafeChanges(cmd, args)
	}
	if safeInline {
		if len(args) == 0 {
			return fmt.Errorf("target file required for --inline mode")
		}
		return runSafeInline(cmd, args[0])
	}

	// Default mode: full check or quick (impact-only) - requires a target
	if len(args) == 0 {
		return fmt.Errorf("target file or entity required (or use --coverage, --drift, --changes, or --inline)")
	}

	target := args[0]

	if safeQuick {
		return runSafeQuick(cmd, target)
	}

	return runSafeFull(cmd, target)
}

// runSafeFull performs the full safety assessment (impact + coverage + drift)
func runSafeFull(cmd *cobra.Command, target string) error {
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
	directEntities, err := findDirectEntitiesSafe(target, storeDB)
	if err != nil {
		return err
	}

	if len(directEntities) == 0 {
		if isFilePath(target) {
			return fmt.Errorf("no entities found matching: %s\n\nIf this is a file path, ensure:\n  - The file was included in the last scan (run 'cx scan')\n  - The path matches the stored format (try 'cx map' to see paths)\n\nOr use an entity name instead:\n  - Run 'cx find <name>' to discover entities\n  - Use 'cx safe EntityName' with an entity name", target)
		}
		return fmt.Errorf("no entities found matching: %s\n\nTry:\n  - 'cx find %s' to search for similar entities\n  - 'cx map' to see available entities", target, target)
	}

	// Find all affected entities via BFS
	affected := findAffectedEntitiesSafe(directEntities, g, storeDB, cfg, safeDepth)

	// === PHASE 2: Coverage Gap Detection ===
	enrichWithCoverageSafe(affected, storeDB)

	// === PHASE 2.5: Tag Enrichment ===
	enrichWithTagsSafe(affected, storeDB)

	// === PHASE 3: Drift Detection ===
	baseDir, _ := os.Getwd()
	driftCount := detectDriftSafe(affected, storeDB, baseDir)

	// === Build Output ===
	safeOutput := buildSafeOutput(target, affected, cfg, driftCount)

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

	if err := formatter.FormatToWriter(cmd.OutOrStdout(), safeOutput, density); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Create beads task if requested
	if safeCreateTask && len(safeOutput.Warnings) > 0 {
		if err := createSafeTask(target, safeOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create task: %v\n", err)
		}
	}

	return nil
}

// runSafeQuick performs just impact analysis (blast radius) - equivalent to old cx impact
func runSafeQuick(cmd *cobra.Command, target string) error {
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

	// Find direct entities using shared resolution logic
	var directEntries []*impactEntry
	var entities []*store.Entity

	if isFilePath(target) {
		entities = resolveFilePathToEntities(target, storeDB)
	} else {
		entity, err := storeDB.GetEntity(target)
		if err == nil && entity != nil {
			entities = []*store.Entity{entity}
		} else {
			entities, _ = storeDB.QueryEntities(store.EntityFilter{
				Name:   target,
				Status: "active",
				Limit:  10,
			})
		}
	}

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

	if len(directEntries) == 0 {
		if isFilePath(target) {
			return fmt.Errorf("no entities found matching: %s\n\nIf this is a file path, ensure:\n  - The file was included in the last scan (run 'cx scan')\n  - The path matches the stored format (try 'cx map' to see paths)\n\nOr use an entity name instead:\n  - Run 'cx find <name>' to discover entities\n  - Use 'cx safe EntityName' with an entity name", target)
		}
		return fmt.Errorf("no entities found matching: %s\n\nTry:\n  - 'cx find %s' to search for similar entities\n  - 'cx map' to see available entities", target, target)
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
		for len(queue) > 0 && depth <= safeDepth {
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

					if depth <= safeDepth {
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

					if pr >= safeImpactThreshold {
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
	impactOutput := buildImpactOutput(target, affected, safeDepth)

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

// runSafeCoverage shows coverage gaps - equivalent to old cx gaps
func runSafeCoverage(cmd *cobra.Command, args []string) error {
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

		if cov.CoveragePercent >= float64(safeThreshold) {
			continue
		}

		if safeKeystonesOnly && m.PageRank < cfg.Metrics.KeystoneThreshold {
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

	if safeCreateTask {
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

// runSafeDrift performs staleness verification - equivalent to old cx verify
func runSafeDrift(cmd *cobra.Command, args []string) error {
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
	if (safeFix || safeDryRun) && len(result.drifted) > 0 {
		if safeDryRun && !quiet {
			fmt.Fprintf(cmd.OutOrStdout(), "\n[dry-run] Would fix %d drifted entities:\n", len(result.drifted))
		}
		for _, entry := range result.drifted {
			if entry.newSig != "" && entry.newBody != "" {
				if safeDryRun && !quiet {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run]   - %s (%s)\n",
						entry.entity.Name, entry.entity.ID)
				} else if !safeDryRun {
					entry.entity.SigHash = entry.newSig
					entry.entity.BodyHash = entry.newBody
					if err := storeDB.UpdateEntity(entry.entity); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to update %s: %v\n",
							entry.entity.ID, err)
					}
				}
			}
		}
		if safeDryRun && !quiet {
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
		if safeFix {
			actions = append(actions, "Fixed drifted entities")
		} else {
			actions = append(actions, "Run `cx safe --drift --fix` to update hashes for drifted entities")
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
	if safeCreateTask && (len(result.drifted) > 0 || len(result.missing) > 0) {
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
		desc.WriteString("- Run `cx safe --drift --fix` to update hashes for drifted entities\n")

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

	if safeStrict && (len(result.drifted) > 0 || len(result.missing) > 0) {
		return fmt.Errorf("verification failed: %d drifted, %d missing",
			len(result.drifted), len(result.missing))
	}

	return nil
}

// runSafeInline performs inline drift detection for a specific file
// This compares the current file content with the indexed version without a full scan
func runSafeInline(cmd *cobra.Command, target string) error {
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

	// Create inline drift analyzer
	analyzer := diff.NewInlineDriftAnalyzer(storeDB, projectRoot)

	// Analyze the target file
	report, err := analyzer.AnalyzeFile(target)
	if err != nil {
		return fmt.Errorf("failed to analyze file: %w", err)
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

	if err := formatter.FormatToWriter(cmd.OutOrStdout(), report, density); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Exit with error if strict mode and drift detected
	if safeStrict && report.Summary.Status == "drifted" {
		return fmt.Errorf("drift detected: %d signature changes, %d body changes, %d missing",
			report.Summary.SignatureChanges, report.Summary.BodyChanges, report.Summary.MissingEntities)
	}

	return nil
}

// runSafeChanges shows what changed since last scan - equivalent to old cx diff
func runSafeChanges(cmd *cobra.Command, args []string) error {
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
	if safeSemantic {
		cfg, err := config.Load(projectRoot)
		if err != nil {
			cfg = config.DefaultConfig()
		}
		analyzer := semdiff.NewAnalyzer(storeDB, projectRoot, cfg)
		result, err := analyzer.Analyze(safeFile)
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
		if safeFile != "" {
			if entry.FilePath != safeFile && !hasPrefix(entry.FilePath, safeFile) {
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
				if safeDetailed {
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
		if safeFile != "" {
			if relPath != safeFile && !hasPrefix(relPath, safeFile) {
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

// findDirectEntitiesSafe finds entities that match the target (file or entity name)
func findDirectEntitiesSafe(target string, storeDB *store.Store) ([]*safeEntity, error) {
	var results []*safeEntity

	if isFilePath(target) {
		entities := resolveFilePathToEntities(target, storeDB)
		for _, e := range entities {
			m, _ := storeDB.GetMetrics(e.ID)
			results = append(results, &safeEntity{
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
			results = append(results, &safeEntity{
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
					results = append(results, &safeEntity{
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

// resolveFilePathToEntities tries multiple strategies to match a file path to entities
func resolveFilePathToEntities(target string, storeDB *store.Store) []*store.Entity {
	normalizedTarget := normalizeFilePath(target)

	// Strategy 1: Exact/prefix match (current behavior)
	entities, err := storeDB.QueryEntities(store.EntityFilter{
		FilePath: normalizedTarget,
		Status:   "active",
	})
	if err == nil && len(entities) > 0 {
		return entities
	}

	// Strategy 2: Suffix match (handles partial paths like cmd/safe.go -> internal/cmd/safe.go)
	// Strip leading slash for absolute paths to get a usable suffix
	suffix := normalizedTarget
	if strings.HasPrefix(suffix, "/") {
		// For absolute paths, try to find a reasonable suffix
		// e.g., /home/user/project/internal/cmd/safe.go -> internal/cmd/safe.go
		parts := strings.Split(suffix, "/")
		// Try progressively shorter suffixes until we find a match
		for i := 1; i < len(parts); i++ {
			testSuffix := strings.Join(parts[i:], "/")
			if testSuffix == "" {
				continue
			}
			entities, err = storeDB.QueryEntities(store.EntityFilter{
				FilePathSuffix: "/" + testSuffix,
				Status:         "active",
			})
			if err == nil && len(entities) > 0 {
				return entities
			}
		}
	} else {
		// For relative paths, try suffix match directly
		entities, err = storeDB.QueryEntities(store.EntityFilter{
			FilePathSuffix: "/" + normalizedTarget,
			Status:         "active",
		})
		if err == nil && len(entities) > 0 {
			return entities
		}
	}

	// Strategy 3: Basename match (handles just filename like safe.go)
	if !strings.Contains(normalizedTarget, "/") {
		entities, err = storeDB.QueryEntities(store.EntityFilter{
			FilePathSuffix: "/" + normalizedTarget,
			Status:         "active",
		})
		if err == nil && len(entities) > 0 {
			return entities
		}
	}

	// Strategy 4: Try resolving via filesystem if it exists
	if absPath, err := filepath.Abs(target); err == nil {
		if _, statErr := os.Stat(absPath); statErr == nil {
			// File exists - try to find it relative to working directory
			if wd, wdErr := os.Getwd(); wdErr == nil {
				if relPath, relErr := filepath.Rel(wd, absPath); relErr == nil {
					relPath = filepath.ToSlash(relPath) // Normalize to forward slashes
					entities, err = storeDB.QueryEntities(store.EntityFilter{
						FilePath: relPath,
						Status:   "active",
					})
					if err == nil && len(entities) > 0 {
						return entities
					}
				}
			}
		}
	}

	return nil
}

// findAffectedEntitiesSafe performs BFS to find all transitively affected entities
func findAffectedEntitiesSafe(direct []*safeEntity, g *graph.Graph, storeDB *store.Store, cfg *config.Config, maxDepth int) map[string]*safeEntity {
	affected := make(map[string]*safeEntity)

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
					affected[pred] = &safeEntity{
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

// enrichWithCoverageSafe adds coverage data to affected entities
func enrichWithCoverageSafe(affected map[string]*safeEntity, storeDB *store.Store) {
	for _, e := range affected {
		cov, err := coverage.GetEntityCoverage(storeDB, e.entity.ID)
		if err == nil && cov != nil {
			e.coverage = cov
		}
	}
}

// enrichWithTagsSafe adds tags data to affected entities
func enrichWithTagsSafe(affected map[string]*safeEntity, storeDB *store.Store) {
	for _, e := range affected {
		tags, err := storeDB.GetTags(e.entity.ID)
		if err == nil && len(tags) > 0 {
			tagNames := make([]string, len(tags))
			for i, t := range tags {
				tagNames[i] = t.Tag
			}
			e.tags = tagNames
		}
	}
}

// warningTagsSafe defines tags that should generate warnings when entities are modified
var warningTagsSafe = map[string]string{
	"critical":      "CRITICAL: Entity '%s' is tagged as critical - changes require careful review",
	"deprecated":    "DEPRECATED: Entity '%s' is tagged as deprecated - consider removing references instead of modifying",
	"unstable":      "UNSTABLE: Entity '%s' is tagged as unstable - API may change",
	"do-not-modify": "DO NOT MODIFY: Entity '%s' is tagged as do-not-modify - requires special approval",
	"security":      "SECURITY: Entity '%s' is tagged as security-sensitive - changes require security review",
	"breaking":      "BREAKING: Entity '%s' is tagged as breaking - changes may affect downstream consumers",
}

// detectDriftSafe checks for code drift in affected entities
func detectDriftSafe(affected map[string]*safeEntity, storeDB *store.Store, baseDir string) int {
	driftCount := 0

	// Group by file for efficient parsing
	byFile := make(map[string][]*safeEntity)
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
		lang := detectLanguageForSafe(filePath)
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

// detectLanguageForSafe detects the parser language from a file path
func detectLanguageForSafe(path string) parser.Language {
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

// buildSafeOutput creates the final output structure
func buildSafeOutput(target string, affected map[string]*safeEntity, cfg *config.Config, driftCount int) *SafeOutput {
	// Identify keystones using top-N approach (top 5% of entities by PageRank within affected set)
	// This avoids issues with absolute thresholds that don't match actual PageRank distributions
	keystoneThreshold := computeDynamicKeystoneThresholdSafe(affected)

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
	riskLevel := computeSafeRiskLevel(len(affected), keystoneCount, coverageGaps, driftCount)

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

	// Add tag-based warnings for affected entities
	tagWarningsAdded := make(map[string]bool) // Prevent duplicate warnings
	for _, e := range affected {
		if len(e.tags) == 0 {
			continue
		}
		for _, tag := range e.tags {
			tagLower := strings.ToLower(tag)
			if msgTemplate, hasWarning := warningTagsSafe[tagLower]; hasWarning {
				warningKey := e.entity.Name + ":" + tagLower
				if !tagWarningsAdded[warningKey] {
					tagWarningsAdded[warningKey] = true
					warnings = append(warnings, fmt.Sprintf(msgTemplate, e.entity.Name))
				}
			}
		}
	}

	// Build recommendations
	recommendations := buildSafeRecommendations(riskLevel, driftCount, coverageGaps, keystoneCount)

	return &SafeOutput{
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

// computeDynamicKeystoneThresholdSafe calculates a threshold based on the actual PageRank distribution
// Uses top 5% of entities or minimum of top 10, whichever identifies more keystones
func computeDynamicKeystoneThresholdSafe(affected map[string]*safeEntity) float64 {
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

// computeSafeRiskLevel determines overall risk level
func computeSafeRiskLevel(impactRadius, keystoneCount, coverageGaps, driftCount int) string {
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

// buildSafeRecommendations generates actionable recommendations
func buildSafeRecommendations(riskLevel string, driftCount, coverageGaps, keystoneCount int) []string {
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

// createSafeTask creates a beads task for safety findings
func createSafeTask(target string, safeOut *SafeOutput) error {
	if !integration.BeadsAvailable() {
		return fmt.Errorf("beads integration not available (bd CLI and .beads/ directory required)")
	}

	// Build description
	var desc strings.Builder
	desc.WriteString("## Safety Check Results\n\n")
	desc.WriteString(fmt.Sprintf("**Target:** `%s`\n", target))
	desc.WriteString(fmt.Sprintf("**Risk Level:** %s\n", strings.ToUpper(safeOut.SafetyAssessment.RiskLevel)))
	desc.WriteString(fmt.Sprintf("**Impact Radius:** %d entities\n", safeOut.SafetyAssessment.ImpactRadius))
	desc.WriteString(fmt.Sprintf("**Keystones Affected:** %d\n", safeOut.SafetyAssessment.KeystoneCount))
	desc.WriteString(fmt.Sprintf("**Coverage Gaps:** %d\n\n", safeOut.SafetyAssessment.CoverageGaps))

	if len(safeOut.Warnings) > 0 {
		desc.WriteString("### Warnings\n")
		for _, w := range safeOut.Warnings {
			desc.WriteString(fmt.Sprintf("- %s\n", w))
		}
		desc.WriteString("\n")
	}

	if len(safeOut.Recommendations) > 0 {
		desc.WriteString("### Recommendations\n")
		for _, r := range safeOut.Recommendations {
			desc.WriteString(fmt.Sprintf("- %s\n", r))
		}
		desc.WriteString("\n")
	}

	if len(safeOut.AffectedKeystones) > 0 {
		desc.WriteString("### Keystones at Risk\n")
		for _, k := range safeOut.AffectedKeystones {
			gap := ""
			if k.CoverageGap {
				gap = " [GAP]"
			}
			desc.WriteString(fmt.Sprintf("- `%s` (%s) - %s%s\n", k.Name, k.Coverage, k.Impact, gap))
		}
	}

	// Determine priority based on risk
	priority := 2
	switch safeOut.SafetyAssessment.RiskLevel {
	case "critical":
		priority = 0
	case "high":
		priority = 1
	}

	title := fmt.Sprintf("Safety Check: %s [%s risk]", filepath.Base(target), strings.ToUpper(safeOut.SafetyAssessment.RiskLevel))

	opts := integration.CreateBeadOptions{
		Title:       title,
		Description: desc.String(),
		Type:        "task",
		Priority:    priority,
		Labels:      []string{"cx:safe", "cx:safety"},
	}

	beadID, err := integration.CreateBead(opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "\n# Created task: %s\n", beadID)
	return nil
}

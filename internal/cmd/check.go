package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/integration"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check <file-or-entity>",
	Short: "Pre-flight safety check before modifying code",
	Long: `Comprehensive safety assessment before modifying a file or entity.

This command combines impact analysis, coverage gap detection, and drift
verification into a single actionable report. Use it before making changes
to understand the risk and required preparations.

The check performs three analyses:
  1. Impact Analysis - What entities are affected by changes here?
  2. Coverage Gaps - Are affected keystones adequately tested?
  3. Drift Detection - Has the code changed since last scan?

Output Structure:
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
  cx check src/auth/jwt.go              # Check a file before editing
  cx check LoginUser                    # Check an entity before refactoring
  cx check --depth 5 src/core/          # Deeper transitive analysis
  cx check --format json src/api.go     # JSON output for tooling
  cx check --create-task src/auth/      # Create beads task for findings`,
	Args: cobra.ExactArgs(1),
	RunE: runCheck,
}

var (
	checkDepth      int
	checkCreateTask bool
)

func init() {
	rootCmd.AddCommand(checkCmd)

	checkCmd.Flags().IntVar(&checkDepth, "depth", 3, "Transitive impact depth")
	checkCmd.Flags().BoolVar(&checkCreateTask, "create-task", false, "Create a beads task for safety findings")
}

// CheckOutput represents the safety check results
type CheckOutput struct {
	SafetyAssessment *SafetyAssessment `yaml:"safety_assessment" json:"safety_assessment"`
	Warnings         []string          `yaml:"warnings,omitempty" json:"warnings,omitempty"`
	Recommendations  []string          `yaml:"recommendations" json:"recommendations"`
	AffectedKeystones []KeystoneInfo   `yaml:"affected_keystones,omitempty" json:"affected_keystones,omitempty"`
}

// SafetyAssessment contains the aggregate safety metrics
type SafetyAssessment struct {
	Target         string `yaml:"target" json:"target"`
	RiskLevel      string `yaml:"risk_level" json:"risk_level"`
	ImpactRadius   int    `yaml:"impact_radius" json:"impact_radius"`
	FilesAffected  int    `yaml:"files_affected" json:"files_affected"`
	KeystoneCount  int    `yaml:"keystone_count" json:"keystone_count"`
	CoverageGaps   int    `yaml:"coverage_gaps" json:"coverage_gaps"`
	DriftDetected  bool   `yaml:"drift_detected" json:"drift_detected"`
	DriftedCount   int    `yaml:"drifted_count,omitempty" json:"drifted_count,omitempty"`
}

// KeystoneInfo contains details about an affected keystone
type KeystoneInfo struct {
	Name       string  `yaml:"name" json:"name"`
	Type       string  `yaml:"type" json:"type"`
	Location   string  `yaml:"location" json:"location"`
	PageRank   float64 `yaml:"pagerank" json:"pagerank"`
	Coverage   string  `yaml:"coverage" json:"coverage"`
	Impact     string  `yaml:"impact" json:"impact"`
	CoverageGap bool   `yaml:"coverage_gap" json:"coverage_gap"`
}

// checkEntity holds entity info for check analysis
type checkEntity struct {
	entity     *store.Entity
	metrics    *store.Metrics
	coverage   *coverage.EntityCoverage
	depth      int
	direct     bool
	drifted    bool
	driftType  string
}

func runCheck(cmd *cobra.Command, args []string) error {
	target := args[0]

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
				gap = " ⚠️"
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

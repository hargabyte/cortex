package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// gapsCmd represents the gaps command
var gapsCmd = &cobra.Command{
	Use:   "gaps",
	Short: "Find coverage gaps in high-importance code",
	Long: `Identify coverage gaps weighted by code importance.

This command finds entities with low test coverage that are also highly important
based on PageRank and dependency metrics. These gaps represent critical risk areas
where untested code has high impact.

Risk Calculation:
  risk_score = (1 - coverage) × pagerank × in_degree

Risk Categories:
  CRITICAL: Keystones (high PageRank) with <25% coverage
  HIGH:     Keystones with 25-50% coverage
  MEDIUM:   Normal entities with <25% coverage
  LOW:      All others with gaps

Output Structure:
  coverage_gaps:
    critical:  List of critical gaps (keystones, <25% coverage)
    high:      List of high-priority gaps (keystones, 25-50% coverage)
    medium:    List of medium-priority gaps (normal entities, <25% coverage)
  summary:
    keystones_total: Total number of keystone entities
    critical_gaps:   Count of critical gaps
    high_gaps:       Count of high-priority gaps
    medium_gaps:     Count of medium-priority gaps
    recommendation:  Suggested action

Filtering Modes:
  (default)         Show all gaps grouped by risk
  --keystones-only  Show only keystones with gaps
  --threshold N     Only show entities below N% coverage (default: 75)

Examples:
  cx gaps                              # All gaps by risk level
  cx gaps --keystones-only             # Only keystone gaps
  cx gaps --threshold 50               # Only <50% coverage
  cx gaps --create-tasks               # Print bd create commands
  cx gaps --format json                # JSON output

Notes:
  - Requires both 'cx rank' and 'cx coverage import' to have been run
  - Use --create-tasks to generate beads commands for each gap
  - Focuses on high-risk code that needs testing before changes`,
	RunE: runGaps,
}

var (
	gapsKeystonesOnly bool
	gapsThreshold     int
	gapsCreateTasks   bool
)

func init() {
	rootCmd.AddCommand(gapsCmd)

	gapsCmd.Flags().BoolVar(&gapsKeystonesOnly, "keystones-only", false, "Only show keystones with gaps")
	gapsCmd.Flags().IntVar(&gapsThreshold, "threshold", 75, "Coverage threshold percentage")
	gapsCmd.Flags().BoolVar(&gapsCreateTasks, "create-tasks", false, "Print bd create commands for gaps")
}

// coverageGap represents an entity with a coverage gap
type coverageGap struct {
	entity       *store.Entity
	metrics      *store.Metrics
	coverage     *coverage.EntityCoverage
	riskScore    float64
	riskCategory string
}

func runGaps(cmd *cobra.Command, args []string) error {
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
		if cov.CoveragePercent >= float64(gapsThreshold) {
			continue
		}

		// Skip if keystones-only mode and not a keystone
		if gapsKeystonesOnly && m.PageRank < cfg.Metrics.KeystoneThreshold {
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

	// If --create-tasks flag, print bd commands
	if gapsCreateTasks {
		return printTaskCommands(gaps, cfg)
	}

	// Parse format and density
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

// categorizeRisk determines the risk category for a coverage gap
func categorizeRisk(m *store.Metrics, cov *coverage.EntityCoverage, cfg *config.Config) string {
	isKeystone := m.PageRank >= cfg.Metrics.KeystoneThreshold

	if isKeystone {
		if cov.CoveragePercent < 25 {
			return "CRITICAL"
		} else if cov.CoveragePercent < 50 {
			return "HIGH"
		}
		return "MEDIUM"
	}

	// Non-keystone
	if cov.CoveragePercent < 25 {
		return "MEDIUM"
	}
	return "LOW"
}

// groupGapsByRisk groups gaps by their risk category
func groupGapsByRisk(gaps []coverageGap) map[string][]coverageGap {
	result := make(map[string][]coverageGap)
	for _, gap := range gaps {
		category := strings.ToLower(gap.riskCategory)
		result[category] = append(result[category], gap)
	}
	return result
}

// buildGapsOutput constructs the output data structure
func buildGapsOutput(gapsByRisk map[string][]coverageGap, keystoneCount int) map[string]interface{} {
	coverageGaps := make(map[string]interface{})

	// Add each risk category
	for _, category := range []string{"critical", "high", "medium", "low"} {
		if gaps, ok := gapsByRisk[category]; ok && len(gaps) > 0 {
			categoryData := make([]map[string]interface{}, 0, len(gaps))
			for _, gap := range gaps {
				gapData := map[string]interface{}{
					"name":        gap.entity.Name,
					"type":        mapStoreEntityTypeToString(gap.entity.EntityType),
					"location":    formatStoreLocation(gap.entity),
					"importance":  determineImportanceLabel(gap.metrics),
					"in_degree":   gap.metrics.InDegree,
					"pagerank":    gap.metrics.PageRank,
					"coverage":    fmt.Sprintf("%.1f%%", gap.coverage.CoveragePercent),
					"risk_score":  fmt.Sprintf("%.3f", gap.riskScore),
					"risk":        gap.riskCategory,
				}

				// Add recommendation for critical/high
				if gap.riskCategory == "CRITICAL" {
					gapData["recommendation"] = "Add tests before ANY changes"
				} else if gap.riskCategory == "HIGH" {
					gapData["recommendation"] = "Increase test coverage before major changes"
				}

				categoryData = append(categoryData, gapData)
			}
			coverageGaps[category] = categoryData
		}
	}

	// Build summary
	summary := map[string]interface{}{
		"keystones_total": keystoneCount,
		"critical_gaps":   len(gapsByRisk["critical"]),
		"high_gaps":       len(gapsByRisk["high"]),
		"medium_gaps":     len(gapsByRisk["medium"]),
		"low_gaps":        len(gapsByRisk["low"]),
	}

	// Add recommendation based on critical/high gaps
	criticalCount := len(gapsByRisk["critical"])
	highCount := len(gapsByRisk["high"])
	if criticalCount > 0 {
		summary["recommendation"] = "URGENT: Address critical gaps before next release"
	} else if highCount > 0 {
		summary["recommendation"] = "Address high-priority gaps in near term"
	} else {
		summary["recommendation"] = "Continue monitoring coverage for important entities"
	}

	return map[string]interface{}{
		"coverage_gaps": coverageGaps,
		"summary":       summary,
	}
}

// determineImportanceLabel determines the importance label for an entity
func determineImportanceLabel(m *store.Metrics) string {
	// Use same thresholds as rank command
	// Default thresholds: keystone >= 0.30, bottleneck >= 0.20
	cfg, _ := config.Load(".")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if m.PageRank >= cfg.Metrics.KeystoneThreshold {
		return "keystone"
	} else if m.Betweenness >= cfg.Metrics.BottleneckThreshold {
		return "bottleneck"
	} else if m.InDegree == 0 {
		return "leaf"
	}
	return "normal"
}

// printTaskCommands prints bd create commands for each gap
func printTaskCommands(gaps []coverageGap, cfg *config.Config) error {
	fmt.Println("# Coverage gap tasks - run these commands to create beads:")
	fmt.Println()

	for _, gap := range gaps {
		// Skip low-priority gaps when creating tasks
		if gap.riskCategory == "LOW" {
			continue
		}

		// Determine priority based on risk
		priority := 2 // default medium
		if gap.riskCategory == "CRITICAL" {
			priority = 0
		} else if gap.riskCategory == "HIGH" {
			priority = 1
		}

		// Build description
		desc := fmt.Sprintf("Add test coverage for %s (currently %.1f%%)\\n\\n"+
			"Risk: %s\\n"+
			"Importance: %s\\n"+
			"PageRank: %.3f\\n"+
			"In-degree: %d\\n"+
			"Location: %s\\n\\n"+
			"This entity has low test coverage and high importance. "+
			"Add tests before making changes to reduce risk.",
			gap.entity.Name,
			gap.coverage.CoveragePercent,
			gap.riskCategory,
			determineImportanceLabel(gap.metrics),
			gap.metrics.PageRank,
			gap.metrics.InDegree,
			formatStoreLocation(gap.entity),
		)

		// Print bd create command
		fmt.Printf("bd create \"Add tests for %s\" -t task -p %d -d \"%s\"\n\n",
			gap.entity.Name,
			priority,
			desc,
		)
	}

	fmt.Printf("# Total gaps: %d (excluding low-priority)\n", len(gaps))

	return nil
}

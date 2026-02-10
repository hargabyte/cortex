package cmd

import (
	"fmt"
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

var impactCmd = &cobra.Command{
	Use:   "impact <target>",
	Short: "Analyze blast radius of changing a file or entity",
	Long: `Show the forward-looking blast radius of changing a file, entity, or directory.

Answers the question: "If I change this, what breaks?"

Uses forward BFS through the dependency graph to find:
  1. Direct dependents — entities/files that call or import the target (1 hop)
  2. Transitive dependents — 2+ hops out, with diminishing detail
  3. Affected tests — test functions that exercise the changed code
  4. Risk assessment — keystone status, dependent count, test coverage

Examples:
  cx impact src/parser/walk.go              # Impact of changing this file
  cx impact sa-fn-abc123                    # Impact of changing this entity
  cx impact --depth 3 src/parser/walk.go    # Limit hop depth (default: 2)
  cx impact src/parser/                     # Impact of changing this directory
  cx impact --format json src/api.go        # JSON output for tooling`,
	Args: cobra.ExactArgs(1),
	RunE: runImpact,
}

var impactDepth int

func init() {
	rootCmd.AddCommand(impactCmd)
	impactCmd.Flags().IntVar(&impactDepth, "depth", 2, "Max traversal depth (hops from target)")
}

type impactCmdEntry struct {
	entity     *store.Entity
	hop        int
	reason     string
	isKeystone bool
	pageRank   float64
	inDegree   int
	isTest     bool
}

func runImpact(cmd *cobra.Command, args []string) error {
	target := args[0]

	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Resolve target to root entities (reuse from context --for)
	rootEntities, err := resolveForTarget(storeDB, target)
	if err != nil {
		return err
	}
	if len(rootEntities) == 0 {
		return fmt.Errorf("no entities found for target: %s", target)
	}

	// Forward BFS: find all dependents (entities that depend ON the target)
	// This means following predecessors (reverse edges) — who calls us?
	seen := make(map[string]bool)
	for _, e := range rootEntities {
		seen[e.ID] = true
	}

	var affected []impactCmdEntry

	// BFS by hop level
	currentLevel := make([]string, 0, len(rootEntities))
	for _, e := range rootEntities {
		currentLevel = append(currentLevel, e.ID)
	}

	for hop := 1; hop <= impactDepth; hop++ {
		var nextLevel []string

		for _, entityID := range currentLevel {
			// Predecessors = entities that depend on / call this entity
			for _, predID := range g.Predecessors(entityID) {
				if seen[predID] {
					continue
				}
				seen[predID] = true

				pred, err := storeDB.GetEntity(predID)
				if err != nil || pred == nil || pred.Status != "active" {
					continue
				}

				// Determine the source entity name for the reason
				srcEntity, _ := storeDB.GetEntity(entityID)
				srcName := entityID
				if srcEntity != nil {
					srcName = srcEntity.Name
				}

				isTest := strings.Contains(pred.FilePath, "_test") ||
					strings.HasPrefix(strings.ToLower(pred.Name), "test")

				reason := fmt.Sprintf("Depends on %s", srcName)
				if hop == 1 {
					reason = fmt.Sprintf("Directly calls %s", srcName)
				}
				if isTest {
					reason = fmt.Sprintf("Tests %s", srcName)
				}

				entry := impactCmdEntry{
					entity: pred,
					hop:    hop,
					reason: reason,
					isTest: isTest,
				}

				// Get metrics
				m, err := storeDB.GetMetrics(pred.ID)
				if err == nil && m != nil {
					entry.pageRank = m.PageRank
					entry.inDegree = m.InDegree
					entry.isKeystone = m.PageRank >= 0.30
				}

				affected = append(affected, entry)
				nextLevel = append(nextLevel, predID)
			}
		}

		currentLevel = nextLevel
	}

	// Also find tests that cover root entities (via coverage data, not just graph)
	for _, e := range rootEntities {
		testEntities := findRelatedTests(storeDB, e)
		for _, te := range testEntities {
			if seen[te.ID] {
				continue
			}
			seen[te.ID] = true
			affected = append(affected, impactCmdEntry{
				entity: te,
				hop:    1,
				reason: fmt.Sprintf("Tests %s (file association)", e.Name),
				isTest: true,
			})
		}
	}

	// Sort: tests first (for suggested command), then by hop, then by importance
	sort.Slice(affected, func(i, j int) bool {
		if affected[i].hop != affected[j].hop {
			return affected[i].hop < affected[j].hop
		}
		return affected[i].pageRank > affected[j].pageRank
	})

	// Compute risk level
	riskLevel := computeImpactRiskLevel(rootEntities, affected, storeDB)

	// Collect affected files and test packages for suggested command
	affectedFiles := make(map[string]bool)
	testPackages := make(map[string]bool)
	var testNames []string

	for _, a := range affected {
		affectedFiles[a.entity.FilePath] = true
		if a.isTest {
			dir := filepath.Dir(a.entity.FilePath)
			testPackages["./" + dir + "/..."] = true
			if strings.HasPrefix(a.entity.Name, "Test") {
				testNames = append(testNames, a.entity.Name)
			}
		}
	}
	// Also add root entity packages
	for _, e := range rootEntities {
		dir := filepath.Dir(e.FilePath)
		testPackages["./"+dir+"/..."] = true
	}

	// Build output
	impactOut := &output.ImpactOutput{
		Impact: &output.ImpactMetadata{
			Target: target,
			Depth:  impactDepth,
		},
		Summary: &output.ImpactSummary{
			FilesAffected:    len(affectedFiles),
			EntitiesAffected: len(affected),
			RiskLevel:        riskLevel,
		},
		Affected:        make(map[string]*output.AffectedEntity),
		Recommendations: buildRecommendations(riskLevel, testPackages, testNames, rootEntities, storeDB),
	}

	for _, a := range affected {
		location := formatStoreLocation(a.entity)
		entType := mapStoreEntityTypeToString(a.entity.EntityType)

		impactType := "indirect"
		if a.hop == 1 {
			if a.isTest {
				impactType = "test"
			} else {
				impactType = "direct"
			}
		}

		importance := ""
		if a.isKeystone {
			importance = "keystone"
		} else if a.pageRank >= 0.15 {
			importance = "high"
		}

		impactOut.Affected[a.entity.Name] = &output.AffectedEntity{
			Type:       entType,
			Location:   location,
			Impact:     impactType,
			Importance: importance,
			Reason:     a.reason,
		}
	}

	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), impactOut, density)
}

// computeRiskLevel determines the risk of changing the target entities.
func computeImpactRiskLevel(roots []*store.Entity, affected []impactCmdEntry, storeDB *store.Store) string {
	// Check if any root is a keystone
	hasKeystone := false
	for _, e := range roots {
		m, err := storeDB.GetMetrics(e.ID)
		if err == nil && m != nil && m.PageRank >= 0.30 {
			hasKeystone = true
			break
		}
	}

	directCount := 0
	for _, a := range affected {
		if a.hop == 1 && !a.isTest {
			directCount++
		}
	}

	// Check coverage
	hasCoverage := false
	for _, e := range roots {
		cov, err := coverage.GetEntityCoverage(storeDB, e.ID)
		if err == nil && cov != nil && cov.CoveragePercent > 0 {
			hasCoverage = true
			break
		}
	}

	testCount := 0
	for _, a := range affected {
		if a.isTest {
			testCount++
		}
	}

	if hasKeystone || directCount >= 10 {
		return "high"
	}
	if directCount >= 5 || (directCount >= 3 && !hasCoverage && testCount == 0) {
		return "medium"
	}
	return "low"
}

// buildRecommendations generates actionable suggestions.
func buildRecommendations(riskLevel string, testPkgs map[string]bool, testNames []string, roots []*store.Entity, storeDB *store.Store) []string {
	var recs []string

	// Suggested test command
	if len(testPkgs) > 0 {
		pkgs := make([]string, 0, len(testPkgs))
		for p := range testPkgs {
			pkgs = append(pkgs, p)
		}
		sort.Strings(pkgs)

		cmd := "go test " + strings.Join(pkgs, " ")
		if len(testNames) > 0 && len(testNames) <= 5 {
			cmd += " -run " + strings.Join(testNames, "|")
		}
		recs = append(recs, "Run: "+cmd)
	}

	if riskLevel == "high" {
		recs = append(recs, "High risk: review all direct dependents before merging")
	}

	// Check for low coverage on roots
	for _, e := range roots {
		cov, err := coverage.GetEntityCoverage(storeDB, e.ID)
		if err == nil && cov != nil && cov.CoveragePercent < 50 {
			recs = append(recs, fmt.Sprintf("Low coverage on %s (%.0f%%) — add tests before changing", e.Name, cov.CoveragePercent))
		}
	}

	return recs
}

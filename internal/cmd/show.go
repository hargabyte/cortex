package cmd

import (
	"fmt"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <name-or-id>",
	Short: "Show detailed information about a symbol",
	Long: `Display single entity details in YAML format.

Accepts entity names or IDs:
  - Simple names: LoginUser (exact match preferred, then prefix)
  - Qualified names: auth.LoginUser (package.symbol)
  - Path-qualified: auth/login.LoginUser (path/file.symbol)
  - Direct IDs: sa-fn-a7f9b2-LoginUser

Information displayed varies by density:
  sparse:  Type and location only
  medium:  Signature, visibility, basic dependencies (default)
  dense:   Metrics (PageRank, degree), hashes, timestamps, extended dependencies

The --include-metrics flag adds metrics to any density level.
The --coverage flag adds test coverage information (also included in dense mode).

Output Fields:
  - type: Entity type (function, struct, interface, etc.)
  - location: File path and line numbers
  - signature: Function/method signature
  - visibility: public or private
  - dependencies: Calls, called_by, uses_types relationships
  - metrics: PageRank, in_degree, out_degree, importance
  - hashes: Signature and body hashes (dense only)
  - timestamps: Created and updated timestamps (dense only)
  - coverage: Test coverage info with tested_by, percent, uncovered_lines

Examples:
  cx show main                                             # Show entity named "main"
  cx show Store                                            # Show entity named "Store"
  cx show store.Store                                      # Qualified name lookup
  cx show sa-fn-a7f9b2-LoginUser                           # Direct ID lookup
  cx show LoginUser --density=dense                        # Full details with metrics
  cx show LoginUser --density=sparse                       # Minimal output
  cx show LoginUser --include-metrics                      # Add metrics to medium
  cx show LoginUser --coverage                             # Include coverage info
  cx show LoginUser --format=json                          # JSON output`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

var (
	showIncludeMetrics bool
	showCoverage       bool
)

func init() {
	rootCmd.AddCommand(showCmd)

	// Show-specific flags
	showCmd.Flags().BoolVar(&showIncludeMetrics, "include-metrics", false, "Add importance scores")
	showCmd.Flags().BoolVar(&showCoverage, "coverage", false, "Include test coverage information")
}

func runShow(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Parse format and density from global flags
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
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

	// Resolve entity by name or ID
	entity, err := resolveEntityByName(query, storeDB, "")
	if err != nil {
		return err
	}

	entityID := entity.ID

	// Build EntityOutput with name as YAML key
	entityOut := &output.EntityOutput{
		Type:     mapStoreEntityTypeToString(entity.EntityType),
		Location: formatStoreLocation(entity),
	}

	// Add signature for medium/dense
	if density.IncludesSignature() && entity.Signature != "" {
		entityOut.Signature = entity.Signature
	}

	// Add visibility
	if density.IncludesSignature() {
		entityOut.Visibility = inferVisibility(entity.Name)
	}

	// Add dependencies for medium/dense
	if density.IncludesEdges() {
		deps := &output.Dependencies{}

		// Get outgoing calls
		depsOut, _ := storeDB.GetDependenciesFrom(entityID)
		for _, dep := range depsOut {
			if dep.DepType == "calls" {
				deps.Calls = append(deps.Calls, dep.ToID)
			} else if dep.DepType == "uses_type" {
				deps.UsesTypes = append(deps.UsesTypes, dep.ToID)
			}
		}

		// Get incoming calls
		depsIn, _ := storeDB.GetDependenciesTo(entityID)
		for _, dep := range depsIn {
			if dep.DepType == "calls" {
				entry := output.CalledByEntry{
					Name: dep.FromID,
				}
				// Add extended context for dense mode
				if density.IncludesExtendedContext() {
					callerEntity, err := storeDB.GetEntity(dep.FromID)
					if err == nil {
						entry.Location = fmt.Sprintf("%s @ %s", mapStoreEntityTypeToString(callerEntity.EntityType), formatStoreLocation(callerEntity))
					}
				}
				deps.CalledBy = append(deps.CalledBy, entry)
			}
		}

		if len(deps.Calls) > 0 || len(deps.CalledBy) > 0 || len(deps.UsesTypes) > 0 {
			entityOut.Dependencies = deps
		}
	}

	// Add metrics if requested or for dense mode
	if showIncludeMetrics || density.IncludesMetrics() {
		metrics, err := storeDB.GetMetrics(entityID)
		if err == nil && metrics != nil {
			entityOut.Metrics = &output.Metrics{
				PageRank:   metrics.PageRank,
				InDegree:   metrics.InDegree,
				OutDegree:  metrics.OutDegree,
				Importance: computeImportanceFromMetrics(metrics.InDegree, metrics.PageRank),
			}
		}
	}

	// Add hashes for dense mode
	if density.IncludesHashes() && (entity.SigHash != "" || entity.BodyHash != "") {
		entityOut.Hashes = &output.Hashes{
			Signature: entity.SigHash,
			Body:      entity.BodyHash,
		}
	}

	// Add coverage information if requested or in dense mode
	if showCoverage || density.IncludesMetrics() {
		coverageData, err := coverage.GetEntityCoverage(storeDB, entityID)
		if err == nil && coverageData != nil {
			// Get tests that cover this entity
			tests, err := coverage.GetTestsForEntity(storeDB, entityID)
			if err == nil {
				// Build coverage output
				entityOut.Coverage = buildCoverageOutput(coverageData, tests, entity, storeDB)
			}
		}
	}

	// Wrap in a map with entity name as key
	result := map[string]*output.EntityOutput{
		entity.Name: entityOut,
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), result, density)
}

// computeImportanceFromMetrics computes importance level from metrics
func computeImportanceFromMetrics(inDegree int, pageRank float64) string {
	// Prioritize PageRank if available
	if pageRank >= 0.30 {
		return "keystone"
	} else if pageRank >= 0.20 {
		return "bottleneck"
	}

	// Fall back to in-degree
	switch {
	case inDegree >= 10:
		return "keystone"
	case inDegree >= 5:
		return "normal"
	default:
		return "leaf"
	}
}

// buildCoverageOutput creates coverage information for display
func buildCoverageOutput(coverageData *coverage.EntityCoverage, tests []coverage.TestInfo, entity *store.Entity, storeDB *store.Store) *output.Coverage {
	cov := &output.Coverage{
		Tested:         len(tests) > 0,
		Percent:        coverageData.CoveragePercent,
		UncoveredLines: coverageData.UncoveredLines,
	}

	// Build tested_by entries as a map
	if len(tests) > 0 {
		testedBy := make(map[string]*output.TestEntry)
		for _, test := range tests {
			// Try to find the test entity to get location
			testEntity, err := findTestEntity(storeDB, test.TestFile, test.TestName)

			entry := &output.TestEntry{}

			if err == nil && testEntity != nil {
				entry.Location = formatStoreLocation(testEntity)

				// Calculate which lines this test covers
				// For now, we'll show the covered lines that overlap with the entity
				// In a more sophisticated implementation, we would track per-test line coverage
				if len(coverageData.CoveredLines) > 0 {
					// Group covered lines into ranges
					entry.CoversLines = groupLinesIntoRanges(coverageData.CoveredLines)
				}
			} else {
				// Fallback if we can't find the test entity
				entry.Location = test.TestFile
			}

			testedBy[test.TestName] = entry
		}
		cov.TestedBy = testedBy
	}

	return cov
}

// findTestEntity tries to find a test function entity in the store
func findTestEntity(storeDB *store.Store, testFile string, testName string) (*store.Entity, error) {
	if storeDB == nil {
		return nil, fmt.Errorf("store is nil")
	}

	// Query for entities in the test file with matching name
	entities, err := storeDB.QueryEntities(store.EntityFilter{
		FilePath: testFile,
		Name:     testName,
		Status:   "active",
	})

	if err != nil || len(entities) == 0 {
		return nil, fmt.Errorf("test entity not found")
	}

	return entities[0], nil
}

// groupLinesIntoRanges converts a list of line numbers into ranges [[start, end], ...]
func groupLinesIntoRanges(lines []int) [][]int {
	if len(lines) == 0 {
		return nil
	}

	// Sort lines first (should already be sorted, but ensure it)
	sorted := make([]int, len(lines))
	copy(sorted, lines)

	// Simple sort
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var ranges [][]int
	start := sorted[0]
	end := sorted[0]

	for i := 1; i < len(sorted); i++ {
		if sorted[i] == end+1 {
			// Consecutive line, extend range
			end = sorted[i]
		} else {
			// Gap found, save current range and start new one
			ranges = append(ranges, []int{start, end})
			start = sorted[i]
			end = sorted[i]
		}
	}

	// Add final range
	ranges = append(ranges, []int{start, end})

	return ranges
}

// Utility functions moved to utils.go

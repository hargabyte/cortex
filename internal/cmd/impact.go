package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/integration"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// impactCmd represents the impact command
var impactCmd = &cobra.Command{
	Use:   "impact <path-or-entity>",
	Short: "Analyze the impact of changes to a symbol or file",
	Long: `Analyze blast radius of potential changes in YAML format.
Shows which entities are directly affected and transitively impacted.

This is useful for:
  - Understanding blast radius before refactoring
  - Identifying test coverage needed for changes
  - Planning code reviews
  - Assessing risk of modifications

Output Structure:
  impact:           Metadata (target, depth analyzed)
  summary:          Aggregate stats (files_affected, entities_affected, risk_level)
  affected:         Map of impacted entities with impact reason and type
  recommendations:  List of action items and warnings

Impact Types:
  direct:           Entity defined in the modified file/entity
  caller:           Direct dependency on changed entity (1-hop)
  indirect:         Transitive dependency (2+ hops)

Risk Level:
  critical:  Keystones or many dependents affected
  high:      Multiple critical paths impacted
  medium:    Moderate scope of impact
  low:       Isolated changes with few dependents

The analysis shows:
  - Direct entities (in the specified file or matching the entity)
  - Affected entities (depend on the direct entities)
  - Warnings for keystone entities (high PageRank)
  - Recommendations for testing and review

Density Effects:
  sparse:   Type, location, impact category
  medium:   Add metrics and reason (default)
  dense:    Add full dependency chain analysis

Examples:
  cx impact src/auth/jwt.go                   # Impact of changing a file
  cx impact sa-fn-a7f9b2-Validate             # Impact of changing an entity
  cx impact --depth 3 src/auth/               # 3 levels of transitive impact
  cx impact --threshold 0.1 User              # Only show entities with importance >= 0.1
  cx impact src/auth/jwt.go --density=sparse  # Minimal impact analysis
  cx impact src/auth/jwt.go --density=dense   # Full dependency analysis
  cx impact src/auth/jwt.go --format=json     # JSON output`,
	Args: cobra.ExactArgs(1),
	RunE: runImpact,
}

var (
	impactDepth      int
	impactThreshold  float64
	impactCreateTask bool
)

func init() {
	rootCmd.AddCommand(impactCmd)

	// Impact-specific flags
	impactCmd.Flags().IntVar(&impactDepth, "depth", 3, "Transitive depth")
	impactCmd.Flags().Float64Var(&impactThreshold, "threshold", 0, "Min importance threshold (PageRank score)")
	impactCmd.Flags().BoolVar(&impactCreateTask, "create-task", false, "Create a beads task from the impact analysis")
}

// impactEntry holds information about an entity in the impact analysis
type impactEntry struct {
	entityID   string
	name       string
	location   string
	entityType output.CGFEntityType
	depth      int
	direct     bool
	pagerank   float64
	deps       int
	importance string
}

func runImpact(cmd *cobra.Command, args []string) error {
	target := args[0]

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
	var directEntities []*impactEntry

	if isFilePath(target) {
		// Get entities in file from store
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
				directEntities = append(directEntities, &impactEntry{
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
		// Direct entity lookup by ID or name pattern
		entity, err := storeDB.GetEntity(target)
		if err == nil && entity != nil {
			m, _ := storeDB.GetMetrics(entity.ID)
			var pr float64
			var deps int
			if m != nil {
				pr = m.PageRank
				deps = m.InDegree
			}
			directEntities = append(directEntities, &impactEntry{
				entityID:   entity.ID,
				name:       entity.Name,
				location:   formatStoreLocation(entity),
				entityType: mapStoreTypeToCGF(entity.EntityType),
				pagerank:   pr,
				deps:       deps,
				importance: computeImportanceLevel(pr),
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
					var pr float64
					var deps int
					if m != nil {
						pr = m.PageRank
						deps = m.InDegree
					}
					directEntities = append(directEntities, &impactEntry{
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

	if len(directEntities) == 0 {
		return fmt.Errorf("no entities found matching: %s", target)
	}

	// Find affected entities using graph traversal
	affected := make(map[string]*impactEntry)
	recommendations := []string{}

	// Add direct entities
	for _, entry := range directEntities {
		entry.direct = true
		entry.depth = 0
		affected[entry.entityID] = entry

		if entry.pagerank >= 0.30 {
			recommendations = append(recommendations, fmt.Sprintf("Review %s - keystone entity (pr=%.2f, %d direct dependents)",
				entry.name, entry.pagerank, entry.deps))
		}
	}

	// BFS to find transitively affected (entities that depend on direct entities)
	for _, direct := range directEntities {
		depth := 1
		visited := make(map[string]int)
		visited[direct.entityID] = 0

		// BFS with depth tracking
		queue := []string{direct.entityID}
		for len(queue) > 0 && depth <= impactDepth {
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

					if depth <= impactDepth {
						queue = append(queue, pred)
					}

					// Skip if already in affected
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

					if pr >= impactThreshold {
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

	// Limit total results to prevent overwhelming output
	maxResults := 100
	if len(affected) > maxResults {
		// Keep the most important ones
		sortedEntries := make([]*impactEntry, 0, len(affected))
		for _, e := range affected {
			sortedEntries = append(sortedEntries, e)
		}
		sort.Slice(sortedEntries, func(i, j int) bool {
			// Direct first, then by pagerank
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

	// Build ImpactOutput
	impactOutput := buildImpactOutput(target, affected)

	// Deduplicate and add recommendations
	if len(recommendations) > 0 {
		seen := make(map[string]bool)
		for _, rec := range recommendations {
			if !seen[rec] {
				impactOutput.Recommendations = append(impactOutput.Recommendations, rec)
				seen[rec] = true
			}
		}
	}

	// Parse format and density from global flags
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

	if err := formatter.FormatToWriter(cmd.OutOrStdout(), impactOutput, density); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Create beads task if requested
	if impactCreateTask {
		if !integration.BeadsAvailable() {
			return fmt.Errorf("--create-task requires beads integration (bd CLI and .beads/ directory)")
		}

		// Build task title and description
		title := fmt.Sprintf("Impact Analysis: %s affects %d entities", filepath.Base(target), len(affected))

		var desc strings.Builder
		desc.WriteString("## Impact Analysis Results\n\n")
		desc.WriteString(fmt.Sprintf("Target: `%s`\n\n", target))

		// List recommendations
		if len(impactOutput.Recommendations) > 0 {
			desc.WriteString("### Recommendations\n")
			for _, rec := range impactOutput.Recommendations {
				desc.WriteString(fmt.Sprintf("- %s\n", rec))
			}
			desc.WriteString("\n")
		}

		// Build sorted list of affected entities for consistent output
		affectedList := make([]*impactEntry, 0, len(affected))
		for _, e := range affected {
			affectedList = append(affectedList, e)
		}
		sort.Slice(affectedList, func(i, j int) bool {
			if affectedList[i].direct != affectedList[j].direct {
				return affectedList[i].direct
			}
			return affectedList[i].pagerank > affectedList[j].pagerank
		})

		// List affected entities (top 10)
		desc.WriteString(fmt.Sprintf("### Affected Entities (%d total)\n", len(affected)))
		count := 0
		for _, e := range affectedList {
			if count >= 10 {
				desc.WriteString(fmt.Sprintf("... and %d more\n", len(affected)-10))
				break
			}
			if e.direct {
				desc.WriteString(fmt.Sprintf("- `%s` (%s) [DIRECT]\n", e.name, e.location))
			} else {
				desc.WriteString(fmt.Sprintf("- `%s` (%s)\n", e.name, e.location))
			}
			count++
		}

		// Create the task
		opts := integration.CreateBeadOptions{
			Title:       title,
			Description: desc.String(),
			Type:        "task",
			Priority:    2, // Medium
			Labels:      []string{"cx:impact", "cx:review-needed"},
		}

		beadID, err := integration.CreateBead(opts)
		if err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}

		fmt.Fprintf(os.Stdout, "\n# Created task: %s\n", beadID)
	}

	return nil
}

// isFilePath checks if the target looks like a file path
func isFilePath(s string) bool {
	// Check for path separators or common file extensions
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return true
	}
	// Check for common source file extensions
	exts := []string{".go", ".py", ".js", ".ts", ".rs", ".java", ".c", ".cpp", ".h", ".hpp"}
	for _, ext := range exts {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}

// computeImportanceLevel computes importance string from PageRank score
func computeImportanceLevel(pagerank float64) string {
	switch {
	case pagerank >= 0.50:
		return "critical"
	case pagerank >= 0.30:
		return "high"
	case pagerank >= 0.10:
		return "medium"
	default:
		return "low"
	}
}

// buildImpactOutput constructs an ImpactOutput from impact analysis results
func buildImpactOutput(target string, affected map[string]*impactEntry) *output.ImpactOutput {
	impactOut := &output.ImpactOutput{
		Impact: &output.ImpactMetadata{
			Target: target,
			Depth:  impactDepth,
		},
		Summary: &output.ImpactSummary{
			EntitiesAffected: len(affected),
			FilesAffected:    countAffectedFiles(affected),
			RiskLevel:        computeRiskLevel(affected),
		},
		Affected:        make(map[string]*output.AffectedEntity),
		Recommendations: []string{},
	}

	// Build affected entities map
	for _, entry := range affected {
		var impactType string
		var reason string

		if entry.direct {
			impactType = "direct"
			reason = "file was changed"
		} else {
			impactType = "caller"
			if entry.depth > 1 {
				impactType = "indirect"
				reason = fmt.Sprintf("transitively depends on changed entity (depth=%d)", entry.depth)
			} else {
				reason = "calls changed entity"
			}
		}

		affectedEntity := &output.AffectedEntity{
			Type:        mapStoreEntityTypeToString(mapCGFTypeToStoreType(entry.entityType)),
			Location:    entry.location,
			Impact:      impactType,
			Importance:  entry.importance,
			Reason:      reason,
		}

		impactOut.Affected[entry.name] = affectedEntity
	}

	return impactOut
}

// countAffectedFiles counts unique files in affected entities
func countAffectedFiles(affected map[string]*impactEntry) int {
	files := make(map[string]bool)
	for _, entry := range affected {
		// Extract file path from location (format: "path:line-line")
		parts := strings.Split(entry.location, ":")
		if len(parts) > 0 {
			files[parts[0]] = true
		}
	}
	return len(files)
}

// computeRiskLevel computes overall risk level from affected entities
func computeRiskLevel(affected map[string]*impactEntry) string {
	keystoneCount := 0
	for _, entry := range affected {
		if entry.pagerank >= 0.30 {
			keystoneCount++
		}
	}

	// Risk based on keystone percentage
	keystonePercent := float64(keystoneCount) / float64(len(affected))
	switch {
	case keystonePercent > 0.25:
		return "high"
	case keystonePercent > 0.10:
		return "medium"
	default:
		return "low"
	}
}

// mapCGFTypeToStoreType maps CGF type back to store type string
func mapCGFTypeToStoreType(t output.CGFEntityType) string {
	switch t {
	case output.CGFFunction:
		return "function"
	case output.CGFType:
		return "type"
	case output.CGFModule:
		return "module"
	case output.CGFConstant:
		return "constant"
	case output.CGFEnum:
		return "enum"
	default:
		return "function"
	}
}

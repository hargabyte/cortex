package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <name-or-id-or-file:line>",
	Short: "Show detailed information about a symbol",
	Long: `Display single entity details in YAML format.

Accepts entity names, IDs, or file:line locations:
  - Simple names: LoginUser (exact match preferred, then prefix)
  - Qualified names: auth.LoginUser (package.symbol)
  - Path-qualified: auth/login.LoginUser (path/file.symbol)
  - Direct IDs: sa-fn-a7f9b2-LoginUser
  - File:line: internal/auth/login.go:45 (find entity at line)

Information displayed varies by density:
  sparse:  Type and location only
  medium:  Signature, visibility, basic dependencies (default)
  dense:   Metrics (PageRank, degree), hashes, timestamps, extended dependencies

The --include-metrics flag adds metrics to any density level.
The --coverage flag adds test coverage information (also included in dense mode).

Neighborhood Mode (--related):
  Shows the entity's neighborhood - what it calls, what calls it, same-file
  entities, and type relationships. Replaces the old 'cx near' command.

Graph Mode (--graph):
  Visualizes the dependency graph around the entity. Use --hops to control
  traversal depth. Replaces the old 'cx graph' command.

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
  cx show internal/auth/login.go:45                        # Show entity at line 45
  cx show LoginUser --density=dense                        # Full details with metrics
  cx show LoginUser --density=sparse                       # Minimal output
  cx show LoginUser --include-metrics                      # Add metrics to medium
  cx show LoginUser --coverage                             # Include coverage info
  cx show LoginUser --format=json                          # JSON output
  cx show LoginUser --related                              # Show neighborhood
  cx show LoginUser --related --depth 2                    # Two hops neighborhood
  cx show LoginUser --graph                                # Show dependency graph
  cx show LoginUser --graph --hops 3                       # 3-level graph traversal
  cx show LoginUser --graph --direction in                 # Only incoming edges`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

var (
	showIncludeMetrics bool
	showCoverage       bool
	showRelated        bool
	showGraph          bool
	showDepth          int
	showHops           int
	showDirection      string
	showEdgeType       string
)

func init() {
	rootCmd.AddCommand(showCmd)

	// Show-specific flags
	showCmd.Flags().BoolVar(&showIncludeMetrics, "include-metrics", false, "Add importance scores")
	showCmd.Flags().BoolVar(&showCoverage, "coverage", false, "Include test coverage information")

	// Neighborhood flags (--related mode, replaces cx near)
	showCmd.Flags().BoolVar(&showRelated, "related", false, "Show neighborhood (calls, callers, same-file entities)")
	showCmd.Flags().IntVar(&showDepth, "depth", 1, "Hop count for neighborhood traversal (used with --related)")

	// Graph flags (--graph mode, replaces cx graph)
	showCmd.Flags().BoolVar(&showGraph, "graph", false, "Show dependency graph visualization")
	showCmd.Flags().IntVar(&showHops, "hops", 2, "Traversal depth for graph (used with --graph)")
	showCmd.Flags().StringVar(&showDirection, "direction", "both", "Edge direction: in|out|both (used with --related or --graph)")
	showCmd.Flags().StringVar(&showEdgeType, "type", "all", "Edge types: calls|uses_type|implements|all (used with --graph)")
}

func runShow(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Validate direction flag
	if showDirection != "in" && showDirection != "out" && showDirection != "both" {
		return fmt.Errorf("invalid direction %q: must be one of in, out, both", showDirection)
	}

	// Validate edge type flag (for graph mode)
	if showGraph {
		validTypes := map[string]bool{"calls": true, "uses_type": true, "implements": true, "all": true}
		if !validTypes[showEdgeType] {
			return fmt.Errorf("invalid type %q: must be one of calls, uses_type, implements, all", showEdgeType)
		}
	}

	// Validate depth/hops
	if showRelated && showDepth < 1 {
		return fmt.Errorf("depth must be at least 1")
	}
	if showGraph && showHops < 1 {
		return fmt.Errorf("hops must be at least 1")
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

	// Resolve the entity - support name, ID, or file:line
	entity, err := resolveShowQuery(query, storeDB)
	if err != nil {
		return err
	}

	// Route to appropriate mode based on flags
	if showGraph {
		return runShowGraph(cmd, entity, storeDB, format, density)
	}

	if showRelated {
		return runShowRelated(cmd, entity, storeDB, format, density)
	}

	// Default: standard show behavior
	return runShowDefault(cmd, entity, storeDB, format, density)
}

// resolveShowQuery resolves the query to an entity.
// Supports: name, ID, or file:line.
func resolveShowQuery(query string, storeDB *store.Store) (*store.Entity, error) {
	// Check if it's a file:line pattern (e.g., internal/auth/login.go:45)
	if strings.Contains(query, ":") && !isDirectIDQuery(query) {
		parts := strings.Split(query, ":")
		if len(parts) == 2 {
			filePath := parts[0]
			lineStr := parts[1]

			// Check if second part is a number (file:line) or text (qualified name)
			if lineNum, err := strconv.Atoi(lineStr); err == nil {
				return resolveShowEntityAtLine(filePath, lineNum, storeDB)
			}
		}
	}

	// Fall back to standard name/ID resolution
	return resolveEntityByName(query, storeDB, "")
}

// resolveShowEntityAtLine finds the entity at a specific line in a file
func resolveShowEntityAtLine(filePath string, lineNum int, storeDB *store.Store) (*store.Entity, error) {
	// Query entities in the file
	entities, err := storeDB.QueryEntities(store.EntityFilter{
		FilePath: filePath,
		Status:   "active",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities found in file %q", filePath)
	}

	// Find entity that contains this line
	var bestMatch *store.Entity
	for _, e := range entities {
		if e.LineStart <= lineNum {
			endLine := e.LineStart
			if e.LineEnd != nil {
				endLine = *e.LineEnd
			}
			if lineNum <= endLine {
				// Line is within this entity
				if bestMatch == nil || e.LineStart > bestMatch.LineStart {
					// Prefer more specific (inner) entity
					bestMatch = e
				}
			}
		}
	}

	if bestMatch != nil {
		return bestMatch, nil
	}

	// If no exact match, find closest entity before the line
	var closest *store.Entity
	for _, e := range entities {
		if e.LineStart <= lineNum {
			if closest == nil || e.LineStart > closest.LineStart {
				closest = e
			}
		}
	}

	if closest != nil {
		return closest, nil
	}

	return nil, fmt.Errorf("no entity found at or before line %d in %q", lineNum, filePath)
}

// runShowDefault handles the standard show command behavior
func runShowDefault(cmd *cobra.Command, entity *store.Entity, storeDB *store.Store, format output.Format, density output.Density) error {
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

	// Add tags for the entity
	tags, err := storeDB.GetTags(entityID)
	if err == nil && len(tags) > 0 {
		tagNames := make([]string, len(tags))
		for i, t := range tags {
			tagNames[i] = t.Tag
		}
		entityOut.Tags = tagNames
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

// runShowRelated handles the --related flag (neighborhood mode, replaces cx near)
func runShowRelated(cmd *cobra.Command, entity *store.Entity, storeDB *store.Store, format output.Format, density output.Density) error {
	// Build neighborhood output (logic from near.go)
	nearOutput, err := buildShowNeighborhood(entity, storeDB, showDepth, showDirection, density)
	if err != nil {
		return fmt.Errorf("failed to build neighborhood: %w", err)
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), nearOutput, density)
}

// buildShowNeighborhood constructs the neighborhood output for an entity
func buildShowNeighborhood(entity *store.Entity, storeDB *store.Store, depth int, direction string, density output.Density) (*output.NearOutput, error) {
	// Build center entity
	center := &output.NearCenterEntity{
		Name:     entity.Name,
		Type:     mapStoreEntityTypeToString(entity.EntityType),
		Location: formatStoreLocation(entity),
	}

	if density.IncludesSignature() {
		if entity.Signature != "" {
			center.Signature = entity.Signature
		}
		center.Visibility = inferVisibility(entity.Name)
	}

	// Build neighborhood
	neighborhood := &output.Neighborhood{}

	// Track visited entities for BFS traversal
	visited := make(map[string]bool)
	visited[entity.ID] = true

	// Collect neighbors at each depth level
	currentLevel := []*store.Entity{entity}

	for d := 1; d <= depth; d++ {
		var nextLevel []*store.Entity

		for _, current := range currentLevel {
			// Get outgoing calls (what this entity calls)
			if direction == "out" || direction == "both" {
				depsOut, _ := storeDB.GetDependenciesFrom(current.ID)
				for _, dep := range depsOut {
					if visited[dep.ToID] {
						continue
					}

					targetEntity, err := storeDB.GetEntity(dep.ToID)
					if err != nil {
						continue
					}

					visited[dep.ToID] = true
					neighbor := buildShowNeighborEntity(targetEntity, d, density)

					switch dep.DepType {
					case "calls":
						neighborhood.Calls = append(neighborhood.Calls, neighbor)
					case "uses_type":
						neighborhood.UsesTypes = append(neighborhood.UsesTypes, neighbor)
					}

					if d < depth {
						nextLevel = append(nextLevel, targetEntity)
					}
				}
			}

			// Get incoming calls (what calls this entity)
			if direction == "in" || direction == "both" {
				depsIn, _ := storeDB.GetDependenciesTo(current.ID)
				for _, dep := range depsIn {
					if visited[dep.FromID] {
						continue
					}

					sourceEntity, err := storeDB.GetEntity(dep.FromID)
					if err != nil {
						continue
					}

					visited[dep.FromID] = true
					neighbor := buildShowNeighborEntity(sourceEntity, d, density)

					switch dep.DepType {
					case "calls":
						neighborhood.CalledBy = append(neighborhood.CalledBy, neighbor)
					case "uses_type":
						neighborhood.UsedByTypes = append(neighborhood.UsedByTypes, neighbor)
					}

					if d < depth {
						nextLevel = append(nextLevel, sourceEntity)
					}
				}
			}
		}

		currentLevel = nextLevel
	}

	// Get same-file entities (only at depth 1, and only if direction includes relevant file context)
	if depth >= 1 {
		sameFileEntities, _ := storeDB.QueryEntities(store.EntityFilter{
			FilePath: entity.FilePath,
			Status:   "active",
		})

		for _, e := range sameFileEntities {
			if e.ID == entity.ID {
				continue
			}
			if visited[e.ID] {
				continue // Already included via dependencies
			}

			neighbor := buildShowNeighborEntity(e, 0, density)
			neighborhood.SameFile = append(neighborhood.SameFile, neighbor)
		}
	}

	return &output.NearOutput{
		Center:       center,
		Neighborhood: neighborhood,
	}, nil
}

// buildShowNeighborEntity creates a NeighborEntity from a store.Entity
func buildShowNeighborEntity(e *store.Entity, depth int, density output.Density) *output.NeighborEntity {
	neighbor := &output.NeighborEntity{
		Name:     e.Name,
		Type:     mapStoreEntityTypeToString(e.EntityType),
		Location: formatStoreLocation(e),
	}

	if depth > 0 {
		neighbor.Depth = depth
	}

	if density.IncludesSignature() && e.Signature != "" {
		neighbor.Signature = e.Signature
	}

	return neighbor
}

// showGraphNode represents a node in the traversal queue for graph mode
type showGraphNode struct {
	id    string
	depth int
}

// runShowGraph handles the --graph flag (graph visualization mode, replaces cx graph)
func runShowGraph(cmd *cobra.Command, entity *store.Entity, storeDB *store.Store, format output.Format, density output.Density) error {
	entityID := entity.ID

	// Build graph from store
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Build GraphOutput structure
	graphOutput := &output.GraphOutput{
		Graph: &output.GraphMetadata{
			Root:      entity.Name,
			Direction: showDirection,
			Depth:     showHops,
		},
		Nodes: make(map[string]*output.GraphNode),
		Edges: [][]string{},
	}

	// BFS traversal for graph
	visited := make(map[string]bool)
	queue := []showGraphNode{{id: entityID, depth: 0}}
	visitedEdges := make(map[string]bool) // Track edges to avoid duplicates

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		// Get entity details
		currentEntity, err := storeDB.GetEntity(current.id)
		if err != nil {
			// Entity might not exist (external reference), skip gracefully
			continue
		}

		// Add node to graph output
		graphOutput.Nodes[currentEntity.Name] = &output.GraphNode{
			Type:      mapStoreEntityTypeToString(currentEntity.EntityType),
			Location:  formatStoreLocation(currentEntity),
			Depth:     current.depth,
			Signature: currentEntity.Signature,
		}

		// Don't traverse edges if we've reached max hops
		if current.depth >= showHops {
			continue
		}

		// Outgoing edges (what this entity calls/uses)
		if showDirection == "out" || showDirection == "both" {
			successors := g.Successors(current.id)
			for _, targetID := range successors {
				// Get all dependencies from current to target
				deps, err := storeDB.GetDependencies(store.DependencyFilter{
					FromID: current.id,
					ToID:   targetID,
				})

				if err == nil {
					for _, dep := range deps {
						if shouldIncludeShowEdge(dep.DepType, showEdgeType) {
							edgeKey := current.id + "->" + dep.ToID + ":" + dep.DepType
							if !visitedEdges[edgeKey] {
								visitedEdges[edgeKey] = true

								// Get target entity for its name
								targetEntity, err := storeDB.GetEntity(dep.ToID)
								targetName := dep.ToID
								if err == nil {
									targetName = targetEntity.Name
								}

								graphOutput.Edges = append(graphOutput.Edges, []string{
									currentEntity.Name,
									targetName,
									dep.DepType,
								})
							}

							// Queue for further traversal if not visited
							if !visited[dep.ToID] {
								queue = append(queue, showGraphNode{id: dep.ToID, depth: current.depth + 1})
							}
						}
					}
				}
			}
		}

		// Incoming edges (what calls/uses this entity)
		if showDirection == "in" || showDirection == "both" {
			predecessors := g.Predecessors(current.id)
			for _, sourceID := range predecessors {
				edgeKey := sourceID + "->" + current.id + ":calls"
				if !visitedEdges[edgeKey] {
					visitedEdges[edgeKey] = true

					// Get source entity for its name
					sourceEntity, err := storeDB.GetEntity(sourceID)
					sourceName := sourceID
					if err == nil {
						sourceName = sourceEntity.Name
					}

					graphOutput.Edges = append(graphOutput.Edges, []string{
						sourceName,
						currentEntity.Name,
						"calls",
					})
				}

				// Queue for further traversal if not visited
				if !visited[sourceID] {
					queue = append(queue, showGraphNode{id: sourceID, depth: current.depth + 1})
				}
			}
		}
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), graphOutput, density)
}

// shouldIncludeShowEdge checks if a dependency type matches the filter
func shouldIncludeShowEdge(depType, filter string) bool {
	if filter == "all" {
		return true
	}
	return depType == filter
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

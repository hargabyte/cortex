package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

Time Travel Mode (--at):
  Query the code graph at a historical point using Dolt's AS OF feature.
  Supported refs: commit hash, branch, tag, HEAD~N.

Change Tracking Mode (--since):
  Shows if the entity was added, modified, or unchanged since the specified ref.
  Includes change_status field in output: added, modified, or unchanged.

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
  cx show LoginUser --graph --direction in                 # Only incoming edges
  cx show LoginUser --at HEAD~5                            # Show entity 5 commits ago
  cx show LoginUser --at abc123                            # Show entity at commit abc123
  cx show LoginUser --since HEAD~10                        # Show with change status since 10 commits ago`,
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
	showAt             string // Time travel: query at specific commit/ref
	showSince          string // Change tracking: show changes since ref
	// Graph output format flags
	graphFormat   string // "yaml", "d2", "mermaid"
	graphOutput   string // output file path (optional)
	graphMaxNodes int    // max nodes before collapsing
	graphRender   bool   // call d2 CLI to render image
	graphAscii    bool   // render D2 to ASCII art
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

	// Graph format output flags (used with --graph)
	showCmd.Flags().StringVar(&graphFormat, "graph-format", "yaml", "Graph output format: yaml|d2|mermaid (used with --graph)")
	showCmd.Flags().StringVarP(&graphOutput, "output", "o", "", "Write graph to file instead of stdout (used with --graph)")
	showCmd.Flags().IntVar(&graphMaxNodes, "max-nodes", 30, "Maximum nodes before auto-collapsing to modules (used with --graph)")
	showCmd.Flags().BoolVar(&graphRender, "render", false, "Render D2 diagram to SVG (requires d2 CLI, used with --graph-format=d2)")
	showCmd.Flags().BoolVar(&graphAscii, "ascii", false, "Render D2 diagram to ASCII art in terminal (requires d2 CLI, used with --graph-format=d2)")

	// Time travel flag
	showCmd.Flags().StringVar(&showAt, "at", "", "Query at specific commit/ref (e.g., HEAD~5, commit hash, branch)")

	// Change tracking flag
	showCmd.Flags().StringVar(&showSince, "since", "", "Show changes since ref (e.g., HEAD~5, commit hash)")
}

func runShow(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Validate --at flag if provided
	if showAt != "" && !store.IsValidRef(showAt) {
		return fmt.Errorf("invalid --at ref %q: must be commit hash, branch, tag, or HEAD~N", showAt)
	}

	// Validate --since flag if provided
	if showSince != "" && !store.IsValidRef(showSince) {
		return fmt.Errorf("invalid --since ref %q: must be commit hash, branch, tag, or HEAD~N", showSince)
	}

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

		// Validate graph format
		validFormats := map[string]bool{"yaml": true, "d2": true, "mermaid": true}
		if !validFormats[graphFormat] {
			return fmt.Errorf("invalid graph-format %q: must be one of yaml, d2, mermaid", graphFormat)
		}

		// Validate max-nodes
		if graphMaxNodes < 1 {
			return fmt.Errorf("max-nodes must be at least 1")
		}

		// Validate render flag only works with d2
		if graphRender && graphFormat != "d2" {
			return fmt.Errorf("--render flag only works with --graph-format=d2")
		}

		// Validate ascii flag only works with d2
		if graphAscii && graphFormat != "d2" {
			return fmt.Errorf("--ascii flag only works with --graph-format=d2")
		}

		// Can't use both render and ascii
		if graphRender && graphAscii {
			return fmt.Errorf("--render and --ascii flags are mutually exclusive")
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
	entity, err := resolveShowQuery(query, storeDB, showAt)
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
// If ref is non-empty, queries at that historical point using AS OF.
func resolveShowQuery(query string, storeDB *store.Store, ref string) (*store.Entity, error) {
	// Check if it's a file:line pattern (e.g., internal/auth/login.go:45)
	if strings.Contains(query, ":") && !isDirectIDQuery(query) {
		parts := strings.Split(query, ":")
		if len(parts) == 2 {
			filePath := parts[0]
			lineStr := parts[1]

			// Check if second part is a number (file:line) or text (qualified name)
			if lineNum, err := strconv.Atoi(lineStr); err == nil {
				return resolveShowEntityAtLine(filePath, lineNum, storeDB, ref)
			}
		}
	}

	// Fall back to standard name/ID resolution
	return resolveEntityByName(query, storeDB, ref)
}

// resolveShowEntityAtLine finds the entity at a specific line in a file
// If ref is non-empty, queries at that historical point using AS OF.
func resolveShowEntityAtLine(filePath string, lineNum int, storeDB *store.Store, ref string) (*store.Entity, error) {
	// Query entities in the file
	filter := store.EntityFilter{
		FilePath: filePath,
		Status:   "active",
	}

	var entities []*store.Entity
	var err error
	if ref != "" {
		entities, err = storeDB.QueryEntitiesAt(filter, ref)
	} else {
		entities, err = storeDB.QueryEntities(filter)
	}
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

		// Get outgoing calls (use AS OF if --at specified)
		var depsOut []*store.Dependency
		if showAt != "" {
			depsOut, _ = storeDB.GetDependenciesFromAt(entityID, showAt)
		} else {
			depsOut, _ = storeDB.GetDependenciesFrom(entityID)
		}
		for _, dep := range depsOut {
			if dep.DepType == "calls" {
				deps.Calls = append(deps.Calls, dep.ToID)
			} else if dep.DepType == "uses_type" {
				deps.UsesTypes = append(deps.UsesTypes, dep.ToID)
			}
		}

		// Get incoming calls (use AS OF if --at specified)
		var depsIn []*store.Dependency
		if showAt != "" {
			depsIn, _ = storeDB.GetDependenciesToAt(entityID, showAt)
		} else {
			depsIn, _ = storeDB.GetDependenciesTo(entityID)
		}
		for _, dep := range depsIn {
			if dep.DepType == "calls" {
				entry := output.CalledByEntry{
					Name: dep.FromID,
				}
				// Add extended context for dense mode
				if density.IncludesExtendedContext() {
					var callerEntity *store.Entity
					var err error
					if showAt != "" {
						callerEntity, err = storeDB.GetEntityAt(dep.FromID, showAt)
					} else {
						callerEntity, err = storeDB.GetEntity(dep.FromID)
					}
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

	// Add change status if --since is specified
	if showSince != "" {
		changeStatus, err := getEntityChangeStatus(storeDB, entity, showSince)
		if err == nil && changeStatus != "" {
			entityOut.ChangeStatus = changeStatus
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
	graphData := &output.GraphOutput{
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
		graphData.Nodes[currentEntity.Name] = &output.GraphNode{
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

								graphData.Edges = append(graphData.Edges, []string{
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

					graphData.Edges = append(graphData.Edges, []string{
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

	// Handle different output formats
	var outputStr string

	switch graphFormat {
	case "d2":
		// Convert direction to D2 format
		d2Direction := "right"
		if showDirection == "down" || showDirection == "in" {
			d2Direction = "down"
		}

		opts := &graph.D2Options{
			MaxNodes:   graphMaxNodes,
			Direction:  d2Direction,
			ShowLabels: true,
			Collapse:   true,
			Title:      entity.Name,
		}
		outputStr = graph.GenerateD2(graphData.Nodes, graphData.Edges, opts)

		// Handle render flag - call d2 CLI to render SVG
		if graphRender {
			return renderD2(cmd, outputStr, graphOutput, "svg")
		}

		// Handle ascii flag - render to ASCII art
		if graphAscii {
			return renderD2(cmd, outputStr, "", "ascii")
		}

	case "mermaid":
		// Convert direction to Mermaid format
		mermaidDirection := "LR"
		if showDirection == "down" || showDirection == "in" {
			mermaidDirection = "TD"
		}

		opts := &graph.MermaidOptions{
			MaxNodes:  graphMaxNodes,
			Direction: mermaidDirection,
			ChartType: "flowchart",
			Collapse:  true,
			Title:     entity.Name,
		}
		outputStr = graph.GenerateMermaid(graphData.Nodes, graphData.Edges, opts)

	default: // "yaml"
		// Use existing formatter for YAML/JSON output
		formatter, err := output.GetFormatter(format)
		if err != nil {
			return fmt.Errorf("failed to get formatter: %w", err)
		}
		return formatter.FormatToWriter(cmd.OutOrStdout(), graphData, density)
	}

	// Write output to file or stdout
	if graphOutput != "" {
		if err := writeGraphToFile(graphOutput, outputStr); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph written to %s\n", graphOutput)
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), outputStr)
	return nil
}

// renderD2 calls the d2 CLI to render a D2 diagram
// format can be "svg", "png", or "ascii"
func renderD2(cmd *cobra.Command, d2Content string, outputFile string, format string) error {
	// Create temp file for D2 input
	tmpFile, err := os.CreateTemp("", "cx-graph-*.d2")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(d2Content); err != nil {
		return fmt.Errorf("failed to write D2 content: %w", err)
	}
	tmpFile.Close()

	// Find d2 binary - check common locations
	d2Path, err := exec.LookPath("d2")
	if err != nil {
		// Try ~/.local/bin/d2
		homeDir, _ := os.UserHomeDir()
		localD2 := filepath.Join(homeDir, ".local", "bin", "d2")
		if _, statErr := os.Stat(localD2); statErr == nil {
			d2Path = localD2
		} else {
			return fmt.Errorf("d2 CLI not found. Install with: curl -fsSL https://d2lang.com/install.sh | sh")
		}
	}

	// Handle ASCII output to stdout
	if format == "ascii" {
		runCmd := exec.Command(d2Path, tmpFile.Name(), "-", "--stdout-format", "ascii")
		output, err := runCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("d2 render failed: %s\n%s", err, string(output))
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	}

	// Determine output file for SVG/PNG
	renderOutput := outputFile
	if renderOutput == "" {
		renderOutput = "graph." + format
	} else if !strings.HasSuffix(renderOutput, "."+format) {
		renderOutput = strings.TrimSuffix(renderOutput, filepath.Ext(renderOutput)) + "." + format
	}

	// Run d2 command
	runCmd := exec.Command(d2Path, tmpFile.Name(), renderOutput)
	output, err := runCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("d2 render failed: %s\n%s", err, string(output))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rendered to %s\n", renderOutput)
	return nil
}

// writeGraphToFile writes graph output to a file
func writeGraphToFile(filePath, content string) error {
	return os.WriteFile(filePath, []byte(content), 0644)
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

// getEntityChangeStatus returns the change status of an entity since a ref.
// Returns: "added", "modified", "unchanged", or "" on error.
func getEntityChangeStatus(storeDB *store.Store, entity *store.Entity, sinceRef string) (string, error) {
	// Query dolt_diff to find if this entity was changed since the ref
	diffOpts := store.DiffOptions{
		FromRef:  sinceRef,
		ToRef:    "HEAD",
		Table:    "entities",
		EntityID: entity.ID,
	}

	diffResult, err := storeDB.DoltDiff(diffOpts)
	if err != nil {
		return "", err
	}

	// Check if entity is in any of the change categories
	for _, c := range diffResult.Added {
		if c.EntityID == entity.ID {
			return "added", nil
		}
	}
	for _, c := range diffResult.Modified {
		if c.EntityID == entity.ID {
			return "modified", nil
		}
	}
	for _, c := range diffResult.Removed {
		if c.EntityID == entity.ID {
			return "removed", nil
		}
	}

	return "unchanged", nil
}

// Utility functions moved to utils.go

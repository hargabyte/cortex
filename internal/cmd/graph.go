package cmd

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// graphCmd represents the graph command
var graphCmd = &cobra.Command{
	Use:   "graph <name-or-id>",
	Short: "Visualize the dependency graph around a symbol",
	Long: `Navigate call graphs and dependencies in YAML format.
Shows what an entity calls (out), what calls it (in), or both.

Accepts entity names or IDs:
  - Simple names: LoginUser (exact match preferred, then prefix)
  - Qualified names: auth.LoginUser (package.symbol)
  - Path-qualified: auth/login.LoginUser (path/file.symbol)
  - Direct IDs: sa-fn-a7f9b2-LoginUser

Output Structure:
  graph:  Metadata (root entity, direction, depth)
  nodes:  Map of entities in the graph with their details
  edges:  Array of [from, to, type] edge tuples

Graph Metadata:
  root:       The starting entity
  direction:  The traversal direction (in, out, or both)
  depth:      Maximum depth reached

Node Details (depends on density):
  sparse:     Type and location
  medium:     Add signature
  dense:      Add metrics, importance scores

Edge Types:
  - calls:        Function/method invocations
  - uses_type:    Type usage relationships
  - implements:   Interface implementation relationships
  - all:          All edge types (default)

Direction Modes:
  out:   Show entities this symbol calls/depends on (forward dependencies)
  in:    Show entities that call/depend on this symbol (reverse dependencies)
  both:  Show bidirectional relationships (default)

Density Levels affect node detail:
  sparse:  Type and location
  medium:  Add signature and visibility
  dense:   Add metrics, importance, betweenness
  smart:   Adaptive based on node importance

Examples:
  cx graph main                                   # Graph for entity "main"
  cx graph Execute --hops=1                       # 1 hop from Execute
  cx graph store.Store --direction=in             # What calls Store
  cx graph LoginUser --direction=out              # Show what LoginUser calls
  cx graph handleRequest --hops=3                 # 3 levels of traversal
  cx graph LoginUser --type=calls                 # Only call relationships
  cx graph LoginUser --density=sparse             # Minimal node details
  cx graph LoginUser --density=dense              # Full metrics
  cx graph LoginUser --format=json                # JSON output`,
	Args: cobra.ExactArgs(1),
	RunE: runGraph,
}

var (
	graphDirection string
	graphHops      int
	graphEdgeType  string
)

func init() {
	rootCmd.AddCommand(graphCmd)

	// Graph-specific flags matching the spec
	graphCmd.Flags().StringVar(&graphDirection, "direction", "both", "Edge direction (out|in|both)")
	graphCmd.Flags().IntVar(&graphHops, "hops", 2, "Traversal depth")
	graphCmd.Flags().StringVar(&graphEdgeType, "type", "all", "Edge types (calls|uses_type|implements|all)")
}

// graphNode represents a node in the traversal queue
type graphNode struct {
	id    string
	depth int
}

func runGraph(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Validate direction flag
	if graphDirection != "out" && graphDirection != "in" && graphDirection != "both" {
		return fmt.Errorf("invalid direction %q: must be one of out, in, both", graphDirection)
	}

	// Validate edge type flag
	validTypes := map[string]bool{"calls": true, "uses_type": true, "implements": true, "all": true}
	if !validTypes[graphEdgeType] {
		return fmt.Errorf("invalid type %q: must be one of calls, uses_type, implements, all", graphEdgeType)
	}

	// Validate hops
	if graphHops < 1 {
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

	// Find and open store
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
	rootEntity, err := resolveEntityByName(query, storeDB, "")
	if err != nil {
		return err
	}

	entityID := rootEntity.ID

	// Build graph from store
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Build GraphOutput structure
	graphOutput := &output.GraphOutput{
		Graph: &output.GraphMetadata{
			Root:      rootEntity.Name,
			Direction: graphDirection,
			Depth:     graphHops,
		},
		Nodes: make(map[string]*output.GraphNode),
		Edges: [][]string{},
	}

	// BFS traversal for graph
	visited := make(map[string]bool)
	queue := []graphNode{{id: entityID, depth: 0}}
	visitedEdges := make(map[string]bool) // Track edges to avoid duplicates

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		// Get entity details
		entity, err := storeDB.GetEntity(current.id)
		if err != nil {
			// Entity might not exist (external reference), skip gracefully
			continue
		}

		// Add node to graph output
		graphOutput.Nodes[entity.Name] = &output.GraphNode{
			Type:      mapStoreEntityTypeToString(entity.EntityType),
			Location:  formatStoreLocation(entity),
			Depth:     current.depth,
			Signature: entity.Signature,
		}

		// Don't traverse edges if we've reached max hops
		if current.depth >= graphHops {
			continue
		}

		// Outgoing edges (what this entity calls/uses)
		if graphDirection == "out" || graphDirection == "both" {
			successors := g.Successors(current.id)
			for _, targetID := range successors {
				// Get all dependencies from current to target
				deps, err := storeDB.GetDependencies(store.DependencyFilter{
					FromID: current.id,
					ToID:   targetID,
				})

				if err == nil {
					for _, dep := range deps {
						if shouldIncludeEdge(dep.DepType, graphEdgeType) {
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
									entity.Name,
									targetName,
									dep.DepType,
								})
							}

							// Queue for further traversal if not visited
							if !visited[dep.ToID] {
								queue = append(queue, graphNode{id: dep.ToID, depth: current.depth + 1})
							}
						}
					}
				}
			}
		}

		// Incoming edges (what calls/uses this entity)
		if graphDirection == "in" || graphDirection == "both" {
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
						entity.Name,
						"calls",
					})
				}

				// Queue for further traversal if not visited
				if !visited[sourceID] {
					queue = append(queue, graphNode{id: sourceID, depth: current.depth + 1})
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

// shouldIncludeEdge checks if a dependency type matches the filter
func shouldIncludeEdge(depType, filter string) bool {
	if filter == "all" {
		return true
	}
	return depType == filter
}

// mapStoreTypeToCGF maps store entity types to CGF entity types
func mapStoreTypeToCGF(entityType string) output.CGFEntityType {
	switch strings.ToLower(entityType) {
	case "function":
		return output.CGFFunction
	case "type":
		return output.CGFType
	case "constant", "const":
		return output.CGFConstant
	case "var", "variable":
		return output.CGFConstant // Variables map to Constant in CGF
	case "enum":
		return output.CGFEnum
	case "import":
		return output.CGFImport
	default:
		return output.CGFFunction // Default to function
	}
}

// mapDepTypeToEdgeType converts a beads dependency type to CGF edge type
func mapDepTypeToEdgeType(depType string) output.CGFEdgeType {
	switch depType {
	case "calls":
		return output.CGFCalls
	case "uses_type":
		return output.CGFUsesType
	case "implements":
		return output.CGFImplements
	case "blocks":
		return output.CGFBlocks
	case "related":
		return output.CGFRelated
	default:
		// For beads-style dependencies, map to appropriate type
		if depType == "discovered-from" {
			return output.CGFRelated
		}
		return output.CGFRelated
	}
}

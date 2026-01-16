package cmd

import (
	"fmt"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// traceCmd represents the trace command for call chain tracing
var traceCmd = &cobra.Command{
	Use:   "trace <from> [to]",
	Short: "Trace call chains between entities",
	Long: `Trace call paths between code entities.

Shows the call chain connecting two entities, or traces upstream callers
or downstream callees from a single entity.

Accepts entity identifiers:
  - Simple names: LoginUser (exact match preferred, then prefix)
  - Qualified names: auth.LoginUser (package.symbol)
  - Path-qualified: auth/login.LoginUser (path/file.symbol)
  - Direct IDs: sa-fn-a7f9b2-LoginUser

Modes:
  Path mode (default):
    cx trace <from> <to>    Show shortest path from <from> to <to>
    cx trace <from> <to> --all   Show all paths (up to --depth)

  Caller mode:
    cx trace <entity> --callers   Show what calls this entity

  Callee mode:
    cx trace <entity> --callees   Show what this entity calls

Output:
  By default, shows the shortest path as a chain of entities.
  With --all, shows all discovered paths.
  With --callers or --callees, shows the call hierarchy.

Examples:
  cx trace HandleRequest SaveUser           # Show path from HandleRequest to SaveUser
  cx trace HandleRequest SaveUser --all     # Show all paths
  cx trace SaveUser --callers               # Show what calls SaveUser
  cx trace SaveUser --callers --depth 3     # Show callers up to 3 hops
  cx trace HandleRequest --callees          # Show what HandleRequest calls
  cx trace "Auth*" "database" --all         # Pattern matching (all paths)`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTrace,
}

var (
	traceCallers bool
	traceCallees bool
	traceAll     bool
	traceDepth   int
)

func init() {
	rootCmd.AddCommand(traceCmd)

	traceCmd.Flags().BoolVar(&traceCallers, "callers", false, "Trace upstream callers")
	traceCmd.Flags().BoolVar(&traceCallees, "callees", false, "Trace downstream callees")
	traceCmd.Flags().BoolVar(&traceAll, "all", false, "Show all paths (not just shortest)")
	traceCmd.Flags().IntVar(&traceDepth, "depth", 5, "Maximum trace depth")
}

func runTrace(cmd *cobra.Command, args []string) error {
	// Validate flags
	if traceCallers && traceCallees {
		return fmt.Errorf("cannot specify both --callers and --callees")
	}

	if (traceCallers || traceCallees) && len(args) > 1 {
		return fmt.Errorf("--callers and --callees modes require exactly one entity")
	}

	if !traceCallers && !traceCallees && len(args) < 2 {
		return fmt.Errorf("path mode requires two entities: <from> <to>")
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

	// Build graph
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	var traceOutput *output.TraceOutput

	if traceCallers {
		traceOutput, err = runTraceCallers(args[0], storeDB, g)
	} else if traceCallees {
		traceOutput, err = runTraceCallees(args[0], storeDB, g)
	} else {
		traceOutput, err = runTracePath(args[0], args[1], storeDB, g)
	}

	if err != nil {
		return err
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), traceOutput, density)
}

// runTracePath traces the path between two entities
func runTracePath(fromQuery, toQuery string, storeDB *store.Store, g *graph.Graph) (*output.TraceOutput, error) {
	// Resolve from entity
	fromEntity, err := resolveEntityByName(fromQuery, storeDB, "")
	if err != nil {
		return nil, fmt.Errorf("could not resolve 'from' entity: %w", err)
	}

	// Resolve to entity
	toEntity, err := resolveEntityByName(toQuery, storeDB, "")
	if err != nil {
		return nil, fmt.Errorf("could not resolve 'to' entity: %w", err)
	}

	traceOutput := &output.TraceOutput{
		Trace: &output.TraceMetadata{
			From:  fromEntity.Name,
			To:    toEntity.Name,
			Mode:  "path",
			Depth: traceDepth,
		},
	}

	if traceAll {
		// Find all paths
		allPaths := g.AllPaths(fromEntity.ID, toEntity.ID, traceDepth)
		traceOutput.Trace.PathFound = len(allPaths) > 0
		traceOutput.Trace.PathCount = len(allPaths)

		// Convert paths to output format
		for _, path := range allPaths {
			nodeNames := make([]string, len(path))
			for i, nodeID := range path {
				entity, err := storeDB.GetEntity(nodeID)
				if err != nil {
					nodeNames[i] = nodeID
				} else {
					nodeNames[i] = entity.Name
				}
			}
			traceOutput.AllPaths = append(traceOutput.AllPaths, output.TracePathList{
				Length: len(path) - 1, // Number of hops
				Nodes:  nodeNames,
			})
		}
	} else {
		// Find shortest path
		path := g.ShortestPath(fromEntity.ID, toEntity.ID, "forward")
		traceOutput.Trace.PathFound = len(path) > 0

		if len(path) > 0 {
			traceOutput.Trace.PathCount = 1
			traceOutput.Path = buildPathNodes(path, storeDB)
		}
	}

	return traceOutput, nil
}

// runTraceCallers traces upstream callers of an entity
func runTraceCallers(query string, storeDB *store.Store, g *graph.Graph) (*output.TraceOutput, error) {
	// Resolve entity
	entity, err := resolveEntityByName(query, storeDB, "")
	if err != nil {
		return nil, fmt.Errorf("could not resolve entity: %w", err)
	}

	traceOutput := &output.TraceOutput{
		Trace: &output.TraceMetadata{
			Target: entity.Name,
			Mode:   "callers",
			Depth:  traceDepth,
		},
	}

	// Collect callers using BFS
	callerChain := g.CollectCallerChain(entity.ID, traceDepth)

	// Build caller nodes (skip first element which is the target itself)
	for i, nodeID := range callerChain {
		nodeEntity, err := storeDB.GetEntity(nodeID)
		if err != nil {
			continue
		}

		node := &output.TracePathNode{
			Name:     nodeEntity.Name,
			Type:     mapStoreEntityTypeToString(nodeEntity.EntityType),
			Location: formatStoreLocation(nodeEntity),
			Depth:    i, // Distance from target (0 = target, 1 = direct caller, etc.)
		}

		if nodeEntity.Signature != "" {
			node.Signature = nodeEntity.Signature
		}

		traceOutput.Callers = append(traceOutput.Callers, node)
	}

	return traceOutput, nil
}

// runTraceCallees traces downstream callees of an entity
func runTraceCallees(query string, storeDB *store.Store, g *graph.Graph) (*output.TraceOutput, error) {
	// Resolve entity
	entity, err := resolveEntityByName(query, storeDB, "")
	if err != nil {
		return nil, fmt.Errorf("could not resolve entity: %w", err)
	}

	traceOutput := &output.TraceOutput{
		Trace: &output.TraceMetadata{
			Target: entity.Name,
			Mode:   "callees",
			Depth:  traceDepth,
		},
	}

	// Collect callees using BFS
	calleeChain := g.CollectCalleeChain(entity.ID, traceDepth)

	// Build callee nodes (skip first element which is the source itself)
	for i, nodeID := range calleeChain {
		nodeEntity, err := storeDB.GetEntity(nodeID)
		if err != nil {
			continue
		}

		node := &output.TracePathNode{
			Name:     nodeEntity.Name,
			Type:     mapStoreEntityTypeToString(nodeEntity.EntityType),
			Location: formatStoreLocation(nodeEntity),
			Depth:    i, // Distance from source (0 = source, 1 = direct callee, etc.)
		}

		if nodeEntity.Signature != "" {
			node.Signature = nodeEntity.Signature
		}

		traceOutput.Callees = append(traceOutput.Callees, node)
	}

	return traceOutput, nil
}

// buildPathNodes converts a path of entity IDs to TracePathNodes
func buildPathNodes(path []string, storeDB *store.Store) []*output.TracePathNode {
	nodes := make([]*output.TracePathNode, 0, len(path))

	for i, nodeID := range path {
		entity, err := storeDB.GetEntity(nodeID)
		if err != nil {
			// Use ID as fallback
			nodes = append(nodes, &output.TracePathNode{
				Name:  nodeID,
				Type:  "unknown",
				Depth: i,
			})
			continue
		}

		node := &output.TracePathNode{
			Name:     entity.Name,
			Type:     mapStoreEntityTypeToString(entity.EntityType),
			Location: formatStoreLocation(entity),
			Depth:    i,
		}

		if entity.Signature != "" {
			node.Signature = entity.Signature
		}

		nodes = append(nodes, node)
	}

	return nodes
}

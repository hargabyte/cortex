package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// guideCmd represents the guide command
var guideCmd = &cobra.Command{
	Use:   "guide [subcommand]",
	Short: "Generate codebase documentation and diagrams",
	Long: `Generate comprehensive documentation and visual diagrams of your codebase.

The guide command helps you understand and document your codebase through
statistics, architecture diagrams, and dependency analysis.

Subcommands:
  overview   - Stats + architecture diagram (default if no subcommand)
  hotspots   - Important entities: keystones, bottlenecks
  modules    - Module breakdown with diagrams
  deps       - Dependency layer analysis with cycle detection

Output Formats:
  --format mermaid   - Mermaid diagrams (default, works in GitHub/GitLab)
  --format d2        - D2 diagrams (requires d2 CLI for rendering)

Examples:
  cx guide                        # Same as 'cx guide overview'
  cx guide overview               # Stats and architecture diagram
  cx guide hotspots               # Top keystones and bottlenecks
  cx guide modules                # Module breakdown
  cx guide deps                   # Dependency analysis
  cx guide overview --format d2   # Use D2 format instead of Mermaid
  cx guide overview -o arch.md    # Write to file`,
	Args: cobra.MaximumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to overview subcommand
		return runGuideOverview(cmd, args)
	},
}

// Subcommand variables
var guideOverviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Show codebase stats and architecture diagram",
	Long: `Display codebase statistics and generate an architecture overview diagram.

This includes:
  - Entity counts by type (functions, types, methods, etc.)
  - Pie chart visualization of entity distribution
  - Top 10 keystones (most important entities by PageRank)

Examples:
  cx guide overview               # Stats and diagram
  cx guide overview --format d2   # Use D2 format
  cx guide overview -o arch.md    # Write to file`,
	RunE: runGuideOverview,
}

var guideHotspotsCmd = &cobra.Command{
	Use:   "hotspots",
	Short: "Show important entities: keystones and bottlenecks",
	Long: `Display the most important entities in your codebase.

Hotspots include:
  - Keystones: High PageRank entities (central to codebase)
  - Bottlenecks: High betweenness entities (many paths flow through)
  - High in-degree: Entities with many dependents
  - High out-degree: Entities with many dependencies

Examples:
  cx guide hotspots               # Show all hotspots
  cx guide hotspots --max-nodes 20`,
	RunE: runGuideHotspots,
}

var guideModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "Show module breakdown with dependency diagrams",
	Long: `Analyze and visualize the module structure of your codebase.

This shows:
  - Entities grouped by directory/package
  - Inter-module dependency diagram
  - Module-level statistics

Examples:
  cx guide modules                # Module breakdown
  cx guide modules --format d2    # Use D2 format
  cx guide modules -o modules.md`,
	RunE: runGuideModules,
}

var guideDepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Analyze dependencies and detect cycles",
	Long: `Analyze the dependency structure of your codebase.

This shows:
  - Circular dependency detection
  - Inter-module coupling analysis
  - Dependency layer visualization

Examples:
  cx guide deps                   # Dependency analysis
  cx guide deps --format mermaid  # Output as Mermaid`,
	RunE: runGuideDeps,
}

// Common flags
var (
	guideFormat   string // "mermaid" or "d2"
	guideOutput   string // output file path
	guideMaxNodes int    // max nodes before collapsing
)

func init() {
	rootCmd.AddCommand(guideCmd)

	// Add subcommands
	guideCmd.AddCommand(guideOverviewCmd)
	guideCmd.AddCommand(guideHotspotsCmd)
	guideCmd.AddCommand(guideModulesCmd)
	guideCmd.AddCommand(guideDepsCmd)

	// Add common flags to parent command (inherited by subcommands)
	guideCmd.PersistentFlags().StringVar(&guideFormat, "format", "mermaid", "Diagram format: mermaid|d2")
	guideCmd.PersistentFlags().StringVarP(&guideOutput, "output", "o", "", "Write output to file")
	guideCmd.PersistentFlags().IntVar(&guideMaxNodes, "max-nodes", 30, "Maximum nodes before auto-collapsing")
}

// runGuideOverview shows stats + architecture diagram
func runGuideOverview(cmd *cobra.Command, args []string) error {
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

	// Get entity counts by type
	stats, err := getEntityTypeCounts(storeDB)
	if err != nil {
		return fmt.Errorf("failed to get entity stats: %w", err)
	}

	// Get top 10 keystones
	keystones, err := storeDB.GetTopByPageRank(10)
	if err != nil {
		return fmt.Errorf("failed to get keystones: %w", err)
	}

	// Build output
	var sb strings.Builder

	sb.WriteString("# Codebase Overview\n\n")

	// Summary stats
	sb.WriteString("## Summary\n\n")
	total := 0
	for _, count := range stats {
		total += count
	}
	sb.WriteString(fmt.Sprintf("**Total Entities:** %d\n\n", total))

	// Entity distribution pie chart
	sb.WriteString("## Entity Distribution\n\n")
	sb.WriteString("```")
	sb.WriteString(guideFormat)
	sb.WriteString("\n")
	pieChart := graph.GeneratePieChart(stats, "Entity Distribution")
	sb.WriteString(pieChart)
	sb.WriteString("```\n\n")

	// Top keystones table
	if len(keystones) > 0 {
		sb.WriteString("## Top Keystones\n\n")
		sb.WriteString("| Entity | Type | Location | PageRank | In-Degree |\n")
		sb.WriteString("|--------|------|----------|----------|----------|\n")

		for _, m := range keystones {
			entity, err := storeDB.GetEntity(m.EntityID)
			if err != nil {
				continue
			}
			entityType := mapStoreEntityTypeToString(entity.EntityType)
			location := formatStoreLocation(entity)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.4f | %d |\n",
				entity.Name, entityType, location, m.PageRank, m.InDegree))
		}
		sb.WriteString("\n")
	}

	// Output result
	return writeGuideOutput(cmd, sb.String())
}

// runGuideHotspots shows keystones, bottlenecks, and high-degree entities
func runGuideHotspots(cmd *cobra.Command, args []string) error {
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

	limit := guideMaxNodes
	if limit <= 0 {
		limit = 30
	}

	var sb strings.Builder

	sb.WriteString("# Codebase Hotspots\n\n")

	// Top by PageRank (keystones)
	keystones, err := storeDB.GetTopByPageRank(limit)
	if err == nil && len(keystones) > 0 {
		sb.WriteString("## Keystones (by PageRank)\n\n")
		sb.WriteString("High PageRank entities are central to the codebase.\n\n")
		sb.WriteString("| Entity | Type | Location | PageRank |\n")
		sb.WriteString("|--------|------|----------|----------|\n")

		for _, m := range keystones {
			entity, err := storeDB.GetEntity(m.EntityID)
			if err != nil {
				continue
			}
			entityType := mapStoreEntityTypeToString(entity.EntityType)
			location := formatStoreLocation(entity)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.4f |\n",
				entity.Name, entityType, location, m.PageRank))
		}
		sb.WriteString("\n")
	}

	// Top by In-Degree (most depended upon)
	highInDegree, err := storeDB.GetTopByInDegree(limit)
	if err == nil && len(highInDegree) > 0 {
		sb.WriteString("## Most Depended Upon (by In-Degree)\n\n")
		sb.WriteString("Entities that many other entities depend on.\n\n")
		sb.WriteString("| Entity | Type | Location | In-Degree |\n")
		sb.WriteString("|--------|------|----------|-----------|\n")

		for _, m := range highInDegree {
			entity, err := storeDB.GetEntity(m.EntityID)
			if err != nil {
				continue
			}
			entityType := mapStoreEntityTypeToString(entity.EntityType)
			location := formatStoreLocation(entity)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d |\n",
				entity.Name, entityType, location, m.InDegree))
		}
		sb.WriteString("\n")
	}

	// Top by Out-Degree (most dependencies)
	highOutDegree, err := storeDB.GetTopByOutDegree(limit)
	if err == nil && len(highOutDegree) > 0 {
		sb.WriteString("## Most Dependencies (by Out-Degree)\n\n")
		sb.WriteString("Entities that depend on many other entities.\n\n")
		sb.WriteString("| Entity | Type | Location | Out-Degree |\n")
		sb.WriteString("|--------|------|----------|------------|\n")

		for _, m := range highOutDegree {
			entity, err := storeDB.GetEntity(m.EntityID)
			if err != nil {
				continue
			}
			entityType := mapStoreEntityTypeToString(entity.EntityType)
			location := formatStoreLocation(entity)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d |\n",
				entity.Name, entityType, location, m.OutDegree))
		}
		sb.WriteString("\n")
	}

	// Top by Betweenness (bottlenecks)
	bottlenecks, err := storeDB.GetTopByBetweenness(limit)
	if err == nil && len(bottlenecks) > 0 {
		sb.WriteString("## Bottlenecks (by Betweenness)\n\n")
		sb.WriteString("Entities that many paths flow through.\n\n")
		sb.WriteString("| Entity | Type | Location | Betweenness |\n")
		sb.WriteString("|--------|------|----------|-------------|\n")

		for _, m := range bottlenecks {
			if m.Betweenness <= 0 {
				continue
			}
			entity, err := storeDB.GetEntity(m.EntityID)
			if err != nil {
				continue
			}
			entityType := mapStoreEntityTypeToString(entity.EntityType)
			location := formatStoreLocation(entity)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.4f |\n",
				entity.Name, entityType, location, m.Betweenness))
		}
		sb.WriteString("\n")
	}

	return writeGuideOutput(cmd, sb.String())
}

// runGuideModules shows module breakdown with diagrams
func runGuideModules(cmd *cobra.Command, args []string) error {
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

	// Get all entities
	entities, err := storeDB.QueryEntities(store.EntityFilter{
		Status: "active",
		Limit:  100000,
	})
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	// Group entities by module (directory)
	modules := make(map[string][]*store.Entity)
	for _, e := range entities {
		module := extractModuleFromPath(e.FilePath)
		modules[module] = append(modules[module], e)
	}

	// Build the graph for module-level dependencies
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Compute module-level dependencies
	moduleEdges := computeModuleEdges(entities, g, storeDB)

	var sb strings.Builder

	sb.WriteString("# Module Breakdown\n\n")

	// Module statistics
	sb.WriteString("## Module Statistics\n\n")
	sb.WriteString("| Module | Entities | Functions | Types | Methods |\n")
	sb.WriteString("|--------|----------|-----------|-------|----------|\n")

	// Sort modules for deterministic output
	sortedModules := make([]string, 0, len(modules))
	for m := range modules {
		sortedModules = append(sortedModules, m)
	}
	sort.Strings(sortedModules)

	for _, module := range sortedModules {
		moduleEntities := modules[module]
		funcs, types, methods := 0, 0, 0
		for _, e := range moduleEntities {
			switch e.EntityType {
			case "function":
				funcs++
			case "type":
				types++
			case "method":
				methods++
			}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d |\n",
			module, len(moduleEntities), funcs, types, methods))
	}
	sb.WriteString("\n")

	// Module dependency diagram
	if len(moduleEdges) > 0 {
		sb.WriteString("## Module Dependencies\n\n")
		sb.WriteString("```")
		sb.WriteString(guideFormat)
		sb.WriteString("\n")

		if guideFormat == "d2" {
			sb.WriteString(generateModuleD2Diagram(sortedModules, moduleEdges))
		} else {
			sb.WriteString(generateModuleMermaidDiagram(sortedModules, moduleEdges))
		}

		sb.WriteString("```\n\n")
	}

	return writeGuideOutput(cmd, sb.String())
}

// runGuideDeps analyzes dependencies and detects cycles
func runGuideDeps(cmd *cobra.Command, args []string) error {
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

	var sb strings.Builder

	sb.WriteString("# Dependency Analysis\n\n")

	// Basic stats
	sb.WriteString("## Graph Statistics\n\n")
	sb.WriteString(fmt.Sprintf("- **Nodes:** %d\n", g.NodeCount()))
	sb.WriteString(fmt.Sprintf("- **Edges:** %d\n", g.EdgeCount()))
	sb.WriteString("\n")

	// Cycle detection
	hasCycles, cycle := g.FindCycles()
	sb.WriteString("## Circular Dependencies\n\n")

	if hasCycles {
		sb.WriteString("**Warning: Circular dependencies detected!**\n\n")
		sb.WriteString("Example cycle:\n\n")
		sb.WriteString("```\n")
		for i, node := range cycle {
			entity, err := storeDB.GetEntity(node)
			name := node
			if err == nil {
				name = entity.Name
			}
			if i < len(cycle)-1 {
				sb.WriteString(fmt.Sprintf("%s -> ", name))
			} else {
				sb.WriteString(name)
			}
		}
		sb.WriteString("\n```\n\n")

		// Visualize the cycle
		sb.WriteString("### Cycle Visualization\n\n")
		sb.WriteString("```")
		sb.WriteString(guideFormat)
		sb.WriteString("\n")

		if guideFormat == "d2" {
			sb.WriteString(generateCycleD2Diagram(cycle, storeDB))
		} else {
			sb.WriteString(generateCycleMermaidDiagram(cycle, storeDB))
		}

		sb.WriteString("```\n\n")
	} else {
		sb.WriteString("No circular dependencies detected. The dependency graph is acyclic.\n\n")
	}

	// Module coupling analysis
	entities, err := storeDB.QueryEntities(store.EntityFilter{
		Status: "active",
		Limit:  100000,
	})
	if err == nil {
		moduleEdges := computeModuleEdges(entities, g, storeDB)
		if len(moduleEdges) > 0 {
			sb.WriteString("## Module Coupling\n\n")
			sb.WriteString("Inter-module dependencies (number of cross-module calls):\n\n")
			sb.WriteString("| From Module | To Module | Connections |\n")
			sb.WriteString("|-------------|-----------|-------------|\n")

			// Count and sort edges by frequency
			type edgeCount struct {
				from  string
				to    string
				count int
			}
			counts := make(map[string]*edgeCount)
			for _, edge := range moduleEdges {
				key := edge[0] + "->" + edge[1]
				if ec, ok := counts[key]; ok {
					ec.count++
				} else {
					counts[key] = &edgeCount{from: edge[0], to: edge[1], count: 1}
				}
			}

			// Sort by count descending
			sortedCounts := make([]*edgeCount, 0, len(counts))
			for _, ec := range counts {
				sortedCounts = append(sortedCounts, ec)
			}
			sort.Slice(sortedCounts, func(i, j int) bool {
				return sortedCounts[i].count > sortedCounts[j].count
			})

			// Output top connections
			shown := 0
			for _, ec := range sortedCounts {
				if shown >= guideMaxNodes {
					break
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", ec.from, ec.to, ec.count))
				shown++
			}
			sb.WriteString("\n")
		}
	}

	return writeGuideOutput(cmd, sb.String())
}

// Helper functions

// getEntityTypeCounts returns counts of entities by type
func getEntityTypeCounts(storeDB *store.Store) (map[string]int, error) {
	stats := make(map[string]int)

	entityTypes := []string{"function", "type", "method", "constant", "variable", "import"}
	for _, et := range entityTypes {
		count, err := storeDB.CountEntities(store.EntityFilter{
			EntityType: et,
			Status:     "active",
		})
		if err != nil {
			return nil, err
		}
		if count > 0 {
			// Capitalize for display
			displayName := strings.Title(et) + "s"
			if et == "type" {
				displayName = "Types"
			}
			stats[displayName] = count
		}
	}

	return stats, nil
}

// extractModuleFromPath extracts the module/package path from a file path
func extractModuleFromPath(filePath string) string {
	// Remove filename, keep directory
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash > 0 {
		return filePath[:lastSlash]
	}
	return "root"
}

// computeModuleEdges computes inter-module dependencies
func computeModuleEdges(entities []*store.Entity, g *graph.Graph, storeDB *store.Store) [][]string {
	// Build entity ID to module map
	entityToModule := make(map[string]string)
	for _, e := range entities {
		entityToModule[e.ID] = extractModuleFromPath(e.FilePath)
	}

	// Find module-level edges
	var moduleEdges [][]string
	seenEdges := make(map[string]bool)

	for _, e := range entities {
		fromModule := entityToModule[e.ID]
		successors := g.Successors(e.ID)
		for _, targetID := range successors {
			toModule, ok := entityToModule[targetID]
			if !ok {
				// Try to get from store
				target, err := storeDB.GetEntity(targetID)
				if err != nil {
					continue
				}
				toModule = extractModuleFromPath(target.FilePath)
			}

			if fromModule != toModule {
				edgeKey := fromModule + "->" + toModule
				if !seenEdges[edgeKey] {
					seenEdges[edgeKey] = true
					moduleEdges = append(moduleEdges, []string{fromModule, toModule})
				}
			}
		}
	}

	return moduleEdges
}

// generateModuleMermaidDiagram generates a Mermaid flowchart for module dependencies
func generateModuleMermaidDiagram(modules []string, edges [][]string) string {
	var sb strings.Builder
	sb.WriteString("flowchart LR\n")

	// Add nodes
	for _, m := range modules {
		safeID := sanitizeMermaidModuleID(m)
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, m))
	}

	// Add edges (deduplicated)
	seen := make(map[string]bool)
	for _, edge := range edges {
		key := edge[0] + "->" + edge[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		fromID := sanitizeMermaidModuleID(edge[0])
		toID := sanitizeMermaidModuleID(edge[1])
		sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
	}

	return sb.String()
}

// generateModuleD2Diagram generates a D2 diagram for module dependencies
func generateModuleD2Diagram(modules []string, edges [][]string) string {
	var sb strings.Builder
	sb.WriteString("direction: right\n\n")

	// Add nodes
	for _, m := range modules {
		safeID := sanitizeD2ModuleID(m)
		sb.WriteString(fmt.Sprintf("%s: {\n  label: \"%s\"\n  shape: rectangle\n}\n", safeID, m))
	}

	sb.WriteString("\n")

	// Add edges (deduplicated)
	seen := make(map[string]bool)
	for _, edge := range edges {
		key := edge[0] + "->" + edge[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		fromID := sanitizeD2ModuleID(edge[0])
		toID := sanitizeD2ModuleID(edge[1])
		sb.WriteString(fmt.Sprintf("%s -> %s\n", fromID, toID))
	}

	return sb.String()
}

// generateCycleMermaidDiagram generates a Mermaid diagram for a cycle
func generateCycleMermaidDiagram(cycle []string, storeDB *store.Store) string {
	var sb strings.Builder
	sb.WriteString("flowchart LR\n")

	// Add nodes and edges in the cycle
	for i := 0; i < len(cycle)-1; i++ {
		fromID := cycle[i]
		toID := cycle[i+1]

		fromName := getEntityName(fromID, storeDB)
		toName := getEntityName(toID, storeDB)

		fromSafe := sanitizeMermaidModuleID(fromID)
		toSafe := sanitizeMermaidModuleID(toID)

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", fromSafe, fromName))
		if i == len(cycle)-2 {
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", toSafe, toName))
		}
		sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromSafe, toSafe))
	}

	return sb.String()
}

// generateCycleD2Diagram generates a D2 diagram for a cycle
func generateCycleD2Diagram(cycle []string, storeDB *store.Store) string {
	var sb strings.Builder
	sb.WriteString("direction: right\n\n")

	// Add nodes
	for _, node := range cycle[:len(cycle)-1] { // Exclude duplicate last node
		name := getEntityName(node, storeDB)
		safeID := sanitizeD2ModuleID(node)
		sb.WriteString(fmt.Sprintf("%s: {\n  label: \"%s\"\n  shape: rectangle\n  style: {\n    fill: \"#ffcccc\"\n  }\n}\n", safeID, name))
	}

	sb.WriteString("\n")

	// Add edges
	for i := 0; i < len(cycle)-1; i++ {
		fromID := sanitizeD2ModuleID(cycle[i])
		toID := sanitizeD2ModuleID(cycle[i+1])
		sb.WriteString(fmt.Sprintf("%s -> %s\n", fromID, toID))
	}

	return sb.String()
}

// getEntityName gets the name of an entity by ID
func getEntityName(entityID string, storeDB *store.Store) string {
	entity, err := storeDB.GetEntity(entityID)
	if err != nil {
		return entityID
	}
	return entity.Name
}

// sanitizeMermaidModuleID makes a module path safe for Mermaid IDs
func sanitizeMermaidModuleID(id string) string {
	// Replace path separators and special chars with underscores
	safe := strings.ReplaceAll(id, "/", "_")
	safe = strings.ReplaceAll(safe, "-", "_")
	safe = strings.ReplaceAll(safe, ".", "_")
	safe = strings.ReplaceAll(safe, " ", "_")
	if len(safe) > 0 && safe[0] >= '0' && safe[0] <= '9' {
		safe = "_" + safe
	}
	return safe
}

// sanitizeD2ModuleID makes a module path safe for D2 IDs
func sanitizeD2ModuleID(id string) string {
	// D2 allows quoted IDs for special characters
	if strings.ContainsAny(id, "/-.") {
		escaped := strings.ReplaceAll(id, "\"", "\\\"")
		return fmt.Sprintf("\"%s\"", escaped)
	}
	return id
}

// writeGuideOutput writes the guide output to file or stdout
func writeGuideOutput(cmd *cobra.Command, content string) error {
	if guideOutput != "" {
		if err := os.WriteFile(guideOutput, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Guide written to %s\n", guideOutput)
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), content)
	return nil
}

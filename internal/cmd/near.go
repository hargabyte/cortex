package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// nearCmd represents the near command for neighborhood exploration
var nearCmd = &cobra.Command{
	Use:   "near <name-or-file:line>",
	Short: "Explore the neighborhood of a symbol",
	Long: `Intuitive command for exploring code neighborhoods.
Shows what's around an entity - calls, callers, same-file entities, and type relationships.

Accepts entity identifiers:
  - Simple names: LoginUser (exact match preferred, then prefix)
  - Qualified names: auth.LoginUser (package.symbol)
  - Path-qualified: auth/login.LoginUser (path/file.symbol)
  - Direct IDs: sa-fn-a7f9b2-LoginUser
  - File:line: internal/auth/login.go:45 (find entity at line)
  - File only: internal/auth/login.go (show all entities + connections)

Output is organized by relationship type:
  - calls: What this entity calls (outgoing)
  - called_by: What calls this entity (incoming)
  - same_file: Other entities in the same file
  - uses_types: Types this entity uses
  - used_by_types: Entities that use this type (for type entities)

Key Differences from cx graph:
  - cx graph: Raw dependency visualization, edge-focused
  - cx near: Organized by relationship type, human-friendly groupings

Density Levels:
  sparse:  Type and location only
  medium:  Add signature (default)
  dense:   Full details

Direction Filtering:
  both:    Show incoming and outgoing (default)
  in:      Only show callers
  out:     Only show callees

Examples:
  cx near LoginUser                    # Show everything related to LoginUser
  cx near internal/auth/login.go:45    # Find entity at line 45, show neighborhood
  cx near internal/auth/login.go       # Show all entities in file + connections
  cx near LoginUser --depth 2          # Two hops out
  cx near LoginUser --direction in     # Only show callers
  cx near LoginUser --density dense    # Full details for all neighbors`,
	Args: cobra.ExactArgs(1),
	RunE: runNear,
}

var (
	nearDepth     int
	nearDirection string
)

func init() {
	rootCmd.AddCommand(nearCmd)

	// Near-specific flags
	nearCmd.Flags().IntVar(&nearDepth, "depth", 1, "Hop count for traversal (default: 1)")
	nearCmd.Flags().StringVar(&nearDirection, "direction", "both", "Filter direction: in|out|both (default: both)")
}

func runNear(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Validate direction flag
	if nearDirection != "in" && nearDirection != "out" && nearDirection != "both" {
		return fmt.Errorf("invalid direction %q: must be one of in, out, both", nearDirection)
	}

	// Validate depth
	if nearDepth < 1 {
		return fmt.Errorf("depth must be at least 1")
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

	// Check if query is a file path without line number (file overview mode)
	if isFilePath(query) && !strings.Contains(query, ":") {
		nearOutput, err := buildFileOverview(query, storeDB, density)
		if err != nil {
			return err
		}
		formatter, err := output.GetFormatter(format)
		if err != nil {
			return fmt.Errorf("failed to get formatter: %w", err)
		}
		return formatter.FormatToWriter(cmd.OutOrStdout(), nearOutput, density)
	}

	// Resolve the entity - support name, ID, or file:line
	entity, err := resolveNearQuery(query, storeDB)
	if err != nil {
		return err
	}

	// Build neighborhood output
	nearOutput, err := buildNeighborhood(entity, storeDB, nearDepth, nearDirection, density)
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

// resolveNearQuery resolves the query to an entity.
// Supports: name, ID, file:line, or just file (returns first entity in file).
func resolveNearQuery(query string, storeDB *store.Store) (*store.Entity, error) {
	// Check if it's a file:line pattern (e.g., internal/auth/login.go:45)
	if strings.Contains(query, ":") && !isDirectIDQuery(query) {
		parts := strings.Split(query, ":")
		if len(parts) == 2 {
			filePath := parts[0]
			lineStr := parts[1]

			// Check if second part is a number (file:line) or text (qualified name)
			if lineNum, err := strconv.Atoi(lineStr); err == nil {
				return resolveEntityAtLine(filePath, lineNum, storeDB)
			}
		}
	}

	// Check if it's just a file path (ends with .go, .ts, .py, .rs, .java)
	if isFilePath(query) && !strings.Contains(query, ":") {
		return resolveFirstEntityInFile(query, storeDB)
	}

	// Fall back to standard name/ID resolution
	return resolveEntityByName(query, storeDB, "")
}

// resolveEntityAtLine finds the entity at a specific line in a file
func resolveEntityAtLine(filePath string, lineNum int, storeDB *store.Store) (*store.Entity, error) {
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

// resolveFirstEntityInFile returns the first entity in a file
func resolveFirstEntityInFile(filePath string, storeDB *store.Store) (*store.Entity, error) {
	entities, err := storeDB.QueryEntities(store.EntityFilter{
		FilePath: filePath,
		Status:   "active",
		Limit:    1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities found in file %q", filePath)
	}

	return entities[0], nil
}

// buildNeighborhood constructs the neighborhood output for an entity
func buildNeighborhood(entity *store.Entity, storeDB *store.Store, depth int, direction string, density output.Density) (*output.NearOutput, error) {
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
					neighbor := buildNeighborEntity(targetEntity, d, density)

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
					neighbor := buildNeighborEntity(sourceEntity, d, density)

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

			neighbor := buildNeighborEntity(e, 0, density)
			neighborhood.SameFile = append(neighborhood.SameFile, neighbor)
		}
	}

	return &output.NearOutput{
		Center:       center,
		Neighborhood: neighborhood,
	}, nil
}

// buildNeighborEntity creates a NeighborEntity from a store.Entity
func buildNeighborEntity(e *store.Entity, depth int, density output.Density) *output.NeighborEntity {
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

// buildFileOverview creates a NearOutput showing all entities in a file
func buildFileOverview(filePath string, storeDB *store.Store, density output.Density) (*output.NearOutput, error) {
	// Query all entities in the file
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

	// Build center as the file itself
	center := &output.NearCenterEntity{
		Name:     filePath,
		Type:     "file",
		Location: filePath,
	}

	// Build neighborhood with all entities in the file
	neighborhood := &output.Neighborhood{}

	for _, e := range entities {
		neighbor := buildNeighborEntity(e, 0, density)
		neighborhood.SameFile = append(neighborhood.SameFile, neighbor)
	}

	return &output.NearOutput{
		Center:       center,
		Neighborhood: neighborhood,
	}, nil
}

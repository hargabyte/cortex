package graph

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/store"
)

// DiagramPreset represents a pre-configured diagram style.
type DiagramPreset string

const (
	PresetArchitecture DiagramPreset = "architecture" // Module containers with entities
	PresetCallFlow     DiagramPreset = "call_flow"    // Sequential function call flow
	PresetDependency   DiagramPreset = "dependency"   // Entity dependency graph
	PresetCoverage     DiagramPreset = "coverage"     // Coverage heatmap overlay
)

// ArchitecturePreset returns a DiagramConfig optimized for architecture diagrams.
// Architecture diagrams show modules as containers with top entities inside,
// and inter-module relationships as edges.
//
// Layout preference: TALA (superior container layout) > ELK > Dagre
func ArchitecturePreset() *DiagramConfig {
	return &DiagramConfig{
		Type:       DiagramArchitecture,
		Theme:      "default",
		Layout:     "elk", // ELK handles containers well and is bundled with D2
		Direction:  "right",
		MaxNodes:   50,     // Architecture diagrams can be larger
		Collapse:   true,   // Auto-collapse dense modules
		ShowLabels: true,   // Show edge labels for dependency types
		ShowIcons:  true,   // Show entity type icons
		Title:      "",     // Set by caller
	}
}

// CallFlowPreset returns a DiagramConfig optimized for call flow diagrams.
// Call flow diagrams show sequential function call relationships.
func CallFlowPreset() *DiagramConfig {
	return &DiagramConfig{
		Type:       DiagramCallFlow,
		Theme:      "default",
		Layout:     "elk", // ELK handles linear flow well
		Direction:  "down",
		MaxNodes:   30,
		Collapse:   false,  // Show full flow
		ShowLabels: true,   // Show call labels
		ShowIcons:  false,  // Simpler visual for flow
		Title:      "",
	}
}

// CoveragePreset returns a DiagramConfig optimized for coverage visualization.
// Coverage diagrams show test coverage as a heatmap overlay.
func CoveragePreset() *DiagramConfig {
	return &DiagramConfig{
		Type:       DiagramCoverage,
		Theme:      "default",
		Layout:     "elk",
		Direction:  "right",
		MaxNodes:   40,
		Collapse:   true,
		ShowLabels: false, // Cleaner for coverage view
		ShowIcons:  true,  // Show status icons (warning, success)
		Title:      "",
	}
}

// DependencyPreset returns a DiagramConfig optimized for dependency diagrams.
// Dependency diagrams show entity relationships without module grouping.
func DependencyPreset() *DiagramConfig {
	return &DiagramConfig{
		Type:       DiagramDeps,
		Theme:      "default",
		Layout:     "elk",
		Direction:  "right",
		MaxNodes:   30,
		Collapse:   true,
		ShowLabels: true,
		ShowIcons:  true,
		Title:      "",
	}
}

// GetPreset returns the DiagramConfig for a named preset.
func GetPreset(preset DiagramPreset) *DiagramConfig {
	switch preset {
	case PresetArchitecture:
		return ArchitecturePreset()
	case PresetCallFlow:
		return CallFlowPreset()
	case PresetCoverage:
		return CoveragePreset()
	case PresetDependency:
		return DependencyPreset()
	default:
		return DefaultDiagramConfig()
	}
}

// EntityToDiagramEntity converts a store.Entity to a DiagramEntity.
// It extracts the module from the file path and determines importance from metrics.
func EntityToDiagramEntity(e *store.Entity, metrics *store.Metrics, coverage float64) DiagramEntity {
	// Determine module from file path
	module := extractModuleFromPath(e.FilePath)

	// Determine importance
	importance := "normal"
	if metrics != nil {
		importance = classifyImportanceFromMetrics(metrics)
	}

	// Determine layer from entity type and module name
	layer := inferLayer(e.EntityType, module)

	return DiagramEntity{
		ID:         e.ID,
		Name:       e.Name,
		Type:       e.EntityType,
		Importance: importance,
		Coverage:   coverage,
		Language:   inferLanguage(e.FilePath),
		Module:     module,
		Layer:      layer,
	}
}

// DependencyToDiagramEdge converts a store.Dependency to a DiagramEdge.
func DependencyToDiagramEdge(dep *store.Dependency) DiagramEdge {
	return DiagramEdge{
		From:  dep.FromID,
		To:    dep.ToID,
		Type:  dep.DepType,
		Label: "", // Optional: could add dep.DepType as label
	}
}

// BuildArchitectureDiagram creates a D2 architecture diagram from store data.
// It queries the store for entities and dependencies, then generates D2 code.
// The theme parameter is optional - pass empty string for default theme.
func BuildArchitectureDiagram(s *store.Store, title string, maxEntities int, theme ...string) (string, error) {
	config := ArchitecturePreset()
	config.Title = title
	if maxEntities > 0 {
		config.MaxNodes = maxEntities
	}
	if len(theme) > 0 && theme[0] != "" {
		config.Theme = theme[0]
	}

	// Query top entities by PageRank
	topMetrics, err := s.GetTopByPageRank(config.MaxNodes)
	if err != nil {
		return "", err
	}

	// Build entity map and list
	entities := make([]DiagramEntity, 0, len(topMetrics))
	entityIDs := make(map[string]bool)

	for _, m := range topMetrics {
		entity, err := s.GetEntity(m.EntityID)
		if err != nil {
			continue
		}

		// Get coverage (optional)
		coverage := -1.0 // Unknown

		diagramEntity := DiagramEntity{
			ID:         entity.ID,
			Name:       entity.Name,
			Type:       entity.EntityType,
			Importance: classifyImportanceFromMetrics(m),
			Coverage:   coverage,
			Language:   inferLanguage(entity.FilePath),
			Module:     extractModuleFromPath(entity.FilePath),
			Layer:      inferLayer(entity.EntityType, extractModuleFromPath(entity.FilePath)),
		}

		entities = append(entities, diagramEntity)
		entityIDs[entity.ID] = true
	}

	// Get dependencies between these entities
	deps := make([]DiagramEdge, 0)
	for _, entity := range entities {
		fromDeps, err := s.GetDependenciesFrom(entity.ID)
		if err != nil {
			continue
		}

		for _, dep := range fromDeps {
			// Only include edges where both ends are in our entity set
			if entityIDs[dep.ToID] {
				deps = append(deps, DiagramEdge{
					From:  dep.FromID,
					To:    dep.ToID,
					Type:  dep.DepType,
					Label: "",
				})
			}
		}
	}

	// Generate D2 code
	gen := NewD2Generator(config)
	return gen.Generate(entities, deps), nil
}

// BuildModuleArchitectureDiagram creates a D2 architecture diagram focused on modules.
// It collapses entities into their modules and shows inter-module relationships.
func BuildModuleArchitectureDiagram(s *store.Store, title string) (string, error) {
	config := ArchitecturePreset()
	config.Title = title

	// Query all active entities
	allEntities, err := s.QueryEntities(store.EntityFilter{Status: "active", Limit: 5000})
	if err != nil {
		return "", err
	}

	// Group by module and select top entities per module
	moduleEntities := make(map[string][]*store.Entity)
	for _, e := range allEntities {
		module := extractModuleFromPath(e.FilePath)
		moduleEntities[module] = append(moduleEntities[module], e)
	}

	// Build diagram entities - top 3 entities per module by PageRank
	entities := make([]DiagramEntity, 0)
	entityIDs := make(map[string]bool)

	moduleNames := make([]string, 0, len(moduleEntities))
	for name := range moduleEntities {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	for _, module := range moduleNames {
		moduleEnts := moduleEntities[module]

		// Get metrics for sorting
		type entityWithRank struct {
			entity   *store.Entity
			pageRank float64
		}
		rankedEntities := make([]entityWithRank, 0, len(moduleEnts))

		for _, e := range moduleEnts {
			metrics, err := s.GetMetrics(e.ID)
			rank := 0.0
			if err == nil && metrics != nil {
				rank = metrics.PageRank
			}
			rankedEntities = append(rankedEntities, entityWithRank{entity: e, pageRank: rank})
		}

		// Sort by PageRank descending
		sort.Slice(rankedEntities, func(i, j int) bool {
			return rankedEntities[i].pageRank > rankedEntities[j].pageRank
		})

		// Take top 5 per module
		limit := 5
		if len(rankedEntities) < limit {
			limit = len(rankedEntities)
		}

		for i := 0; i < limit; i++ {
			e := rankedEntities[i].entity
			entities = append(entities, DiagramEntity{
				ID:         e.ID,
				Name:       e.Name,
				Type:       e.EntityType,
				Importance: classifyImportanceByRank(rankedEntities[i].pageRank),
				Coverage:   -1,
				Language:   inferLanguage(e.FilePath),
				Module:     module,
				Layer:      inferLayer(e.EntityType, module),
			})
			entityIDs[e.ID] = true
		}
	}

	// Get inter-module dependencies
	deps := make([]DiagramEdge, 0)
	for _, entity := range entities {
		fromDeps, err := s.GetDependenciesFrom(entity.ID)
		if err != nil {
			continue
		}

		for _, dep := range fromDeps {
			if entityIDs[dep.ToID] {
				deps = append(deps, DiagramEdge{
					From:  dep.FromID,
					To:    dep.ToID,
					Type:  dep.DepType,
					Label: "",
				})
			}
		}
	}

	gen := NewD2Generator(config)
	return gen.Generate(entities, deps), nil
}

// Helper functions

// extractModuleFromPath extracts the module/package path from a file path.
// Returns "_root" for files in the root directory.
func extractModuleFromPath(filePath string) string {
	dir := filepath.Dir(filePath)
	if dir == "" || dir == "." {
		return "_root"
	}
	return dir
}

// inferLanguage infers the programming language from a file path.
func inferLanguage(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".kt":
		return "kotlin"
	default:
		return ""
	}
}

// inferLayer infers the architectural layer from entity type and module name.
func inferLayer(entityType, module string) string {
	moduleLower := strings.ToLower(module)

	// Infer from module path
	if strings.Contains(moduleLower, "/api/") || strings.Contains(moduleLower, "/handler") ||
		strings.Contains(moduleLower, "/http") || strings.Contains(moduleLower, "/rest") {
		return "api"
	}
	if strings.Contains(moduleLower, "/store") || strings.Contains(moduleLower, "/db") ||
		strings.Contains(moduleLower, "/data") || strings.Contains(moduleLower, "/repo") {
		return "data"
	}
	if strings.Contains(moduleLower, "/model") || strings.Contains(moduleLower, "/domain") ||
		strings.Contains(moduleLower, "/entity") {
		return "domain"
	}
	if strings.Contains(moduleLower, "/service") || strings.Contains(moduleLower, "/pkg") {
		return "service"
	}

	// Infer from entity type
	switch entityType {
	case "http", "handler":
		return "api"
	case "database", "storage":
		return "data"
	case "struct", "type", "interface":
		return "domain"
	default:
		return "service"
	}
}

// classifyImportanceFromMetrics classifies importance from store.Metrics.
func classifyImportanceFromMetrics(m *store.Metrics) string {
	if m == nil {
		return "normal"
	}

	// Keystones: high PageRank
	if m.PageRank >= 0.01 {
		return "keystone"
	}
	// Bottlenecks: many callers
	if m.InDegree >= 10 {
		return "bottleneck"
	}
	// High fan-out
	if m.OutDegree >= 15 {
		return "high-fan-out"
	}
	// Leaves: no dependents
	if m.InDegree == 0 {
		return "leaf"
	}
	return "normal"
}

// classifyImportanceByRank classifies importance by PageRank value alone.
func classifyImportanceByRank(pageRank float64) string {
	if pageRank >= 0.01 {
		return "keystone"
	}
	if pageRank >= 0.005 {
		return "bottleneck"
	}
	if pageRank >= 0.001 {
		return "normal"
	}
	return "leaf"
}

// BuildCallFlowDiagram creates a D2 call flow diagram starting from a root entity.
// It performs BFS traversal following outgoing calls to the specified depth.
// The diagram shows the call chain with entities ordered top-to-bottom.
// The theme parameter is optional - pass empty string for default theme.
func BuildCallFlowDiagram(s *store.Store, rootEntityID string, depth int, title string, theme ...string) (string, error) {
	if depth <= 0 {
		depth = 3 // Default depth
	}
	if depth > 10 {
		depth = 10 // Cap depth to prevent excessive expansion
	}

	config := CallFlowPreset()
	config.Title = title
	if len(theme) > 0 && theme[0] != "" {
		config.Theme = theme[0]
	}

	// Get root entity
	rootEntity, err := s.GetEntity(rootEntityID)
	if err != nil {
		return "", err
	}

	// BFS traversal to collect entities and edges
	entities := make([]DiagramEntity, 0)
	edges := make([]DiagramEdge, 0)
	visited := make(map[string]bool)
	entityMap := make(map[string]*store.Entity)

	// Queue for BFS: (entityID, currentDepth)
	type queueItem struct {
		entityID string
		depth    int
	}
	queue := []queueItem{{entityID: rootEntityID, depth: 0}}
	visited[rootEntityID] = true
	entityMap[rootEntityID] = rootEntity

	for len(queue) > 0 && len(entities) < config.MaxNodes {
		item := queue[0]
		queue = queue[1:]

		// Stop expanding at max depth
		if item.depth >= depth {
			continue
		}

		// Get outgoing dependencies (calls)
		deps, err := s.GetDependenciesFrom(item.entityID)
		if err != nil {
			continue
		}

		for _, dep := range deps {
			// Only follow "calls" dependencies for call flow
			if dep.DepType != "calls" {
				continue
			}

			// Add edge
			edges = append(edges, DiagramEdge{
				From:  dep.FromID,
				To:    dep.ToID,
				Type:  dep.DepType,
				Label: "", // Could add call info here
			})

			// Add target to queue if not visited
			if !visited[dep.ToID] {
				visited[dep.ToID] = true

				targetEntity, err := s.GetEntity(dep.ToID)
				if err != nil {
					continue
				}
				entityMap[dep.ToID] = targetEntity

				queue = append(queue, queueItem{
					entityID: dep.ToID,
					depth:    item.depth + 1,
				})
			}
		}
	}

	// Convert collected entities to diagram entities
	for entityID, entity := range entityMap {
		metrics, _ := s.GetMetrics(entityID)

		importance := "normal"
		if metrics != nil {
			importance = classifyImportanceFromMetrics(metrics)
		}

		// Mark root entity as keystone for visual emphasis
		if entityID == rootEntityID {
			importance = "keystone"
		}

		entities = append(entities, DiagramEntity{
			ID:         entity.ID,
			Name:       entity.Name,
			Type:       entity.EntityType,
			Importance: importance,
			Coverage:   -1, // Not shown in call flow
			Language:   inferLanguage(entity.FilePath),
			Module:     extractModuleFromPath(entity.FilePath),
			Layer:      inferLayer(entity.EntityType, extractModuleFromPath(entity.FilePath)),
		})
	}

	// Sort entities to ensure root is first (for proper flow visualization)
	sort.Slice(entities, func(i, j int) bool {
		// Root entity comes first
		if entities[i].ID == rootEntityID {
			return true
		}
		if entities[j].ID == rootEntityID {
			return false
		}
		// Otherwise sort by ID for deterministic output
		return entities[i].ID < entities[j].ID
	})

	// Generate D2 code
	gen := NewD2Generator(config)
	return gen.Generate(entities, edges), nil
}

// BuildCallFlowDiagramFromName creates a call flow diagram by finding an entity by name.
// This is a convenience wrapper for cases where only the entity name is known.
func BuildCallFlowDiagramFromName(s *store.Store, entityName string, depth int, title string) (string, error) {
	// Search for the entity by name
	opts := store.DefaultSearchOptions()
	opts.Query = entityName
	opts.Limit = 1

	results, err := s.SearchEntities(opts)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		// Try exact match query
		entities, err := s.QueryEntities(store.EntityFilter{Name: entityName, Limit: 1})
		if err != nil || len(entities) == 0 {
			return "", fmt.Errorf("entity not found: %s", entityName)
		}
		return BuildCallFlowDiagram(s, entities[0].ID, depth, title)
	}

	return BuildCallFlowDiagram(s, results[0].Entity.ID, depth, title)
}

// BuildCallersFlowDiagram creates a diagram showing what calls a given entity.
// It traverses incoming dependencies (callers) instead of outgoing calls.
// The theme parameter is optional - pass empty string for default theme.
func BuildCallersFlowDiagram(s *store.Store, targetEntityID string, depth int, title string, theme ...string) (string, error) {
	if depth <= 0 {
		depth = 3
	}
	if depth > 10 {
		depth = 10
	}

	config := CallFlowPreset()
	config.Title = title
	config.Direction = "up" // Reverse direction for callers view
	if len(theme) > 0 && theme[0] != "" {
		config.Theme = theme[0]
	}

	// Get target entity
	targetEntity, err := s.GetEntity(targetEntityID)
	if err != nil {
		return "", err
	}

	// BFS traversal following callers (incoming dependencies)
	entities := make([]DiagramEntity, 0)
	edges := make([]DiagramEdge, 0)
	visited := make(map[string]bool)
	entityMap := make(map[string]*store.Entity)

	type queueItem struct {
		entityID string
		depth    int
	}
	queue := []queueItem{{entityID: targetEntityID, depth: 0}}
	visited[targetEntityID] = true
	entityMap[targetEntityID] = targetEntity

	for len(queue) > 0 && len(entities) < config.MaxNodes {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= depth {
			continue
		}

		// Get incoming dependencies (callers)
		deps, err := s.GetDependenciesTo(item.entityID)
		if err != nil {
			continue
		}

		for _, dep := range deps {
			if dep.DepType != "calls" {
				continue
			}

			// Add edge (caller -> current)
			edges = append(edges, DiagramEdge{
				From:  dep.FromID,
				To:    dep.ToID,
				Type:  dep.DepType,
				Label: "",
			})

			if !visited[dep.FromID] {
				visited[dep.FromID] = true

				callerEntity, err := s.GetEntity(dep.FromID)
				if err != nil {
					continue
				}
				entityMap[dep.FromID] = callerEntity

				queue = append(queue, queueItem{
					entityID: dep.FromID,
					depth:    item.depth + 1,
				})
			}
		}
	}

	// Convert to diagram entities
	for entityID, entity := range entityMap {
		metrics, _ := s.GetMetrics(entityID)

		importance := "normal"
		if metrics != nil {
			importance = classifyImportanceFromMetrics(metrics)
		}

		if entityID == targetEntityID {
			importance = "keystone"
		}

		entities = append(entities, DiagramEntity{
			ID:         entity.ID,
			Name:       entity.Name,
			Type:       entity.EntityType,
			Importance: importance,
			Coverage:   -1,
			Language:   inferLanguage(entity.FilePath),
			Module:     extractModuleFromPath(entity.FilePath),
			Layer:      inferLayer(entity.EntityType, extractModuleFromPath(entity.FilePath)),
		})
	}

	// Sort with target entity first
	sort.Slice(entities, func(i, j int) bool {
		if entities[i].ID == targetEntityID {
			return true
		}
		if entities[j].ID == targetEntityID {
			return false
		}
		return entities[i].ID < entities[j].ID
	})

	gen := NewD2Generator(config)
	return gen.Generate(entities, edges), nil
}

// ChangesDiagramPreset returns a DiagramConfig for change diagrams.
// Uses architecture layout with change state color coding.
func ChangesDiagramPreset() *DiagramConfig {
	return &DiagramConfig{
		Type:       DiagramArchitecture,
		Theme:      "default",
		Layout:     "elk",
		Direction:  "right",
		MaxNodes:   40,
		Collapse:   true,
		ShowLabels: true,
		ShowIcons:  true,
		Title:      "",
	}
}

// ChangedEntityInfo contains entity info with its change state.
type ChangedEntityInfo struct {
	ID          string
	Name        string
	Type        string
	FilePath    string
	ChangeState string // "added", "modified", "deleted"
}

// BuildChangesDiagram creates a D2 diagram showing changed entities with color coding.
// Green = added, Yellow = modified, Red = deleted.
// The diagram groups entities by module and highlights changes.
// The theme parameter is optional - pass empty string for default theme.
func BuildChangesDiagram(
	added []ChangedEntityInfo,
	modified []ChangedEntityInfo,
	deleted []ChangedEntityInfo,
	title string,
	theme ...string,
) string {
	config := ChangesDiagramPreset()
	config.Title = title
	if len(theme) > 0 && theme[0] != "" {
		config.Theme = theme[0]
	}

	// Calculate proportional limits for each change type to ensure representation
	totalChanges := len(added) + len(modified) + len(deleted)
	maxNodes := config.MaxNodes

	addedLimit := len(added)
	modifiedLimit := len(modified)
	deletedLimit := len(deleted)

	// If total exceeds max, allocate proportionally with minimum of 1 for non-empty categories
	if totalChanges > maxNodes && totalChanges > 0 {
		addedLimit = 0
		modifiedLimit = 0
		deletedLimit = 0

		if len(added) > 0 {
			addedLimit = maxNodes * len(added) / totalChanges
			if addedLimit == 0 {
				addedLimit = 1
			}
		}
		if len(modified) > 0 {
			modifiedLimit = maxNodes * len(modified) / totalChanges
			if modifiedLimit == 0 {
				modifiedLimit = 1
			}
		}
		if len(deleted) > 0 {
			deletedLimit = maxNodes * len(deleted) / totalChanges
			if deletedLimit == 0 {
				deletedLimit = 1
			}
		}
	}

	// Combine all changed entities
	entities := make([]DiagramEntity, 0)

	// Add "added" entities (green)
	for i, e := range added {
		if i >= addedLimit {
			break
		}
		entities = append(entities, DiagramEntity{
			ID:          e.ID,
			Name:        e.Name,
			Type:        e.Type,
			Importance:  "normal",
			Coverage:    -1,
			Language:    inferLanguage(e.FilePath),
			Module:      extractModuleFromPath(e.FilePath),
			Layer:       inferLayer(e.Type, extractModuleFromPath(e.FilePath)),
			ChangeState: "added",
		})
	}

	// Add "modified" entities (yellow)
	for i, e := range modified {
		if i >= modifiedLimit {
			break
		}
		entities = append(entities, DiagramEntity{
			ID:          e.ID,
			Name:        e.Name,
			Type:        e.Type,
			Importance:  "normal",
			Coverage:    -1,
			Language:    inferLanguage(e.FilePath),
			Module:      extractModuleFromPath(e.FilePath),
			Layer:       inferLayer(e.Type, extractModuleFromPath(e.FilePath)),
			ChangeState: "modified",
		})
	}

	// Add "deleted" entities (red)
	for i, e := range deleted {
		if i >= deletedLimit {
			break
		}
		entities = append(entities, DiagramEntity{
			ID:          e.ID,
			Name:        e.Name,
			Type:        e.Type,
			Importance:  "normal",
			Coverage:    -1,
			Language:    inferLanguage(e.FilePath),
			Module:      extractModuleFromPath(e.FilePath),
			Layer:       inferLayer(e.Type, extractModuleFromPath(e.FilePath)),
			ChangeState: "deleted",
		})
	}

	// No edges for changes diagram - focus on changed entities
	edges := make([]DiagramEdge, 0)

	gen := NewD2Generator(config)
	return gen.Generate(entities, edges)
}

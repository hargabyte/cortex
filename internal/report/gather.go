// Package report provides data gathering and schema types for CX report generation.
package report

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/store"
)

// DataGatherer gathers data from the store for report generation.
type DataGatherer struct {
	store          *store.Store
	theme          string // Optional theme for D2 diagrams
	playgroundMode bool   // Enable playground metadata generation
}

// NewDataGatherer creates a new DataGatherer with the given store.
func NewDataGatherer(s *store.Store) *DataGatherer {
	return &DataGatherer{store: s, theme: "", playgroundMode: false}
}

// SetTheme sets the diagram theme for report generation.
// Use empty string for default theme.
func (g *DataGatherer) SetTheme(theme string) {
	g.theme = theme
}

// SetPlaygroundMode enables or disables playground metadata generation.
// When enabled, reports include extra metadata for interactive playground features.
func (g *DataGatherer) SetPlaygroundMode(enabled bool) {
	g.playgroundMode = enabled
}

// GatherOverviewData populates an OverviewReportData with data from the store.
func (g *DataGatherer) GatherOverviewData(data *OverviewReportData) error {
	data.Report.GeneratedAt = time.Now().UTC()

	// Gather statistics
	if err := g.gatherStatistics(&data.Statistics, &data.Metadata); err != nil {
		return fmt.Errorf("gather statistics: %w", err)
	}

	// Gather keystones (top entities by PageRank)
	if err := g.gatherKeystones(&data.Keystones); err != nil {
		return fmt.Errorf("gather keystones: %w", err)
	}

	// Add layer and CSS classes to keystones if playground mode is enabled
	if g.playgroundMode {
		for i := range data.Keystones {
			g.enrichEntityForPlayground(&data.Keystones[i])
		}
	}

	// Gather module structure
	if err := g.gatherModules(&data.Modules); err != nil {
		return fmt.Errorf("gather modules: %w", err)
	}

	// Gather health summary
	health := &HealthSummary{}
	if err := g.gatherHealthSummary(health); err != nil {
		// Health data is optional, don't fail the whole report
		health = nil
	}
	data.Health = health

	// Gather playground metadata if enabled
	if g.playgroundMode {
		if err := g.gatherPlaygroundMetadata(data); err != nil {
			// Playground data is optional, log but don't fail
			data.Playground = &PlaygroundMetadata{Enabled: true}
		}
	}

	// Generate architecture diagram using preset
	if err := g.gatherArchitectureDiagram(data); err != nil {
		// Diagram is optional, don't fail the whole report
		// But ensure the map exists
		if data.Diagrams == nil {
			data.Diagrams = make(map[string]DiagramData)
		}
	}

	return nil
}

// GatherFeatureData populates a FeatureReportData with data from the store.
func (g *DataGatherer) GatherFeatureData(data *FeatureReportData, query string) error {
	data.Report.GeneratedAt = time.Now().UTC()
	data.Report.Query = query

	// Get total entity count for metadata
	totalCount, err := g.store.CountEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("count entities: %w", err)
	}
	data.Metadata.TotalEntitiesSearched = totalCount

	// Perform FTS search
	searchOpts := store.DefaultSearchOptions()
	searchOpts.Query = query
	searchOpts.Limit = 50 // Get more results for feature reports

	results, err := g.store.SearchEntities(searchOpts)
	if err != nil {
		return fmt.Errorf("search entities: %w", err)
	}

	data.Metadata.MatchesFound = len(results)
	data.Metadata.SearchMethod = "fts"

	// Convert search results to EntityData
	entityIDs := make([]string, 0, len(results))
	for _, result := range results {
		entityData := g.convertEntityToData(result.Entity)
		entityData.RelevanceScore = result.CombinedScore
		entityData.PageRank = result.PageRank

		// Get metrics for importance classification
		metrics, err := g.store.GetMetrics(result.Entity.ID)
		if err == nil {
			entityData.InDegree = metrics.InDegree
			entityData.Importance = classifyImportance(metrics.PageRank, metrics.InDegree)
		}

		// Get coverage if available
		cov, err := coverage.GetEntityCoverage(g.store, result.Entity.ID)
		if err == nil {
			entityData.Coverage = cov.CoveragePercent
		} else {
			entityData.Coverage = -1 // Not available
		}

		data.Entities = append(data.Entities, entityData)
		entityIDs = append(entityIDs, result.Entity.ID)
	}

	data.Metadata.EntityCount = len(data.Entities)

	// Gather dependencies between matched entities
	if err := g.gatherDependenciesBetween(&data.Dependencies, entityIDs); err != nil {
		return fmt.Errorf("gather dependencies: %w", err)
	}

	// Gather language breakdown
	data.Metadata.LanguageBreakdown = g.computeLanguageBreakdown(data.Entities)

	// Gather coverage data
	if err := g.gatherCoverageData(&data.Coverage, entityIDs); err != nil {
		// Coverage is optional
		data.Coverage = nil
	}
	data.Metadata.CoverageAvailable = data.Coverage != nil

	// Gather associated tests
	if err := g.gatherTestsForEntities(&data.Tests, entityIDs); err != nil {
		// Tests are optional
		data.Tests = nil
	}

	// Generate call flow diagram for the top search result
	if err := g.gatherCallFlowDiagram(data); err != nil {
		// Diagram is optional, don't fail the whole report
		if data.Diagrams == nil {
			data.Diagrams = make(map[string]DiagramData)
		}
	}

	// Add layer and CSS classes to entities if playground mode is enabled
	if g.playgroundMode {
		for i := range data.Entities {
			g.enrichEntityForPlayground(&data.Entities[i])
		}
		// Create playground metadata for feature reports
		if err := g.gatherFeaturePlaygroundMetadata(data); err != nil {
			// Playground data is optional
			data.Playground = &PlaygroundMetadata{Enabled: true}
		}
	}

	return nil
}

// GatherChangesData populates a ChangesReportData with data from Dolt time-travel.
func (g *DataGatherer) GatherChangesData(data *ChangesReportData, fromRef, toRef string) error {
	data.Report.GeneratedAt = time.Now().UTC()
	data.Report.FromRef = fromRef
	data.Report.ToRef = toRef

	// Validate refs
	if !store.IsValidRef(fromRef) {
		return fmt.Errorf("invalid fromRef: %s", fromRef)
	}
	if !store.IsValidRef(toRef) {
		return fmt.Errorf("invalid toRef: %s", toRef)
	}

	// Get entities at both points in time
	entitiesFrom, err := g.store.QueryEntitiesAt(store.EntityFilter{Status: "active"}, fromRef)
	if err != nil {
		return fmt.Errorf("query entities at %s: %w", fromRef, err)
	}

	entitiesTo, err := g.store.QueryEntitiesAt(store.EntityFilter{Status: "active"}, toRef)
	if err != nil {
		return fmt.Errorf("query entities at %s: %w", toRef, err)
	}

	// Build maps for comparison
	fromMap := make(map[string]*store.Entity)
	for _, e := range entitiesFrom {
		fromMap[e.ID] = e
	}

	toMap := make(map[string]*store.Entity)
	for _, e := range entitiesTo {
		toMap[e.ID] = e
	}

	// Find added, modified, and deleted entities
	for id, toEntity := range toMap {
		if fromEntity, exists := fromMap[id]; exists {
			// Entity exists in both - check if modified
			if g.entityModified(fromEntity, toEntity) {
				data.ModifiedEntities = append(data.ModifiedEntities, g.convertToChangedEntity(toEntity, "modified"))
			}
		} else {
			// Entity only in toRef - added
			data.AddedEntities = append(data.AddedEntities, g.convertToChangedEntity(toEntity, "added"))
		}
	}

	for id, fromEntity := range fromMap {
		if _, exists := toMap[id]; !exists {
			// Entity only in fromRef - deleted
			deleted := g.convertToChangedEntity(fromEntity, "deleted")
			deleted.WasFile = fromEntity.FilePath
			deleted.File = ""
			data.DeletedEntities = append(data.DeletedEntities, deleted)
		}
	}

	// Update statistics
	data.Statistics.Added = len(data.AddedEntities)
	data.Statistics.Modified = len(data.ModifiedEntities)
	data.Statistics.Deleted = len(data.DeletedEntities)

	// Update metadata
	data.Metadata.EntityCount = data.Statistics.Added + data.Statistics.Modified + data.Statistics.Deleted

	// Compute impact analysis for modified entities
	if err := g.computeImpactAnalysis(&data.Impact, data.ModifiedEntities, data.DeletedEntities); err != nil {
		// Impact analysis is optional
		data.Impact = nil
	}

	// Generate changes diagram showing added/modified/deleted entities
	if err := g.gatherChangesDiagram(data); err != nil {
		// Diagram generation is optional - don't fail the report
		_ = err
	}

	return nil
}

// gatherChangesDiagram generates a D2 diagram showing changed entities.
// Added entities are shown in green, modified in yellow, deleted in red.
func (g *DataGatherer) gatherChangesDiagram(data *ChangesReportData) error {
	// Skip if no changes
	if data.Statistics.Added == 0 && data.Statistics.Modified == 0 && data.Statistics.Deleted == 0 {
		return nil
	}

	if data.Diagrams == nil {
		data.Diagrams = make(map[string]DiagramData)
	}

	// Convert ChangedEntity to ChangedEntityInfo for diagram generation
	added := make([]graph.ChangedEntityInfo, 0, len(data.AddedEntities))
	for _, e := range data.AddedEntities {
		added = append(added, graph.ChangedEntityInfo{
			ID:          e.ID,
			Name:        e.Name,
			Type:        e.Type,
			FilePath:    e.File,
			ChangeState: "added",
		})
	}

	modified := make([]graph.ChangedEntityInfo, 0, len(data.ModifiedEntities))
	for _, e := range data.ModifiedEntities {
		modified = append(modified, graph.ChangedEntityInfo{
			ID:          e.ID,
			Name:        e.Name,
			Type:        e.Type,
			FilePath:    e.File,
			ChangeState: "modified",
		})
	}

	deleted := make([]graph.ChangedEntityInfo, 0, len(data.DeletedEntities))
	for _, e := range data.DeletedEntities {
		// For deleted entities, use WasFile if File is empty
		filePath := e.File
		if filePath == "" {
			filePath = e.WasFile
		}
		deleted = append(deleted, graph.ChangedEntityInfo{
			ID:          e.ID,
			Name:        e.Name,
			Type:        e.Type,
			FilePath:    filePath,
			ChangeState: "deleted",
		})
	}

	title := fmt.Sprintf("Changes: %s → %s", data.Report.FromRef, data.Report.ToRef)
	d2Code := graph.BuildChangesDiagram(added, modified, deleted, title, g.theme)

	data.Diagrams["changes_summary"] = DiagramData{
		Title: title,
		D2:    d2Code,
	}

	return nil
}

// GatherHealthData populates a HealthReportData with data from the store.
func (g *DataGatherer) GatherHealthData(data *HealthReportData) error {
	data.Report.GeneratedAt = time.Now().UTC()

	// Gather coverage data
	coverageData := &CoverageData{}
	if err := g.gatherOverallCoverage(coverageData); err == nil {
		data.Coverage = coverageData
	}

	// Find untested keystones (critical issues)
	if err := g.findUntestedKeystones(data); err != nil {
		// Continue even if this fails
	}

	// Find dead code candidates (info issues)
	if err := g.findDeadCodeCandidates(data); err != nil {
		// Continue even if this fails
	}

	// Find complexity hotspots
	complexity := &ComplexityAnalysis{}
	if err := g.findComplexityHotspots(complexity); err == nil && len(complexity.Hotspots) > 0 {
		data.Complexity = complexity
	}

	// Calculate risk score
	data.RiskScore = g.calculateRiskScore(data)

	return nil
}

// gatherStatistics collects entity statistics from the store.
func (g *DataGatherer) gatherStatistics(stats *StatisticsData, metadata *MetadataData) error {
	// Count total entities
	totalCount, err := g.store.CountEntities(store.EntityFilter{})
	if err != nil {
		return err
	}
	metadata.TotalEntities = totalCount

	// Count active entities
	activeCount, err := g.store.CountEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return err
	}
	metadata.ActiveEntities = activeCount
	metadata.ArchivedEntities = totalCount - activeCount

	// Count by type
	stats.ByType = make(map[string]int)
	entityTypes := []string{"function", "method", "type", "constant", "variable", "interface"}
	for _, et := range entityTypes {
		count, err := g.store.CountEntities(store.EntityFilter{EntityType: et, Status: "active"})
		if err == nil && count > 0 {
			stats.ByType[et] = count
		}
	}

	// Count by language
	stats.ByLanguage = make(map[string]int)
	languages := []string{"go", "typescript", "javascript", "python", "rust", "java"}
	for _, lang := range languages {
		count, err := g.store.CountEntities(store.EntityFilter{Language: lang, Status: "active"})
		if err == nil && count > 0 {
			stats.ByLanguage[lang] = count
		}
	}
	metadata.LanguageBreakdown = stats.ByLanguage

	return nil
}

// gatherKeystones collects top entities by PageRank.
func (g *DataGatherer) gatherKeystones(keystones *[]EntityData) error {
	// Get top 20 by PageRank
	metrics, err := g.store.GetTopByPageRank(20)
	if err != nil {
		return err
	}

	for _, m := range metrics {
		entity, err := g.store.GetEntity(m.EntityID)
		if err != nil {
			continue // Skip if entity not found
		}

		entityData := g.convertEntityToData(entity)
		entityData.PageRank = m.PageRank
		entityData.InDegree = m.InDegree
		entityData.Importance = classifyImportance(m.PageRank, m.InDegree)

		// Get coverage if available
		cov, err := coverage.GetEntityCoverage(g.store, entity.ID)
		if err == nil {
			entityData.Coverage = cov.CoveragePercent
		} else {
			entityData.Coverage = -1
		}

		*keystones = append(*keystones, entityData)
	}

	return nil
}

// gatherModules collects module/package structure from the store.
func (g *DataGatherer) gatherModules(modules *[]ModuleData) error {
	// Query all active entities
	entities, err := g.store.QueryEntities(store.EntityFilter{Status: "active", Limit: 10000})
	if err != nil {
		return err
	}

	// Group by directory
	moduleMap := make(map[string]*ModuleData)
	for _, e := range entities {
		dir := filepath.Dir(e.FilePath)
		if dir == "" || dir == "." {
			continue
		}

		if _, exists := moduleMap[dir]; !exists {
			moduleMap[dir] = &ModuleData{
				Path:      dir,
				Entities:  0,
				Functions: 0,
				Types:     0,
			}
		}

		mod := moduleMap[dir]
		mod.Entities++

		switch e.EntityType {
		case "function", "method":
			mod.Functions++
		case "type", "interface":
			mod.Types++
		}
	}

	// Convert map to sorted slice
	for _, mod := range moduleMap {
		*modules = append(*modules, *mod)
	}

	// Sort by entity count descending
	sort.Slice(*modules, func(i, j int) bool {
		return (*modules)[i].Entities > (*modules)[j].Entities
	})

	// Limit to top 20 modules
	if len(*modules) > 20 {
		*modules = (*modules)[:20]
	}

	return nil
}

// gatherHealthSummary collects health metrics.
func (g *DataGatherer) gatherHealthSummary(health *HealthSummary) error {
	// Get coverage stats
	stats, err := coverage.GetCoverageStats(g.store)
	if err == nil {
		if avgCov, ok := stats["average_coverage_percent"].(float64); ok {
			health.CoverageOverall = avgCov
		}
	}

	// Count untested keystones
	keystones, err := g.store.GetTopByPageRank(50)
	if err == nil {
		for _, k := range keystones {
			cov, err := coverage.GetEntityCoverage(g.store, k.EntityID)
			if err != nil || cov.CoveragePercent == 0 {
				health.UntestedKeystones++
			}
		}
	}

	return nil
}

// gatherArchitectureDiagram generates the architecture diagram for overview reports.
// It uses the ArchitecturePreset from the graph package to create a D2 diagram
// showing modules as containers with top entities inside and inter-module edges.
// When playground mode is enabled, generates multiple filtered diagrams for presets.
func (g *DataGatherer) gatherArchitectureDiagram(data *OverviewReportData) error {
	if data.Diagrams == nil {
		data.Diagrams = make(map[string]DiagramData)
	}

	// Always generate the full architecture diagram
	d2Code, err := graph.BuildArchitectureDiagram(g.store, "System Architecture", 50, g.theme)
	if err != nil {
		return fmt.Errorf("build architecture diagram: %w", err)
	}

	data.Diagrams["architecture"] = DiagramData{
		Title: "System Architecture",
		D2:    d2Code,
	}

	// In playground mode, generate additional filtered diagrams for presets
	if g.playgroundMode {
		presets := map[string][]string{
			"architecture_core":   {"core", "parser"},
			"architecture_store":  {"store", "core"},
			"architecture_parser": {"parser", "core"},
		}

		for key, layers := range presets {
			filteredD2, err := graph.BuildFilteredArchitectureDiagram(
				g.store,
				"System Architecture",
				50,
				layers,
				g.theme,
			)
			if err != nil {
				// Skip this preset if it fails, don't fail the whole report
				continue
			}
			data.Diagrams[key] = DiagramData{
				Title: "System Architecture",
				D2:    filteredD2,
			}
		}
	}

	return nil
}

// gatherCallFlowDiagram generates a call flow diagram for feature reports.
// It uses the top search result as the root entity and shows its call chain.
func (g *DataGatherer) gatherCallFlowDiagram(data *FeatureReportData) error {
	if len(data.Entities) == 0 {
		return nil // No entities to diagram
	}

	if data.Diagrams == nil {
		data.Diagrams = make(map[string]DiagramData)
	}

	// Use the top entity as the root for the call flow
	rootEntity := data.Entities[0]
	title := fmt.Sprintf("Call Flow: %s", rootEntity.Name)

	// Build call flow diagram with depth 3 and optional theme
	d2Code, err := graph.BuildCallFlowDiagram(g.store, rootEntity.ID, 3, title, g.theme)
	if err != nil {
		return fmt.Errorf("build call flow diagram: %w", err)
	}

	data.Diagrams["call_flow"] = DiagramData{
		Title: title,
		D2:    d2Code,
	}

	return nil
}

// gatherDependenciesBetween collects dependencies between a set of entity IDs.
func (g *DataGatherer) gatherDependenciesBetween(deps *[]DependencyData, entityIDs []string) error {
	// Create a set for quick lookup
	idSet := make(map[string]bool)
	for _, id := range entityIDs {
		idSet[id] = true
	}

	// Get all dependencies from these entities
	for _, fromID := range entityIDs {
		fromDeps, err := g.store.GetDependenciesFrom(fromID)
		if err != nil {
			continue
		}

		fromEntity, err := g.store.GetEntity(fromID)
		if err != nil {
			continue
		}

		for _, dep := range fromDeps {
			// Only include if target is also in our set
			if !idSet[dep.ToID] {
				continue
			}

			toEntity, err := g.store.GetEntity(dep.ToID)
			if err != nil {
				continue
			}

			*deps = append(*deps, DependencyData{
				From: fromEntity.Name,
				To:   toEntity.Name,
				Type: dep.DepType,
			})
		}
	}

	return nil
}

// gatherCoverageData collects coverage statistics for a set of entities.
func (g *DataGatherer) gatherCoverageData(coveragePtr **CoverageData, entityIDs []string) error {
	if len(entityIDs) == 0 {
		return nil
	}

	coverageData := &CoverageData{
		ByEntity:     make(map[string]float64),
		ByImportance: make(map[string]float64),
	}

	var totalCoverage float64
	var coveredCount int
	importanceCounts := make(map[string]int)
	importanceTotals := make(map[string]float64)

	for _, id := range entityIDs {
		cov, err := coverage.GetEntityCoverage(g.store, id)
		if err != nil {
			continue
		}

		entity, err := g.store.GetEntity(id)
		if err != nil {
			continue
		}

		coverageData.ByEntity[entity.Name] = cov.CoveragePercent
		totalCoverage += cov.CoveragePercent
		coveredCount++

		// Get importance for per-importance coverage
		metrics, err := g.store.GetMetrics(id)
		if err == nil {
			importance := classifyImportance(metrics.PageRank, metrics.InDegree)
			importanceCounts[string(importance)]++
			importanceTotals[string(importance)] += cov.CoveragePercent
		}
	}

	if coveredCount > 0 {
		coverageData.Overall = totalCoverage / float64(coveredCount)
	}

	// Calculate per-importance coverage
	for imp, count := range importanceCounts {
		if count > 0 {
			coverageData.ByImportance[imp] = importanceTotals[imp] / float64(count)
		}
	}

	*coveragePtr = coverageData
	return nil
}

// gatherTestsForEntities collects tests that cover the given entities.
func (g *DataGatherer) gatherTestsForEntities(tests *[]TestData, entityIDs []string) error {
	testMap := make(map[string]*TestData)

	for _, entityID := range entityIDs {
		testInfos, err := coverage.GetTestsForEntity(g.store, entityID)
		if err != nil {
			continue
		}

		entity, err := g.store.GetEntity(entityID)
		if err != nil {
			continue
		}

		for _, info := range testInfos {
			key := info.TestFile + ":" + info.TestName
			if _, exists := testMap[key]; !exists {
				testMap[key] = &TestData{
					Name:   info.TestName,
					File:   info.TestFile,
					Covers: []string{},
				}
			}
			testMap[key].Covers = append(testMap[key].Covers, entity.Name)
		}
	}

	for _, test := range testMap {
		*tests = append(*tests, *test)
	}

	return nil
}

// gatherOverallCoverage collects overall coverage statistics.
func (g *DataGatherer) gatherOverallCoverage(coverageData *CoverageData) error {
	stats, err := coverage.GetCoverageStats(g.store)
	if err != nil {
		return err
	}

	if avgCov, ok := stats["average_coverage_percent"].(float64); ok {
		coverageData.Overall = avgCov
	}

	return nil
}

// findUntestedKeystones finds critical entities without test coverage.
func (g *DataGatherer) findUntestedKeystones(data *HealthReportData) error {
	keystones, err := g.store.GetTopByPageRank(50)
	if err != nil {
		return err
	}

	for _, k := range keystones {
		cov, err := coverage.GetEntityCoverage(g.store, k.EntityID)
		if err != nil || cov.CoveragePercent == 0 {
			entity, err := g.store.GetEntity(k.EntityID)
			if err != nil {
				continue
			}

			issue := HealthIssue{
				Type:           IssueTypeUntestedKeystone,
				Entity:         entity.Name,
				File:           entity.FilePath,
				PageRank:       k.PageRank,
				Importance:     ImportanceKeystone,
				Recommendation: fmt.Sprintf("Add tests for %s - this is a keystone entity with high importance", entity.Name),
			}

			if cov != nil {
				issue.Coverage = cov.CoveragePercent
			}

			data.AddCriticalIssue(issue)
		}
	}

	return nil
}

// DeadCodeEntityTypes defines which entity types can be detected as dead code.
// These are types tracked in the call graph (have callers/callees relationships).
// Types NOT in this list (import, variable, constant) are excluded because
// their usage cannot be tracked via the dependency graph.
var DeadCodeEntityTypes = map[string]bool{
	"function":  true,
	"method":    true,
	"class":     true,
	"interface": true,
	"struct":    true,
	"type":      true,
	"trait":     true,
	"enum":      true,
}

// findDeadCodeCandidates finds entities with no incoming dependencies.
func (g *DataGatherer) findDeadCodeCandidates(data *HealthReportData) error {
	// Get entities with 0 in-degree (no callers)
	entities, err := g.store.QueryEntities(store.EntityFilter{
		Status: "active",
		Limit:  1000,
	})
	if err != nil {
		return err
	}

	// Group candidates by module (directory)
	moduleGroups := make(map[string][]DeadCodeCandidate)

	for _, e := range entities {
		// Skip entity types not tracked in call graph
		if !DeadCodeEntityTypes[e.EntityType] {
			continue
		}

		// Skip test files
		if strings.Contains(e.FilePath, "_test.go") || strings.Contains(e.FilePath, ".test.") {
			continue
		}

		// Check if this entity has any callers
		callers, err := g.store.GetDependenciesTo(e.ID)
		if err != nil {
			continue
		}

		// Also check metrics for in-degree
		metrics, _ := g.store.GetMetrics(e.ID)

		if len(callers) == 0 && (metrics == nil || metrics.InDegree == 0) {
			// Skip main functions, init functions, exported types
			if e.Name == "main" || e.Name == "init" || e.Visibility == "pub" {
				continue
			}

			// Extract module (directory) from file path
			module := filepath.Dir(e.FilePath)

			candidate := DeadCodeCandidate{
				Entity:         e.Name,
				EntityType:     e.EntityType,
				File:           e.FilePath,
				Line:           e.LineStart,
				Recommendation: fmt.Sprintf("Consider removing %s if it's unused", e.Name),
			}

			moduleGroups[module] = append(moduleGroups[module], candidate)
		}
	}

	// Convert to sorted slice of groups (most candidates first)
	var groups []DeadCodeGroup
	for module, candidates := range moduleGroups {
		groups = append(groups, DeadCodeGroup{
			Type:       IssueTypeDeadCodeGroup,
			Module:     module,
			Count:      len(candidates),
			Candidates: candidates,
		})
	}

	// Sort by count descending
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})

	// Add groups as info issues
	for _, group := range groups {
		data.AddInfoIssue(HealthIssue{
			Type:           IssueTypeDeadCodeGroup,
			Entity:         group.Module,
			Entities:       groupCandidateNames(group.Candidates),
			File:           group.Module,
			InDegree:       group.Count,
			Recommendation: fmt.Sprintf("%d dead code candidates in %s", group.Count, group.Module),
		})
	}

	// Store detailed groups in data for structured output
	data.DeadCodeGroups = groups

	return nil
}

// groupCandidateNames extracts entity names from candidates.
func groupCandidateNames(candidates []DeadCodeCandidate) []string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Entity
	}
	return names
}

// findComplexityHotspots finds entities with high complexity.
func (g *DataGatherer) findComplexityHotspots(complexity *ComplexityAnalysis) error {
	// Get entities with high out-degree (many dependencies)
	metrics, err := g.store.GetTopByOutDegree(20)
	if err != nil {
		return err
	}

	for _, m := range metrics {
		if m.OutDegree < 10 {
			continue // Only include entities with significant dependencies
		}

		entity, err := g.store.GetEntity(m.EntityID)
		if err != nil {
			continue
		}

		lines := 0
		if entity.LineEnd != nil {
			lines = *entity.LineEnd - entity.LineStart + 1
		}

		hotspot := ComplexityHotspot{
			Entity:    entity.Name,
			OutDegree: m.OutDegree,
			Lines:     lines,
		}

		complexity.Hotspots = append(complexity.Hotspots, hotspot)
	}

	return nil
}

// calculateRiskScore computes an overall health score (0-100, higher = healthier).
func (g *DataGatherer) calculateRiskScore(data *HealthReportData) int {
	score := 100

	// Deduct for critical issues (10 points each, max 50)
	criticalDeduct := len(data.Issues.Critical) * 10
	if criticalDeduct > 50 {
		criticalDeduct = 50
	}
	score -= criticalDeduct

	// Deduct for warnings (5 points each, max 25)
	warningDeduct := len(data.Issues.Warning) * 5
	if warningDeduct > 25 {
		warningDeduct = 25
	}
	score -= warningDeduct

	// Deduct for low coverage (up to 25 points)
	if data.Coverage != nil && data.Coverage.Overall < 80 {
		coverageDeduct := int((80 - data.Coverage.Overall) / 80 * 25)
		score -= coverageDeduct
	}

	if score < 0 {
		score = 0
	}

	return score
}

// Helper functions

// convertEntityToData converts a store.Entity to report.EntityData.
func (g *DataGatherer) convertEntityToData(e *store.Entity) EntityData {
	lines := [2]int{e.LineStart, e.LineStart}
	if e.LineEnd != nil {
		lines[1] = *e.LineEnd
	}

	return EntityData{
		ID:         e.ID,
		Name:       e.Name,
		Type:       e.EntityType,
		File:       e.FilePath,
		Lines:      lines,
		Signature:  e.Signature,
		DocComment: e.DocComment,
	}
}

// convertToChangedEntity converts a store.Entity to report.ChangedEntity.
func (g *DataGatherer) convertToChangedEntity(e *store.Entity, changeType string) ChangedEntity {
	lines := [2]int{e.LineStart, e.LineStart}
	if e.LineEnd != nil {
		lines[1] = *e.LineEnd
	}

	return ChangedEntity{
		ID:    e.ID,
		Name:  e.Name,
		Type:  e.EntityType,
		File:  e.FilePath,
		Lines: lines,
	}
}

// entityModified checks if an entity was modified between two versions.
func (g *DataGatherer) entityModified(from, to *store.Entity) bool {
	// Check signature hash change
	if from.SigHash != to.SigHash {
		return true
	}
	// Check body hash change
	if from.BodyHash != to.BodyHash {
		return true
	}
	// Check file path change
	if from.FilePath != to.FilePath {
		return true
	}
	return false
}

// computeLanguageBreakdown computes language distribution from a set of entities.
func (g *DataGatherer) computeLanguageBreakdown(entities []EntityData) map[string]int {
	breakdown := make(map[string]int)

	for _, e := range entities {
		// Infer language from file extension
		ext := filepath.Ext(e.File)
		var lang string
		switch ext {
		case ".go":
			lang = "go"
		case ".ts", ".tsx":
			lang = "typescript"
		case ".js", ".jsx":
			lang = "javascript"
		case ".py":
			lang = "python"
		case ".rs":
			lang = "rust"
		case ".java":
			lang = "java"
		default:
			lang = "other"
		}
		breakdown[lang]++
	}

	return breakdown
}

// computeImpactAnalysis computes the impact of changes.
func (g *DataGatherer) computeImpactAnalysis(impact **ImpactAnalysis, modified, deleted []ChangedEntity) error {
	analysis := &ImpactAnalysis{}

	// For each modified or deleted entity, count affected dependents
	allChanged := append(modified, deleted...)
	for _, change := range allChanged {
		deps, err := g.store.GetDependenciesTo(change.ID)
		if err != nil {
			continue
		}

		if len(deps) >= 5 { // Only track high-impact changes
			risk := "medium"
			if len(deps) >= 10 {
				risk = "high"
			}

			analysis.HighImpactChanges = append(analysis.HighImpactChanges, ImpactChange{
				Entity:             change.Name,
				DependentsAffected: len(deps),
				Risk:               risk,
			})
		}
	}

	if len(analysis.HighImpactChanges) > 0 {
		*impact = analysis
	}

	return nil
}

// classifyImportance determines the importance level based on PageRank and in-degree.
func classifyImportance(pageRank float64, inDegree int) Importance {
	// Keystones: high PageRank (top tier)
	if pageRank >= 0.01 {
		return ImportanceKeystone
	}
	// Bottlenecks: many callers
	if inDegree >= 10 {
		return ImportanceBottleneck
	}
	// Leaves: no dependents
	if inDegree == 0 {
		return ImportanceLeaf
	}
	// Normal: everything else
	return ImportanceNormal
}

// GetAllEntityCoverage retrieves all coverage data from the store.
func GetAllEntityCoverage(s *store.Store) ([]coverage.EntityCoverage, error) {
	rows, err := s.DB().Query(`
		SELECT entity_id, coverage_percent, covered_lines, uncovered_lines, last_run
		FROM entity_coverage
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var results []coverage.EntityCoverage
	for rows.Next() {
		var cov coverage.EntityCoverage
		var coveredJSON, uncoveredJSON, lastRunStr string

		if err := rows.Scan(&cov.EntityID, &cov.CoveragePercent, &coveredJSON, &uncoveredJSON, &lastRunStr); err != nil {
			return nil, err
		}

		results = append(results, cov)
	}

	return results, rows.Err()
}

// gatherPlaygroundMetadata populates playground-specific metadata for interactive reports.
func (g *DataGatherer) gatherPlaygroundMetadata(data *OverviewReportData) error {
	// Initialize playground metadata
	pm := &PlaygroundMetadata{
		Enabled:    true,
		Layers:     make([]LayerInfo, 0),
		ElementMap: make(map[string]string),
	}

	// Build layer information from modules
	layerCounts := make(map[string]int)
	for _, mod := range data.Modules {
		layer := deriveLayerFromPath(mod.Path)
		layerCounts[layer] += mod.Entities
	}

	// Define layer colors and create LayerInfo entries
	layerColors := map[string]string{
		LayerCore:     "#4a90d9", // Blue
		LayerAPI:      "#50c878", // Emerald green
		LayerStore:    "#9b59b6", // Purple
		LayerParser:   "#e67e22", // Orange
		LayerGraph:    "#1abc9c", // Teal
		LayerOutput:   "#f39c12", // Yellow
		LayerTest:     "#95a5a6", // Gray
		LayerExternal: "#34495e", // Dark gray
	}

	for layer, count := range layerCounts {
		color := layerColors[layer]
		if color == "" {
			color = "#7f8c8d" // Default gray
		}
		pm.Layers = append(pm.Layers, LayerInfo{
			ID:             layer,
			Label:          strings.Title(layer),
			Color:          color,
			EntityCount:    count,
			DefaultVisible: true,
		})
	}

	// Sort layers by entity count (descending)
	sort.Slice(pm.Layers, func(i, j int) bool {
		return pm.Layers[i].EntityCount > pm.Layers[j].EntityCount
	})

	// Define connection types
	pm.ConnectionTypes = []ConnectionTypeInfo{
		{Type: DepTypeCalls, Label: "Calls", Color: "#3498db", DefaultVisible: true},
		{Type: DepTypeUsesType, Label: "Uses Type", Color: "#2ecc71", DefaultVisible: true},
		{Type: DepTypeImplements, Label: "Implements", Color: "#9b59b6", DefaultVisible: true},
		{Type: DepTypeExtends, Label: "Extends", Color: "#e74c3c", DefaultVisible: true},
		{Type: DepTypeImports, Label: "Imports", Color: "#95a5a6", DefaultVisible: false},
	}

	// Define view presets
	pm.ViewPresets = []ViewPreset{
		{
			ID:                 "full",
			Label:              "Full System",
			Description:        "Show all layers and connections",
			VisibleLayers:      []string{LayerCore, LayerAPI, LayerStore, LayerParser, LayerGraph, LayerOutput},
			VisibleConnections: []string{DepTypeCalls, DepTypeUsesType, DepTypeImplements},
		},
		{
			ID:                 "keystones",
			Label:              "Keystones Only",
			Description:        "Show only keystone entities and their connections",
			VisibleLayers:      []string{LayerCore, LayerAPI, LayerStore, LayerParser, LayerGraph, LayerOutput},
			VisibleConnections: []string{DepTypeCalls, DepTypeUsesType},
			ImportanceFilter:   []string{string(ImportanceKeystone)},
		},
		{
			ID:                 "core",
			Label:              "Core Architecture",
			Description:        "Focus on core, store, and parser modules",
			VisibleLayers:      []string{LayerCore, LayerStore, LayerParser},
			VisibleConnections: []string{DepTypeCalls, DepTypeUsesType},
		},
		{
			ID:                 "api",
			Label:              "API Layer",
			Description:        "Focus on API and output modules",
			VisibleLayers:      []string{LayerAPI, LayerOutput},
			VisibleConnections: []string{DepTypeCalls},
		},
	}

	// Build element map from keystones (entity ID -> SVG element ID)
	for _, entity := range data.Keystones {
		// Generate a valid SVG element ID from entity name
		svgID := "node-" + sanitizeForSVGID(entity.Name)
		pm.ElementMap[entity.ID] = svgID
	}

	data.Playground = pm
	return nil
}

// enrichEntityForPlayground adds playground-specific fields to an entity.
func (g *DataGatherer) enrichEntityForPlayground(entity *EntityData) {
	// Derive layer from file path
	entity.Layer = deriveLayerFromPath(entity.File)

	// Build CSS classes for filtering
	var classes []string
	classes = append(classes, "layer-"+entity.Layer)
	classes = append(classes, "entity-"+entity.Type)
	if entity.Importance != "" {
		classes = append(classes, "importance-"+string(entity.Importance))
	}
	entity.CSSClasses = strings.Join(classes, " ")
}

// deriveLayerFromPath determines the logical layer from a file path.
func deriveLayerFromPath(filePath string) string {
	// Normalize path separators
	path := filepath.ToSlash(filePath)
	parts := strings.Split(path, "/")

	// Look for common directory patterns
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "cmd", "cli", "commands":
			return LayerAPI
		case "api", "handler", "handlers", "routes", "http":
			return LayerAPI
		case "store", "storage", "db", "database", "repository":
			return LayerStore
		case "parser", "parse", "ast", "syntax":
			return LayerParser
		case "graph", "deps", "dependencies":
			return LayerGraph
		case "output", "format", "render", "view":
			return LayerOutput
		case "test", "tests", "testing", "_test":
			return LayerTest
		case "internal", "pkg", "lib":
			// Continue looking for more specific directories
			continue
		case "external", "vendor", "third_party":
			return LayerExternal
		}
	}

	// Check for test files by suffix
	if strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/test/") {
		return LayerTest
	}

	// Default to core for everything else
	return LayerCore
}

// sanitizeForSVGID converts a string to a valid SVG element ID.
func sanitizeForSVGID(s string) string {
	// Replace invalid characters with dashes
	result := strings.Builder{}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	return result.String()
}

// gatherFeaturePlaygroundMetadata populates feature-specific playground metadata.
func (g *DataGatherer) gatherFeaturePlaygroundMetadata(data *FeatureReportData) error {
	pm := &PlaygroundMetadata{
		Enabled:    true,
		Layers:     make([]LayerInfo, 0),
		ElementMap: make(map[string]string),
	}

	// Build layer information from entities
	layerCounts := make(map[string]int)
	for _, entity := range data.Entities {
		layer := entity.Layer
		if layer == "" {
			layer = deriveLayerFromPath(entity.File)
		}
		layerCounts[layer]++
	}

	// Define layer colors
	layerColors := map[string]string{
		LayerCore:     "#4a90d9",
		LayerAPI:      "#50c878",
		LayerStore:    "#9b59b6",
		LayerParser:   "#e67e22",
		LayerGraph:    "#1abc9c",
		LayerOutput:   "#f39c12",
		LayerTest:     "#95a5a6",
		LayerExternal: "#34495e",
	}

	for layer, count := range layerCounts {
		color := layerColors[layer]
		if color == "" {
			color = "#7f8c8d"
		}
		pm.Layers = append(pm.Layers, LayerInfo{
			ID:             layer,
			Label:          strings.Title(layer),
			Color:          color,
			EntityCount:    count,
			DefaultVisible: true,
		})
	}

	// Sort layers by count
	sort.Slice(pm.Layers, func(i, j int) bool {
		return pm.Layers[i].EntityCount > pm.Layers[j].EntityCount
	})

	// Count connection types
	connCounts := make(map[string]int)
	for _, dep := range data.Dependencies {
		connCounts[dep.Type]++
	}

	pm.ConnectionTypes = []ConnectionTypeInfo{
		{Type: DepTypeCalls, Label: "Calls", Color: "#3498db", Count: connCounts[DepTypeCalls], DefaultVisible: true},
		{Type: DepTypeUsesType, Label: "Uses Type", Color: "#2ecc71", Count: connCounts[DepTypeUsesType], DefaultVisible: true},
		{Type: DepTypeImplements, Label: "Implements", Color: "#9b59b6", Count: connCounts[DepTypeImplements], DefaultVisible: true},
	}

	// Feature-specific view presets
	pm.ViewPresets = []ViewPreset{
		{
			ID:                 "full",
			Label:              "Full Feature",
			Description:        "Show all entities and connections in this feature",
			VisibleLayers:      []string{LayerCore, LayerAPI, LayerStore, LayerParser},
			VisibleConnections: []string{DepTypeCalls, DepTypeUsesType},
		},
		{
			ID:                 "call-flow",
			Label:              "Call Flow",
			Description:        "Focus on function calls only",
			VisibleLayers:      []string{LayerCore, LayerAPI, LayerStore, LayerParser},
			VisibleConnections: []string{DepTypeCalls},
		},
		{
			ID:                 "keystones",
			Label:              "Keystones Only",
			Description:        "Show only high-importance entities",
			VisibleLayers:      []string{LayerCore, LayerAPI, LayerStore},
			VisibleConnections: []string{DepTypeCalls, DepTypeUsesType},
			ImportanceFilter:   []string{string(ImportanceKeystone), string(ImportanceBottleneck)},
		},
	}

	// Build element map
	for _, entity := range data.Entities {
		svgID := "node-" + sanitizeForSVGID(entity.Name)
		pm.ElementMap[entity.ID] = svgID
	}

	data.Playground = pm
	return nil
}

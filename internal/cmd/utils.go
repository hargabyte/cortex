package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/coverage"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
)

// Shared utility functions for command implementations

// queryType represents the type of query
type queryType int

const (
	queryTypeSimple queryType = iota
	queryTypeQualified
	queryTypePathQualified
	queryTypeDirect
)

// entityTypePriority returns a priority score for entity types.
// Lower is better. Types/structs/functions are preferred over imports/constants.
func entityTypePriority(entityType string) int {
	switch strings.ToLower(entityType) {
	case "struct", "type", "class":
		return 1
	case "interface":
		return 2
	case "function", "func":
		return 3
	case "method":
		return 4
	case "constant", "const":
		return 5
	case "variable", "var":
		return 6
	case "import":
		return 100 // Lowest priority - imports are usually noise
	default:
		return 50
	}
}

// resolveEntityByName resolves an entity by name, ID, or qualified name.
// It supports the same query formats as the find command:
// - Direct IDs: sa-fn-a7f9b2-LoginUser
// - Simple names: LoginUser (prefix match)
// - Qualified names: auth.LoginUser (package.symbol)
// - Path-qualified: auth/login.LoginUser (path/file.symbol)
// - File-path hints: Store@internal/store/db.go (name@file-path)
//
// Resolution priority:
// 1. Exact ID match
// 2. Types/structs/interfaces over functions over imports
// 3. Higher PageRank (more important) entities first
//
// If multiple entities match, it returns an error listing the options.
// If typeFilter is non-empty, only entities of that type are considered.
func resolveEntityByName(query string, storeDB *store.Store, typeFilter string) (*store.Entity, error) {
	// Check for file-path hint syntax: name@path
	var fileHint string
	if atIdx := strings.LastIndex(query, "@"); atIdx > 0 && atIdx < len(query)-1 {
		fileHint = query[atIdx+1:]
		query = query[:atIdx]
	}

	// If query looks like a direct ID, try direct lookup first
	if isDirectIDQuery(query) {
		entity, err := storeDB.GetEntity(query)
		if err == nil {
			return entity, nil
		}
		// Fall through to name-based lookup if direct lookup fails
	}

	// Query all entities
	filter := store.EntityFilter{
		Status: "active",
		Limit:  10000,
	}
	if typeFilter != "" {
		filter.EntityType = typeFilter
	}

	entities, err := storeDB.QueryEntities(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}

	// Filter by name pattern (exact match first)
	var exactMatches []*store.Entity
	var prefixMatches []*store.Entity

	for _, e := range entities {
		// Apply file hint filter if provided
		if fileHint != "" && !strings.Contains(e.FilePath, fileHint) {
			continue
		}

		if matchesQueryExact(e, query) {
			exactMatches = append(exactMatches, e)
		} else if matchesQueryPrefix(e, query) {
			prefixMatches = append(prefixMatches, e)
		}
	}

	// Prefer exact matches
	matches := exactMatches
	if len(matches) == 0 {
		matches = prefixMatches
	}

	// Handle results
	if len(matches) == 0 {
		if fileHint != "" {
			return nil, fmt.Errorf("entity not found: %q with file hint %q", query, fileHint)
		}
		return nil, fmt.Errorf("entity not found: %q", query)
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	// Multiple matches - try smart resolution
	// Sort by: entity type priority, then by PageRank (importance)
	type rankedMatch struct {
		entity   *store.Entity
		priority int
		pagerank float64
	}

	ranked := make([]rankedMatch, len(matches))
	for i, e := range matches {
		priority := entityTypePriority(e.EntityType)
		var pagerank float64
		if m, err := storeDB.GetMetrics(e.ID); err == nil && m != nil {
			pagerank = m.PageRank
		}
		ranked[i] = rankedMatch{entity: e, priority: priority, pagerank: pagerank}
	}

	// Sort: lower priority number first, then higher pagerank
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].priority != ranked[j].priority {
			return ranked[i].priority < ranked[j].priority
		}
		return ranked[i].pagerank > ranked[j].pagerank
	})

	// Check if top match is clearly better (different priority tier)
	if ranked[0].priority < ranked[1].priority {
		// Count how many share the top priority
		topPriority := ranked[0].priority
		topCount := 0
		for _, r := range ranked {
			if r.priority == topPriority {
				topCount++
			}
		}
		// If only one entity at top priority tier, return it
		if topCount == 1 {
			return ranked[0].entity, nil
		}
	}

	// Still ambiguous - return helpful error with sorted suggestions
	var suggestions []string
	for i, r := range ranked {
		if i >= 10 {
			suggestions = append(suggestions, fmt.Sprintf("  ... and %d more", len(ranked)-10))
			break
		}
		// Include pagerank hint for disambiguation
		prHint := ""
		if r.pagerank > 0 {
			prHint = fmt.Sprintf(" [pr=%.3f]", r.pagerank)
		}
		suggestions = append(suggestions, fmt.Sprintf("  - %s (%s) at %s%s",
			r.entity.Name, r.entity.EntityType, formatStoreLocation(r.entity), prHint))
	}

	// Add helpful hint about file-path syntax
	hint := "\n\nUse a more specific name, full entity ID, or file hint: name@path"
	return nil, fmt.Errorf("multiple entities match %q:\n%s%s", query, strings.Join(suggestions, "\n"), hint)
}

// matchesQueryExact checks if an entity exactly matches the query
func matchesQueryExact(e *store.Entity, query string) bool {
	queryT, pkg, name := parseQuery(query)

	switch queryT {
	case queryTypeDirect:
		return e.ID == query

	case queryTypePathQualified:
		entityFile := e.FilePath
		entityName := extractSymbolName(e)
		return matchesPathQualified(entityFile, entityName, pkg, name, true)

	case queryTypeQualified:
		entityPkg := extractPackage(e)
		entityName := extractSymbolName(e)
		return matchesQualified(entityPkg, entityName, pkg, name, true)

	case queryTypeSimple:
		entityName := extractSymbolName(e)
		return strings.EqualFold(entityName, name)
	}

	return false
}

// matchesQueryPrefix checks if an entity matches the query with prefix matching
func matchesQueryPrefix(e *store.Entity, query string) bool {
	queryT, pkg, name := parseQuery(query)

	switch queryT {
	case queryTypeDirect:
		return e.ID == query

	case queryTypePathQualified:
		entityFile := e.FilePath
		entityName := extractSymbolName(e)
		return matchesPathQualified(entityFile, entityName, pkg, name, false)

	case queryTypeQualified:
		entityPkg := extractPackage(e)
		entityName := extractSymbolName(e)
		return matchesQualified(entityPkg, entityName, pkg, name, false)

	case queryTypeSimple:
		entityName := extractSymbolName(e)
		return strings.HasPrefix(strings.ToLower(entityName), strings.ToLower(name))
	}

	return false
}

// parseQuery parses a query string and returns its type and components
func parseQuery(query string) (queryType, string, string) {
	// Check for direct ID (starts with common prefixes like "sa-", "bd-", etc.)
	if isDirectIDQuery(query) {
		return queryTypeDirect, "", query
	}

	// Check for path-qualified (contains "/" and ".")
	// Format: auth/login.LoginUser
	if strings.Contains(query, "/") && strings.Contains(query, ".") {
		lastDot := strings.LastIndex(query, ".")
		if lastDot > 0 && lastDot < len(query)-1 {
			path := query[:lastDot]
			name := query[lastDot+1:]
			return queryTypePathQualified, path, name
		}
	}

	// Check for qualified (contains "." but not "/")
	// Format: auth.LoginUser
	if strings.Contains(query, ".") && !strings.Contains(query, "/") {
		lastDot := strings.LastIndex(query, ".")
		if lastDot > 0 && lastDot < len(query)-1 {
			pkg := query[:lastDot]
			name := query[lastDot+1:]
			return queryTypeQualified, pkg, name
		}
	}

	// Simple name
	return queryTypeSimple, "", query
}

// isDirectIDQuery checks if the query looks like a direct entity ID
func isDirectIDQuery(query string) bool {
	// Check for common bead ID patterns
	idPrefixes := []string{"sa-", "bd-", "cx-"}
	for _, prefix := range idPrefixes {
		if strings.HasPrefix(strings.ToLower(query), prefix) {
			return true
		}
	}

	// Also match pattern like "sa-fn-a7f9b2-LoginUser" (type marker ID)
	// Pattern: prefix-type-hash-name
	typeIDPattern := regexp.MustCompile(`^[a-z]+-[a-z]+-[a-f0-9]+-`)
	return typeIDPattern.MatchString(strings.ToLower(query))
}

// matchesPathQualified checks if file/name matches a path-qualified query
func matchesPathQualified(entityFile, entityName, queryPath, queryName string, exact bool) bool {
	// Check if file path matches
	if !strings.Contains(strings.ToLower(entityFile), strings.ToLower(queryPath)) &&
		!strings.HasSuffix(strings.ToLower(entityFile), strings.ToLower(queryPath)) {
		return false
	}

	// Check name match
	if exact {
		return strings.EqualFold(entityName, queryName)
	}
	return strings.HasPrefix(strings.ToLower(entityName), strings.ToLower(queryName))
}

// matchesQualified checks if package/name matches a qualified query
func matchesQualified(entityPkg, entityName, queryPkg, queryName string, exact bool) bool {
	// Check if package matches
	if !strings.EqualFold(entityPkg, queryPkg) &&
		!strings.HasSuffix(strings.ToLower(entityPkg), strings.ToLower(queryPkg)) {
		return false
	}

	// Check name match
	if exact {
		return strings.EqualFold(entityName, queryName)
	}
	return strings.HasPrefix(strings.ToLower(entityName), strings.ToLower(queryName))
}

// extractSymbolName extracts the symbol name from an entity
func extractSymbolName(e *store.Entity) string {
	name := e.Name

	// If name contains package qualifier, extract just the name
	if idx := strings.LastIndex(name, "."); idx > 0 {
		return name[idx+1:]
	}

	return name
}

// extractPackage extracts the package/module name from an entity
func extractPackage(e *store.Entity) string {
	// Try to get from name if qualified
	name := e.Name
	if idx := strings.LastIndex(name, "."); idx > 0 {
		return name[:idx]
	}

	// Try to infer from file path
	filePath := e.FilePath
	if filePath != "" {
		// Extract directory as package
		if idx := strings.LastIndex(filePath, "/"); idx > 0 {
			return filePath[:idx]
		}
	}

	return ""
}

// openStore is a helper to open the store from the current directory
func openStore() (*store.Store, error) {
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return nil, fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}

	return storeDB, nil
}

// mapStoreEntityTypeToString converts store entity type to output type string
func mapStoreEntityTypeToString(t string) string {
	switch strings.ToLower(t) {
	case "function", "func":
		return "function"
	case "method":
		return "method"
	case "type", "struct", "class":
		return "struct"
	case "interface":
		return "interface"
	case "constant", "const", "var", "variable":
		return "constant"
	default:
		return t
	}
}

// formatStoreLocation formats a store entity's location as file:line-line
func formatStoreLocation(e *store.Entity) string {
	if e.LineEnd != nil && *e.LineEnd != e.LineStart {
		return fmt.Sprintf("%s:%d-%d", e.FilePath, e.LineStart, *e.LineEnd)
	}
	return fmt.Sprintf("%s:%d", e.FilePath, e.LineStart)
}

// inferVisibility infers visibility from entity name
func inferVisibility(name string) string {
	// Simple heuristic: if name starts with uppercase, it's public
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return "public"
	}
	return "private"
}

// formatEntityLocation is an alias for formatStoreLocation for backward compatibility
func formatEntityLocation(e *store.Entity) string {
	return formatStoreLocation(e)
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

// normalizeFilePath cleans a file path for database queries.
// It removes leading "./" and cleans the path for consistency with stored paths.
func normalizeFilePath(path string) string {
	// Clean the path (removes ./, resolves .., etc.)
	cleaned := filepath.Clean(path)
	// Remove leading "./" if still present after cleaning
	if strings.HasPrefix(cleaned, "./") {
		cleaned = cleaned[2:]
	}
	return cleaned
}

// isFilePath checks if a string looks like a file path
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

// computeImportanceLevel converts PageRank to importance level string
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

// SearchOutput represents the output of a search command
type SearchOutput struct {
	Query   string                          `json:"query" yaml:"query"`
	Results map[string]*output.EntityOutput `json:"results" yaml:"results"`
	Count   int                             `json:"count" yaml:"count"`
	Scores  map[string]*SearchScore         `json:"scores,omitempty" yaml:"scores,omitempty"`
}

// SearchScore represents the relevance scores for an entity
type SearchScore struct {
	FTSScore      float64 `json:"fts_score" yaml:"fts_score"`
	PageRank      float64 `json:"pagerank" yaml:"pagerank"`
	CombinedScore float64 `json:"combined_score" yaml:"combined_score"`
}

// MarshalYAML implements custom YAML marshaling for SearchOutput
func (s *SearchOutput) MarshalYAML() (interface{}, error) {
	m := make(map[string]interface{})
	m["query"] = s.Query
	m["results"] = s.Results
	m["count"] = s.Count
	if len(s.Scores) > 0 {
		m["scores"] = s.Scores
	}
	return m, nil
}

// buildSearchOutput converts search results to output format
func buildSearchOutput(results []*store.SearchResult, density output.Density, storeDB *store.Store, query string) *SearchOutput {
	searchOutput := &SearchOutput{
		Query:   query,
		Results: make(map[string]*output.EntityOutput),
		Scores:  make(map[string]*SearchScore),
	}

	for _, r := range results {
		e := r.Entity
		name := e.Name

		// Convert to EntityOutput
		entityOut := &output.EntityOutput{
			Type:     mapStoreEntityTypeToString(e.EntityType),
			Location: formatStoreLocation(e),
		}

		// Add signature for medium/dense
		if density.IncludesSignature() && e.Signature != "" {
			entityOut.Signature = e.Signature
		}

		// Add visibility
		if density.IncludesSignature() {
			entityOut.Visibility = inferVisibility(e.Name)
		}

		// Add dependencies for medium/dense
		if density.IncludesEdges() {
			deps := &output.Dependencies{}

			// Get outgoing calls
			depsOut, err := storeDB.GetDependenciesFrom(e.ID)
			if err == nil {
				for _, d := range depsOut {
					if d.DepType == "calls" {
						deps.Calls = append(deps.Calls, d.ToID)
					}
				}
			}

			// Get incoming calls
			depsIn, err := storeDB.GetDependenciesTo(e.ID)
			if err == nil {
				for _, d := range depsIn {
					if d.DepType == "calls" {
						deps.CalledBy = append(deps.CalledBy, output.CalledByEntry{
							Name: d.FromID,
						})
					}
				}
			}

			if len(deps.Calls) > 0 || len(deps.CalledBy) > 0 {
				entityOut.Dependencies = deps
			}
		}

		// Add metrics for dense
		if density.IncludesMetrics() {
			entityOut.Metrics = &output.Metrics{
				PageRank:  r.PageRank,
				InDegree:  0,
				OutDegree: 0,
			}

			// Add search-specific scores
			searchOutput.Scores[name] = &SearchScore{
				FTSScore:      r.FTSScore,
				PageRank:      r.PageRank,
				CombinedScore: r.CombinedScore,
			}
		}

		searchOutput.Results[name] = entityOut
		searchOutput.Count++
	}

	return searchOutput
}

// ============================================================================
// Impact Analysis Types and Functions (from impact.go)
// ============================================================================

// buildImpactOutput constructs the impact output structure
func buildImpactOutput(target string, affected map[string]*impactEntry, depth int) *output.ImpactOutput {
	impactOut := &output.ImpactOutput{
		Impact: &output.ImpactMetadata{
			Target: target,
			Depth:  depth,
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
			Type:       mapStoreEntityTypeToString(mapCGFTypeToStoreType(entry.entityType)),
			Location:   entry.location,
			Impact:     impactType,
			Importance: entry.importance,
			Reason:     reason,
		}

		impactOut.Affected[entry.name] = affectedEntity
	}

	return impactOut
}

// countAffectedFiles counts unique files in affected entities
func countAffectedFiles(affected map[string]*impactEntry) int {
	files := make(map[string]bool)
	for _, entry := range affected {
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

// ============================================================================
// Coverage Gap Types and Functions (from gaps.go)
// ============================================================================

// coverageGap represents an entity with insufficient test coverage
type coverageGap struct {
	entity       *store.Entity
	metrics      *store.Metrics
	coverage     *coverage.EntityCoverage
	riskScore    float64
	riskCategory string
}

// categorizeRisk determines risk category based on metrics and coverage
func categorizeRisk(m *store.Metrics, cov *coverage.EntityCoverage, cfg *config.Config) string {
	isKeystone := m.PageRank >= cfg.Metrics.KeystoneThreshold

	if isKeystone {
		if cov.CoveragePercent < 25 {
			return "CRITICAL"
		} else if cov.CoveragePercent < 50 {
			return "HIGH"
		}
		return "MEDIUM"
	}

	if cov.CoveragePercent < 25 {
		return "MEDIUM"
	}
	return "LOW"
}

// groupGapsByRisk groups gaps by their risk category
func groupGapsByRisk(gaps []coverageGap) map[string][]coverageGap {
	result := make(map[string][]coverageGap)
	for _, gap := range gaps {
		category := strings.ToLower(gap.riskCategory)
		result[category] = append(result[category], gap)
	}
	return result
}

// buildGapsOutput constructs the output data structure for coverage gaps
func buildGapsOutput(gapsByRisk map[string][]coverageGap, keystoneCount int) map[string]interface{} {
	coverageGaps := make(map[string]interface{})

	for _, category := range []string{"critical", "high", "medium", "low"} {
		if gaps, ok := gapsByRisk[category]; ok && len(gaps) > 0 {
			categoryData := make([]map[string]interface{}, 0, len(gaps))
			for _, gap := range gaps {
				gapData := map[string]interface{}{
					"name":       gap.entity.Name,
					"type":       mapStoreEntityTypeToString(gap.entity.EntityType),
					"location":   formatStoreLocation(gap.entity),
					"importance": determineImportanceLabel(gap.metrics),
					"in_degree":  gap.metrics.InDegree,
					"pagerank":   gap.metrics.PageRank,
					"coverage":   fmt.Sprintf("%.1f%%", gap.coverage.CoveragePercent),
					"risk_score": fmt.Sprintf("%.3f", gap.riskScore),
					"risk":       gap.riskCategory,
				}

				if gap.riskCategory == "CRITICAL" {
					gapData["recommendation"] = "Add tests before ANY changes"
				} else if gap.riskCategory == "HIGH" {
					gapData["recommendation"] = "Increase test coverage before major changes"
				}

				categoryData = append(categoryData, gapData)
			}
			coverageGaps[category] = categoryData
		}
	}

	summary := map[string]interface{}{
		"keystones_total": keystoneCount,
		"critical_gaps":   len(gapsByRisk["critical"]),
		"high_gaps":       len(gapsByRisk["high"]),
		"medium_gaps":     len(gapsByRisk["medium"]),
		"low_gaps":        len(gapsByRisk["low"]),
	}

	criticalCount := len(gapsByRisk["critical"])
	highCount := len(gapsByRisk["high"])
	if criticalCount > 0 {
		summary["recommendation"] = "URGENT: Address critical gaps before next release"
	} else if highCount > 0 {
		summary["recommendation"] = "Address high-priority gaps in near term"
	} else {
		summary["recommendation"] = "Continue monitoring coverage for important entities"
	}

	return map[string]interface{}{
		"coverage_gaps": coverageGaps,
		"summary":       summary,
	}
}

// determineImportanceLabel determines the importance label for an entity
func determineImportanceLabel(m *store.Metrics) string {
	cfg, _ := config.Load(".")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if m.PageRank >= cfg.Metrics.KeystoneThreshold {
		return "keystone"
	} else if m.Betweenness >= cfg.Metrics.BottleneckThreshold {
		return "bottleneck"
	} else if m.InDegree == 0 {
		return "leaf"
	}
	return "normal"
}

// printTaskCommands prints bd commands to create tasks for coverage gaps
func printTaskCommands(gaps []coverageGap, cfg *config.Config) error {
	fmt.Println("# Coverage gap tasks - run these commands to create beads:")
	fmt.Println()

	for _, gap := range gaps {
		if gap.riskCategory == "LOW" {
			continue
		}

		priority := 2
		if gap.riskCategory == "CRITICAL" {
			priority = 0
		} else if gap.riskCategory == "HIGH" {
			priority = 1
		}

		desc := fmt.Sprintf("Add test coverage for %s (currently %.1f%%)",
			gap.entity.Name, gap.coverage.CoveragePercent)

		fmt.Printf("bd create --title \"Test: %s\" --type task --priority %d --description \"%s\"\n",
			gap.entity.Name, priority, desc)
	}

	return nil
}

// ============================================================================
// Verification Types (from verify.go)
// ============================================================================

// VerifyOutput represents the output of the verify command
type VerifyOutput struct {
	Verification *VerificationData `yaml:"verification" json:"verification"`
}

// VerificationData holds the detailed verification information
type VerificationData struct {
	Status          string        `yaml:"status" json:"status"`
	EntitiesChecked int           `yaml:"entities_checked" json:"entities_checked"`
	Valid           int           `yaml:"valid" json:"valid"`
	Drifted         int           `yaml:"drifted" json:"drifted"`
	Missing         int           `yaml:"missing" json:"missing"`
	Issues          []VerifyIssue `yaml:"issues" json:"issues"`
	Actions         []string      `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// VerifyIssue represents an individual verification issue
type VerifyIssue struct {
	Entity   string `yaml:"entity" json:"entity"`
	Type     string `yaml:"type" json:"type"`
	Location string `yaml:"location" json:"location"`
	Reason   string `yaml:"reason" json:"reason"`
	Detail   string `yaml:"detail" json:"detail"`
	Expected string `yaml:"expected,omitempty" json:"expected,omitempty"`
	Actual   string `yaml:"actual,omitempty" json:"actual,omitempty"`
	HashType string `yaml:"hash_type,omitempty" json:"hash_type,omitempty"`
}

// DiffEntity represents an entity that changed
type DiffEntity struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Location string `yaml:"location" json:"location"`
	Change   string `yaml:"change,omitempty" json:"change,omitempty"`
	OldHash  string `yaml:"old_hash,omitempty" json:"old_hash,omitempty"`
	NewHash  string `yaml:"new_hash,omitempty" json:"new_hash,omitempty"`
}

// DiffOutput represents the output of the diff command
type DiffOutput struct {
	Summary  DiffSummary  `yaml:"summary" json:"summary"`
	Added    []DiffEntity `yaml:"added,omitempty" json:"added,omitempty"`
	Modified []DiffEntity `yaml:"modified,omitempty" json:"modified,omitempty"`
	Removed  []DiffEntity `yaml:"removed,omitempty" json:"removed,omitempty"`
}

// DiffSummary contains counts of changes
type DiffSummary struct {
	FilesChanged     int    `yaml:"files_changed" json:"files_changed"`
	EntitiesAdded    int    `yaml:"entities_added" json:"entities_added"`
	EntitiesModified int    `yaml:"entities_modified" json:"entities_modified"`
	EntitiesRemoved  int    `yaml:"entities_removed" json:"entities_removed"`
	LastScan         string `yaml:"last_scan,omitempty" json:"last_scan,omitempty"`
}

// hasPrefix checks if a path has a given directory prefix
func hasPrefix(path, prefix string) bool {
	if prefix != "" && prefix[len(prefix)-1] != filepath.Separator {
		prefix = prefix + string(filepath.Separator)
	}
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// verifyResult holds the categorized verification results
type verifyResult struct {
	valid   []verifyEntry
	drifted []verifyEntry
	missing []verifyEntry
}

// verifyEntry holds details about a single entity verification
type verifyEntry struct {
	entity  *store.Entity
	file    string
	line    string
	oldSig  string
	newSig  string
	oldBody string
	newBody string
	reason  string
	detail  string
}

// groupByFileStore groups entities by their file path
func groupByFileStore(entities []*store.Entity) map[string][]*store.Entity {
	result := make(map[string][]*store.Entity)
	for _, e := range entities {
		if e.FilePath != "" {
			result[e.FilePath] = append(result[e.FilePath], e)
		}
	}
	return result
}

// buildEntityLookup builds a map from entity name to extracted entity
func buildEntityLookup(entities []extract.Entity) map[string]*extract.Entity {
	result := make(map[string]*extract.Entity)

	for i := range entities {
		e := &entities[i]
		result[e.Name] = e
		if e.Receiver != "" {
			result[e.Receiver+"."+e.Name] = e
		}
	}

	return result
}

// findMatchingEntityByName finds the current entity matching a stored entity by name
func findMatchingEntityByName(name string, currentMap map[string]*extract.Entity) *extract.Entity {
	if current, ok := currentMap[name]; ok {
		return current
	}
	return nil
}

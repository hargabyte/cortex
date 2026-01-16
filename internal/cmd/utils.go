package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
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

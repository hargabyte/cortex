// Package coverage provides test suggestion generation based on coverage and importance metrics.
// This file implements prioritized test suggestions combining coverage gaps with code importance.
package coverage

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/store"
)

// TestSuggestion represents a prioritized suggestion for adding tests.
type TestSuggestion struct {
	// Entity is the code entity needing tests
	EntityID   string `json:"entity_id"`
	EntityName string `json:"entity_name"`
	EntityType string `json:"entity_type"`
	FilePath   string `json:"file_path"`
	LineStart  int    `json:"line_start"`
	LineEnd    int    `json:"line_end,omitempty"`

	// Coverage metrics
	CoveragePercent float64 `json:"coverage_percent"`
	CoveredLines    int     `json:"covered_lines"`
	UncoveredLines  int     `json:"uncovered_lines"`

	// Importance metrics
	PageRank    float64 `json:"pagerank"`
	InDegree    int     `json:"in_degree"`
	OutDegree   int     `json:"out_degree"`
	Betweenness float64 `json:"betweenness"`
	IsKeystone  bool    `json:"is_keystone"`

	// Priority calculation
	Priority      int     `json:"priority"`       // 0=highest, 4=lowest
	PriorityLabel string  `json:"priority_label"` // critical, high, medium, low, backlog
	PriorityScore float64 `json:"priority_score"` // Raw score for sorting

	// Signature analysis
	Signature     string   `json:"signature,omitempty"`
	TestScenarios []string `json:"test_scenarios,omitempty"` // Suggested test scenarios
}

// SuggestionOptions controls how suggestions are generated.
type SuggestionOptions struct {
	// TopN limits the number of suggestions returned (0 = unlimited)
	TopN int
	// EntityID filters suggestions to a specific entity
	EntityID string
	// KeystonesOnly returns only keystone entities
	KeystonesOnly bool
	// CoverageThreshold filters entities above this coverage percentage
	CoverageThreshold float64
	// IncludeSignatureAnalysis enables signature-based test scenario suggestions
	IncludeSignatureAnalysis bool
}

// SuggestionResult contains the generated suggestions and summary statistics.
type SuggestionResult struct {
	Suggestions []TestSuggestion            `json:"suggestions"`
	Summary     SuggestionSummary           `json:"summary"`
	ByPriority  map[string][]TestSuggestion `json:"by_priority,omitempty"`
}

// SuggestionSummary provides aggregate statistics about test suggestions.
type SuggestionSummary struct {
	TotalEntities     int     `json:"total_entities"`
	EntitiesWithGaps  int     `json:"entities_with_gaps"`
	KeystonesWithGaps int     `json:"keystones_with_gaps"`
	CriticalCount     int     `json:"critical_count"`
	HighCount         int     `json:"high_count"`
	MediumCount       int     `json:"medium_count"`
	LowCount          int     `json:"low_count"`
	AverageCoverage   float64 `json:"average_coverage"`
	Recommendation    string  `json:"recommendation"`
}

// GenerateSuggestions creates prioritized test suggestions combining coverage and importance.
func GenerateSuggestions(s *store.Store, opts SuggestionOptions) (*SuggestionResult, error) {
	// Load config for thresholds
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Default coverage threshold
	if opts.CoverageThreshold == 0 {
		opts.CoverageThreshold = 75.0
	}

	// If specific entity requested, handle separately
	if opts.EntityID != "" {
		return generateSuggestionForEntity(s, cfg, opts)
	}

	// Get all active entities
	entities, err := s.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}

	var suggestions []TestSuggestion
	var totalCoverage float64
	var entitiesWithCoverage int
	keystonesWithGaps := 0

	for _, e := range entities {
		// Get metrics for this entity
		m, err := s.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue // Skip entities without metrics
		}

		// Get coverage for this entity
		cov, err := GetEntityCoverage(s, e.ID)
		if err != nil {
			// No coverage data - treat as 0% coverage
			cov = &EntityCoverage{
				EntityID:        e.ID,
				CoveragePercent: 0,
				CoveredLines:    []int{},
				UncoveredLines:  []int{},
			}
		} else {
			entitiesWithCoverage++
			totalCoverage += cov.CoveragePercent
		}

		// Skip entities with sufficient coverage
		if cov.CoveragePercent >= opts.CoverageThreshold {
			continue
		}

		// Check keystone status
		isKeystone := m.PageRank >= cfg.Metrics.KeystoneThreshold

		// Skip if keystones-only and not a keystone
		if opts.KeystonesOnly && !isKeystone {
			continue
		}

		if isKeystone {
			keystonesWithGaps++
		}

		// Calculate priority
		priority, priorityLabel, priorityScore := calculatePriority(m, cov, cfg)

		// Create suggestion
		suggestion := TestSuggestion{
			EntityID:        e.ID,
			EntityName:      e.Name,
			EntityType:      e.EntityType,
			FilePath:        e.FilePath,
			LineStart:       e.LineStart,
			CoveragePercent: cov.CoveragePercent,
			CoveredLines:    len(cov.CoveredLines),
			UncoveredLines:  len(cov.UncoveredLines),
			PageRank:        m.PageRank,
			InDegree:        m.InDegree,
			OutDegree:       m.OutDegree,
			Betweenness:     m.Betweenness,
			IsKeystone:      isKeystone,
			Priority:        priority,
			PriorityLabel:   priorityLabel,
			PriorityScore:   priorityScore,
			Signature:       e.Signature,
		}

		if e.LineEnd != nil {
			suggestion.LineEnd = *e.LineEnd
		}

		// Generate test scenarios from signature if requested
		if opts.IncludeSignatureAnalysis && e.Signature != "" {
			suggestion.TestScenarios = generateTestScenarios(e)
		}

		suggestions = append(suggestions, suggestion)
	}

	// Sort by priority score (higher = more urgent)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].PriorityScore > suggestions[j].PriorityScore
	})

	// Limit results if TopN specified
	if opts.TopN > 0 && len(suggestions) > opts.TopN {
		suggestions = suggestions[:opts.TopN]
	}

	// Group by priority
	byPriority := groupSuggestionsByPriority(suggestions)

	// Calculate average coverage
	var avgCoverage float64
	if entitiesWithCoverage > 0 {
		avgCoverage = totalCoverage / float64(entitiesWithCoverage)
	}

	// Build summary
	summary := SuggestionSummary{
		TotalEntities:     len(entities),
		EntitiesWithGaps:  len(suggestions),
		KeystonesWithGaps: keystonesWithGaps,
		CriticalCount:     len(byPriority["critical"]),
		HighCount:         len(byPriority["high"]),
		MediumCount:       len(byPriority["medium"]),
		LowCount:          len(byPriority["low"]),
		AverageCoverage:   avgCoverage,
		Recommendation:    generateSuggestionRecommendation(byPriority),
	}

	return &SuggestionResult{
		Suggestions: suggestions,
		Summary:     summary,
		ByPriority:  byPriority,
	}, nil
}

// generateSuggestionForEntity creates suggestions for a specific entity.
func generateSuggestionForEntity(s *store.Store, cfg *config.Config, opts SuggestionOptions) (*SuggestionResult, error) {
	// Get the entity
	e, err := s.GetEntity(opts.EntityID)
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", opts.EntityID, err)
	}

	// Get metrics
	m, err := s.GetMetrics(e.ID)
	if err != nil {
		return nil, fmt.Errorf("get metrics for %s: %w", opts.EntityID, err)
	}

	// Get coverage
	cov, err := GetEntityCoverage(s, e.ID)
	if err != nil {
		cov = &EntityCoverage{
			EntityID:        e.ID,
			CoveragePercent: 0,
			CoveredLines:    []int{},
			UncoveredLines:  []int{},
		}
	}

	// Check keystone status
	isKeystone := m.PageRank >= cfg.Metrics.KeystoneThreshold

	// Calculate priority
	priority, priorityLabel, priorityScore := calculatePriority(m, cov, cfg)

	// Create suggestion
	suggestion := TestSuggestion{
		EntityID:        e.ID,
		EntityName:      e.Name,
		EntityType:      e.EntityType,
		FilePath:        e.FilePath,
		LineStart:       e.LineStart,
		CoveragePercent: cov.CoveragePercent,
		CoveredLines:    len(cov.CoveredLines),
		UncoveredLines:  len(cov.UncoveredLines),
		PageRank:        m.PageRank,
		InDegree:        m.InDegree,
		OutDegree:       m.OutDegree,
		Betweenness:     m.Betweenness,
		IsKeystone:      isKeystone,
		Priority:        priority,
		PriorityLabel:   priorityLabel,
		PriorityScore:   priorityScore,
		Signature:       e.Signature,
	}

	if e.LineEnd != nil {
		suggestion.LineEnd = *e.LineEnd
	}

	// Always generate test scenarios for specific entity queries
	suggestion.TestScenarios = generateTestScenarios(e)

	return &SuggestionResult{
		Suggestions: []TestSuggestion{suggestion},
		Summary: SuggestionSummary{
			TotalEntities:    1,
			EntitiesWithGaps: 1,
			AverageCoverage:  cov.CoveragePercent,
		},
	}, nil
}

// calculatePriority determines the priority level based on coverage and importance.
// Priority formula: priority_score = (1 - coverage/100) * pagerank * (1 + log(in_degree+1))
// Returns: priority (0-4), priority_label, priority_score
func calculatePriority(m *store.Metrics, cov *EntityCoverage, cfg *config.Config) (int, string, float64) {
	// Calculate coverage gap factor (0 to 1, higher = less coverage)
	coverageGap := 1.0 - (cov.CoveragePercent / 100.0)

	// Calculate importance factor
	// Use pagerank as primary, with in_degree as secondary factor
	importanceFactor := m.PageRank
	if m.InDegree > 0 {
		// Add logarithmic boost for high in-degree
		importanceFactor *= (1.0 + float64(m.InDegree)/10.0)
	}

	// Calculate priority score
	priorityScore := coverageGap * importanceFactor

	// Determine priority level
	isKeystone := m.PageRank >= cfg.Metrics.KeystoneThreshold

	// Priority assignment logic:
	// - Critical (0): Keystones with <25% coverage
	// - High (1): Keystones with 25-50% coverage OR normal entities with 0% coverage and high in-degree
	// - Medium (2): Normal entities with <50% coverage
	// - Low (3): Entities with 50-75% coverage
	// - Backlog (4): Everything else

	var priority int
	var priorityLabel string

	if isKeystone {
		if cov.CoveragePercent < 25 {
			priority = 0
			priorityLabel = "critical"
		} else if cov.CoveragePercent < 50 {
			priority = 1
			priorityLabel = "high"
		} else {
			priority = 2
			priorityLabel = "medium"
		}
	} else {
		// Not a keystone
		if cov.CoveragePercent == 0 && m.InDegree >= 5 {
			priority = 1
			priorityLabel = "high"
		} else if cov.CoveragePercent < 25 {
			priority = 2
			priorityLabel = "medium"
		} else if cov.CoveragePercent < 50 {
			priority = 3
			priorityLabel = "low"
		} else {
			priority = 4
			priorityLabel = "backlog"
		}
	}

	return priority, priorityLabel, priorityScore
}

// generateTestScenarios analyzes a function signature and suggests test scenarios.
func generateTestScenarios(e *store.Entity) []string {
	var scenarios []string

	sig := e.Signature
	if sig == "" {
		// No signature, provide generic suggestions
		return []string{
			"Test basic functionality",
			"Test with valid input",
			"Test with invalid input",
		}
	}

	// Parse signature to extract parameter and return types
	params, returns := parseSignature(sig)

	// Generate scenarios based on parameter types
	for _, param := range params {
		scenarios = append(scenarios, generateParamScenarios(param)...)
	}

	// Generate scenarios based on return types
	for _, ret := range returns {
		scenarios = append(scenarios, generateReturnScenarios(ret)...)
	}

	// Add common scenarios
	scenarios = append(scenarios, generateCommonScenarios(e)...)

	// Deduplicate scenarios
	return deduplicateStrings(scenarios)
}

// signatureParam represents a parsed parameter from a signature.
type signatureParam struct {
	name     string
	typeName string
}

// parseSignature extracts parameters and return types from a signature string.
// Handles formats like: (email:str,pass:str)->(*User,err) or (ctx context.Context, id string) (User, error)
func parseSignature(sig string) ([]signatureParam, []string) {
	var params []signatureParam
	var returns []string

	// Try to find arrow-style signature first
	if arrowIdx := strings.Index(sig, "->"); arrowIdx != -1 {
		paramPart := sig[:arrowIdx]
		returnPart := sig[arrowIdx+2:]

		params = parseParamList(paramPart)
		returns = parseReturnList(returnPart)
	} else if parenIdx := strings.LastIndex(sig, ")"); parenIdx != -1 {
		// Go-style signature: (params) (returns) or (params) returns
		// Find the first opening paren
		firstParen := strings.Index(sig, "(")
		if firstParen != -1 && firstParen < parenIdx {
			paramPart := sig[firstParen : parenIdx+1]
			params = parseParamList(paramPart)

			// Look for return types after the closing paren
			remaining := strings.TrimSpace(sig[parenIdx+1:])
			if remaining != "" {
				returns = parseReturnList(remaining)
			}
		}
	}

	return params, returns
}

// parseParamList extracts parameters from a parameter list string.
func parseParamList(paramPart string) []signatureParam {
	var params []signatureParam

	// Remove outer parentheses
	paramPart = strings.TrimPrefix(paramPart, "(")
	paramPart = strings.TrimSuffix(paramPart, ")")
	paramPart = strings.TrimSpace(paramPart)

	if paramPart == "" {
		return params
	}

	// Split by comma
	parts := strings.Split(paramPart, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Try to extract name and type
		// Formats: "name:type", "name type", "type"
		var name, typeName string

		if colonIdx := strings.Index(part, ":"); colonIdx != -1 {
			name = strings.TrimSpace(part[:colonIdx])
			typeName = strings.TrimSpace(part[colonIdx+1:])
		} else if spaceIdx := strings.LastIndex(part, " "); spaceIdx != -1 {
			name = strings.TrimSpace(part[:spaceIdx])
			typeName = strings.TrimSpace(part[spaceIdx+1:])
		} else {
			typeName = part
		}

		params = append(params, signatureParam{name: name, typeName: typeName})
	}

	return params
}

// parseReturnList extracts return types from a return specification.
func parseReturnList(returnPart string) []string {
	var returns []string

	// Remove outer parentheses
	returnPart = strings.TrimPrefix(returnPart, "(")
	returnPart = strings.TrimSuffix(returnPart, ")")
	returnPart = strings.TrimSpace(returnPart)

	if returnPart == "" {
		return returns
	}

	// Split by comma
	parts := strings.Split(returnPart, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// If it's "name type" format, extract just the type
		if spaceIdx := strings.LastIndex(part, " "); spaceIdx != -1 {
			part = strings.TrimSpace(part[spaceIdx+1:])
		}
		returns = append(returns, part)
	}

	return returns
}

// generateParamScenarios generates test scenarios based on parameter types.
func generateParamScenarios(param signatureParam) []string {
	var scenarios []string
	typeName := strings.ToLower(param.typeName)
	paramName := param.name
	if paramName == "" {
		paramName = "parameter"
	}

	// String types
	if containsAny(typeName, "string", "str", "text") {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with empty %s", paramName),
			fmt.Sprintf("Test with valid %s", paramName),
			fmt.Sprintf("Test with special characters in %s", paramName),
		)
		// Check for specific string patterns
		if containsAny(strings.ToLower(paramName), "email") {
			scenarios = append(scenarios,
				"Test with invalid email format",
				"Test with valid email address",
			)
		}
		if containsAny(strings.ToLower(paramName), "password", "pass") {
			scenarios = append(scenarios,
				"Test with weak password",
				"Test with strong password",
			)
		}
		if containsAny(strings.ToLower(paramName), "url", "uri", "path") {
			scenarios = append(scenarios,
				"Test with malformed URL",
				"Test with valid URL",
			)
		}
	}

	// Numeric types
	if containsAny(typeName, "int", "int32", "int64", "uint", "float", "number") {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with zero %s", paramName),
			fmt.Sprintf("Test with negative %s", paramName),
			fmt.Sprintf("Test with maximum value for %s", paramName),
		)
	}

	// Boolean types
	if containsAny(typeName, "bool", "boolean") {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with %s=true", paramName),
			fmt.Sprintf("Test with %s=false", paramName),
		)
	}

	// Pointer/nullable types
	if strings.HasPrefix(typeName, "*") || containsAny(typeName, "optional", "nullable") {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with nil %s", paramName),
			fmt.Sprintf("Test with valid %s pointer", paramName),
		)
	}

	// Slice/array types
	if strings.HasPrefix(typeName, "[]") || containsAny(typeName, "slice", "array", "list") {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with empty %s", paramName),
			fmt.Sprintf("Test with single-element %s", paramName),
			fmt.Sprintf("Test with multiple elements in %s", paramName),
		)
	}

	// Map types
	if strings.HasPrefix(typeName, "map") || containsAny(typeName, "dict", "hash") {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with empty %s", paramName),
			fmt.Sprintf("Test with single entry in %s", paramName),
		)
	}

	// Context types
	if containsAny(typeName, "context") {
		scenarios = append(scenarios,
			"Test with canceled context",
			"Test with context timeout",
		)
	}

	// Interface types
	if typeName == "interface{}" || typeName == "any" {
		scenarios = append(scenarios,
			fmt.Sprintf("Test with various types for %s", paramName),
		)
	}

	return scenarios
}

// generateReturnScenarios generates test scenarios based on return types.
func generateReturnScenarios(returnType string) []string {
	var scenarios []string
	typeName := strings.ToLower(returnType)

	// Error returns
	if containsAny(typeName, "error", "err") {
		scenarios = append(scenarios,
			"Test success case (no error)",
			"Test error handling path",
			"Verify error message content",
		)
	}

	// Boolean returns
	if containsAny(typeName, "bool", "boolean") {
		scenarios = append(scenarios,
			"Test condition for true return",
			"Test condition for false return",
		)
	}

	// Pointer returns
	if strings.HasPrefix(returnType, "*") {
		scenarios = append(scenarios,
			"Test case returning valid pointer",
			"Test case returning nil",
		)
	}

	return scenarios
}

// generateCommonScenarios generates common test scenarios based on function characteristics.
func generateCommonScenarios(e *store.Entity) []string {
	var scenarios []string
	name := strings.ToLower(e.Name)

	// CRUD operations
	if containsAny(name, "create", "add", "insert", "new") {
		scenarios = append(scenarios,
			"Test creating with valid data",
			"Test duplicate creation handling",
		)
	}
	if containsAny(name, "get", "find", "fetch", "load", "read") {
		scenarios = append(scenarios,
			"Test finding existing item",
			"Test finding non-existent item",
		)
	}
	if containsAny(name, "update", "modify", "edit", "set") {
		scenarios = append(scenarios,
			"Test updating existing item",
			"Test updating non-existent item",
		)
	}
	if containsAny(name, "delete", "remove", "destroy") {
		scenarios = append(scenarios,
			"Test deleting existing item",
			"Test deleting non-existent item",
			"Test idempotent deletion",
		)
	}

	// Validation operations
	if containsAny(name, "validate", "check", "verify", "is", "has") {
		scenarios = append(scenarios,
			"Test validation with valid input",
			"Test validation with invalid input",
		)
	}

	// Parse/convert operations
	if containsAny(name, "parse", "convert", "transform", "marshal", "unmarshal") {
		scenarios = append(scenarios,
			"Test parsing valid input",
			"Test parsing malformed input",
			"Test parsing edge cases",
		)
	}

	// Init/setup operations
	if containsAny(name, "init", "setup", "configure", "open", "connect") {
		scenarios = append(scenarios,
			"Test initialization with valid config",
			"Test initialization with missing config",
			"Test double initialization handling",
		)
	}

	// Close/cleanup operations
	if containsAny(name, "close", "cleanup", "dispose", "shutdown") {
		scenarios = append(scenarios,
			"Test cleanup after normal operation",
			"Test cleanup on error condition",
			"Test double cleanup handling",
		)
	}

	return scenarios
}

// groupSuggestionsByPriority groups suggestions by their priority label.
func groupSuggestionsByPriority(suggestions []TestSuggestion) map[string][]TestSuggestion {
	result := make(map[string][]TestSuggestion)
	for _, s := range suggestions {
		result[s.PriorityLabel] = append(result[s.PriorityLabel], s)
	}
	return result
}

// generateSuggestionRecommendation creates an overall recommendation based on suggestions.
func generateSuggestionRecommendation(byPriority map[string][]TestSuggestion) string {
	criticalCount := len(byPriority["critical"])
	highCount := len(byPriority["high"])

	if criticalCount > 0 {
		return fmt.Sprintf("URGENT: %d critical gaps in keystones. Add tests before any changes.", criticalCount)
	}
	if highCount > 0 {
		return fmt.Sprintf("HIGH PRIORITY: %d high-priority gaps need attention soon.", highCount)
	}
	if len(byPriority["medium"]) > 0 {
		return "Address medium-priority gaps to improve codebase stability."
	}
	return "Test coverage is in good shape. Continue monitoring."
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// deduplicateStrings removes duplicate strings from a slice.
func deduplicateStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// Regular expressions for signature parsing
var (
	funcNamePattern = regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)`)
)

// ExtractFunctionName extracts the function name from a signature.
func ExtractFunctionName(sig string) string {
	matches := funcNamePattern.FindStringSubmatch(sig)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

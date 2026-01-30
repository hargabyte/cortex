// Package report provides schema types for CX report generation.
//
// This package defines the core data structures used across all report types
// (feature, overview, changes, health). Reports are designed for AI agents
// to consume and transform into narrative-driven documentation.
//
// The types defined here correspond to the data contracts specified in
// docs/specs/CX_REPORT_SPEC.md.
package report

import (
	"fmt"
	"strings"
	"time"
)

// ReportType represents the type of report being generated.
type ReportType string

const (
	// ReportTypeFeature is for deep-dive feature analysis reports.
	// These reports focus on a specific query/feature and include
	// semantic search results, dependencies, and call flows.
	ReportTypeFeature ReportType = "feature"

	// ReportTypeOverview is for system-level summary reports.
	// These reports provide architecture diagrams, keystone identification,
	// and module structure analysis.
	ReportTypeOverview ReportType = "overview"

	// ReportTypeChanges is for historical change analysis reports.
	// These reports use Dolt time-travel to show what changed between
	// two points in time.
	ReportTypeChanges ReportType = "changes"

	// ReportTypeHealth is for risk analysis and recommendations.
	// These reports identify untested keystones, dead code, circular
	// dependencies, and complexity hotspots.
	ReportTypeHealth ReportType = "health"
)

// String returns the string representation of the report type.
func (rt ReportType) String() string {
	return string(rt)
}

// ParseReportType parses a string into a ReportType.
// Returns an error for invalid report type values.
func ParseReportType(s string) (ReportType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "feature":
		return ReportTypeFeature, nil
	case "overview":
		return ReportTypeOverview, nil
	case "changes":
		return ReportTypeChanges, nil
	case "health":
		return ReportTypeHealth, nil
	default:
		return "", fmt.Errorf("invalid report type: %q (expected feature, overview, changes, or health)", s)
	}
}

// ValidateReportType checks if a report type value is valid.
func ValidateReportType(rt ReportType) bool {
	switch rt {
	case ReportTypeFeature, ReportTypeOverview, ReportTypeChanges, ReportTypeHealth:
		return true
	default:
		return false
	}
}

// ReportHeader contains the common header fields for all report types.
// This structure appears at the top of every report output.
type ReportHeader struct {
	// Type identifies the kind of report (feature, overview, changes, health).
	Type ReportType `yaml:"type" json:"type"`

	// GeneratedAt is the timestamp when the report was generated.
	GeneratedAt time.Time `yaml:"generated_at" json:"generated_at"`

	// Query is the search query used for feature reports.
	// Empty for other report types.
	Query string `yaml:"query,omitempty" json:"query,omitempty"`

	// FromRef is the starting git/Dolt reference for change reports.
	// Examples: "HEAD~50", "v1.0", "2026-01-01"
	FromRef string `yaml:"from_ref,omitempty" json:"from_ref,omitempty"`

	// ToRef is the ending git/Dolt reference for change reports.
	// Defaults to "HEAD" if not specified.
	ToRef string `yaml:"to_ref,omitempty" json:"to_ref,omitempty"`
}

// Importance represents the importance classification of an entity.
type Importance string

const (
	// ImportanceKeystone indicates a critical entity with many dependents.
	// These are high-impact entities that should be well-tested.
	ImportanceKeystone Importance = "keystone"

	// ImportanceBottleneck indicates an entity that is frequently used.
	// These are common call targets that affect many code paths.
	ImportanceBottleneck Importance = "bottleneck"

	// ImportanceNormal indicates a standard entity with typical connectivity.
	ImportanceNormal Importance = "normal"

	// ImportanceLeaf indicates an entity with few or no dependents.
	// These are typically helper functions or edge implementations.
	ImportanceLeaf Importance = "leaf"
)

// String returns the string representation of the importance level.
func (i Importance) String() string {
	return string(i)
}

// EntityData contains information about a code entity for reports.
// This structure is used to represent functions, types, methods, etc.
// in report output.
type EntityData struct {
	// ID is the unique entity identifier (e.g., "sa-fn-abc123-45-LoginUser").
	ID string `yaml:"id" json:"id"`

	// Name is the entity name (e.g., "LoginUser", "Store.GetEntity").
	Name string `yaml:"name" json:"name"`

	// Type is the entity type (function, method, type, constant, etc.).
	// Note: YAML/JSON field is "entity_type" to match spec, but Go field
	// is "Type" for clarity.
	Type string `yaml:"entity_type" json:"entity_type"`

	// File is the path to the source file containing this entity.
	File string `yaml:"file" json:"file"`

	// Lines contains the start and end line numbers [start, end].
	Lines [2]int `yaml:"lines" json:"lines"`

	// Signature is the full function/method signature.
	// Empty for non-callable entities.
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`

	// Importance indicates the entity's importance classification.
	Importance Importance `yaml:"importance,omitempty" json:"importance,omitempty"`

	// PageRank is the computed PageRank score for graph importance.
	PageRank float64 `yaml:"pagerank,omitempty" json:"pagerank,omitempty"`

	// Coverage is the test coverage percentage (0-100).
	// -1 indicates coverage data is not available.
	Coverage float64 `yaml:"coverage,omitempty" json:"coverage,omitempty"`

	// DocComment is the documentation comment for this entity.
	DocComment string `yaml:"doc_comment,omitempty" json:"doc_comment,omitempty"`

	// RelevanceScore indicates how relevant this entity is to a search query.
	// Used in feature reports with semantic search. Range: 0.0-1.0.
	RelevanceScore float64 `yaml:"relevance_score,omitempty" json:"relevance_score,omitempty"`

	// InDegree is the number of entities that depend on this one.
	InDegree int `yaml:"in_degree,omitempty" json:"in_degree,omitempty"`

	// Layer is the logical layer this entity belongs to (for playground filtering).
	// Examples: "core", "api", "store", "parser", "test"
	// Only populated when playground mode is enabled.
	Layer string `yaml:"layer,omitempty" json:"layer,omitempty"`

	// CSSClasses contains space-separated CSS classes for SVG filtering.
	// Example: "layer-core entity-function importance-keystone"
	// Only populated when playground mode is enabled.
	CSSClasses string `yaml:"css_classes,omitempty" json:"css_classes,omitempty"`
}

// DependencyData represents a dependency relationship between entities.
type DependencyData struct {
	// From is the source entity name or ID.
	From string `yaml:"from" json:"from"`

	// To is the target entity name or ID.
	To string `yaml:"to" json:"to"`

	// Type is the dependency type (calls, uses_type, implements, extends, imports).
	Type string `yaml:"type" json:"type"`
}

// DependencyType constants for common dependency types.
const (
	DepTypeCalls      = "calls"
	DepTypeUsesType   = "uses_type"
	DepTypeImplements = "implements"
	DepTypeExtends    = "extends"
	DepTypeImports    = "imports"
)

// DiagramData contains D2 diagram information for visualization.
type DiagramData struct {
	// Title is the human-readable title for the diagram.
	Title string `yaml:"title" json:"title"`

	// D2 is the D2 language code for the diagram.
	// This can be rendered to SVG using the D2 CLI.
	D2 string `yaml:"d2" json:"d2"`
}

// MetadataData contains report metadata and statistics.
type MetadataData struct {
	// EntityCount is the number of entities included in the report.
	EntityCount int `yaml:"entity_count,omitempty" json:"entity_count,omitempty"`

	// TotalEntitiesSearched is the total number of entities that were searched.
	// Used in feature reports to show search scope.
	TotalEntitiesSearched int `yaml:"total_entities_searched,omitempty" json:"total_entities_searched,omitempty"`

	// MatchesFound is the number of entities matching the search query.
	// Used in feature reports.
	MatchesFound int `yaml:"matches_found,omitempty" json:"matches_found,omitempty"`

	// LanguageBreakdown maps language names to entity counts.
	// Example: {"go": 800, "typescript": 400}
	LanguageBreakdown map[string]int `yaml:"language_breakdown,omitempty" json:"language_breakdown,omitempty"`

	// CoverageAvailable indicates whether coverage data is available.
	CoverageAvailable bool `yaml:"coverage_available,omitempty" json:"coverage_available,omitempty"`

	// SearchMethod describes how entities were found.
	// Examples: "fts", "embedding", "hybrid"
	SearchMethod string `yaml:"search_method,omitempty" json:"search_method,omitempty"`

	// TotalEntities is the total number of entities in the codebase.
	// Used in overview reports.
	TotalEntities int `yaml:"total_entities,omitempty" json:"total_entities,omitempty"`

	// ActiveEntities is the count of non-archived entities.
	ActiveEntities int `yaml:"active_entities,omitempty" json:"active_entities,omitempty"`

	// ArchivedEntities is the count of archived/deleted entities.
	ArchivedEntities int `yaml:"archived_entities,omitempty" json:"archived_entities,omitempty"`

	// CommitsAnalyzed is the number of commits analyzed (for changes reports).
	CommitsAnalyzed int `yaml:"commits_analyzed,omitempty" json:"commits_analyzed,omitempty"`

	// TimeRange specifies the time range for changes reports.
	TimeRange *TimeRange `yaml:"time_range,omitempty" json:"time_range,omitempty"`
}

// TimeRange specifies a time range for change analysis.
type TimeRange struct {
	// From is the start time of the range.
	From time.Time `yaml:"from" json:"from"`

	// To is the end time of the range.
	To time.Time `yaml:"to" json:"to"`
}

// TestData contains information about a test and what it covers.
type TestData struct {
	// Name is the test function name (e.g., "TestLoginUser_Success").
	Name string `yaml:"name" json:"name"`

	// File is the path to the test file.
	File string `yaml:"file" json:"file"`

	// Lines contains the start and end line numbers [start, end].
	Lines [2]int `yaml:"lines" json:"lines"`

	// Covers lists the entity names this test covers.
	Covers []string `yaml:"covers" json:"covers"`
}

// CoverageData contains test coverage information for a report.
type CoverageData struct {
	// Overall is the overall coverage percentage (0-100).
	Overall float64 `yaml:"overall" json:"overall"`

	// ByEntity maps entity names to their coverage percentages.
	ByEntity map[string]float64 `yaml:"by_entity,omitempty" json:"by_entity,omitempty"`

	// ByImportance maps importance levels to coverage percentages.
	// Example: {"keystone": 85.0, "bottleneck": 72.0, "normal": 78.0, "leaf": 65.0}
	ByImportance map[string]float64 `yaml:"by_importance,omitempty" json:"by_importance,omitempty"`

	// Gaps lists coverage gaps that may represent risk.
	Gaps []CoverageGap `yaml:"gaps,omitempty" json:"gaps,omitempty"`
}

// CoverageGap represents a coverage gap that may be a risk.
type CoverageGap struct {
	// Entity is the name of the entity with low coverage.
	Entity string `yaml:"entity" json:"entity"`

	// Coverage is the current coverage percentage (0-100).
	Coverage float64 `yaml:"coverage" json:"coverage"`

	// Importance is the entity's importance classification.
	Importance Importance `yaml:"importance" json:"importance"`

	// Risk indicates the risk level ("high", "medium", "low").
	Risk string `yaml:"risk" json:"risk"`
}

// RiskLevel constants for coverage gap risk assessment.
const (
	RiskHigh   = "high"
	RiskMedium = "medium"
	RiskLow    = "low"
)

// StatisticsData contains statistical breakdowns for reports.
type StatisticsData struct {
	// ByType maps entity types to counts.
	// Example: {"function": 1500, "method": 800, "type": 400}
	ByType map[string]int `yaml:"by_type,omitempty" json:"by_type,omitempty"`

	// ByLanguage maps languages to entity counts.
	// Example: {"go": 2500, "typescript": 1000}
	ByLanguage map[string]int `yaml:"by_language,omitempty" json:"by_language,omitempty"`

	// Trends contains historical entity count data points.
	Trends []TrendPoint `yaml:"trends,omitempty" json:"trends,omitempty"`

	// Added is the count of added entities (for changes reports).
	Added int `yaml:"added,omitempty" json:"added,omitempty"`

	// Modified is the count of modified entities (for changes reports).
	Modified int `yaml:"modified,omitempty" json:"modified,omitempty"`

	// Deleted is the count of deleted entities (for changes reports).
	Deleted int `yaml:"deleted,omitempty" json:"deleted,omitempty"`
}

// TrendPoint represents a data point in a historical trend.
type TrendPoint struct {
	// Date is the date of the data point (YYYY-MM-DD format).
	Date string `yaml:"date" json:"date"`

	// Entities is the entity count at this date.
	Entities int `yaml:"entities" json:"entities"`
}

// ModuleData contains information about a code module/package.
type ModuleData struct {
	// Path is the module path (e.g., "internal/store").
	Path string `yaml:"path" json:"path"`

	// Entities is the total entity count in this module.
	Entities int `yaml:"entities" json:"entities"`

	// Functions is the function count in this module.
	Functions int `yaml:"functions" json:"functions"`

	// Types is the type count in this module.
	Types int `yaml:"types" json:"types"`

	// Coverage is the module's test coverage percentage (0-100).
	Coverage float64 `yaml:"coverage,omitempty" json:"coverage,omitempty"`
}

// HealthSummary contains overall health indicators for a codebase.
type HealthSummary struct {
	// CoverageOverall is the overall test coverage percentage.
	CoverageOverall float64 `yaml:"coverage_overall" json:"coverage_overall"`

	// UntestedKeystones is the count of keystone entities without tests.
	UntestedKeystones int `yaml:"untested_keystones" json:"untested_keystones"`

	// CircularDependencies is the count of circular dependency cycles.
	CircularDependencies int `yaml:"circular_dependencies" json:"circular_dependencies"`

	// DeadCodeCandidates is the count of potentially unused code.
	DeadCodeCandidates int `yaml:"dead_code_candidates" json:"dead_code_candidates"`
}

// HealthIssue represents a health issue found during analysis.
type HealthIssue struct {
	// Type is the issue type (e.g., "untested_keystone", "circular_dependency").
	Type string `yaml:"type" json:"type"`

	// Entity is the primary entity involved (if applicable).
	Entity string `yaml:"entity,omitempty" json:"entity,omitempty"`

	// Entities is the list of entities involved (for multi-entity issues).
	Entities []string `yaml:"entities,omitempty" json:"entities,omitempty"`

	// File is the file path (if applicable).
	File string `yaml:"file,omitempty" json:"file,omitempty"`

	// PageRank is the entity's PageRank (for importance context).
	PageRank float64 `yaml:"pagerank,omitempty" json:"pagerank,omitempty"`

	// Coverage is the entity's coverage (if relevant).
	Coverage float64 `yaml:"coverage,omitempty" json:"coverage,omitempty"`

	// Importance is the entity's importance classification.
	Importance Importance `yaml:"importance,omitempty" json:"importance,omitempty"`

	// Cycle describes the circular dependency cycle (if applicable).
	Cycle string `yaml:"cycle,omitempty" json:"cycle,omitempty"`

	// InDegree is the entity's in-degree (for dead code detection).
	InDegree int `yaml:"in_degree,omitempty" json:"in_degree,omitempty"`

	// Recommendation is the suggested action to fix this issue.
	Recommendation string `yaml:"recommendation" json:"recommendation"`
}

// DeadCodeGroup groups dead code candidates by module/directory.
type DeadCodeGroup struct {
	// Type is always "dead_code_group" for this issue type.
	Type string `yaml:"type" json:"type"`

	// Module is the directory path containing the dead code.
	Module string `yaml:"module" json:"module"`

	// Count is the number of candidates in this module.
	Count int `yaml:"count" json:"count"`

	// Candidates are the dead code entities in this module.
	Candidates []DeadCodeCandidate `yaml:"candidates" json:"candidates"`
}

// DeadCodeCandidate represents a single dead code candidate.
type DeadCodeCandidate struct {
	// Entity is the entity name.
	Entity string `yaml:"entity" json:"entity"`

	// EntityType is the type (function, method, class, etc.).
	EntityType string `yaml:"entity_type" json:"entity_type"`

	// File is the full file path.
	File string `yaml:"file" json:"file"`

	// Line is the starting line number.
	Line int `yaml:"line" json:"line"`

	// Recommendation is the suggested action.
	Recommendation string `yaml:"recommendation,omitempty" json:"recommendation,omitempty"`
}

// HealthIssueType constants for health report issues.
const (
	IssueTypeUntestedKeystone    = "untested_keystone"
	IssueTypeCircularDependency  = "circular_dependency"
	IssueTypeLowCoverageBottle   = "low_coverage_bottleneck"
	IssueTypeDeadCodeCandidate   = "dead_code_candidate"
	IssueTypeComplexityHotspot   = "complexity_hotspot"
	IssueTypeDeadCodeGroup       = "dead_code_group"
)

// HealthIssues groups health issues by severity.
type HealthIssues struct {
	// Critical contains issues that should be addressed immediately.
	Critical []HealthIssue `yaml:"critical,omitempty" json:"critical,omitempty"`

	// Warning contains issues that should be addressed soon.
	Warning []HealthIssue `yaml:"warning,omitempty" json:"warning,omitempty"`

	// Info contains informational issues for awareness.
	Info []HealthIssue `yaml:"info,omitempty" json:"info,omitempty"`
}

// ComplexityHotspot represents a complexity hotspot in the codebase.
type ComplexityHotspot struct {
	// Entity is the entity name.
	Entity string `yaml:"entity" json:"entity"`

	// OutDegree is the number of dependencies this entity has.
	OutDegree int `yaml:"out_degree" json:"out_degree"`

	// Lines is the number of lines of code.
	Lines int `yaml:"lines" json:"lines"`

	// Cyclomatic is the cyclomatic complexity (if computed).
	Cyclomatic int `yaml:"cyclomatic,omitempty" json:"cyclomatic,omitempty"`
}

// ChangedEntity represents an entity that was added, modified, or deleted.
type ChangedEntity struct {
	// ID is the entity identifier.
	ID string `yaml:"id" json:"id"`

	// Name is the entity name.
	Name string `yaml:"name" json:"name"`

	// Type is the entity type.
	Type string `yaml:"entity_type" json:"entity_type"`

	// File is the current file path (or was_file for deleted entities).
	File string `yaml:"file,omitempty" json:"file,omitempty"`

	// WasFile is the previous file path (for deleted entities).
	WasFile string `yaml:"was_file,omitempty" json:"was_file,omitempty"`

	// Lines contains the line numbers [start, end] for the entity.
	Lines [2]int `yaml:"lines,omitempty" json:"lines,omitempty"`

	// AddedIn is the commit hash where this entity was added.
	AddedIn string `yaml:"added_in,omitempty" json:"added_in,omitempty"`

	// DeletedIn is the commit hash where this entity was deleted.
	DeletedIn string `yaml:"deleted_in,omitempty" json:"deleted_in,omitempty"`

	// ChangeSummary describes what changed (for modified entities).
	ChangeSummary string `yaml:"change_summary,omitempty" json:"change_summary,omitempty"`

	// LinesChanged is the number of lines that changed.
	LinesChanged int `yaml:"lines_changed,omitempty" json:"lines_changed,omitempty"`
}

// ImpactAnalysis contains change impact analysis data.
type ImpactAnalysis struct {
	// HighImpactChanges lists changes that affect many dependents.
	HighImpactChanges []ImpactChange `yaml:"high_impact_changes,omitempty" json:"high_impact_changes,omitempty"`
}

// ImpactChange represents a change and its impact on the codebase.
type ImpactChange struct {
	// Entity is the changed entity name.
	Entity string `yaml:"entity" json:"entity"`

	// DependentsAffected is the count of entities affected by this change.
	DependentsAffected int `yaml:"dependents_affected" json:"dependents_affected"`

	// Risk indicates the risk level of this change.
	Risk string `yaml:"risk" json:"risk"`
}

// PlaygroundMetadata contains metadata for interactive playground generation.
// When playground mode is enabled, reports include this extra data to power
// interactive features like filtering, zooming, and annotation.
type PlaygroundMetadata struct {
	// Enabled indicates whether playground mode is active.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Layers defines the available layers for filtering.
	// Each layer represents a logical grouping (e.g., "core", "api", "test").
	Layers []LayerInfo `yaml:"layers,omitempty" json:"layers,omitempty"`

	// ConnectionTypes defines the available connection types for filtering.
	ConnectionTypes []ConnectionTypeInfo `yaml:"connection_types,omitempty" json:"connection_types,omitempty"`

	// ElementMap maps entity IDs to SVG element IDs for interactivity.
	// Key: entity ID, Value: SVG element ID
	ElementMap map[string]string `yaml:"element_map,omitempty" json:"element_map,omitempty"`

	// PrerenderedSVG contains the pre-rendered SVG diagram for faster loading.
	// Optional - only included when SVG pre-rendering is enabled.
	PrerenderedSVG string `yaml:"prerendered_svg,omitempty" json:"prerendered_svg,omitempty"`

	// ViewPresets defines quick-access view configurations.
	ViewPresets []ViewPreset `yaml:"view_presets,omitempty" json:"view_presets,omitempty"`
}

// LayerInfo describes a filterable layer in the playground.
type LayerInfo struct {
	// ID is the layer identifier (e.g., "core", "api", "test").
	ID string `yaml:"id" json:"id"`

	// Label is the human-readable label for the layer.
	Label string `yaml:"label" json:"label"`

	// Color is the CSS color for this layer (e.g., "#4a90d9").
	Color string `yaml:"color" json:"color"`

	// EntityCount is the number of entities in this layer.
	EntityCount int `yaml:"entity_count" json:"entity_count"`

	// DefaultVisible indicates whether this layer is visible by default.
	DefaultVisible bool `yaml:"default_visible" json:"default_visible"`
}

// ConnectionTypeInfo describes a filterable connection type.
type ConnectionTypeInfo struct {
	// Type is the connection type (e.g., "calls", "uses_type", "implements").
	Type string `yaml:"type" json:"type"`

	// Label is the human-readable label.
	Label string `yaml:"label" json:"label"`

	// Color is the CSS color for this connection type.
	Color string `yaml:"color" json:"color"`

	// Count is the number of connections of this type.
	Count int `yaml:"count" json:"count"`

	// DefaultVisible indicates whether this type is visible by default.
	DefaultVisible bool `yaml:"default_visible" json:"default_visible"`
}

// ViewPreset defines a quick-access view configuration for playgrounds.
type ViewPreset struct {
	// ID is the preset identifier.
	ID string `yaml:"id" json:"id"`

	// Label is the human-readable label.
	Label string `yaml:"label" json:"label"`

	// Description explains what this preset shows.
	Description string `yaml:"description" json:"description"`

	// VisibleLayers lists the layer IDs that should be visible.
	VisibleLayers []string `yaml:"visible_layers" json:"visible_layers"`

	// VisibleConnections lists the connection types that should be visible.
	VisibleConnections []string `yaml:"visible_connections" json:"visible_connections"`

	// ImportanceFilter limits to entities of certain importance levels.
	ImportanceFilter []string `yaml:"importance_filter,omitempty" json:"importance_filter,omitempty"`
}

// Layer constants for common layers derived from module paths.
const (
	LayerCore     = "core"
	LayerAPI      = "api"
	LayerStore    = "store"
	LayerParser   = "parser"
	LayerGraph    = "graph"
	LayerOutput   = "output"
	LayerTest     = "test"
	LayerExternal = "external"
)

// Package output provides YAML/JSON output schema for CX 2.0.
//
// This package defines the structured output types that replace the
// compact CGF format with self-documenting YAML output.
package output

// EntityOutput represents a single entity output for cx show.
// The entity name becomes the top-level YAML key.
type EntityOutput struct {
	// Type is the entity type: function, method, struct, interface, etc.
	Type string `yaml:"type" json:"type"`

	// Location is the file path and line range in format: path:start-end
	// Example: "internal/auth/login.go:45-89"
	Location string `yaml:"location" json:"location"`

	// Signature is the function/method signature with full type names
	// Example: "(email: string, password: string) -> (*User, error)"
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`

	// Receiver is the method receiver type (nil for functions)
	// Example: "*AuthService" or null
	Receiver *string `yaml:"receiver,omitempty" json:"receiver,omitempty"`

	// Visibility is the entity visibility: public or private
	Visibility string `yaml:"visibility,omitempty" json:"visibility,omitempty"`

	// Fields contains struct/interface fields (for type entities)
	// Format: map of field name to type
	Fields map[string]string `yaml:"fields,omitempty" json:"fields,omitempty"`

	// Methods contains method names (for struct/interface entities)
	// Example: ["CreateUser", "GetUser", "UpdateUser"]
	Methods []string `yaml:"methods,omitempty" json:"methods,omitempty"`

	// Implements contains interface names this type implements
	// Example: ["UserManager", "io.Closer"]
	Implements []string `yaml:"implements,omitempty" json:"implements,omitempty"`

	// ImplementedBy contains types that implement this interface
	// Example: ["SQLUserRepository", "MockUserRepository"]
	ImplementedBy []string `yaml:"implemented_by,omitempty" json:"implemented_by,omitempty"`

	// Dependencies contains relationship information
	Dependencies *Dependencies `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// Metrics contains computed graph metrics
	Metrics *Metrics `yaml:"metrics,omitempty" json:"metrics,omitempty"`

	// Hashes contains signature and body hashes (dense mode only)
	Hashes *Hashes `yaml:"hashes,omitempty" json:"hashes,omitempty"`

	// Timestamps contains creation/update times (dense mode only)
	Timestamps *Timestamps `yaml:"timestamps,omitempty" json:"timestamps,omitempty"`

	// Coverage contains test coverage information (when --coverage flag is used)
	Coverage *Coverage `yaml:"coverage,omitempty" json:"coverage,omitempty"`

	// Tags contains entity tags/bookmarks
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// ChangeStatus indicates how the entity changed relative to a ref (--since flag)
	// Values: "added", "modified", "unchanged", or empty if --since not used
	ChangeStatus string `yaml:"change_status,omitempty" json:"change_status,omitempty"`

	// History contains commit history for the entity (when --history flag is used)
	History []*HistoryEntry `yaml:"history,omitempty" json:"history,omitempty"`
}

// HistoryEntry represents a single commit in an entity's history.
type HistoryEntry struct {
	// Commit is the short commit hash
	Commit string `yaml:"commit" json:"commit"`

	// Date is the commit date
	Date string `yaml:"date" json:"date"`

	// Committer is who made the commit
	Committer string `yaml:"committer,omitempty" json:"committer,omitempty"`

	// ChangeType is how the entity changed: "added", "modified", "current", "unchanged"
	ChangeType string `yaml:"change_type" json:"change_type"`

	// Location is the file:line-line at this commit
	Location string `yaml:"location" json:"location"`

	// Signature is the entity signature at this commit (optional)
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`
}

// Dependencies represents entity relationships and edges.
type Dependencies struct {
	// Calls lists entities that this entity calls (outgoing edges)
	// Example: ["ValidateEmail", "HashPassword", "userRepo.Create"]
	Calls []string `yaml:"calls,omitempty" json:"calls,omitempty"`

	// CalledBy lists entities that call this entity (incoming edges)
	// Example: ["HandleLogin", "HandleRegister"]
	CalledBy []CalledByEntry `yaml:"called_by,omitempty" json:"called_by,omitempty"`

	// UsesTypes lists type entities this entity references
	// Example: ["User", "AuthError"]
	UsesTypes []string `yaml:"uses_types,omitempty" json:"uses_types,omitempty"`
}

// CalledByEntry represents an incoming call edge with optional context.
// In sparse/medium modes, this is just a string in the YAML.
// In dense mode, it includes location and "why" context.
type CalledByEntry struct {
	// Name is the calling entity name
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Location is the caller location (dense mode only)
	// Example: "function @ handlers/auth.go:50"
	Location string `yaml:"location,omitempty" json:"location,omitempty"`

	// Why is the relationship context (dense mode only)
	// Example: "Entry point for user authentication"
	Why string `yaml:"why,omitempty" json:"why,omitempty"`
}

// Metrics represents computed graph metrics for importance ranking.
type Metrics struct {
	// PageRank is the PageRank score (0.0 to 1.0)
	PageRank float64 `yaml:"pagerank,omitempty" json:"pagerank,omitempty"`

	// InDegree is the number of incoming edges (how many call this)
	InDegree int `yaml:"in_degree,omitempty" json:"in_degree,omitempty"`

	// OutDegree is the number of outgoing edges (how many this calls)
	OutDegree int `yaml:"out_degree,omitempty" json:"out_degree,omitempty"`

	// Importance is the computed importance level based on metrics
	// Values: keystone, bottleneck, normal, leaf
	Importance string `yaml:"importance,omitempty" json:"importance,omitempty"`

	// Betweenness is the betweenness centrality score (0.0 to 1.0)
	Betweenness float64 `yaml:"betweenness,omitempty" json:"betweenness,omitempty"`
}

// Hashes contains signature and body content hashes.
type Hashes struct {
	// Signature is the 8-char signature hash
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`

	// Body is the 8-char body hash (for functions)
	Body string `yaml:"body,omitempty" json:"body,omitempty"`
}

// Timestamps contains creation and modification times.
type Timestamps struct {
	// Created is the entity creation timestamp
	Created string `yaml:"created,omitempty" json:"created,omitempty"`

	// Updated is the entity last update timestamp
	Updated string `yaml:"updated,omitempty" json:"updated,omitempty"`
}

// Coverage represents test coverage information for an entity.
type Coverage struct {
	// Tested indicates whether the entity has any test coverage
	Tested bool `yaml:"tested" json:"tested"`

	// Percent is the coverage percentage (0-100)
	Percent float64 `yaml:"percent,omitempty" json:"percent,omitempty"`

	// TestedBy maps test names to their coverage details
	TestedBy map[string]*TestEntry `yaml:"tested_by,omitempty" json:"tested_by,omitempty"`

	// UncoveredLines lists line numbers that are not covered
	UncoveredLines []int `yaml:"uncovered_lines,omitempty" json:"uncovered_lines,omitempty"`

	// UncoveredReason provides context about what uncovered code does (optional)
	UncoveredReason string `yaml:"uncovered_reason,omitempty" json:"uncovered_reason,omitempty"`
}

// TestEntry represents a test that covers an entity.
type TestEntry struct {
	// Location is the test file and line range
	Location string `yaml:"location,omitempty" json:"location,omitempty"`

	// CoversLines is the list of line ranges covered by this test
	// Format: [[45, 67], [75, 80]]
	CoversLines [][]int `yaml:"covers_lines,omitempty" json:"covers_lines,omitempty"`
}

// ListOutput represents list results for cx find, cx rank.
// Results are a map where entity names are keys.
type ListOutput struct {
	// Results contains entity outputs keyed by entity name
	// This enables direct lookup: "I need LoginUser" -> scan for that key
	Results map[string]*EntityOutput `yaml:"results" json:"results"`

	// Count is the total number of results
	Count int `yaml:"count" json:"count"`
}

// GraphOutput represents graph traversal results for cx graph.
type GraphOutput struct {
	// Graph contains metadata about the graph query
	Graph *GraphMetadata `yaml:"graph" json:"graph"`

	// Nodes contains entity information for graph nodes
	Nodes map[string]*GraphNode `yaml:"nodes" json:"nodes"`

	// Edges contains the graph edges as tuples [from, to, type]
	Edges [][]string `yaml:"edges" json:"edges"`
}

// GraphMetadata contains metadata about a graph query.
type GraphMetadata struct {
	// Root is the starting entity name
	Root string `yaml:"root" json:"root"`

	// Direction is the traversal direction: in, out, or both
	Direction string `yaml:"direction" json:"direction"`

	// Depth is the maximum traversal depth
	Depth int `yaml:"depth" json:"depth"`
}

// GraphNode represents a node in a graph traversal.
type GraphNode struct {
	// Type is the entity type
	Type string `yaml:"type" json:"type"`

	// Location is the file:line-line location
	Location string `yaml:"location" json:"location"`

	// Depth is the distance from the root node
	Depth int `yaml:"depth" json:"depth"`

	// Signature is the function signature (optional)
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`
}

// ImpactOutput represents impact analysis results for cx impact.
type ImpactOutput struct {
	// Impact contains metadata about the impact query
	Impact *ImpactMetadata `yaml:"impact" json:"impact"`

	// Summary contains high-level impact statistics
	Summary *ImpactSummary `yaml:"summary" json:"summary"`

	// Affected contains entities affected by the change
	Affected map[string]*AffectedEntity `yaml:"affected" json:"affected"`

	// Recommendations contains suggested actions
	Recommendations []string `yaml:"recommendations,omitempty" json:"recommendations,omitempty"`
}

// ImpactMetadata contains metadata about an impact query.
type ImpactMetadata struct {
	// Target is the changed file/entity
	Target string `yaml:"target" json:"target"`

	// Depth is the analysis depth
	Depth int `yaml:"depth" json:"depth"`
}

// ImpactSummary contains high-level impact statistics.
type ImpactSummary struct {
	// FilesAffected is the count of affected files
	FilesAffected int `yaml:"files_affected" json:"files_affected"`

	// EntitiesAffected is the count of affected entities
	EntitiesAffected int `yaml:"entities_affected" json:"entities_affected"`

	// RiskLevel is the overall risk assessment: low, medium, high
	RiskLevel string `yaml:"risk_level" json:"risk_level"`
}

// AffectedEntity represents an entity affected by a change.
type AffectedEntity struct {
	// Type is the entity type
	Type string `yaml:"type" json:"type"`

	// Location is the file:line-line location
	Location string `yaml:"location" json:"location"`

	// Impact describes the type of impact: direct, caller, indirect
	Impact string `yaml:"impact" json:"impact"`

	// Importance is the entity importance level (if significant)
	Importance string `yaml:"importance,omitempty" json:"importance,omitempty"`

	// Reason explains why this entity is affected
	Reason string `yaml:"reason" json:"reason"`
}

// ContextOutput represents context assembly results for cx context.
type ContextOutput struct {
	// Context contains metadata about the context query
	Context *ContextMetadata `yaml:"context" json:"context"`

	// EntryPoints contains discovered entry points for the task
	EntryPoints map[string]*EntryPoint `yaml:"entry_points,omitempty" json:"entry_points,omitempty"`

	// Relevant contains entities relevant to the task
	Relevant map[string]*RelevantEntity `yaml:"relevant" json:"relevant"`

	// Excluded contains entities excluded due to budget/relevance
	Excluded map[string]string `yaml:"excluded,omitempty" json:"excluded,omitempty"`
}

// ContextMetadata contains metadata about a context query.
type ContextMetadata struct {
	// Target is the task description
	Target string `yaml:"target" json:"target"`

	// Budget is the token budget limit
	Budget int `yaml:"budget" json:"budget"`

	// TokensUsed is the actual tokens used
	TokensUsed int `yaml:"tokens_used" json:"tokens_used"`
}

// EntryPoint represents an entry point for a task.
type EntryPoint struct {
	// Type is the entity type
	Type string `yaml:"type" json:"type"`

	// Location is the file:line-line location
	Location string `yaml:"location" json:"location"`

	// Note is additional context about this entry point
	Note string `yaml:"note,omitempty" json:"note,omitempty"`
}

// RelevantEntity represents an entity relevant to a task.
type RelevantEntity struct {
	// Type is the entity type
	Type string `yaml:"type" json:"type"`

	// Location is the file:line-line location
	Location string `yaml:"location" json:"location"`

	// Relevance is the relevance level: high, medium, low
	Relevance string `yaml:"relevance" json:"relevance"`

	// Reason explains why this entity is relevant
	Reason string `yaml:"reason" json:"reason"`

	// Code contains the entity code (skeleton or full based on relevance)
	Code string `yaml:"code,omitempty" json:"code,omitempty"`

	// Coverage is the test coverage percentage (when --with-coverage is used)
	Coverage string `yaml:"coverage,omitempty" json:"coverage,omitempty"`

	// CoverageWarning is a warning about low coverage on keystones (when --with-coverage is used)
	CoverageWarning string `yaml:"coverage_warning,omitempty" json:"coverage_warning,omitempty"`
}

// NearOutput represents neighborhood exploration results for cx near.
type NearOutput struct {
	// Center contains the focal entity information
	Center *NearCenterEntity `yaml:"center" json:"center"`

	// Neighborhood contains related entities grouped by relationship type
	Neighborhood *Neighborhood `yaml:"neighborhood" json:"neighborhood"`
}

// NearCenterEntity represents the center entity in a neighborhood query.
type NearCenterEntity struct {
	// Name is the entity name
	Name string `yaml:"name" json:"name"`

	// Type is the entity type: function, struct, interface, etc.
	Type string `yaml:"type" json:"type"`

	// Location is the file path and line range
	Location string `yaml:"location" json:"location"`

	// Signature is the function/method signature (optional)
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`

	// Visibility is public or private
	Visibility string `yaml:"visibility,omitempty" json:"visibility,omitempty"`
}

// Neighborhood contains entities grouped by relationship type.
type Neighborhood struct {
	// Calls lists entities that the center entity calls (outgoing)
	Calls []*NeighborEntity `yaml:"calls,omitempty" json:"calls,omitempty"`

	// CalledBy lists entities that call the center entity (incoming)
	CalledBy []*NeighborEntity `yaml:"called_by,omitempty" json:"called_by,omitempty"`

	// SameFile lists other entities in the same file
	SameFile []*NeighborEntity `yaml:"same_file,omitempty" json:"same_file,omitempty"`

	// UsesTypes lists types that the center entity uses
	UsesTypes []*NeighborEntity `yaml:"uses_types,omitempty" json:"uses_types,omitempty"`

	// UsedByTypes lists entities that use the center type (for type entities)
	UsedByTypes []*NeighborEntity `yaml:"used_by_types,omitempty" json:"used_by_types,omitempty"`
}

// NeighborEntity represents an entity in the neighborhood.
type NeighborEntity struct {
	// Name is the entity name
	Name string `yaml:"name" json:"name"`

	// Type is the entity type
	Type string `yaml:"type" json:"type"`

	// Location is the file:line-line location
	Location string `yaml:"location" json:"location"`

	// Signature is the function signature (optional, for medium/dense)
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`

	// Depth is the hop distance from center (for multi-hop traversal)
	Depth int `yaml:"depth,omitempty" json:"depth,omitempty"`
}

// TraceOutput represents call chain trace results for cx trace.
type TraceOutput struct {
	// Trace contains metadata about the trace query
	Trace *TraceMetadata `yaml:"trace" json:"trace"`

	// Path contains the shortest path between entities (if found)
	Path []*TracePathNode `yaml:"path,omitempty" json:"path,omitempty"`

	// AllPaths contains all paths found (when --all is specified)
	AllPaths []TracePathList `yaml:"all_paths,omitempty" json:"all_paths,omitempty"`

	// Callers contains upstream caller chains (when --callers is specified)
	Callers []*TracePathNode `yaml:"callers,omitempty" json:"callers,omitempty"`

	// Callees contains downstream callee chains (when --callees is specified)
	Callees []*TracePathNode `yaml:"callees,omitempty" json:"callees,omitempty"`
}

// TraceMetadata contains metadata about a trace query.
type TraceMetadata struct {
	// From is the starting entity name
	From string `yaml:"from,omitempty" json:"from,omitempty"`

	// To is the ending entity name
	To string `yaml:"to,omitempty" json:"to,omitempty"`

	// Target is the entity being traced (for --callers/--callees)
	Target string `yaml:"target,omitempty" json:"target,omitempty"`

	// Mode is the trace mode: path, callers, or callees
	Mode string `yaml:"mode" json:"mode"`

	// Depth is the maximum trace depth
	Depth int `yaml:"depth" json:"depth"`

	// PathFound indicates whether a path was found between entities
	PathFound bool `yaml:"path_found,omitempty" json:"path_found,omitempty"`

	// PathCount is the number of paths found (for --all)
	PathCount int `yaml:"path_count,omitempty" json:"path_count,omitempty"`
}

// TracePathNode represents a node in a trace path.
type TracePathNode struct {
	// Name is the entity name
	Name string `yaml:"name" json:"name"`

	// Type is the entity type
	Type string `yaml:"type" json:"type"`

	// Location is the file:line-line location
	Location string `yaml:"location" json:"location"`

	// Signature is the function signature (optional)
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`

	// Depth is the hop distance from the starting entity
	Depth int `yaml:"depth,omitempty" json:"depth,omitempty"`
}

// TracePathList represents a single path in the all_paths list.
type TracePathList struct {
	// Length is the number of hops in this path
	Length int `yaml:"length" json:"length"`

	// Nodes contains the entities in this path
	Nodes []string `yaml:"nodes" json:"nodes"`
}

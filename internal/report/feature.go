// Package report provides schema types for CX report generation.
package report

import "time"

// FeatureReportData is the complete data structure for feature reports.
//
// Feature reports provide deep-dive analysis of specific features identified
// through semantic search. They answer questions like "how does authentication
// work?" or "what's involved in payment processing?" by gathering all related
// entities, their dependencies, test coverage, and call flows.
//
// The report structure is designed for AI agents to consume and transform into
// narrative-driven documentation and visualizations.
//
// Typical usage:
//
//	report := NewFeatureReport("authentication")
//	report.Report.Query = "authentication"
//	report.Entities = [...EntityData...]
//	report.Dependencies = [...DependencyData...]
//	report.Diagrams = map[string]DiagramData{...}
type FeatureReportData struct {
	// Report contains the report header with type, timestamp, and query.
	// The Type field will always be ReportTypeFeature for feature reports.
	Report ReportHeader `yaml:"report" json:"report"`

	// Metadata contains statistics about the search and entities found.
	// This includes total entities searched, number of matches, language breakdown,
	// and search method (FTS, embedding, or hybrid).
	Metadata MetadataData `yaml:"metadata" json:"metadata"`

	// Entities is the list of code entities relevant to the feature.
	// These are ordered by relevance score (highest first) and include information
	// about each entity's importance, coverage, dependencies, and documentation.
	Entities []EntityData `yaml:"entities" json:"entities"`

	// Dependencies is the list of dependency relationships between entities.
	// These define the call graph, type usage, and implementation relationships
	// that constitute the feature's internal structure.
	Dependencies []DependencyData `yaml:"dependencies" json:"dependencies"`

	// Diagrams contains D2 diagrams for visualization.
	// Common diagram types for feature reports:
	// - "call_flow": Shows the main call sequence through the feature
	// - "data_flow": Illustrates how data moves through the feature
	// - "dependency_graph": Complete dependency graph visualization
	//
	// Keys are diagram names, values are DiagramData with title and D2 code.
	Diagrams map[string]DiagramData `yaml:"diagrams" json:"diagrams"`

	// Tests contains information about tests that cover this feature.
	// This is optional and only populated if test data is available.
	Tests []TestData `yaml:"tests,omitempty" json:"tests,omitempty"`

	// Coverage contains test coverage statistics for the feature.
	// This is optional and only populated if coverage data is available.
	// It includes overall coverage, per-entity coverage, and identified gaps.
	Coverage *CoverageData `yaml:"coverage,omitempty" json:"coverage,omitempty"`
}

// NewFeatureReport creates a new FeatureReportData initialized with the query.
//
// The constructor initializes the report with:
// - Report type set to ReportTypeFeature
// - Current timestamp in UTC
// - Empty slices for Entities and Dependencies
// - Empty map for Diagrams
//
// The caller should populate the other fields (Entities, Dependencies, etc.)
// based on the semantic search results and analysis.
func NewFeatureReport(query string) *FeatureReportData {
	return &FeatureReportData{
		Report: ReportHeader{
			Type:        ReportTypeFeature,
			GeneratedAt: time.Now().UTC(),
			Query:       query,
		},
		Metadata:     MetadataData{},
		Entities:     []EntityData{},
		Dependencies: []DependencyData{},
		Diagrams:     make(map[string]DiagramData),
	}
}

// EntityCount returns the number of entities in the report.
// This is a convenience method for accessing Metadata.EntityCount.
func (f *FeatureReportData) EntityCount() int {
	return len(f.Entities)
}

// DependencyCount returns the number of dependencies in the report.
// This is a convenience method for quick access to dependency statistics.
func (f *FeatureReportData) DependencyCount() int {
	return len(f.Dependencies)
}

// DiagramCount returns the number of diagrams in the report.
// This is a convenience method for quick access to diagram statistics.
func (f *FeatureReportData) DiagramCount() int {
	return len(f.Diagrams)
}

// TestCount returns the number of tests in the report.
// This is a convenience method for quick access to test statistics.
func (f *FeatureReportData) TestCount() int {
	return len(f.Tests)
}

// GetDiagram retrieves a diagram by name.
// Returns the DiagramData and true if found, or zero value and false if not found.
func (f *FeatureReportData) GetDiagram(name string) (DiagramData, bool) {
	diagram, ok := f.Diagrams[name]
	return diagram, ok
}

// AddDiagram adds or updates a diagram in the report.
// If a diagram with the same name already exists, it will be replaced.
func (f *FeatureReportData) AddDiagram(name string, diagram DiagramData) {
	if f.Diagrams == nil {
		f.Diagrams = make(map[string]DiagramData)
	}
	f.Diagrams[name] = diagram
}

// SetCoverage sets the coverage data for the feature report.
// This is a convenience method that ensures the Coverage pointer is properly initialized.
func (f *FeatureReportData) SetCoverage(coverage *CoverageData) {
	f.Coverage = coverage
}

// HasCoverage returns true if coverage data is available in the report.
func (f *FeatureReportData) HasCoverage() bool {
	return f.Coverage != nil
}

// GetOverallCoverage returns the overall coverage percentage if available.
// Returns -1 if coverage data is not available.
func (f *FeatureReportData) GetOverallCoverage() float64 {
	if f.Coverage == nil {
		return -1
	}
	return f.Coverage.Overall
}

// GetKeystoneEntities returns all entities with keystone importance level.
// This is a convenience method for filtering important entities.
func (f *FeatureReportData) GetKeystoneEntities() []EntityData {
	var keystones []EntityData
	for _, entity := range f.Entities {
		if entity.Importance == ImportanceKeystone {
			keystones = append(keystones, entity)
		}
	}
	return keystones
}

// GetBottleneckEntities returns all entities with bottleneck importance level.
// This is a convenience method for identifying frequently-used code.
func (f *FeatureReportData) GetBottleneckEntities() []EntityData {
	var bottlenecks []EntityData
	for _, entity := range f.Entities {
		if entity.Importance == ImportanceBottleneck {
			bottlenecks = append(bottlenecks, entity)
		}
	}
	return bottlenecks
}

// GetLowCoverageEntities returns entities with coverage below the threshold.
// This is useful for identifying testing gaps in important code.
//
// Pass a threshold like 80.0 to find entities with less than 80% coverage.
// Returns an empty slice if coverage data is not available.
func (f *FeatureReportData) GetLowCoverageEntities(threshold float64) []EntityData {
	var lowCoverage []EntityData
	for _, entity := range f.Entities {
		if entity.Coverage >= 0 && entity.Coverage < threshold {
			lowCoverage = append(lowCoverage, entity)
		}
	}
	return lowCoverage
}

// GetEntitiesByFile groups entities by their source file.
// Returns a map from file path to slice of entities in that file.
func (f *FeatureReportData) GetEntitiesByFile() map[string][]EntityData {
	fileMap := make(map[string][]EntityData)
	for _, entity := range f.Entities {
		fileMap[entity.File] = append(fileMap[entity.File], entity)
	}
	return fileMap
}

// GetDependenciesForEntity returns all dependencies where the given entity is the source.
// This shows what this entity calls or depends on.
func (f *FeatureReportData) GetDependenciesForEntity(entityName string) []DependencyData {
	var deps []DependencyData
	for _, dep := range f.Dependencies {
		if dep.From == entityName {
			deps = append(deps, dep)
		}
	}
	return deps
}

// GetDependentsOfEntity returns all dependencies where the given entity is the target.
// This shows what entities depend on or call the given entity.
func (f *FeatureReportData) GetDependentsOfEntity(entityName string) []DependencyData {
	var deps []DependencyData
	for _, dep := range f.Dependencies {
		if dep.To == entityName {
			deps = append(deps, dep)
		}
	}
	return deps
}

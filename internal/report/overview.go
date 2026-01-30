package report

import "time"

// OverviewReportData is the complete data structure for overview reports.
//
// Overview reports provide system-level summaries with architecture diagrams.
// They include:
// - Report metadata (type, generation timestamp)
// - System-wide statistics (entity counts, language breakdown, trends)
// - Keystone entities (high-importance code)
// - Module structure (package organization and metrics)
// - Architecture diagrams (D2 visualizations)
// - Health indicators (coverage, untested keystones, issues)
//
// This report type is designed for stakeholder communication and
// architectural documentation.
type OverviewReportData struct {
	// Report contains the report header with type and generation timestamp.
	Report ReportHeader `yaml:"report" json:"report"`

	// Metadata contains entity and coverage information for the overview.
	Metadata MetadataData `yaml:"metadata" json:"metadata"`

	// Statistics contains breakdowns of entities by type and language,
	// as well as historical trends if available.
	Statistics StatisticsData `yaml:"statistics" json:"statistics"`

	// Keystones lists the most important entities in the codebase.
	// These are entities with high PageRank, many dependents, or both.
	Keystones []EntityData `yaml:"keystones" json:"keystones"`

	// Modules lists the code modules/packages in the system with their
	// structure and metrics.
	Modules []ModuleData `yaml:"modules" json:"modules"`

	// Diagrams contains D2 visualizations of the system architecture.
	// Common diagrams include "architecture" showing module relationships.
	Diagrams map[string]DiagramData `yaml:"diagrams" json:"diagrams"`

	// Health contains overall health indicators and risk metrics.
	// Optional - only included if health analysis was performed.
	Health *HealthSummary `yaml:"health,omitempty" json:"health,omitempty"`

	// Playground contains metadata for interactive playground generation.
	// Only populated when --playground flag is used.
	Playground *PlaygroundMetadata `yaml:"playground,omitempty" json:"playground,omitempty"`
}

// NewOverviewReport creates a new OverviewReportData with default values.
//
// The report is initialized with:
// - Report type set to ReportTypeOverview
// - Current timestamp as generation time
// - Empty slices and maps for data (caller fills these in)
// - No health summary (optional, set separately if needed)
func NewOverviewReport() *OverviewReportData {
	return &OverviewReportData{
		Report: ReportHeader{
			Type:        ReportTypeOverview,
			GeneratedAt: time.Now(),
		},
		Metadata:   MetadataData{},
		Statistics: StatisticsData{},
		Keystones:  make([]EntityData, 0),
		Modules:    make([]ModuleData, 0),
		Diagrams:   make(map[string]DiagramData),
		Health:     nil,
	}
}

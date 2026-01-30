// Package report provides schema types for CX report generation.
package report

import "time"

// ChangesReportData is the complete data structure for changes reports.
//
// Changes reports show what evolved between two points in time using Dolt time-travel.
// They track added, modified, and deleted entities, providing before/after architecture
// snapshots and impact analysis on dependent code.
//
// This structure is used to output structured data (YAML/JSON) that AI agents can
// consume to generate narrative-driven change reports and documentation.
//
// Example usage:
//
//	report := NewChangesReport("HEAD~50", "HEAD")
//	report.Report = ReportHeader{
//		Type: ReportTypeChanges,
//		FromRef: "HEAD~50",
//		ToRef: "HEAD",
//	}
//	report.Metadata = MetadataData{
//		CommitsAnalyzed: 50,
//		TimeRange: &TimeRange{...},
//	}
//	report.Statistics = StatisticsData{
//		Added: 45,
//		Modified: 89,
//		Deleted: 12,
//	}
//	report.AddedEntities = []ChangedEntity{...}
//	report.ModifiedEntities = []ChangedEntity{...}
//	report.DeletedEntities = []ChangedEntity{...}
//	// Output as YAML or JSON
type ChangesReportData struct {
	// Report contains the report header with type, timestamp, and refs.
	Report ReportHeader `yaml:"report" json:"report"`

	// Metadata contains report metadata (commits analyzed, time range, etc.).
	Metadata MetadataData `yaml:"metadata" json:"metadata"`

	// Statistics contains counts of added, modified, and deleted entities.
	Statistics StatisticsData `yaml:"statistics" json:"statistics"`

	// AddedEntities lists entities that were added between the two refs.
	AddedEntities []ChangedEntity `yaml:"added_entities" json:"added_entities"`

	// ModifiedEntities lists entities that were modified between the two refs.
	ModifiedEntities []ChangedEntity `yaml:"modified_entities" json:"modified_entities"`

	// DeletedEntities lists entities that were deleted between the two refs.
	DeletedEntities []ChangedEntity `yaml:"deleted_entities" json:"deleted_entities"`

	// Diagrams maps diagram names to their D2 code and metadata.
	// Common diagrams for changes reports:
	// - "architecture_before": System architecture at fromRef
	// - "architecture_after": System architecture at toRef
	// - "changes_summary": Visual representation of changes
	Diagrams map[string]DiagramData `yaml:"diagrams" json:"diagrams"`

	// Impact contains impact analysis showing high-impact changes and their effects
	// on dependent code. This field is optional and may be nil.
	Impact *ImpactAnalysis `yaml:"impact,omitempty" json:"impact,omitempty"`

	// Playground contains metadata for interactive playground generation.
	// Only populated when --playground flag is used.
	Playground *PlaygroundMetadata `yaml:"playground,omitempty" json:"playground,omitempty"`
}

// NewChangesReport creates a new ChangesReportData for a changes report.
//
// This constructor initializes a changes report with the provided from and to refs,
// setting up empty slices for entities and an empty diagrams map. The caller
// should populate the remaining fields as needed.
//
// Parameters:
// - fromRef: The starting git/Dolt reference (e.g., "HEAD~50", "v1.0", "2026-01-01")
// - toRef: The ending git/Dolt reference (typically "HEAD" or another ref)
//
// Returns a pointer to an initialized ChangesReportData with:
// - Report.Type set to ReportTypeChanges
// - Report.FromRef and Report.ToRef set to provided values
// - Report.GeneratedAt set to current time
// - Empty slices for AddedEntities, ModifiedEntities, DeletedEntities
// - Empty map for Diagrams
// - Other fields ready for population
//
// Example:
//
//	report := NewChangesReport("HEAD~50", "HEAD")
//	report.Statistics = StatisticsData{Added: 45, Modified: 89, Deleted: 12}
//	// ... populate other fields
func NewChangesReport(fromRef, toRef string) *ChangesReportData {
	return &ChangesReportData{
		Report: ReportHeader{
			Type:        ReportTypeChanges,
			GeneratedAt: zeroTime(), // Will be set by caller
			FromRef:     fromRef,
			ToRef:       toRef,
		},
		Metadata:         MetadataData{},
		Statistics:       StatisticsData{},
		AddedEntities:    []ChangedEntity{},
		ModifiedEntities: []ChangedEntity{},
		DeletedEntities:  []ChangedEntity{},
		Diagrams:         make(map[string]DiagramData),
	}
}

// zeroTime returns the zero time value.
// This is used as a placeholder in NewChangesReport; the caller should set
// Report.GeneratedAt to the actual generation time.
func zeroTime() time.Time {
	return time.Time{}
}

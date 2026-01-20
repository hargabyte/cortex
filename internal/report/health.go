// Package report provides schema types for CX report generation.
package report

import (
	"time"
)

// HealthReportData is the complete data structure for health reports.
// Health reports provide risk analysis, coverage gaps, and recommendations for improving
// codebase health. They identify untested critical code, dead code candidates, circular
// dependencies, and complexity hotspots.
//
// The health score ranges from 0-100, where higher values indicate a healthier codebase.
// A score of 100 indicates excellent health (high coverage, no dead code, no cycles).
// A score of 0 indicates critical health issues that need immediate attention.
type HealthReportData struct {
	// Report contains the common report header fields (type, generated_at, etc.).
	Report ReportHeader `yaml:"report" json:"report"`

	// RiskScore is a health score from 0-100, where higher = healthier.
	// Computed from overall coverage, untested keystones, dead code candidates, and
	// circular dependencies.
	RiskScore int `yaml:"risk_score" json:"risk_score"`

	// Issues groups health issues by severity level (critical, warning, info).
	Issues HealthIssues `yaml:"issues" json:"issues"`

	// Coverage contains test coverage analysis, including overall coverage,
	// per-entity coverage, and identified gaps.
	// Optional but recommended for complete health analysis.
	Coverage *CoverageData `yaml:"coverage,omitempty" json:"coverage,omitempty"`

	// Complexity contains complexity hotspot identification for code maintenance risk.
	// Optional but recommended for large codebases.
	Complexity *ComplexityAnalysis `yaml:"complexity,omitempty" json:"complexity,omitempty"`

	// Diagrams maps diagram names to D2 diagram definitions for visualization.
	// Typically includes a risk_map showing risk distribution across the codebase.
	Diagrams map[string]DiagramData `yaml:"diagrams" json:"diagrams"`

	// DeadCodeGroups contains dead code grouped by module.
	DeadCodeGroups []DeadCodeGroup `yaml:"dead_code_groups,omitempty" json:"dead_code_groups,omitempty"`
}

// ComplexityAnalysis contains complexity hotspot information for the codebase.
// Hotspots are entities with high cyclomatic complexity, high out-degree (many dependencies),
// or large line counts that may indicate maintenance risk.
type ComplexityAnalysis struct {
	// Hotspots is a list of detected complexity hotspots in the codebase.
	Hotspots []ComplexityHotspot `yaml:"hotspots" json:"hotspots"`
}

// NewHealthReport creates a new HealthReportData with initialized fields.
// The report type is automatically set to ReportTypeHealth and generated_at is set to now.
func NewHealthReport() *HealthReportData {
	return &HealthReportData{
		Report: ReportHeader{
			Type:        ReportTypeHealth,
			GeneratedAt: time.Now(),
		},
		RiskScore: 0,
		Issues: HealthIssues{
			Critical: []HealthIssue{},
			Warning:  []HealthIssue{},
			Info:     []HealthIssue{},
		},
		Diagrams: make(map[string]DiagramData),
	}
}

// AddCriticalIssue adds a critical health issue to the report.
// Critical issues should be addressed immediately as they represent serious risks.
func (h *HealthReportData) AddCriticalIssue(issue HealthIssue) {
	h.Issues.Critical = append(h.Issues.Critical, issue)
}

// AddWarningIssue adds a warning-level health issue to the report.
// Warning issues should be addressed soon but are less urgent than critical issues.
func (h *HealthReportData) AddWarningIssue(issue HealthIssue) {
	h.Issues.Warning = append(h.Issues.Warning, issue)
}

// AddInfoIssue adds an informational health issue to the report.
// Info issues are for awareness and may not require immediate action.
func (h *HealthReportData) AddInfoIssue(issue HealthIssue) {
	h.Issues.Info = append(h.Issues.Info, issue)
}

// SetCoverage sets the coverage analysis for this report.
func (h *HealthReportData) SetCoverage(coverage *CoverageData) {
	h.Coverage = coverage
}

// SetComplexity sets the complexity analysis for this report.
func (h *HealthReportData) SetComplexity(complexity *ComplexityAnalysis) {
	h.Complexity = complexity
}

// AddDiagram adds a D2 diagram to the report diagrams collection.
func (h *HealthReportData) AddDiagram(name string, diagram DiagramData) {
	h.Diagrams[name] = diagram
}

// TotalIssues returns the total count of all issues across all severity levels.
func (h *HealthReportData) TotalIssues() int {
	return len(h.Issues.Critical) + len(h.Issues.Warning) + len(h.Issues.Info)
}

// CriticalIssueCount returns the count of critical issues.
func (h *HealthReportData) CriticalIssueCount() int {
	return len(h.Issues.Critical)
}

// WarningIssueCount returns the count of warning issues.
func (h *HealthReportData) WarningIssueCount() int {
	return len(h.Issues.Warning)
}

// InfoIssueCount returns the count of info issues.
func (h *HealthReportData) InfoIssueCount() int {
	return len(h.Issues.Info)
}

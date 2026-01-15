package integration

import (
	"fmt"
	"strings"
)

// ExportTarget represents a target system for export.
type ExportTarget string

const (
	ExportTargetBeads  ExportTarget = "beads"
	ExportTargetStdout ExportTarget = "stdout"
)

// ImpactReport contains the results of an impact analysis.
type ImpactReport struct {
	TriggerEntity    string   // The entity that changed
	AffectedEntities []string // Entities affected by the change
	Severity         string   // low, medium, high, critical
	Summary          string   // Human-readable summary
}

// ExportImpactToBeads creates a bead task from an impact report.
// Returns the created bead ID.
func ExportImpactToBeads(report *ImpactReport) (string, error) {
	if !BeadsAvailable() {
		return "", fmt.Errorf("beads not available")
	}

	// Determine priority from severity
	priority := 2 // default medium
	switch report.Severity {
	case "critical":
		priority = 0
	case "high":
		priority = 1
	case "medium":
		priority = 2
	case "low":
		priority = 3
	}

	// Build description
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("Impact analysis for: %s\n\n", report.TriggerEntity))
	desc.WriteString(fmt.Sprintf("Severity: %s\n", report.Severity))
	desc.WriteString(fmt.Sprintf("Affected entities: %d\n\n", len(report.AffectedEntities)))

	if len(report.AffectedEntities) > 0 {
		desc.WriteString("Affected:\n")
		for i, entity := range report.AffectedEntities {
			if i >= 10 {
				desc.WriteString(fmt.Sprintf("... and %d more\n", len(report.AffectedEntities)-10))
				break
			}
			desc.WriteString(fmt.Sprintf("- %s\n", entity))
		}
	}

	title := fmt.Sprintf("Review: Impact of changes to %s", report.TriggerEntity)
	if len(title) > 80 {
		title = title[:77] + "..."
	}

	opts := CreateBeadOptions{
		Title:       title,
		Description: desc.String(),
		Type:        "task",
		Priority:    priority,
		Labels:      []string{"cx:impact", "cx:review-needed"},
	}

	return CreateBead(opts)
}

// StaleEntityReport contains the results of a staleness check.
type StaleEntityReport struct {
	StaleEntities []StaleEntity
	Summary       string
}

// StaleEntity represents an entity that may be stale.
type StaleEntity struct {
	ID      string
	Name    string
	Reason  string // e.g., "signature changed", "body changed", "file deleted"
	OldHash string
	NewHash string
}

// ExportStaleToBeads creates a bead task from a stale entity report.
func ExportStaleToBeads(report *StaleEntityReport) (string, error) {
	if !BeadsAvailable() {
		return "", fmt.Errorf("beads not available")
	}

	if len(report.StaleEntities) == 0 {
		return "", fmt.Errorf("no stale entities to report")
	}

	// Build description
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("Found %d stale entities that need review:\n\n", len(report.StaleEntities)))

	for i, entity := range report.StaleEntities {
		if i >= 20 {
			desc.WriteString(fmt.Sprintf("\n... and %d more\n", len(report.StaleEntities)-20))
			break
		}
		desc.WriteString(fmt.Sprintf("- %s: %s\n", entity.Name, entity.Reason))
	}

	priority := 2
	if len(report.StaleEntities) > 10 {
		priority = 1 // Many stale entities = higher priority
	}

	opts := CreateBeadOptions{
		Title:       fmt.Sprintf("Review: %d stale entities detected", len(report.StaleEntities)),
		Description: desc.String(),
		Type:        "task",
		Priority:    priority,
		Labels:      []string{"cx:stale", "cx:review-needed"},
	}

	return CreateBead(opts)
}

// DiscoveredWorkReport contains work discovered during analysis.
type DiscoveredWorkReport struct {
	Title         string
	Description   string
	Type          string // bug, task, feature
	Priority      int
	DiscoveredFrom string // Entity ID that led to this discovery
	Labels        []string
}

// ExportDiscoveredWork creates a bead for discovered work with a discovered-from dependency.
func ExportDiscoveredWork(report *DiscoveredWorkReport) (string, error) {
	if !BeadsAvailable() {
		return "", fmt.Errorf("beads not available")
	}

	labels := append([]string{"cx:discovered"}, report.Labels...)

	opts := CreateBeadOptions{
		Title:       report.Title,
		Description: report.Description,
		Type:        report.Type,
		Priority:    report.Priority,
		Labels:      labels,
	}

	beadID, err := CreateBead(opts)
	if err != nil {
		return "", err
	}

	// Add discovered-from dependency if source is provided
	if report.DiscoveredFrom != "" {
		if err := AddDependency(beadID, report.DiscoveredFrom, "discovered-from"); err != nil {
			// Non-fatal: bead was created, just couldn't add dep
			// The bead ID is still returned
			return beadID, fmt.Errorf("created bead %s but failed to add dependency: %w", beadID, err)
		}
	}

	return beadID, nil
}

// FormatImpactSummary creates a human-readable summary of an impact report.
func FormatImpactSummary(report *ImpactReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Impact Analysis: %s\n", report.TriggerEntity))
	sb.WriteString(fmt.Sprintf("Severity: %s\n", report.Severity))
	sb.WriteString(fmt.Sprintf("Affected: %d entities\n", len(report.AffectedEntities)))

	if report.Summary != "" {
		sb.WriteString(fmt.Sprintf("\n%s\n", report.Summary))
	}

	return sb.String()
}

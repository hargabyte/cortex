package report

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestNewChangesReport verifies that NewChangesReport initializes a changes report correctly.
func TestNewChangesReport(t *testing.T) {
	fromRef := "HEAD~50"
	toRef := "HEAD"

	report := NewChangesReport(fromRef, toRef)

	// Check basic initialization
	if report == nil {
		t.Fatal("NewChangesReport returned nil")
	}

	if report.Report.Type != ReportTypeChanges {
		t.Errorf("Report.Type = %v, want %v", report.Report.Type, ReportTypeChanges)
	}

	if report.Report.FromRef != fromRef {
		t.Errorf("Report.FromRef = %q, want %q", report.Report.FromRef, fromRef)
	}

	if report.Report.ToRef != toRef {
		t.Errorf("Report.ToRef = %q, want %q", report.Report.ToRef, toRef)
	}

	// Check that slices are initialized (not nil)
	if report.AddedEntities == nil {
		t.Error("AddedEntities is nil, want initialized slice")
	}

	if len(report.AddedEntities) != 0 {
		t.Errorf("AddedEntities length = %d, want 0", len(report.AddedEntities))
	}

	if report.ModifiedEntities == nil {
		t.Error("ModifiedEntities is nil, want initialized slice")
	}

	if len(report.ModifiedEntities) != 0 {
		t.Errorf("ModifiedEntities length = %d, want 0", len(report.ModifiedEntities))
	}

	if report.DeletedEntities == nil {
		t.Error("DeletedEntities is nil, want initialized slice")
	}

	if len(report.DeletedEntities) != 0 {
		t.Errorf("DeletedEntities length = %d, want 0", len(report.DeletedEntities))
	}

	// Check that map is initialized (not nil)
	if report.Diagrams == nil {
		t.Error("Diagrams is nil, want initialized map")
	}

	if len(report.Diagrams) != 0 {
		t.Errorf("Diagrams length = %d, want 0", len(report.Diagrams))
	}
}

// TestChangesReportYAMLMarshaling verifies that ChangesReportData marshals to YAML correctly.
func TestChangesReportYAMLMarshaling(t *testing.T) {
	now := time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC)

	report := &ChangesReportData{
		Report: ReportHeader{
			Type:        ReportTypeChanges,
			GeneratedAt: now,
			FromRef:     "HEAD~50",
			ToRef:       "HEAD",
		},
		Metadata: MetadataData{
			CommitsAnalyzed: 50,
			TimeRange: &TimeRange{
				From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				To:   now,
			},
		},
		Statistics: StatisticsData{
			Added:    45,
			Modified: 89,
			Deleted:  12,
		},
		AddedEntities: []ChangedEntity{
			{
				ID:      "sa-fn-new123",
				Name:    "GenerateReport",
				Type:    "function",
				File:    "internal/report/generate.go",
				Lines:   [2]int{1, 45},
				AddedIn: "abc1234",
			},
		},
		ModifiedEntities: []ChangedEntity{
			{
				ID:            "sa-fn-mod456",
				Name:          "SearchEntities",
				Type:          "function",
				File:          "internal/store/fts.go",
				ChangeSummary: "Added embedding search support",
				LinesChanged:  23,
			},
		},
		DeletedEntities: []ChangedEntity{
			{
				ID:        "sa-fn-del789",
				Name:      "OldSearch",
				Type:      "function",
				WasFile:   "internal/store/search_old.go",
				DeletedIn: "def5678",
			},
		},
		Diagrams: map[string]DiagramData{
			"architecture_before": {
				Title: "Architecture at HEAD~50",
				D2:    "direction: down\n# D2 code for before state",
			},
			"architecture_after": {
				Title: "Architecture at HEAD",
				D2:    "direction: down\n# D2 code for after state",
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	if data == nil || len(data) == 0 {
		t.Fatal("yaml.Marshal returned empty data")
	}

	// Unmarshal back to verify round-trip
	var unmarshaled ChangesReportData
	err = yaml.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	// Verify key fields survived round-trip
	if unmarshaled.Report.Type != ReportTypeChanges {
		t.Errorf("Unmarshaled Report.Type = %v, want %v", unmarshaled.Report.Type, ReportTypeChanges)
	}

	if unmarshaled.Report.FromRef != "HEAD~50" {
		t.Errorf("Unmarshaled Report.FromRef = %q, want %q", unmarshaled.Report.FromRef, "HEAD~50")
	}

	if unmarshaled.Statistics.Added != 45 {
		t.Errorf("Unmarshaled Statistics.Added = %d, want 45", unmarshaled.Statistics.Added)
	}

	if len(unmarshaled.AddedEntities) != 1 {
		t.Errorf("Unmarshaled AddedEntities length = %d, want 1", len(unmarshaled.AddedEntities))
	}

	if unmarshaled.AddedEntities[0].Name != "GenerateReport" {
		t.Errorf("Unmarshaled AddedEntities[0].Name = %q, want %q", unmarshaled.AddedEntities[0].Name, "GenerateReport")
	}

	if len(unmarshaled.Diagrams) != 2 {
		t.Errorf("Unmarshaled Diagrams length = %d, want 2", len(unmarshaled.Diagrams))
	}
}

// TestChangesReportJSONMarshaling verifies that ChangesReportData marshals to JSON correctly.
func TestChangesReportJSONMarshaling(t *testing.T) {
	now := time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC)

	report := &ChangesReportData{
		Report: ReportHeader{
			Type:        ReportTypeChanges,
			GeneratedAt: now,
			FromRef:     "v1.0",
			ToRef:       "v2.0",
		},
		Metadata: MetadataData{
			CommitsAnalyzed: 100,
		},
		Statistics: StatisticsData{
			Added:    20,
			Modified: 50,
			Deleted:  5,
		},
		AddedEntities: []ChangedEntity{
			{
				ID:      "sa-fn-xyz999",
				Name:    "NewFunction",
				Type:    "function",
				File:    "internal/new/func.go",
				AddedIn: "commit123",
			},
		},
		Diagrams: make(map[string]DiagramData),
	}

	// Marshal to JSON
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if data == nil || len(data) == 0 {
		t.Fatal("json.Marshal returned empty data")
	}

	// Unmarshal back to verify round-trip
	var unmarshaled ChangesReportData
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify key fields survived round-trip
	if unmarshaled.Report.Type != ReportTypeChanges {
		t.Errorf("Unmarshaled Report.Type = %v, want %v", unmarshaled.Report.Type, ReportTypeChanges)
	}

	if unmarshaled.Report.FromRef != "v1.0" {
		t.Errorf("Unmarshaled Report.FromRef = %q, want %q", unmarshaled.Report.FromRef, "v1.0")
	}

	if unmarshaled.Report.ToRef != "v2.0" {
		t.Errorf("Unmarshaled Report.ToRef = %q, want %q", unmarshaled.Report.ToRef, "v2.0")
	}

	if unmarshaled.Statistics.Modified != 50 {
		t.Errorf("Unmarshaled Statistics.Modified = %d, want 50", unmarshaled.Statistics.Modified)
	}

	if len(unmarshaled.AddedEntities) != 1 {
		t.Errorf("Unmarshaled AddedEntities length = %d, want 1", len(unmarshaled.AddedEntities))
	}

	if unmarshaled.AddedEntities[0].Name != "NewFunction" {
		t.Errorf("Unmarshaled AddedEntities[0].Name = %q, want %q", unmarshaled.AddedEntities[0].Name, "NewFunction")
	}
}

// TestChangesReportWithImpactAnalysis verifies the optional impact field marshals correctly.
func TestChangesReportWithImpactAnalysis(t *testing.T) {
	now := time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC)

	report := &ChangesReportData{
		Report: ReportHeader{
			Type:        ReportTypeChanges,
			GeneratedAt: now,
			FromRef:     "HEAD~20",
			ToRef:       "HEAD",
		},
		Metadata:         MetadataData{CommitsAnalyzed: 20},
		Statistics:       StatisticsData{Added: 5, Modified: 15, Deleted: 2},
		AddedEntities:    []ChangedEntity{},
		ModifiedEntities: []ChangedEntity{},
		DeletedEntities:  []ChangedEntity{},
		Diagrams:         make(map[string]DiagramData),
		Impact: &ImpactAnalysis{
			HighImpactChanges: []ImpactChange{
				{
					Entity:             "SearchEntities",
					DependentsAffected: 23,
					Risk:               "medium",
				},
			},
		},
	}

	// Marshal to JSON with impact
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ChangesReportData
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Impact == nil {
		t.Error("Unmarshaled Impact is nil, want non-nil")
	}

	if len(unmarshaled.Impact.HighImpactChanges) != 1 {
		t.Errorf("Unmarshaled Impact.HighImpactChanges length = %d, want 1", len(unmarshaled.Impact.HighImpactChanges))
	}

	if unmarshaled.Impact.HighImpactChanges[0].Entity != "SearchEntities" {
		t.Errorf("Unmarshaled Impact.HighImpactChanges[0].Entity = %q, want %q", unmarshaled.Impact.HighImpactChanges[0].Entity, "SearchEntities")
	}

	if unmarshaled.Impact.HighImpactChanges[0].DependentsAffected != 23 {
		t.Errorf("Unmarshaled Impact.HighImpactChanges[0].DependentsAffected = %d, want 23", unmarshaled.Impact.HighImpactChanges[0].DependentsAffected)
	}
}

// TestChangesReportStructureLargeDataset verifies the structure with a realistic large dataset.
func TestChangesReportStructureLargeDataset(t *testing.T) {
	now := time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC)

	report := &ChangesReportData{
		Report: ReportHeader{
			Type:        ReportTypeChanges,
			GeneratedAt: now,
			FromRef:     "HEAD~100",
			ToRef:       "HEAD",
		},
		Metadata: MetadataData{
			CommitsAnalyzed: 100,
			TimeRange: &TimeRange{
				From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				To:   now,
			},
		},
		Statistics: StatisticsData{
			Added:    100,
			Modified: 250,
			Deleted:  30,
		},
		AddedEntities:    make([]ChangedEntity, 0, 100),
		ModifiedEntities: make([]ChangedEntity, 0, 250),
		DeletedEntities:  make([]ChangedEntity, 0, 30),
		Diagrams:         make(map[string]DiagramData),
	}

	// Populate with realistic data
	for i := 0; i < 100; i++ {
		report.AddedEntities = append(report.AddedEntities, ChangedEntity{
			ID:      "sa-fn-add" + string(rune(i)),
			Name:    "AddedFunc" + string(rune(i)),
			Type:    "function",
			File:    "internal/new/added.go",
			AddedIn: "commit" + string(rune(i)),
		})
	}

	for i := 0; i < 250; i++ {
		report.ModifiedEntities = append(report.ModifiedEntities, ChangedEntity{
			ID:            "sa-fn-mod" + string(rune(i)),
			Name:          "ModifiedFunc" + string(rune(i)),
			Type:          "function",
			File:          "internal/modified/mod.go",
			ChangeSummary: "Updated implementation",
			LinesChanged:  10 + i,
		})
	}

	for i := 0; i < 30; i++ {
		report.DeletedEntities = append(report.DeletedEntities, ChangedEntity{
			ID:        "sa-fn-del" + string(rune(i)),
			Name:      "DeletedFunc" + string(rune(i)),
			Type:      "function",
			WasFile:   "internal/old/deleted.go",
			DeletedIn: "commit" + string(rune(i)),
		})
	}

	report.Diagrams["architecture_before"] = DiagramData{
		Title: "Architecture at HEAD~100",
		D2:    "# Before state",
	}

	report.Diagrams["architecture_after"] = DiagramData{
		Title: "Architecture at HEAD",
		D2:    "# After state",
	}

	// Verify structure
	if len(report.AddedEntities) != 100 {
		t.Errorf("AddedEntities length = %d, want 100", len(report.AddedEntities))
	}

	if len(report.ModifiedEntities) != 250 {
		t.Errorf("ModifiedEntities length = %d, want 250", len(report.ModifiedEntities))
	}

	if len(report.DeletedEntities) != 30 {
		t.Errorf("DeletedEntities length = %d, want 30", len(report.DeletedEntities))
	}

	if len(report.Diagrams) != 2 {
		t.Errorf("Diagrams length = %d, want 2", len(report.Diagrams))
	}

	// Marshal to JSON to verify it can handle large dataset
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("json.Marshal returned empty data")
	}
}

// TestChangesReportEmptyStructure verifies that an empty changes report marshals correctly.
func TestChangesReportEmptyStructure(t *testing.T) {
	report := NewChangesReport("HEAD~10", "HEAD")
	report.Report.GeneratedAt = time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC)

	// Marshal to YAML
	yamlData, err := yaml.Marshal(report)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if len(yamlData) == 0 || len(jsonData) == 0 {
		t.Fatal("Marshal returned empty data for empty structure")
	}

	// Verify impact field is omitted when nil
	var unmarshaled ChangesReportData
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Impact != nil {
		t.Error("Unmarshaled Impact should be nil when not set (omitempty should apply)")
	}
}

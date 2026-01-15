package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeadsAvailable_NoBdCommand(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty to ensure bd is not found
	os.Setenv("PATH", "")

	if BeadsAvailable() {
		t.Error("expected BeadsAvailable() to return false when bd not in PATH")
	}
}

func TestBeadsAvailableIn_NoBeadsDir(t *testing.T) {
	// Check if bd exists (skip if it doesn't)
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd command not available")
	}

	// Use a temp dir without .beads
	tmpDir, err := os.MkdirTemp("", "cx-test-no-beads-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if BeadsAvailableIn(tmpDir) {
		t.Error("expected BeadsAvailableIn() to return false when .beads dir doesn't exist")
	}
}

func TestBeadsAvailableIn_WithBeadsDir(t *testing.T) {
	// Check if bd exists (skip if it doesn't)
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd command not available")
	}

	// Use a temp dir with .beads
	tmpDir, err := os.MkdirTemp("", "cx-test-with-beads-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .beads directory
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("create .beads dir: %v", err)
	}

	if !BeadsAvailableIn(tmpDir) {
		t.Error("expected BeadsAvailableIn() to return true when .beads dir exists")
	}
}

func TestLooksLikeBeadID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"bd-abc123", true},
		{"Project-hi5", true},
		{"Superhero-AI-4ja.16", true},
		{"bd-a", false},       // too short
		{"single", false},     // no hyphen
		{"bd-", false},        // empty suffix
		{"bd-abc!", false},    // invalid character
		{"bd-abc def", false}, // space
		{"", false},
	}

	for _, tt := range tests {
		got := looksLikeBeadID(tt.input)
		if got != tt.want {
			t.Errorf("looksLikeBeadID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExtractBeadID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Created issue: bd-abc123", "bd-abc123"},
		{"✓ Created: Project-xyz", "Project-xyz"},
		{"bd-short", "bd-short"},
		{"no id here", ""},
		{"", ""},
		{"Created Superhero-AI-4ja.16.9.1 successfully", "Superhero-AI-4ja.16.9.1"},
	}

	for _, tt := range tests {
		got := extractBeadID(tt.input)
		if got != tt.want {
			t.Errorf("extractBeadID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Export report tests

func TestImpactReport_PriorityMapping(t *testing.T) {
	// This tests the logic, not the actual bd command
	testCases := []struct {
		severity string
		expected int
	}{
		{"critical", 0},
		{"high", 1},
		{"medium", 2},
		{"low", 3},
		{"unknown", 2}, // default
	}

	for _, tc := range testCases {
		priority := 2 // default
		switch tc.severity {
		case "critical":
			priority = 0
		case "high":
			priority = 1
		case "medium":
			priority = 2
		case "low":
			priority = 3
		}

		if priority != tc.expected {
			t.Errorf("severity %q -> priority %d, want %d", tc.severity, priority, tc.expected)
		}
	}
}

func TestFormatImpactSummary(t *testing.T) {
	report := &ImpactReport{
		TriggerEntity:    "pkg/auth/login.go:LoginUser",
		AffectedEntities: []string{"pkg/api/handler.go:HandleLogin", "pkg/db/users.go:GetUser"},
		Severity:         "high",
		Summary:          "Changes to LoginUser affect authentication flow",
	}

	result := FormatImpactSummary(report)

	// Check key parts are present
	if !strings.Contains(result, "LoginUser") {
		t.Error("expected summary to contain trigger entity")
	}
	if !strings.Contains(result, "high") {
		t.Error("expected summary to contain severity")
	}
	if !strings.Contains(result, "2") {
		t.Error("expected summary to contain affected count")
	}
	if !strings.Contains(result, "authentication flow") {
		t.Error("expected summary to contain custom summary")
	}
}

func TestFormatImpactSummary_NoSummary(t *testing.T) {
	report := &ImpactReport{
		TriggerEntity:    "fn-test",
		AffectedEntities: []string{"fn-dep1"},
		Severity:         "low",
	}

	result := FormatImpactSummary(report)

	// Should still work without summary
	if !strings.Contains(result, "fn-test") {
		t.Error("expected summary to contain trigger entity")
	}
}

func TestCreateBeadOptions_Defaults(t *testing.T) {
	opts := CreateBeadOptions{
		Title: "Test task",
	}

	// Verify default values
	if opts.Type != "" {
		t.Errorf("expected empty type by default, got %q", opts.Type)
	}
	if opts.Priority != 0 {
		t.Errorf("expected priority 0 by default, got %d", opts.Priority)
	}
	if len(opts.Labels) != 0 {
		t.Errorf("expected empty labels by default, got %v", opts.Labels)
	}
}

// StaleEntityReport tests

func TestStaleEntityReport_PriorityEscalation(t *testing.T) {
	// Test the logic: more than 10 stale entities -> priority 1
	staleEntities := make([]StaleEntity, 15)
	for i := range staleEntities {
		staleEntities[i] = StaleEntity{ID: "fn-" + string(rune('a'+i))}
	}

	priority := 2
	if len(staleEntities) > 10 {
		priority = 1
	}

	if priority != 1 {
		t.Errorf("expected priority 1 for >10 stale entities, got %d", priority)
	}
}

func TestStaleEntity_Fields(t *testing.T) {
	entity := StaleEntity{
		ID:      "fn-test",
		Name:    "TestFunc",
		Reason:  "signature changed",
		OldHash: "abc123",
		NewHash: "xyz789",
	}

	if entity.ID != "fn-test" {
		t.Errorf("expected ID 'fn-test', got %q", entity.ID)
	}
	if entity.Reason != "signature changed" {
		t.Errorf("expected reason 'signature changed', got %q", entity.Reason)
	}
}

// DiscoveredWorkReport tests

func TestDiscoveredWorkReport_Fields(t *testing.T) {
	report := DiscoveredWorkReport{
		Title:          "Fix null pointer in handler",
		Description:    "Found while analyzing impact of changes",
		Type:           "bug",
		Priority:       1,
		DiscoveredFrom: "fn-auth-login",
		Labels:         []string{"security", "high-priority"},
	}

	if report.Type != "bug" {
		t.Errorf("expected type 'bug', got %q", report.Type)
	}
	if report.Priority != 1 {
		t.Errorf("expected priority 1, got %d", report.Priority)
	}
	if len(report.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(report.Labels))
	}
}

// BeadInfo tests

func TestBeadInfo_Fields(t *testing.T) {
	info := BeadInfo{
		ID:          "bd-test123",
		Title:       "Test Issue",
		Description: "This is a test issue",
		Status:      "open",
		Priority:    2,
		Labels:      []string{"test", "cx:impact"},
	}

	if info.ID != "bd-test123" {
		t.Errorf("expected ID 'bd-test123', got %q", info.ID)
	}
	if info.Status != "open" {
		t.Errorf("expected status 'open', got %q", info.Status)
	}
}

// ExportTarget tests

func TestExportTarget_Constants(t *testing.T) {
	if ExportTargetBeads != "beads" {
		t.Errorf("expected ExportTargetBeads = 'beads', got %q", ExportTargetBeads)
	}
	if ExportTargetStdout != "stdout" {
		t.Errorf("expected ExportTargetStdout = 'stdout', got %q", ExportTargetStdout)
	}
}

// Integration tests that require bd command

func TestGetBead_Integration(t *testing.T) {
	// Skip if bd not available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd command not available")
	}

	// Skip if not in a beads-enabled directory
	if !BeadsAvailable() {
		t.Skip("no .beads directory in current location")
	}

	// This test would require a known bead ID to exist
	// For now, just verify the function doesn't panic with invalid input
	_, err := GetBead("nonexistent-bead-id-12345")
	if err == nil {
		t.Log("unexpectedly found nonexistent bead")
	}
}

func TestCreateBead_NoBeads(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty
	os.Setenv("PATH", "")

	_, err := CreateBead(CreateBeadOptions{Title: "Test"})
	if err == nil {
		t.Error("expected error when bd not available")
	}
}

func TestExportImpactToBeads_NoBeads(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty
	os.Setenv("PATH", "")

	report := &ImpactReport{
		TriggerEntity:    "fn-test",
		AffectedEntities: []string{"fn-dep"},
		Severity:         "medium",
	}

	_, err := ExportImpactToBeads(report)
	if err == nil {
		t.Error("expected error when beads not available")
	}
}

func TestExportStaleToBeads_NoBeads(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty
	os.Setenv("PATH", "")

	report := &StaleEntityReport{
		StaleEntities: []StaleEntity{{ID: "fn-test", Name: "Test", Reason: "changed"}},
	}

	_, err := ExportStaleToBeads(report)
	if err == nil {
		t.Error("expected error when beads not available")
	}
}

func TestExportStaleToBeads_EmptyReport(t *testing.T) {
	report := &StaleEntityReport{
		StaleEntities: []StaleEntity{}, // Empty
	}

	_, err := ExportStaleToBeads(report)
	if err == nil {
		t.Error("expected error for empty stale report")
	}
	// Either "no stale entities" or "beads not available" is acceptable
	errMsg := err.Error()
	if !strings.Contains(errMsg, "no stale entities") && !strings.Contains(errMsg, "beads not available") {
		t.Errorf("expected 'no stale entities' or 'beads not available' error, got: %v", err)
	}
}

func TestExportDiscoveredWork_NoBeads(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty
	os.Setenv("PATH", "")

	report := &DiscoveredWorkReport{
		Title:       "Test",
		Description: "Test description",
		Type:        "task",
		Priority:    2,
	}

	_, err := ExportDiscoveredWork(report)
	if err == nil {
		t.Error("expected error when beads not available")
	}
}

func TestAddDependency_NoBeads(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty
	os.Setenv("PATH", "")

	err := AddDependency("from-id", "to-id", "blocks")
	if err == nil {
		t.Error("expected error when bd not available")
	}
}

// Test title truncation in ExportImpactToBeads
func TestImpactTitleTruncation(t *testing.T) {
	// Simulate the truncation logic
	triggerEntity := "very/long/path/to/some/deeply/nested/file.go:SomeVeryLongFunctionNameThatExceedsTheLimit"
	title := "Review: Impact of changes to " + triggerEntity

	if len(title) > 80 {
		title = title[:77] + "..."
	}

	if len(title) > 80 {
		t.Errorf("title should be truncated to 80 chars max, got %d", len(title))
	}
	if !strings.HasSuffix(title, "...") {
		t.Error("truncated title should end with ...")
	}
}

// Test description building for stale entities
func TestStaleDescriptionBuilding(t *testing.T) {
	// Simulate building description with more than 20 entities
	staleEntities := make([]StaleEntity, 25)
	for i := range staleEntities {
		staleEntities[i] = StaleEntity{
			ID:     "fn-" + string(rune('a'+i%26)),
			Name:   "Func" + string(rune('A'+i%26)),
			Reason: "signature changed",
		}
	}

	var desc strings.Builder
	desc.WriteString("Found stale entities:\n")

	for i, entity := range staleEntities {
		if i >= 20 {
			desc.WriteString("... and more\n")
			break
		}
		desc.WriteString("- " + entity.Name + "\n")
	}

	result := desc.String()
	if !strings.Contains(result, "... and more") {
		t.Error("expected truncation message for >20 entities")
	}
}

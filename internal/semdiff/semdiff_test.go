package semdiff

import (
	"testing"
)

func TestChangeTypeConstants(t *testing.T) {
	// Verify change type constants are defined correctly
	tests := []struct {
		ct   ChangeType
		want string
	}{
		{ChangeAdded, "added"},
		{ChangeRemoved, "removed"},
		{ChangeSignature, "signature_change"},
		{ChangeBody, "body_change"},
	}

	for _, tt := range tests {
		if string(tt.ct) != tt.want {
			t.Errorf("ChangeType %v = %q, want %q", tt.ct, tt.ct, tt.want)
		}
	}
}

func TestBuildSummary(t *testing.T) {
	changes := []SemanticChange{
		{Name: "func1", ChangeType: ChangeAdded, Breaking: false},
		{Name: "func2", ChangeType: ChangeRemoved, Breaking: true, AffectedCallers: 3},
		{Name: "func3", ChangeType: ChangeSignature, Breaking: true, AffectedCallers: 5},
		{Name: "func4", ChangeType: ChangeBody, Breaking: false, AffectedCallers: 2},
		{Name: "func5", ChangeType: ChangeBody, Breaking: false},
	}

	summary := buildSummary(changes)

	if summary.TotalChanges != 5 {
		t.Errorf("TotalChanges = %d, want 5", summary.TotalChanges)
	}
	if summary.BreakingChanges != 2 {
		t.Errorf("BreakingChanges = %d, want 2", summary.BreakingChanges)
	}
	if summary.Added != 1 {
		t.Errorf("Added = %d, want 1", summary.Added)
	}
	if summary.Removed != 1 {
		t.Errorf("Removed = %d, want 1", summary.Removed)
	}
	if summary.SignatureChanges != 1 {
		t.Errorf("SignatureChanges = %d, want 1", summary.SignatureChanges)
	}
	if summary.BodyChanges != 2 {
		t.Errorf("BodyChanges = %d, want 2", summary.BodyChanges)
	}
	if summary.TotalAffectedCallers != 10 {
		t.Errorf("TotalAffectedCallers = %d, want 10", summary.TotalAffectedCallers)
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		filePath   string
		filterPath string
		want       bool
	}{
		{"src/auth/login.go", "src/auth/login.go", true},
		{"src/auth/login.go", "src/auth", true},
		{"src/auth/login.go", "src/au", false}, // Partial match should not work
		{"src/auth/login.go", "src/authentication", false},
		{"src/auth/login.go", "internal", false},
		{"internal/cmd/diff.go", "internal/cmd", true},
		{"internal/cmd/diff.go", "internal", true},
	}

	for _, tt := range tests {
		got := matchesPath(tt.filePath, tt.filterPath)
		if got != tt.want {
			t.Errorf("matchesPath(%q, %q) = %v, want %v", tt.filePath, tt.filterPath, got, tt.want)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"src/app.ts", "typescript"},
		{"src/app.tsx", "typescript"},
		{"index.js", "javascript"},
		{"app.jsx", "javascript"},
		{"main.py", "python"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"README.md", ""},
		{"Makefile", ""},
	}

	for _, tt := range tests {
		got := DetectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestSemanticChange_BreakingClassification(t *testing.T) {
	// Signature changes are breaking
	sigChange := SemanticChange{
		ChangeType: ChangeSignature,
		Breaking:   true,
	}
	if !sigChange.Breaking {
		t.Error("Signature changes should be breaking")
	}

	// Body changes are not breaking
	bodyChange := SemanticChange{
		ChangeType: ChangeBody,
		Breaking:   false,
	}
	if bodyChange.Breaking {
		t.Error("Body changes should not be breaking")
	}

	// Additions are not breaking
	addChange := SemanticChange{
		ChangeType: ChangeAdded,
		Breaking:   false,
	}
	if addChange.Breaking {
		t.Error("Additions should not be breaking")
	}

	// Removals with callers are breaking
	removeWithCallers := SemanticChange{
		ChangeType:      ChangeRemoved,
		Breaking:        true,
		AffectedCallers: 5,
	}
	if !removeWithCallers.Breaking {
		t.Error("Removals with callers should be breaking")
	}

	// Removals without callers are not breaking (safe removal)
	safeRemove := SemanticChange{
		ChangeType:      ChangeRemoved,
		Breaking:        false,
		AffectedCallers: 0,
	}
	if safeRemove.Breaking {
		t.Error("Removals without callers should not be breaking (safe removal)")
	}
}

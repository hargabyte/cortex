package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidRef(t *testing.T) {
	tests := []struct {
		ref   string
		valid bool
	}{
		{"HEAD", true},
		{"HEAD~1", true},
		{"HEAD~10", true},
		{"HEAD^", true},
		{"main", true},
		{"feature/branch", true},
		{"v1.0.0", true},
		{"abc123def", true},
		{"entities", true},
		{"dependencies", true},
		{"", false},
		{"'; DROP TABLE --", false},
		{"foo bar", false},
		{"test\"quote", false},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := isValidRef(tt.ref)
			if got != tt.valid {
				t.Errorf("isValidRef(%q) = %v, want %v", tt.ref, got, tt.valid)
			}
		})
	}
}

func TestDoltDiff_EmptyStore(t *testing.T) {
	// Create a temporary directory for the test store
	tmpDir, err := os.MkdirTemp("", "cortex-diff-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")

	// Open store (auto-initializes)
	store, err := Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	// Test DoltDiff with empty store (should return empty result, not error)
	result, err := store.DoltDiff(DiffOptions{})
	if err != nil {
		t.Errorf("DoltDiff on empty store should not error: %v", err)
	}
	if result == nil {
		t.Fatal("DoltDiff should return a result")
	}
	if len(result.Added) != 0 || len(result.Modified) != 0 || len(result.Removed) != 0 {
		t.Errorf("empty store should have no changes, got %d added, %d modified, %d removed",
			len(result.Added), len(result.Modified), len(result.Removed))
	}

	// Test DoltDiffSummary with empty store
	added, modified, removed, err := store.DoltDiffSummary("HEAD~1", "HEAD")
	if err != nil {
		t.Errorf("DoltDiffSummary on empty store should not error: %v", err)
	}
	if added != 0 || modified != 0 || removed != 0 {
		t.Errorf("empty store summary should be zeros, got %d added, %d modified, %d removed",
			added, modified, removed)
	}
}

func TestDoltDiff_InvalidRef(t *testing.T) {
	// Create a temporary directory for the test store
	tmpDir, err := os.MkdirTemp("", "cortex-diff-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cxDir := filepath.Join(tmpDir, ".cx")

	store, err := Open(cxDir)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	// Test with SQL injection attempt
	_, err = store.DoltDiff(DiffOptions{
		FromRef: "'; DROP TABLE entities; --",
	})
	if err == nil {
		t.Error("expected error for invalid ref format")
	}
}

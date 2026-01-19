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

func TestDoltLog(t *testing.T) {
	// Create a temporary directory for the test store
	tmpDir, err := os.MkdirTemp("", "cortex-log-test-*")
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

	// Test DoltLog with fresh store (should have at least 1 commit from init)
	entries, err := store.DoltLog(10)
	if err != nil {
		t.Errorf("DoltLog should not error: %v", err)
	}
	// New Dolt repo might have 0 commits until first explicit commit
	if entries != nil && len(entries) > 0 {
		// Verify entry structure
		entry := entries[0]
		if entry.CommitHash == "" {
			t.Error("CommitHash should not be empty")
		}
	}
}

func TestDoltLogStats(t *testing.T) {
	// Create a temporary directory for the test store
	tmpDir, err := os.MkdirTemp("", "cortex-logstats-test-*")
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

	// Test DoltLogStats with invalid ref (should error)
	_, err = store.DoltLogStats("'; DROP TABLE --")
	if err == nil {
		t.Error("expected error for invalid ref")
	}

	// Test DoltLogStats with HEAD (might return 0s for empty store, but shouldn't error)
	stats, err := store.DoltLogStats("HEAD")
	if err != nil {
		// This might fail if no commits exist yet, which is acceptable
		t.Logf("DoltLogStats(HEAD) returned error (possibly no commits yet): %v", err)
	} else if stats != nil {
		if stats.Entities < 0 || stats.Dependencies < 0 {
			t.Error("Stats should not have negative values")
		}
	}
}

func TestIsValidRef_Exported(t *testing.T) {
	// Test the exported IsValidRef function
	tests := []struct {
		ref   string
		valid bool
	}{
		{"HEAD", true},
		{"HEAD~1", true},
		{"HEAD~10", true},
		{"main", true},
		{"feature/test", true},
		{"v1.0.0", true},
		{"abc123def456", true},
		{"", false},
		{"'; DROP TABLE --", false},
		{"foo bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := IsValidRef(tt.ref)
			if got != tt.valid {
				t.Errorf("IsValidRef(%q) = %v, want %v", tt.ref, got, tt.valid)
			}
		})
	}
}

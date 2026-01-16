package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/cx/internal/store"
)

func TestRunDead(t *testing.T) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "cx-dead-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cx directory
	cxDir := filepath.Join(tmpDir, ".cx")
	if err := os.MkdirAll(cxDir, 0755); err != nil {
		t.Fatalf("Failed to create .cx dir: %v", err)
	}

	// Create store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer storeDB.Close()

	// Create test entities
	now := time.Now()

	// Private function with no callers (dead code)
	deadPrivateFunc := &store.Entity{
		ID:         "sa-fn-dead1-unusedHelper",
		Name:       "unusedHelper",
		EntityType: "function",
		FilePath:   "internal/utils/helpers.go",
		LineStart:  45,
		Visibility: "private",
		Language:   "go",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Private function with callers (not dead)
	usedPrivateFunc := &store.Entity{
		ID:         "sa-fn-used1-usedHelper",
		Name:       "usedHelper",
		EntityType: "function",
		FilePath:   "internal/utils/helpers.go",
		LineStart:  60,
		Visibility: "private",
		Language:   "go",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Public function with no callers (not dead by default)
	unusedExport := &store.Entity{
		ID:         "sa-fn-export1-UnusedExport",
		Name:       "UnusedExport",
		EntityType: "function",
		FilePath:   "pkg/api/handlers.go",
		LineStart:  10,
		Visibility: "public",
		Language:   "go",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Import (should be skipped)
	importEntity := &store.Entity{
		ID:         "sa-im-import1-fmt",
		Name:       "fmt",
		EntityType: "import",
		FilePath:   "internal/utils/helpers.go",
		LineStart:  5,
		Visibility: "public",
		Language:   "go",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Private method with no callers (dead code)
	deadMethod := &store.Entity{
		ID:         "sa-mt-dead2-deadMethod",
		Name:       "deadMethod",
		EntityType: "method",
		FilePath:   "internal/service/service.go",
		LineStart:  100,
		Visibility: "private",
		Language:   "go",
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Create entities
	entities := []*store.Entity{deadPrivateFunc, usedPrivateFunc, unusedExport, importEntity, deadMethod}
	if err := storeDB.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("Failed to create entities: %v", err)
	}

	// Create metrics
	metrics := []*store.Metrics{
		{EntityID: deadPrivateFunc.ID, InDegree: 0, OutDegree: 2, ComputedAt: now},
		{EntityID: usedPrivateFunc.ID, InDegree: 3, OutDegree: 1, ComputedAt: now},
		{EntityID: unusedExport.ID, InDegree: 0, OutDegree: 5, ComputedAt: now},
		{EntityID: importEntity.ID, InDegree: 10, OutDegree: 0, ComputedAt: now},
		{EntityID: deadMethod.ID, InDegree: 0, OutDegree: 0, ComputedAt: now},
	}
	if err := storeDB.SaveBulkMetrics(metrics); err != nil {
		t.Fatalf("Failed to save metrics: %v", err)
	}

	// Change to temp directory so command can find the store
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	t.Run("finds dead private code", func(t *testing.T) {
		// Reset flags
		deadIncludeExports = false
		deadByFile = false
		deadCreateTask = false
		deadTypeFilter = ""

		var buf bytes.Buffer
		deadCmd.SetOut(&buf)

		err := runDead(deadCmd, []string{})
		if err != nil {
			t.Fatalf("runDead failed: %v", err)
		}

		output := buf.String()

		// Should find unusedHelper
		if !bytes.Contains(buf.Bytes(), []byte("unusedHelper")) {
			t.Error("Expected to find unusedHelper in output")
		}

		// Should find deadMethod
		if !bytes.Contains(buf.Bytes(), []byte("deadMethod")) {
			t.Error("Expected to find deadMethod in output")
		}

		// Should NOT find usedHelper (has callers)
		// Note: We search for "name: usedHelper" to avoid false positive from "unusedHelper"
		if bytes.Contains(buf.Bytes(), []byte("name: usedHelper")) {
			t.Error("Did not expect to find usedHelper in output")
		}

		// Should NOT find UnusedExport (public, not included by default)
		if bytes.Contains(buf.Bytes(), []byte("UnusedExport")) {
			t.Error("Did not expect to find UnusedExport in output (without --include-exports)")
		}

		// Should NOT find fmt (import)
		if bytes.Contains(buf.Bytes(), []byte("fmt")) {
			t.Error("Did not expect to find import 'fmt' in output")
		}

		t.Logf("Output:\n%s", output)
	})

	t.Run("include-exports flag works", func(t *testing.T) {
		deadIncludeExports = true
		deadByFile = false
		deadCreateTask = false
		deadTypeFilter = ""

		var buf bytes.Buffer
		deadCmd.SetOut(&buf)

		err := runDead(deadCmd, []string{})
		if err != nil {
			t.Fatalf("runDead failed: %v", err)
		}

		// Should now find UnusedExport
		if !bytes.Contains(buf.Bytes(), []byte("UnusedExport")) {
			t.Error("Expected to find UnusedExport in output with --include-exports")
		}

		// Should still find private dead code
		if !bytes.Contains(buf.Bytes(), []byte("unusedHelper")) {
			t.Error("Expected to find unusedHelper in output")
		}
	})

	t.Run("type filter works", func(t *testing.T) {
		deadIncludeExports = false
		deadByFile = false
		deadCreateTask = false
		deadTypeFilter = "F"

		var buf bytes.Buffer
		deadCmd.SetOut(&buf)

		err := runDead(deadCmd, []string{})
		if err != nil {
			t.Fatalf("runDead failed: %v", err)
		}

		// Should find unusedHelper (function)
		if !bytes.Contains(buf.Bytes(), []byte("unusedHelper")) {
			t.Error("Expected to find unusedHelper in output")
		}

		// Should NOT find deadMethod (method, not function)
		if bytes.Contains(buf.Bytes(), []byte("deadMethod")) {
			t.Error("Did not expect to find deadMethod when filtering for functions only")
		}
	})

	t.Run("imports are skipped", func(t *testing.T) {
		deadIncludeExports = true
		deadByFile = false
		deadCreateTask = false
		deadTypeFilter = ""

		var buf bytes.Buffer
		deadCmd.SetOut(&buf)

		err := runDead(deadCmd, []string{})
		if err != nil {
			t.Fatalf("runDead failed: %v", err)
		}

		// Should NOT find fmt import
		if bytes.Contains(buf.Bytes(), []byte(`name: fmt`)) {
			t.Error("Did not expect to find import 'fmt' in dead code output")
		}
	})
}

func TestNormalizeDeadTypeFilter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"F", "function"},
		{"f", "function"},
		{"FUNCTION", "function"},
		{"func", "function"},
		{"M", "method"},
		{"method", "method"},
		{"T", "type"},
		{"TYPE", "type"},
		{"struct", "type"},
		{"C", "constant"},
		{"const", "constant"},
		{"", ""},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeDeadTypeFilter(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDeadTypeFilter(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMatchesDeadTypeFilter(t *testing.T) {
	tests := []struct {
		entityType string
		filter     string
		expected   bool
	}{
		{"function", "function", true},
		{"func", "function", true},
		{"method", "function", false},
		{"method", "method", true},
		{"type", "type", true},
		{"struct", "type", true},
		{"interface", "type", true},
		{"constant", "constant", true},
		{"const", "constant", true},
		{"var", "constant", true},
		{"variable", "constant", true},
		{"function", "method", false},
	}

	for _, tt := range tests {
		t.Run(tt.entityType+"_"+tt.filter, func(t *testing.T) {
			result := matchesDeadTypeFilter(tt.entityType, tt.filter)
			if result != tt.expected {
				t.Errorf("matchesDeadTypeFilter(%q, %q) = %v, want %v", tt.entityType, tt.filter, result, tt.expected)
			}
		})
	}
}

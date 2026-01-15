package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
)

// TestNearCommand tests the cx near command functionality
func TestNearCommand(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-near-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .cx directory and store
	cxDir := filepath.Join(tmpDir, ".cx")
	st, err := store.Open(cxDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Create test entities
	lineEnd20 := 20
	lineEnd50 := 50
	lineEnd15 := 15
	entities := []*store.Entity{
		{ID: "fn-main", Name: "main", EntityType: "function", FilePath: "main.go", LineStart: 1, LineEnd: &lineEnd20, Visibility: "pub", Signature: "func main()"},
		{ID: "fn-init", Name: "init", EntityType: "function", FilePath: "main.go", LineStart: 25, LineEnd: &lineEnd50, Visibility: "priv", Signature: "func init()"},
		{ID: "fn-handler", Name: "Handler", EntityType: "function", FilePath: "pkg/api/handler.go", LineStart: 5, Visibility: "pub", Signature: "func Handler(w http.ResponseWriter, r *http.Request)"},
		{ID: "tp-user", Name: "User", EntityType: "type", Kind: "struct", FilePath: "pkg/models/user.go", LineStart: 1, LineEnd: &lineEnd15, Visibility: "pub"},
		{ID: "tp-request", Name: "Request", EntityType: "type", Kind: "struct", FilePath: "pkg/api/handler.go", LineStart: 20, Visibility: "pub"},
	}
	if err := st.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities: %v", err)
	}

	// Create dependencies
	deps := []*store.Dependency{
		{FromID: "fn-main", ToID: "fn-init", DepType: "calls"},
		{FromID: "fn-main", ToID: "fn-handler", DepType: "calls"},
		{FromID: "fn-handler", ToID: "tp-user", DepType: "uses_type"},
		{FromID: "fn-handler", ToID: "tp-request", DepType: "uses_type"},
	}
	if err := st.CreateDependenciesBulk(deps); err != nil {
		t.Fatalf("create deps: %v", err)
	}

	// Test: resolveEntityByName
	t.Run("resolve by name", func(t *testing.T) {
		entity, err := resolveEntityByName("main", st, "")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if entity.Name != "main" {
			t.Errorf("expected name 'main', got %q", entity.Name)
		}
	})

	// Test: resolveEntityAtLine
	t.Run("resolve by file:line", func(t *testing.T) {
		entity, err := resolveEntityAtLine("main.go", 10, st)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if entity.Name != "main" {
			t.Errorf("expected name 'main', got %q", entity.Name)
		}
	})

	t.Run("resolve by file:line - later entity", func(t *testing.T) {
		entity, err := resolveEntityAtLine("main.go", 30, st)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if entity.Name != "init" {
			t.Errorf("expected name 'init', got %q", entity.Name)
		}
	})

	// Test: resolveFirstEntityInFile
	t.Run("resolve first in file", func(t *testing.T) {
		entity, err := resolveFirstEntityInFile("main.go", st)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if entity.Name != "main" {
			t.Errorf("expected name 'main', got %q", entity.Name)
		}
	})

	// Test: buildNeighborhood
	t.Run("build neighborhood - basic", func(t *testing.T) {
		entity, _ := st.GetEntity("fn-main")
		near, err := buildNeighborhood(entity, st, 1, "both", output.DensityMedium)
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		// Center should be main
		if near.Center.Name != "main" {
			t.Errorf("expected center 'main', got %q", near.Center.Name)
		}

		// Should have 2 calls (init and Handler)
		if len(near.Neighborhood.Calls) != 2 {
			t.Errorf("expected 2 calls, got %d", len(near.Neighborhood.Calls))
		}

		// init is called by main, so it's in Calls not SameFile
		// Check that SameFile doesn't include entities already in other categories
		for _, e := range near.Neighborhood.SameFile {
			// Verify no duplicates with Calls
			for _, call := range near.Neighborhood.Calls {
				if e.Name == call.Name {
					t.Errorf("entity %q appears in both SameFile and Calls", e.Name)
				}
			}
		}
	})

	t.Run("build neighborhood - direction out", func(t *testing.T) {
		entity, _ := st.GetEntity("fn-main")
		near, err := buildNeighborhood(entity, st, 1, "out", output.DensityMedium)
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		// Should have calls
		if len(near.Neighborhood.Calls) == 0 {
			t.Error("expected some calls")
		}

		// Should NOT have called_by
		if len(near.Neighborhood.CalledBy) != 0 {
			t.Errorf("expected no called_by for direction=out, got %d", len(near.Neighborhood.CalledBy))
		}
	})

	t.Run("build neighborhood - direction in", func(t *testing.T) {
		entity, _ := st.GetEntity("fn-handler")
		near, err := buildNeighborhood(entity, st, 1, "in", output.DensityMedium)
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		// Should have called_by (main calls Handler)
		if len(near.Neighborhood.CalledBy) != 1 {
			t.Errorf("expected 1 caller, got %d", len(near.Neighborhood.CalledBy))
		}

		// Should NOT have calls (direction=in)
		if len(near.Neighborhood.Calls) != 0 {
			t.Errorf("expected no calls for direction=in, got %d", len(near.Neighborhood.Calls))
		}
	})

	t.Run("build neighborhood - depth 2", func(t *testing.T) {
		entity, _ := st.GetEntity("fn-main")
		near, err := buildNeighborhood(entity, st, 2, "out", output.DensityMedium)
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		// Should have uses_types from Handler (depth 2)
		if len(near.Neighborhood.UsesTypes) == 0 {
			t.Error("expected some uses_types at depth 2")
		}

		// Check that User type is found (Handler uses User)
		found := false
		for _, e := range near.Neighborhood.UsesTypes {
			if e.Name == "User" {
				found = true
				if e.Depth != 2 {
					t.Errorf("expected depth 2 for User, got %d", e.Depth)
				}
				break
			}
		}
		if !found {
			t.Error("expected to find User type at depth 2")
		}
	})

	t.Run("build neighborhood - sparse density", func(t *testing.T) {
		entity, _ := st.GetEntity("fn-main")
		near, err := buildNeighborhood(entity, st, 1, "both", output.DensitySparse)
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		// Center should not have signature in sparse mode
		if near.Center.Signature != "" {
			t.Error("expected no signature in sparse mode")
		}

		// Center should not have visibility in sparse mode
		if near.Center.Visibility != "" {
			t.Error("expected no visibility in sparse mode")
		}
	})

	t.Run("build neighborhood - medium density", func(t *testing.T) {
		entity, _ := st.GetEntity("fn-main")
		near, err := buildNeighborhood(entity, st, 1, "both", output.DensityMedium)
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		// Center should have signature in medium mode
		if near.Center.Signature == "" {
			t.Error("expected signature in medium mode")
		}

		// Center should have visibility in medium mode
		if near.Center.Visibility == "" {
			t.Error("expected visibility in medium mode")
		}
	})
}

// TestNearQueryParsing tests the query parsing for cx near
func TestNearQueryParsing(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		isFile   bool
		hasColon bool
	}{
		{"simple name", "main", false, false},
		{"file path", "pkg/api/handler.go", true, false},
		{"file:line", "pkg/api/handler.go:45", true, true},
		{"qualified name", "api.Handler", false, false}, // Contains "." not ":"
		{"ID", "sa-fn-abc123-main", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isFile := isFilePath(tt.query)
			hasColon := strings.Contains(tt.query, ":")

			if isFile != tt.isFile {
				t.Errorf("isFilePath(%q) = %v, want %v", tt.query, isFile, tt.isFile)
			}
			if hasColon != tt.hasColon {
				t.Errorf("hasColon(%q) = %v, want %v", tt.query, hasColon, tt.hasColon)
			}
		})
	}
}

// TestNearOutputFormat tests the output formatting for cx near
func TestNearOutputFormat(t *testing.T) {
	// Create a sample NearOutput
	nearOut := &output.NearOutput{
		Center: &output.NearCenterEntity{
			Name:       "TestFunc",
			Type:       "function",
			Location:   "test.go:10-20",
			Signature:  "func TestFunc() error",
			Visibility: "public",
		},
		Neighborhood: &output.Neighborhood{
			Calls: []*output.NeighborEntity{
				{Name: "helper", Type: "function", Location: "test.go:30", Depth: 1},
			},
			CalledBy: []*output.NeighborEntity{
				{Name: "main", Type: "function", Location: "main.go:5", Depth: 1},
			},
		},
	}

	// Test YAML formatter
	t.Run("yaml format", func(t *testing.T) {
		formatter := output.NewYAMLFormatter()
		var buf bytes.Buffer
		err := formatter.FormatToWriter(&buf, nearOut, output.DensityMedium)
		if err != nil {
			t.Fatalf("format: %v", err)
		}

		result := buf.String()
		if !strings.Contains(result, "center:") {
			t.Error("expected 'center:' in YAML output")
		}
		if !strings.Contains(result, "neighborhood:") {
			t.Error("expected 'neighborhood:' in YAML output")
		}
		if !strings.Contains(result, "TestFunc") {
			t.Error("expected 'TestFunc' in YAML output")
		}
	})

	// Test JSON formatter
	t.Run("json format", func(t *testing.T) {
		formatter := output.NewJSONFormatter()
		var buf bytes.Buffer
		err := formatter.FormatToWriter(&buf, nearOut, output.DensityMedium)
		if err != nil {
			t.Fatalf("format: %v", err)
		}

		result := buf.String()
		if !strings.Contains(result, "\"center\"") {
			t.Error("expected '\"center\"' in JSON output")
		}
		if !strings.Contains(result, "\"neighborhood\"") {
			t.Error("expected '\"neighborhood\"' in JSON output")
		}
		if !strings.Contains(result, "TestFunc") {
			t.Error("expected 'TestFunc' in JSON output")
		}
	})
}

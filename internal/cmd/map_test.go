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

// TestMapCommand tests the cx map command functionality
func TestMapCommand(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cx-map-*")
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

	// Create test entities with skeletons and doc comments
	lineEnd20 := 20
	lineEnd50 := 50
	lineEnd15 := 15
	entities := []*store.Entity{
		{
			ID:         "fn-main",
			Name:       "main",
			EntityType: "function",
			FilePath:   "main.go",
			LineStart:  1,
			LineEnd:    &lineEnd20,
			Visibility: "priv",
			Signature:  "() -> ()",
			Skeleton:   "func main() { ... }",
		},
		{
			ID:         "fn-init",
			Name:       "init",
			EntityType: "function",
			FilePath:   "main.go",
			LineStart:  25,
			LineEnd:    &lineEnd50,
			Visibility: "priv",
			Signature:  "() -> ()",
			Skeleton:   "func init() { ... }",
		},
		{
			ID:         "fn-handler",
			Name:       "Handler",
			EntityType: "function",
			FilePath:   "pkg/api/handler.go",
			LineStart:  5,
			Visibility: "pub",
			Signature:  "(w http.ResponseWriter, r *http.Request) -> ()",
			DocComment: "// Handler handles HTTP requests",
			Skeleton:   "// Handler handles HTTP requests\nfunc Handler(w http.ResponseWriter, r *http.Request) { ... }",
		},
		{
			ID:         "tp-user",
			Name:       "User",
			EntityType: "type",
			Kind:       "struct",
			FilePath:   "pkg/models/user.go",
			LineStart:  1,
			LineEnd:    &lineEnd15,
			Visibility: "pub",
			DocComment: "// User represents a user in the system",
			Skeleton:   "// User represents a user in the system\ntype User struct {\n\tID int\n\tName string\n}",
		},
		{
			ID:         "tp-request",
			Name:       "Request",
			EntityType: "type",
			Kind:       "struct",
			FilePath:   "pkg/api/handler.go",
			LineStart:  20,
			Visibility: "pub",
			Skeleton:   "type Request struct { ... }",
		},
		{
			ID:         "const-version",
			Name:       "Version",
			EntityType: "constant",
			FilePath:   "main.go",
			LineStart:  60,
			Visibility: "pub",
			Skeleton:   "const Version = \"1.0.0\"",
		},
	}
	if err := st.CreateEntitiesBulk(entities); err != nil {
		t.Fatalf("create entities: %v", err)
	}

	t.Run("text output", func(t *testing.T) {
		filter := store.EntityFilter{Status: "active"}
		entities, err := st.QueryEntities(filter)
		if err != nil {
			t.Fatalf("query entities: %v", err)
		}

		var buf bytes.Buffer
		mapCmd.SetOut(&buf)
		err = outputMapText(mapCmd, entities)
		if err != nil {
			t.Fatalf("output map text: %v", err)
		}

		result := buf.String()

		// Check file headers are present
		if !strings.Contains(result, "// main.go") {
			t.Error("expected '// main.go' header in output")
		}
		if !strings.Contains(result, "// pkg/api/handler.go") {
			t.Error("expected '// pkg/api/handler.go' header in output")
		}

		// Check skeletons are present
		if !strings.Contains(result, "func main()") {
			t.Error("expected 'func main()' in output")
		}
		if !strings.Contains(result, "func Handler") {
			t.Error("expected 'func Handler' in output")
		}

		// Check doc comments are preserved
		if !strings.Contains(result, "// Handler handles HTTP requests") {
			t.Error("expected doc comment in output")
		}
	})

	t.Run("yaml output", func(t *testing.T) {
		filter := store.EntityFilter{Status: "active"}
		entities, err := st.QueryEntities(filter)
		if err != nil {
			t.Fatalf("query entities: %v", err)
		}

		var buf bytes.Buffer
		mapCmd.SetOut(&buf)
		err = outputMapStructured(mapCmd, entities, output.FormatYAML)
		if err != nil {
			t.Fatalf("output map structured: %v", err)
		}

		result := buf.String()

		// Check YAML structure
		if !strings.Contains(result, "files:") {
			t.Error("expected 'files:' in YAML output")
		}
		if !strings.Contains(result, "main.go:") {
			t.Error("expected 'main.go:' in YAML output")
		}
		if !strings.Contains(result, "skeleton:") {
			t.Error("expected 'skeleton:' in YAML output")
		}
		if !strings.Contains(result, "count:") {
			t.Error("expected 'count:' in YAML output")
		}
	})

	t.Run("json output", func(t *testing.T) {
		filter := store.EntityFilter{Status: "active"}
		entities, err := st.QueryEntities(filter)
		if err != nil {
			t.Fatalf("query entities: %v", err)
		}

		var buf bytes.Buffer
		mapCmd.SetOut(&buf)
		err = outputMapStructured(mapCmd, entities, output.FormatJSON)
		if err != nil {
			t.Fatalf("output map structured: %v", err)
		}

		result := buf.String()

		// Check JSON structure
		if !strings.Contains(result, "\"files\"") {
			t.Error("expected '\"files\"' in JSON output")
		}
		if !strings.Contains(result, "\"skeleton\"") {
			t.Error("expected '\"skeleton\"' in JSON output")
		}
		if !strings.Contains(result, "\"count\"") {
			t.Error("expected '\"count\"' in JSON output")
		}
	})
}

// TestMapTypeFilter tests the type filter conversion
func TestMapTypeFilter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"F", "function"},
		{"f", "function"},
		{"T", "type"},
		{"t", "type"},
		{"M", "method"},
		{"m", "method"},
		{"C", "constant"},
		{"c", "constant"},
		{"V", "variable"},
		{"v", "variable"},
		{"I", "import"},
		{"i", "import"},
		{"X", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapTypeFilter(tt.input)
			if result != tt.expected {
				t.Errorf("mapTypeFilter(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGenerateSkeletonFromEntity tests skeleton generation fallback
func TestGenerateSkeletonFromEntity(t *testing.T) {
	tests := []struct {
		name     string
		entity   *store.Entity
		contains []string
	}{
		{
			name: "function",
			entity: &store.Entity{
				EntityType: "function",
				Name:       "TestFunc",
				Signature:  "(name: string) -> error",
			},
			contains: []string{"func", "TestFunc", "{ ... }"},
		},
		{
			name: "method with receiver",
			entity: &store.Entity{
				EntityType: "method",
				Name:       "Handle",
				Receiver:   "*Server",
				Signature:  "(ctx context.Context) -> error",
			},
			contains: []string{"func", "(s *Server)", "Handle", "{ ... }"},
		},
		{
			name: "struct type",
			entity: &store.Entity{
				EntityType: "type",
				Name:       "User",
				Kind:       "struct",
			},
			contains: []string{"type", "User", "struct"},
		},
		{
			name: "interface type",
			entity: &store.Entity{
				EntityType: "type",
				Name:       "Handler",
				Kind:       "interface",
			},
			contains: []string{"type", "Handler", "interface"},
		},
		{
			name: "constant",
			entity: &store.Entity{
				EntityType: "constant",
				Name:       "MaxSize",
			},
			contains: []string{"const", "MaxSize"},
		},
		{
			name: "function with doc comment",
			entity: &store.Entity{
				EntityType: "function",
				Name:       "Process",
				DocComment: "// Process handles the input",
				Signature:  "(input string) -> (string, error)",
			},
			contains: []string{"// Process handles the input", "func", "Process"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSkeletonFromEntity(tt.entity)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("generateSkeletonFromEntity() result %q does not contain %q", result, s)
				}
			}
		})
	}
}

// TestFormatSignatureForSkeleton tests signature conversion
func TestFormatSignatureForSkeleton(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"()", "()"},
		{"(name: string) -> error", "(name string) error"},
		{"(a: int, b: int) -> (int, error)", "(a int, b int) (int, error)"},
		{"", "()"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatSignatureForSkeleton(tt.input)
			if result != tt.expected {
				t.Errorf("formatSignatureForSkeleton(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

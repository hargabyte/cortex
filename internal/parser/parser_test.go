package parser

import (
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

const testGoSource = `package main

import "fmt"

// Greeter is a simple interface for greeting.
type Greeter interface {
	Greet(name string) string
}

// SimpleGreeter implements Greeter.
type SimpleGreeter struct {
	prefix string
}

// Greet returns a greeting for the given name.
func (s *SimpleGreeter) Greet(name string) string {
	return s.prefix + name
}

// NewGreeter creates a new SimpleGreeter.
func NewGreeter(prefix string) *SimpleGreeter {
	return &SimpleGreeter{prefix: prefix}
}

func main() {
	g := NewGreeter("Hello, ")
	fmt.Println(g.Greet("World"))
}
`

func TestNewParser(t *testing.T) {
	t.Run("creates Go parser", func(t *testing.T) {
		p, err := NewParser(Go)
		if err != nil {
			t.Fatalf("NewParser(Go) failed: %v", err)
		}
		defer p.Close()

		if p.Language() != Go {
			t.Errorf("expected language %s, got %s", Go, p.Language())
		}
	})

	t.Run("rejects unsupported language", func(t *testing.T) {
		_, err := NewParser(Language("fortran"))
		if err == nil {
			t.Fatal("expected error for unsupported language")
		}

		if _, ok := err.(*UnsupportedLanguageError); !ok {
			t.Errorf("expected UnsupportedLanguageError, got %T", err)
		}
	})

	t.Run("rejects unknown language", func(t *testing.T) {
		_, err := NewParser(Language("cobol"))
		if err == nil {
			t.Fatal("expected error for unknown language")
		}
	})
}

func TestParser_Parse(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	t.Run("parses valid Go source", func(t *testing.T) {
		result, err := p.Parse([]byte(testGoSource))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		if result.Root == nil {
			t.Fatal("expected non-nil root node")
		}

		if result.Root.Type() != "source_file" {
			t.Errorf("expected root type 'source_file', got %q", result.Root.Type())
		}

		if result.Language != Go {
			t.Errorf("expected language %s, got %s", Go, result.Language)
		}
	})

	t.Run("preserves source", func(t *testing.T) {
		source := []byte(testGoSource)
		result, err := p.Parse(source)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		if string(result.Source) != string(source) {
			t.Error("source was not preserved")
		}
	})
}

func TestParseResult_FindNodesByType(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testGoSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	t.Run("finds function declarations", func(t *testing.T) {
		funcs := result.FindNodesByType("function_declaration")
		// main and NewGreeter are function declarations
		if len(funcs) < 1 {
			t.Errorf("expected at least 1 function_declaration, got %d", len(funcs))
		}

		// Verify we found the expected functions
		funcNames := make(map[string]bool)
		for _, fn := range funcs {
			// Get function name from the identifier child
			for i := uint32(0); i < fn.ChildCount(); i++ {
				child := fn.Child(int(i))
				if child.Type() == "identifier" {
					funcNames[result.NodeText(child)] = true
					break
				}
			}
		}

		if !funcNames["main"] {
			t.Error("expected to find 'main' function")
		}
		if !funcNames["NewGreeter"] {
			t.Error("expected to find 'NewGreeter' function")
		}
	})

	t.Run("finds method declarations", func(t *testing.T) {
		methods := result.FindNodesByType("method_declaration")
		// Greet is a method declaration
		if len(methods) < 1 {
			t.Errorf("expected at least 1 method_declaration, got %d", len(methods))
		}
	})

	t.Run("finds type declarations", func(t *testing.T) {
		types := result.FindNodesByType("type_declaration")
		if len(types) < 1 {
			t.Errorf("expected at least 1 type_declaration, got %d", len(types))
		}
	})

	t.Run("finds package clause", func(t *testing.T) {
		pkgs := result.FindNodesByType("package_clause")
		if len(pkgs) != 1 {
			t.Errorf("expected exactly 1 package_clause, got %d", len(pkgs))
		}
	})
}

func TestParseResult_WalkNodes(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testGoSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	t.Run("visits all nodes", func(t *testing.T) {
		count := 0
		result.WalkNodes(func(node *sitter.Node) bool {
			count++
			return true
		})

		if count == 0 {
			t.Error("expected to visit some nodes")
		}
	})

	t.Run("stops on false return", func(t *testing.T) {
		count := 0
		limit := 5
		result.WalkNodes(func(node *sitter.Node) bool {
			count++
			return count < limit
		})

		if count != limit {
			t.Errorf("expected to visit %d nodes, visited %d", limit, count)
		}
	})
}

func TestParseResult_NodeText(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testGoSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	// Find a package clause and check its text
	pkgs := result.FindNodesByType("package_clause")
	if len(pkgs) == 0 {
		t.Fatal("no package clause found")
	}

	text := result.NodeText(pkgs[0])
	if !strings.Contains(text, "package main") {
		t.Errorf("expected package clause text to contain 'package main', got %q", text)
	}
}

func TestParseResult_HasErrors(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	t.Run("valid source has no errors", func(t *testing.T) {
		result, err := p.Parse([]byte(testGoSource))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		if result.HasErrors() {
			t.Error("expected no parse errors for valid source")
		}
	})

	t.Run("invalid source has errors", func(t *testing.T) {
		invalidSource := `package main
func broken( {
	return
}
`
		result, err := p.Parse([]byte(invalidSource))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		if !result.HasErrors() {
			t.Error("expected parse errors for invalid source")
		}
	})
}

func TestIsGoEntityNode(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testGoSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	t.Run("identifies function declarations", func(t *testing.T) {
		funcs := result.FindNodesByType("function_declaration")
		if len(funcs) == 0 {
			t.Fatal("no functions found")
		}
		if !IsGoEntityNode(funcs[0]) {
			t.Error("function_declaration should be identified as entity")
		}
	})

	t.Run("identifies method declarations", func(t *testing.T) {
		methods := result.FindNodesByType("method_declaration")
		if len(methods) == 0 {
			t.Fatal("no methods found")
		}
		if !IsGoEntityNode(methods[0]) {
			t.Error("method_declaration should be identified as entity")
		}
	})

	t.Run("returns false for nil", func(t *testing.T) {
		if IsGoEntityNode(nil) {
			t.Error("nil should not be identified as entity")
		}
	})
}

func TestGetGoEntityType(t *testing.T) {
	p, err := NewParser(Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testGoSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	t.Run("maps function declaration to function", func(t *testing.T) {
		funcs := result.FindNodesByType("function_declaration")
		if len(funcs) == 0 {
			t.Fatal("no functions found")
		}
		if got := GetGoEntityType(funcs[0]); got != "function" {
			t.Errorf("expected 'function', got %q", got)
		}
	})

	t.Run("maps method declaration to method", func(t *testing.T) {
		methods := result.FindNodesByType("method_declaration")
		if len(methods) == 0 {
			t.Fatal("no methods found")
		}
		if got := GetGoEntityType(methods[0]); got != "method" {
			t.Errorf("expected 'method', got %q", got)
		}
	})

	t.Run("returns empty for unknown types", func(t *testing.T) {
		// Find a non-entity node (like identifier)
		ids := result.FindNodesByType("identifier")
		if len(ids) == 0 {
			t.Fatal("no identifiers found")
		}
		if got := GetGoEntityType(ids[0]); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

func TestParseError(t *testing.T) {
	t.Run("formats with file", func(t *testing.T) {
		err := &ParseError{
			Message: "syntax error",
			File:    "main.go",
			Line:    10,
			Column:  5,
		}
		expected := "main.go:10:5: syntax error"
		if got := err.Error(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("formats without file", func(t *testing.T) {
		err := &ParseError{
			Message: "syntax error",
			Line:    10,
			Column:  5,
		}
		expected := "10:5: syntax error"
		if got := err.Error(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestUnsupportedLanguageError(t *testing.T) {
	err := &UnsupportedLanguageError{Language: "fortran"}
	expected := "unsupported language: fortran"
	if got := err.Error(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

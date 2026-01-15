package parser

import (
	"testing"
)

func TestRubyParser(t *testing.T) {
	code := `
def hello(name)
  puts "Hello, #{name}"
end

class Greeter
  def initialize(name)
    @name = name
  end

  def greet
    puts "Hello, #{@name}"
  end
end
`

	p, err := NewParser(Ruby)
	if err != nil {
		t.Fatalf("Failed to create Ruby parser: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(code))
	if err != nil {
		t.Fatalf("Failed to parse Ruby code: %v", err)
	}
	defer result.Close()

	if result.Language != Ruby {
		t.Errorf("Expected language Ruby, got %s", result.Language)
	}

	if result.Root == nil {
		t.Fatal("Root node is nil")
	}

	if result.Root.Type() != "program" {
		t.Errorf("Expected root type 'program', got %s", result.Root.Type())
	}

	// Check that we can find method nodes
	methodNodes := result.FindNodesByType("method")
	if len(methodNodes) < 1 {
		t.Errorf("Expected at least 1 method node, got %d", len(methodNodes))
	}

	// Check that we can find class nodes
	classNodes := result.FindNodesByType("class")
	if len(classNodes) < 1 {
		t.Errorf("Expected at least 1 class node, got %d", len(classNodes))
	}
}

func TestRubyNodeTypes(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		isEntity bool
	}{
		{"method", "method", true},
		{"singleton_method", "singleton_method", true},
		{"class", "class", true},
		{"module", "module", true},
		{"constant", "constant", true},
		{"identifier", "identifier", false},
		{"string", "string", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't create a real node without parsing, so just test the map
			entityType, ok := RubyNodeTypes[tt.nodeType]
			if tt.isEntity {
				if !ok {
					t.Errorf("Expected %s to be in RubyNodeTypes", tt.nodeType)
				}
				if entityType == "" {
					t.Errorf("Expected %s to have a non-empty entity type", tt.nodeType)
				}
			} else {
				if ok {
					t.Errorf("Expected %s to NOT be in RubyNodeTypes", tt.nodeType)
				}
			}
		})
	}
}

func TestRubyRelevantNodeTypes(t *testing.T) {
	types := RubyRelevantNodeTypes()
	if len(types) == 0 {
		t.Error("Expected RubyRelevantNodeTypes to return non-empty list")
	}

	// Check that all expected types are present
	expectedTypes := map[string]bool{
		"method":           true,
		"singleton_method": true,
		"class":            true,
		"module":           true,
		"constant":         true,
		"call":             true, // for attr_accessor etc
	}

	for _, nodeType := range types {
		if !expectedTypes[nodeType] {
			t.Errorf("Unexpected node type in RubyRelevantNodeTypes: %s", nodeType)
		}
		delete(expectedTypes, nodeType)
	}

	if len(expectedTypes) > 0 {
		t.Errorf("Missing expected node types: %v", expectedTypes)
	}
}

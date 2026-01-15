package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

// newRubyParser creates a tree-sitter parser configured for Ruby.
func newRubyParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(ruby.GetLanguage())
	return parser, nil
}

// RubyNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var RubyNodeTypes = map[string]string{
	"method":           "method",
	"singleton_method": "singleton_method",
	"class":            "class",
	"module":           "module",
	"constant":         "constant",
	"call":             "call", // for attr_accessor etc
}

// IsRubyEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (method, class, module, etc.).
func IsRubyEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := RubyNodeTypes[node.Type()]
	return ok
}

// GetRubyEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetRubyEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return RubyNodeTypes[node.Type()]
}

// RubyRelevantNodeTypes returns the list of node types that represent
// code entities in Ruby source files.
func RubyRelevantNodeTypes() []string {
	types := make([]string, 0, len(RubyNodeTypes))
	for nodeType := range RubyNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

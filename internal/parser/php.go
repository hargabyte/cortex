package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

// newPHPParser creates a tree-sitter parser configured for PHP.
func newPHPParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(php.GetLanguage())
	return parser, nil
}

// PHPNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var PHPNodeTypes = map[string]string{
	"function_definition":   "function",
	"class_declaration":     "class",
	"interface_declaration": "interface",
	"trait_declaration":     "trait",
	"method_declaration":    "method",
	"property_declaration":  "property",
	"const_declaration":     "constant",
	"enum_declaration":      "enum",
	"namespace_definition":  "namespace",
}

// IsPHPEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (class, method, interface, etc.).
func IsPHPEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := PHPNodeTypes[node.Type()]
	return ok
}

// GetPHPEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetPHPEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return PHPNodeTypes[node.Type()]
}

// PHPRelevantNodeTypes returns the list of node types that represent
// code entities in PHP source files.
func PHPRelevantNodeTypes() []string {
	types := make([]string, 0, len(PHPNodeTypes))
	for nodeType := range PHPNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

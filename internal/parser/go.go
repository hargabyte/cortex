package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// newGoParser creates a tree-sitter parser configured for Go.
func newGoParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	return parser, nil
}

// GoNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var GoNodeTypes = map[string]string{
	"function_declaration": "function",
	"method_declaration":   "method",
	"type_declaration":     "type",
	"type_spec":            "type",
	"const_declaration":    "constant",
	"var_declaration":      "variable",
	"import_declaration":   "import",
	"package_clause":       "package",
	"struct_type":          "struct",
	"interface_type":       "interface",
}

// IsGoEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (function, type, method, etc.).
func IsGoEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := GoNodeTypes[node.Type()]
	return ok
}

// GetGoEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetGoEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return GoNodeTypes[node.Type()]
}

// GoRelevantNodeTypes returns the list of node types that represent
// code entities in Go source files.
func GoRelevantNodeTypes() []string {
	types := make([]string, 0, len(GoNodeTypes))
	for nodeType := range GoNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

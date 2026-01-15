package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
)

// newCParser creates a tree-sitter parser configured for C.
func newCParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(c.GetLanguage())
	return parser, nil
}

// CNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var CNodeTypes = map[string]string{
	"function_definition":  "function",
	"declaration":          "declaration", // for function declarations, vars, typedefs
	"struct_specifier":     "struct",
	"union_specifier":      "union",
	"enum_specifier":       "enum",
	"type_definition":      "typedef",
	"preproc_def":          "macro",
	"preproc_function_def": "macro_function",
}

// IsCEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (function, struct, enum, etc.).
func IsCEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := CNodeTypes[node.Type()]
	return ok
}

// GetCEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetCEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return CNodeTypes[node.Type()]
}

// CRelevantNodeTypes returns the list of node types that represent
// code entities in C source files.
func CRelevantNodeTypes() []string {
	types := make([]string, 0, len(CNodeTypes))
	for nodeType := range CNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

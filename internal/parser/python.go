package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// newPythonParser creates a tree-sitter parser configured for Python.
func newPythonParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	return parser, nil
}

// PythonNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var PythonNodeTypes = map[string]string{
	"function_definition":       "function",
	"class_definition":          "class",
	"decorated_definition":      "decorated",
	"import_statement":          "import",
	"import_from_statement":     "import",
	"assignment":                "variable",
	"expression_statement":      "expression",
	"module":                    "module",
}

// IsPythonEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (function, class, import, etc.).
func IsPythonEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := PythonNodeTypes[node.Type()]
	return ok
}

// GetPythonEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetPythonEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return PythonNodeTypes[node.Type()]
}

// PythonRelevantNodeTypes returns the list of node types that represent
// code entities in Python source files.
func PythonRelevantNodeTypes() []string {
	types := make([]string, 0, len(PythonNodeTypes))
	for nodeType := range PythonNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

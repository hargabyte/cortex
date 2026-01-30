package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// newTypeScriptParser creates a tree-sitter parser configured for TypeScript.
func newTypeScriptParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	return parser, nil
}

// newJavaScriptParser creates a tree-sitter parser configured for JavaScript.
func newJavaScriptParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())
	return parser, nil
}

// TypeScriptNodeTypes maps tree-sitter node types to semantic entity types for TypeScript.
var TypeScriptNodeTypes = map[string]string{
	// Functions
	"function_declaration":           "function",
	"function_expression":            "function",
	"arrow_function":                 "function",
	"generator_function_declaration": "function",

	// Methods
	"method_definition": "method",

	// Classes
	"class_declaration": "class",
	"class_expression":  "class",

	// Interfaces and types
	"interface_declaration":  "interface",
	"type_alias_declaration": "type",
	"enum_declaration":       "enum",

	// Variables and constants
	"lexical_declaration":  "variable", // const, let
	"variable_declaration": "variable", // var

	// Imports and exports
	"import_statement": "import",
	"export_statement": "export",
	"export_clause":    "export",
}

// IsTypeScriptEntityNode checks if a tree-sitter node represents a code entity.
func IsTypeScriptEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := TypeScriptNodeTypes[node.Type()]
	return ok
}

// GetTypeScriptEntityType returns the semantic entity type for a tree-sitter node.
func GetTypeScriptEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return TypeScriptNodeTypes[node.Type()]
}

// TypeScriptRelevantNodeTypes returns the list of node types that represent
// code entities in TypeScript/JavaScript source files.
func TypeScriptRelevantNodeTypes() []string {
	types := make([]string, 0, len(TypeScriptNodeTypes))
	for nodeType := range TypeScriptNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

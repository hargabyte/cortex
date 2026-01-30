package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
)

// newCppParser creates a tree-sitter parser configured for C++.
func newCppParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(cpp.GetLanguage())
	return parser, nil
}

// CppNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var CppNodeTypes = map[string]string{
	"function_definition":  "function",
	"declaration":          "declaration",
	"class_specifier":      "class",
	"struct_specifier":     "struct",
	"enum_specifier":       "enum",
	"namespace_definition": "namespace",
	"template_declaration": "template",
	"type_definition":      "typedef",
	"using_declaration":    "using",
	"alias_declaration":    "alias",
	"preproc_def":          "macro",
	"preproc_function_def": "macro_function",
}

// IsCppEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (function, class, struct, etc.).
func IsCppEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := CppNodeTypes[node.Type()]
	return ok
}

// GetCppEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetCppEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return CppNodeTypes[node.Type()]
}

// CppRelevantNodeTypes returns the list of node types that represent
// code entities in C++ source files.
func CppRelevantNodeTypes() []string {
	types := make([]string, 0, len(CppNodeTypes))
	for nodeType := range CppNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

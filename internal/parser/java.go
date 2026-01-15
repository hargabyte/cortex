package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// newJavaParser creates a tree-sitter parser configured for Java.
func newJavaParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	return parser, nil
}

// JavaNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var JavaNodeTypes = map[string]string{
	"class_declaration":       "class",
	"interface_declaration":   "interface",
	"enum_declaration":        "enum",
	"method_declaration":      "method",
	"constructor_declaration": "constructor",
	"field_declaration":       "field",
	"import_declaration":      "import",
	"package_declaration":     "package",
	"annotation_declaration":  "annotation",
	"record_declaration":      "record",
}

// IsJavaEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (class, method, interface, etc.).
func IsJavaEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := JavaNodeTypes[node.Type()]
	return ok
}

// GetJavaEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetJavaEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return JavaNodeTypes[node.Type()]
}

// JavaRelevantNodeTypes returns the list of node types that represent
// code entities in Java source files.
func JavaRelevantNodeTypes() []string {
	types := make([]string, 0, len(JavaNodeTypes))
	for nodeType := range JavaNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

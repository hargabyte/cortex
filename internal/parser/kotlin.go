package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// newKotlinParser creates a tree-sitter parser configured for Kotlin.
func newKotlinParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())
	return parser, nil
}

// KotlinNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var KotlinNodeTypes = map[string]string{
	"function_declaration":  "function",
	"class_declaration":     "class",
	"object_declaration":    "object",
	"interface_declaration": "interface",
	"property_declaration":  "property",
	"type_alias":            "typealias",
	"import_header":         "import",
	"package_header":        "package",
}

// IsKotlinEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (class, function, interface, etc.).
func IsKotlinEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := KotlinNodeTypes[node.Type()]
	return ok
}

// GetKotlinEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetKotlinEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return KotlinNodeTypes[node.Type()]
}

// KotlinRelevantNodeTypes returns the list of node types that represent
// code entities in Kotlin source files.
func KotlinRelevantNodeTypes() []string {
	types := make([]string, 0, len(KotlinNodeTypes))
	for nodeType := range KotlinNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

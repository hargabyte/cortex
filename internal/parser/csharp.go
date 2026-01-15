package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	csharp "github.com/smacker/go-tree-sitter/csharp"
)

// newCSharpParser creates a tree-sitter parser configured for C#.
func newCSharpParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(csharp.GetLanguage())
	return parser, nil
}

// CSharpNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var CSharpNodeTypes = map[string]string{
	"class_declaration":       "class",
	"interface_declaration":   "interface",
	"struct_declaration":      "struct",
	"record_declaration":      "record",
	"enum_declaration":        "enum",
	"method_declaration":      "method",
	"property_declaration":    "property",
	"event_declaration":       "event",
	"delegate_declaration":    "delegate",
	"namespace_declaration":   "namespace",
	"constructor_declaration": "constructor",
	"field_declaration":       "field",
	"using_directive":         "using",
}

// IsCSharpEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (class, method, interface, etc.).
func IsCSharpEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := CSharpNodeTypes[node.Type()]
	return ok
}

// GetCSharpEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetCSharpEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return CSharpNodeTypes[node.Type()]
}

// CSharpRelevantNodeTypes returns the list of node types that represent
// code entities in C# source files.
func CSharpRelevantNodeTypes() []string {
	types := make([]string, 0, len(CSharpNodeTypes))
	for nodeType := range CSharpNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

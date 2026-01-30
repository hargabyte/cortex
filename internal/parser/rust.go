package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

// newRustParser creates a tree-sitter parser configured for Rust.
func newRustParser() (*sitter.Parser, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(rust.GetLanguage())
	return parser, nil
}

// RustNodeTypes maps tree-sitter node types to semantic entity types.
// This is used to identify code entities when traversing the AST.
var RustNodeTypes = map[string]string{
	"function_item":           "function",
	"function_signature_item": "function", // extern fn declarations
	"impl_item":               "impl",
	"struct_item":             "struct",
	"enum_item":               "enum",
	"trait_item":              "trait",
	"type_item":               "type",
	"const_item":              "constant",
	"static_item":             "static",
	"use_declaration":         "import",
	"mod_item":                "module",
	"macro_definition":        "macro",
}

// IsRustEntityNode checks if a tree-sitter node represents a code entity
// that we want to extract (function, struct, impl, etc.).
func IsRustEntityNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	_, ok := RustNodeTypes[node.Type()]
	return ok
}

// GetRustEntityType returns the semantic entity type for a tree-sitter node,
// or an empty string if the node is not a recognized entity.
func GetRustEntityType(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return RustNodeTypes[node.Type()]
}

// RustRelevantNodeTypes returns the list of node types that represent
// code entities in Rust source files.
func RustRelevantNodeTypes() []string {
	types := make([]string, 0, len(RustNodeTypes))
	for nodeType := range RustNodeTypes {
		types = append(types, nodeType)
	}
	return types
}

// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements C++-specific call graph extraction.
package extract

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// CppCallGraphExtractor extracts dependencies from C++ AST
type CppCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewCppCallGraphExtractor creates a C++ call graph extractor
func NewCppCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *CppCallGraphExtractor {
	cge := &CppCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: make(map[string]*CallGraphEntity),
		entityByID:   make(map[string]*CallGraphEntity),
	}

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		cge.entityByName[e.Name] = e
		if e.QualifiedName != "" {
			cge.entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			cge.entityByID[e.ID] = e
		}
	}

	return cge
}

// ExtractDependencies extracts all dependencies from the parsed C++ code
func (cge *CppCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range cge.entities {
		entity := &cge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "function", "method":
			// Extract function calls
			callDeps := cge.extractFunctionCalls(entity)
			deps = append(deps, callDeps...)

			// Extract type references
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

		case "class", "struct", "type":
			// Extract inheritance (base classes)
			inheritDeps := cge.extractInheritance(entity)
			deps = append(deps, inheritDeps...)

			// Extract type references in fields
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call_expression nodes within a function
func (cge *CppCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := cge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for call_expression nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Regular function calls
		if nodeType == "call_expression" {
			// Get function being called
			callTarget := cge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] && !isCppBuiltinFunction(callTarget) {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: cge.nodeLocation(node),
				}

				// Check if call is conditional
				if cge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := cge.resolveTarget(callTarget); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Constructor calls (new expressions)
		if nodeType == "new_expression" {
			typeNode := cge.findNewExpressionType(node)
			if typeNode != nil {
				typeName := cge.nodeText(typeNode)
				if !seen[typeName] && !isCppBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  Instantiates,
						Location: cge.nodeLocation(node),
					}

					if target := cge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
		}

		return true
	})

	return deps
}

// extractTypeReferences finds type identifiers used in the entity
func (cge *CppCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers
		if nodeType == "type_identifier" {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !isCppBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  UsesType,
					Location: cge.nodeLocation(node),
				}

				// Try to resolve
				if target := cge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for qualified identifiers (Namespace::Type)
		if nodeType == "qualified_identifier" {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !isCppBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  UsesType,
					Location: cge.nodeLocation(node),
				}

				if target := cge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for class/struct/enum specifiers (e.g., struct MyStruct)
		if nodeType == "class_specifier" || nodeType == "struct_specifier" || nodeType == "enum_specifier" {
			// Get the type name
			nameNode := findChildByType(node, "type_identifier")
			if nameNode != nil {
				typeName := cge.nodeText(nameNode)
				if !seen[typeName] && !isCppBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  UsesType,
						Location: cge.nodeLocation(node),
					}

					if target := cge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
		}

		return true
	})

	return deps
}

// extractInheritance extracts base class dependencies from class/struct
func (cge *CppCallGraphExtractor) extractInheritance(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Find base_class_clause
	var baseClauseNode *sitter.Node
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "base_class_clause" {
			baseClauseNode = node
			return false
		}
		return true
	})

	if baseClauseNode == nil {
		return deps
	}

	// Extract base classes
	for i := uint32(0); i < baseClauseNode.ChildCount(); i++ {
		child := baseClauseNode.Child(int(i))
		if child.Type() == "type_identifier" || child.Type() == "qualified_identifier" {
			baseClass := cge.nodeText(child)
			if !isCppBuiltinType(baseClass) {
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   baseClass,
					DepType:  Extends,
					Location: cge.nodeLocation(child),
				}

				if target := cge.resolveTarget(baseClass); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
	}

	return deps
}

// isConditionalCall checks if a call is inside an if/switch statement
func (cge *CppCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_statement", "conditional_expression":
			return true
		case "function_definition":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isCppBuiltinType checks if type is a C++ builtin
func isCppBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Basic C types
		"int": true, "char": true, "short": true, "long": true,
		"float": true, "double": true, "void": true,
		"signed": true, "unsigned": true,

		// C++ specific types
		"bool": true, "wchar_t": true, "char8_t": true, "char16_t": true, "char32_t": true,
		"nullptr_t": true,

		// Fixed-width types from stdint.h
		"int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"intptr_t": true, "uintptr_t": true, "ptrdiff_t": true,

		// Size types
		"size_t": true, "ssize_t": true, "off_t": true,

		// Common standard library types (just the basic ones)
		"string": true, "vector": true, "map": true, "set": true,
		"list": true, "deque": true, "array": true,
		"unique_ptr": true, "shared_ptr": true, "weak_ptr": true,
		"pair": true, "tuple": true, "optional": true,
	}
	return builtins[name]
}

// isCppBuiltinFunction checks if function is a C++ standard library function
func isCppBuiltinFunction(name string) bool {
	// Include all C builtins
	builtins := map[string]bool{
		// C standard library (stdio.h)
		"printf": true, "fprintf": true, "sprintf": true, "snprintf": true,
		"scanf": true, "fscanf": true, "sscanf": true,
		"puts": true, "fputs": true, "gets": true, "fgets": true,
		"putchar": true, "fputc": true, "putc": true,
		"getchar": true, "fgetc": true, "getc": true,
		"fopen": true, "fclose": true, "fflush": true,
		"fread": true, "fwrite": true,

		// C standard library (stdlib.h)
		"malloc": true, "calloc": true, "realloc": true, "free": true,
		"exit": true, "abort": true, "atoi": true, "atol": true, "atof": true,

		// C standard library (string.h)
		"memcpy": true, "memmove": true, "memset": true, "memcmp": true,
		"strcpy": true, "strncpy": true, "strcat": true, "strncat": true,
		"strcmp": true, "strncmp": true, "strlen": true,

		// C++ specific functions
		"new": true, "delete": true, "sizeof": true, "typeid": true,
		"nullptr": true, "NULL": true,

		// C++ I/O
		"cout": true, "cin": true, "cerr": true, "clog": true,
		"endl": true, "flush": true,

		// C++ utilities
		"make_unique": true, "make_shared": true,
		"move": true, "forward": true, "swap": true,
		"begin": true, "end": true, "size": true, "empty": true,

		// Common STL algorithms
		"sort": true, "find": true, "copy": true, "transform": true,
		"accumulate": true, "for_each": true,
	}
	return builtins[name]
}

// Helper methods

// walkNode performs a depth-first walk of the AST
func (cge *CppCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		cge.walkNode(node.Child(int(i)), fn)
	}
}

// nodeText returns the source text for a node
func (cge *CppCallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || cge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	endByte := node.EndByte()
	if endByte > uint32(len(cge.result.Source)) {
		return ""
	}
	return node.Content(cge.result.Source)
}

// nodeLocation returns file:line for a node
func (cge *CppCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if cge.result.FilePath != "" {
		return fmt.Sprintf("%s:%d", cge.result.FilePath, line)
	}
	return fmt.Sprintf(":%d", line)
}

// extractCallTarget extracts the function name from a call_expression
func (cge *CppCallGraphExtractor) extractCallTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "call_expression" {
		return ""
	}

	// In C++, the function being called is typically the first child
	// call_expression structure:
	// - function (identifier, field_expression, qualified_identifier, or template_function)
	// - argument_list (arguments)

	funcNode := node.ChildByFieldName("function")
	if funcNode == nil && node.ChildCount() > 0 {
		funcNode = node.Child(0)
	}

	if funcNode == nil {
		return ""
	}

	switch funcNode.Type() {
	case "identifier":
		return cge.nodeText(funcNode)

	case "qualified_identifier":
		// Namespace::function or Class::method
		return cge.nodeText(funcNode)

	case "field_expression":
		// obj.method() or obj->method() - extract the field name
		// The field is the last identifier
		var fieldName string
		for i := uint32(0); i < funcNode.ChildCount(); i++ {
			child := funcNode.Child(int(i))
			if child.Type() == "field_identifier" {
				fieldName = cge.nodeText(child)
			}
		}
		if fieldName != "" {
			return fieldName
		}
		// Fallback: return the whole expression
		return cge.nodeText(funcNode)

	case "template_function":
		// template_function<T>() - extract function name
		idNode := findChildByType(funcNode, "identifier")
		if idNode != nil {
			return cge.nodeText(idNode)
		}
		return cge.nodeText(funcNode)

	case "parenthesized_expression":
		// Function pointer call: (*func_ptr)()
		return ""

	case "subscript_expression":
		// Array of function pointers: funcs[0]()
		return ""
	}

	return ""
}

// findNewExpressionType extracts the type from a new_expression
func (cge *CppCallGraphExtractor) findNewExpressionType(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// new expression structure: new Type(args) or new Type[size]
	// Look for type_identifier or qualified_identifier
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" || child.Type() == "qualified_identifier" {
			return child
		}
	}

	return nil
}

// findFunctionBody finds the compound_statement (block) in a function definition
func (cge *CppCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// For function_definition, the body is a compound_statement
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "compound_statement" {
			return child
		}
	}

	return nil
}

// resolveTarget attempts to resolve a target name to an entity
func (cge *CppCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without namespace prefix for qualified names
	if strings.Contains(name, "::") {
		parts := strings.Split(name, "::")
		// Try just the last part (method/function name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	// Try without struct/class prefix for qualified names
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (field name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

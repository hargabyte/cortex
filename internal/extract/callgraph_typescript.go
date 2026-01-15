// Package extract provides call graph extraction for TypeScript/JavaScript.
// This file implements dependency extraction for TypeScript and JavaScript code,
// handling function calls, type references, class inheritance, and method ownership.
package extract

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// TypeScriptCallGraphExtractor extracts dependencies from TypeScript/JavaScript AST
type TypeScriptCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewTypeScriptCallGraphExtractor creates a call graph extractor for TypeScript/JavaScript
func NewTypeScriptCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *TypeScriptCallGraphExtractor {
	cge := &TypeScriptCallGraphExtractor{
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

// ExtractDependencies extracts all dependencies from the parsed TypeScript/JavaScript code
func (cge *TypeScriptCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range cge.entities {
		entity := &cge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "function", "method", "arrow_function":
			// Extract function calls
			callDeps := cge.extractFunctionCalls(entity)
			deps = append(deps, callDeps...)

			// Extract type references
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract method owner (for methods)
			if entity.Type == "method" {
				if ownerDep := cge.extractMethodOwner(entity); ownerDep != nil {
					deps = append(deps, *ownerDep)
				}
			}

		case "class", "interface":
			// Extract class inheritance (extends/implements)
			classDeps := cge.extractClassRelations(entity)
			deps = append(deps, classDeps...)

			// Extract type references from class body
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call_expression and new_expression nodes within a function
func (cge *TypeScriptCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := cge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		// For arrow functions without block body, search the whole node
		bodyNode = entity.Node
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for call_expression and new_expression nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		nodeType := node.Type()

		if nodeType == "call_expression" {
			// Get function being called
			callTarget := cge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] && !cge.isBuiltinType(callTarget) {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: cge.nodeLocation(node),
				}

				// Parse qualified names
				if strings.Contains(callTarget, ".") {
					dep.ToQualified = callTarget
					parts := strings.Split(callTarget, ".")
					dep.ToName = parts[len(parts)-1]
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
		} else if nodeType == "new_expression" {
			// Constructor call: new ClassName()
			constructorTarget := cge.extractNewTarget(node)
			if constructorTarget != "" && !seen["new:"+constructorTarget] && !cge.isBuiltinType(constructorTarget) {
				seen["new:"+constructorTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   constructorTarget,
					DepType:  Calls,
					Location: cge.nodeLocation(node),
				}

				// Parse qualified names
				if strings.Contains(constructorTarget, ".") {
					dep.ToQualified = constructorTarget
					parts := strings.Split(constructorTarget, ".")
					dep.ToName = parts[len(parts)-1]
				}

				// Check if constructor call is conditional
				if cge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := cge.resolveTarget(constructorTarget); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		return true
	})

	return deps
}

// extractTypeReferences finds type identifiers used in function/class
func (cge *TypeScriptCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers (TypeScript type annotations)
		if nodeType == "type_identifier" {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !cge.isBuiltinType(typeName) {
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

		// Check for qualified types (e.g., namespace.Type)
		if nodeType == "nested_type_identifier" || nodeType == "member_expression" {
			// Only process member_expression in type context
			if nodeType == "member_expression" && !cge.isInTypeContext(node) {
				return true
			}

			typeName := cge.nodeText(node)
			if !seen[typeName] && !cge.isBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:      entity.ID,
					ToName:      typeName,
					ToQualified: typeName,
					DepType:     UsesType,
					Location:    cge.nodeLocation(node),
				}

				if target := cge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		return true
	})

	return deps
}

// extractClassRelations extracts extends/implements dependencies for classes
func (cge *TypeScriptCallGraphExtractor) extractClassRelations(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Walk the class/interface node looking for heritage clauses
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Handle class_heritage - contains both extends and implements clauses
		// We need to continue walking to find the nested clauses
		if nodeType == "class_heritage" {
			return true // Continue walking to find extends_clause and implements_clause
		}

		// Handle extends_clause (class extends)
		if nodeType == "extends_clause" {
			cge.walkNode(node, func(child *sitter.Node) bool {
				// Look for the type being extended
				if child.Type() == "identifier" || child.Type() == "type_identifier" {
					typeName := cge.nodeText(child)
					if !cge.isBuiltinType(typeName) {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Extends,
							Location: cge.nodeLocation(child),
						}

						if target := cge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
				// Handle qualified extends (e.g., extends namespace.BaseClass)
				if child.Type() == "member_expression" || child.Type() == "nested_type_identifier" {
					typeName := cge.nodeText(child)
					if !cge.isBuiltinType(typeName) {
						dep := Dependency{
							FromID:      entity.ID,
							ToName:      typeName,
							ToQualified: typeName,
							DepType:     Extends,
							Location:    cge.nodeLocation(child),
						}

						if target := cge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
				return true
			})
			// Don't descend further from extends_clause - we handled it
			return false
		}

		// Handle implements_clause
		if nodeType == "implements_clause" {
			cge.walkNode(node, func(child *sitter.Node) bool {
				// Look for the interfaces being implemented
				if child.Type() == "identifier" || child.Type() == "type_identifier" {
					typeName := cge.nodeText(child)
					if !cge.isBuiltinType(typeName) {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Implements,
							Location: cge.nodeLocation(child),
						}

						if target := cge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
				// Handle qualified implements
				if child.Type() == "member_expression" || child.Type() == "nested_type_identifier" {
					typeName := cge.nodeText(child)
					if !cge.isBuiltinType(typeName) {
						dep := Dependency{
							FromID:      entity.ID,
							ToName:      typeName,
							ToQualified: typeName,
							DepType:     Implements,
							Location:    cge.nodeLocation(child),
						}

						if target := cge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
				return true
			})
			// Don't descend further from implements_clause - we handled it
			return false
		}

		// Handle interface extends (extends in interfaces means extending another interface)
		if nodeType == "extends_type_clause" {
			cge.walkNode(node, func(child *sitter.Node) bool {
				if child.Type() == "identifier" || child.Type() == "type_identifier" {
					typeName := cge.nodeText(child)
					if !cge.isBuiltinType(typeName) {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Extends,
							Location: cge.nodeLocation(child),
						}

						if target := cge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
				return true
			})
			return false
		}

		return true
	})

	return deps
}

// extractMethodOwner extracts method_of dependency for methods
func (cge *TypeScriptCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Walk up to find the containing class
	parent := entity.Node.Parent()
	for parent != nil {
		parentType := parent.Type()
		if parentType == "class_declaration" || parentType == "class" {
			// Find the class name
			className := cge.extractClassName(parent)
			if className != "" {
				dep := &Dependency{
					FromID:   entity.ID,
					ToName:   className,
					DepType:  MethodOf,
					Location: entity.Location,
				}

				// Try to resolve
				if target := cge.resolveTarget(className); target != nil {
					dep.ToID = target.ID
				}

				return dep
			}
		}
		parent = parent.Parent()
	}

	return nil
}

// isBuiltinType checks if type is a TypeScript/JavaScript builtin
func (cge *TypeScriptCallGraphExtractor) isBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Primitive types
		"string": true, "number": true, "boolean": true, "bigint": true,
		"symbol": true, "any": true, "void": true, "null": true,
		"undefined": true, "never": true, "unknown": true, "object": true,

		// Common global objects
		"Array": true, "Object": true, "Function": true, "Promise": true,
		"Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
		"Error": true, "TypeError": true, "RangeError": true, "SyntaxError": true,
		"ReferenceError": true, "EvalError": true, "URIError": true,
		"Date": true, "RegExp": true, "JSON": true, "Math": true,
		"console": true, "window": true, "document": true, "global": true,
		"process": true,

		// TypeScript utility types
		"Partial": true, "Required": true, "Readonly": true, "Record": true,
		"Pick": true, "Omit": true, "Exclude": true, "Extract": true,
		"NonNullable": true, "Parameters": true, "ConstructorParameters": true,
		"ReturnType": true, "InstanceType": true, "ThisParameterType": true,
		"OmitThisParameter": true, "ThisType": true, "Uppercase": true,
		"Lowercase": true, "Capitalize": true, "Uncapitalize": true,

		// Typed arrays
		"ArrayBuffer": true, "SharedArrayBuffer": true, "DataView": true,
		"Int8Array": true, "Uint8Array": true, "Uint8ClampedArray": true,
		"Int16Array": true, "Uint16Array": true, "Int32Array": true,
		"Uint32Array": true, "Float32Array": true, "Float64Array": true,
		"BigInt64Array": true, "BigUint64Array": true,

		// Async/Iterator types
		"AsyncFunction": true, "Generator": true, "GeneratorFunction": true,
		"AsyncGenerator": true, "AsyncGeneratorFunction": true,
		"Iterator": true, "AsyncIterator": true, "Iterable": true,
		"AsyncIterable": true, "IterableIterator": true,

		// Common DOM types (subset)
		"Element": true, "HTMLElement": true, "Event": true, "EventTarget": true,
		"Node": true, "NodeList": true, "Document": true, "Window": true,
	}
	return builtins[name]
}

// isConditionalCall checks if a call is inside an if/switch/ternary statement
func (cge *TypeScriptCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_statement", "ternary_expression", "conditional_expression":
			return true
		case "function_declaration", "method_definition", "arrow_function",
			"function_expression", "class_declaration":
			return false // Reached function/class boundary
		}
		parent = parent.Parent()
	}
	return false
}

// Helper methods

// walkNode performs a depth-first walk of the AST
func (cge *TypeScriptCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
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
func (cge *TypeScriptCallGraphExtractor) nodeText(node *sitter.Node) string {
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
func (cge *TypeScriptCallGraphExtractor) nodeLocation(node *sitter.Node) string {
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
func (cge *TypeScriptCallGraphExtractor) extractCallTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "call_expression" {
		return ""
	}

	// The function being called is typically the first child
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil && node.ChildCount() > 0 {
		funcNode = node.Child(0)
	}

	if funcNode == nil {
		return ""
	}

	return cge.extractExpressionName(funcNode)
}

// extractNewTarget extracts the constructor name from a new_expression
func (cge *TypeScriptCallGraphExtractor) extractNewTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "new_expression" {
		return ""
	}

	// Find the constructor - typically the second child after 'new' keyword
	constructorNode := node.ChildByFieldName("constructor")
	if constructorNode == nil {
		// Try to find the identifier or member_expression after 'new'
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			childType := child.Type()
			if childType == "identifier" || childType == "member_expression" {
				constructorNode = child
				break
			}
		}
	}

	if constructorNode == nil {
		return ""
	}

	return cge.extractExpressionName(constructorNode)
}

// extractExpressionName extracts a name from an expression node
func (cge *TypeScriptCallGraphExtractor) extractExpressionName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "identifier":
		return cge.nodeText(node)

	case "member_expression":
		// e.g., obj.method or module.function
		// For chained calls like a.b().c(), we want the full path
		return cge.nodeText(node)

	case "call_expression":
		// Chained call: a.b().c() - extract from the inner call
		// The call itself returns something, so we can't really name it
		return ""

	case "parenthesized_expression":
		// e.g., (funcPtr)()
		return ""

	case "subscript_expression":
		// e.g., funcs[0]()
		return ""

	case "this":
		return "this"

	case "super":
		return "super"
	}

	return ""
}

// extractClassName finds the class name from a class declaration
func (cge *TypeScriptCallGraphExtractor) extractClassName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Look for name field
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return cge.nodeText(nameNode)
	}

	// Fallback: look for identifier child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" || child.Type() == "type_identifier" {
			return cge.nodeText(child)
		}
	}

	return ""
}

// findFunctionBody finds the block/statement_block node in a function
func (cge *TypeScriptCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// Try field name first
	if body := node.ChildByFieldName("body"); body != nil {
		return body
	}

	// Search for statement_block or block child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()
		if childType == "statement_block" || childType == "block" {
			return child
		}
	}

	return nil
}

// isInTypeContext checks if a member_expression is in a type context
func (cge *TypeScriptCallGraphExtractor) isInTypeContext(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "type_annotation", "type_alias_declaration", "interface_declaration",
			"type_parameter", "constraint", "default_type", "as_expression",
			"satisfies_expression", "implements_clause", "extends_clause",
			"extends_type_clause":
			return true
		case "call_expression", "new_expression":
			return false // In value context
		}
		parent = parent.Parent()
	}
	return false
}

// resolveTarget attempts to resolve a target name to an entity
func (cge *TypeScriptCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	// First try exact match
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without module/namespace prefix for qualified names
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (method/function/class name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements Kotlin-specific call graph extraction.
package extract

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// KotlinCallGraphExtractor extracts dependencies from Kotlin AST
type KotlinCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewKotlinCallGraphExtractor creates a Kotlin call graph extractor
func NewKotlinCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *KotlinCallGraphExtractor {
	kcge := &KotlinCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: make(map[string]*CallGraphEntity),
		entityByID:   make(map[string]*CallGraphEntity),
	}

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		kcge.entityByName[e.Name] = e
		if e.QualifiedName != "" {
			kcge.entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			kcge.entityByID[e.ID] = e
		}
	}

	return kcge
}

// NewKotlinCallGraphExtractorWithMaps creates an extractor with pre-built lookup maps
func NewKotlinCallGraphExtractorWithMaps(result *parser.ParseResult, entities []CallGraphEntity,
	entityByName map[string]*CallGraphEntity, entityByID map[string]*CallGraphEntity) *KotlinCallGraphExtractor {
	return &KotlinCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// ExtractDependencies extracts all dependencies from the parsed Kotlin code
func (kcge *KotlinCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range kcge.entities {
		entity := &kcge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "function", "method":
			// Extract function calls
			callDeps := kcge.extractFunctionCalls(entity)
			deps = append(deps, callDeps...)

			// Extract object creation (constructor calls)
			newDeps := kcge.extractObjectCreation(entity)
			deps = append(deps, newDeps...)

			// Extract type references (parameters, return types, local variables)
			typeDeps := kcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract function owner relationship
			if ownerDep := kcge.extractFunctionOwner(entity); ownerDep != nil {
				deps = append(deps, *ownerDep)
			}

		case "class", "interface", "object":
			// Extract class inheritance (extends) and interface implementation
			classDeps := kcge.extractClassRelations(entity)
			deps = append(deps, classDeps...)

			// Extract type references in property declarations
			typeDeps := kcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call_expression nodes within a function
func (kcge *KotlinCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := kcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for call_expression nodes
	kcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "call_expression" {
			callTarget, qualified := kcge.extractCallExpressionTarget(node)
			if callTarget != "" && !seen[callTarget] {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: kcge.nodeLocation(node),
				}

				if qualified != "" {
					dep.ToQualified = qualified
				}

				// Check if call is conditional
				if kcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := kcge.resolveTarget(callTarget); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractObjectCreation extracts constructor invocations
func (kcge *KotlinCallGraphExtractor) extractObjectCreation(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := kcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for constructor_invocation nodes
	kcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "constructor_invocation" {
			typeName := kcge.extractConstructorType(node)
			if typeName != "" && !seen[typeName] && !kcge.isKotlinBuiltinType(typeName) {
				seen[typeName] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  Calls, // Constructor call
					Location: kcge.nodeLocation(node),
				}

				// Check if call is conditional
				if kcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := kcge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractTypeReferences finds type references used in the entity
func (kcge *KotlinCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	kcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers in type contexts
		if nodeType == "user_type" {
			// user_type is Kotlin's AST node for custom types
			if kcge.isInTypeContext(node) {
				typeName := kcge.extractUserType(node)
				if !seen[typeName] && !kcge.isKotlinBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  UsesType,
						Location: kcge.nodeLocation(node),
					}

					// Try to resolve
					if target := kcge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
		}

		// Check for simple type identifiers
		if nodeType == "simple_identifier" {
			// Make sure we're in a type context
			if kcge.isInTypeContext(node) {
				typeName := kcge.nodeText(node)
				if !seen[typeName] && !kcge.isKotlinBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  UsesType,
						Location: kcge.nodeLocation(node),
					}

					if target := kcge.resolveTarget(typeName); target != nil {
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

// extractClassRelations extracts inheritance and interface implementation relationships
func (kcge *KotlinCallGraphExtractor) extractClassRelations(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Look for delegation_specifiers (contains superclass and interfaces)
	kcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "delegation_specifiers" {
			// Find all delegation specifiers
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))

				// Constructor invocation = superclass
				if child.Type() == "constructor_invocation" {
					typeName := kcge.extractConstructorType(child)
					if typeName != "" && !kcge.isKotlinBuiltinType(typeName) {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Extends,
							Location: kcge.nodeLocation(child),
						}

						if target := kcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}

				// user_type = interface (for classes) or superinterface (for interfaces)
				if child.Type() == "user_type" {
					typeName := kcge.extractUserType(child)
					if typeName != "" && !kcge.isKotlinBuiltinType(typeName) {
						depType := Implements
						if entity.Type == "interface" {
							depType = Extends // Interface extends interface
						}

						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  depType,
							Location: kcge.nodeLocation(child),
						}

						if target := kcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
			}
			return false // Don't recurse further
		}
		return true
	})

	return deps
}

// extractFunctionOwner extracts function_of dependency for functions
func (kcge *KotlinCallGraphExtractor) extractFunctionOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find the containing class/object/interface by walking up the tree
	parent := entity.Node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "class_body":
			// Go one more level up to get the class/object/interface declaration
			parent = parent.Parent()
			if parent != nil {
				className := kcge.extractClassName(parent)
				if className != "" {
					dep := &Dependency{
						FromID:   entity.ID,
						ToName:   className,
						DepType:  MethodOf,
						Location: entity.Location,
					}

					// Try to resolve
					if target := kcge.resolveTarget(className); target != nil {
						dep.ToID = target.ID
					}

					return dep
				}
			}
			return nil
		}
		parent = parent.Parent()
	}

	return nil
}

// extractCallExpressionTarget extracts the function name and qualifier from a call_expression
func (kcge *KotlinCallGraphExtractor) extractCallExpressionTarget(node *sitter.Node) (functionName string, qualified string) {
	if node == nil || node.Type() != "call_expression" {
		return "", ""
	}

	// call_expression structure:
	// - simple_identifier (function name)
	// - navigation_expression (for obj.method())
	// - value_arguments (the arguments)

	// Look for the callee expression
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		// Direct function call: functionName()
		if childType == "simple_identifier" {
			functionName = kcge.nodeText(child)
			return functionName, ""
		}

		// Navigation expression: obj.method() or receiver?.method()
		if childType == "navigation_expression" {
			// Extract receiver and method name
			receiver, method := kcge.extractNavigationExpression(child)
			if method != "" {
				if receiver != "" {
					qualified = receiver + "." + method
					return method, qualified
				}
				return method, ""
			}
		}
	}

	return "", ""
}

// extractNavigationExpression extracts receiver and method from navigation_expression
func (kcge *KotlinCallGraphExtractor) extractNavigationExpression(node *sitter.Node) (receiver string, method string) {
	if node == nil || node.Type() != "navigation_expression" {
		return "", ""
	}

	// navigation_expression contains the receiver and the member being accessed
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		// The first significant child is usually the receiver
		if i == 0 && (childType == "simple_identifier" || childType == "navigation_expression" || childType == "call_expression") {
			receiver = kcge.nodeText(child)
		}

		// simple_identifier after navigation operator is the member name
		if childType == "simple_identifier" && i > 0 {
			method = kcge.nodeText(child)
		}

		// Nested navigation_expression
		if childType == "navigation_expression" && i > 0 {
			_, nestedMethod := kcge.extractNavigationExpression(child)
			if nestedMethod != "" {
				method = nestedMethod
			}
		}
	}

	return receiver, method
}

// extractConstructorType extracts the type name from a constructor_invocation
func (kcge *KotlinCallGraphExtractor) extractConstructorType(node *sitter.Node) string {
	if node == nil || node.Type() != "constructor_invocation" {
		return ""
	}

	// constructor_invocation contains user_type or simple_identifier
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "user_type":
			return kcge.extractUserType(child)
		case "simple_identifier":
			return kcge.nodeText(child)
		}
	}

	return ""
}

// extractUserType extracts the type name from a user_type node
func (kcge *KotlinCallGraphExtractor) extractUserType(node *sitter.Node) string {
	if node == nil || node.Type() != "user_type" {
		return ""
	}

	// user_type can contain simple_identifier or nested user_type (for qualified names)
	var parts []string
	kcge.walkNode(node, func(child *sitter.Node) bool {
		if child.Type() == "simple_identifier" {
			parts = append(parts, kcge.nodeText(child))
		}
		return true
	})

	if len(parts) > 0 {
		// Return the simple name (last part)
		return parts[len(parts)-1]
	}

	return ""
}

// extractClassName extracts the class name from a class/interface/object declaration
func (kcge *KotlinCallGraphExtractor) extractClassName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "class_declaration", "interface_declaration", "object_declaration":
		nameNode := findChildByType(node, "simple_identifier")
		if nameNode != nil {
			return kcge.nodeText(nameNode)
		}
	}

	return ""
}

// findFunctionBody finds the function_body node in a function_declaration
func (kcge *KotlinCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// Look for function_body child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "function_body" {
			return child
		}
	}

	return nil
}

// isConditionalCall checks if a call is inside an if/when/try statement
func (kcge *KotlinCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_expression", "when_expression", "try_expression":
			return true
		case "function_declaration", "lambda_literal":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isInTypeContext checks if a node is in a type context
func (kcge *KotlinCallGraphExtractor) isInTypeContext(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	switch parent.Type() {
	// Direct type contexts
	case "parameter", "property_declaration", "function_declaration",
		"variable_declaration", "type", "user_type",
		"type_arguments", "type_parameter", "nullable_type",
		"delegation_specifiers", "constructor_invocation",
		"class_declaration", "interface_declaration", "object_declaration":
		return true
	}

	// Check grandparent for certain contexts
	grandparent := parent.Parent()
	if grandparent != nil {
		switch grandparent.Type() {
		case "parameter", "property_declaration", "variable_declaration":
			return true
		}
	}

	return false
}

// isKotlinBuiltinType checks if type is a Kotlin builtin or common library type
func (kcge *KotlinCallGraphExtractor) isKotlinBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Primitive types
		"String": true, "Int": true, "Long": true, "Double": true,
		"Float": true, "Boolean": true, "Byte": true, "Short": true,
		"Char": true, "Unit": true, "Any": true, "Nothing": true,

		// Collections
		"List": true, "MutableList": true, "Map": true, "MutableMap": true,
		"Set": true, "MutableSet": true, "Collection": true, "MutableCollection": true,
		"ArrayList": true, "HashMap": true, "HashSet": true,
		"LinkedHashMap": true, "LinkedHashSet": true,
		"Iterable": true, "Iterator": true, "Sequence": true,

		// Common types
		"Pair": true, "Triple": true, "Result": true, "Lazy": true,
		"Function": true, "Comparable": true, "Comparator": true,
		"Array": true, "IntArray": true, "LongArray": true, "DoubleArray": true,
		"BooleanArray": true, "CharArray": true, "ByteArray": true,

		// Common functions (built-in)
		"println": true, "print": true, "require": true, "check": true,
		"requireNotNull": true, "checkNotNull": true, "error": true,
		"listOf": true, "mutableListOf": true, "mapOf": true, "mutableMapOf": true,
		"setOf": true, "mutableSetOf": true, "arrayOf": true,
		"let": true, "apply": true, "run": true, "with": true, "also": true,
		"takeIf": true, "takeUnless": true, "repeat": true,
		"TODO": true,
	}

	return builtins[name]
}

// resolveTarget attempts to resolve a target name to an entity
func (kcge *KotlinCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := kcge.entityByName[name]; ok {
		return e
	}

	// Try without package prefix for qualified names
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (method/class name)
		if e, ok := kcge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

// walkNode performs a depth-first walk of the AST
func (kcge *KotlinCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		kcge.walkNode(node.Child(int(i)), fn)
	}
}

// nodeText returns the source text for a node
func (kcge *KotlinCallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || kcge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	endByte := node.EndByte()
	if endByte > uint32(len(kcge.result.Source)) {
		return ""
	}
	return node.Content(kcge.result.Source)
}

// nodeLocation returns file:line for a node
func (kcge *KotlinCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if kcge.result.FilePath != "" {
		return kcge.result.FilePath + ":" + itoa(int(line))
	}
	return ":" + itoa(int(line))
}

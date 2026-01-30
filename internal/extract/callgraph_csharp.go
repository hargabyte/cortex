// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements C#-specific call graph extraction.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// CSharpCallGraphExtractor extracts dependencies from C# AST
type CSharpCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewCSharpCallGraphExtractor creates a C# call graph extractor
func NewCSharpCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *CSharpCallGraphExtractor {
	cge := &CSharpCallGraphExtractor{
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

// NewCSharpCallGraphExtractorWithMaps creates an extractor with pre-built lookup maps
func NewCSharpCallGraphExtractorWithMaps(result *parser.ParseResult, entities []CallGraphEntity,
	entityByName map[string]*CallGraphEntity, entityByID map[string]*CallGraphEntity) *CSharpCallGraphExtractor {
	return &CSharpCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// ExtractDependencies extracts all dependencies from the parsed C# code
func (cge *CSharpCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range cge.entities {
		entity := &cge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "method", "constructor":
			// Extract method calls (invocation_expression)
			callDeps := cge.extractMethodCalls(entity)
			deps = append(deps, callDeps...)

			// Extract object creation (object_creation_expression)
			newDeps := cge.extractObjectCreation(entity)
			deps = append(deps, newDeps...)

			// Extract type references (parameters, return types, local variables)
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract method owner relationship
			if ownerDep := cge.extractMethodOwner(entity); ownerDep != nil {
				deps = append(deps, *ownerDep)
			}

		case "class", "interface", "struct", "record", "enum":
			// Extract class inheritance (extends) and interface implementation (implements)
			classDeps := cge.extractClassRelations(entity)
			deps = append(deps, classDeps...)

			// Extract type references in field declarations
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractMethodCalls finds invocation_expression nodes within a method/constructor
func (cge *CSharpCallGraphExtractor) extractMethodCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get method body
	bodyNode := cge.findMethodBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk method body looking for invocation_expression nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "invocation_expression" {
			callTarget, qualified := cge.extractInvocationTarget(node)
			if callTarget != "" && !seen[callTarget] {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: cge.nodeLocation(node),
				}

				if qualified != "" {
					dep.ToQualified = qualified
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
		return true
	})

	return deps
}

// extractObjectCreation extracts object_creation_expression (new ClassName()) nodes
func (cge *CSharpCallGraphExtractor) extractObjectCreation(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get method body
	bodyNode := cge.findMethodBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track to deduplicate
	seen := make(map[string]bool)

	// Walk method body looking for object_creation_expression nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "object_creation_expression" {
			typeName := cge.extractCreatedTypeName(node)
			if typeName != "" && !seen[typeName] && !cge.isCSharpBuiltinType(typeName) {
				seen[typeName] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  Calls, // Constructor call
					Location: cge.nodeLocation(node),
				}

				// Check if call is conditional
				if cge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
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

// extractTypeReferences finds type nodes used in the entity
func (cge *CSharpCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers in type contexts
		if nodeType == "identifier_name" || nodeType == "identifier" {
			// Make sure we're in a type context (not just an identifier)
			if cge.isInTypeContext(node) {
				typeName := cge.nodeText(node)
				if !seen[typeName] && !cge.isCSharpBuiltinType(typeName) {
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
		}

		// Check for generic types like List<String>
		if nodeType == "generic_name" {
			// Extract the outer type (e.g., List from List<String>)
			outerType := cge.extractGenericOuterType(node)
			if outerType != "" && !seen[outerType] && !cge.isCSharpBuiltinType(outerType) {
				seen[outerType] = true
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   outerType,
					DepType:  UsesType,
					Location: cge.nodeLocation(node),
				}

				if target := cge.resolveTarget(outerType); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for qualified types like Namespace.ClassName
		if nodeType == "qualified_name" {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !cge.isCSharpBuiltinType(typeName) {
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

		// Check for predefined_type usage
		if nodeType == "predefined_type" {
			// Skip builtin types
			return true
		}

		return true
	})

	return deps
}

// extractClassRelations extracts extends and implements relationships
func (cge *CSharpCallGraphExtractor) extractClassRelations(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Look for base_list (contains base class and interfaces)
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "base_list" {
			isFirst := true
			// Walk children to find base types
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				childType := child.Type()

				// Look for type references in base list
				if childType == "simple_base_type" || childType == "identifier_name" ||
					childType == "identifier" || childType == "generic_name" ||
					childType == "qualified_name" {
					typeName := cge.extractTypeFromNode(child)
					if typeName != "" && !cge.isCSharpBuiltinType(typeName) {
						// First type after colon is usually base class (for classes)
						// or first interface (for interfaces)
						depType := Implements
						if isFirst && (entity.Type == "class" || entity.Type == "record") {
							depType = Extends
							isFirst = false
						} else if entity.Type == "interface" {
							depType = Extends // Interface extends interface
						}

						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  depType,
							Location: cge.nodeLocation(child),
						}

						if target := cge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
			}
			return false // Don't recurse further into base_list
		}
		return true
	})

	return deps
}

// extractMethodOwner extracts method_of dependency for methods
func (cge *CSharpCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find the containing class/struct/interface by walking up the tree
	parent := entity.Node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "declaration_list":
			// Go one more level up to get the class/interface declaration
			parent = parent.Parent()
			if parent != nil {
				className := cge.extractTypeName(parent)
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
			return nil
		}
		parent = parent.Parent()
	}

	return nil
}

// extractInvocationTarget extracts the method name and qualifier from an invocation_expression
func (cge *CSharpCallGraphExtractor) extractInvocationTarget(node *sitter.Node) (methodName string, qualified string) {
	if node == nil || node.Type() != "invocation_expression" {
		return "", ""
	}

	// invocation_expression structure:
	// - expression (can be identifier, member_access_expression, etc.)
	// - argument_list

	var receiver string
	var name string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "identifier_name" || childType == "identifier" {
			name = cge.nodeText(child)
		} else if childType == "member_access_expression" {
			// e.g., obj.Method or namespace.Type.Method
			name, receiver = cge.extractMemberAccess(child)
		} else if childType == "generic_name" {
			// e.g., Method<T>()
			name = cge.extractGenericOuterType(child)
		}
	}

	if name == "" {
		return "", ""
	}

	if receiver != "" {
		qualified = receiver + "." + name
		return name, qualified
	}

	return name, ""
}

// extractMemberAccess extracts name and receiver from a member_access_expression
func (cge *CSharpCallGraphExtractor) extractMemberAccess(node *sitter.Node) (name string, receiver string) {
	if node == nil || node.Type() != "member_access_expression" {
		return "", ""
	}

	// Structure: expression.name
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		name = cge.nodeText(nameNode)
	}

	exprNode := node.ChildByFieldName("expression")
	if exprNode != nil {
		receiver = cge.nodeText(exprNode)
	}

	// Fallback: iterate children
	if name == "" || receiver == "" {
		var parts []string
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			childType := child.Type()
			if childType == "identifier_name" || childType == "identifier" ||
				childType == "generic_name" || childType == "predefined_type" ||
				childType == "this_expression" {
				parts = append(parts, cge.nodeText(child))
			} else if childType == "member_access_expression" {
				parts = append(parts, cge.nodeText(child))
			}
		}
		if len(parts) >= 2 {
			name = parts[len(parts)-1]
			receiver = strings.Join(parts[:len(parts)-1], ".")
		} else if len(parts) == 1 {
			name = parts[0]
		}
	}

	return name, receiver
}

// extractCreatedTypeName extracts the type name from an object_creation_expression
func (cge *CSharpCallGraphExtractor) extractCreatedTypeName(node *sitter.Node) string {
	if node == nil || node.Type() != "object_creation_expression" {
		return ""
	}

	// object_creation_expression structure:
	// - "new" keyword
	// - type (identifier_name, generic_name, qualified_name)
	// - argument_list (optional)
	// - initializer (optional)

	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		return cge.extractTypeFromNode(typeNode)
	}

	// Fallback: look for type identifiers
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier_name", "identifier":
			return cge.nodeText(child)
		case "generic_name":
			return cge.extractGenericOuterType(child)
		case "qualified_name":
			return cge.nodeText(child)
		}
	}

	return ""
}

// extractTypeFromNode extracts the type name from various type node types
func (cge *CSharpCallGraphExtractor) extractTypeFromNode(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "identifier_name", "identifier":
		return cge.nodeText(node)
	case "generic_name":
		return cge.extractGenericOuterType(node)
	case "qualified_name":
		// Get the last identifier in the qualified name
		for i := int(node.ChildCount()) - 1; i >= 0; i-- {
			child := node.Child(i)
			if child.Type() == "identifier_name" || child.Type() == "identifier" {
				return cge.nodeText(child)
			}
		}
		return cge.nodeText(node)
	case "simple_base_type":
		// Extract type from simple_base_type
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "identifier_name" || child.Type() == "generic_name" ||
				child.Type() == "qualified_name" {
				return cge.extractTypeFromNode(child)
			}
		}
	case "nullable_type":
		// Extract the underlying type from nullable
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() != "?" {
				return cge.extractTypeFromNode(child)
			}
		}
	case "array_type":
		// Extract the element type
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			childType := child.Type()
			if childType == "identifier_name" || childType == "generic_name" ||
				childType == "predefined_type" {
				return cge.extractTypeFromNode(child)
			}
		}
	case "predefined_type":
		return cge.nodeText(node)
	}

	return ""
}

// extractGenericOuterType extracts the outer type from a generic_name (e.g., List from List<String>)
func (cge *CSharpCallGraphExtractor) extractGenericOuterType(node *sitter.Node) string {
	if node == nil || node.Type() != "generic_name" {
		return ""
	}

	// generic_name structure:
	// - identifier (the outer type)
	// - type_argument_list (contains the generic params)

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier_name" || child.Type() == "identifier" {
			return cge.nodeText(child)
		}
	}

	return ""
}

// extractTypeName extracts the type name from a class/interface/struct declaration
func (cge *CSharpCallGraphExtractor) extractTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "class_declaration", "interface_declaration", "struct_declaration",
		"record_declaration", "enum_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			return cge.nodeText(nameNode)
		}
		// Fallback: look for identifier child
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "identifier" || child.Type() == "identifier_name" {
				return cge.nodeText(child)
			}
		}
	}

	return ""
}

// findMethodBody finds the block node in a method/constructor declaration
func (cge *CSharpCallGraphExtractor) findMethodBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// Try field name first
	if body := node.ChildByFieldName("body"); body != nil {
		return body
	}

	// Search for block child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "block" {
			return child
		}
		// Also check for arrow_expression_clause (expression-bodied members)
		if child.Type() == "arrow_expression_clause" {
			return child
		}
	}

	return nil
}

// isConditionalCall checks if a call is inside an if/switch/ternary statement
func (cge *CSharpCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_statement", "switch_expression",
			"conditional_expression", "switch_section":
			return true
		case "method_declaration", "constructor_declaration", "local_function_statement",
			"lambda_expression", "anonymous_method_expression":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isInTypeContext checks if an identifier is in a type context
func (cge *CSharpCallGraphExtractor) isInTypeContext(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	switch parent.Type() {
	// Direct type contexts
	case "parameter", "variable_declaration", "field_declaration",
		"method_declaration", "constructor_declaration",
		"cast_expression", "is_pattern_expression", "as_expression",
		"class_declaration", "interface_declaration", "struct_declaration",
		"record_declaration", "base_list", "simple_base_type",
		"type_argument_list", "type_parameter", "array_type",
		"nullable_type", "pointer_type", "ref_type",
		"attribute", "property_declaration", "event_declaration",
		"indexer_declaration", "delegate_declaration",
		"local_declaration_statement", "typeof_expression":
		return true
	case "generic_name", "qualified_name":
		// These are type nodes themselves
		return true
	}

	return false
}

// isCSharpBuiltinType checks if type is a C# builtin or common library type
func (cge *CSharpCallGraphExtractor) isCSharpBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Primitive types (C# keywords)
		"int": true, "long": true, "double": true, "float": true,
		"bool": true, "byte": true, "short": true, "char": true,
		"void": true, "decimal": true, "sbyte": true, "ushort": true,
		"uint": true, "ulong": true, "object": true, "string": true,
		"dynamic": true, "var": true,

		// System types
		"String": true, "Int32": true, "Int64": true, "Double": true,
		"Float": true, "Boolean": true, "Byte": true, "Int16": true,
		"Char": true, "Void": true, "Decimal": true, "SByte": true,
		"UInt16": true, "UInt32": true, "UInt64": true, "Object": true,
		"Single": true,

		// Common classes
		"Exception": true, "ArgumentException": true, "ArgumentNullException": true,
		"InvalidOperationException": true, "NotImplementedException": true,
		"NotSupportedException": true, "NullReferenceException": true,
		"IndexOutOfRangeException": true, "FormatException": true,

		// Collections
		"List": true, "Dictionary": true, "HashSet": true, "Queue": true,
		"Stack": true, "LinkedList": true, "SortedList": true,
		"SortedDictionary": true, "SortedSet": true,
		"IList": true, "IDictionary": true, "ICollection": true,
		"IEnumerable": true, "IEnumerator": true, "IReadOnlyList": true,
		"IReadOnlyCollection": true, "IReadOnlyDictionary": true,

		// Other common types
		"Console": true, "Math": true, "Convert": true, "Array": true,
		"StringBuilder": true, "Guid": true, "DateTime": true,
		"TimeSpan": true, "Task": true, "ValueTask": true,
		"CancellationToken": true, "IDisposable": true, "IAsyncDisposable": true,
		"Action": true, "Func": true, "Predicate": true, "EventHandler": true,
		"Nullable": true, "Span": true, "Memory": true, "ReadOnlySpan": true,
		"ReadOnlyMemory": true, "Type": true, "Attribute": true,
		"Enum": true, "Tuple": true, "ValueTuple": true,
	}

	return builtins[name]
}

// resolveTarget attempts to resolve a target name to an entity
func (cge *CSharpCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without namespace prefix for qualified names
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (method/class name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

// walkNode performs a depth-first walk of the AST
func (cge *CSharpCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
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
func (cge *CSharpCallGraphExtractor) nodeText(node *sitter.Node) string {
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
func (cge *CSharpCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if cge.result.FilePath != "" {
		return cge.result.FilePath + ":" + itoa(int(line))
	}
	return ":" + itoa(int(line))
}

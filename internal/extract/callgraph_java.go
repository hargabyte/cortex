// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements Java-specific call graph extraction.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// JavaCallGraphExtractor extracts dependencies from Java AST
type JavaCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewJavaCallGraphExtractor creates a Java call graph extractor
func NewJavaCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *JavaCallGraphExtractor {
	jcge := &JavaCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: make(map[string]*CallGraphEntity),
		entityByID:   make(map[string]*CallGraphEntity),
	}

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		jcge.entityByName[e.Name] = e
		if e.QualifiedName != "" {
			jcge.entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			jcge.entityByID[e.ID] = e
		}
	}

	return jcge
}

// NewJavaCallGraphExtractorWithMaps creates an extractor with pre-built lookup maps
func NewJavaCallGraphExtractorWithMaps(result *parser.ParseResult, entities []CallGraphEntity,
	entityByName map[string]*CallGraphEntity, entityByID map[string]*CallGraphEntity) *JavaCallGraphExtractor {
	return &JavaCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// ExtractDependencies extracts all dependencies from the parsed Java code
func (jcge *JavaCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range jcge.entities {
		entity := &jcge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "method", "constructor":
			// Extract method calls
			callDeps := jcge.extractMethodCalls(entity)
			deps = append(deps, callDeps...)

			// Extract object creation (new expressions)
			newDeps := jcge.extractObjectCreation(entity)
			deps = append(deps, newDeps...)

			// Extract type references (parameters, return types, local variables)
			typeDeps := jcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract method owner relationship
			if ownerDep := jcge.extractMethodOwner(entity); ownerDep != nil {
				deps = append(deps, *ownerDep)
			}

		case "class", "interface", "enum", "record":
			// Extract class inheritance (extends) and interface implementation (implements)
			classDeps := jcge.extractClassRelations(entity)
			deps = append(deps, classDeps...)

			// Extract type references in field declarations
			typeDeps := jcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractMethodCalls finds method_invocation nodes within a method/constructor
func (jcge *JavaCallGraphExtractor) extractMethodCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get method body
	bodyNode := jcge.findMethodBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk method body looking for method_invocation nodes
	jcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "method_invocation" {
			callTarget, qualified := jcge.extractMethodInvocationTarget(node)
			if callTarget != "" && !seen[callTarget] {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: jcge.nodeLocation(node),
				}

				if qualified != "" {
					dep.ToQualified = qualified
				}

				// Check if call is conditional
				if jcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := jcge.resolveTarget(callTarget); target != nil {
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
func (jcge *JavaCallGraphExtractor) extractObjectCreation(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get method body
	bodyNode := jcge.findMethodBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track to deduplicate
	seen := make(map[string]bool)

	// Walk method body looking for object_creation_expression nodes
	jcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "object_creation_expression" {
			typeName := jcge.extractCreatedTypeName(node)
			if typeName != "" && !seen[typeName] && !jcge.isJavaBuiltinType(typeName) {
				seen[typeName] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  Calls, // Constructor call
					Location: jcge.nodeLocation(node),
				}

				// Check if call is conditional
				if jcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := jcge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractTypeReferences finds type_identifier nodes used in the entity
func (jcge *JavaCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	jcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers in type contexts
		if nodeType == "type_identifier" {
			// Make sure we're in a type context (not just an identifier)
			if jcge.isInTypeContext(node) {
				typeName := jcge.nodeText(node)
				if !seen[typeName] && !jcge.isJavaBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  UsesType,
						Location: jcge.nodeLocation(node),
					}

					// Try to resolve
					if target := jcge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
		}

		// Check for generic types like List<String>
		if nodeType == "generic_type" {
			// Extract the outer type (e.g., List from List<String>)
			outerType := jcge.extractGenericOuterType(node)
			if outerType != "" && !seen[outerType] && !jcge.isJavaBuiltinType(outerType) {
				seen[outerType] = true
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   outerType,
					DepType:  UsesType,
					Location: jcge.nodeLocation(node),
				}

				if target := jcge.resolveTarget(outerType); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for scoped types like pkg.ClassName
		if nodeType == "scoped_type_identifier" {
			typeName := jcge.nodeText(node)
			if !seen[typeName] && !jcge.isJavaBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:      entity.ID,
					ToName:      typeName,
					ToQualified: typeName,
					DepType:     UsesType,
					Location:    jcge.nodeLocation(node),
				}

				if target := jcge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		return true
	})

	return deps
}

// extractClassRelations extracts extends and implements relationships
func (jcge *JavaCallGraphExtractor) extractClassRelations(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Look for superclass (extends)
	jcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "superclass" {
			// Find the type_identifier inside superclass
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				typeName := jcge.extractTypeFromNode(child)
				if typeName != "" && !jcge.isJavaBuiltinType(typeName) {
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  Extends,
						Location: jcge.nodeLocation(node),
					}

					if target := jcge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
			return false // Don't recurse into superclass
		}
		return true
	})

	// Look for super_interfaces (implements for classes, extends for interfaces)
	jcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "super_interfaces" {
			// Find all type_identifier nodes inside
			jcge.walkNode(node, func(typeNode *sitter.Node) bool {
				if typeNode.Type() == "type_identifier" || typeNode.Type() == "generic_type" || typeNode.Type() == "scoped_type_identifier" {
					typeName := jcge.extractTypeFromNode(typeNode)
					if typeName != "" && !jcge.isJavaBuiltinType(typeName) {
						depType := Implements
						if entity.Type == "interface" {
							depType = Extends // Interface extends interface
						}

						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  depType,
							Location: jcge.nodeLocation(typeNode),
						}

						if target := jcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
				return true
			})
			return false // Don't recurse further after processing
		}
		return true
	})

	// Also check extends_interfaces for interface declarations
	jcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "extends_interfaces" {
			jcge.walkNode(node, func(typeNode *sitter.Node) bool {
				if typeNode.Type() == "type_identifier" || typeNode.Type() == "generic_type" || typeNode.Type() == "scoped_type_identifier" {
					typeName := jcge.extractTypeFromNode(typeNode)
					if typeName != "" && !jcge.isJavaBuiltinType(typeName) {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Extends,
							Location: jcge.nodeLocation(typeNode),
						}

						if target := jcge.resolveTarget(typeName); target != nil {
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
func (jcge *JavaCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find the containing class/interface by walking up the tree
	parent := entity.Node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "class_body", "interface_body", "enum_body":
			// Go one more level up to get the class/interface declaration
			parent = parent.Parent()
			if parent != nil {
				className := jcge.extractClassName(parent)
				if className != "" {
					dep := &Dependency{
						FromID:   entity.ID,
						ToName:   className,
						DepType:  MethodOf,
						Location: entity.Location,
					}

					// Try to resolve
					if target := jcge.resolveTarget(className); target != nil {
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

// extractMethodInvocationTarget extracts the method name and qualifier from a method_invocation
func (jcge *JavaCallGraphExtractor) extractMethodInvocationTarget(node *sitter.Node) (methodName string, qualified string) {
	if node == nil || node.Type() != "method_invocation" {
		return "", ""
	}

	// method_invocation structure:
	// - object (optional): the receiver
	// - name: identifier (the method name)
	// - arguments: argument_list

	var receiver string
	var name string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		// The name field contains the method name
		if childType == "identifier" {
			// Check if this is the method name (comes after object or at start)
			nameField := node.ChildByFieldName("name")
			if nameField != nil && nameField == child {
				name = jcge.nodeText(child)
			} else if name == "" {
				// Fallback: first identifier is the method name
				name = jcge.nodeText(child)
			}
		}

		// Object can be identifier, field_access, method_invocation, or this/super
		if childType == "identifier" && name == "" {
			// This could be receiver like obj.method()
			objectField := node.ChildByFieldName("object")
			if objectField != nil && objectField == child {
				receiver = jcge.nodeText(child)
			}
		}

		if childType == "field_access" {
			objectField := node.ChildByFieldName("object")
			if objectField != nil && objectField == child {
				receiver = jcge.nodeText(child)
			}
		}

		if childType == "this" || childType == "super" {
			receiver = childType
		}
	}

	// Try to get name via field if not found
	if name == "" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			name = jcge.nodeText(nameNode)
		}
	}

	// Get object for receiver if not found
	if receiver == "" {
		objectNode := node.ChildByFieldName("object")
		if objectNode != nil {
			receiver = jcge.nodeText(objectNode)
		}
	}

	if name == "" {
		return "", ""
	}

	if receiver != "" && receiver != "this" {
		qualified = receiver + "." + name
		return name, qualified
	}

	return name, ""
}

// extractCreatedTypeName extracts the type name from an object_creation_expression
func (jcge *JavaCallGraphExtractor) extractCreatedTypeName(node *sitter.Node) string {
	if node == nil || node.Type() != "object_creation_expression" {
		return ""
	}

	// object_creation_expression structure:
	// - "new" keyword
	// - type: type_identifier or generic_type
	// - arguments: argument_list

	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		return jcge.extractTypeFromNode(typeNode)
	}

	// Fallback: look for type_identifier or generic_type child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "type_identifier":
			return jcge.nodeText(child)
		case "generic_type":
			return jcge.extractGenericOuterType(child)
		case "scoped_type_identifier":
			return jcge.nodeText(child)
		}
	}

	return ""
}

// extractTypeFromNode extracts the type name from various type node types
func (jcge *JavaCallGraphExtractor) extractTypeFromNode(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "type_identifier":
		return jcge.nodeText(node)
	case "generic_type":
		return jcge.extractGenericOuterType(node)
	case "scoped_type_identifier":
		// Get the last identifier in the scoped type
		for i := int(node.ChildCount()) - 1; i >= 0; i-- {
			child := node.Child(i)
			if child.Type() == "type_identifier" {
				return jcge.nodeText(child)
			}
		}
		return jcge.nodeText(node)
	case "array_type":
		// Extract the element type
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "type_identifier" || child.Type() == "generic_type" {
				return jcge.extractTypeFromNode(child)
			}
		}
	}

	return ""
}

// extractGenericOuterType extracts the outer type from a generic_type (e.g., List from List<String>)
func (jcge *JavaCallGraphExtractor) extractGenericOuterType(node *sitter.Node) string {
	if node == nil || node.Type() != "generic_type" {
		return ""
	}

	// generic_type structure:
	// - type_identifier or scoped_type_identifier (the outer type)
	// - type_arguments (contains the generic params)

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" {
			return jcge.nodeText(child)
		}
		if child.Type() == "scoped_type_identifier" {
			return jcge.extractTypeFromNode(child)
		}
	}

	return ""
}

// extractClassName extracts the class name from a class/interface declaration
func (jcge *JavaCallGraphExtractor) extractClassName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			return jcge.nodeText(nameNode)
		}
		// Fallback: look for identifier child
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "identifier" {
				return jcge.nodeText(child)
			}
		}
	}

	return ""
}

// findMethodBody finds the block node in a method/constructor declaration
func (jcge *JavaCallGraphExtractor) findMethodBody(node *sitter.Node) *sitter.Node {
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
		if child.Type() == "block" || child.Type() == "constructor_body" {
			return child
		}
	}

	return nil
}

// isConditionalCall checks if a call is inside an if/switch/ternary statement
func (jcge *JavaCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_expression", "switch_block_statement_group",
			"ternary_expression", "switch_block":
			return true
		case "method_declaration", "constructor_declaration", "lambda_expression":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isInTypeContext checks if a type_identifier is in a type context
func (jcge *JavaCallGraphExtractor) isInTypeContext(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	switch parent.Type() {
	// Direct type contexts
	case "formal_parameter", "local_variable_declaration", "field_declaration",
		"method_declaration", "constructor_declaration",
		"cast_expression", "instanceof_expression",
		"class_declaration", "interface_declaration", "enum_declaration",
		"superclass", "super_interfaces", "extends_interfaces",
		"type_arguments", "type_parameter", "array_type",
		"throws", "annotation", "record_declaration":
		return true
	case "generic_type", "scoped_type_identifier":
		// These are type nodes themselves
		return true
	}

	return false
}

// isJavaBuiltinType checks if type is a Java builtin or common library type
func (jcge *JavaCallGraphExtractor) isJavaBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Primitive types
		"int": true, "long": true, "double": true, "float": true,
		"boolean": true, "byte": true, "short": true, "char": true,
		"void": true,

		// Primitive wrappers
		"Integer": true, "Long": true, "Double": true, "Float": true,
		"Boolean": true, "Byte": true, "Short": true, "Character": true,
		"Void": true, "Number": true,

		// Common classes
		"String": true, "Object": true, "Class": true,
		"Exception": true, "RuntimeException": true, "Throwable": true,
		"Error": true, "IllegalArgumentException": true, "IllegalStateException": true,
		"NullPointerException": true, "IndexOutOfBoundsException": true,

		// Collections
		"List": true, "Map": true, "Set": true, "Collection": true,
		"Iterator": true, "Iterable": true, "ArrayList": true,
		"HashMap": true, "HashSet": true, "LinkedList": true,
		"TreeMap": true, "TreeSet": true, "Queue": true, "Deque": true,
		"LinkedHashMap": true, "LinkedHashSet": true, "Vector": true,
		"Stack": true, "Properties": true, "Hashtable": true,
		"Collections": true, "Arrays": true,

		// Other common types
		"System": true, "Math": true, "StringBuilder": true, "StringBuffer": true,
		"Optional": true, "Stream": true, "Comparable": true, "Comparator": true,
		"Runnable": true, "Callable": true, "Future": true,
		"Thread": true, "Enum": true, "Annotation": true,
	}

	return builtins[name]
}

// resolveTarget attempts to resolve a target name to an entity
func (jcge *JavaCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := jcge.entityByName[name]; ok {
		return e
	}

	// Try without package prefix for qualified names
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (method/class name)
		if e, ok := jcge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

// walkNode performs a depth-first walk of the AST
func (jcge *JavaCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		jcge.walkNode(node.Child(int(i)), fn)
	}
}

// nodeText returns the source text for a node
func (jcge *JavaCallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || jcge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	endByte := node.EndByte()
	if endByte > uint32(len(jcge.result.Source)) {
		return ""
	}
	return node.Content(jcge.result.Source)
}

// nodeLocation returns file:line for a node
func (jcge *JavaCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if jcge.result.FilePath != "" {
		return jcge.result.FilePath + ":" + itoa(int(line))
	}
	return ":" + itoa(int(line))
}

// itoa is a simple int to string conversion to avoid importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

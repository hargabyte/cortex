// Package extract provides Rust call graph extraction from parsed AST.
package extract

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// RustCallGraphExtractor extracts dependencies from Rust AST
type RustCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewRustCallGraphExtractor creates a Rust call graph extractor
func NewRustCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *RustCallGraphExtractor {
	rcge := &RustCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: make(map[string]*CallGraphEntity),
		entityByID:   make(map[string]*CallGraphEntity),
	}

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		rcge.entityByName[e.Name] = e
		if e.QualifiedName != "" {
			rcge.entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			rcge.entityByID[e.ID] = e
		}
	}

	return rcge
}

// NewRustCallGraphExtractorWithMaps creates an extractor with pre-built lookup maps
func NewRustCallGraphExtractorWithMaps(result *parser.ParseResult, entities []CallGraphEntity,
	entityByName map[string]*CallGraphEntity, entityByID map[string]*CallGraphEntity) *RustCallGraphExtractor {
	return &RustCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// ExtractDependencies extracts all dependencies from the parsed Rust code
func (rcge *RustCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range rcge.entities {
		entity := &rcge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "function", "method":
			// Extract function calls (direct and method calls)
			callDeps := rcge.extractFunctionCalls(entity)
			deps = append(deps, callDeps...)

			// Extract method calls
			methodCallDeps := rcge.extractMethodCalls(entity)
			deps = append(deps, methodCallDeps...)

			// Extract type references
			typeDeps := rcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract trait bounds
			traitBoundDeps := rcge.extractTraitBounds(entity)
			deps = append(deps, traitBoundDeps...)

			// Extract method owner for methods
			if entity.Type == "method" {
				if ownerDep := rcge.extractMethodOwner(entity); ownerDep != nil {
					deps = append(deps, *ownerDep)
				}
			}

		case "struct", "type", "enum":
			// Extract trait implementations
			implDeps := rcge.extractTraitImpl(entity)
			deps = append(deps, implDeps...)

			// Extract type references in struct fields
			typeDeps := rcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

		case "trait", "interface":
			// Extract super trait bounds
			traitBoundDeps := rcge.extractTraitBounds(entity)
			deps = append(deps, traitBoundDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call_expression nodes within a function
func (rcge *RustCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := rcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for call_expression nodes
	rcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "call_expression" {
			// Get function being called
			callTarget := rcge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] {
				// Skip macro invocations (end with !)
				if strings.HasSuffix(callTarget, "!") {
					// Handle macros as optional - lower priority
					macroName := strings.TrimSuffix(callTarget, "!")
					if rcge.isBuiltinMacro(macroName) {
						return true
					}
					callTarget = macroName
				}

				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: rcge.nodeLocation(node),
				}

				// Parse qualified names (Rust uses ::)
				if strings.Contains(callTarget, "::") {
					dep.ToQualified = callTarget
					parts := strings.Split(callTarget, "::")
					dep.ToName = parts[len(parts)-1]
				}

				// Check if call is conditional
				if rcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := rcge.resolveTarget(callTarget); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractMethodCalls finds method_call_expression nodes within a function
func (rcge *RustCallGraphExtractor) extractMethodCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := rcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track method calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for method_call_expression nodes
	rcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "method_call_expression" {
			// Extract method name from the call
			methodName := rcge.extractMethodCallName(node)
			if methodName != "" && !seen[methodName] {
				seen[methodName] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   methodName,
					DepType:  Calls,
					Location: rcge.nodeLocation(node),
				}

				// Check if call is conditional
				if rcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := rcge.resolveTarget(methodName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractTypeReferences finds type identifiers used in entity
func (rcge *RustCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	rcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers
		if nodeType == "type_identifier" {
			typeName := rcge.nodeText(node)
			if !seen[typeName] && !rcge.isBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  UsesType,
					Location: rcge.nodeLocation(node),
				}

				// Try to resolve
				if target := rcge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for scoped type paths (e.g., std::collections::HashMap)
		if nodeType == "scoped_type_identifier" {
			typeName := rcge.extractScopedTypeName(node)
			if !seen[typeName] && !rcge.isBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:      entity.ID,
					ToName:      extractLastComponent(typeName),
					ToQualified: typeName,
					DepType:     UsesType,
					Location:    rcge.nodeLocation(node),
				}

				if target := rcge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for generic types
		if nodeType == "generic_type" {
			// Get the base type
			for i := uint32(0); i < node.NamedChildCount(); i++ {
				child := node.NamedChild(int(i))
				if child.Type() == "type_identifier" || child.Type() == "scoped_type_identifier" {
					typeName := rcge.nodeText(child)
					if !seen[typeName] && !rcge.isBuiltinType(typeName) {
						seen[typeName] = true
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   extractLastComponent(typeName),
							DepType:  UsesType,
							Location: rcge.nodeLocation(child),
						}
						if strings.Contains(typeName, "::") {
							dep.ToQualified = typeName
						}
						if target := rcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}
						deps = append(deps, dep)
					}
					break
				}
			}
		}

		return true
	})

	return deps
}

// extractTraitBounds extracts trait bound dependencies (T: Trait)
func (rcge *RustCallGraphExtractor) extractTraitBounds(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for trait bounds
	rcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Look for trait bounds in type parameters
		if nodeType == "trait_bounds" || nodeType == "type_bound_list" {
			// Extract each trait in the bound
			rcge.walkNode(node, func(boundNode *sitter.Node) bool {
				if boundNode.Type() == "type_identifier" || boundNode.Type() == "scoped_type_identifier" {
					traitName := rcge.nodeText(boundNode)
					if !seen[traitName] && !rcge.isBuiltinType(traitName) {
						seen[traitName] = true
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   extractLastComponent(traitName),
							DepType:  UsesType,
							Location: rcge.nodeLocation(boundNode),
						}
						if strings.Contains(traitName, "::") {
							dep.ToQualified = traitName
						}
						if target := rcge.resolveTarget(traitName); target != nil {
							dep.ToID = target.ID
						}
						deps = append(deps, dep)
					}
				}
				return true
			})
		}

		// Look for where clauses
		if nodeType == "where_clause" {
			rcge.walkNode(node, func(whereNode *sitter.Node) bool {
				if whereNode.Type() == "type_identifier" || whereNode.Type() == "scoped_type_identifier" {
					traitName := rcge.nodeText(whereNode)
					if !seen[traitName] && !rcge.isBuiltinType(traitName) {
						seen[traitName] = true
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   extractLastComponent(traitName),
							DepType:  UsesType,
							Location: rcge.nodeLocation(whereNode),
						}
						if strings.Contains(traitName, "::") {
							dep.ToQualified = traitName
						}
						if target := rcge.resolveTarget(traitName); target != nil {
							dep.ToID = target.ID
						}
						deps = append(deps, dep)
					}
				}
				return true
			})
		}

		return true
	})

	return deps
}

// extractTraitImpl extracts impl Trait for Type dependencies
func (rcge *RustCallGraphExtractor) extractTraitImpl(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// For structs/enums, we need to find associated impl blocks
	// This is complex because impl blocks are separate from the type definition
	// The impl block relationship is typically tracked during scanning

	// However, we can check if this entity IS an impl block
	if entity.Node != nil && entity.Node.Type() == "impl_item" {
		// Check for trait implementation
		traitNode := entity.Node.ChildByFieldName("trait")
		if traitNode != nil {
			traitName := rcge.nodeText(traitNode)
			if traitName != "" && !rcge.isBuiltinType(traitName) {
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   extractLastComponent(traitName),
					DepType:  Implements,
					Location: rcge.nodeLocation(traitNode),
				}
				if strings.Contains(traitName, "::") {
					dep.ToQualified = traitName
				}
				if target := rcge.resolveTarget(traitName); target != nil {
					dep.ToID = target.ID
				}
				deps = append(deps, dep)
			}
		}
	}

	return deps
}

// extractMethodOwner extracts method_of dependency for methods in impl blocks
func (rcge *RustCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find parent impl block
	parent := entity.Node.Parent()
	for parent != nil {
		if parent.Type() == "impl_item" {
			// Get the type being implemented
			typeNode := parent.ChildByFieldName("type")
			if typeNode != nil {
				typeName := rcge.nodeText(typeNode)
				// Strip generic parameters if present
				if idx := strings.Index(typeName, "<"); idx > 0 {
					typeName = typeName[:idx]
				}
				if typeName != "" {
					dep := &Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  MethodOf,
						Location: entity.Location,
					}

					// Try to resolve
					if target := rcge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					return dep
				}
			}
			break
		}
		// Also check declaration_list which is the body of impl blocks
		if parent.Type() == "declaration_list" {
			grandParent := parent.Parent()
			if grandParent != nil && grandParent.Type() == "impl_item" {
				parent = grandParent
				continue
			}
		}
		parent = parent.Parent()
	}

	return nil
}

// isBuiltinType checks if type is a Rust builtin
func (rcge *RustCallGraphExtractor) isBuiltinType(name string) bool {
	// Extract just the type name without path
	simpleName := extractLastComponent(name)

	builtins := map[string]bool{
		// Primitive types
		"String": true, "str": true,
		"i8": true, "i16": true, "i32": true, "i64": true, "i128": true, "isize": true,
		"u8": true, "u16": true, "u32": true, "u64": true, "u128": true, "usize": true,
		"f32": true, "f64": true, "bool": true, "char": true,
		// Common collections and smart pointers
		"Vec": true, "HashMap": true, "HashSet": true,
		"BTreeMap": true, "BTreeSet": true, "VecDeque": true,
		"Option": true, "Result": true, "Box": true,
		"Rc": true, "Arc": true, "RefCell": true, "Cell": true,
		"Mutex": true, "RwLock": true,
		// Other common types
		"Self": true, "self": true,
		"Sized": true, "Copy": true, "Clone": true, "Debug": true, "Display": true,
		"Default": true, "Eq": true, "PartialEq": true, "Ord": true, "PartialOrd": true,
		"Hash": true, "Send": true, "Sync": true, "Unpin": true,
		"Drop": true, "Fn": true, "FnMut": true, "FnOnce": true,
		"Iterator": true, "IntoIterator": true, "FromIterator": true,
		"From": true, "Into": true, "TryFrom": true, "TryInto": true,
		"AsRef": true, "AsMut": true, "Borrow": true, "BorrowMut": true,
		"ToOwned": true, "ToString": true,
		"Deref": true, "DerefMut": true,
		"Read": true, "Write": true, "Seek": true, "BufRead": true,
		"Future": true, "Stream": true,
		"Error": true,
	}
	return builtins[simpleName]
}

// isBuiltinMacro checks if macro is a Rust builtin macro
func (rcge *RustCallGraphExtractor) isBuiltinMacro(name string) bool {
	builtinMacros := map[string]bool{
		"println": true, "eprintln": true, "print": true, "eprint": true,
		"format": true, "panic": true, "assert": true, "assert_eq": true, "assert_ne": true,
		"dbg": true, "todo": true, "unimplemented": true, "unreachable": true,
		"vec": true, "cfg": true, "env": true, "include": true, "include_str": true,
		"include_bytes": true, "concat": true, "stringify": true, "file": true,
		"line": true, "column": true, "module_path": true,
		"write": true, "writeln": true, "format_args": true,
		"matches": true, "debug_assert": true, "debug_assert_eq": true, "debug_assert_ne": true,
	}
	return builtinMacros[name]
}

// isConditionalCall checks if a call is inside an if/match statement
func (rcge *RustCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_expression", "match_expression", "if_let_expression":
			return true
		case "function_item", "closure_expression":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// Helper methods

// walkNode performs a depth-first walk of the AST
func (rcge *RustCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		rcge.walkNode(node.Child(int(i)), fn)
	}
}

// nodeText returns the source text for a node
func (rcge *RustCallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || rcge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	endByte := node.EndByte()
	if endByte > uint32(len(rcge.result.Source)) {
		return ""
	}
	return node.Content(rcge.result.Source)
}

// nodeLocation returns file:line for a node
func (rcge *RustCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if rcge.result.FilePath != "" {
		return fmt.Sprintf("%s:%d", rcge.result.FilePath, line)
	}
	return fmt.Sprintf(":%d", line)
}

// extractCallTarget extracts the function name from a call_expression
func (rcge *RustCallGraphExtractor) extractCallTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "call_expression" {
		return ""
	}

	// The function being called is the first child
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil && node.ChildCount() > 0 {
		funcNode = node.Child(0)
	}

	if funcNode == nil {
		return ""
	}

	switch funcNode.Type() {
	case "identifier":
		return rcge.nodeText(funcNode)

	case "scoped_identifier":
		// e.g., Type::associated_fn or module::function
		return rcge.nodeText(funcNode)

	case "field_expression":
		// e.g., obj.method() - extract the method name (field part)
		// In tree-sitter-rust, method calls on objects are parsed as:
		// call_expression -> field_expression -> (value, ".", field)
		fieldNode := funcNode.ChildByFieldName("field")
		if fieldNode != nil {
			return rcge.nodeText(fieldNode)
		}
		// Fallback: get the last identifier
		for i := int(funcNode.ChildCount()) - 1; i >= 0; i-- {
			child := funcNode.Child(i)
			if child.Type() == "identifier" || child.Type() == "field_identifier" {
				return rcge.nodeText(child)
			}
		}
		return ""

	case "generic_function":
		// e.g., function::<T>()
		// Get the function identifier
		for i := uint32(0); i < funcNode.ChildCount(); i++ {
			child := funcNode.Child(int(i))
			if child.Type() == "identifier" || child.Type() == "scoped_identifier" {
				return rcge.nodeText(child)
			}
		}
		return ""
	}

	return ""
}

// extractMethodCallName extracts the method name from a method_call_expression
func (rcge *RustCallGraphExtractor) extractMethodCallName(node *sitter.Node) string {
	if node == nil || node.Type() != "method_call_expression" {
		return ""
	}

	// Method name is in the 'name' field
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return rcge.nodeText(nameNode)
	}

	// Fallback: search for identifier after the dot
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			// Skip if this is the receiver
			prevSibling := node.Child(int(i) - 1)
			if prevSibling != nil && rcge.nodeText(prevSibling) == "." {
				return rcge.nodeText(child)
			}
		}
	}

	return ""
}

// extractScopedTypeName extracts the full scoped type name
func (rcge *RustCallGraphExtractor) extractScopedTypeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return rcge.nodeText(node)
}

// findFunctionBody finds the block node in a function item
func (rcge *RustCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
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
	}

	return nil
}

// resolveTarget attempts to resolve a target name to an entity
func (rcge *RustCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := rcge.entityByName[name]; ok {
		return e
	}

	// Try without path prefix for scoped identifiers
	if strings.Contains(name, "::") {
		parts := strings.Split(name, "::")
		// Try just the last part (method/function name)
		if e, ok := rcge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

// extractLastComponent extracts the last component of a path (e.g., "std::vec::Vec" -> "Vec")
func extractLastComponent(path string) string {
	if idx := strings.LastIndex(path, "::"); idx >= 0 {
		return path[idx+2:]
	}
	return path
}

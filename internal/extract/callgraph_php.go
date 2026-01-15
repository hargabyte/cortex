// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements PHP-specific call graph extraction.
package extract

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// PHPCallGraphExtractor extracts dependencies from PHP AST
type PHPCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewPHPCallGraphExtractor creates a PHP call graph extractor
func NewPHPCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *PHPCallGraphExtractor {
	pcge := &PHPCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: make(map[string]*CallGraphEntity),
		entityByID:   make(map[string]*CallGraphEntity),
	}

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		pcge.entityByName[e.Name] = e
		if e.QualifiedName != "" {
			pcge.entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			pcge.entityByID[e.ID] = e
		}
	}

	return pcge
}

// ExtractDependencies extracts all dependencies from the parsed PHP code
func (pcge *PHPCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range pcge.entities {
		entity := &pcge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "function", "method":
			// Extract function calls
			callDeps := pcge.extractFunctionCalls(entity)
			deps = append(deps, callDeps...)

			// Extract method calls
			methodDeps := pcge.extractMethodCalls(entity)
			deps = append(deps, methodDeps...)

			// Extract static calls
			staticDeps := pcge.extractStaticCalls(entity)
			deps = append(deps, staticDeps...)

			// Extract object creation (new expressions)
			newDeps := pcge.extractObjectCreation(entity)
			deps = append(deps, newDeps...)

			// Extract type references (parameters, return types, local variables)
			typeDeps := pcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract method owner relationship
			if ownerDep := pcge.extractMethodOwner(entity); ownerDep != nil {
				deps = append(deps, *ownerDep)
			}

		case "class", "interface", "trait", "enum":
			// Extract class inheritance (extends) and interface implementation (implements)
			classDeps := pcge.extractClassRelations(entity)
			deps = append(deps, classDeps...)

			// Extract trait usage
			traitDeps := pcge.extractTraitUsage(entity)
			deps = append(deps, traitDeps...)

			// Extract type references in property declarations
			typeDeps := pcge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds function_call_expression nodes within a method/function
func (pcge *PHPCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := pcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk body looking for function_call_expression nodes
	pcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "function_call_expression" {
			callTarget := pcge.extractFunctionCallTarget(node)
			if callTarget != "" && !seen[callTarget] && !pcge.isPHPBuiltin(callTarget) {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: pcge.nodeLocation(node),
				}

				// Check if call is conditional
				if pcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := pcge.resolveTarget(callTarget); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractMethodCalls finds member_call_expression nodes ($obj->method())
func (pcge *PHPCallGraphExtractor) extractMethodCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := pcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk body looking for member_call_expression nodes
	pcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "member_call_expression" {
			methodName, qualified := pcge.extractMemberCallTarget(node)
			if methodName != "" && !seen[methodName] {
				seen[methodName] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   methodName,
					DepType:  Calls,
					Location: pcge.nodeLocation(node),
				}

				if qualified != "" {
					dep.ToQualified = qualified
				}

				// Check if call is conditional
				if pcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := pcge.resolveTarget(methodName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractStaticCalls finds scoped_call_expression nodes (ClassName::staticMethod())
func (pcge *PHPCallGraphExtractor) extractStaticCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := pcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk body looking for scoped_call_expression nodes
	pcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "scoped_call_expression" {
			methodName, className, qualified := pcge.extractStaticCallTarget(node)
			if methodName != "" && !seen[qualified] {
				seen[qualified] = true

				dep := Dependency{
					FromID:      entity.ID,
					ToName:      methodName,
					ToQualified: qualified,
					DepType:     Calls,
					Location:    pcge.nodeLocation(node),
				}

				// Check if call is conditional
				if pcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve - check class first, then method
				if target := pcge.resolveTarget(methodName); target != nil {
					dep.ToID = target.ID
				} else if target := pcge.resolveTarget(className); target != nil {
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
func (pcge *PHPCallGraphExtractor) extractObjectCreation(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := pcge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track to deduplicate
	seen := make(map[string]bool)

	// Walk body looking for object_creation_expression nodes
	pcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "object_creation_expression" {
			typeName := pcge.extractCreatedTypeName(node)
			if typeName != "" && !seen[typeName] && !pcge.isPHPBuiltinType(typeName) {
				seen[typeName] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  Calls, // Constructor call
					Location: pcge.nodeLocation(node),
				}

				// Check if call is conditional
				if pcge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := pcge.resolveTarget(typeName); target != nil {
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
func (pcge *PHPCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	pcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers in type contexts
		if nodeType == "named_type" || nodeType == "name" || nodeType == "qualified_name" {
			// Make sure we're in a type context
			if pcge.isInTypeContext(node) {
				typeName := pcge.nodeText(node)
				if !seen[typeName] && !pcge.isPHPBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  UsesType,
						Location: pcge.nodeLocation(node),
					}

					// Try to resolve
					if target := pcge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
		}

		// Check for union types (Type|Type)
		if nodeType == "union_type" {
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				if child.Type() == "named_type" || child.Type() == "name" || child.Type() == "qualified_name" {
					typeName := pcge.nodeText(child)
					if !seen[typeName] && !pcge.isPHPBuiltinType(typeName) {
						seen[typeName] = true
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  UsesType,
							Location: pcge.nodeLocation(child),
						}

						if target := pcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
			}
		}

		return true
	})

	return deps
}

// extractClassRelations extracts extends and implements relationships
func (pcge *PHPCallGraphExtractor) extractClassRelations(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Look for base_clause (extends)
	pcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "base_clause" {
			// Find the type inside base_clause
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				if child.Type() == "name" || child.Type() == "qualified_name" {
					typeName := pcge.nodeText(child)
					if typeName != "" && !pcge.isPHPBuiltinType(typeName) {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Extends,
							Location: pcge.nodeLocation(node),
						}

						if target := pcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
			}
			return false // Don't recurse into base_clause
		}
		return true
	})

	// Look for class_interface_clause (implements)
	pcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "class_interface_clause" {
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				if child.Type() == "name" || child.Type() == "qualified_name" {
					typeName := pcge.nodeText(child)
					if typeName != "" && !pcge.isPHPBuiltinType(typeName) {
						depType := Implements
						if entity.Type == "interface" {
							depType = Extends // Interface extends interface
						}

						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  depType,
							Location: pcge.nodeLocation(child),
						}

						if target := pcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
			}
			return false // Don't recurse
		}
		return true
	})

	return deps
}

// extractTraitUsage extracts trait usage (use TraitName)
func (pcge *PHPCallGraphExtractor) extractTraitUsage(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Find declaration_list (class body)
	var bodyNode *sitter.Node
	pcge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "declaration_list" {
			bodyNode = node
			return false
		}
		return true
	})

	if bodyNode == nil {
		return deps
	}

	// Look for use_declaration nodes
	pcge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "use_declaration" {
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				if child.Type() == "name" || child.Type() == "qualified_name" {
					typeName := pcge.nodeText(child)
					if typeName != "" {
						dep := Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							DepType:  Extends, // Traits are similar to extends (mixin pattern)
							Location: pcge.nodeLocation(child),
						}

						if target := pcge.resolveTarget(typeName); target != nil {
							dep.ToID = target.ID
						}

						deps = append(deps, dep)
					}
				}
			}
			return false
		}
		return true
	})

	return deps
}

// extractMethodOwner extracts method_of dependency for methods
func (pcge *PHPCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find the containing class/interface/trait by walking up the tree
	parent := entity.Node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "declaration_list":
			// Go one more level up to get the class/interface/trait declaration
			parent = parent.Parent()
			if parent != nil {
				className := pcge.extractClassName(parent)
				if className != "" {
					dep := &Dependency{
						FromID:   entity.ID,
						ToName:   className,
						DepType:  MethodOf,
						Location: entity.Location,
					}

					// Try to resolve
					if target := pcge.resolveTarget(className); target != nil {
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

// extractFunctionCallTarget extracts the function name from a function_call_expression
func (pcge *PHPCallGraphExtractor) extractFunctionCallTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "function_call_expression" {
		return ""
	}

	// The function being called is usually the first child (name or qualified_name)
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil {
		// Try to find name child
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "name" || child.Type() == "qualified_name" {
				funcNode = child
				break
			}
		}
	}

	if funcNode == nil {
		return ""
	}

	return pcge.nodeText(funcNode)
}

// extractMemberCallTarget extracts the method name and qualifier from a member_call_expression
func (pcge *PHPCallGraphExtractor) extractMemberCallTarget(node *sitter.Node) (methodName string, qualified string) {
	if node == nil || node.Type() != "member_call_expression" {
		return "", ""
	}

	// member_call_expression structure:
	// - object: the receiver ($this, $obj, etc.)
	// - name: the method name
	// - arguments: argument_list

	var receiver string
	var name string

	// Get object (receiver)
	objectNode := node.ChildByFieldName("object")
	if objectNode != nil {
		receiver = pcge.nodeText(objectNode)
	}

	// Get name (method name)
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		name = pcge.nodeText(nameNode)
	}

	if name == "" {
		return "", ""
	}

	if receiver != "" && receiver != "$this" && receiver != "self" && receiver != "parent" {
		qualified = receiver + "->" + name
		return name, qualified
	}

	return name, ""
}

// extractStaticCallTarget extracts the method name and class from a scoped_call_expression
func (pcge *PHPCallGraphExtractor) extractStaticCallTarget(node *sitter.Node) (methodName string, className string, qualified string) {
	if node == nil || node.Type() != "scoped_call_expression" {
		return "", "", ""
	}

	// scoped_call_expression structure:
	// - scope: the class name (name, qualified_name, or self/parent/static)
	// - name: the method name
	// - arguments: argument_list

	// Get scope (class name)
	scopeNode := node.ChildByFieldName("scope")
	if scopeNode != nil {
		className = pcge.nodeText(scopeNode)
	}

	// Get name (method name)
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		methodName = pcge.nodeText(nameNode)
	}

	if methodName == "" {
		return "", "", ""
	}

	if className != "" {
		qualified = className + "::" + methodName
	} else {
		qualified = methodName
	}

	return methodName, className, qualified
}

// extractCreatedTypeName extracts the type name from an object_creation_expression
func (pcge *PHPCallGraphExtractor) extractCreatedTypeName(node *sitter.Node) string {
	if node == nil || node.Type() != "object_creation_expression" {
		return ""
	}

	// object_creation_expression structure:
	// - "new" keyword
	// - type: name or qualified_name
	// - arguments: optional argument_list

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "name" || child.Type() == "qualified_name" {
			return pcge.nodeText(child)
		}
	}

	return ""
}

// extractClassName extracts the class name from a class/interface/trait declaration
func (pcge *PHPCallGraphExtractor) extractClassName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "class_declaration", "interface_declaration", "trait_declaration", "enum_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			return pcge.nodeText(nameNode)
		}
		// Fallback: look for name child
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "name" {
				return pcge.nodeText(child)
			}
		}
	}

	return ""
}

// findFunctionBody finds the compound_statement node in a function/method declaration
func (pcge *PHPCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// Try field name first
	if body := node.ChildByFieldName("body"); body != nil {
		return body
	}

	// Search for compound_statement child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "compound_statement" {
			return child
		}
	}

	return nil
}

// isConditionalCall checks if a call is inside an if/switch/ternary statement
func (pcge *PHPCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_statement", "conditional_expression",
			"match_expression", "case_statement":
			return true
		case "function_definition", "method_declaration", "anonymous_function_creation_expression",
			"arrow_function":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isInTypeContext checks if a name is in a type context
func (pcge *PHPCallGraphExtractor) isInTypeContext(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	switch parent.Type() {
	// Direct type contexts
	case "simple_parameter", "property_declaration", "formal_parameters",
		"return_type", "type", "union_type", "intersection_type", "optional_type",
		"class_declaration", "interface_declaration", "trait_declaration",
		"base_clause", "class_interface_clause", "property_promotion_parameter",
		"catch_clause", "instanceof_expression", "cast_expression":
		return true
	case "named_type":
		// named_type is a type node itself
		return true
	}

	return false
}

// isPHPBuiltin checks if name is a PHP builtin function
func (pcge *PHPCallGraphExtractor) isPHPBuiltin(name string) bool {
	builtins := map[string]bool{
		// Output
		"echo": true, "print": true, "printf": true, "print_r": true, "var_dump": true, "var_export": true,

		// Variable handling
		"isset": true, "empty": true, "unset": true, "gettype": true, "settype": true, "is_null": true,
		"is_bool": true, "is_int": true, "is_float": true, "is_string": true, "is_array": true,
		"is_object": true, "is_callable": true, "is_numeric": true,

		// String functions
		"strlen": true, "substr": true, "strpos": true, "str_replace": true, "trim": true,
		"strtolower": true, "strtoupper": true, "explode": true, "implode": true, "sprintf": true,
		"preg_match": true, "preg_replace": true, "str_contains": true, "str_starts_with": true,
		"str_ends_with": true,

		// Array functions
		"count": true, "sizeof": true, "array_push": true, "array_pop": true, "array_shift": true,
		"array_unshift": true, "array_merge": true, "array_keys": true, "array_values": true,
		"array_map": true, "array_filter": true, "array_reduce": true, "in_array": true,
		"array_search": true, "array_key_exists": true, "array_slice": true, "array_splice": true,
		"usort": true, "sort": true, "asort": true, "ksort": true,

		// JSON
		"json_encode": true, "json_decode": true,

		// File functions
		"file_get_contents": true, "file_put_contents": true, "file_exists": true, "is_file": true,
		"is_dir": true, "mkdir": true, "rmdir": true, "unlink": true, "rename": true, "copy": true,
		"fopen": true, "fclose": true, "fread": true, "fwrite": true, "fgets": true,

		// Date/Time
		"date": true, "time": true, "strtotime": true, "mktime": true,

		// Math
		"abs": true, "ceil": true, "floor": true, "round": true, "max": true, "min": true,
		"rand": true, "mt_rand": true,

		// Type casting (technically not functions but used similarly)
		"intval": true, "floatval": true, "strval": true, "boolval": true,

		// Class/Object
		"get_class": true, "get_parent_class": true, "class_exists": true, "interface_exists": true,
		"method_exists": true, "property_exists": true, "is_a": true, "get_object_vars": true,

		// Error handling
		"trigger_error": true, "error_reporting": true, "set_error_handler": true,

		// Misc
		"defined": true, "define": true, "constant": true, "header": true, "exit": true, "die": true,
		"sleep": true, "usleep": true, "compact": true, "extract": true, "list": true,
		"call_user_func": true, "call_user_func_array": true, "func_get_args": true,
	}

	return builtins[name]
}

// isPHPBuiltinType checks if type is a PHP builtin or common type
func (pcge *PHPCallGraphExtractor) isPHPBuiltinType(name string) bool {
	// Normalize: remove leading \ for fully qualified names
	name = strings.TrimPrefix(name, "\\")

	builtins := map[string]bool{
		// Scalar types
		"int": true, "integer": true, "float": true, "double": true, "string": true,
		"bool": true, "boolean": true, "null": true,

		// Compound types
		"array": true, "object": true, "callable": true, "iterable": true,

		// Special types
		"void": true, "never": true, "mixed": true, "self": true, "parent": true, "static": true,
		"false": true, "true": true,

		// Common classes
		"Exception": true, "Error": true, "Throwable": true, "RuntimeException": true,
		"InvalidArgumentException": true, "LogicException": true, "OutOfBoundsException": true,

		// SPL
		"stdClass": true, "ArrayObject": true, "ArrayIterator": true, "Iterator": true,
		"IteratorAggregate": true, "Countable": true, "Traversable": true, "Serializable": true,
		"ArrayAccess": true, "JsonSerializable": true, "Stringable": true,

		// DateTime
		"DateTime": true, "DateTimeInterface": true, "DateTimeImmutable": true, "DateInterval": true,
		"DatePeriod": true, "DateTimeZone": true,

		// Closures
		"Closure": true,
	}

	return builtins[name]
}

// resolveTarget attempts to resolve a target name to an entity
func (pcge *PHPCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := pcge.entityByName[name]; ok {
		return e
	}

	// Try without namespace prefix for qualified names
	if strings.Contains(name, "\\") {
		parts := strings.Split(name, "\\")
		// Try just the last part (class/function name)
		if e, ok := pcge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

// walkNode performs a depth-first walk of the AST
func (pcge *PHPCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		pcge.walkNode(node.Child(int(i)), fn)
	}
}

// nodeText returns the source text for a node
func (pcge *PHPCallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || pcge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	endByte := node.EndByte()
	if endByte > uint32(len(pcge.result.Source)) {
		return ""
	}
	return node.Content(pcge.result.Source)
}

// nodeLocation returns file:line for a node
func (pcge *PHPCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if pcge.result.FilePath != "" {
		return pcge.result.FilePath + ":" + phpItoa(int(line))
	}
	return ":" + phpItoa(int(line))
}

// phpItoa is a simple int to string conversion to avoid importing strconv
func phpItoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + phpItoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

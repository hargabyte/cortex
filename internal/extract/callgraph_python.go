// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements Python-specific call graph extraction.
package extract

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// PythonCallGraphExtractor extracts dependencies from Python AST
type PythonCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewPythonCallGraphExtractor creates a call graph extractor for Python
func NewPythonCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *PythonCallGraphExtractor {
	cge := &PythonCallGraphExtractor{
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

// NewPythonCallGraphExtractorWithMaps creates an extractor with pre-built lookup maps
func NewPythonCallGraphExtractorWithMaps(result *parser.ParseResult, entities []CallGraphEntity,
	entityByName map[string]*CallGraphEntity, entityByID map[string]*CallGraphEntity) *PythonCallGraphExtractor {
	return &PythonCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// ExtractDependencies extracts all dependencies from the parsed Python code
func (cge *PythonCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
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

			// Extract type references (type hints)
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

			// Extract decorator calls
			decoratorDeps := cge.extractDecorators(entity)
			deps = append(deps, decoratorDeps...)

			// Extract method owner (for methods)
			if entity.Type == "method" {
				if ownerDep := cge.extractMethodOwner(entity); ownerDep != nil {
					deps = append(deps, *ownerDep)
				}
			}

		case "class":
			// Extract base classes (extends relationship)
			baseDeps := cge.extractBaseClasses(entity)
			deps = append(deps, baseDeps...)

			// Extract decorator calls on class
			decoratorDeps := cge.extractDecorators(entity)
			deps = append(deps, decoratorDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call nodes within a function body
func (cge *PythonCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := cge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for call nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "call" {
			// Get function being called
			callTarget := cge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] && !isPythonBuiltinType(callTarget) {
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
		}
		return true
	})

	return deps
}

// extractTypeReferences finds type annotations in function signatures and body
func (cge *PythonCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type annotations (Python type hints)
		if nodeType == "type" {
			typeName := cge.nodeText(node)
			// Clean up the type name (remove Optional[], List[], etc.)
			typeName = cge.extractBaseTypeName(typeName)
			if !seen[typeName] && !isPythonBuiltinType(typeName) && typeName != "" {
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

		// Check for identifier in annotation context
		if nodeType == "identifier" && cge.isInTypeAnnotationContext(node) {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !isPythonBuiltinType(typeName) {
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

		return true
	})

	return deps
}

// extractBaseClasses extracts inheritance relationships from class definitions
func (cge *PythonCallGraphExtractor) extractBaseClasses(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Find the argument_list child (contains base classes)
	var argList *sitter.Node
	for i := uint32(0); i < entity.Node.ChildCount(); i++ {
		child := entity.Node.Child(int(i))
		if child.Type() == "argument_list" {
			argList = child
			break
		}
	}

	if argList == nil {
		return deps
	}

	// Extract base class names from argument_list
	for i := uint32(0); i < argList.NamedChildCount(); i++ {
		child := argList.NamedChild(int(i))

		var baseName string
		switch child.Type() {
		case "identifier":
			baseName = cge.nodeText(child)
		case "attribute":
			baseName = cge.nodeText(child)
		case "call":
			// e.g., Generic[T] - extract the function name
			baseName = cge.extractCallTarget(child)
		}

		if baseName != "" && !isPythonBuiltinType(baseName) {
			dep := Dependency{
				FromID:   entity.ID,
				ToName:   baseName,
				DepType:  Extends,
				Location: cge.nodeLocation(child),
			}

			if strings.Contains(baseName, ".") {
				dep.ToQualified = baseName
				parts := strings.Split(baseName, ".")
				dep.ToName = parts[len(parts)-1]
			}

			if target := cge.resolveTarget(baseName); target != nil {
				dep.ToID = target.ID
			}

			deps = append(deps, dep)
		}
	}

	return deps
}

// extractDecorators extracts decorator calls from decorated functions/classes
func (cge *PythonCallGraphExtractor) extractDecorators(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Check if this entity has decorators
	// Decorators can be siblings in a decorated_definition or direct children
	nodeToCheck := entity.Node

	// If the parent is a decorated_definition, check siblings
	if parent := entity.Node.Parent(); parent != nil && parent.Type() == "decorated_definition" {
		nodeToCheck = parent
	}

	// Find decorator nodes
	for i := uint32(0); i < nodeToCheck.ChildCount(); i++ {
		child := nodeToCheck.Child(int(i))
		if child.Type() == "decorator" {
			decoratorName := cge.extractDecoratorName(child)
			if decoratorName != "" && !isPythonBuiltinType(decoratorName) {
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   decoratorName,
					DepType:  Calls,
					Location: cge.nodeLocation(child),
				}

				if strings.Contains(decoratorName, ".") {
					dep.ToQualified = decoratorName
					parts := strings.Split(decoratorName, ".")
					dep.ToName = parts[len(parts)-1]
				}

				if target := cge.resolveTarget(decoratorName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
	}

	return deps
}

// extractMethodOwner extracts the method_of relationship for class methods
func (cge *PythonCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Walk up to find the enclosing class
	parent := entity.Node.Parent()
	for parent != nil {
		// Skip decorated_definition wrapper
		if parent.Type() == "decorated_definition" {
			parent = parent.Parent()
			continue
		}

		// Check if we're inside a class body (block)
		if parent.Type() == "block" {
			grandparent := parent.Parent()
			if grandparent != nil && grandparent.Type() == "class_definition" {
				// Extract class name
				className := cge.extractClassName(grandparent)
				if className != "" {
					dep := &Dependency{
						FromID:   entity.ID,
						ToName:   className,
						DepType:  MethodOf,
						Location: entity.Location,
					}

					if target := cge.resolveTarget(className); target != nil {
						dep.ToID = target.ID
					}

					return dep
				}
			}
		}

		parent = parent.Parent()
	}

	return nil
}

// extractDecoratorName extracts the name from a decorator node
func (cge *PythonCallGraphExtractor) extractDecoratorName(node *sitter.Node) string {
	if node == nil || node.Type() != "decorator" {
		return ""
	}

	// Decorator structure: @ expression
	// Walk children to find the decorator expression
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			return cge.nodeText(child)
		case "attribute":
			return cge.nodeText(child)
		case "call":
			// Decorator with arguments, e.g., @decorator(arg)
			return cge.extractCallTarget(child)
		}
	}

	return ""
}

// extractClassName extracts the name from a class_definition node
func (cge *PythonCallGraphExtractor) extractClassName(node *sitter.Node) string {
	if node == nil || node.Type() != "class_definition" {
		return ""
	}

	// Try to find the name child
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return cge.nodeText(nameNode)
	}

	// Fallback: look for identifier after 'class' keyword
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			return cge.nodeText(child)
		}
	}

	return ""
}

// extractCallTarget extracts the function name from a call node
func (cge *PythonCallGraphExtractor) extractCallTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "call" {
		return ""
	}

	// Python call structure:
	// call
	// ├── function (identifier or attribute)
	// └── argument_list (arguments)

	// The function being called is typically the first named child
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil && node.NamedChildCount() > 0 {
		funcNode = node.NamedChild(0)
	}

	if funcNode == nil {
		return ""
	}

	switch funcNode.Type() {
	case "identifier":
		return cge.nodeText(funcNode)

	case "attribute":
		// e.g., obj.method or module.function
		return cge.nodeText(funcNode)

	case "subscript":
		// e.g., funcs[0]()
		return ""

	case "call":
		// Chained call, e.g., func()()
		return ""
	}

	return ""
}

// findFunctionBody finds the block node in a function definition
func (cge *PythonCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// For decorated definitions, get the actual function
	if node.Type() == "decorated_definition" {
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "function_definition" {
				node = child
				break
			}
		}
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

// isConditionalCall checks if a call is inside an if/elif/match statement
func (cge *PythonCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "elif_clause", "match_statement", "try_statement", "except_clause":
			return true
		case "function_definition", "class_definition", "decorated_definition":
			return false // Reached function/class boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isInTypeAnnotationContext checks if an identifier is in a type annotation context
func (cge *PythonCallGraphExtractor) isInTypeAnnotationContext(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "type", "return_type", "annotation":
			return true
		case "function_definition", "class_definition":
			// Check if we're in parameter annotations
			return false
		}
		parent = parent.Parent()
	}
	return false
}

// extractBaseTypeName extracts the base type from complex type annotations
// e.g., Optional[User] -> User, List[str] -> str (but str is builtin so filtered)
func (cge *PythonCallGraphExtractor) extractBaseTypeName(typeName string) string {
	// Handle subscript types like Optional[User], List[User], Dict[str, User]
	if idx := strings.Index(typeName, "["); idx != -1 {
		// Check if it's a generic wrapper (Optional, List, Dict, etc.)
		wrapper := typeName[:idx]
		if isPythonBuiltinType(wrapper) {
			// Extract inner type(s)
			inner := typeName[idx+1 : len(typeName)-1]
			// For Dict[K, V], take the value type
			if strings.Contains(inner, ",") {
				parts := strings.Split(inner, ",")
				if len(parts) > 1 {
					inner = strings.TrimSpace(parts[len(parts)-1])
				}
			}
			return cge.extractBaseTypeName(inner)
		}
		return wrapper
	}

	// Handle attribute types like module.Type
	if strings.Contains(typeName, ".") {
		return typeName // Keep qualified name
	}

	return typeName
}

// isPythonBuiltinType checks if type is a Python builtin
func isPythonBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Basic types
		"str": true, "int": true, "float": true, "bool": true,
		"list": true, "dict": true, "set": true, "tuple": true,
		"None": true, "type": true, "object": true,

		// Typing module types (commonly used in annotations)
		"List": true, "Dict": true, "Set": true, "Tuple": true,
		"Optional": true, "Union": true, "Any": true, "Callable": true,
		"Iterable": true, "Iterator": true, "Generator": true,
		"Sequence": true, "Mapping": true, "MutableMapping": true,
		"Type": true, "Generic": true, "TypeVar": true,
		"Protocol": true, "Final": true, "Literal": true,
		"ClassVar": true, "Annotated": true,

		// Exceptions
		"Exception": true, "BaseException": true,
		"ValueError": true, "TypeError": true, "KeyError": true,
		"IndexError": true, "AttributeError": true, "RuntimeError": true,
		"StopIteration": true, "AssertionError": true, "ImportError": true,
		"OSError": true, "IOError": true, "FileNotFoundError": true,

		// Builtin functions
		"print": true, "len": true, "range": true, "enumerate": true,
		"zip": true, "map": true, "filter": true, "open": true,
		"input": true, "sorted": true, "reversed": true, "abs": true,
		"max": true, "min": true, "sum": true, "all": true, "any": true,
		"isinstance": true, "issubclass": true, "hasattr": true,
		"getattr": true, "setattr": true, "delattr": true,
		"callable": true, "repr": true, "bytes": true, "bytearray": true,
		"id": true, "hash": true, "iter": true, "next": true,
		"super": true, "property": true, "classmethod": true, "staticmethod": true,

		// Special values
		"True": true, "False": true,
		"self": true, "cls": true,
	}
	return builtins[name]
}

// Helper methods

// walkNode performs a depth-first walk of the AST
func (cge *PythonCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
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
func (cge *PythonCallGraphExtractor) nodeText(node *sitter.Node) string {
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
func (cge *PythonCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if cge.result.FilePath != "" {
		return fmt.Sprintf("%s:%d", cge.result.FilePath, line)
	}
	return fmt.Sprintf(":%d", line)
}

// resolveTarget attempts to resolve a target name to an entity
func (cge *PythonCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without module prefix for attribute expressions
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (method/function name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

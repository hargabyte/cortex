// Package extract provides call graph and dependency extraction from parsed AST.
// It extracts function calls, type references, and other relationships between
// code entities to build a dependency graph.
package extract

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// DepType represents the type of dependency relationship
type DepType string

const (
	// Calls represents a function invoking another function
	Calls DepType = "calls"

	// UsesType represents a function/type referencing a type
	UsesType DepType = "uses_type"

	// Implements represents a type implementing an interface
	Implements DepType = "implements"

	// Extends represents a type extending another type (embedding)
	Extends DepType = "extends"

	// MethodOf represents a function being a method of a type
	MethodOf DepType = "method_of"

	// Contains represents a module containing a function/type
	Contains DepType = "contains"

	// Instantiates represents creating an instance of a type (e.g., new ClassName())
	Instantiates DepType = "instantiates"
)

// Dependency represents a relationship between entities
type Dependency struct {
	// FromID is the entity ID of the source (caller/user)
	FromID string

	// ToID is the entity ID of the target (callee/used)
	ToID string

	// ToName is the unresolved target name (used when ToID is not yet resolved)
	ToName string

	// ToQualified is the qualified name (e.g., "pkg.Function")
	ToQualified string

	// DepType is the type of dependency relationship
	DepType DepType

	// Optional indicates if this is a conditional/optional dependency
	Optional bool

	// Location is the file:line where the dependency occurs
	Location string
}

// CallGraphEntity represents a code entity that can be a dependency source or target.
// This is a simplified version for call graph extraction, distinct from the
// full Entity type used for code entity extraction.
type CallGraphEntity struct {
	// ID is the unique identifier for this entity
	ID string

	// Name is the entity name
	Name string

	// QualifiedName is the fully qualified name (e.g., "pkg.TypeName.MethodName")
	QualifiedName string

	// Type is the entity type (function, method, type, interface, struct)
	Type string

	// Location is the file:line location
	Location string

	// Node is the AST node for this entity (used during extraction)
	Node *sitter.Node
}

// CallGraphExtractor extracts dependencies from AST
type CallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewCallGraphExtractor creates a call graph extractor (builds lookup maps internally)
func NewCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *CallGraphExtractor {
	entityByName := make(map[string]*CallGraphEntity)
	entityByID := make(map[string]*CallGraphEntity)

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		entityByName[e.Name] = e
		if e.QualifiedName != "" {
			entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			entityByID[e.ID] = e
		}
	}

	return &CallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// NewCallGraphExtractorWithMaps creates an extractor with pre-built lookup maps (for performance)
// Use this when processing multiple files to avoid rebuilding maps for each file.
func NewCallGraphExtractorWithMaps(result *parser.ParseResult, entities []CallGraphEntity,
	entityByName map[string]*CallGraphEntity, entityByID map[string]*CallGraphEntity) *CallGraphExtractor {
	return &CallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: entityByName,
		entityByID:   entityByID,
	}
}

// ExtractDependencies extracts all dependencies from the parsed code
func (cge *CallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
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

			// Extract method receiver (for methods)
			if entity.Type == "method" {
				if receiverDep := cge.extractMethodReceiver(entity); receiverDep != nil {
					deps = append(deps, *receiverDep)
				}
			}

		case "struct", "type":
			// Extract embedded types (extends relationship)
			embeddedDeps := cge.extractEmbeddedTypes(entity)
			deps = append(deps, embeddedDeps...)

			// Extract interface implementations
			implDeps := cge.extractImplements(entity)
			deps = append(deps, implDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call_expression nodes within a function
func (cge *CallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
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
		if node.Type() == "call_expression" {
			// Get function being called
			callTarget := cge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] {
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

				// Try to resolve to entity ID - only keep resolved dependencies
				// Unresolved calls (stdlib, external packages) are skipped for performance
				if target := cge.resolveTarget(callTarget); target != nil {
					dep.ToID = target.ID

					// Check if call is conditional
					if cge.isConditionalCall(node) {
						dep.Optional = true
					}

					deps = append(deps, dep)
				}
			}
		}
		return true
	})

	return deps
}

// extractTypeReferences finds type identifiers used in function
func (cge *CallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers - only keep resolved references
		if nodeType == "type_identifier" {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !isBuiltinType(typeName) {
				// Only track if it resolves to a known entity
				if target := cge.resolveTarget(typeName); target != nil {
					seen[typeName] = true
					deps = append(deps, Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						ToID:     target.ID,
						DepType:  UsesType,
						Location: cge.nodeLocation(node),
					})
				}
			}
		}

		// Check for qualified types (e.g., pkg.Type) - only keep resolved references
		if nodeType == "qualified_type" || nodeType == "selector_expression" {
			// Only process selector_expression in type context
			if nodeType == "selector_expression" && !cge.isInTypeContext(node) {
				return true
			}

			typeName := cge.nodeText(node)
			if !seen[typeName] && !isBuiltinType(typeName) {
				// Only track if it resolves to a known entity
				if target := cge.resolveTarget(typeName); target != nil {
					seen[typeName] = true
					deps = append(deps, Dependency{
						FromID:      entity.ID,
						ToName:      typeName,
						ToQualified: typeName,
						ToID:        target.ID,
						DepType:     UsesType,
						Location:    cge.nodeLocation(node),
					})
				}
			}
		}

		return true
	})

	return deps
}

// extractMethodReceiver extracts method_of dependency for methods
// Returns nil if the receiver type doesn't resolve to a known entity
func (cge *CallGraphExtractor) extractMethodReceiver(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find receiver parameter
	receiverNode := entity.Node.ChildByFieldName("receiver")
	if receiverNode == nil {
		return nil
	}

	// Extract receiver type
	receiverType := cge.extractReceiverType(receiverNode)
	if receiverType == "" {
		return nil
	}

	// Only return if it resolves to a known entity
	target := cge.resolveTarget(receiverType)
	if target == nil {
		return nil
	}

	return &Dependency{
		FromID:   entity.ID,
		ToName:   receiverType,
		ToID:     target.ID,
		DepType:  MethodOf,
		Location: entity.Location,
	}
}

// extractEmbeddedTypes extracts extends dependencies for embedded types in structs
func (cge *CallGraphExtractor) extractEmbeddedTypes(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// Find struct type body
	var structBody *sitter.Node
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "field_declaration_list" {
			structBody = node
			return false
		}
		return true
	})

	if structBody == nil {
		return deps
	}

	// Look for embedded fields (fields without names) - only keep resolved
	for i := uint32(0); i < structBody.NamedChildCount(); i++ {
		child := structBody.NamedChild(int(i))
		if child.Type() == "field_declaration" {
			// Check if this is an embedded field (no name, just type)
			if cge.isEmbeddedField(child) {
				typeName := cge.extractFieldType(child)
				if typeName != "" && !isBuiltinType(typeName) {
					// Only track if it resolves to a known entity
					if target := cge.resolveTarget(typeName); target != nil {
						deps = append(deps, Dependency{
							FromID:   entity.ID,
							ToName:   typeName,
							ToID:     target.ID,
							DepType:  Extends,
							Location: cge.nodeLocation(child),
						})
					}
				}
			}
		}
	}

	return deps
}

// extractImplements extracts interface implementations
// Note: In Go, this is implicit and would require type analysis
// For now, we look for explicit interface embedding
func (cge *CallGraphExtractor) extractImplements(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	if entity.Node == nil {
		return deps
	}

	// For interface types, look for embedded interfaces
	var interfaceBody *sitter.Node
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		if node.Type() == "interface_type" {
			interfaceBody = node
			return false
		}
		return true
	})

	if interfaceBody == nil {
		return deps
	}

	// Look for embedded interfaces - only keep resolved
	for i := uint32(0); i < interfaceBody.NamedChildCount(); i++ {
		child := interfaceBody.NamedChild(int(i))
		// Type identifiers at the interface level are embedded interfaces
		if child.Type() == "type_identifier" || child.Type() == "qualified_type" {
			typeName := cge.nodeText(child)
			if !isBuiltinType(typeName) {
				// Only track if it resolves to a known entity
				if target := cge.resolveTarget(typeName); target != nil {
					deps = append(deps, Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						ToID:     target.ID,
						DepType:  Implements,
						Location: cge.nodeLocation(child),
					})
				}
			}
		}
	}

	return deps
}

// isConditionalCall checks if a call is inside an if/switch statement
func (cge *CallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_statement", "select_statement":
			return true
		case "function_declaration", "method_declaration", "func_literal":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isBuiltinType checks if type is a Go builtin
func isBuiltinType(name string) bool {
	builtins := map[string]bool{
		"string": true, "int": true, "int8": true, "int16": true,
		"int32": true, "int64": true, "uint": true, "uint8": true,
		"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
		"float32": true, "float64": true, "complex64": true, "complex128": true,
		"bool": true, "byte": true, "rune": true, "error": true,
		"any": true, "interface{}": true, "comparable": true,
	}
	return builtins[name]
}

// Helper methods

// walkNode performs a depth-first walk of the AST
func (cge *CallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
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
func (cge *CallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || cge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	// This can happen if source was truncated or node byte range is invalid
	endByte := node.EndByte()
	if endByte > uint32(len(cge.result.Source)) {
		return ""
	}
	return node.Content(cge.result.Source)
}

// nodeLocation returns file:line for a node
func (cge *CallGraphExtractor) nodeLocation(node *sitter.Node) string {
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
func (cge *CallGraphExtractor) extractCallTarget(node *sitter.Node) string {
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
		return cge.nodeText(funcNode)

	case "selector_expression":
		// e.g., pkg.Function or obj.Method
		return cge.nodeText(funcNode)

	case "parenthesized_expression":
		// e.g., (funcPtr)()
		return ""

	case "index_expression":
		// e.g., funcs[0]()
		return ""
	}

	return ""
}

// findFunctionBody finds the block node in a function declaration
func (cge *CallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
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

// extractReceiverType extracts the type name from a receiver parameter
func (cge *CallGraphExtractor) extractReceiverType(receiverNode *sitter.Node) string {
	if receiverNode == nil {
		return ""
	}

	var typeName string

	// Walk to find the type identifier, stripping pointer
	cge.walkNode(receiverNode, func(node *sitter.Node) bool {
		switch node.Type() {
		case "type_identifier":
			typeName = cge.nodeText(node)
			return false
		case "pointer_type":
			// Continue walking to find the underlying type
			return true
		}
		return true
	})

	return typeName
}

// isInTypeContext checks if a selector_expression is in a type context
func (cge *CallGraphExtractor) isInTypeContext(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "parameter_declaration", "field_declaration", "type_assertion",
			"type_spec", "type_conversion_expression", "composite_literal":
			return true
		case "call_expression":
			// Check if we're the type argument to a type conversion
			funcChild := parent.ChildByFieldName("function")
			return funcChild == node
		}
		parent = parent.Parent()
	}
	return false
}

// isEmbeddedField checks if a field declaration is an embedded field
func (cge *CallGraphExtractor) isEmbeddedField(fieldNode *sitter.Node) bool {
	if fieldNode == nil || fieldNode.Type() != "field_declaration" {
		return false
	}

	// Embedded fields have no name identifier, just a type
	// Check if there's a name child by looking at structure
	nameCount := 0
	typeCount := 0

	for i := uint32(0); i < fieldNode.NamedChildCount(); i++ {
		child := fieldNode.NamedChild(int(i))
		switch child.Type() {
		case "field_identifier":
			nameCount++
		case "type_identifier", "pointer_type", "qualified_type":
			typeCount++
		}
	}

	// Embedded field: has type but no name
	return nameCount == 0 && typeCount > 0
}

// extractFieldType extracts the type from a field declaration
func (cge *CallGraphExtractor) extractFieldType(fieldNode *sitter.Node) string {
	if fieldNode == nil {
		return ""
	}

	var typeName string

	for i := uint32(0); i < fieldNode.NamedChildCount(); i++ {
		child := fieldNode.NamedChild(int(i))
		switch child.Type() {
		case "type_identifier":
			return cge.nodeText(child)
		case "pointer_type":
			// Extract underlying type
			cge.walkNode(child, func(n *sitter.Node) bool {
				if n.Type() == "type_identifier" {
					typeName = cge.nodeText(n)
					return false
				}
				return true
			})
			if typeName != "" {
				return typeName
			}
		case "qualified_type":
			return cge.nodeText(child)
		}
	}

	return typeName
}

// resolveTarget attempts to resolve a target name to an entity
func (cge *CallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without package prefix for selector expressions
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (method/function name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

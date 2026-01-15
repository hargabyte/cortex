// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements Ruby-specific call graph extraction.
package extract

import (
	"fmt"
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// RubyCallGraphExtractor extracts dependencies from Ruby AST.
type RubyCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewRubyCallGraphExtractor creates a call graph extractor for Ruby.
func NewRubyCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *RubyCallGraphExtractor {
	cge := &RubyCallGraphExtractor{
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

// ExtractDependencies extracts all dependencies from the parsed Ruby code.
func (cge *RubyCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range cge.entities {
		entity := &cge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "method", "singleton_method":
			// Extract method calls
			callDeps := cge.extractMethodCalls(entity)
			deps = append(deps, callDeps...)

			// Extract method owner (for instance methods)
			if entity.Type == "method" {
				if ownerDep := cge.extractMethodOwner(entity); ownerDep != nil {
					deps = append(deps, *ownerDep)
				}
			}

		case "class":
			// Extract superclass (extends relationship)
			if superDep := cge.extractSuperclass(entity); superDep != nil {
				deps = append(deps, *superDep)
			}

		case "module":
			// Modules don't typically have explicit dependencies
			// but could have include/extend relationships
		}
	}

	return deps, nil
}

// extractMethodCalls finds call nodes within a method body.
func (cge *RubyCallGraphExtractor) extractMethodCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get method body
	bodyNode := cge.findMethodBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk method body looking for call nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Ruby has several call types
		if nodeType == "call" || nodeType == "method_call" {
			// Get method being called
			callTarget := cge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] && !isRubyBuiltin(callTarget) {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: cge.nodeLocation(node),
				}

				// Parse qualified names (e.g., obj.method)
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

// extractSuperclass extracts the superclass from a class definition.
func (cge *RubyCallGraphExtractor) extractSuperclass(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Find superclass node
	superclassNode := findChildByType(entity.Node, "superclass")
	if superclassNode == nil {
		return nil
	}

	// Extract superclass name
	superclassName := cge.nodeText(superclassNode)
	// Clean up "< ClassName" to just "ClassName"
	superclassName = strings.TrimPrefix(superclassName, "<")
	superclassName = strings.TrimSpace(superclassName)

	if superclassName == "" || isRubyBuiltin(superclassName) {
		return nil
	}

	dep := &Dependency{
		FromID:   entity.ID,
		ToName:   superclassName,
		DepType:  Extends,
		Location: cge.nodeLocation(superclassNode),
	}

	// Try to resolve
	if target := cge.resolveTarget(superclassName); target != nil {
		dep.ToID = target.ID
	}

	return dep
}

// extractMethodOwner extracts the method_of relationship for instance methods.
func (cge *RubyCallGraphExtractor) extractMethodOwner(entity *CallGraphEntity) *Dependency {
	if entity.Node == nil {
		return nil
	}

	// Walk up to find the enclosing class or module
	parent := entity.Node.Parent()
	for parent != nil {
		if parent.Type() == "class" {
			// Extract class name
			className := cge.extractClassName(parent)
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
		} else if parent.Type() == "module" {
			// Extract module name
			moduleName := cge.extractModuleName(parent)
			if moduleName != "" {
				dep := &Dependency{
					FromID:   entity.ID,
					ToName:   moduleName,
					DepType:  MethodOf,
					Location: entity.Location,
				}

				if target := cge.resolveTarget(moduleName); target != nil {
					dep.ToID = target.ID
				}

				return dep
			}
		}

		parent = parent.Parent()
	}

	return nil
}

// extractCallTarget extracts the method name from a call node.
func (cge *RubyCallGraphExtractor) extractCallTarget(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	nodeType := node.Type()
	if nodeType != "call" && nodeType != "method_call" {
		return ""
	}

	// Ruby call structure can vary:
	// call
	// ├── method (identifier or call)
	// └── arguments (optional)

	// Look for identifier or method name
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		switch childType {
		case "identifier":
			return cge.nodeText(child)

		case "constant":
			return cge.nodeText(child)

		case "scope_resolution":
			// e.g., Module::method
			return cge.nodeText(child)

		case "call":
			// Nested call (e.g., obj.method1.method2)
			// Get the rightmost method name
			text := cge.nodeText(child)
			if parts := strings.Split(text, "."); len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}

	// If we can't find specific parts, try getting the whole call text
	// and extract the method name
	text := cge.nodeText(node)
	if text != "" {
		// Handle "obj.method" or "method(args)"
		if idx := strings.Index(text, "("); idx > 0 {
			text = text[:idx]
		}
		if idx := strings.LastIndex(text, "."); idx >= 0 {
			return strings.TrimSpace(text[idx+1:])
		}
		return strings.TrimSpace(text)
	}

	return ""
}

// extractClassName extracts the name from a class node.
func (cge *RubyCallGraphExtractor) extractClassName(node *sitter.Node) string {
	if node == nil || node.Type() != "class" {
		return ""
	}

	// Try constant child
	constantNode := findChildByType(node, "constant")
	if constantNode != nil {
		return cge.nodeText(constantNode)
	}

	// Try scope_resolution (for nested classes)
	scopeNode := findChildByType(node, "scope_resolution")
	if scopeNode != nil {
		return cge.nodeText(scopeNode)
	}

	return ""
}

// extractModuleName extracts the name from a module node.
func (cge *RubyCallGraphExtractor) extractModuleName(node *sitter.Node) string {
	if node == nil || node.Type() != "module" {
		return ""
	}

	// Try constant child
	constantNode := findChildByType(node, "constant")
	if constantNode != nil {
		return cge.nodeText(constantNode)
	}

	// Try scope_resolution (for nested modules)
	scopeNode := findChildByType(node, "scope_resolution")
	if scopeNode != nil {
		return cge.nodeText(scopeNode)
	}

	return ""
}

// findMethodBody finds the body_statement node in a method definition.
func (cge *RubyCallGraphExtractor) findMethodBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// Try finding body_statement child
	bodyNode := findChildByType(node, "body_statement")
	if bodyNode != nil {
		return bodyNode
	}

	// Sometimes the body is direct children
	return node
}

// isConditionalCall checks if a call is inside an if/unless/case statement.
func (cge *RubyCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if", "unless", "case", "when", "rescue":
			return true
		case "method", "singleton_method", "class", "module":
			return false // Reached method/class boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isRubyBuiltin checks if a name is a Ruby builtin method or class.
func isRubyBuiltin(name string) bool {
	builtins := map[string]bool{
		// Common methods
		"puts":     true,
		"print":    true,
		"p":        true,
		"pp":       true,
		"raise":    true,
		"fail":     true,
		"require":  true,
		"require_relative": true,
		"load":     true,
		"new":      true,
		"initialize": true,

		// Attribute methods
		"attr_reader":   true,
		"attr_writer":   true,
		"attr_accessor": true,

		// Core classes
		"String":   true,
		"Integer":  true,
		"Float":    true,
		"Array":    true,
		"Hash":     true,
		"Symbol":   true,
		"Numeric":  true,
		"Object":   true,
		"Class":    true,
		"Module":   true,
		"Proc":     true,
		"Lambda":   true,
		"Range":    true,
		"Regexp":   true,
		"File":     true,
		"Dir":      true,
		"IO":       true,

		// Visibility modifiers
		"private":   true,
		"protected": true,
		"public":    true,
		"module_function": true,

		// Mixin methods
		"include":  true,
		"extend":   true,
		"prepend":  true,

		// Special values
		"nil":   true,
		"true":  true,
		"false": true,
		"self":  true,
		"super": true,

		// Common instance methods
		"each":     true,
		"map":      true,
		"select":   true,
		"reject":   true,
		"find":     true,
		"reduce":   true,
		"inject":   true,
		"to_s":     true,
		"to_i":     true,
		"to_f":     true,
		"to_a":     true,
		"to_h":     true,
		"empty":    true,
		"length":   true,
		"size":     true,
		"first":    true,
		"last":     true,
		"push":     true,
		"pop":      true,
		"shift":    true,
		"unshift":  true,
		"delete":   true,
		"include?": true,
	}
	return builtins[name]
}

// Helper methods

// walkNode performs a depth-first walk of the AST.
func (cge *RubyCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
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

// nodeText returns the source text for a node.
func (cge *RubyCallGraphExtractor) nodeText(node *sitter.Node) string {
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

// nodeLocation returns file:line for a node.
func (cge *RubyCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if cge.result.FilePath != "" {
		return fmt.Sprintf("%s:%d", cge.result.FilePath, line)
	}
	return fmt.Sprintf(":%d", line)
}

// resolveTarget attempts to resolve a target name to an entity.
func (cge *RubyCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without module/class prefix for qualified names
	if strings.Contains(name, ".") || strings.Contains(name, "::") {
		// Try both . and :: separators
		separators := []string{".", "::"}
		for _, sep := range separators {
			if strings.Contains(name, sep) {
				parts := strings.Split(name, sep)
				// Try just the last part (method/class name)
				if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
					return e
				}
			}
		}
	}

	return nil
}

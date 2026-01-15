// Package extract provides Ruby entity extraction from parsed AST trees.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// RubyExtractor extracts code entities from a parsed Ruby AST.
type RubyExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewRubyExtractor creates an extractor for the given parse result.
func NewRubyExtractor(result *parser.ParseResult) *RubyExtractor {
	return &RubyExtractor{
		result: result,
	}
}

// NewRubyExtractorWithBase creates an extractor with a base path for relative paths.
func NewRubyExtractorWithBase(result *parser.ParseResult, basePath string) *RubyExtractor {
	return &RubyExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the Ruby AST.
// Returns methods, classes, modules, and constants.
func (e *RubyExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract methods
	methods, err := e.ExtractMethods()
	if err != nil {
		return nil, err
	}
	entities = append(entities, methods...)

	// Extract classes (and their methods)
	classes, err := e.ExtractClasses()
	if err != nil {
		return nil, err
	}
	entities = append(entities, classes...)

	// Extract modules
	modules, err := e.ExtractModules()
	if err != nil {
		return nil, err
	}
	entities = append(entities, modules...)

	// Extract constants
	consts, err := e.ExtractConstants()
	if err != nil {
		return nil, err
	}
	entities = append(entities, consts...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *RubyExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract module-level methods
	methodNodes := e.result.FindNodesByType("method")
	for _, node := range methodNodes {
		// Skip methods inside classes/modules
		if e.isInsideClassOrModule(node) {
			continue
		}
		entity := e.extractMethod(node, "")
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract singleton methods
	singletonNodes := e.result.FindNodesByType("singleton_method")
	for _, node := range singletonNodes {
		entity := e.extractSingletonMethod(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract classes
	classNodes := e.result.FindNodesByType("class")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})

			// Extract methods from this class
			className := e.getClassName(node)
			methods := e.extractMethodsFromClass(node, className)
			for i := range methods {
				methodNode := methods[i].node
				methodEntity := methods[i].entity
				result = append(result, EntityWithNode{Entity: methodEntity, Node: methodNode})
			}
		}
	}

	// Extract modules
	moduleNodes := e.result.FindNodesByType("module")
	for _, node := range moduleNodes {
		entity := e.extractModule(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})

			// Extract methods from this module
			moduleName := e.getModuleName(node)
			methods := e.extractMethodsFromClass(node, moduleName) // Reuse class method extraction
			for i := range methods {
				methodNode := methods[i].node
				methodEntity := methods[i].entity
				result = append(result, EntityWithNode{Entity: methodEntity, Node: methodNode})
			}
		}
	}

	// Extract constants
	assignmentNodes := e.result.FindNodesByType("assignment")
	for _, node := range assignmentNodes {
		// Only extract top-level constants
		if e.isInsideMethod(node) {
			continue
		}
		entities := e.extractConstantFromAssignment(node)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result, nil
}

// ExtractMethods extracts all module-level method declarations.
func (e *RubyExtractor) ExtractMethods() ([]Entity, error) {
	var entities []Entity

	// Find method nodes
	methodNodes := e.result.FindNodesByType("method")
	for _, node := range methodNodes {
		// Skip methods inside classes/modules
		if e.isInsideClassOrModule(node) {
			continue
		}
		entity := e.extractMethod(node, "")
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Find singleton method nodes (e.g., def self.method_name)
	singletonNodes := e.result.FindNodesByType("singleton_method")
	for _, node := range singletonNodes {
		entity := e.extractSingletonMethod(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractClasses extracts all class definitions.
func (e *RubyExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	// Find class nodes
	classNodes := e.result.FindNodesByType("class")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)

			// Extract methods from this class
			className := e.getClassName(node)
			methods := e.extractMethodsFromClass(node, className)
			for i := range methods {
				entities = append(entities, *methods[i].entity)
			}
		}
	}

	return entities, nil
}

// ExtractModules extracts all module definitions.
func (e *RubyExtractor) ExtractModules() ([]Entity, error) {
	var entities []Entity

	// Find module nodes
	moduleNodes := e.result.FindNodesByType("module")
	for _, node := range moduleNodes {
		entity := e.extractModule(node)
		if entity != nil {
			entities = append(entities, *entity)

			// Extract methods from this module
			moduleName := e.getModuleName(node)
			methods := e.extractMethodsFromClass(node, moduleName)
			for i := range methods {
				entities = append(entities, *methods[i].entity)
			}
		}
	}

	return entities, nil
}

// ExtractConstants extracts module-level constant assignments.
func (e *RubyExtractor) ExtractConstants() ([]Entity, error) {
	var entities []Entity

	// Find assignment nodes
	assignmentNodes := e.result.FindNodesByType("assignment")
	for _, node := range assignmentNodes {
		// Skip if inside method
		if e.isInsideMethod(node) {
			continue
		}

		constEntities := e.extractConstantFromAssignment(node)
		entities = append(entities, constEntities...)
	}

	return entities, nil
}

// extractMethodsFromClass extracts all methods from a class or module definition.
func (e *RubyExtractor) extractMethodsFromClass(classNode *sitter.Node, className string) []methodWithNode {
	var methods []methodWithNode

	// Find body/block containing class content
	bodyNode := findChildByType(classNode, "body_statement")
	if bodyNode == nil {
		bodyNode = classNode // Sometimes methods are direct children
	}

	// Find regular methods
	methodNodes := findNodesByTypeRecursive(bodyNode, "method")
	for _, methodNode := range methodNodes {
		// Only process direct children (skip nested methods)
		if e.isDirectChildOfClass(methodNode, classNode) {
			method := e.extractMethod(methodNode, className)
			if method != nil {
				methods = append(methods, methodWithNode{entity: method, node: methodNode})
			}
		}
	}

	// Find singleton methods (class methods)
	singletonNodes := findNodesByTypeRecursive(bodyNode, "singleton_method")
	for _, singletonNode := range singletonNodes {
		if e.isDirectChildOfClass(singletonNode, classNode) {
			method := e.extractSingletonMethod(singletonNode)
			if method != nil {
				// Set receiver to indicate it's a class method
				if method.Receiver == "" {
					method.Receiver = className
				}
				methods = append(methods, methodWithNode{entity: method, node: singletonNode})
			}
		}
	}

	return methods
}

// extractMethod extracts a method entity from a method node.
func (e *RubyExtractor) extractMethod(node *sitter.Node, className string) *Entity {
	if node == nil || node.Type() != "method" {
		return nil
	}

	// Get method name
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters
	paramsNode := findChildByType(node, "method_parameters")
	params := e.extractRubyParameters(paramsNode)

	// Get method body
	bodyNode := findChildByType(node, "body_statement")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getLineRange(node)

	kind := FunctionEntity
	receiver := ""
	if className != "" {
		kind = MethodEntity
		receiver = className
	}

	entity := &Entity{
		Kind:       kind,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Receiver:   receiver,
		RawBody:    rawBody,
		Visibility: determineRubyVisibility(name),
		Language:   "ruby",
	}

	entity.ComputeHashes()
	return entity
}

// extractSingletonMethod extracts a singleton/class method entity.
func (e *RubyExtractor) extractSingletonMethod(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "singleton_method" {
		return nil
	}

	// Get method name
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		// Try finding it in a different structure
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "identifier" {
				nameNode = child
				break
			}
		}
	}
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get receiver (object) - e.g., "self" in "def self.method_name"
	var receiver string
	objectNode := findChildByType(node, "self")
	if objectNode != nil {
		receiver = "self"
	} else {
		// Could be "def ClassName.method_name"
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "constant" {
				receiver = e.nodeText(child)
				break
			}
		}
	}

	// Get parameters
	paramsNode := findChildByType(node, "method_parameters")
	params := e.extractRubyParameters(paramsNode)

	// Get method body
	bodyNode := findChildByType(node, "body_statement")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       MethodEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Receiver:   receiver,
		RawBody:    rawBody,
		Visibility: determineRubyVisibility(name),
		Language:   "ruby",
	}

	entity.ComputeHashes()
	return entity
}

// extractClass extracts a class entity from a class node.
func (e *RubyExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class" {
		return nil
	}

	// Get class name
	className := e.getClassName(node)
	if className == "" {
		return nil
	}

	// Get superclass (inheritance)
	var implements []string
	superclassNode := findChildByType(node, "superclass")
	if superclassNode != nil {
		superclassName := e.nodeText(superclassNode)
		// Clean up "< ClassName" to just "ClassName"
		superclassName = strings.TrimPrefix(superclassName, "<")
		superclassName = strings.TrimSpace(superclassName)
		if superclassName != "" {
			implements = []string{superclassName}
		}
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       className,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind, // Ruby classes are similar to structs
		Implements: implements,
		Visibility: determineRubyVisibility(className),
		Language:   "ruby",
	}

	entity.ComputeHashes()
	return entity
}

// extractModule extracts a module entity from a module node.
func (e *RubyExtractor) extractModule(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "module" {
		return nil
	}

	// Get module name
	moduleName := e.getModuleName(node)
	if moduleName == "" {
		return nil
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       moduleName,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   InterfaceKind, // Modules are more like interfaces/mixins
		Visibility: determineRubyVisibility(moduleName),
		Language:   "ruby",
	}

	entity.ComputeHashes()
	return entity
}

// extractConstantFromAssignment extracts constants from an assignment node.
func (e *RubyExtractor) extractConstantFromAssignment(node *sitter.Node) []Entity {
	var entities []Entity

	if node == nil || node.Type() != "assignment" {
		return nil
	}

	// Get left-hand side (variable name)
	leftNode := findChildByType(node, "constant")
	if leftNode == nil {
		return nil // Not a constant assignment
	}
	name := e.nodeText(leftNode)

	// Ruby constants start with uppercase letter
	if len(name) == 0 || (name[0] < 'A' || name[0] > 'Z') {
		return nil
	}

	// Get right-hand side (value)
	var value string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		// Skip the constant and assignment operator
		if child.Type() != "constant" && child.Type() != "=" {
			value = e.nodeText(child)
			if len(value) > 50 {
				value = value[:47] + "..."
			}
			break
		}
	}

	startLine, _ := getLineRange(node)

	entity := Entity{
		Kind:       ConstEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		Value:      value,
		Visibility: determineRubyVisibility(name),
		Language:   "ruby",
	}
	entity.ComputeHashes()
	entities = append(entities, entity)

	return entities
}

// extractRubyParameters extracts parameters from a method_parameters node.
func (e *RubyExtractor) extractRubyParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		var param *Param

		switch childType {
		case "identifier":
			// Simple parameter
			name := e.nodeText(child)
			param = &Param{Name: name}

		case "optional_parameter":
			// Parameter with default value: name = value
			nameNode := findChildByType(child, "identifier")
			if nameNode != nil {
				name := e.nodeText(nameNode)
				param = &Param{Name: name}
			}

		case "splat_parameter":
			// *args
			nameNode := findChildByType(child, "identifier")
			if nameNode != nil {
				name := "*" + e.nodeText(nameNode)
				param = &Param{Name: name}
			}

		case "hash_splat_parameter":
			// **kwargs
			nameNode := findChildByType(child, "identifier")
			if nameNode != nil {
				name := "**" + e.nodeText(nameNode)
				param = &Param{Name: name}
			}

		case "block_parameter":
			// &block
			nameNode := findChildByType(child, "identifier")
			if nameNode != nil {
				name := "&" + e.nodeText(nameNode)
				param = &Param{Name: name}
			}

		case "keyword_parameter":
			// Keyword parameter: name:
			nameNode := findChildByType(child, "identifier")
			if nameNode != nil {
				name := e.nodeText(nameNode) + ":"
				param = &Param{Name: name}
			}
		}

		if param != nil {
			params = append(params, *param)
		}
	}

	return params
}

// getClassName extracts the class name from a class node.
func (e *RubyExtractor) getClassName(node *sitter.Node) string {
	if node == nil || node.Type() != "class" {
		return ""
	}

	// Try constant child
	constantNode := findChildByType(node, "constant")
	if constantNode != nil {
		return e.nodeText(constantNode)
	}

	// Try scope_resolution (for nested classes like Module::ClassName)
	scopeNode := findChildByType(node, "scope_resolution")
	if scopeNode != nil {
		return e.nodeText(scopeNode)
	}

	return ""
}

// getModuleName extracts the module name from a module node.
func (e *RubyExtractor) getModuleName(node *sitter.Node) string {
	if node == nil || node.Type() != "module" {
		return ""
	}

	// Try constant child
	constantNode := findChildByType(node, "constant")
	if constantNode != nil {
		return e.nodeText(constantNode)
	}

	// Try scope_resolution (for nested modules)
	scopeNode := findChildByType(node, "scope_resolution")
	if scopeNode != nil {
		return e.nodeText(scopeNode)
	}

	return ""
}

// isInsideClassOrModule checks if a node is inside a class or module definition.
func (e *RubyExtractor) isInsideClassOrModule(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "class" || parent.Type() == "module" {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

// isInsideMethod checks if a node is inside a method definition.
func (e *RubyExtractor) isInsideMethod(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "method" || parent.Type() == "singleton_method" {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

// isDirectChildOfClass checks if a method is a direct child of a class/module.
func (e *RubyExtractor) isDirectChildOfClass(methodNode *sitter.Node, classNode *sitter.Node) bool {
	// Walk up from method to see if we hit the class before hitting another method/class
	parent := methodNode.Parent()
	for parent != nil {
		if parent == classNode {
			return true
		}
		// If we hit another class/module/method first, it's not a direct child
		if parent.Type() == "class" || parent.Type() == "module" || parent.Type() == "method" {
			return false
		}
		parent = parent.Parent()
	}
	return false
}

// getFilePath returns the normalized file path.
func (e *RubyExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *RubyExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// determineRubyVisibility determines visibility from Ruby naming convention.
// Methods starting with _ are considered private by convention.
// Ruby has explicit private/protected/public keywords, but we use naming convention here.
func determineRubyVisibility(name string) Visibility {
	if len(name) == 0 {
		return VisibilityPrivate
	}
	// In Ruby, underscore prefix suggests private by convention
	if strings.HasPrefix(name, "_") {
		return VisibilityPrivate
	}
	return VisibilityPublic
}

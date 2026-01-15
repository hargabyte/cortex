// Package extract provides Python entity extraction from parsed AST trees.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// PythonExtractor extracts code entities from a parsed Python AST.
type PythonExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewPythonExtractor creates an extractor for the given parse result.
func NewPythonExtractor(result *parser.ParseResult) *PythonExtractor {
	return &PythonExtractor{
		result: result,
	}
}

// NewPythonExtractorWithBase creates an extractor with a base path for relative paths.
func NewPythonExtractorWithBase(result *parser.ParseResult, basePath string) *PythonExtractor {
	return &PythonExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the Python AST.
// Returns functions, classes, methods, constants, and imports.
func (e *PythonExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions
	funcs, err := e.ExtractFunctions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, funcs...)

	// Extract classes (and their methods)
	classes, err := e.ExtractClasses()
	if err != nil {
		return nil, err
	}
	entities = append(entities, classes...)

	// Extract module-level constants
	consts, err := e.ExtractConstants()
	if err != nil {
		return nil, err
	}
	entities = append(entities, consts...)

	// Extract imports
	imports, err := e.ExtractImports()
	if err != nil {
		return nil, err
	}
	entities = append(entities, imports...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *PythonExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract functions
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		// Skip methods (functions inside classes)
		if e.isInsideClass(node) {
			continue
		}
		// Skip decorated functions - they'll be processed via decorated_definition
		if e.isDecoratedFunction(node) {
			continue
		}
		entity := e.extractFunction(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract decorated functions (that are not inside classes)
	decoratedNodes := e.result.FindNodesByType("decorated_definition")
	for _, node := range decoratedNodes {
		// Skip if inside a class
		if e.isInsideClass(node) {
			continue
		}
		// Check if it contains a function definition
		if funcNode := findChildByType(node, "function_definition"); funcNode != nil {
			decorators := e.extractDecorators(node)
			entity := e.extractFunction(funcNode)
			if entity != nil {
				entity.Decorators = decorators
				result = append(result, EntityWithNode{Entity: entity, Node: node})
			}
		}
	}

	// Extract classes
	classNodes := e.result.FindNodesByType("class_definition")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})

			// Extract methods from this class
			methods := e.extractMethodsFromClass(node)
			for i := range methods {
				methodNode := methods[i].node
				methodEntity := methods[i].entity
				result = append(result, EntityWithNode{Entity: methodEntity, Node: methodNode})
			}
		}
	}

	// Extract decorated classes
	for _, node := range decoratedNodes {
		// Skip if inside another class
		if e.isInsideClass(node) {
			continue
		}
		if classNode := findChildByType(node, "class_definition"); classNode != nil {
			decorators := e.extractDecorators(node)
			entity := e.extractClass(classNode)
			if entity != nil {
				entity.Decorators = decorators
				result = append(result, EntityWithNode{Entity: entity, Node: node})

				// Extract methods from this class
				methods := e.extractMethodsFromClass(classNode)
				for i := range methods {
					methodNode := methods[i].node
					methodEntity := methods[i].entity
					result = append(result, EntityWithNode{Entity: methodEntity, Node: methodNode})
				}
			}
		}
	}

	// Extract module-level constants
	exprNodes := e.result.FindNodesByType("expression_statement")
	for _, node := range exprNodes {
		// Skip if inside class or function
		if e.isInsideClass(node) || e.isInsideFunction(node) {
			continue
		}
		entities := e.extractConstantFromExpression(node)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	// Extract imports
	importNodes := e.result.FindNodesByType("import_statement")
	for _, node := range importNodes {
		entity := e.extractImport(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	importFromNodes := e.result.FindNodesByType("import_from_statement")
	for _, node := range importFromNodes {
		entities := e.extractImportFrom(node)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result, nil
}

// ExtractFunctions extracts all module-level function declarations.
func (e *PythonExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	// Find function_definition nodes
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		// Skip methods (functions inside classes)
		if e.isInsideClass(node) {
			continue
		}
		// Skip decorated functions - they'll be processed via decorated_definition
		if e.isDecoratedFunction(node) {
			continue
		}
		entity := e.extractFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Find decorated functions
	decoratedNodes := e.result.FindNodesByType("decorated_definition")
	for _, node := range decoratedNodes {
		// Skip if inside a class
		if e.isInsideClass(node) {
			continue
		}
		if funcNode := findChildByType(node, "function_definition"); funcNode != nil {
			decorators := e.extractDecorators(node)
			entity := e.extractFunction(funcNode)
			if entity != nil {
				entity.Decorators = decorators
				// Use the decorated_definition's line range
				startLine, endLine := getLineRange(node)
				entity.StartLine = startLine
				entity.EndLine = endLine
				entities = append(entities, *entity)
			}
		}
	}

	return entities, nil
}

// ExtractClasses extracts all class definitions.
func (e *PythonExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	// Track processed classes to avoid duplicates
	processed := make(map[*sitter.Node]bool)

	// Find decorated classes first
	decoratedNodes := e.result.FindNodesByType("decorated_definition")
	for _, node := range decoratedNodes {
		// Skip if inside a class
		if e.isInsideClass(node) {
			continue
		}
		if classNode := findChildByType(node, "class_definition"); classNode != nil {
			decorators := e.extractDecorators(node)
			entity := e.extractClass(classNode)
			if entity != nil {
				entity.Decorators = decorators
				// Use the decorated_definition's line range
				startLine, endLine := getLineRange(node)
				entity.StartLine = startLine
				entity.EndLine = endLine
				entities = append(entities, *entity)
			}

			// Extract methods from this class
			methods := e.extractMethodsFromClass(classNode)
			for i := range methods {
				entities = append(entities, *methods[i].entity)
			}

			processed[classNode] = true
		}
	}

	// Find non-decorated class_definition nodes
	classNodes := e.result.FindNodesByType("class_definition")
	for _, node := range classNodes {
		if processed[node] {
			continue
		}
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)

			// Extract methods from this class
			methods := e.extractMethodsFromClass(node)
			for i := range methods {
				entities = append(entities, *methods[i].entity)
			}
		}
	}

	return entities, nil
}

// ExtractConstants extracts module-level constant assignments.
func (e *PythonExtractor) ExtractConstants() ([]Entity, error) {
	var entities []Entity

	// Find expression_statement nodes at module level containing assignments
	exprNodes := e.result.FindNodesByType("expression_statement")
	for _, node := range exprNodes {
		// Skip if inside class or function
		if e.isInsideClass(node) || e.isInsideFunction(node) {
			continue
		}

		constEntities := e.extractConstantFromExpression(node)
		entities = append(entities, constEntities...)
	}

	return entities, nil
}

// ExtractImports extracts all import statements.
func (e *PythonExtractor) ExtractImports() ([]Entity, error) {
	var entities []Entity

	// Find import_statement nodes
	importNodes := e.result.FindNodesByType("import_statement")
	for _, node := range importNodes {
		entity := e.extractImport(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Find import_from_statement nodes
	importFromNodes := e.result.FindNodesByType("import_from_statement")
	for _, node := range importFromNodes {
		importEntities := e.extractImportFrom(node)
		entities = append(entities, importEntities...)
	}

	return entities, nil
}

// methodWithNode pairs a method entity with its AST node.
type methodWithNode struct {
	entity *Entity
	node   *sitter.Node
}

// extractMethodsFromClass extracts all methods from a class definition.
func (e *PythonExtractor) extractMethodsFromClass(classNode *sitter.Node) []methodWithNode {
	var methods []methodWithNode

	// Get class name for receiver
	nameNode := findChildByFieldName(classNode, "name")
	if nameNode == nil {
		return nil
	}
	className := e.nodeText(nameNode)

	// Find block containing class body
	blockNode := findChildByType(classNode, "block")
	if blockNode == nil {
		return nil
	}

	// Track processed function nodes
	processed := make(map[*sitter.Node]bool)

	// Find decorated methods first
	decoratedNodes := findNodesByTypeRecursive(blockNode, "decorated_definition")
	for _, decoratedNode := range decoratedNodes {
		if funcNode := findChildByType(decoratedNode, "function_definition"); funcNode != nil {
			decorators := e.extractDecorators(decoratedNode)
			method := e.extractMethod(funcNode, className, decorators)
			if method != nil {
				// Use decorated_definition's line range
				startLine, endLine := getLineRange(decoratedNode)
				method.StartLine = startLine
				method.EndLine = endLine
				methods = append(methods, methodWithNode{entity: method, node: decoratedNode})
			}
			processed[funcNode] = true
		}
	}

	// Find regular methods
	funcNodes := findNodesByTypeRecursive(blockNode, "function_definition")
	for _, funcNode := range funcNodes {
		if processed[funcNode] {
			continue
		}
		// Only process direct children of the class block
		if funcNode.Parent() != blockNode {
			continue
		}
		method := e.extractMethod(funcNode, className, nil)
		if method != nil {
			methods = append(methods, methodWithNode{entity: method, node: funcNode})
		}
	}

	return methods
}

// extractFunction extracts a function entity from a function_definition node.
func (e *PythonExtractor) extractFunction(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "function_definition" {
		return nil
	}

	// Get function name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractPythonParameters(paramsNode, false)

	// Get return type
	var returns []string
	returnTypeNode := findChildByFieldName(node, "return_type")
	if returnTypeNode != nil {
		returnType := e.extractPythonType(returnTypeNode)
		if returnType != "" {
			returns = []string{returnType}
		}
	}

	// Check if async
	isAsync := e.isAsyncFunction(node)

	// Get function body
	bodyNode := findChildByType(node, "block")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    returns,
		RawBody:    rawBody,
		Visibility: determinePythonVisibility(name),
		IsAsync:    isAsync,
		Language:   "python",
	}

	entity.ComputeHashes()
	return entity
}

// extractMethod extracts a method entity from a function_definition node inside a class.
func (e *PythonExtractor) extractMethod(node *sitter.Node, className string, decorators []string) *Entity {
	if node == nil || node.Type() != "function_definition" {
		return nil
	}

	// Get method name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Determine method type from decorators
	methodType := "instance"
	for _, dec := range decorators {
		switch dec {
		case "classmethod":
			methodType = "classmethod"
		case "staticmethod":
			methodType = "staticmethod"
		case "property":
			methodType = "property"
		}
	}

	// Get parameters (skip self/cls for instance/class methods)
	paramsNode := findChildByFieldName(node, "parameters")
	skipFirst := methodType == "instance" || methodType == "classmethod" || methodType == "property"
	params := e.extractPythonParameters(paramsNode, skipFirst)

	// Get return type
	var returns []string
	returnTypeNode := findChildByFieldName(node, "return_type")
	if returnTypeNode != nil {
		returnType := e.extractPythonType(returnTypeNode)
		if returnType != "" {
			returns = []string{returnType}
		}
	}

	// Check if async
	isAsync := e.isAsyncFunction(node)

	// Get method body
	bodyNode := findChildByType(node, "block")
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
		Returns:    returns,
		Receiver:   className,
		RawBody:    rawBody,
		Visibility: determinePythonVisibility(name),
		IsAsync:    isAsync,
		Decorators: decorators,
		Language:   "python",
	}

	entity.ComputeHashes()
	return entity
}

// extractClass extracts a class entity from a class_definition node.
func (e *PythonExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_definition" {
		return nil
	}

	// Get class name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get base classes (inheritance)
	var implements []string
	superclassNode := findChildByFieldName(node, "superclasses")
	if superclassNode == nil {
		// Try argument_list (tree-sitter uses this for base classes)
		superclassNode = findChildByType(node, "argument_list")
	}
	if superclassNode != nil {
		implements = e.extractBaseClasses(superclassNode)
	}

	// Get class fields (from typed assignments in class body)
	var fields []Field
	blockNode := findChildByType(node, "block")
	if blockNode != nil {
		fields = e.extractClassFields(blockNode)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind, // Python classes are similar to structs
		Fields:     fields,
		Implements: implements,
		Visibility: determinePythonVisibility(name),
		Language:   "python",
	}

	entity.ComputeHashes()
	return entity
}

// extractConstantFromExpression extracts constants from an expression_statement with assignment.
func (e *PythonExtractor) extractConstantFromExpression(node *sitter.Node) []Entity {
	var entities []Entity

	// Look for assignment child
	assignNode := findChildByType(node, "assignment")
	if assignNode == nil {
		return nil
	}

	// Get the left-hand side (variable name)
	var names []string
	var typeNode *sitter.Node

	for i := uint32(0); i < assignNode.ChildCount(); i++ {
		child := assignNode.Child(int(i))
		switch child.Type() {
		case "identifier":
			// Only consider UPPER_CASE names as constants at module level
			name := e.nodeText(child)
			if isModuleLevelConstant(name) {
				names = append(names, name)
			}
		case "type":
			typeNode = child
		}
	}

	if len(names) == 0 {
		return nil
	}

	// Get type annotation
	typeName := ""
	if typeNode != nil {
		typeName = e.extractPythonType(typeNode)
	}

	// Get value (right-hand side)
	value := ""
	for i := uint32(0); i < assignNode.ChildCount(); i++ {
		child := assignNode.Child(int(i))
		if child.Type() != "identifier" && child.Type() != "type" && child.Type() != ":" && child.Type() != "=" {
			value = e.nodeText(child)
			if len(value) > 50 {
				value = value[:47] + "..."
			}
			break
		}
	}

	startLine, _ := getLineRange(node)

	for _, name := range names {
		entity := Entity{
			Kind:       ConstEntity,
			Name:       name,
			File:       e.getFilePath(),
			StartLine:  startLine,
			EndLine:    startLine,
			ValueType:  typeName,
			Value:      value,
			Visibility: determinePythonVisibility(name),
			Language:   "python",
		}
		entity.ComputeHashes()
		entities = append(entities, entity)
	}

	return entities
}

// extractImport extracts an import entity from an import_statement node.
func (e *PythonExtractor) extractImport(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "import_statement" {
		return nil
	}

	// Get imported module names
	var names []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "dotted_name" || child.Type() == "aliased_import" {
			name := e.nodeText(child)
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return nil
	}

	startLine, _ := getLineRange(node)

	// Use first import as the entity name
	importPath := names[0]
	importName := extractPythonImportName(importPath)

	return &Entity{
		Kind:       ImportEntity,
		Name:       importName,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		ImportPath: importPath,
		Language:   "python",
	}
}

// extractImportFrom extracts import entities from an import_from_statement node.
func (e *PythonExtractor) extractImportFrom(node *sitter.Node) []Entity {
	var entities []Entity

	if node == nil || node.Type() != "import_from_statement" {
		return nil
	}

	// Get the module being imported from
	var modulePath string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "dotted_name" || child.Type() == "relative_import" {
			modulePath = e.nodeText(child)
			break
		}
	}

	// Get imported names
	var importedNames []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "import_from_list" || child.Type() == "dotted_name" {
			// Skip the module path (first dotted_name)
			if child.Type() == "dotted_name" && e.nodeText(child) == modulePath {
				continue
			}
			// Extract names from import list
			for j := uint32(0); j < child.ChildCount(); j++ {
				item := child.Child(int(j))
				if item.Type() == "identifier" || item.Type() == "dotted_name" {
					importedNames = append(importedNames, e.nodeText(item))
				} else if item.Type() == "aliased_import" {
					// Handle "X as Y"
					if nameNode := findChildByType(item, "identifier"); nameNode != nil {
						importedNames = append(importedNames, e.nodeText(nameNode))
					}
				}
			}
		} else if child.Type() == "identifier" {
			// Single import: from X import Y
			importedNames = append(importedNames, e.nodeText(child))
		}
	}

	startLine, _ := getLineRange(node)

	// Create an entity for each imported name
	for _, name := range importedNames {
		fullPath := modulePath
		if fullPath != "" {
			fullPath = modulePath + "." + name
		} else {
			fullPath = name
		}

		entity := Entity{
			Kind:        ImportEntity,
			Name:        name,
			File:        e.getFilePath(),
			StartLine:   startLine,
			EndLine:     startLine,
			ImportPath:  fullPath,
			ImportAlias: modulePath,
			Language:    "python",
		}
		entities = append(entities, entity)
	}

	return entities
}

// extractPythonParameters extracts parameters from a parameters node.
func (e *PythonExtractor) extractPythonParameters(node *sitter.Node, skipFirst bool) []Param {
	if node == nil {
		return nil
	}

	var params []Param
	skipped := !skipFirst

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		var param *Param

		switch childType {
		case "identifier":
			// Simple parameter without type
			name := e.nodeText(child)
			if !skipped {
				skipped = true
				continue
			}
			param = &Param{Name: name}

		case "typed_parameter":
			// Parameter with type annotation: name: type
			name := ""
			typeName := ""
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				if subChild.Type() == "identifier" {
					name = e.nodeText(subChild)
				} else if subChild.Type() == "type" {
					typeName = e.extractPythonType(subChild)
				}
			}
			if !skipped {
				skipped = true
				continue
			}
			param = &Param{Name: name, Type: typeName}

		case "default_parameter":
			// Parameter with default value: name=value
			name := ""
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				if subChild.Type() == "identifier" {
					name = e.nodeText(subChild)
					break
				}
			}
			if !skipped {
				skipped = true
				continue
			}
			param = &Param{Name: name}

		case "typed_default_parameter":
			// Parameter with type and default: name: type = value
			name := ""
			typeName := ""
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				if subChild.Type() == "identifier" {
					name = e.nodeText(subChild)
				} else if subChild.Type() == "type" {
					typeName = e.extractPythonType(subChild)
				}
			}
			if !skipped {
				skipped = true
				continue
			}
			param = &Param{Name: name, Type: typeName}

		case "list_splat_pattern", "dictionary_splat_pattern":
			// *args or **kwargs
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				if subChild.Type() == "identifier" {
					prefix := "*"
					if childType == "dictionary_splat_pattern" {
						prefix = "**"
					}
					param = &Param{Name: prefix + e.nodeText(subChild)}
					break
				}
			}
		}

		if param != nil {
			params = append(params, *param)
		}
	}

	return params
}

// extractPythonType extracts a type from a type node.
func (e *PythonExtractor) extractPythonType(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Handle direct type node
	if node.Type() == "type" {
		// Get the content - it contains the full type expression
		text := e.nodeText(node)
		return strings.TrimSpace(text)
	}

	// Handle return_type which wraps type
	if node.Type() == "return_type" {
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "type" {
				return e.extractPythonType(child)
			}
		}
	}

	return e.nodeText(node)
}

// extractDecorators extracts decorator names from a decorated_definition node.
func (e *PythonExtractor) extractDecorators(node *sitter.Node) []string {
	var decorators []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "decorator" {
			// Get the decorator expression
			for j := uint32(0); j < child.ChildCount(); j++ {
				subChild := child.Child(int(j))
				if subChild.Type() == "identifier" {
					decorators = append(decorators, e.nodeText(subChild))
				} else if subChild.Type() == "call" {
					// Decorator with arguments: @decorator(args)
					if funcNode := findChildByType(subChild, "identifier"); funcNode != nil {
						decorators = append(decorators, e.nodeText(funcNode))
					} else if attrNode := findChildByType(subChild, "attribute"); attrNode != nil {
						decorators = append(decorators, e.nodeText(attrNode))
					}
				} else if subChild.Type() == "attribute" {
					decorators = append(decorators, e.nodeText(subChild))
				}
			}
		}
	}

	return decorators
}

// extractBaseClasses extracts base class names from an argument_list node.
func (e *PythonExtractor) extractBaseClasses(node *sitter.Node) []string {
	var bases []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "identifier":
			bases = append(bases, e.nodeText(child))
		case "attribute":
			bases = append(bases, e.nodeText(child))
		case "keyword_argument":
			// Skip keyword arguments like metaclass=X
			continue
		}
	}

	return bases
}

// extractClassFields extracts typed fields from a class body.
func (e *PythonExtractor) extractClassFields(blockNode *sitter.Node) []Field {
	var fields []Field

	for i := uint32(0); i < blockNode.ChildCount(); i++ {
		child := blockNode.Child(int(i))
		if child.Type() == "expression_statement" {
			assignNode := findChildByType(child, "assignment")
			if assignNode == nil {
				continue
			}

			// Look for typed assignments (field: type)
			var name string
			var typeName string

			for j := uint32(0); j < assignNode.ChildCount(); j++ {
				subChild := assignNode.Child(int(j))
				switch subChild.Type() {
				case "identifier":
					if name == "" {
						name = e.nodeText(subChild)
					}
				case "type":
					typeName = e.extractPythonType(subChild)
				}
			}

			if name != "" && typeName != "" {
				fields = append(fields, Field{Name: name, Type: typeName})
			}
		}
	}

	return fields
}

// isInsideClass checks if a node is inside a class definition.
func (e *PythonExtractor) isInsideClass(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "class_definition" {
			return true
		}
		// Check if parent is a decorated class
		if parent.Type() == "decorated_definition" {
			if findChildByType(parent, "class_definition") != nil {
				return true
			}
		}
		parent = parent.Parent()
	}
	return false
}

// isInsideFunction checks if a node is inside a function definition.
func (e *PythonExtractor) isInsideFunction(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "function_definition" {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

// isDecoratedFunction checks if a function_definition node is the direct child of a decorated_definition.
// This helps avoid processing decorated functions twice (once as function_definition, once as decorated_definition).
func (e *PythonExtractor) isDecoratedFunction(node *sitter.Node) bool {
	parent := node.Parent()
	return parent != nil && parent.Type() == "decorated_definition"
}

// isAsyncFunction checks if a function is async.
func (e *PythonExtractor) isAsyncFunction(node *sitter.Node) bool {
	// Check if the function has an "async" sibling before "def"
	parent := node.Parent()
	if parent == nil {
		return false
	}

	// Walk through parent's children looking for "async" keyword before this function
	for i := uint32(0); i < parent.ChildCount(); i++ {
		child := parent.Child(int(i))
		if child == node {
			break
		}
		if child.Type() == "async" || e.nodeText(child) == "async" {
			return true
		}
	}

	// Also check the raw source - the function may contain "async def"
	text := e.nodeText(node)
	return strings.HasPrefix(text, "async ")
}

// getFilePath returns the normalized file path.
func (e *PythonExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *PythonExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// determinePythonVisibility determines visibility from Python naming convention.
// Names starting with _ are private, names starting with __ are "very private".
func determinePythonVisibility(name string) Visibility {
	if len(name) == 0 {
		return VisibilityPrivate
	}
	if strings.HasPrefix(name, "_") {
		return VisibilityPrivate
	}
	return VisibilityPublic
}

// isModuleLevelConstant checks if a name looks like a constant (UPPER_CASE).
func isModuleLevelConstant(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Constants are typically UPPER_CASE or Title_Case
	hasUpper := false
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
		}
	}
	// Check if it's mostly uppercase
	if hasUpper {
		upper := strings.ToUpper(name)
		// Allow UPPER_CASE_WITH_UNDERSCORES
		return name == upper || strings.Contains(name, "_")
	}
	return false
}

// extractPythonImportName extracts the package name from an import path.
func extractPythonImportName(path string) string {
	// Handle aliased imports: "module as alias"
	if idx := strings.Index(path, " as "); idx >= 0 {
		return strings.TrimSpace(path[idx+4:])
	}
	// Handle dotted imports: return last component
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

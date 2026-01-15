// Package extract provides TypeScript/JavaScript entity extraction from parsed AST.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// TypeScriptExtractor extracts code entities from TypeScript/JavaScript AST.
type TypeScriptExtractor struct {
	result   *parser.ParseResult
	basePath string
	language string // "typescript" or "javascript"
}

// NewTypeScriptExtractor creates an extractor for TypeScript.
func NewTypeScriptExtractor(result *parser.ParseResult) *TypeScriptExtractor {
	lang := "typescript"
	if result.Language == parser.JavaScript {
		lang = "javascript"
	}
	return &TypeScriptExtractor{
		result:   result,
		language: lang,
	}
}

// NewTypeScriptExtractorWithBase creates an extractor with a base path.
func NewTypeScriptExtractorWithBase(result *parser.ParseResult, basePath string) *TypeScriptExtractor {
	e := NewTypeScriptExtractor(result)
	e.basePath = basePath
	return e
}

// ExtractAll extracts all entities from the TypeScript/JavaScript AST.
func (e *TypeScriptExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions (including arrow functions and function expressions)
	funcs, err := e.ExtractFunctions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, funcs...)

	// Extract classes
	classes, err := e.ExtractClasses()
	if err != nil {
		return nil, err
	}
	entities = append(entities, classes...)

	// Extract interfaces
	interfaces, err := e.ExtractInterfaces()
	if err != nil {
		return nil, err
	}
	entities = append(entities, interfaces...)

	// Extract type aliases
	types, err := e.ExtractTypeAliases()
	if err != nil {
		return nil, err
	}
	entities = append(entities, types...)

	// Extract enums
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

	// Extract variables/constants
	vars, err := e.ExtractVariables()
	if err != nil {
		return nil, err
	}
	entities = append(entities, vars...)

	// Extract imports
	imports, err := e.ExtractImports()
	if err != nil {
		return nil, err
	}
	entities = append(entities, imports...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
func (e *TypeScriptExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract functions
	funcNodes := e.result.FindNodesByType("function_declaration")
	for _, node := range funcNodes {
		entity := e.extractFunction(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract arrow functions (only top-level named ones)
	arrowNodes := e.result.FindNodesByType("arrow_function")
	for _, node := range arrowNodes {
		entity := e.extractArrowFunction(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract function expressions (only named ones or assigned to variables)
	funcExprNodes := e.result.FindNodesByType("function_expression")
	for _, node := range funcExprNodes {
		entity := e.extractFunctionExpression(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract classes
	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract interfaces
	interfaceNodes := e.result.FindNodesByType("interface_declaration")
	for _, node := range interfaceNodes {
		entity := e.extractInterface(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract type aliases
	typeNodes := e.result.FindNodesByType("type_alias_declaration")
	for _, node := range typeNodes {
		entity := e.extractTypeAlias(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract enums
	enumNodes := e.result.FindNodesByType("enum_declaration")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract variables from lexical declarations
	lexicalNodes := e.result.FindNodesByType("lexical_declaration")
	for _, node := range lexicalNodes {
		entities := e.extractLexicalDeclaration(node)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	// Extract imports
	importNodes := e.result.FindNodesByType("import_statement")
	for _, node := range importNodes {
		entity := e.extractImportStatement(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// ExtractFunctions extracts all function declarations.
func (e *TypeScriptExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	// Function declarations
	funcNodes := e.result.FindNodesByType("function_declaration")
	for _, node := range funcNodes {
		entity := e.extractFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Arrow functions (only named/exported ones)
	arrowNodes := e.result.FindNodesByType("arrow_function")
	for _, node := range arrowNodes {
		entity := e.extractArrowFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Function expressions
	funcExprNodes := e.result.FindNodesByType("function_expression")
	for _, node := range funcExprNodes {
		entity := e.extractFunctionExpression(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Generator functions
	genNodes := e.result.FindNodesByType("generator_function_declaration")
	for _, node := range genNodes {
		entity := e.extractFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractFunction extracts a function declaration.
func (e *TypeScriptExtractor) extractFunction(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Get function name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	params := e.extractTSParameters(paramsNode)

	// Get return type annotation (TypeScript)
	returnType := e.extractReturnTypeAnnotation(node)
	var returns []string
	if returnType != "" {
		returns = []string{returnType}
	}

	// Get function body
	bodyNode := node.ChildByFieldName("body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getTSLineRange(node)

	// Check if exported
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    returns,
		RawBody:    rawBody,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractArrowFunction extracts an arrow function (only if it has a name via assignment).
func (e *TypeScriptExtractor) extractArrowFunction(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Arrow functions are only named if assigned to a variable
	// Look for parent variable_declarator
	name := e.findArrowFunctionName(node)
	if name == "" {
		return nil
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	// Arrow functions can have single param without parens
	if paramsNode == nil {
		paramsNode = node.ChildByFieldName("parameter")
	}
	params := e.extractTSParameters(paramsNode)

	// Get return type
	returnType := e.extractReturnTypeAnnotation(node)
	var returns []string
	if returnType != "" {
		returns = []string{returnType}
	}

	// Get body
	bodyNode := node.ChildByFieldName("body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    returns,
		RawBody:    rawBody,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// findArrowFunctionName finds the name of an arrow function from its parent context.
func (e *TypeScriptExtractor) findArrowFunctionName(node *sitter.Node) string {
	parent := node.Parent()
	if parent == nil {
		return ""
	}

	// Check if parent is a variable_declarator
	if parent.Type() == "variable_declarator" {
		nameNode := parent.ChildByFieldName("name")
		if nameNode != nil {
			return e.nodeText(nameNode)
		}
	}

	// Check if parent is a pair (in object)
	if parent.Type() == "pair" {
		keyNode := parent.ChildByFieldName("key")
		if keyNode != nil {
			return e.nodeText(keyNode)
		}
	}

	// Check if parent is an assignment_expression
	if parent.Type() == "assignment_expression" {
		leftNode := parent.ChildByFieldName("left")
		if leftNode != nil && leftNode.Type() == "identifier" {
			return e.nodeText(leftNode)
		}
	}

	return ""
}

// extractFunctionExpression extracts a function expression if named or assigned.
func (e *TypeScriptExtractor) extractFunctionExpression(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Try to get name from the function itself
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = e.nodeText(nameNode)
	}

	// If unnamed, look for parent variable assignment
	if name == "" {
		name = e.findArrowFunctionName(node) // Same pattern as arrow functions
	}

	if name == "" {
		return nil
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	params := e.extractTSParameters(paramsNode)

	// Get return type
	returnType := e.extractReturnTypeAnnotation(node)
	var returns []string
	if returnType != "" {
		returns = []string{returnType}
	}

	// Get body
	bodyNode := node.ChildByFieldName("body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    returns,
		RawBody:    rawBody,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// ExtractClasses extracts all class declarations.
func (e *TypeScriptExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractClass extracts a class declaration.
func (e *TypeScriptExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Get class name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get class body to extract fields and methods
	bodyNode := node.ChildByFieldName("body")
	fields := e.extractClassMembers(bodyNode)

	// Check for extends/implements
	var implements []string
	heritageNode := findTSChildByType(node, "class_heritage")
	if heritageNode != nil {
		implements = e.extractHeritage(heritageNode)
	}

	startLine, endLine := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind, // Use struct for classes
		Fields:     fields,
		Implements: implements,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractClassMembers extracts fields and method signatures from a class body.
func (e *TypeScriptExtractor) extractClassMembers(bodyNode *sitter.Node) []Field {
	if bodyNode == nil {
		return nil
	}

	var fields []Field

	for i := uint32(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(int(i))
		switch child.Type() {
		case "method_definition":
			method := e.extractMethodSignature(child)
			if method != nil {
				fields = append(fields, *method)
			}
		case "public_field_definition", "field_definition":
			field := e.extractFieldDefinition(child)
			if field != nil {
				fields = append(fields, *field)
			}
		case "property_signature":
			prop := e.extractPropertySignature(child)
			if prop != nil {
				fields = append(fields, *prop)
			}
		}
	}

	return fields
}

// extractMethodSignature extracts a method signature from method_definition.
func (e *TypeScriptExtractor) extractMethodSignature(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	params := e.extractTSParameters(paramsNode)

	// Get return type
	returnType := e.extractReturnTypeAnnotation(node)

	// Build signature
	sig := formatTSMethodSignature(params, returnType)

	return &Field{
		Name: name,
		Type: sig,
	}
}

// extractFieldDefinition extracts a class field.
func (e *TypeScriptExtractor) extractFieldDefinition(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type annotation
	typeAnnotation := e.extractTypeAnnotation(node)

	return &Field{
		Name: name,
		Type: typeAnnotation,
	}
}

// extractPropertySignature extracts a property signature from interface.
func (e *TypeScriptExtractor) extractPropertySignature(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type annotation
	typeAnnotation := e.extractTypeAnnotation(node)

	return &Field{
		Name: name,
		Type: typeAnnotation,
	}
}

// ExtractInterfaces extracts all interface declarations.
func (e *TypeScriptExtractor) ExtractInterfaces() ([]Entity, error) {
	var entities []Entity

	interfaceNodes := e.result.FindNodesByType("interface_declaration")
	for _, node := range interfaceNodes {
		entity := e.extractInterface(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractInterface extracts an interface declaration.
func (e *TypeScriptExtractor) extractInterface(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Get interface name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get interface body
	bodyNode := node.ChildByFieldName("body")
	fields := e.extractInterfaceMembers(bodyNode)

	// Check for extends
	var implements []string
	extendsNode := findTSChildByType(node, "extends_type_clause")
	if extendsNode != nil {
		implements = e.extractExtendsClause(extendsNode)
	}

	startLine, endLine := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   InterfaceKind,
		Fields:     fields,
		Implements: implements,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractInterfaceMembers extracts members from an interface body.
func (e *TypeScriptExtractor) extractInterfaceMembers(bodyNode *sitter.Node) []Field {
	if bodyNode == nil {
		return nil
	}

	var fields []Field

	for i := uint32(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(int(i))
		switch child.Type() {
		case "property_signature":
			field := e.extractPropertySignature(child)
			if field != nil {
				fields = append(fields, *field)
			}
		case "method_signature":
			method := e.extractInterfaceMethodSignature(child)
			if method != nil {
				fields = append(fields, *method)
			}
		case "index_signature":
			// Index signatures like [key: string]: any
			sig := e.nodeText(child)
			fields = append(fields, Field{Name: "[index]", Type: sig})
		case "call_signature":
			sig := e.extractCallSignature(child)
			if sig != nil {
				fields = append(fields, *sig)
			}
		}
	}

	return fields
}

// extractInterfaceMethodSignature extracts a method signature from interface.
func (e *TypeScriptExtractor) extractInterfaceMethodSignature(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	paramsNode := node.ChildByFieldName("parameters")
	params := e.extractTSParameters(paramsNode)

	returnType := e.extractReturnTypeAnnotation(node)
	sig := formatTSMethodSignature(params, returnType)

	return &Field{
		Name: name,
		Type: sig,
	}
}

// extractCallSignature extracts a call signature from interface.
func (e *TypeScriptExtractor) extractCallSignature(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	paramsNode := node.ChildByFieldName("parameters")
	params := e.extractTSParameters(paramsNode)

	returnType := e.extractReturnTypeAnnotation(node)
	sig := formatTSMethodSignature(params, returnType)

	return &Field{
		Name: "()",
		Type: sig,
	}
}

// ExtractTypeAliases extracts all type alias declarations.
func (e *TypeScriptExtractor) ExtractTypeAliases() ([]Entity, error) {
	var entities []Entity

	typeNodes := e.result.FindNodesByType("type_alias_declaration")
	for _, node := range typeNodes {
		entity := e.extractTypeAlias(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractTypeAlias extracts a type alias declaration.
func (e *TypeScriptExtractor) extractTypeAlias(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Get type name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get the aliased type
	valueNode := node.ChildByFieldName("value")
	valueType := ""
	if valueNode != nil {
		valueType = e.nodeText(valueNode)
	}

	startLine, endLine := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   AliasKind,
		ValueType:  valueType,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// ExtractEnums extracts all enum declarations.
func (e *TypeScriptExtractor) ExtractEnums() ([]Entity, error) {
	var entities []Entity

	enumNodes := e.result.FindNodesByType("enum_declaration")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractEnum extracts an enum declaration.
func (e *TypeScriptExtractor) extractEnum(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Get enum name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get enum body
	bodyNode := node.ChildByFieldName("body")
	enumValues := e.extractEnumMembers(bodyNode)

	startLine, endLine := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       EnumEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		EnumValues: enumValues,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractEnumMembers extracts members from an enum body.
func (e *TypeScriptExtractor) extractEnumMembers(bodyNode *sitter.Node) []EnumValue {
	if bodyNode == nil {
		return nil
	}

	var values []EnumValue

	for i := uint32(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(int(i))
		if child.Type() == "enum_assignment" || child.Type() == "property_identifier" {
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil && child.Type() == "property_identifier" {
				nameNode = child
			}
			if nameNode != nil {
				name := e.nodeText(nameNode)
				value := ""
				valueNode := child.ChildByFieldName("value")
				if valueNode != nil {
					value = e.nodeText(valueNode)
				}
				values = append(values, EnumValue{Name: name, Value: value})
			}
		}
	}

	return values
}

// ExtractVariables extracts variable/constant declarations.
func (e *TypeScriptExtractor) ExtractVariables() ([]Entity, error) {
	var entities []Entity

	// Lexical declarations (const, let)
	lexicalNodes := e.result.FindNodesByType("lexical_declaration")
	for _, node := range lexicalNodes {
		extracted := e.extractLexicalDeclaration(node)
		entities = append(entities, extracted...)
	}

	// Variable declarations (var)
	varNodes := e.result.FindNodesByType("variable_declaration")
	for _, node := range varNodes {
		extracted := e.extractVariableDeclaration(node)
		entities = append(entities, extracted...)
	}

	return entities, nil
}

// extractLexicalDeclaration extracts const/let declarations.
func (e *TypeScriptExtractor) extractLexicalDeclaration(node *sitter.Node) []Entity {
	if node == nil {
		return nil
	}

	var entities []Entity
	kind := ConstEntity

	// Check if it's const or let
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "let" {
			kind = VarEntity
			break
		}
	}

	// Find all variable_declarator children
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "variable_declarator" {
			entity := e.extractVariableDeclarator(child, kind)
			if entity != nil {
				entities = append(entities, *entity)
			}
		}
	}

	return entities
}

// extractVariableDeclaration extracts var declarations.
func (e *TypeScriptExtractor) extractVariableDeclaration(node *sitter.Node) []Entity {
	if node == nil {
		return nil
	}

	var entities []Entity

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "variable_declarator" {
			entity := e.extractVariableDeclarator(child, VarEntity)
			if entity != nil {
				entities = append(entities, *entity)
			}
		}
	}

	return entities
}

// extractVariableDeclarator extracts a single variable.
func (e *TypeScriptExtractor) extractVariableDeclarator(node *sitter.Node, kind EntityKind) *Entity {
	if node == nil {
		return nil
	}

	// Skip if this is a function/arrow function (already extracted separately)
	valueNode := node.ChildByFieldName("value")
	if valueNode != nil {
		valueType := valueNode.Type()
		if valueType == "arrow_function" || valueType == "function" || valueType == "function_expression" {
			return nil
		}
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type annotation
	typeAnnotation := e.extractTypeAnnotation(node)

	// Get value
	value := ""
	if valueNode != nil {
		value = e.nodeText(valueNode)
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		value = strings.ReplaceAll(value, "\n", " ")
		value = strings.TrimSpace(value)
	}

	startLine, _ := getTSLineRange(node)
	visibility := e.determineVisibility(node, name)

	entity := &Entity{
		Kind:       kind,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		ValueType:  typeAnnotation,
		Value:      value,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// ExtractImports extracts all import statements.
func (e *TypeScriptExtractor) ExtractImports() ([]Entity, error) {
	var entities []Entity

	importNodes := e.result.FindNodesByType("import_statement")
	for _, node := range importNodes {
		entity := e.extractImportStatement(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractImportStatement extracts an import statement.
func (e *TypeScriptExtractor) extractImportStatement(node *sitter.Node) *Entity {
	if node == nil {
		return nil
	}

	// Get import source
	sourceNode := node.ChildByFieldName("source")
	if sourceNode == nil {
		return nil
	}
	importPath := e.nodeText(sourceNode)
	importPath = strings.Trim(importPath, "\"'")

	// Get imported name (simplified - use the module name)
	name := extractTSImportName(importPath)

	startLine, _ := getTSLineRange(node)

	return &Entity{
		Kind:       ImportEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		ImportPath: importPath,
	}
}

// extractTSParameters extracts parameters from a formal_parameters node.
func (e *TypeScriptExtractor) extractTSParameters(paramsNode *sitter.Node) []Param {
	if paramsNode == nil {
		return nil
	}

	var params []Param

	for i := uint32(0); i < paramsNode.ChildCount(); i++ {
		child := paramsNode.Child(int(i))
		switch child.Type() {
		case "required_parameter", "optional_parameter", "rest_parameter":
			param := e.extractTSParameter(child)
			if param != nil {
				params = append(params, *param)
			}
		case "identifier":
			// Simple parameter (JavaScript style)
			name := e.nodeText(child)
			params = append(params, Param{Name: name})
		}
	}

	return params
}

// extractTSParameter extracts a single parameter.
func (e *TypeScriptExtractor) extractTSParameter(node *sitter.Node) *Param {
	if node == nil {
		return nil
	}

	// Get parameter name
	patternNode := node.ChildByFieldName("pattern")
	if patternNode == nil {
		patternNode = node.ChildByFieldName("name")
	}
	if patternNode == nil {
		return nil
	}
	name := e.nodeText(patternNode)

	// Get type annotation
	typeAnnotation := e.extractTypeAnnotation(node)

	// Handle rest parameters
	if node.Type() == "rest_parameter" {
		typeAnnotation = "..." + typeAnnotation
	}

	return &Param{
		Name: name,
		Type: typeAnnotation,
	}
}

// extractTypeAnnotation extracts a type annotation from a node.
func (e *TypeScriptExtractor) extractTypeAnnotation(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Look for type field - in tree-sitter TypeScript, this returns the type_annotation node
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		// If it's a type_annotation, extract the actual type from inside
		if typeNode.Type() == "type_annotation" {
			return e.extractTypeFromAnnotation(typeNode)
		}
		return e.nodeText(typeNode)
	}

	// Look for type_annotation node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_annotation" {
			return e.extractTypeFromAnnotation(child)
		}
	}

	return ""
}

// extractTypeFromAnnotation extracts the type from a type_annotation node.
// Type annotations have format ": Type" - we extract just the Type part.
func (e *TypeScriptExtractor) extractTypeFromAnnotation(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Iterate through children to find the actual type (skip ":")
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		nodeType := child.Type()
		// Skip punctuation and whitespace
		if nodeType == ":" || nodeType == "comment" {
			continue
		}
		// Return the first non-punctuation child which should be the type
		return e.nodeText(child)
	}

	return ""
}

// extractReturnTypeAnnotation extracts return type annotation.
func (e *TypeScriptExtractor) extractReturnTypeAnnotation(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Look for return_type field
	returnNode := node.ChildByFieldName("return_type")
	if returnNode != nil {
		// return_type might be a type_annotation or the type itself
		if returnNode.Type() == "type_annotation" {
			return e.extractTypeFromAnnotation(returnNode)
		}
		return e.nodeText(returnNode)
	}

	// Look for type_annotation after parameters
	paramsFound := false
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "formal_parameters" {
			paramsFound = true
		} else if paramsFound && child.Type() == "type_annotation" {
			return e.extractTypeFromAnnotation(child)
		}
	}

	return ""
}

// extractHeritage extracts extends/implements from class heritage.
func (e *TypeScriptExtractor) extractHeritage(heritageNode *sitter.Node) []string {
	if heritageNode == nil {
		return nil
	}

	var heritage []string
	e.walkTSNode(heritageNode, func(node *sitter.Node) bool {
		if node.Type() == "type_identifier" || node.Type() == "identifier" {
			heritage = append(heritage, e.nodeText(node))
		}
		return true
	})

	return heritage
}

// extractExtendsClause extracts interface extends clause.
func (e *TypeScriptExtractor) extractExtendsClause(extendsNode *sitter.Node) []string {
	if extendsNode == nil {
		return nil
	}

	var extends []string
	for i := uint32(0); i < extendsNode.ChildCount(); i++ {
		child := extendsNode.Child(int(i))
		if child.Type() == "type_identifier" || child.Type() == "generic_type" {
			extends = append(extends, e.nodeText(child))
		}
	}

	return extends
}

// determineVisibility determines entity visibility based on export status.
func (e *TypeScriptExtractor) determineVisibility(node *sitter.Node, name string) Visibility {
	// Check if node has export parent
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "export_statement" {
			return VisibilityPublic
		}
		// Stop at statement boundaries
		if strings.HasSuffix(parent.Type(), "_declaration") || strings.HasSuffix(parent.Type(), "_statement") {
			break
		}
		parent = parent.Parent()
	}

	// Check if node starts with 'export' keyword by looking at previous sibling
	// or if it's inside export statement
	if node.PrevSibling() != nil && e.nodeText(node.PrevSibling()) == "export" {
		return VisibilityPublic
	}

	return VisibilityPrivate
}

// getFilePath returns the normalized file path.
func (e *TypeScriptExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *TypeScriptExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// walkTSNode walks the AST depth-first.
func (e *TypeScriptExtractor) walkTSNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		e.walkTSNode(node.Child(int(i)), fn)
	}
}

// Helper functions

// getTSLineRange returns the start and end line numbers for a node.
func getTSLineRange(node *sitter.Node) (uint32, uint32) {
	start := node.StartPoint().Row + 1
	end := node.EndPoint().Row + 1
	return start, end
}

// findTSChildByType finds the first child of a specific type.
func findTSChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	if node == nil {
		return nil
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// extractTSImportName extracts a module name from import path.
func extractTSImportName(path string) string {
	// Handle scoped packages (@org/pkg)
	if strings.HasPrefix(path, "@") {
		return path
	}
	// Get last part of path
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// formatTSMethodSignature formats a TypeScript method signature.
func formatTSMethodSignature(params []Param, returnType string) string {
	var sb strings.Builder

	sb.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Name != "" {
			sb.WriteString(p.Name)
			if p.Type != "" {
				sb.WriteString(": ")
				sb.WriteString(p.Type)
			}
		} else if p.Type != "" {
			sb.WriteString(p.Type)
		}
	}
	sb.WriteByte(')')

	if returnType != "" {
		sb.WriteString(" => ")
		sb.WriteString(returnType)
	}

	return sb.String()
}

// GetLanguage returns the language for this extractor.
func (e *TypeScriptExtractor) GetLanguage() string {
	return e.language
}

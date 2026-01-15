// Package extract provides C++ entity extraction from parsed AST trees.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// CppExtractor extracts code entities from a parsed C++ AST.
type CppExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewCppExtractor creates an extractor for the given C++ parse result.
func NewCppExtractor(result *parser.ParseResult) *CppExtractor {
	return &CppExtractor{
		result: result,
	}
}

// NewCppExtractorWithBase creates an extractor with a base path for relative paths.
func NewCppExtractorWithBase(result *parser.ParseResult, basePath string) *CppExtractor {
	return &CppExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the C++ AST.
// Returns functions, classes, structs, namespaces, enums, typedefs, macros, and global variables.
func (e *CppExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions
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

	// Extract structs
	structs, err := e.ExtractStructs()
	if err != nil {
		return nil, err
	}
	entities = append(entities, structs...)

	// Extract namespaces
	namespaces, err := e.ExtractNamespaces()
	if err != nil {
		return nil, err
	}
	entities = append(entities, namespaces...)

	// Extract enums
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

	// Extract typedefs and aliases
	typedefs, err := e.ExtractTypedefs()
	if err != nil {
		return nil, err
	}
	entities = append(entities, typedefs...)

	// Extract macros
	macros, err := e.ExtractMacros()
	if err != nil {
		return nil, err
	}
	entities = append(entities, macros...)

	// Extract global variables
	globals, err := e.ExtractGlobalVariables()
	if err != nil {
		return nil, err
	}
	entities = append(entities, globals...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *CppExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract function definitions
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		entity := e.extractFunctionDefinition(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract classes
	classNodes := e.result.FindNodesByType("class_specifier")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract methods from class
		methodsWithNodes := e.extractMethodsFromClass(node)
		result = append(result, methodsWithNodes...)
	}

	// Extract structs
	structNodes := e.result.FindNodesByType("struct_specifier")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract methods from struct
		methodsWithNodes := e.extractMethodsFromClass(node)
		result = append(result, methodsWithNodes...)
	}

	// Extract namespaces
	namespaceNodes := e.result.FindNodesByType("namespace_definition")
	for _, node := range namespaceNodes {
		entity := e.extractNamespace(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract enums
	enumNodes := e.result.FindNodesByType("enum_specifier")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract typedefs
	typedefNodes := e.result.FindNodesByType("type_definition")
	for _, node := range typedefNodes {
		entity := e.extractTypedef(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract alias declarations (using alias = type)
	aliasNodes := e.result.FindNodesByType("alias_declaration")
	for _, node := range aliasNodes {
		entity := e.extractAliasDeclaration(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract macros
	macroNodes := e.result.FindNodesByType("preproc_def")
	for _, node := range macroNodes {
		entity := e.extractMacro(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract function-like macros
	funcMacroNodes := e.result.FindNodesByType("preproc_function_def")
	for _, node := range funcMacroNodes {
		entity := e.extractFunctionMacro(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// ExtractFunctions extracts all function definitions and declarations.
func (e *CppExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	// Extract function definitions
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		entity := e.extractFunctionDefinition(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractClasses extracts all class declarations.
func (e *CppExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	classNodes := e.result.FindNodesByType("class_specifier")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods from class
		methods := e.extractMethodsFromClassEntities(node)
		entities = append(entities, methods...)
	}

	return entities, nil
}

// ExtractStructs extracts all struct definitions.
func (e *CppExtractor) ExtractStructs() ([]Entity, error) {
	var entities []Entity

	structNodes := e.result.FindNodesByType("struct_specifier")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods from struct
		methods := e.extractMethodsFromClassEntities(node)
		entities = append(entities, methods...)
	}

	return entities, nil
}

// ExtractNamespaces extracts all namespace definitions.
func (e *CppExtractor) ExtractNamespaces() ([]Entity, error) {
	var entities []Entity

	namespaceNodes := e.result.FindNodesByType("namespace_definition")
	for _, node := range namespaceNodes {
		entity := e.extractNamespace(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractEnums extracts all enum definitions.
func (e *CppExtractor) ExtractEnums() ([]Entity, error) {
	var entities []Entity

	enumNodes := e.result.FindNodesByType("enum_specifier")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractTypedefs extracts all typedef declarations and alias declarations.
func (e *CppExtractor) ExtractTypedefs() ([]Entity, error) {
	var entities []Entity

	// Extract typedef statements
	typedefNodes := e.result.FindNodesByType("type_definition")
	for _, node := range typedefNodes {
		entity := e.extractTypedef(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Extract using alias = type statements
	aliasNodes := e.result.FindNodesByType("alias_declaration")
	for _, node := range aliasNodes {
		entity := e.extractAliasDeclaration(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractMacros extracts all macro definitions.
func (e *CppExtractor) ExtractMacros() ([]Entity, error) {
	var entities []Entity

	// Simple macros (#define NAME value)
	macroNodes := e.result.FindNodesByType("preproc_def")
	for _, node := range macroNodes {
		entity := e.extractMacro(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Function-like macros (#define NAME(args) body)
	funcMacroNodes := e.result.FindNodesByType("preproc_function_def")
	for _, node := range funcMacroNodes {
		entity := e.extractFunctionMacro(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractGlobalVariables extracts global variable declarations.
func (e *CppExtractor) ExtractGlobalVariables() ([]Entity, error) {
	var entities []Entity

	declNodes := e.result.FindNodesByType("declaration")
	for _, node := range declNodes {
		// Only process file-scope declarations
		if !e.isFileScopeDeclaration(node) {
			continue
		}
		// Skip function declarations
		if e.isFunctionDeclaration(node) {
			continue
		}
		vars := e.extractVariableDeclaration(node)
		entities = append(entities, vars...)
	}

	return entities, nil
}

// extractFunctionDefinition extracts a function entity from a function_definition node.
func (e *CppExtractor) extractFunctionDefinition(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "function_definition" {
		return nil
	}

	// Get function declarator
	declarator := e.findFunctionDeclarator(node)
	if declarator == nil {
		return nil
	}

	// Get function name (could be qualified like MyClass::method)
	name := e.extractDeclaratorName(declarator)
	if name == "" {
		return nil
	}

	// Check if this is a method (has :: in name or is inside a class)
	isMethod := strings.Contains(name, "::")
	className := ""
	methodName := name

	if isMethod {
		parts := strings.Split(name, "::")
		if len(parts) >= 2 {
			className = strings.Join(parts[:len(parts)-1], "::")
			methodName = parts[len(parts)-1]
		}
	}

	// Check if function is inside a class body
	if !isMethod {
		classNode := e.findParentClass(node)
		if classNode != nil {
			classNameNode := findChildByType(classNode, "type_identifier")
			if classNameNode != nil {
				className = e.nodeText(classNameNode)
				isMethod = true
			}
		}
	}

	// Detect constructor/destructor
	isConstructor := false
	isDestructor := false
	if isMethod && className != "" {
		baseClassName := className
		if strings.Contains(baseClassName, "::") {
			parts := strings.Split(baseClassName, "::")
			baseClassName = parts[len(parts)-1]
		}
		if methodName == baseClassName {
			isConstructor = true
		} else if strings.HasPrefix(methodName, "~") && methodName[1:] == baseClassName {
			isDestructor = true
		}
	}

	// Get parameters
	params := e.extractFunctionParameters(declarator)

	// Get return type
	returnType := ""
	if !isConstructor && !isDestructor {
		returnType = e.extractReturnType(node)
	}

	// Check visibility (public/private/protected)
	visibility := e.determineCppVisibility(node, isMethod)

	// Check for const, static, virtual, override, final
	modifiers := e.extractFunctionModifiers(node)

	// Get function body for hash
	bodyNode := findChildByType(node, "compound_statement")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       methodName,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Visibility: visibility,
		Language:   "cpp",
		RawBody:    rawBody,
	}

	// Set return type
	if returnType != "" {
		entity.Returns = []string{returnType}
	}

	// If it's a method, set receiver
	if isMethod && className != "" {
		entity.Receiver = className
		// Qualified name for methods
		entity.Name = name
	}

	// Add modifiers to value type
	if len(modifiers) > 0 {
		entity.ValueType = strings.Join(modifiers, " ")
	}

	// Mark special methods
	if isConstructor {
		entity.ValueType = "constructor"
	} else if isDestructor {
		entity.ValueType = "destructor"
	}

	entity.ComputeHashes()
	return entity
}

// extractClass extracts a class entity from a class_specifier node.
func (e *CppExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_specifier" {
		return nil
	}

	// Get class name
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		// Anonymous class
		return nil
	}
	name := e.nodeText(nameNode)

	// Check if this is a class definition (has field_declaration_list) or just a reference
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		// This is a class reference, not a definition
		return nil
	}

	// Extract base classes (inheritance)
	var baseClasses []string
	baseClauseNode := findChildByType(node, "base_class_clause")
	if baseClauseNode != nil {
		baseClasses = e.extractBaseClasses(baseClauseNode)
	}

	// Extract fields (member variables)
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Implements: baseClasses,
		Visibility: VisibilityPublic,
		Language:   "cpp",
		ValueType:  "class",
	}

	entity.ComputeHashes()
	return entity
}

// extractStruct extracts a struct entity from a struct_specifier node.
func (e *CppExtractor) extractStruct(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "struct_specifier" {
		return nil
	}

	// Get struct name
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		// Anonymous struct
		return nil
	}
	name := e.nodeText(nameNode)

	// Check if this is a struct definition (has field_declaration_list) or just a reference
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		// This is a struct reference, not a definition
		return nil
	}

	// Extract base classes (structs can inherit too)
	var baseClasses []string
	baseClauseNode := findChildByType(node, "base_class_clause")
	if baseClauseNode != nil {
		baseClasses = e.extractBaseClasses(baseClauseNode)
	}

	// Extract fields
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Implements: baseClasses,
		Visibility: VisibilityPublic, // Structs have public default visibility
		Language:   "cpp",
		ValueType:  "struct",
	}

	entity.ComputeHashes()
	return entity
}

// extractNamespace extracts a namespace entity from a namespace_definition node.
func (e *CppExtractor) extractNamespace(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "namespace_definition" {
		return nil
	}

	// Get namespace name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		// Anonymous namespace
		return nil
	}
	name := e.nodeText(nameNode)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   InterfaceKind, // Using InterfaceKind as namespace container
		Visibility: VisibilityPublic,
		Language:   "cpp",
		ValueType:  "namespace",
	}

	entity.ComputeHashes()
	return entity
}

// extractEnum extracts an enum entity from an enum_specifier node.
func (e *CppExtractor) extractEnum(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "enum_specifier" {
		return nil
	}

	// Get enum name
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		// Anonymous enum
		return nil
	}
	name := e.nodeText(nameNode)

	// Check if this is an enum definition (has enumerator_list) or just a reference
	enumList := findChildByType(node, "enumerator_list")
	if enumList == nil {
		// This is an enum reference, not a definition
		return nil
	}

	// Check for enum class vs regular enum
	isEnumClass := false
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "class" {
			isEnumClass = true
			break
		}
	}

	// Get enum values
	enumValues := e.extractEnumValues(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       EnumEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		EnumValues: enumValues,
		Visibility: VisibilityPublic,
		Language:   "cpp",
	}

	if isEnumClass {
		entity.ValueType = "enum class"
	}

	entity.ComputeHashes()
	return entity
}

// extractTypedef extracts a typedef entity from a type_definition node.
func (e *CppExtractor) extractTypedef(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "type_definition" {
		return nil
	}

	// Get the type declarator to find the new type name
	var name string
	var baseType string

	// Find the declarator with the typedef name
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "type_identifier" {
			// This could be the typedef name or the base type
			if name == "" {
				baseType = e.nodeText(child)
			}
		} else if childType == "primitive_type" || childType == "sized_type_specifier" {
			baseType = e.nodeText(child)
		} else if childType == "class_specifier" || childType == "struct_specifier" || childType == "enum_specifier" {
			baseType = e.nodeText(child)
		}
	}

	// Find the actual typedef name from declarators
	declarators := e.findTypedefDeclarators(node)
	if len(declarators) > 0 {
		name = declarators[0]
	}

	if name == "" {
		return nil
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   AliasKind,
		ValueType:  baseType,
		Visibility: VisibilityPublic,
		Language:   "cpp",
	}

	entity.ComputeHashes()
	return entity
}

// extractAliasDeclaration extracts a using alias declaration (using alias = type).
func (e *CppExtractor) extractAliasDeclaration(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "alias_declaration" {
		return nil
	}

	// Get alias name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type being aliased
	typeNode := node.ChildByFieldName("type")
	baseType := ""
	if typeNode != nil {
		baseType = e.nodeText(typeNode)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   AliasKind,
		ValueType:  baseType,
		Visibility: VisibilityPublic,
		Language:   "cpp",
	}

	entity.ComputeHashes()
	return entity
}

// extractMacro extracts a simple macro definition.
func (e *CppExtractor) extractMacro(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "preproc_def" {
		return nil
	}

	// Get macro name
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get macro value
	value := ""
	valueNode := findChildByType(node, "preproc_arg")
	if valueNode != nil {
		value = strings.TrimSpace(e.nodeText(valueNode))
		if len(value) > 50 {
			value = value[:47] + "..."
		}
	}

	startLine, _ := getLineRange(node)

	entity := &Entity{
		Kind:       ConstEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		Value:      value,
		Visibility: VisibilityPublic,
		Language:   "cpp",
	}

	entity.ComputeHashes()
	return entity
}

// extractFunctionMacro extracts a function-like macro definition.
func (e *CppExtractor) extractFunctionMacro(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "preproc_function_def" {
		return nil
	}

	// Get macro name
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters
	var params []Param
	paramsNode := findChildByType(node, "preproc_params")
	if paramsNode != nil {
		for i := uint32(0); i < paramsNode.ChildCount(); i++ {
			child := paramsNode.Child(int(i))
			if child.Type() == "identifier" {
				params = append(params, Param{Name: e.nodeText(child)})
			}
		}
	}

	// Get macro body
	rawBody := ""
	bodyNode := findChildByType(node, "preproc_arg")
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
		RawBody:    rawBody,
		Visibility: VisibilityPublic,
		Language:   "cpp",
		ValueType:  "macro",
	}

	entity.ComputeHashes()
	return entity
}

// extractVariableDeclaration extracts global variable declarations.
func (e *CppExtractor) extractVariableDeclaration(node *sitter.Node) []Entity {
	var entities []Entity

	if node == nil || node.Type() != "declaration" {
		return nil
	}

	// Get type
	typeName := e.extractDeclarationType(node)

	// Check visibility
	visibility := e.determineCppVisibilityForDecl(node)

	// Find declarators
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "init_declarator" || child.Type() == "identifier" {
			var name string
			var value string

			if child.Type() == "init_declarator" {
				// Has initializer
				declNode := findChildByType(child, "identifier")
				if declNode == nil {
					declNode = findChildByType(child, "pointer_declarator")
					if declNode != nil {
						declNode = findChildByType(declNode, "identifier")
					}
				}
				if declNode != nil {
					name = e.nodeText(declNode)
				}

				// Get value
				for j := uint32(0); j < child.ChildCount(); j++ {
					valChild := child.Child(int(j))
					if valChild.Type() != "identifier" && valChild.Type() != "pointer_declarator" && valChild.Type() != "=" {
						value = e.nodeText(valChild)
						if len(value) > 50 {
							value = value[:47] + "..."
						}
						break
					}
				}
			} else {
				// Just identifier
				name = e.nodeText(child)
			}

			if name == "" {
				continue
			}

			startLine, _ := getLineRange(node)

			entity := Entity{
				Kind:       VarEntity,
				Name:       name,
				File:       e.getFilePath(),
				StartLine:  startLine,
				EndLine:    startLine,
				ValueType:  typeName,
				Value:      value,
				Visibility: visibility,
				Language:   "cpp",
			}
			entity.ComputeHashes()
			entities = append(entities, entity)
		}
	}

	return entities
}

// extractMethodsFromClass extracts methods from a class/struct definition.
func (e *CppExtractor) extractMethodsFromClass(node *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Find field_declaration_list
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		return nil
	}

	// Walk through field list looking for function_definition and declaration
	for i := uint32(0); i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(int(i))
		if child.Type() == "function_definition" {
			entity := e.extractFunctionDefinition(child)
			if entity != nil {
				result = append(result, EntityWithNode{Entity: entity, Node: child})
			}
		}
	}

	return result
}

// extractMethodsFromClassEntities extracts methods as entities (no nodes).
func (e *CppExtractor) extractMethodsFromClassEntities(node *sitter.Node) []Entity {
	var result []Entity

	// Find field_declaration_list
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		return nil
	}

	// Walk through field list looking for function_definition
	for i := uint32(0); i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(int(i))
		if child.Type() == "function_definition" {
			entity := e.extractFunctionDefinition(child)
			if entity != nil {
				result = append(result, *entity)
			}
		}
	}

	return result
}

// Helper methods

// isFunctionDeclaration checks if a declaration node is a function declaration.
func (e *CppExtractor) isFunctionDeclaration(node *sitter.Node) bool {
	return e.findFunctionDeclarator(node) != nil
}

// isFileScopeDeclaration checks if a declaration is at file scope.
func (e *CppExtractor) isFileScopeDeclaration(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return true
	}
	// File-scope declarations have translation_unit as parent
	return parent.Type() == "translation_unit"
}

// findFunctionDeclarator finds the function_declarator node within a declaration.
func (e *CppExtractor) findFunctionDeclarator(node *sitter.Node) *sitter.Node {
	var result *sitter.Node
	e.walkNode(node, func(n *sitter.Node) bool {
		if n.Type() == "function_declarator" {
			result = n
			return false
		}
		return true
	})
	return result
}

// findParentClass finds the enclosing class or struct node.
func (e *CppExtractor) findParentClass(node *sitter.Node) *sitter.Node {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "class_specifier" || parent.Type() == "struct_specifier" {
			return parent
		}
		parent = parent.Parent()
	}
	return nil
}

// extractDeclaratorName extracts the name from a declarator.
func (e *CppExtractor) extractDeclaratorName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// For function_declarator, look for declarator child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "identifier" {
			return e.nodeText(child)
		}

		// Qualified identifier (MyClass::method)
		if childType == "qualified_identifier" {
			return e.nodeText(child)
		}

		// Destructor (~MyClass)
		if childType == "destructor_name" {
			return e.nodeText(child)
		}

		// Field identifier
		if childType == "field_identifier" {
			return e.nodeText(child)
		}

		// If we encounter a pointer_declarator, look inside it
		if childType == "pointer_declarator" || childType == "reference_declarator" {
			return e.extractDeclaratorName(child)
		}

		// Array declarator
		if childType == "array_declarator" {
			return e.extractDeclaratorName(child)
		}

		// Don't recurse into parameter_list
		if childType == "parameter_list" {
			continue
		}
	}

	return ""
}

// extractFunctionParameters extracts parameters from a function_declarator.
func (e *CppExtractor) extractFunctionParameters(declarator *sitter.Node) []Param {
	if declarator == nil {
		return nil
	}

	var params []Param

	// Find parameter_list
	paramList := findChildByType(declarator, "parameter_list")
	if paramList == nil {
		return nil
	}

	// Extract parameter declarations
	for i := uint32(0); i < paramList.ChildCount(); i++ {
		child := paramList.Child(int(i))
		if child.Type() == "parameter_declaration" || child.Type() == "optional_parameter_declaration" {
			param := e.extractParameter(child)
			if param != nil {
				params = append(params, *param)
			}
		} else if child.Type() == "variadic_parameter_declaration" {
			params = append(params, Param{Name: "...", Type: "..."})
		}
	}

	return params
}

// extractParameter extracts a single parameter from a parameter_declaration.
func (e *CppExtractor) extractParameter(node *sitter.Node) *Param {
	if node == nil {
		return nil
	}

	var name string
	var typeName string

	// Get type and name
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		// Extract type components
		if childType == "primitive_type" || childType == "type_identifier" || childType == "sized_type_specifier" {
			if typeName == "" {
				typeName = e.nodeText(child)
			}
		}

		// Get name from identifier
		if childType == "identifier" {
			name = e.nodeText(child)
		}

		// Handle pointer declarator
		if childType == "pointer_declarator" || childType == "reference_declarator" {
			idNode := findChildByType(child, "identifier")
			if idNode != nil {
				name = e.nodeText(idNode)
			}
			if childType == "pointer_declarator" {
				typeName += "*"
			} else {
				typeName += "&"
			}
		}

		// Handle array declarator
		if childType == "array_declarator" {
			idNode := findChildByType(child, "identifier")
			if idNode != nil {
				name = e.nodeText(idNode)
			}
			typeName += "[]"
		}

		// Handle abstract declarator (parameter without name)
		if childType == "abstract_pointer_declarator" {
			typeName += "*"
		}
		if childType == "abstract_reference_declarator" {
			typeName += "&"
		}
	}

	return &Param{Name: name, Type: typeName}
}

// extractReturnType extracts the return type from a function_definition.
func (e *CppExtractor) extractReturnType(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	var typeSpecs []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		switch childType {
		case "primitive_type", "type_identifier", "sized_type_specifier":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "type_qualifier":
			// Include const, volatile
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "storage_class_specifier":
			// Skip static, extern, inline for return type
			continue
		case "class_specifier", "struct_specifier", "enum_specifier":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "qualified_identifier":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "template_type":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "function_declarator", "pointer_declarator", "reference_declarator":
			// Stop when we hit the declarator
			break
		}
	}

	if len(typeSpecs) == 0 {
		return "void"
	}

	return strings.Join(typeSpecs, " ")
}

// extractDeclarationType extracts the type from a declaration node.
func (e *CppExtractor) extractDeclarationType(node *sitter.Node) string {
	return e.extractReturnType(node)
}

// extractBaseClasses extracts base classes from a base_class_clause.
func (e *CppExtractor) extractBaseClasses(node *sitter.Node) []string {
	var baseClasses []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" || child.Type() == "qualified_identifier" {
			baseClasses = append(baseClasses, e.nodeText(child))
		}
	}

	return baseClasses
}

// extractClassFields extracts fields from a class/struct body.
func (e *CppExtractor) extractClassFields(node *sitter.Node) []Field {
	var fields []Field

	// Find field_declaration_list
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		return nil
	}

	// Current visibility (default: private for class, public for struct)
	currentVisibility := VisibilityPrivate
	if node.Type() == "struct_specifier" {
		currentVisibility = VisibilityPublic
	}

	// Extract field declarations
	for i := uint32(0); i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(int(i))

		// Handle access specifiers
		if child.Type() == "access_specifier" {
			spec := e.nodeText(child)
			if strings.Contains(spec, "public") {
				currentVisibility = VisibilityPublic
			} else if strings.Contains(spec, "protected") {
				currentVisibility = VisibilityProtected
			} else if strings.Contains(spec, "private") {
				currentVisibility = VisibilityPrivate
			}
			continue
		}

		// Extract field declaration
		if child.Type() == "field_declaration" {
			extracted := e.extractFieldDecl(child, currentVisibility)
			fields = append(fields, extracted...)
		}
	}

	return fields
}

// extractFieldDecl extracts fields from a field_declaration.
func (e *CppExtractor) extractFieldDecl(node *sitter.Node, visibility Visibility) []Field {
	var fields []Field

	if node == nil {
		return nil
	}

	// Get type
	var typeName string
	var fieldNames []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		switch childType {
		case "primitive_type", "type_identifier", "sized_type_specifier":
			typeName = e.nodeText(child)
		case "class_specifier", "struct_specifier", "enum_specifier":
			typeName = e.nodeText(child)
		case "qualified_identifier":
			typeName = e.nodeText(child)
		case "field_identifier":
			fieldNames = append(fieldNames, e.nodeText(child))
		case "pointer_declarator", "reference_declarator":
			idNode := findChildByType(child, "field_identifier")
			if idNode != nil {
				fieldNames = append(fieldNames, e.nodeText(idNode))
				if childType == "pointer_declarator" {
					typeName = "*" + typeName
				} else {
					typeName = "&" + typeName
				}
			}
		case "array_declarator":
			idNode := findChildByType(child, "field_identifier")
			if idNode != nil {
				fieldNames = append(fieldNames, e.nodeText(idNode))
				typeName = typeName + "[]"
			}
		}
	}

	for _, name := range fieldNames {
		fields = append(fields, Field{
			Name:       name,
			Type:       typeName,
			Visibility: visibility,
		})
	}

	return fields
}

// extractEnumValues extracts values from an enum body.
func (e *CppExtractor) extractEnumValues(node *sitter.Node) []EnumValue {
	var values []EnumValue

	// Find enumerator_list
	enumList := findChildByType(node, "enumerator_list")
	if enumList == nil {
		return nil
	}

	// Extract enumerators
	for i := uint32(0); i < enumList.ChildCount(); i++ {
		child := enumList.Child(int(i))
		if child.Type() == "enumerator" {
			ev := e.extractEnumerator(child)
			if ev != nil {
				values = append(values, *ev)
			}
		}
	}

	return values
}

// extractEnumerator extracts a single enum value.
func (e *CppExtractor) extractEnumerator(node *sitter.Node) *EnumValue {
	if node == nil {
		return nil
	}

	var name string
	var value string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			name = e.nodeText(child)
		} else if child.Type() == "number_literal" || child.Type() == "binary_expression" {
			value = e.nodeText(child)
		}
	}

	if name == "" {
		return nil
	}

	return &EnumValue{Name: name, Value: value}
}

// findTypedefDeclarators finds typedef declarator names.
func (e *CppExtractor) findTypedefDeclarators(node *sitter.Node) []string {
	var names []string

	e.walkNode(node, func(n *sitter.Node) bool {
		if n.Type() == "type_identifier" {
			// Skip if this is inside a struct/union/enum specifier
			parent := n.Parent()
			if parent != nil {
				pt := parent.Type()
				if pt == "struct_specifier" || pt == "union_specifier" || pt == "enum_specifier" || pt == "class_specifier" {
					return true
				}
			}
			names = append(names, e.nodeText(n))
		}
		return true
	})

	// The last identifier is typically the typedef name
	if len(names) > 0 {
		return []string{names[len(names)-1]}
	}

	return nil
}

// determineCppVisibility determines visibility based on context.
func (e *CppExtractor) determineCppVisibility(node *sitter.Node, isMethod bool) Visibility {
	// For methods, need to find the access specifier
	if isMethod {
		// Find the parent field_declaration_list
		parent := node.Parent()
		for parent != nil && parent.Type() != "field_declaration_list" {
			parent = parent.Parent()
		}

		if parent != nil {
			// Walk backwards to find the last access_specifier before this node
			nodeIdx := -1
			for i := uint32(0); i < parent.ChildCount(); i++ {
				if parent.Child(int(i)) == node {
					nodeIdx = int(i)
					break
				}
			}

			if nodeIdx >= 0 {
				for i := nodeIdx - 1; i >= 0; i-- {
					child := parent.Child(i)
					if child.Type() == "access_specifier" {
						spec := e.nodeText(child)
						if strings.Contains(spec, "public") {
							return VisibilityPublic
						} else if strings.Contains(spec, "protected") {
							return VisibilityProtected
						} else if strings.Contains(spec, "private") {
							return VisibilityPrivate
						}
					}
				}
			}

			// Check if parent is inside a class or struct
			classParent := parent.Parent()
			if classParent != nil {
				if classParent.Type() == "struct_specifier" {
					return VisibilityPublic // Default for struct
				}
				return VisibilityPrivate // Default for class
			}
		}
	}

	// Check for static keyword
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "storage_class_specifier" {
			if e.nodeText(child) == "static" {
				return VisibilityPrivate
			}
		}
	}

	return VisibilityPublic
}

// determineCppVisibilityForDecl determines visibility for a declaration.
func (e *CppExtractor) determineCppVisibilityForDecl(node *sitter.Node) Visibility {
	// Check for static keyword
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "storage_class_specifier" {
			if e.nodeText(child) == "static" {
				return VisibilityPrivate
			}
		}
	}
	return VisibilityPublic
}

// extractFunctionModifiers extracts modifiers like const, static, virtual, override, final.
func (e *CppExtractor) extractFunctionModifiers(node *sitter.Node) []string {
	var modifiers []string
	seen := make(map[string]bool)

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "storage_class_specifier" {
			mod := e.nodeText(child)
			if !seen[mod] {
				modifiers = append(modifiers, mod)
				seen[mod] = true
			}
		}

		if childType == "virtual_specifier" || childType == "virtual" {
			mod := e.nodeText(child)
			if !seen[mod] {
				modifiers = append(modifiers, mod)
				seen[mod] = true
			}
		}

		// Check for const, noexcept after parameter list
		if childType == "type_qualifier" {
			mod := e.nodeText(child)
			if mod == "const" && !seen[mod] {
				modifiers = append(modifiers, mod)
				seen[mod] = true
			}
		}
	}

	return modifiers
}

// walkNode performs a depth-first walk of the AST.
func (e *CppExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		e.walkNode(node.Child(int(i)), fn)
	}
}

// getFilePath returns the normalized file path.
func (e *CppExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *CppExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

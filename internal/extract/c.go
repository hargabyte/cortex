// Package extract provides C entity extraction from parsed AST trees.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// CExtractor extracts code entities from a parsed C AST.
type CExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewCExtractor creates an extractor for the given C parse result.
func NewCExtractor(result *parser.ParseResult) *CExtractor {
	return &CExtractor{
		result: result,
	}
}

// NewCExtractorWithBase creates an extractor with a base path for relative paths.
func NewCExtractorWithBase(result *parser.ParseResult, basePath string) *CExtractor {
	return &CExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the C AST.
// Returns functions, structs, unions, enums, typedefs, macros, and global variables.
func (e *CExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions
	funcs, err := e.ExtractFunctions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, funcs...)

	// Extract structs
	structs, err := e.ExtractStructs()
	if err != nil {
		return nil, err
	}
	entities = append(entities, structs...)

	// Extract unions
	unions, err := e.ExtractUnions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, unions...)

	// Extract enums
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

	// Extract typedefs
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
func (e *CExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract function definitions
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		entity := e.extractFunctionDefinition(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract function declarations (prototypes)
	declNodes := e.result.FindNodesByType("declaration")
	for _, node := range declNodes {
		if e.isFunctionDeclaration(node) {
			entity := e.extractFunctionDeclaration(node)
			if entity != nil {
				result = append(result, EntityWithNode{Entity: entity, Node: node})
			}
		}
	}

	// Extract structs
	structNodes := e.result.FindNodesByType("struct_specifier")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract unions
	unionNodes := e.result.FindNodesByType("union_specifier")
	for _, node := range unionNodes {
		entity := e.extractUnion(node)
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
func (e *CExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	// Extract function definitions
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		entity := e.extractFunctionDefinition(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Extract function declarations (prototypes in header files)
	declNodes := e.result.FindNodesByType("declaration")
	for _, node := range declNodes {
		if e.isFunctionDeclaration(node) {
			entity := e.extractFunctionDeclaration(node)
			if entity != nil {
				entities = append(entities, *entity)
			}
		}
	}

	return entities, nil
}

// ExtractStructs extracts all struct definitions.
func (e *CExtractor) ExtractStructs() ([]Entity, error) {
	var entities []Entity

	structNodes := e.result.FindNodesByType("struct_specifier")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractUnions extracts all union definitions.
func (e *CExtractor) ExtractUnions() ([]Entity, error) {
	var entities []Entity

	unionNodes := e.result.FindNodesByType("union_specifier")
	for _, node := range unionNodes {
		entity := e.extractUnion(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractEnums extracts all enum definitions.
func (e *CExtractor) ExtractEnums() ([]Entity, error) {
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

// ExtractTypedefs extracts all typedef declarations.
func (e *CExtractor) ExtractTypedefs() ([]Entity, error) {
	var entities []Entity

	typedefNodes := e.result.FindNodesByType("type_definition")
	for _, node := range typedefNodes {
		entity := e.extractTypedef(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractMacros extracts all macro definitions.
func (e *CExtractor) ExtractMacros() ([]Entity, error) {
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
func (e *CExtractor) ExtractGlobalVariables() ([]Entity, error) {
	var entities []Entity

	declNodes := e.result.FindNodesByType("declaration")
	for _, node := range declNodes {
		// Skip function declarations
		if e.isFunctionDeclaration(node) {
			continue
		}
		// Only process file-scope declarations
		if !e.isFileScopeDeclaration(node) {
			continue
		}
		vars := e.extractVariableDeclaration(node)
		entities = append(entities, vars...)
	}

	return entities, nil
}

// extractFunctionDefinition extracts a function entity from a function_definition node.
func (e *CExtractor) extractFunctionDefinition(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "function_definition" {
		return nil
	}

	// Get function declarator
	declarator := e.findFunctionDeclarator(node)
	if declarator == nil {
		return nil
	}

	// Get function name
	name := e.extractDeclaratorName(declarator)
	if name == "" {
		return nil
	}

	// Get parameters
	params := e.extractFunctionParameters(declarator)

	// Get return type
	returnType := e.extractReturnType(node)

	// Check for static (private visibility)
	visibility := e.determineCVisibility(node)

	// Get function body for hash
	bodyNode := findChildByType(node, "compound_statement")
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
		Returns:    []string{returnType},
		RawBody:    rawBody,
		Visibility: visibility,
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractFunctionDeclaration extracts a function declaration (prototype).
func (e *CExtractor) extractFunctionDeclaration(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "declaration" {
		return nil
	}

	// Get function declarator
	declarator := e.findFunctionDeclarator(node)
	if declarator == nil {
		return nil
	}

	// Get function name
	name := e.extractDeclaratorName(declarator)
	if name == "" {
		return nil
	}

	// Get parameters
	params := e.extractFunctionParameters(declarator)

	// Get return type
	returnType := e.extractDeclarationReturnType(node)

	// Check for static (private visibility)
	visibility := e.determineCVisibility(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    []string{returnType},
		Visibility: visibility,
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractStruct extracts a struct entity from a struct_specifier node.
func (e *CExtractor) extractStruct(node *sitter.Node) *Entity {
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
	// A reference like "struct Node *next" has no body
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		// This is a struct reference, not a definition
		return nil
	}

	// Get struct fields
	fields := e.extractStructFields(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Visibility: VisibilityPublic, // C structs are always public
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractUnion extracts a union entity from a union_specifier node.
func (e *CExtractor) extractUnion(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "union_specifier" {
		return nil
	}

	// Get union name
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		// Anonymous union
		return nil
	}
	name := e.nodeText(nameNode)

	// Check if this is a union definition (has field_declaration_list) or just a reference
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		// This is a union reference, not a definition
		return nil
	}

	// Get union fields
	fields := e.extractStructFields(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   UnionKind,
		Fields:     fields,
		Visibility: VisibilityPublic,
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractEnum extracts an enum entity from an enum_specifier node.
func (e *CExtractor) extractEnum(node *sitter.Node) *Entity {
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
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractTypedef extracts a typedef entity from a type_definition node.
func (e *CExtractor) extractTypedef(node *sitter.Node) *Entity {
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
		} else if childType == "struct_specifier" || childType == "union_specifier" || childType == "enum_specifier" {
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
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractMacro extracts a simple macro definition.
func (e *CExtractor) extractMacro(node *sitter.Node) *Entity {
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
		Visibility: VisibilityPublic, // Macros are always public
		Language:   "c",
	}

	entity.ComputeHashes()
	return entity
}

// extractFunctionMacro extracts a function-like macro definition.
func (e *CExtractor) extractFunctionMacro(node *sitter.Node) *Entity {
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
		Language:   "c",
		ValueType:  "macro", // Mark as macro function
	}

	entity.ComputeHashes()
	return entity
}

// extractVariableDeclaration extracts global variable declarations.
func (e *CExtractor) extractVariableDeclaration(node *sitter.Node) []Entity {
	var entities []Entity

	if node == nil || node.Type() != "declaration" {
		return nil
	}

	// Get type
	typeName := e.extractDeclarationReturnType(node)

	// Check visibility
	visibility := e.determineCVisibility(node)

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
				Language:   "c",
			}
			entity.ComputeHashes()
			entities = append(entities, entity)
		}
	}

	return entities
}

// Helper methods

// isFunctionDeclaration checks if a declaration node is a function declaration.
func (e *CExtractor) isFunctionDeclaration(node *sitter.Node) bool {
	return e.findFunctionDeclarator(node) != nil
}

// isFileScopeDeclaration checks if a declaration is at file scope.
func (e *CExtractor) isFileScopeDeclaration(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return true
	}
	// File-scope declarations have translation_unit as parent
	return parent.Type() == "translation_unit"
}

// findFunctionDeclarator finds the function_declarator node within a declaration.
func (e *CExtractor) findFunctionDeclarator(node *sitter.Node) *sitter.Node {
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

// extractDeclaratorName extracts the name from a declarator.
// For function_declarator, the name is a direct identifier child.
func (e *CExtractor) extractDeclaratorName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// For function_declarator, the identifier is a direct child (not in parameter_list)
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "identifier" {
			return e.nodeText(child)
		}

		// If we encounter a pointer_declarator, look inside it
		if childType == "pointer_declarator" {
			return e.extractDeclaratorName(child)
		}

		// If we encounter an array_declarator, look inside it
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
func (e *CExtractor) extractFunctionParameters(declarator *sitter.Node) []Param {
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
		if child.Type() == "parameter_declaration" {
			param := e.extractParameter(child)
			if param != nil {
				params = append(params, *param)
			}
		} else if child.Type() == "variadic_parameter" {
			params = append(params, Param{Name: "...", Type: "..."})
		}
	}

	return params
}

// extractParameter extracts a single parameter from a parameter_declaration.
func (e *CExtractor) extractParameter(node *sitter.Node) *Param {
	if node == nil {
		return nil
	}

	var name string
	var typeName string

	// Get type specifiers
	var typeSpecs []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		switch childType {
		case "primitive_type", "type_identifier", "sized_type_specifier":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "type_qualifier":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "identifier":
			name = e.nodeText(child)
		case "pointer_declarator":
			// Extract name from pointer declarator
			idNode := findChildByType(child, "identifier")
			if idNode != nil {
				name = e.nodeText(idNode)
			}
			typeName = "*"
		case "array_declarator":
			idNode := findChildByType(child, "identifier")
			if idNode != nil {
				name = e.nodeText(idNode)
			}
			typeName = "[]"
		}
	}

	if len(typeSpecs) > 0 {
		typeName = strings.Join(typeSpecs, " ") + typeName
	}

	return &Param{Name: name, Type: typeName}
}

// extractReturnType extracts the return type from a function_definition.
func (e *CExtractor) extractReturnType(node *sitter.Node) string {
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
		case "type_qualifier", "storage_class_specifier":
			// Include const, volatile, static, etc.
			text := e.nodeText(child)
			if text != "static" && text != "extern" && text != "inline" {
				typeSpecs = append(typeSpecs, text)
			}
		case "struct_specifier", "union_specifier", "enum_specifier":
			typeSpecs = append(typeSpecs, e.nodeText(child))
		case "pointer_declarator", "function_declarator":
			// Stop when we hit the declarator
			break
		}
	}

	if len(typeSpecs) == 0 {
		return "int" // Default return type in C89
	}

	return strings.Join(typeSpecs, " ")
}

// extractDeclarationReturnType extracts the return type from a declaration.
func (e *CExtractor) extractDeclarationReturnType(node *sitter.Node) string {
	return e.extractReturnType(node)
}

// extractStructFields extracts fields from a struct/union body.
func (e *CExtractor) extractStructFields(node *sitter.Node) []Field {
	var fields []Field

	// Find field_declaration_list
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		return nil
	}

	// Extract field declarations
	for i := uint32(0); i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(int(i))
		if child.Type() == "field_declaration" {
			extracted := e.extractFieldDecl(child)
			fields = append(fields, extracted...)
		}
	}

	return fields
}

// extractFieldDecl extracts fields from a field_declaration.
func (e *CExtractor) extractFieldDecl(node *sitter.Node) []Field {
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
		case "struct_specifier", "union_specifier", "enum_specifier":
			typeName = e.nodeText(child)
		case "field_identifier":
			fieldNames = append(fieldNames, e.nodeText(child))
		case "pointer_declarator":
			idNode := findChildByType(child, "field_identifier")
			if idNode != nil {
				fieldNames = append(fieldNames, e.nodeText(idNode))
				typeName = "*" + typeName
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
		fields = append(fields, Field{Name: name, Type: typeName})
	}

	return fields
}

// extractEnumValues extracts values from an enum body.
func (e *CExtractor) extractEnumValues(node *sitter.Node) []EnumValue {
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
func (e *CExtractor) extractEnumerator(node *sitter.Node) *EnumValue {
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
func (e *CExtractor) findTypedefDeclarators(node *sitter.Node) []string {
	var names []string

	e.walkNode(node, func(n *sitter.Node) bool {
		if n.Type() == "type_identifier" {
			// Skip if this is inside a struct/union/enum specifier
			parent := n.Parent()
			if parent != nil {
				pt := parent.Type()
				if pt == "struct_specifier" || pt == "union_specifier" || pt == "enum_specifier" {
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

// determineCVisibility determines visibility based on storage class.
func (e *CExtractor) determineCVisibility(node *sitter.Node) Visibility {
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

// walkNode performs a depth-first walk of the AST.
func (e *CExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
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
func (e *CExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *CExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// CSharpExtractor extracts code entities from a parsed C# AST.
type CSharpExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewCSharpExtractor creates an extractor for the given C# parse result.
func NewCSharpExtractor(result *parser.ParseResult) *CSharpExtractor {
	return &CSharpExtractor{
		result: result,
	}
}

// NewCSharpExtractorWithBase creates an extractor with a base path for relative paths.
func NewCSharpExtractorWithBase(result *parser.ParseResult, basePath string) *CSharpExtractor {
	return &CSharpExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the C# AST.
// Returns classes, interfaces, structs, records, enums, methods, properties, and usings.
func (e *CSharpExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract classes (regular, abstract, sealed, partial)
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

	// Extract structs
	structs, err := e.ExtractStructs()
	if err != nil {
		return nil, err
	}
	entities = append(entities, structs...)

	// Extract records
	records, err := e.ExtractRecords()
	if err != nil {
		return nil, err
	}
	entities = append(entities, records...)

	// Extract enums
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

	// Extract using directives
	usings, err := e.ExtractUsings()
	if err != nil {
		return nil, err
	}
	entities = append(entities, usings...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *CSharpExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract classes
	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract methods, constructors, properties, and fields from class
		methodsWithNodes := e.extractMethodsFromType(node)
		result = append(result, methodsWithNodes...)

		fieldsWithNodes := e.extractFieldsFromType(node)
		result = append(result, fieldsWithNodes...)

		propertiesWithNodes := e.extractPropertiesFromType(node)
		result = append(result, propertiesWithNodes...)
	}

	// Extract interfaces
	interfaceNodes := e.result.FindNodesByType("interface_declaration")
	for _, node := range interfaceNodes {
		entity := e.extractInterface(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract method signatures from interface
		methodsWithNodes := e.extractMethodsFromInterface(node)
		result = append(result, methodsWithNodes...)
	}

	// Extract structs
	structNodes := e.result.FindNodesByType("struct_declaration")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract methods, properties, and fields from struct
		methodsWithNodes := e.extractMethodsFromType(node)
		result = append(result, methodsWithNodes...)

		fieldsWithNodes := e.extractFieldsFromType(node)
		result = append(result, fieldsWithNodes...)

		propertiesWithNodes := e.extractPropertiesFromType(node)
		result = append(result, propertiesWithNodes...)
	}

	// Extract records
	recordNodes := e.result.FindNodesByType("record_declaration")
	for _, node := range recordNodes {
		entity := e.extractRecord(node)
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

	// Extract usings
	usingNodes := e.result.FindNodesByType("using_directive")
	for _, node := range usingNodes {
		entity := e.extractUsing(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// ExtractClasses extracts all class declarations.
func (e *CSharpExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods from class
		methods := e.extractMethodsFromTypeEntities(node)
		entities = append(entities, methods...)

		// Extract fields from class
		fields := e.extractFieldsFromTypeEntities(node)
		entities = append(entities, fields...)

		// Extract properties from class
		properties := e.extractPropertiesFromTypeEntities(node)
		entities = append(entities, properties...)
	}

	return entities, nil
}

// ExtractInterfaces extracts all interface declarations.
func (e *CSharpExtractor) ExtractInterfaces() ([]Entity, error) {
	var entities []Entity

	interfaceNodes := e.result.FindNodesByType("interface_declaration")
	for _, node := range interfaceNodes {
		entity := e.extractInterface(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract method signatures from interface
		methods := e.extractMethodsFromInterfaceEntities(node)
		entities = append(entities, methods...)
	}

	return entities, nil
}

// ExtractStructs extracts all struct declarations.
func (e *CSharpExtractor) ExtractStructs() ([]Entity, error) {
	var entities []Entity

	structNodes := e.result.FindNodesByType("struct_declaration")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods from struct
		methods := e.extractMethodsFromTypeEntities(node)
		entities = append(entities, methods...)

		// Extract fields from struct
		fields := e.extractFieldsFromTypeEntities(node)
		entities = append(entities, fields...)

		// Extract properties from struct
		properties := e.extractPropertiesFromTypeEntities(node)
		entities = append(entities, properties...)
	}

	return entities, nil
}

// ExtractRecords extracts all record declarations.
func (e *CSharpExtractor) ExtractRecords() ([]Entity, error) {
	var entities []Entity

	recordNodes := e.result.FindNodesByType("record_declaration")
	for _, node := range recordNodes {
		entity := e.extractRecord(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractEnums extracts all enum declarations.
func (e *CSharpExtractor) ExtractEnums() ([]Entity, error) {
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

// ExtractUsings extracts all using directives.
func (e *CSharpExtractor) ExtractUsings() ([]Entity, error) {
	var entities []Entity

	usingNodes := e.result.FindNodesByType("using_directive")
	for _, node := range usingNodes {
		entity := e.extractUsing(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractClass extracts a class declaration from its AST node.
func (e *CSharpExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_declaration" {
		return nil
	}

	// Get class name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers (abstract, sealed, public, private, protected, internal, partial)
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)

	// Check for base class and implemented interfaces
	baseClass := ""
	var implements []string
	baseListNode := findChildByType(node, "base_list")
	if baseListNode != nil {
		baseClass, implements = e.extractBaseClass(baseListNode)
	}

	// Extract type parameters (generics)
	typeParams := e.extractTypeParameters(node)

	// Extract fields as struct fields
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	// Build kind description
	typeKind := StructKind
	kindSuffix := ""
	if containsStr(modifiers, "abstract") {
		kindSuffix = " (abstract)"
	} else if containsStr(modifiers, "sealed") {
		kindSuffix = " (sealed)"
	} else if containsStr(modifiers, "static") {
		kindSuffix = " (static)"
	}

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name + typeParams,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   typeKind,
		Fields:     fields,
		Visibility: visibility,
		Implements: implements,
		ValueType:  "class" + kindSuffix,
	}

	// Store base class in receiver field for tracking inheritance
	if baseClass != "" {
		entity.Receiver = baseClass
	}

	entity.ComputeHashes()
	return entity
}

// extractInterface extracts an interface declaration from its AST node.
func (e *CSharpExtractor) extractInterface(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "interface_declaration" {
		return nil
	}

	// Get interface name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)

	// Check for extended interfaces
	var extends []string
	baseListNode := findChildByType(node, "base_list")
	if baseListNode != nil {
		_, extends = e.extractBaseClass(baseListNode)
	}

	// Extract type parameters (generics)
	typeParams := e.extractTypeParameters(node)

	// Extract method signatures as fields
	methods := e.extractInterfaceMethods(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name + typeParams,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   InterfaceKind,
		Fields:     methods,
		Visibility: visibility,
		Implements: extends, // For interfaces, "extends" is stored in Implements
		ValueType:  "interface",
	}

	entity.ComputeHashes()
	return entity
}

// extractStruct extracts a struct declaration from its AST node.
func (e *CSharpExtractor) extractStruct(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "struct_declaration" {
		return nil
	}

	// Get struct name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)

	// Check for implemented interfaces
	var implements []string
	baseListNode := findChildByType(node, "base_list")
	if baseListNode != nil {
		_, implements = e.extractBaseClass(baseListNode)
	}

	// Extract type parameters (generics)
	typeParams := e.extractTypeParameters(node)

	// Extract fields
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	kindSuffix := ""
	if containsStr(modifiers, "readonly") {
		kindSuffix = " (readonly)"
	}

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name + typeParams,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Visibility: visibility,
		Implements: implements,
		ValueType:  "struct" + kindSuffix,
	}

	entity.ComputeHashes()
	return entity
}

// extractRecord extracts a record declaration from its AST node.
func (e *CSharpExtractor) extractRecord(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "record_declaration" {
		return nil
	}

	// Get record name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)

	// Check for base class and implemented interfaces
	baseClass := ""
	var implements []string
	baseListNode := findChildByType(node, "base_list")
	if baseListNode != nil {
		baseClass, implements = e.extractBaseClass(baseListNode)
	}

	// Extract type parameters (generics)
	typeParams := e.extractTypeParameters(node)

	// Extract primary constructor parameters as fields
	fields := e.extractRecordParameters(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name + typeParams,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Visibility: visibility,
		Implements: implements,
		ValueType:  "record",
	}

	// Store base class in receiver field for tracking inheritance
	if baseClass != "" {
		entity.Receiver = baseClass
	}

	entity.ComputeHashes()
	return entity
}

// extractEnum extracts an enum declaration from its AST node.
func (e *CSharpExtractor) extractEnum(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "enum_declaration" {
		return nil
	}

	// Get enum name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)

	// Extract enum members
	enumValues := e.extractEnumMembers(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       EnumEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		EnumValues: enumValues,
		Visibility: visibility,
		ValueType:  "enum",
	}

	entity.ComputeHashes()
	return entity
}

// extractUsing extracts a using directive from its AST node.
func (e *CSharpExtractor) extractUsing(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "using_directive" {
		return nil
	}

	// Get the namespace being imported
	var namespacePath string
	var isStatic bool
	var alias string
	var foundEquals bool

	// Track if we see an equals sign (alias syntax: using Alias = Namespace;)
	var identifiers []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "static" {
			isStatic = true
		} else if childType == "=" {
			foundEquals = true
		} else if childType == "qualified_name" {
			namespacePath = e.nodeText(child)
		} else if childType == "identifier" || childType == "identifier_name" {
			identifiers = append(identifiers, e.nodeText(child))
		} else if childType == "name_equals" {
			// Handle alias: using Alias = Namespace;
			aliasIdent := findChildByType(child, "identifier")
			if aliasIdent == nil {
				aliasIdent = findChildByType(child, "identifier_name")
			}
			if aliasIdent != nil {
				alias = e.nodeText(aliasIdent)
			}
			foundEquals = true
		}
	}

	// Handle alias syntax where identifier appears before =
	// using MyAlias = System.Text.StringBuilder;
	if foundEquals && len(identifiers) > 0 && namespacePath != "" {
		alias = identifiers[0]
	} else if namespacePath == "" && len(identifiers) > 0 {
		// Simple using without qualified name
		namespacePath = identifiers[0]
	}

	if namespacePath == "" {
		return nil
	}

	// Extract the simple name (last part of namespace)
	name := extractCSharpNamespaceName(namespacePath)

	startLine, _ := getLineRange(node)

	importAlias := alias
	if isStatic && alias == "" {
		importAlias = "static"
	}

	return &Entity{
		Kind:        ImportEntity,
		Name:        name,
		File:        e.getFilePath(),
		StartLine:   startLine,
		EndLine:     startLine,
		ImportPath:  namespacePath,
		ImportAlias: importAlias,
	}
}

// extractMethod extracts a method declaration from its AST node.
func (e *CSharpExtractor) extractMethod(node *sitter.Node, typeName string) *Entity {
	if node == nil || node.Type() != "method_declaration" {
		return nil
	}

	// Get method name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)
	isStatic := containsStr(modifiers, "static")
	isAsync := containsStr(modifiers, "async")

	// Get return type
	returnType := ""
	typeNode := findChildByFieldName(node, "type")
	if typeNode == nil {
		typeNode = findChildByFieldName(node, "returns")
	}
	if typeNode != nil {
		returnType = abbreviateCSharpType(e.nodeText(typeNode))
	}

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractCSharpParameters(paramsNode)

	// Get method body for hash computation
	bodyNode := findChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	// Extract type parameters (generics on method)
	typeParams := e.extractTypeParameters(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       MethodEntity,
		Name:       name + typeParams,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    []string{returnType},
		RawBody:    rawBody,
		Visibility: visibility,
		IsAsync:    isAsync,
	}

	// For instance methods, store the type name as receiver
	// For static methods, mark with (static)
	if typeName != "" {
		if isStatic {
			entity.Receiver = typeName + " (static)"
		} else {
			entity.Receiver = typeName
		}
	}

	entity.ComputeHashes()
	return entity
}

// extractConstructor extracts a constructor declaration from its AST node.
func (e *CSharpExtractor) extractConstructor(node *sitter.Node, typeName string) *Entity {
	if node == nil || node.Type() != "constructor_declaration" {
		return nil
	}

	// Get constructor name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)
	isStatic := containsStr(modifiers, "static")

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractCSharpParameters(paramsNode)

	// Get constructor body for hash computation
	bodyNode := findChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getLineRange(node)

	receiverSuffix := " (constructor)"
	if isStatic {
		receiverSuffix = " (static constructor)"
	}

	entity := &Entity{
		Kind:       MethodEntity, // Constructors are treated as methods
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    []string{typeName}, // Constructor returns the type
		Receiver:   typeName + receiverSuffix,
		RawBody:    rawBody,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractProperty extracts a property declaration from its AST node.
func (e *CSharpExtractor) extractProperty(node *sitter.Node, typeName string) *Entity {
	if node == nil || node.Type() != "property_declaration" {
		return nil
	}

	// Get property name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)
	isStatic := containsStr(modifiers, "static")

	// Get property type
	propertyType := ""
	typeNode := findChildByFieldName(node, "type")
	if typeNode != nil {
		propertyType = abbreviateCSharpType(e.nodeText(typeNode))
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       VarEntity, // Properties are treated as variables
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		ValueType:  propertyType,
		Visibility: visibility,
	}

	// Store type context and static info in receiver
	if typeName != "" {
		if isStatic {
			entity.Receiver = typeName + " (static property)"
		} else {
			entity.Receiver = typeName + " (property)"
		}
	}

	entity.ComputeHashes()
	return entity
}

// extractField extracts a field declaration from its AST node.
func (e *CSharpExtractor) extractField(node *sitter.Node, typeName string) []Entity {
	if node == nil || node.Type() != "field_declaration" {
		return nil
	}

	var entities []Entity

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineCSharpVisibility(modifiers)
	isStatic := containsStr(modifiers, "static")
	isReadonly := containsStr(modifiers, "readonly")
	isConst := containsStr(modifiers, "const")

	// Get type
	typeNode := findChildByFieldName(node, "type")
	fieldTypeName := ""
	if typeNode != nil {
		fieldTypeName = abbreviateCSharpType(e.nodeText(typeNode))
	}

	// Get variable declarators (can be multiple: int x, y, z;)
	declarationNode := findChildByType(node, "variable_declaration")
	if declarationNode != nil {
		typeNode = findChildByFieldName(declarationNode, "type")
		if typeNode != nil {
			fieldTypeName = abbreviateCSharpType(e.nodeText(typeNode))
		}
	}

	// Find all variable declarators
	var declarators []*sitter.Node
	if declarationNode != nil {
		declarators = findChildrenByType(declarationNode, "variable_declarator")
	} else {
		declarators = findChildrenByType(node, "variable_declarator")
	}

	startLine, _ := getLineRange(node)

	for _, decl := range declarators {
		nameNode := findChildByFieldName(decl, "name")
		if nameNode == nil {
			// Try to find identifier child
			nameNode = findChildByType(decl, "identifier")
		}
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get value if present
		value := ""
		valueNode := findChildByFieldName(decl, "value")
		if valueNode == nil {
			valueNode = findChildByType(decl, "equals_value_clause")
		}
		if valueNode != nil {
			value = e.nodeText(valueNode)
			if len(value) > 50 {
				value = value[:47] + "..."
			}
		}

		// Determine entity kind based on modifiers
		kind := VarEntity
		if isConst || isReadonly {
			kind = ConstEntity
		}

		entity := Entity{
			Kind:       kind,
			Name:       name,
			File:       e.getFilePath(),
			StartLine:  startLine,
			EndLine:    startLine,
			ValueType:  fieldTypeName,
			Value:      value,
			Visibility: visibility,
		}

		// Store type context and static info in receiver
		if typeName != "" {
			if isStatic {
				entity.Receiver = typeName + " (static)"
			} else {
				entity.Receiver = typeName
			}
		}

		entity.ComputeHashes()
		entities = append(entities, entity)
	}

	return entities
}

// extractMethodsFromType extracts all methods and constructors from a type as EntityWithNode.
func (e *CSharpExtractor) extractMethodsFromType(typeNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get type name
	nameNode := findChildByFieldName(typeNode, "name")
	typeName := ""
	if nameNode != nil {
		typeName = e.nodeText(nameNode)
	}

	// Find declaration list (body)
	bodyNode := findChildByFieldName(typeNode, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(typeNode, "declaration_list")
	}
	if bodyNode == nil {
		return result
	}

	// Extract methods
	methodNodes := findChildrenByType(bodyNode, "method_declaration")
	for _, node := range methodNodes {
		entity := e.extractMethod(node, typeName)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract constructors
	constructorNodes := findChildrenByType(bodyNode, "constructor_declaration")
	for _, node := range constructorNodes {
		entity := e.extractConstructor(node, typeName)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractMethodsFromTypeEntities extracts all methods from a type as Entity slice.
func (e *CSharpExtractor) extractMethodsFromTypeEntities(typeNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromType(typeNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractFieldsFromType extracts all fields from a type as EntityWithNode.
func (e *CSharpExtractor) extractFieldsFromType(typeNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get type name
	nameNode := findChildByFieldName(typeNode, "name")
	typeName := ""
	if nameNode != nil {
		typeName = e.nodeText(nameNode)
	}

	// Find declaration list (body)
	bodyNode := findChildByFieldName(typeNode, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(typeNode, "declaration_list")
	}
	if bodyNode == nil {
		return result
	}

	// Extract fields
	fieldNodes := findChildrenByType(bodyNode, "field_declaration")
	for _, node := range fieldNodes {
		entities := e.extractField(node, typeName)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result
}

// extractFieldsFromTypeEntities extracts all fields from a type as Entity slice.
func (e *CSharpExtractor) extractFieldsFromTypeEntities(typeNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractFieldsFromType(typeNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractPropertiesFromType extracts all properties from a type as EntityWithNode.
func (e *CSharpExtractor) extractPropertiesFromType(typeNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get type name
	nameNode := findChildByFieldName(typeNode, "name")
	typeName := ""
	if nameNode != nil {
		typeName = e.nodeText(nameNode)
	}

	// Find declaration list (body)
	bodyNode := findChildByFieldName(typeNode, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(typeNode, "declaration_list")
	}
	if bodyNode == nil {
		return result
	}

	// Extract properties
	propertyNodes := findChildrenByType(bodyNode, "property_declaration")
	for _, node := range propertyNodes {
		entity := e.extractProperty(node, typeName)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractPropertiesFromTypeEntities extracts all properties from a type as Entity slice.
func (e *CSharpExtractor) extractPropertiesFromTypeEntities(typeNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractPropertiesFromType(typeNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractMethodsFromInterface extracts method signatures from an interface as EntityWithNode.
func (e *CSharpExtractor) extractMethodsFromInterface(interfaceNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get interface name
	nameNode := findChildByFieldName(interfaceNode, "name")
	interfaceName := ""
	if nameNode != nil {
		interfaceName = e.nodeText(nameNode)
	}

	// Find declaration list (body)
	bodyNode := findChildByFieldName(interfaceNode, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(interfaceNode, "declaration_list")
	}
	if bodyNode == nil {
		return result
	}

	// Extract method declarations
	methodNodes := findChildrenByType(bodyNode, "method_declaration")
	for _, node := range methodNodes {
		entity := e.extractMethod(node, interfaceName)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractMethodsFromInterfaceEntities extracts method signatures from an interface as Entity slice.
func (e *CSharpExtractor) extractMethodsFromInterfaceEntities(interfaceNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromInterface(interfaceNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// Helper functions

// extractModifiers extracts all modifiers from a node.
func (e *CSharpExtractor) extractModifiers(node *sitter.Node) []string {
	var modifiers []string

	// Modifiers can be direct children or in a modifiers node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "modifier" {
			modifiers = append(modifiers, e.nodeText(child))
		} else if isCSharpModifier(childType) {
			modifiers = append(modifiers, childType)
		} else if childType == "attribute_list" {
			// Capture attribute name for context
			attrName := e.extractAttributeName(child)
			if attrName != "" {
				modifiers = append(modifiers, "["+attrName+"]")
			}
		}
	}

	return modifiers
}

// extractAttributeName extracts the name from an attribute list node.
func (e *CSharpExtractor) extractAttributeName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Look for attribute nodes
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "attribute" {
			nameNode := findChildByFieldName(child, "name")
			if nameNode != nil {
				return e.nodeText(nameNode)
			}
			// Try to find identifier child
			for j := uint32(0); j < child.ChildCount(); j++ {
				attrChild := child.Child(int(j))
				if attrChild.Type() == "identifier" || attrChild.Type() == "identifier_name" {
					return e.nodeText(attrChild)
				}
			}
		}
	}

	return ""
}

// extractTypeParameters extracts generic type parameters like <T> or <K, V>.
func (e *CSharpExtractor) extractTypeParameters(node *sitter.Node) string {
	typeParamsNode := findChildByFieldName(node, "type_parameters")
	if typeParamsNode == nil {
		typeParamsNode = findChildByType(node, "type_parameter_list")
	}
	if typeParamsNode == nil {
		return ""
	}
	return e.nodeText(typeParamsNode)
}

// extractBaseClass extracts the base class and implemented interfaces from a base_list node.
func (e *CSharpExtractor) extractBaseClass(node *sitter.Node) (string, []string) {
	var baseTypes []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		// Skip punctuation
		if childType == ":" || childType == "," {
			continue
		}

		if childType == "identifier" || childType == "identifier_name" ||
			childType == "generic_name" || childType == "qualified_name" {
			typeName := e.nodeText(child)
			if typeName != "" {
				baseTypes = append(baseTypes, typeName)
			}
		} else if childType == "simple_base_type" || childType == "base_type" {
			typeName := e.nodeText(child)
			if typeName != "" {
				baseTypes = append(baseTypes, typeName)
			}
		}
	}

	// First type is the base class (if not an interface name starting with I)
	// Rest are interfaces
	if len(baseTypes) == 0 {
		return "", nil
	}

	baseClass := baseTypes[0]
	interfaces := baseTypes[1:]

	return baseClass, interfaces
}

// extractClassFields extracts fields from a class body as Field structs.
func (e *CSharpExtractor) extractClassFields(node *sitter.Node) []Field {
	var fields []Field

	bodyNode := findChildByFieldName(node, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(node, "declaration_list")
	}
	if bodyNode == nil {
		return fields
	}

	fieldDecls := findChildrenByType(bodyNode, "field_declaration")
	for _, decl := range fieldDecls {
		// Get type
		var fieldTypeName string
		typeNode := findChildByFieldName(decl, "type")
		if typeNode != nil {
			fieldTypeName = abbreviateCSharpType(e.nodeText(typeNode))
		}

		// Check variable_declaration for type
		varDeclNode := findChildByType(decl, "variable_declaration")
		if varDeclNode != nil {
			typeNode = findChildByFieldName(varDeclNode, "type")
			if typeNode != nil {
				fieldTypeName = abbreviateCSharpType(e.nodeText(typeNode))
			}
		}

		// Get variable names
		var declarators []*sitter.Node
		if varDeclNode != nil {
			declarators = findChildrenByType(varDeclNode, "variable_declarator")
		} else {
			declarators = findChildrenByType(decl, "variable_declarator")
		}

		for _, d := range declarators {
			nameNode := findChildByFieldName(d, "name")
			if nameNode == nil {
				nameNode = findChildByType(d, "identifier")
			}
			if nameNode != nil {
				name := e.nodeText(nameNode)
				fields = append(fields, Field{Name: name, Type: fieldTypeName})
			}
		}
	}

	return fields
}

// extractRecordParameters extracts primary constructor parameters from a record.
func (e *CSharpExtractor) extractRecordParameters(node *sitter.Node) []Field {
	var fields []Field

	paramsNode := findChildByFieldName(node, "parameters")
	if paramsNode == nil {
		paramsNode = findChildByType(node, "parameter_list")
	}
	if paramsNode == nil {
		return fields
	}

	paramNodes := findChildrenByType(paramsNode, "parameter")
	for _, param := range paramNodes {
		// Get type
		typeNode := findChildByFieldName(param, "type")
		typeName := ""
		if typeNode != nil {
			typeName = abbreviateCSharpType(e.nodeText(typeNode))
		}

		// Get name
		nameNode := findChildByFieldName(param, "name")
		if nameNode == nil {
			nameNode = findChildByType(param, "identifier")
		}
		name := ""
		if nameNode != nil {
			name = e.nodeText(nameNode)
		}

		if name != "" {
			fields = append(fields, Field{Name: name, Type: typeName})
		}
	}

	return fields
}

// extractInterfaceMethods extracts method signatures from an interface as Field structs.
func (e *CSharpExtractor) extractInterfaceMethods(node *sitter.Node) []Field {
	var methods []Field

	bodyNode := findChildByFieldName(node, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(node, "declaration_list")
	}
	if bodyNode == nil {
		return methods
	}

	methodDecls := findChildrenByType(bodyNode, "method_declaration")
	for _, decl := range methodDecls {
		// Get method name
		nameNode := findChildByFieldName(decl, "name")
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get return type
		returnType := "void"
		typeNode := findChildByFieldName(decl, "type")
		if typeNode == nil {
			typeNode = findChildByFieldName(decl, "returns")
		}
		if typeNode != nil {
			returnType = abbreviateCSharpType(e.nodeText(typeNode))
		}

		// Get parameters
		paramsNode := findChildByFieldName(decl, "parameters")
		params := e.extractCSharpParameters(paramsNode)

		// Build method signature
		sig := formatCSharpMethodSignature(params, returnType)

		methods = append(methods, Field{Name: name, Type: sig})
	}

	return methods
}

// extractEnumMembers extracts enum member values.
func (e *CSharpExtractor) extractEnumMembers(node *sitter.Node) []EnumValue {
	var values []EnumValue

	bodyNode := findChildByFieldName(node, "body")
	if bodyNode == nil {
		bodyNode = findChildByType(node, "enum_member_declaration_list")
	}
	if bodyNode == nil {
		return values
	}

	// Find enum_member_declaration nodes
	members := findChildrenByType(bodyNode, "enum_member_declaration")
	for i, m := range members {
		nameNode := findChildByFieldName(m, "name")
		if nameNode == nil {
			nameNode = findChildByType(m, "identifier")
		}
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get explicit value if present
		value := ""
		valueNode := findChildByFieldName(m, "value")
		if valueNode == nil {
			valueNode = findChildByType(m, "equals_value_clause")
		}
		if valueNode != nil {
			value = e.nodeText(valueNode)
		} else {
			// Use ordinal if no explicit value
			value = itoa(i)
		}

		values = append(values, EnumValue{
			Name:  name,
			Value: value,
		})
	}

	return values
}

// extractCSharpParameters extracts parameters from a parameter_list node.
func (e *CSharpExtractor) extractCSharpParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param

	// Find parameter nodes
	paramDecls := findChildrenByType(node, "parameter")
	for _, decl := range paramDecls {
		// Get type
		typeNode := findChildByFieldName(decl, "type")
		typeName := ""
		if typeNode != nil {
			typeName = abbreviateCSharpType(e.nodeText(typeNode))
		}

		// Get name
		nameNode := findChildByFieldName(decl, "name")
		if nameNode == nil {
			nameNode = findChildByType(decl, "identifier")
		}
		name := ""
		if nameNode != nil {
			name = e.nodeText(nameNode)
		}

		// Check for params keyword (varargs)
		modifiers := e.extractModifiers(decl)
		if containsStr(modifiers, "params") {
			typeName = "params " + typeName
		}

		// Check for ref/out/in keywords
		for _, mod := range modifiers {
			if mod == "ref" || mod == "out" || mod == "in" {
				typeName = mod + " " + typeName
				break
			}
		}

		params = append(params, Param{Name: name, Type: typeName})
	}

	return params
}

// getFilePath returns the normalized file path.
func (e *CSharpExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *CSharpExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// isCSharpModifier checks if a node type is a C# modifier keyword.
func isCSharpModifier(nodeType string) bool {
	modifiers := map[string]bool{
		"public":    true,
		"private":   true,
		"protected": true,
		"internal":  true,
		"static":    true,
		"readonly":  true,
		"const":     true,
		"abstract":  true,
		"sealed":    true,
		"virtual":   true,
		"override":  true,
		"new":       true,
		"partial":   true,
		"async":     true,
		"extern":    true,
		"volatile":  true,
		"unsafe":    true,
		"ref":       true,
		"out":       true,
		"in":        true,
		"params":    true,
	}
	return modifiers[nodeType]
}

// determineCSharpVisibility determines visibility from C# modifiers.
func determineCSharpVisibility(modifiers []string) Visibility {
	for _, m := range modifiers {
		switch m {
		case "public":
			return VisibilityPublic
		case "private", "protected", "internal":
			return VisibilityPrivate
		}
	}
	// Default (no modifier) is private in C#
	return VisibilityPrivate
}

// abbreviateCSharpType converts C# types to abbreviated form.
func abbreviateCSharpType(csharpType string) string {
	csharpType = strings.TrimSpace(csharpType)

	// Handle nullable types and generics
	return csharpType
}

// formatCSharpMethodSignature formats a method signature for interface methods.
func formatCSharpMethodSignature(params []Param, returnType string) string {
	var sb strings.Builder

	sb.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')

	if returnType != "" && returnType != "void" {
		sb.WriteString(" -> ")
		sb.WriteString(returnType)
	}

	return sb.String()
}

// extractCSharpNamespaceName extracts the simple name from a namespace path.
func extractCSharpNamespaceName(namespacePath string) string {
	parts := strings.Split(namespacePath, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return namespacePath
}

// containsStr checks if a slice contains a string.
func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

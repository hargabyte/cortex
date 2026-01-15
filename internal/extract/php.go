package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// PHPExtractor extracts code entities from a parsed PHP AST.
type PHPExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewPHPExtractor creates an extractor for the given PHP parse result.
func NewPHPExtractor(result *parser.ParseResult) *PHPExtractor {
	return &PHPExtractor{
		result: result,
	}
}

// NewPHPExtractorWithBase creates an extractor with a base path for relative paths.
func NewPHPExtractorWithBase(result *parser.ParseResult, basePath string) *PHPExtractor {
	return &PHPExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the PHP AST.
// Returns classes, interfaces, traits, enums, functions, methods, properties, and constants.
func (e *PHPExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions
	functions, err := e.ExtractFunctions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, functions...)

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

	// Extract traits
	traits, err := e.ExtractTraits()
	if err != nil {
		return nil, err
	}
	entities = append(entities, traits...)

	// Extract enums (PHP 8.1+)
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *PHPExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract functions
	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		entity := e.extractFunction(node)
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

		// Extract methods from class
		methodsWithNodes := e.extractMethodsFromClass(node)
		result = append(result, methodsWithNodes...)

		// Extract properties from class
		propsWithNodes := e.extractPropertiesFromClass(node)
		result = append(result, propsWithNodes...)

		// Extract constants from class
		constsWithNodes := e.extractConstantsFromClass(node)
		result = append(result, constsWithNodes...)
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

	// Extract traits
	traitNodes := e.result.FindNodesByType("trait_declaration")
	for _, node := range traitNodes {
		entity := e.extractTrait(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract methods from trait
		methodsWithNodes := e.extractMethodsFromTrait(node)
		result = append(result, methodsWithNodes...)
	}

	// Extract enums (PHP 8.1+)
	enumNodes := e.result.FindNodesByType("enum_declaration")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// ExtractFunctions extracts all function declarations.
func (e *PHPExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	funcNodes := e.result.FindNodesByType("function_definition")
	for _, node := range funcNodes {
		// Skip methods (functions inside classes/traits/interfaces)
		if e.isMethodContext(node) {
			continue
		}
		entity := e.extractFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractClasses extracts all class declarations.
func (e *PHPExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods from class
		methods := e.extractMethodsFromClassEntities(node)
		entities = append(entities, methods...)

		// Extract properties from class
		props := e.extractPropertiesFromClassEntities(node)
		entities = append(entities, props...)

		// Extract constants from class
		consts := e.extractConstantsFromClassEntities(node)
		entities = append(entities, consts...)
	}

	return entities, nil
}

// ExtractInterfaces extracts all interface declarations.
func (e *PHPExtractor) ExtractInterfaces() ([]Entity, error) {
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

// ExtractTraits extracts all trait declarations.
func (e *PHPExtractor) ExtractTraits() ([]Entity, error) {
	var entities []Entity

	traitNodes := e.result.FindNodesByType("trait_declaration")
	for _, node := range traitNodes {
		entity := e.extractTrait(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods from trait
		methods := e.extractMethodsFromTraitEntities(node)
		entities = append(entities, methods...)
	}

	return entities, nil
}

// ExtractEnums extracts all enum declarations (PHP 8.1+).
func (e *PHPExtractor) ExtractEnums() ([]Entity, error) {
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

// extractFunction extracts a function definition from its AST node.
func (e *PHPExtractor) extractFunction(node *sitter.Node) *Entity {
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
	params := e.extractPHPParameters(paramsNode)

	// Get return type
	returnType := ""
	returnTypeNode := findChildByFieldName(node, "return_type")
	if returnTypeNode != nil {
		returnType = e.extractPHPType(returnTypeNode)
	}

	// Get function body for hash computation
	bodyNode := findChildByFieldName(node, "body")
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
		RawBody:    rawBody,
		Visibility: VisibilityPublic, // PHP functions are public by default
	}

	if returnType != "" {
		entity.Returns = []string{returnType}
	}

	entity.ComputeHashes()
	return entity
}

// extractClass extracts a class declaration from its AST node.
func (e *PHPExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_declaration" {
		return nil
	}

	// Get class name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers (abstract, final)
	modifiers := e.extractPHPModifiers(node)
	visibility := determinePHPVisibility(modifiers)

	// Check for parent class (extends)
	superclass := ""
	baseClauseNode := findChildByType(node, "base_clause")
	if baseClauseNode != nil {
		// Find the name node inside base_clause
		for i := uint32(0); i < baseClauseNode.ChildCount(); i++ {
			child := baseClauseNode.Child(int(i))
			if child.Type() == "name" || child.Type() == "qualified_name" {
				superclass = e.nodeText(child)
				break
			}
		}
	}

	// Check for implemented interfaces
	var implements []string
	classInterfaceClause := findChildByType(node, "class_interface_clause")
	if classInterfaceClause != nil {
		implements = e.extractInterfaceList(classInterfaceClause)
	}

	// Check for used traits
	usedTraits := e.extractUsedTraits(node)

	// Extract properties as struct fields
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	// Build kind description
	typeKind := StructKind
	kindSuffix := ""
	if containsString(modifiers, "abstract") {
		kindSuffix = " (abstract)"
	} else if containsString(modifiers, "final") {
		kindSuffix = " (final)"
	} else if containsString(modifiers, "readonly") {
		kindSuffix = " (readonly)"
	}

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   typeKind,
		Fields:     fields,
		Visibility: visibility,
		Implements: implements,
		ValueType:  "class" + kindSuffix,
	}

	// Store superclass in receiver field for tracking inheritance
	if superclass != "" {
		entity.Receiver = superclass
	}

	// Store used traits as decorators
	if len(usedTraits) > 0 {
		entity.Decorators = usedTraits
	}

	entity.ComputeHashes()
	return entity
}

// extractInterface extracts an interface declaration from its AST node.
func (e *PHPExtractor) extractInterface(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "interface_declaration" {
		return nil
	}

	// Get interface name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for extended interfaces
	var extends []string
	baseClauseNode := findChildByType(node, "base_clause")
	if baseClauseNode != nil {
		extends = e.extractInterfaceList(baseClauseNode)
	}

	// Extract method signatures as fields
	methods := e.extractInterfaceMethods(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   InterfaceKind,
		Fields:     methods,
		Visibility: VisibilityPublic,
		Implements: extends, // For interfaces, "extends" is stored in Implements
		ValueType:  "interface",
	}

	entity.ComputeHashes()
	return entity
}

// extractTrait extracts a trait declaration from its AST node.
func (e *PHPExtractor) extractTrait(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "trait_declaration" {
		return nil
	}

	// Get trait name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Extract methods as fields
	methods := e.extractTraitMethods(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind, // Traits are similar to structs
		Fields:     methods,
		Visibility: VisibilityPublic,
		ValueType:  "trait",
	}

	entity.ComputeHashes()
	return entity
}

// extractEnum extracts an enum declaration from its AST node (PHP 8.1+).
func (e *PHPExtractor) extractEnum(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "enum_declaration" {
		return nil
	}

	// Get enum name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get backing type if present (int or string)
	backingType := ""
	colonTypeNode := findChildByType(node, ":")
	if colonTypeNode != nil {
		// Find type after colon
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "primitive_type" || child.Type() == "name" {
				backingType = e.nodeText(child)
				break
			}
		}
	}

	// Extract enum cases
	enumValues := e.extractEnumCases(node)

	// Check for implemented interfaces
	var implements []string
	classInterfaceClause := findChildByType(node, "class_interface_clause")
	if classInterfaceClause != nil {
		implements = e.extractInterfaceList(classInterfaceClause)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       EnumEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		EnumValues: enumValues,
		Visibility: VisibilityPublic,
		Implements: implements,
		ValueType:  backingType,
	}

	entity.ComputeHashes()
	return entity
}

// extractMethod extracts a method declaration from its AST node.
func (e *PHPExtractor) extractMethod(node *sitter.Node, className string) *Entity {
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
	modifiers := e.extractPHPModifiers(node)
	visibility := determinePHPVisibility(modifiers)
	isStatic := containsString(modifiers, "static")
	isAbstract := containsString(modifiers, "abstract")

	// Get return type
	returnType := ""
	returnTypeNode := findChildByFieldName(node, "return_type")
	if returnTypeNode != nil {
		returnType = e.extractPHPType(returnTypeNode)
	}

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractPHPParameters(paramsNode)

	// Get method body for hash computation
	bodyNode := findChildByFieldName(node, "body")
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
		RawBody:    rawBody,
		Visibility: visibility,
	}

	if returnType != "" {
		entity.Returns = []string{returnType}
	}

	// For instance methods, store the class name as receiver
	// For static methods, mark with (static)
	if className != "" {
		if isStatic {
			entity.Receiver = className + " (static)"
		} else if isAbstract {
			entity.Receiver = className + " (abstract)"
		} else {
			entity.Receiver = className
		}
	}

	// Handle magic methods
	if strings.HasPrefix(name, "__") {
		entity.ValueType = "magic"
	}

	entity.ComputeHashes()
	return entity
}

// extractProperty extracts a property declaration from its AST node.
func (e *PHPExtractor) extractProperty(node *sitter.Node, className string) []Entity {
	if node == nil || node.Type() != "property_declaration" {
		return nil
	}

	var entities []Entity

	// Check for modifiers
	modifiers := e.extractPHPModifiers(node)
	visibility := determinePHPVisibility(modifiers)
	isStatic := containsString(modifiers, "static")
	isReadonly := containsString(modifiers, "readonly")

	// Get type
	typeName := ""
	typeNode := findChildByType(node, "property_type") // PHP 7.4+ typed properties
	if typeNode == nil {
		typeNode = findChildByType(node, "type")
	}
	if typeNode != nil {
		typeName = e.extractPHPType(typeNode)
	}

	// Get property elements
	propElements := findChildrenByType(node, "property_element")

	startLine, _ := getLineRange(node)

	for _, elem := range propElements {
		// Get variable name (includes $)
		varNode := findChildByType(elem, "variable_name")
		if varNode == nil {
			continue
		}
		varName := e.nodeText(varNode)
		// Strip leading $
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:]
		}

		// Get value if present
		value := ""
		valueNode := findChildByFieldName(elem, "value")
		if valueNode == nil {
			// Try to find = and the next sibling
			for i := uint32(0); i < elem.ChildCount(); i++ {
				child := elem.Child(int(i))
				if child.Type() == "=" {
					if int(i)+1 < int(elem.ChildCount()) {
						valueNode = elem.Child(int(i) + 1)
					}
					break
				}
			}
		}
		if valueNode != nil {
			value = e.nodeText(valueNode)
			if len(value) > 50 {
				value = value[:47] + "..."
			}
		}

		// Determine entity kind
		kind := VarEntity
		if isReadonly {
			kind = ConstEntity // readonly properties are immutable
		}

		entity := Entity{
			Kind:       kind,
			Name:       varName,
			File:       e.getFilePath(),
			StartLine:  startLine,
			EndLine:    startLine,
			ValueType:  typeName,
			Value:      value,
			Visibility: visibility,
		}

		// Store class context and static info in receiver
		if className != "" {
			if isStatic {
				entity.Receiver = className + " (static)"
			} else {
				entity.Receiver = className
			}
		}

		entity.ComputeHashes()
		entities = append(entities, entity)
	}

	return entities
}

// extractClassConstant extracts a const declaration from a class.
func (e *PHPExtractor) extractClassConstant(node *sitter.Node, className string) []Entity {
	if node == nil || node.Type() != "const_declaration" {
		return nil
	}

	var entities []Entity

	// Check for modifiers (visibility)
	modifiers := e.extractPHPModifiers(node)
	visibility := determinePHPVisibility(modifiers)

	// Get const elements
	constElements := findChildrenByType(node, "const_element")

	startLine, _ := getLineRange(node)

	for _, elem := range constElements {
		// Get constant name
		nameNode := findChildByFieldName(elem, "name")
		if nameNode == nil {
			// Try to find identifier/name as first child
			for i := uint32(0); i < elem.ChildCount(); i++ {
				child := elem.Child(int(i))
				if child.Type() == "name" || child.Type() == "const_element" {
					nameNode = child
					break
				}
			}
		}
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get value
		value := ""
		valueNode := findChildByFieldName(elem, "value")
		if valueNode == nil {
			// Try to find = and the next sibling
			for i := uint32(0); i < elem.ChildCount(); i++ {
				child := elem.Child(int(i))
				if child.Type() == "=" {
					if int(i)+1 < int(elem.ChildCount()) {
						valueNode = elem.Child(int(i) + 1)
					}
					break
				}
			}
		}
		if valueNode != nil {
			value = e.nodeText(valueNode)
			if len(value) > 50 {
				value = value[:47] + "..."
			}
		}

		entity := Entity{
			Kind:       ConstEntity,
			Name:       name,
			File:       e.getFilePath(),
			StartLine:  startLine,
			EndLine:    startLine,
			Value:      value,
			Visibility: visibility,
		}

		if className != "" {
			entity.Receiver = className
		}

		entity.ComputeHashes()
		entities = append(entities, entity)
	}

	return entities
}

// extractMethodsFromClass extracts all methods from a class as EntityWithNode.
func (e *PHPExtractor) extractMethodsFromClass(classNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get class name
	nameNode := findChildByFieldName(classNode, "name")
	className := ""
	if nameNode != nil {
		className = e.nodeText(nameNode)
	}

	// Find declaration list (class body)
	bodyNode := findChildByType(classNode, "declaration_list")
	if bodyNode == nil {
		return result
	}

	// Extract methods
	methodNodes := findChildrenByType(bodyNode, "method_declaration")
	for _, node := range methodNodes {
		entity := e.extractMethod(node, className)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractMethodsFromClassEntities extracts all methods from a class as Entity slice.
func (e *PHPExtractor) extractMethodsFromClassEntities(classNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromClass(classNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractPropertiesFromClass extracts all properties from a class as EntityWithNode.
func (e *PHPExtractor) extractPropertiesFromClass(classNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get class name
	nameNode := findChildByFieldName(classNode, "name")
	className := ""
	if nameNode != nil {
		className = e.nodeText(nameNode)
	}

	// Find declaration list (class body)
	bodyNode := findChildByType(classNode, "declaration_list")
	if bodyNode == nil {
		return result
	}

	// Extract properties
	propNodes := findChildrenByType(bodyNode, "property_declaration")
	for _, node := range propNodes {
		entities := e.extractProperty(node, className)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result
}

// extractPropertiesFromClassEntities extracts all properties from a class as Entity slice.
func (e *PHPExtractor) extractPropertiesFromClassEntities(classNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractPropertiesFromClass(classNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractConstantsFromClass extracts all constants from a class as EntityWithNode.
func (e *PHPExtractor) extractConstantsFromClass(classNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get class name
	nameNode := findChildByFieldName(classNode, "name")
	className := ""
	if nameNode != nil {
		className = e.nodeText(nameNode)
	}

	// Find declaration list (class body)
	bodyNode := findChildByType(classNode, "declaration_list")
	if bodyNode == nil {
		return result
	}

	// Extract constants
	constNodes := findChildrenByType(bodyNode, "const_declaration")
	for _, node := range constNodes {
		entities := e.extractClassConstant(node, className)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result
}

// extractConstantsFromClassEntities extracts all constants from a class as Entity slice.
func (e *PHPExtractor) extractConstantsFromClassEntities(classNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractConstantsFromClass(classNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractMethodsFromInterface extracts method signatures from an interface as EntityWithNode.
func (e *PHPExtractor) extractMethodsFromInterface(interfaceNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get interface name
	nameNode := findChildByFieldName(interfaceNode, "name")
	interfaceName := ""
	if nameNode != nil {
		interfaceName = e.nodeText(nameNode)
	}

	// Find declaration list (interface body)
	bodyNode := findChildByType(interfaceNode, "declaration_list")
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
func (e *PHPExtractor) extractMethodsFromInterfaceEntities(interfaceNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromInterface(interfaceNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractMethodsFromTrait extracts all methods from a trait as EntityWithNode.
func (e *PHPExtractor) extractMethodsFromTrait(traitNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get trait name
	nameNode := findChildByFieldName(traitNode, "name")
	traitName := ""
	if nameNode != nil {
		traitName = e.nodeText(nameNode)
	}

	// Find declaration list (trait body)
	bodyNode := findChildByType(traitNode, "declaration_list")
	if bodyNode == nil {
		return result
	}

	// Extract methods
	methodNodes := findChildrenByType(bodyNode, "method_declaration")
	for _, node := range methodNodes {
		entity := e.extractMethod(node, traitName)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractMethodsFromTraitEntities extracts all methods from a trait as Entity slice.
func (e *PHPExtractor) extractMethodsFromTraitEntities(traitNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromTrait(traitNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// Helper functions

// extractPHPModifiers extracts all modifiers from a node.
func (e *PHPExtractor) extractPHPModifiers(node *sitter.Node) []string {
	var modifiers []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		// Check for visibility and other modifiers
		switch childType {
		case "visibility_modifier":
			modifiers = append(modifiers, e.nodeText(child))
		case "static_modifier":
			modifiers = append(modifiers, "static")
		case "final_modifier":
			modifiers = append(modifiers, "final")
		case "abstract_modifier":
			modifiers = append(modifiers, "abstract")
		case "readonly_modifier":
			modifiers = append(modifiers, "readonly")
		}

		// Check for modifier_list node
		if childType == "modifier_list" || childType == "visibility_modifier" {
			for j := uint32(0); j < child.ChildCount(); j++ {
				mod := child.Child(int(j))
				modType := mod.Type()
				switch modType {
				case "visibility_modifier":
					modifiers = append(modifiers, e.nodeText(mod))
				case "static_modifier":
					modifiers = append(modifiers, "static")
				case "final_modifier":
					modifiers = append(modifiers, "final")
				case "abstract_modifier":
					modifiers = append(modifiers, "abstract")
				case "readonly_modifier":
					modifiers = append(modifiers, "readonly")
				}
			}
		}
	}

	return modifiers
}

// extractPHPParameters extracts parameters from a formal_parameters node.
func (e *PHPExtractor) extractPHPParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param

	// Find simple_parameter nodes
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "simple_parameter" || child.Type() == "variadic_parameter" || child.Type() == "property_promotion_parameter" {
			param := e.extractParameter(child)
			if param.Name != "" {
				params = append(params, param)
			}
		}
	}

	return params
}

// extractParameter extracts a single parameter.
func (e *PHPExtractor) extractParameter(node *sitter.Node) Param {
	param := Param{}

	// Get type
	typeNode := findChildByFieldName(node, "type")
	if typeNode != nil {
		param.Type = e.extractPHPType(typeNode)
	}

	// Get name (variable_name)
	nameNode := findChildByFieldName(node, "name")
	if nameNode != nil {
		name := e.nodeText(nameNode)
		// Strip leading $
		if strings.HasPrefix(name, "$") {
			name = name[1:]
		}
		param.Name = name
	}

	// Check for variadic
	if node.Type() == "variadic_parameter" {
		param.Type = "..." + param.Type
	}

	return param
}

// extractPHPType extracts a type from a type node.
func (e *PHPExtractor) extractPHPType(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Handle different type node types
	switch node.Type() {
	case "primitive_type", "named_type", "name", "qualified_name":
		return e.nodeText(node)
	case "optional_type":
		// ?Type
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() != "?" {
				return "?" + e.extractPHPType(child)
			}
		}
	case "union_type":
		// Type|Type
		var types []string
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() != "|" {
				types = append(types, e.extractPHPType(child))
			}
		}
		return strings.Join(types, "|")
	case "intersection_type":
		// Type&Type
		var types []string
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() != "&" {
				types = append(types, e.extractPHPType(child))
			}
		}
		return strings.Join(types, "&")
	case "type_list":
		// For return types in colon notation
		var types []string
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			childType := e.extractPHPType(child)
			if childType != "" && childType != ":" && childType != "|" && childType != "&" {
				types = append(types, childType)
			}
		}
		if len(types) == 1 {
			return types[0]
		}
		return strings.Join(types, "|")
	}

	// Fallback
	return e.nodeText(node)
}

// extractInterfaceList extracts a list of interface names.
func (e *PHPExtractor) extractInterfaceList(node *sitter.Node) []string {
	var interfaces []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "name" || child.Type() == "qualified_name" {
			interfaces = append(interfaces, e.nodeText(child))
		}
	}

	return interfaces
}

// extractUsedTraits extracts traits used by a class.
func (e *PHPExtractor) extractUsedTraits(node *sitter.Node) []string {
	var traits []string

	// Find declaration list (class body)
	bodyNode := findChildByType(node, "declaration_list")
	if bodyNode == nil {
		return traits
	}

	// Find use_declaration nodes
	useNodes := findChildrenByType(bodyNode, "use_declaration")
	for _, useNode := range useNodes {
		for i := uint32(0); i < useNode.ChildCount(); i++ {
			child := useNode.Child(int(i))
			if child.Type() == "name" || child.Type() == "qualified_name" {
				traits = append(traits, e.nodeText(child))
			}
		}
	}

	return traits
}

// extractClassFields extracts properties from a class body as Field structs.
func (e *PHPExtractor) extractClassFields(node *sitter.Node) []Field {
	var fields []Field

	bodyNode := findChildByType(node, "declaration_list")
	if bodyNode == nil {
		return fields
	}

	propDecls := findChildrenByType(bodyNode, "property_declaration")
	for _, decl := range propDecls {
		// Get type
		typeName := ""
		typeNode := findChildByType(decl, "property_type")
		if typeNode == nil {
			typeNode = findChildByType(decl, "type")
		}
		if typeNode != nil {
			typeName = e.extractPHPType(typeNode)
		}

		// Get property names
		propElements := findChildrenByType(decl, "property_element")
		for _, elem := range propElements {
			varNode := findChildByType(elem, "variable_name")
			if varNode != nil {
				name := e.nodeText(varNode)
				// Strip leading $
				if strings.HasPrefix(name, "$") {
					name = name[1:]
				}
				fields = append(fields, Field{Name: name, Type: typeName})
			}
		}
	}

	return fields
}

// extractInterfaceMethods extracts method signatures from an interface as Field structs.
func (e *PHPExtractor) extractInterfaceMethods(node *sitter.Node) []Field {
	var methods []Field

	bodyNode := findChildByType(node, "declaration_list")
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
		returnTypeNode := findChildByFieldName(decl, "return_type")
		if returnTypeNode != nil {
			returnType = e.extractPHPType(returnTypeNode)
		}

		// Get parameters
		paramsNode := findChildByFieldName(decl, "parameters")
		params := e.extractPHPParameters(paramsNode)

		// Build method signature
		sig := formatPHPMethodSignature(params, returnType)

		methods = append(methods, Field{Name: name, Type: sig})
	}

	return methods
}

// extractTraitMethods extracts methods from a trait as Field structs.
func (e *PHPExtractor) extractTraitMethods(node *sitter.Node) []Field {
	var methods []Field

	bodyNode := findChildByType(node, "declaration_list")
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
		returnType := ""
		returnTypeNode := findChildByFieldName(decl, "return_type")
		if returnTypeNode != nil {
			returnType = e.extractPHPType(returnTypeNode)
		}

		// Get parameters
		paramsNode := findChildByFieldName(decl, "parameters")
		params := e.extractPHPParameters(paramsNode)

		// Build method signature
		sig := formatPHPMethodSignature(params, returnType)

		methods = append(methods, Field{Name: name, Type: sig})
	}

	return methods
}

// extractEnumCases extracts enum case values.
func (e *PHPExtractor) extractEnumCases(node *sitter.Node) []EnumValue {
	var values []EnumValue

	bodyNode := findChildByType(node, "enum_declaration_list")
	if bodyNode == nil {
		bodyNode = findChildByType(node, "declaration_list")
	}
	if bodyNode == nil {
		return values
	}

	// Find enum_case nodes
	caseNodes := findChildrenByType(bodyNode, "enum_case")
	for _, c := range caseNodes {
		nameNode := findChildByFieldName(c, "name")
		if nameNode == nil {
			// Try to find name child
			for i := uint32(0); i < c.ChildCount(); i++ {
				child := c.Child(int(i))
				if child.Type() == "name" {
					nameNode = child
					break
				}
			}
		}
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get value if present
		value := ""
		valueNode := findChildByFieldName(c, "value")
		if valueNode == nil {
			// Look for = and next sibling
			for i := uint32(0); i < c.ChildCount(); i++ {
				child := c.Child(int(i))
				if child.Type() == "=" {
					if int(i)+1 < int(c.ChildCount()) {
						valueNode = c.Child(int(i) + 1)
					}
					break
				}
			}
		}
		if valueNode != nil {
			value = e.nodeText(valueNode)
		}

		values = append(values, EnumValue{
			Name:  name,
			Value: value,
		})
	}

	return values
}

// isMethodContext checks if a function is inside a class/trait/interface context.
func (e *PHPExtractor) isMethodContext(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "class_declaration", "trait_declaration", "interface_declaration", "enum_declaration":
			return true
		case "program":
			return false
		}
		parent = parent.Parent()
	}
	return false
}

// getFilePath returns the normalized file path.
func (e *PHPExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *PHPExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// determinePHPVisibility determines visibility from PHP modifiers.
func determinePHPVisibility(modifiers []string) Visibility {
	for _, m := range modifiers {
		switch strings.ToLower(m) {
		case "public":
			return VisibilityPublic
		case "private", "protected":
			return VisibilityPrivate
		}
	}
	// Default visibility in PHP is public for class members
	return VisibilityPublic
}

// formatPHPMethodSignature formats a method signature for interface/trait methods.
func formatPHPMethodSignature(params []Param, returnType string) string {
	var sb strings.Builder

	sb.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Type != "" {
			sb.WriteString(p.Type)
			sb.WriteByte(' ')
		}
		sb.WriteString("$")
		sb.WriteString(p.Name)
	}
	sb.WriteByte(')')

	if returnType != "" && returnType != "void" {
		sb.WriteString(": ")
		sb.WriteString(returnType)
	}

	return sb.String()
}

// containsString checks if a slice contains a string.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

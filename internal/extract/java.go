package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// JavaExtractor extracts code entities from a parsed Java AST.
type JavaExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewJavaExtractor creates an extractor for the given Java parse result.
func NewJavaExtractor(result *parser.ParseResult) *JavaExtractor {
	return &JavaExtractor{
		result: result,
	}
}

// NewJavaExtractorWithBase creates an extractor with a base path for relative paths.
func NewJavaExtractorWithBase(result *parser.ParseResult, basePath string) *JavaExtractor {
	return &JavaExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the Java AST.
// Returns classes, interfaces, enums, methods, fields, and imports.
func (e *JavaExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract classes (regular, abstract, final)
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

	// Extract enums
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

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
func (e *JavaExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract classes
	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract methods and fields from class
		methodsWithNodes := e.extractMethodsFromClass(node)
		result = append(result, methodsWithNodes...)

		fieldsWithNodes := e.extractFieldsFromClass(node)
		result = append(result, fieldsWithNodes...)
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

	// Extract enums
	enumNodes := e.result.FindNodesByType("enum_declaration")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract imports
	importNodes := e.result.FindNodesByType("import_declaration")
	for _, node := range importNodes {
		entity := e.extractImport(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// ExtractClasses extracts all class declarations.
func (e *JavaExtractor) ExtractClasses() ([]Entity, error) {
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

		// Extract fields from class
		fields := e.extractFieldsFromClassEntities(node)
		entities = append(entities, fields...)
	}

	return entities, nil
}

// ExtractInterfaces extracts all interface declarations.
func (e *JavaExtractor) ExtractInterfaces() ([]Entity, error) {
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

// ExtractEnums extracts all enum declarations.
func (e *JavaExtractor) ExtractEnums() ([]Entity, error) {
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

// ExtractImports extracts all import declarations.
func (e *JavaExtractor) ExtractImports() ([]Entity, error) {
	var entities []Entity

	importNodes := e.result.FindNodesByType("import_declaration")
	for _, node := range importNodes {
		entity := e.extractImport(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractClass extracts a class declaration from its AST node.
func (e *JavaExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_declaration" {
		return nil
	}

	// Get class name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers (abstract, final, public, private, protected)
	modifiers := e.extractModifiers(node)
	visibility := determineJavaVisibility(modifiers)

	// Check for superclass
	superclass := ""
	superclassNode := findChildByFieldName(node, "superclass")
	if superclassNode != nil {
		// superclass is a superclass node containing a type
		typeNode := findChildByType(superclassNode, "type_identifier")
		if typeNode != nil {
			superclass = e.nodeText(typeNode)
		}
	}

	// Check for implemented interfaces
	var implements []string
	interfacesNode := findChildByFieldName(node, "interfaces")
	if interfacesNode != nil {
		implements = e.extractTypeList(interfacesNode)
	}

	// Extract type parameters (generics)
	typeParams := e.extractTypeParameters(node)

	// Extract fields as struct fields
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	// Build kind description
	typeKind := StructKind
	kindSuffix := ""
	if contains(modifiers, "abstract") {
		kindSuffix = " (abstract)"
	} else if contains(modifiers, "final") {
		kindSuffix = " (final)"
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

	// Store superclass in receiver field for tracking inheritance
	if superclass != "" {
		entity.Receiver = superclass
	}

	entity.ComputeHashes()
	return entity
}

// extractInterface extracts an interface declaration from its AST node.
func (e *JavaExtractor) extractInterface(node *sitter.Node) *Entity {
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
	visibility := determineJavaVisibility(modifiers)

	// Check for extended interfaces
	var extends []string
	extendsNode := findChildByFieldName(node, "extends")
	if extendsNode != nil {
		extends = e.extractTypeList(extendsNode)
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

// extractEnum extracts an enum declaration from its AST node.
func (e *JavaExtractor) extractEnum(node *sitter.Node) *Entity {
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
	visibility := determineJavaVisibility(modifiers)

	// Check for implemented interfaces
	var implements []string
	interfacesNode := findChildByFieldName(node, "interfaces")
	if interfacesNode != nil {
		implements = e.extractTypeList(interfacesNode)
	}

	// Extract enum constants
	enumValues := e.extractEnumConstants(node)

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       EnumEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		EnumValues: enumValues,
		Visibility: visibility,
		Implements: implements,
		ValueType:  "enum",
	}

	entity.ComputeHashes()
	return entity
}

// extractImport extracts an import declaration from its AST node.
func (e *JavaExtractor) extractImport(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "import_declaration" {
		return nil
	}

	// Get the import path - it's typically a scoped_identifier or identifier
	var importPath string
	var isStatic bool
	var isWildcard bool

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "static" {
			isStatic = true
		} else if childType == "scoped_identifier" || childType == "identifier" {
			importPath = e.nodeText(child)
		} else if childType == "asterisk" {
			isWildcard = true
		}
	}

	if importPath == "" {
		return nil
	}

	// Handle wildcard imports
	if isWildcard {
		importPath = importPath + ".*"
	}

	// Extract the simple name (last part of import)
	name := extractJavaImportName(importPath)

	startLine, _ := getLineRange(node)

	alias := ""
	if isStatic {
		alias = "static"
	}

	return &Entity{
		Kind:        ImportEntity,
		Name:        name,
		File:        e.getFilePath(),
		StartLine:   startLine,
		EndLine:     startLine,
		ImportPath:  importPath,
		ImportAlias: alias,
	}
}

// extractMethod extracts a method declaration from its AST node.
func (e *JavaExtractor) extractMethod(node *sitter.Node, className string) *Entity {
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
	visibility := determineJavaVisibility(modifiers)
	isStatic := contains(modifiers, "static")

	// Get return type
	returnType := ""
	typeNode := findChildByFieldName(node, "type")
	if typeNode != nil {
		returnType = abbreviateJavaType(e.nodeText(typeNode))
	}

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractJavaParameters(paramsNode)

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
	}

	// For instance methods, store the class name as receiver
	// For static methods, mark with (static)
	if className != "" {
		if isStatic {
			entity.Receiver = className + " (static)"
		} else {
			entity.Receiver = className
		}
	}

	entity.ComputeHashes()
	return entity
}

// extractConstructor extracts a constructor declaration from its AST node.
func (e *JavaExtractor) extractConstructor(node *sitter.Node, className string) *Entity {
	if node == nil || node.Type() != "constructor_declaration" {
		return nil
	}

	// Get constructor name (should match class name)
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineJavaVisibility(modifiers)

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractJavaParameters(paramsNode)

	// Get constructor body for hash computation
	bodyNode := findChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       MethodEntity, // Constructors are treated as methods
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    []string{className}, // Constructor returns the class type
		Receiver:   className + " (constructor)",
		RawBody:    rawBody,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractField extracts a field declaration from its AST node.
func (e *JavaExtractor) extractField(node *sitter.Node, className string) []Entity {
	if node == nil || node.Type() != "field_declaration" {
		return nil
	}

	var entities []Entity

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineJavaVisibility(modifiers)
	isStatic := contains(modifiers, "static")
	isFinal := contains(modifiers, "final")

	// Get type
	typeNode := findChildByFieldName(node, "type")
	typeName := ""
	if typeNode != nil {
		typeName = abbreviateJavaType(e.nodeText(typeNode))
	}

	// Get variable declarators (can be multiple: int x, y, z;)
	declarators := findChildrenByType(node, "variable_declarator")

	startLine, _ := getLineRange(node)

	for _, decl := range declarators {
		nameNode := findChildByFieldName(decl, "name")
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get value if present
		value := ""
		valueNode := findChildByFieldName(decl, "value")
		if valueNode != nil {
			value = e.nodeText(valueNode)
			if len(value) > 50 {
				value = value[:47] + "..."
			}
		}

		// Determine entity kind based on modifiers
		kind := VarEntity
		if isFinal {
			kind = ConstEntity
		}

		entity := Entity{
			Kind:       kind,
			Name:       name,
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

// extractMethodsFromClass extracts all methods from a class as EntityWithNode.
func (e *JavaExtractor) extractMethodsFromClass(classNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get class name
	nameNode := findChildByFieldName(classNode, "name")
	className := ""
	if nameNode != nil {
		className = e.nodeText(nameNode)
	}

	// Find class body
	bodyNode := findChildByFieldName(classNode, "body")
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

	// Extract constructors
	constructorNodes := findChildrenByType(bodyNode, "constructor_declaration")
	for _, node := range constructorNodes {
		entity := e.extractConstructor(node, className)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractMethodsFromClassEntities extracts all methods from a class as Entity slice.
func (e *JavaExtractor) extractMethodsFromClassEntities(classNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromClass(classNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractFieldsFromClass extracts all fields from a class as EntityWithNode.
func (e *JavaExtractor) extractFieldsFromClass(classNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get class name
	nameNode := findChildByFieldName(classNode, "name")
	className := ""
	if nameNode != nil {
		className = e.nodeText(nameNode)
	}

	// Find class body
	bodyNode := findChildByFieldName(classNode, "body")
	if bodyNode == nil {
		return result
	}

	// Extract fields
	fieldNodes := findChildrenByType(bodyNode, "field_declaration")
	for _, node := range fieldNodes {
		entities := e.extractField(node, className)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result
}

// extractFieldsFromClassEntities extracts all fields from a class as Entity slice.
func (e *JavaExtractor) extractFieldsFromClassEntities(classNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractFieldsFromClass(classNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractMethodsFromInterface extracts method signatures from an interface as EntityWithNode.
func (e *JavaExtractor) extractMethodsFromInterface(interfaceNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get interface name
	nameNode := findChildByFieldName(interfaceNode, "name")
	interfaceName := ""
	if nameNode != nil {
		interfaceName = e.nodeText(nameNode)
	}

	// Find interface body
	bodyNode := findChildByFieldName(interfaceNode, "body")
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
func (e *JavaExtractor) extractMethodsFromInterfaceEntities(interfaceNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMethodsFromInterface(interfaceNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// Helper functions

// extractModifiers extracts all modifiers from a node.
func (e *JavaExtractor) extractModifiers(node *sitter.Node) []string {
	var modifiers []string

	// Modifiers can be direct children or in a modifiers node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "modifiers" {
			// Extract from modifiers container
			for j := uint32(0); j < child.ChildCount(); j++ {
				mod := child.Child(int(j))
				modType := mod.Type()
				// Check if it's a modifier keyword
				if isJavaModifier(modType) {
					modifiers = append(modifiers, modType)
				} else if modType == "marker_annotation" || modType == "annotation" {
					// Capture annotation name for context
					annotationName := e.extractAnnotationName(mod)
					if annotationName != "" {
						modifiers = append(modifiers, "@"+annotationName)
					}
				}
			}
		} else if isJavaModifier(childType) {
			modifiers = append(modifiers, childType)
		}
	}

	return modifiers
}

// extractAnnotationName extracts the name from an annotation node.
func (e *JavaExtractor) extractAnnotationName(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	nameNode := findChildByFieldName(node, "name")
	if nameNode != nil {
		return e.nodeText(nameNode)
	}

	// Try to find identifier child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			return e.nodeText(child)
		}
	}

	return ""
}

// extractTypeParameters extracts generic type parameters like <T> or <K, V>.
func (e *JavaExtractor) extractTypeParameters(node *sitter.Node) string {
	typeParamsNode := findChildByFieldName(node, "type_parameters")
	if typeParamsNode == nil {
		return ""
	}
	return e.nodeText(typeParamsNode)
}

// extractTypeList extracts a list of types from a type list node (for extends/implements).
func (e *JavaExtractor) extractTypeList(node *sitter.Node) []string {
	var types []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "type_identifier" || childType == "generic_type" || childType == "scoped_type_identifier" {
			types = append(types, e.nodeText(child))
		} else if childType == "type_list" {
			// Recurse into type_list
			types = append(types, e.extractTypeList(child)...)
		}
	}

	return types
}

// extractClassFields extracts fields from a class body as Field structs.
func (e *JavaExtractor) extractClassFields(node *sitter.Node) []Field {
	var fields []Field

	bodyNode := findChildByFieldName(node, "body")
	if bodyNode == nil {
		return fields
	}

	fieldDecls := findChildrenByType(bodyNode, "field_declaration")
	for _, decl := range fieldDecls {
		// Get type
		typeNode := findChildByFieldName(decl, "type")
		typeName := ""
		if typeNode != nil {
			typeName = abbreviateJavaType(e.nodeText(typeNode))
		}

		// Get variable names
		declarators := findChildrenByType(decl, "variable_declarator")
		for _, d := range declarators {
			nameNode := findChildByFieldName(d, "name")
			if nameNode != nil {
				name := e.nodeText(nameNode)
				fields = append(fields, Field{Name: name, Type: typeName})
			}
		}
	}

	return fields
}

// extractInterfaceMethods extracts method signatures from an interface as Field structs.
func (e *JavaExtractor) extractInterfaceMethods(node *sitter.Node) []Field {
	var methods []Field

	bodyNode := findChildByFieldName(node, "body")
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
		if typeNode != nil {
			returnType = abbreviateJavaType(e.nodeText(typeNode))
		}

		// Get parameters
		paramsNode := findChildByFieldName(decl, "parameters")
		params := e.extractJavaParameters(paramsNode)

		// Build method signature
		sig := formatJavaMethodSignature(params, returnType)

		methods = append(methods, Field{Name: name, Type: sig})
	}

	return methods
}

// extractEnumConstants extracts enum constant values.
func (e *JavaExtractor) extractEnumConstants(node *sitter.Node) []EnumValue {
	var values []EnumValue

	bodyNode := findChildByFieldName(node, "body")
	if bodyNode == nil {
		return values
	}

	// Find enum_constant nodes
	constants := findChildrenByType(bodyNode, "enum_constant")
	for i, c := range constants {
		nameNode := findChildByFieldName(c, "name")
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Enum constants don't have explicit values in Java, use ordinal
		values = append(values, EnumValue{
			Name:  name,
			Value: string(rune('0' + i)),
		})
	}

	return values
}

// extractJavaParameters extracts parameters from a formal_parameters node.
func (e *JavaExtractor) extractJavaParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param

	// Find formal_parameter nodes
	paramDecls := findChildrenByType(node, "formal_parameter")
	for _, decl := range paramDecls {
		// Get type
		typeNode := findChildByFieldName(decl, "type")
		typeName := ""
		if typeNode != nil {
			typeName = abbreviateJavaType(e.nodeText(typeNode))
		}

		// Get name
		nameNode := findChildByFieldName(decl, "name")
		name := ""
		if nameNode != nil {
			name = e.nodeText(nameNode)
		}

		params = append(params, Param{Name: name, Type: typeName})
	}

	// Handle spread/varargs parameters
	spreadParams := findChildrenByType(node, "spread_parameter")
	for _, decl := range spreadParams {
		// Get type
		typeNode := findChildByFieldName(decl, "type")
		typeName := ""
		if typeNode != nil {
			typeName = "..." + abbreviateJavaType(e.nodeText(typeNode))
		}

		// Get name
		nameNode := findChildByFieldName(decl, "name")
		name := ""
		if nameNode != nil {
			name = e.nodeText(nameNode)
		}

		params = append(params, Param{Name: name, Type: typeName})
	}

	return params
}

// getFilePath returns the normalized file path.
func (e *JavaExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *JavaExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// isJavaModifier checks if a node type is a Java modifier keyword.
func isJavaModifier(nodeType string) bool {
	modifiers := map[string]bool{
		"public":       true,
		"private":      true,
		"protected":    true,
		"static":       true,
		"final":        true,
		"abstract":     true,
		"synchronized": true,
		"native":       true,
		"transient":    true,
		"volatile":     true,
		"strictfp":     true,
		"default":      true,
	}
	return modifiers[nodeType]
}

// determineJavaVisibility determines visibility from Java modifiers.
func determineJavaVisibility(modifiers []string) Visibility {
	for _, m := range modifiers {
		switch m {
		case "public":
			return VisibilityPublic
		case "private", "protected":
			return VisibilityPrivate
		}
	}
	// Package-private (no modifier) is treated as private
	return VisibilityPrivate
}

// abbreviateJavaType converts Java types to abbreviated form.
func abbreviateJavaType(javaType string) string {
	javaType = strings.TrimSpace(javaType)

	// Handle generic types by preserving them
	// Handle array types by preserving []
	return javaType
}

// formatJavaMethodSignature formats a method signature for interface methods.
func formatJavaMethodSignature(params []Param, returnType string) string {
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

// extractJavaImportName extracts the simple name from an import path.
func extractJavaImportName(importPath string) string {
	// Handle wildcard imports
	if strings.HasSuffix(importPath, ".*") {
		// Return package name for wildcard imports
		parts := strings.Split(strings.TrimSuffix(importPath, ".*"), ".")
		if len(parts) > 0 {
			return parts[len(parts)-1] + ".*"
		}
		return "*"
	}

	// Return simple class name
	parts := strings.Split(importPath, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return importPath
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

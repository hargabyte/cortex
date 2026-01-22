package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// KotlinExtractor extracts code entities from a parsed Kotlin AST.
type KotlinExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewKotlinExtractor creates an extractor for the given Kotlin parse result.
func NewKotlinExtractor(result *parser.ParseResult) *KotlinExtractor {
	return &KotlinExtractor{
		result: result,
	}
}

// NewKotlinExtractorWithBase creates an extractor with a base path for relative paths.
func NewKotlinExtractorWithBase(result *parser.ParseResult, basePath string) *KotlinExtractor {
	return &KotlinExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the Kotlin AST.
// Returns classes, interfaces, objects, functions, properties, and imports.
func (e *KotlinExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract classes (regular, data, sealed, enum, annotation)
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

	// Extract objects
	objects, err := e.ExtractObjects()
	if err != nil {
		return nil, err
	}
	entities = append(entities, objects...)

	// Extract top-level functions
	functions, err := e.ExtractFunctions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, functions...)

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
func (e *KotlinExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract classes and interfaces (both use class_declaration node type)
	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		// Check if this is an interface (first child is "interface" keyword)
		if node.ChildCount() > 0 && node.Child(0).Type() == "interface" {
			entity := e.extractInterface(node)
			if entity != nil {
				result = append(result, EntityWithNode{Entity: entity, Node: node})
			}

			// Extract method signatures from interface
			membersWithNodes := e.extractMembersFromInterface(node)
			result = append(result, membersWithNodes...)
		} else {
			// Regular class
			entity := e.extractClass(node)
			if entity != nil {
				result = append(result, EntityWithNode{Entity: entity, Node: node})
			}

			// Extract methods, properties from class
			membersWithNodes := e.extractMembersFromClass(node)
			result = append(result, membersWithNodes...)
		}
	}

	// Extract objects
	objectNodes := e.result.FindNodesByType("object_declaration")
	for _, node := range objectNodes {
		entity := e.extractObject(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract members from object
		membersWithNodes := e.extractMembersFromClass(node)
		result = append(result, membersWithNodes...)
	}

	// Extract companion objects
	companionNodes := e.result.FindNodesByType("companion_object")
	for _, node := range companionNodes {
		entity := e.extractCompanionObject(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}

		// Extract members from companion object
		membersWithNodes := e.extractMembersFromClass(node)
		result = append(result, membersWithNodes...)
	}

	// Extract top-level functions
	functionNodes := e.result.FindNodesByType("function_declaration")
	for _, node := range functionNodes {
		// Skip if inside a class/object/interface
		if !e.isTopLevel(node) {
			continue
		}
		entity := e.extractFunction(node, "")
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract imports
	importNodes := e.result.FindNodesByType("import_header")
	for _, node := range importNodes {
		entity := e.extractImport(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// ExtractClasses extracts all class declarations.
func (e *KotlinExtractor) ExtractClasses() ([]Entity, error) {
	var entities []Entity

	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		entity := e.extractClass(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract methods and properties from class
		members := e.extractMembersFromClassEntities(node)
		entities = append(entities, members...)
	}

	return entities, nil
}

// ExtractInterfaces extracts all interface declarations.
// In tree-sitter-kotlin, interfaces are class_declaration nodes with "interface" keyword.
func (e *KotlinExtractor) ExtractInterfaces() ([]Entity, error) {
	var entities []Entity

	// tree-sitter-kotlin uses class_declaration for interfaces too
	classNodes := e.result.FindNodesByType("class_declaration")
	for _, node := range classNodes {
		// Check if first child is "interface" keyword
		if node.ChildCount() > 0 {
			firstChild := node.Child(0)
			if firstChild != nil && firstChild.Type() == "interface" {
				entity := e.extractInterface(node)
				if entity != nil {
					entities = append(entities, *entity)
				}

				// Extract method signatures from interface
				members := e.extractMembersFromInterfaceEntities(node)
				entities = append(entities, members...)
			}
		}
	}

	return entities, nil
}

// ExtractObjects extracts all object declarations (including companion objects).
func (e *KotlinExtractor) ExtractObjects() ([]Entity, error) {
	var entities []Entity

	// Regular object declarations
	objectNodes := e.result.FindNodesByType("object_declaration")
	for _, node := range objectNodes {
		entity := e.extractObject(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract members from object
		members := e.extractMembersFromClassEntities(node)
		entities = append(entities, members...)
	}

	// Companion objects (nested inside classes)
	companionNodes := e.result.FindNodesByType("companion_object")
	for _, node := range companionNodes {
		entity := e.extractCompanionObject(node)
		if entity != nil {
			entities = append(entities, *entity)
		}

		// Extract members from companion object
		members := e.extractMembersFromClassEntities(node)
		entities = append(entities, members...)
	}

	return entities, nil
}

// ExtractFunctions extracts top-level function declarations.
func (e *KotlinExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	functionNodes := e.result.FindNodesByType("function_declaration")
	for _, node := range functionNodes {
		// Only extract top-level functions
		if e.isTopLevel(node) {
			entity := e.extractFunction(node, "")
			if entity != nil {
				entities = append(entities, *entity)
			}
		}
	}

	return entities, nil
}

// ExtractImports extracts all import declarations.
func (e *KotlinExtractor) ExtractImports() ([]Entity, error) {
	var entities []Entity

	importNodes := e.result.FindNodesByType("import_header")
	for _, node := range importNodes {
		entity := e.extractImport(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// extractClass extracts a class declaration from its AST node.
func (e *KotlinExtractor) extractClass(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_declaration" {
		return nil
	}

	// Get class name - for classes it's type_identifier, for functions it's simple_identifier
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		nameNode = findChildByType(node, "simple_identifier")
	}
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers (data, sealed, open, final, abstract, etc.)
	modifiers := e.extractModifiers(node)
	visibility := determineKotlinVisibility(modifiers)

	// Determine class kind
	classKind := "class"
	kindSuffix := ""
	if containsKotlin(modifiers, "data") {
		kindSuffix = " (data)"
	} else if containsKotlin(modifiers, "sealed") {
		kindSuffix = " (sealed)"
	} else if containsKotlin(modifiers, "enum") {
		kindSuffix = " (enum)"
	} else if containsKotlin(modifiers, "annotation") {
		kindSuffix = " (annotation)"
	} else if containsKotlin(modifiers, "abstract") {
		kindSuffix = " (abstract)"
	} else if containsKotlin(modifiers, "open") {
		kindSuffix = " (open)"
	}

	// Check for superclass and interfaces
	var superclass string
	var implements []string

	// Look for delegation_specifier nodes (singular, not plural)
	// tree-sitter-kotlin produces multiple delegation_specifier nodes, not a single delegation_specifiers container
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "delegation_specifier" {
			// Check if this is a constructor_invocation (superclass) or just a type (interface)
			constructorNode := findChildByType(child, "constructor_invocation")
			if constructorNode != nil {
				// This is a superclass with constructor call
				typeNode := findChildByType(constructorNode, "user_type")
				if typeNode != nil {
					typeIdNode := findChildByType(typeNode, "type_identifier")
					if typeIdNode != nil {
						superclass = e.nodeText(typeIdNode)
					}
				}
			} else {
				// This is an interface (or superclass without constructor call)
				typeNode := findChildByType(child, "user_type")
				if typeNode != nil {
					typeIdNode := findChildByType(typeNode, "type_identifier")
					if typeIdNode != nil {
						typeName := e.nodeText(typeIdNode)
						if superclass == "" {
							// If we haven't found a superclass yet and this looks like a class, use it
							// Otherwise treat it as an interface
							implements = append(implements, typeName)
						} else {
							implements = append(implements, typeName)
						}
					}
				}
			}
		}
	}

	// Extract type parameters (generics)
	typeParams := e.extractTypeParameters(node)

	// Extract properties as struct fields
	fields := e.extractClassFields(node)

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
		ValueType:  classKind + kindSuffix,
	}

	// Store superclass in receiver field for tracking inheritance
	if superclass != "" {
		entity.Receiver = superclass
	}

	entity.ComputeHashes()
	return entity
}

// extractInterface extracts an interface declaration from its AST node.
// In tree-sitter-kotlin, interfaces are class_declaration nodes with "interface" keyword.
func (e *KotlinExtractor) extractInterface(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "class_declaration" {
		return nil
	}
	// Verify it's actually an interface
	if node.ChildCount() == 0 || node.Child(0).Type() != "interface" {
		return nil
	}

	// Get interface name
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		nameNode = findChildByType(node, "simple_identifier")
	}
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineKotlinVisibility(modifiers)

	// Check for extended interfaces
	var extends []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "delegation_specifiers" {
			extends = e.extractDelegationSpecifiers(child)
		}
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
		Implements: extends,
		ValueType:  "interface",
	}

	entity.ComputeHashes()
	return entity
}

// extractObject extracts an object declaration from its AST node.
func (e *KotlinExtractor) extractObject(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "object_declaration" {
		return nil
	}

	// Get object name - tree-sitter-kotlin uses type_identifier for object names
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		nameNode = findChildByType(node, "simple_identifier")
	}
	if nameNode == nil {
		return nil // Object must have a name
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineKotlinVisibility(modifiers)
	isCompanion := containsKotlin(modifiers, "companion")

	// Check for implemented interfaces
	var implements []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "delegation_specifiers" {
			implements = e.extractDelegationSpecifiers(child)
		}
	}

	// Extract properties as struct fields
	fields := e.extractClassFields(node)

	startLine, endLine := getLineRange(node)

	kindSuffix := ""
	if isCompanion {
		kindSuffix = " (companion)"
	}

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Visibility: visibility,
		Implements: implements,
		ValueType:  "object" + kindSuffix,
	}

	entity.ComputeHashes()
	return entity
}

// extractCompanionObject extracts a companion object from its AST node.
func (e *KotlinExtractor) extractCompanionObject(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "companion_object" {
		return nil
	}

	// Companion objects may have a name or be anonymous
	nameNode := findChildByType(node, "type_identifier")
	name := "Companion" // Default name for anonymous companion objects
	if nameNode != nil {
		name = e.nodeText(nameNode)
	}

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineKotlinVisibility(modifiers)

	// Check for implemented interfaces
	var implements []string
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "delegation_specifiers" {
			implements = e.extractDelegationSpecifiers(child)
		}
	}

	// Extract properties as struct fields
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
		Visibility: visibility,
		Implements: implements,
		ValueType:  "object (companion)",
	}

	entity.ComputeHashes()
	return entity
}

// extractFunction extracts a function declaration from its AST node.
func (e *KotlinExtractor) extractFunction(node *sitter.Node, className string) *Entity {
	if node == nil || node.Type() != "function_declaration" {
		return nil
	}

	// Get function name
	nameNode := findChildByType(node, "simple_identifier")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineKotlinVisibility(modifiers)
	isStatic := false // In Kotlin, companion object members act as static
	isSuspend := containsKotlin(modifiers, "suspend")
	isInline := containsKotlin(modifiers, "inline")
	isExtension := e.isExtensionFunction(node)

	// Get receiver type for extension functions
	receiverType := ""
	if isExtension {
		receiverType = e.extractReceiverType(node)
	}

	// Get return type - look for user_type first
	returnType := "Unit"
	userTypeNode := findChildByType(node, "user_type")
	if userTypeNode != nil {
		// Extract the type_identifier from user_type
		typeIdNode := findChildByType(userTypeNode, "type_identifier")
		if typeIdNode != nil {
			returnType = abbreviateKotlinType(e.nodeText(typeIdNode))
		} else {
			returnType = abbreviateKotlinType(e.nodeText(userTypeNode))
		}
	}

	// Get parameters
	paramsNode := findChildByType(node, "function_value_parameters")
	params := e.extractKotlinParameters(paramsNode)

	// Get function body for hash computation
	bodyNode := findChildByType(node, "function_body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	// Extract type parameters (generics on function)
	typeParams := e.extractTypeParameters(node)

	startLine, endLine := getLineRange(node)

	// Build function name with modifiers
	fullName := name + typeParams
	if isSuspend {
		fullName = "suspend " + fullName
	}
	if isInline {
		fullName = "inline " + fullName
	}

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       fullName,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    []string{returnType},
		RawBody:    rawBody,
		Visibility: visibility,
	}

	// Determine receiver
	if isExtension && receiverType != "" {
		entity.Receiver = receiverType + " (extension)"
	} else if className != "" {
		if isStatic {
			entity.Receiver = className + " (static)"
		} else {
			entity.Receiver = className
		}
	}

	entity.ComputeHashes()
	return entity
}

// extractProperty extracts a property declaration from its AST node.
func (e *KotlinExtractor) extractProperty(node *sitter.Node, className string) []Entity {
	if node == nil || node.Type() != "property_declaration" {
		return nil
	}

	var entities []Entity

	// Check for modifiers
	modifiers := e.extractModifiers(node)
	visibility := determineKotlinVisibility(modifiers)
	isConst := containsKotlin(modifiers, "const")
	isVal := false

	// Check binding_pattern_kind for val/var
	bindingPatternNode := findChildByType(node, "binding_pattern_kind")
	if bindingPatternNode != nil {
		for i := uint32(0); i < bindingPatternNode.ChildCount(); i++ {
			child := bindingPatternNode.Child(int(i))
			if child.Type() == "val" {
				isVal = true
			}
		}
	}

	// Get variable declaration
	varDecl := findChildByType(node, "variable_declaration")
	if varDecl == nil {
		return nil
	}

	// Get name
	nameNode := findChildByType(varDecl, "simple_identifier")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type - it's in a user_type node inside variable_declaration
	userTypeNode := findChildByType(varDecl, "user_type")
	typeName := ""
	if userTypeNode != nil {
		typeIdNode := findChildByType(userTypeNode, "type_identifier")
		if typeIdNode != nil {
			typeName = abbreviateKotlinType(e.nodeText(typeIdNode))
		} else {
			typeName = abbreviateKotlinType(e.nodeText(userTypeNode))
		}
	}

	// Get initial value if present
	value := ""
	// Look for expression after "="
	for j := uint32(0); j < node.ChildCount(); j++ {
		child := node.Child(int(j))
		if child.Type() == "=" {
			// Next node should be the expression
			if j+1 < node.ChildCount() {
				valueNode := node.Child(int(j + 1))
				value = e.nodeText(valueNode)
				if len(value) > 50 {
					value = value[:47] + "..."
				}
			}
			break
		}
	}

	// Determine entity kind
	kind := VarEntity
	if isConst || isVal {
		kind = ConstEntity
	}

	startLine, _ := getLineRange(node)

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

	// Store class context
	if className != "" {
		entity.Receiver = className
	}

	entity.ComputeHashes()
	entities = append(entities, entity)

	return entities
}

// extractImport extracts an import declaration from its AST node.
func (e *KotlinExtractor) extractImport(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "import_header" {
		return nil
	}

	// Get the import path - it's typically an identifier chain
	var importPath string

	// Look for identifier nodes within import_header
	identifiers := findChildrenByType(node, "identifier")
	if len(identifiers) > 0 {
		// Build the path from identifiers
		var parts []string
		for _, id := range identifiers {
			parts = append(parts, e.nodeText(id))
		}
		importPath = strings.Join(parts, ".")
	}

	// Fallback: get all text after "import" keyword
	if importPath == "" {
		text := e.nodeText(node)
		text = strings.TrimPrefix(text, "import")
		text = strings.TrimSpace(text)
		importPath = text
	}

	if importPath == "" {
		return nil
	}

	// Check for wildcard imports (*)
	if strings.HasSuffix(importPath, ".*") {
		// Already has wildcard
	} else if strings.Contains(e.nodeText(node), "*") {
		importPath = importPath + ".*"
	}

	// Extract the simple name (last part of import)
	name := extractKotlinImportName(importPath)

	startLine, _ := getLineRange(node)

	// Check for import alias ("as")
	alias := ""
	asNode := findChildByType(node, "import_alias")
	if asNode != nil {
		// Tree-sitter Kotlin uses type_identifier for the alias name
		aliasId := findChildByType(asNode, "type_identifier")
		if aliasId == nil {
			// Fallback to simple_identifier for compatibility
			aliasId = findChildByType(asNode, "simple_identifier")
		}
		if aliasId != nil {
			alias = e.nodeText(aliasId)
		}
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

// extractMembersFromClass extracts all members from a class/object as EntityWithNode.
func (e *KotlinExtractor) extractMembersFromClass(classNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get class name
	nameNode := findChildByType(classNode, "simple_identifier")
	className := ""
	if nameNode != nil {
		className = e.nodeText(nameNode)
	}

	// Find class body
	bodyNode := findChildByType(classNode, "class_body")
	if bodyNode == nil {
		return result
	}

	// Extract functions
	functionNodes := findChildrenByType(bodyNode, "function_declaration")
	for _, node := range functionNodes {
		entity := e.extractFunction(node, className)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract properties
	propertyNodes := findChildrenByType(bodyNode, "property_declaration")
	for _, node := range propertyNodes {
		entities := e.extractProperty(node, className)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result
}

// extractMembersFromClassEntities extracts all members from a class/object as Entity slice.
func (e *KotlinExtractor) extractMembersFromClassEntities(classNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMembersFromClass(classNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// extractMembersFromInterface extracts method signatures from an interface as EntityWithNode.
func (e *KotlinExtractor) extractMembersFromInterface(interfaceNode *sitter.Node) []EntityWithNode {
	var result []EntityWithNode

	// Get interface name
	nameNode := findChildByType(interfaceNode, "simple_identifier")
	interfaceName := ""
	if nameNode != nil {
		interfaceName = e.nodeText(nameNode)
	}

	// Find class body (interfaces also use class_body in tree-sitter-kotlin)
	bodyNode := findChildByType(interfaceNode, "class_body")
	if bodyNode == nil {
		return result
	}

	// Extract function declarations
	functionNodes := findChildrenByType(bodyNode, "function_declaration")
	for _, node := range functionNodes {
		entity := e.extractFunction(node, interfaceName)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result
}

// extractMembersFromInterfaceEntities extracts method signatures from an interface as Entity slice.
func (e *KotlinExtractor) extractMembersFromInterfaceEntities(interfaceNode *sitter.Node) []Entity {
	var entities []Entity
	ewns := e.extractMembersFromInterface(interfaceNode)
	for _, ewn := range ewns {
		entities = append(entities, *ewn.Entity)
	}
	return entities
}

// Helper functions

// extractModifiers extracts all modifiers from a node.
func (e *KotlinExtractor) extractModifiers(node *sitter.Node) []string {
	var modifiers []string

	// Look for modifiers node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "modifiers" {
			// Extract from modifiers container - recurse to handle nested modifiers
			e.extractModifiersRecursive(child, &modifiers)
		} else if isKotlinModifier(childType) {
			modifiers = append(modifiers, childType)
		}

		// Check for special keywords at top level
		if childType == "companion" || childType == "data" || childType == "sealed" ||
			childType == "enum" || childType == "annotation" || childType == "suspend" ||
			childType == "inline" {
			modifiers = append(modifiers, childType)
		}
	}

	return modifiers
}

// extractModifiersRecursive recursively extracts modifiers from nested nodes.
func (e *KotlinExtractor) extractModifiersRecursive(node *sitter.Node, modifiers *[]string) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Check if this node itself is a modifier
	if isKotlinModifier(nodeType) {
		*modifiers = append(*modifiers, nodeType)
		return
	}

	// Special keywords
	if nodeType == "companion" || nodeType == "data" || nodeType == "sealed" ||
		nodeType == "enum" || nodeType == "annotation" || nodeType == "suspend" ||
		nodeType == "inline" {
		*modifiers = append(*modifiers, nodeType)
		return
	}

	// Recurse into children
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		e.extractModifiersRecursive(child, modifiers)
	}
}

// extractTypeParameters extracts generic type parameters like <T> or <K, V>.
func (e *KotlinExtractor) extractTypeParameters(node *sitter.Node) string {
	typeParamsNode := findChildByType(node, "type_parameters")
	if typeParamsNode == nil {
		return ""
	}
	return e.nodeText(typeParamsNode)
}

// extractDelegationSpecifiers extracts superclass and interfaces from delegation_specifiers.
func (e *KotlinExtractor) extractDelegationSpecifiers(node *sitter.Node) []string {
	var types []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))

		// Look for type references or constructor invocations
		if child.Type() == "user_type" || child.Type() == "constructor_invocation" {
			text := e.nodeText(child)
			if text != "" {
				types = append(types, text)
			}
		}
	}

	return types
}

// extractClassFields extracts properties from a class body as Field structs.
func (e *KotlinExtractor) extractClassFields(node *sitter.Node) []Field {
	var fields []Field

	bodyNode := findChildByType(node, "class_body")
	if bodyNode == nil {
		// Check for primary constructor parameters (can be properties)
		primaryConstructor := findChildByType(node, "primary_constructor")
		if primaryConstructor != nil {
			// class_parameters are direct children of primary_constructor
			return e.extractClassParametersAsFields(primaryConstructor)
		}
		return fields
	}

	propertyDecls := findChildrenByType(bodyNode, "property_declaration")
	for _, decl := range propertyDecls {
		// Get type
		typeNode := findChildByType(decl, "type")
		typeName := ""
		if typeNode != nil {
			typeName = abbreviateKotlinType(e.nodeText(typeNode))
		}

		// Get variable declaration
		varDecl := findChildByType(decl, "variable_declaration")
		if varDecl != nil {
			nameNode := findChildByType(varDecl, "simple_identifier")
			if nameNode != nil {
				name := e.nodeText(nameNode)
				fields = append(fields, Field{Name: name, Type: typeName})
			}
		}
	}

	return fields
}

// extractClassParametersAsFields extracts class constructor parameters as fields.
func (e *KotlinExtractor) extractClassParametersAsFields(node *sitter.Node) []Field {
	var fields []Field

	classParams := findChildrenByType(node, "class_parameter")
	for _, param := range classParams {
		// Check if this parameter has val/var in binding_pattern_kind
		hasVal := false
		hasVar := false

		bindingPatternNode := findChildByType(param, "binding_pattern_kind")
		if bindingPatternNode != nil {
			// Check if it contains val or var
			for i := uint32(0); i < bindingPatternNode.ChildCount(); i++ {
				child := bindingPatternNode.Child(int(i))
				if child.Type() == "val" {
					hasVal = true
				} else if child.Type() == "var" {
					hasVar = true
				}
			}
		}

		if !hasVal && !hasVar {
			continue // Not a property parameter
		}

		// Get parameter name
		nameNode := findChildByType(param, "simple_identifier")
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get parameter type - it's in a user_type node
		userTypeNode := findChildByType(param, "user_type")
		typeName := ""
		if userTypeNode != nil {
			typeIdNode := findChildByType(userTypeNode, "type_identifier")
			if typeIdNode != nil {
				typeName = abbreviateKotlinType(e.nodeText(typeIdNode))
			} else {
				typeName = abbreviateKotlinType(e.nodeText(userTypeNode))
			}
		}

		fields = append(fields, Field{Name: name, Type: typeName})
	}

	return fields
}

// extractInterfaceMethods extracts method signatures from an interface as Field structs.
func (e *KotlinExtractor) extractInterfaceMethods(node *sitter.Node) []Field {
	var methods []Field

	bodyNode := findChildByType(node, "class_body")
	if bodyNode == nil {
		return methods
	}

	functionDecls := findChildrenByType(bodyNode, "function_declaration")
	for _, decl := range functionDecls {
		// Get function name
		nameNode := findChildByType(decl, "simple_identifier")
		if nameNode == nil {
			continue
		}
		name := e.nodeText(nameNode)

		// Get return type
		returnType := "Unit"
		typeNode := findChildByType(decl, "type")
		if typeNode != nil {
			returnType = abbreviateKotlinType(e.nodeText(typeNode))
		}

		// Get parameters
		paramsNode := findChildByType(decl, "function_value_parameters")
		params := e.extractKotlinParameters(paramsNode)

		// Build method signature
		sig := formatKotlinMethodSignature(params, returnType)

		methods = append(methods, Field{Name: name, Type: sig})
	}

	return methods
}

// extractKotlinParameters extracts parameters from a function_value_parameters node.
func (e *KotlinExtractor) extractKotlinParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param

	// Find parameter nodes
	paramDecls := findChildrenByType(node, "parameter")
	for _, decl := range paramDecls {
		// Get name
		nameNode := findChildByType(decl, "simple_identifier")
		name := ""
		if nameNode != nil {
			name = e.nodeText(nameNode)
		}

		// Get type
		typeNode := findChildByType(decl, "type")
		typeName := ""
		if typeNode != nil {
			typeName = abbreviateKotlinType(e.nodeText(typeNode))
		}

		params = append(params, Param{Name: name, Type: typeName})
	}

	return params
}

// isExtensionFunction checks if a function is an extension function.
// Extension functions have a user_type followed by "." before the function name
func (e *KotlinExtractor) isExtensionFunction(node *sitter.Node) bool {
	// Look for pattern: user_type, ".", simple_identifier
	foundUserType := false
	foundDot := false

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "user_type" && !foundDot {
			foundUserType = true
		} else if childType == "." && foundUserType {
			foundDot = true
			return true
		} else if childType == "simple_identifier" && foundDot {
			return true
		}
	}

	return false
}

// extractReceiverType extracts the receiver type for extension functions.
func (e *KotlinExtractor) extractReceiverType(node *sitter.Node) string {
	// Look for the user_type node that appears before the "."
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "user_type" {
			// Check if next sibling is "."
			if i+1 < node.ChildCount() {
				nextChild := node.Child(int(i + 1))
				if nextChild.Type() == "." {
					// This is the receiver type
					typeIdNode := findChildByType(child, "type_identifier")
					if typeIdNode != nil {
						return abbreviateKotlinType(e.nodeText(typeIdNode))
					}
					return abbreviateKotlinType(e.nodeText(child))
				}
			}
		}
	}

	return ""
}

// isTopLevel checks if a node is at the top level (not inside a class/object/interface).
func (e *KotlinExtractor) isTopLevel(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "class_body":
			return false
		case "source_file":
			return true
		}
		parent = parent.Parent()
	}
	return true
}

// getFilePath returns the normalized file path.
func (e *KotlinExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *KotlinExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// isKotlinModifier checks if a node type is a Kotlin modifier keyword.
func isKotlinModifier(nodeType string) bool {
	modifiers := map[string]bool{
		"public":    true,
		"private":   true,
		"protected": true,
		"internal":  true,
		"open":      true,
		"final":     true,
		"abstract":  true,
		"override":  true,
		"inline":    true,
		"external":  true,
		"suspend":   true,
		"tailrec":   true,
		"operator":  true,
		"infix":     true,
	}
	return modifiers[nodeType]
}

// determineKotlinVisibility determines visibility from Kotlin modifiers.
func determineKotlinVisibility(modifiers []string) Visibility {
	for _, m := range modifiers {
		switch m {
		case "public":
			return VisibilityPublic
		case "private", "protected", "internal":
			return VisibilityPrivate
		}
	}
	// Default visibility in Kotlin is public
	return VisibilityPublic
}

// abbreviateKotlinType converts Kotlin types to abbreviated form.
func abbreviateKotlinType(kotlinType string) string {
	kotlinType = strings.TrimSpace(kotlinType)
	// Preserve generic types and nullable types
	return kotlinType
}

// formatKotlinMethodSignature formats a method signature for interface methods.
func formatKotlinMethodSignature(params []Param, returnType string) string {
	var sb strings.Builder

	sb.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')

	if returnType != "" && returnType != "Unit" {
		sb.WriteString(": ")
		sb.WriteString(returnType)
	}

	return sb.String()
}

// extractKotlinImportName extracts the simple name from an import path.
func extractKotlinImportName(importPath string) string {
	// Handle wildcard imports
	if strings.HasSuffix(importPath, ".*") {
		// Return package name for wildcard imports
		parts := strings.Split(strings.TrimSuffix(importPath, ".*"), ".")
		if len(parts) > 0 {
			return parts[len(parts)-1] + ".*"
		}
		return "*"
	}

	// Return simple class/function name
	parts := strings.Split(importPath, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return importPath
}

// contains checks if a slice contains a string.
func containsKotlin(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

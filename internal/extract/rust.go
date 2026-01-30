// Package extract provides Rust entity extraction from parsed AST trees.
package extract

import (
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// RustExtractor extracts code entities from a parsed Rust AST.
type RustExtractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewRustExtractor creates an extractor for the given Rust parse result.
func NewRustExtractor(result *parser.ParseResult) *RustExtractor {
	return &RustExtractor{
		result: result,
	}
}

// NewRustExtractorWithBase creates an extractor with a base path for relative paths.
func NewRustExtractorWithBase(result *parser.ParseResult, basePath string) *RustExtractor {
	return &RustExtractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the Rust AST.
// Returns functions, structs, impl blocks, traits, enums, constants, and imports.
func (e *RustExtractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions (standalone functions)
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

	// Extract impl blocks (methods)
	methods, err := e.ExtractImplBlocks()
	if err != nil {
		return nil, err
	}
	entities = append(entities, methods...)

	// Extract traits
	traits, err := e.ExtractTraits()
	if err != nil {
		return nil, err
	}
	entities = append(entities, traits...)

	// Extract enums
	enums, err := e.ExtractEnums()
	if err != nil {
		return nil, err
	}
	entities = append(entities, enums...)

	// Extract type aliases
	types, err := e.ExtractTypeAliases()
	if err != nil {
		return nil, err
	}
	entities = append(entities, types...)

	// Extract constants and statics
	consts, err := e.ExtractConstants()
	if err != nil {
		return nil, err
	}
	entities = append(entities, consts...)

	// Extract use statements (imports)
	imports, err := e.ExtractUseStatements()
	if err != nil {
		return nil, err
	}
	entities = append(entities, imports...)

	return entities, nil
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *RustExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract functions
	funcNodes := e.result.FindNodesByType("function_item")
	for _, node := range funcNodes {
		entity := e.extractFunction(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract structs
	structNodes := e.result.FindNodesByType("struct_item")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract impl blocks (methods)
	implNodes := e.result.FindNodesByType("impl_item")
	for _, node := range implNodes {
		methods := e.extractImplBlock(node)
		for i := range methods {
			// Find the method's AST node within the impl block
			methodNode := e.findMethodNode(node, methods[i].Name)
			result = append(result, EntityWithNode{Entity: &methods[i], Node: methodNode})
		}
	}

	// Extract traits
	traitNodes := e.result.FindNodesByType("trait_item")
	for _, node := range traitNodes {
		entity := e.extractTrait(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract enums
	enumNodes := e.result.FindNodesByType("enum_item")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract type aliases
	typeNodes := e.result.FindNodesByType("type_item")
	for _, node := range typeNodes {
		entity := e.extractTypeAlias(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract constants
	constNodes := e.result.FindNodesByType("const_item")
	for _, node := range constNodes {
		entity := e.extractConstItem(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract statics
	staticNodes := e.result.FindNodesByType("static_item")
	for _, node := range staticNodes {
		entity := e.extractStaticItem(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract use statements (imports)
	useNodes := e.result.FindNodesByType("use_declaration")
	for _, node := range useNodes {
		entities := e.extractUseDeclaration(node)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	return result, nil
}

// ExtractFunctions extracts all standalone function declarations.
func (e *RustExtractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	funcNodes := e.result.FindNodesByType("function_item")
	for _, node := range funcNodes {
		// Skip functions inside impl blocks (those are methods)
		if e.isInsideImplBlock(node) {
			continue
		}
		entity := e.extractFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractStructs extracts all struct definitions.
func (e *RustExtractor) ExtractStructs() ([]Entity, error) {
	var entities []Entity

	structNodes := e.result.FindNodesByType("struct_item")
	for _, node := range structNodes {
		entity := e.extractStruct(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractImplBlocks extracts all methods from impl blocks.
func (e *RustExtractor) ExtractImplBlocks() ([]Entity, error) {
	var entities []Entity

	implNodes := e.result.FindNodesByType("impl_item")
	for _, node := range implNodes {
		methods := e.extractImplBlock(node)
		entities = append(entities, methods...)
	}

	return entities, nil
}

// ExtractTraits extracts all trait definitions.
func (e *RustExtractor) ExtractTraits() ([]Entity, error) {
	var entities []Entity

	traitNodes := e.result.FindNodesByType("trait_item")
	for _, node := range traitNodes {
		entity := e.extractTrait(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractEnums extracts all enum definitions.
func (e *RustExtractor) ExtractEnums() ([]Entity, error) {
	var entities []Entity

	enumNodes := e.result.FindNodesByType("enum_item")
	for _, node := range enumNodes {
		entity := e.extractEnum(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractTypeAliases extracts all type alias definitions.
func (e *RustExtractor) ExtractTypeAliases() ([]Entity, error) {
	var entities []Entity

	typeNodes := e.result.FindNodesByType("type_item")
	for _, node := range typeNodes {
		entity := e.extractTypeAlias(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractConstants extracts all constant and static declarations.
func (e *RustExtractor) ExtractConstants() ([]Entity, error) {
	var entities []Entity

	// Extract const items
	constNodes := e.result.FindNodesByType("const_item")
	for _, node := range constNodes {
		entity := e.extractConstItem(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Extract static items
	staticNodes := e.result.FindNodesByType("static_item")
	for _, node := range staticNodes {
		entity := e.extractStaticItem(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractUseStatements extracts all use declarations (imports).
func (e *RustExtractor) ExtractUseStatements() ([]Entity, error) {
	var entities []Entity

	useNodes := e.result.FindNodesByType("use_declaration")
	for _, node := range useNodes {
		extracted := e.extractUseDeclaration(node)
		entities = append(entities, extracted...)
	}

	return entities, nil
}

// extractFunction extracts a function from its AST node.
func (e *RustExtractor) extractFunction(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "function_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get function name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get generic parameters
	generics := e.extractGenericParams(node)

	// Get parameters
	paramsNode := findRustChildByFieldName(node, "parameters")
	params := e.extractRustParameters(paramsNode)

	// Get return type
	returnNode := findRustChildByFieldName(node, "return_type")
	returns := e.extractRustReturnType(returnNode)

	// Get function body for hash computation
	bodyNode := findRustChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	// Check for async/const/unsafe modifiers
	modifiers := e.extractFunctionModifiers(node)

	startLine, endLine := getRustLineRange(node)

	entity := &Entity{
		Kind:       FunctionEntity,
		Name:       name + generics + modifiers,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    returns,
		RawBody:    rawBody,
		Visibility: visibility,
	}

	// Store just the name for display, with modifiers in a normalized form
	entity.Name = name

	entity.ComputeHashes()
	return entity
}

// extractStruct extracts a struct from its AST node.
func (e *RustExtractor) extractStruct(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "struct_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get struct name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get fields
	fields := e.extractStructFields(node)

	startLine, endLine := getRustLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   StructKind,
		Fields:     fields,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractImplBlock extracts methods from an impl block.
func (e *RustExtractor) extractImplBlock(node *sitter.Node) []Entity {
	if node == nil || node.Type() != "impl_item" {
		return nil
	}

	var entities []Entity

	// Get the type being implemented
	typeNode := findRustChildByFieldName(node, "type")
	receiverType := ""
	if typeNode != nil {
		receiverType = e.nodeText(typeNode)
	}

	// Check if this is a trait impl
	traitNode := findRustChildByFieldName(node, "trait")
	traitName := ""
	if traitNode != nil {
		traitName = e.nodeText(traitNode)
	}

	// Find the declaration_list (body of impl block)
	bodyNode := findRustChildByFieldName(node, "body")
	if bodyNode == nil {
		return nil
	}

	// Extract methods from the impl block
	for i := uint32(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(int(i))
		if child.Type() == "function_item" {
			entity := e.extractMethod(child, receiverType, traitName)
			if entity != nil {
				entities = append(entities, *entity)
			}
		}
	}

	return entities
}

// extractMethod extracts a method from within an impl block.
func (e *RustExtractor) extractMethod(node *sitter.Node, receiverType, traitName string) *Entity {
	if node == nil || node.Type() != "function_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get method name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters (including self)
	paramsNode := findRustChildByFieldName(node, "parameters")
	params, selfParam := e.extractRustMethodParameters(paramsNode)

	// Get return type
	returnNode := findRustChildByFieldName(node, "return_type")
	returns := e.extractRustReturnType(returnNode)

	// Get method body for hash computation
	bodyNode := findRustChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	startLine, endLine := getRustLineRange(node)

	// Build receiver string (like Go's receiver)
	receiver := receiverType
	if selfParam != "" {
		// Include self parameter type in receiver
		receiver = selfParam + " " + receiverType
	}
	if traitName != "" {
		receiver = receiverType + " for " + traitName
	}

	entity := &Entity{
		Kind:       MethodEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		Params:     params,
		Returns:    returns,
		Receiver:   receiver,
		RawBody:    rawBody,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractTrait extracts a trait definition.
func (e *RustExtractor) extractTrait(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "trait_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get trait name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get trait methods as fields
	fields := e.extractTraitMethods(node)

	startLine, endLine := getRustLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   InterfaceKind, // Traits are like interfaces
		Fields:     fields,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractEnum extracts an enum definition.
func (e *RustExtractor) extractEnum(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "enum_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get enum name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get enum variants
	enumValues := e.extractEnumVariants(node)

	startLine, endLine := getRustLineRange(node)

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

// extractTypeAlias extracts a type alias definition.
func (e *RustExtractor) extractTypeAlias(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "type_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get type name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get the aliased type
	typeNode := findRustChildByFieldName(node, "type")
	aliasedType := ""
	if typeNode != nil {
		aliasedType = e.nodeText(typeNode)
	}

	startLine, endLine := getRustLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   AliasKind,
		ValueType:  aliasedType,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractConstItem extracts a const declaration.
func (e *RustExtractor) extractConstItem(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "const_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get const name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type
	typeNode := findRustChildByFieldName(node, "type")
	typeName := ""
	if typeNode != nil {
		typeName = e.nodeText(typeNode)
	}

	// Get value
	valueNode := findRustChildByFieldName(node, "value")
	value := ""
	if valueNode != nil {
		value = e.nodeText(valueNode)
		if len(value) > 50 {
			value = value[:47] + "..."
		}
	}

	startLine, _ := getRustLineRange(node)

	entity := &Entity{
		Kind:       ConstEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		ValueType:  typeName,
		Value:      value,
		Visibility: visibility,
	}

	entity.ComputeHashes()
	return entity
}

// extractStaticItem extracts a static declaration.
func (e *RustExtractor) extractStaticItem(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "static_item" {
		return nil
	}

	// Get visibility
	visibility := e.extractVisibility(node)

	// Get static name
	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get type
	typeNode := findRustChildByFieldName(node, "type")
	typeName := ""
	if typeNode != nil {
		typeName = e.nodeText(typeNode)
	}

	// Get value
	valueNode := findRustChildByFieldName(node, "value")
	value := ""
	if valueNode != nil {
		value = e.nodeText(valueNode)
		if len(value) > 50 {
			value = value[:47] + "..."
		}
	}

	// Check for mut keyword
	isMut := false
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if e.nodeText(child) == "mut" {
			isMut = true
			break
		}
	}

	startLine, _ := getRustLineRange(node)

	entity := &Entity{
		Kind:       VarEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    startLine,
		ValueType:  typeName,
		Value:      value,
		Visibility: visibility,
	}

	// Annotate mutable statics
	if isMut {
		entity.ValueType = "mut " + entity.ValueType
	}

	entity.ComputeHashes()
	return entity
}

// extractUseDeclaration extracts use statements (imports).
func (e *RustExtractor) extractUseDeclaration(node *sitter.Node) []Entity {
	if node == nil || node.Type() != "use_declaration" {
		return nil
	}

	var entities []Entity

	// Get visibility
	visibility := e.extractVisibility(node)

	startLine, _ := getRustLineRange(node)

	// Find the use tree (can be simple path, wildcard, or group)
	var useTree *sitter.Node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()
		if childType == "use_tree" || childType == "scoped_identifier" ||
			childType == "identifier" || childType == "use_wildcard" ||
			childType == "use_list" || childType == "use_as_clause" {
			useTree = child
			break
		}
	}

	if useTree == nil {
		// Try to get the whole path text
		pathText := e.nodeText(node)
		pathText = strings.TrimPrefix(pathText, "use ")
		pathText = strings.TrimSuffix(pathText, ";")
		pathText = strings.TrimSpace(pathText)

		if pathText != "" {
			entity := Entity{
				Kind:       ImportEntity,
				Name:       extractRustImportName(pathText),
				File:       e.getFilePath(),
				StartLine:  startLine,
				EndLine:    startLine,
				ImportPath: pathText,
				Visibility: visibility,
			}
			entities = append(entities, entity)
		}
		return entities
	}

	// Extract imports from the use tree
	imports := e.extractUseTree(useTree, "")
	for _, imp := range imports {
		entity := Entity{
			Kind:        ImportEntity,
			Name:        extractRustImportName(imp.path),
			File:        e.getFilePath(),
			StartLine:   startLine,
			EndLine:     startLine,
			ImportPath:  imp.path,
			ImportAlias: imp.alias,
			Visibility:  visibility,
		}
		entities = append(entities, entity)
	}

	return entities
}

// useImport represents a single import from a use tree.
type useImport struct {
	path  string
	alias string
}

// extractUseTree recursively extracts imports from a use tree.
func (e *RustExtractor) extractUseTree(node *sitter.Node, prefix string) []useImport {
	if node == nil {
		return nil
	}

	var imports []useImport
	nodeType := node.Type()

	switch nodeType {
	case "identifier", "scoped_identifier":
		// Simple path
		path := e.nodeText(node)
		if prefix != "" {
			path = prefix + "::" + path
		}
		imports = append(imports, useImport{path: path})

	case "use_as_clause":
		// path as alias
		pathNode := node.ChildByFieldName("path")
		aliasNode := node.ChildByFieldName("alias")
		path := ""
		alias := ""
		if pathNode != nil {
			path = e.nodeText(pathNode)
		}
		if aliasNode != nil {
			alias = e.nodeText(aliasNode)
		}
		if prefix != "" {
			path = prefix + "::" + path
		}
		imports = append(imports, useImport{path: path, alias: alias})

	case "use_wildcard":
		// path::*
		path := prefix
		if path == "" {
			path = "*"
		} else {
			path = path + "::*"
		}
		imports = append(imports, useImport{path: path})

	case "use_list":
		// path::{a, b, c}
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			childType := child.Type()
			if childType == "identifier" || childType == "use_as_clause" ||
				childType == "scoped_identifier" || childType == "use_wildcard" ||
				childType == "scoped_use_list" {
				subImports := e.extractUseTree(child, prefix)
				imports = append(imports, subImports...)
			}
		}

	case "scoped_use_list":
		// Get the path prefix
		pathNode := node.ChildByFieldName("path")
		newPrefix := prefix
		if pathNode != nil {
			pathText := e.nodeText(pathNode)
			if newPrefix != "" {
				newPrefix = newPrefix + "::" + pathText
			} else {
				newPrefix = pathText
			}
		}
		// Get the list
		listNode := node.ChildByFieldName("list")
		if listNode != nil {
			subImports := e.extractUseTree(listNode, newPrefix)
			imports = append(imports, subImports...)
		}

	case "use_tree":
		// Generic use tree - check children
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			subImports := e.extractUseTree(child, prefix)
			imports = append(imports, subImports...)
		}

	default:
		// Try to get text for unhandled types
		text := e.nodeText(node)
		if text != "" && !strings.Contains(text, "{") {
			path := text
			if prefix != "" {
				path = prefix + "::" + path
			}
			imports = append(imports, useImport{path: path})
		}
	}

	return imports
}

// Helper functions

// extractVisibility extracts visibility modifier from a node.
func (e *RustExtractor) extractVisibility(node *sitter.Node) Visibility {
	if node == nil {
		return VisibilityPrivate
	}

	// Look for visibility_modifier child
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "visibility_modifier" {
			visText := e.nodeText(child)
			if visText == "pub" {
				return VisibilityPublic
			}
			// pub(crate), pub(super), etc. are considered "internal"
			// but we normalize to "public" for simplicity since they allow some visibility
			if strings.HasPrefix(visText, "pub") {
				return VisibilityPublic
			}
		}
	}

	return VisibilityPrivate
}

// extractGenericParams extracts generic parameters from a node.
func (e *RustExtractor) extractGenericParams(node *sitter.Node) string {
	typeParams := findRustChildByFieldName(node, "type_parameters")
	if typeParams == nil {
		return ""
	}
	return e.nodeText(typeParams)
}

// extractFunctionModifiers extracts async/const/unsafe modifiers.
func (e *RustExtractor) extractFunctionModifiers(node *sitter.Node) string {
	var modifiers []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		text := e.nodeText(child)
		switch text {
		case "async", "const", "unsafe", "extern":
			modifiers = append(modifiers, text)
		}
	}

	if len(modifiers) == 0 {
		return ""
	}
	return " [" + strings.Join(modifiers, " ") + "]"
}

// extractRustParameters extracts parameters from a function parameter list.
func (e *RustExtractor) extractRustParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "parameter" {
			param := e.extractParameter(child)
			if param != nil {
				params = append(params, *param)
			}
		}
	}

	return params
}

// extractRustMethodParameters extracts parameters from a method, returning self param separately.
func (e *RustExtractor) extractRustMethodParameters(node *sitter.Node) ([]Param, string) {
	if node == nil {
		return nil, ""
	}

	var params []Param
	selfParam := ""

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		switch childType {
		case "self_parameter":
			// Handle self, &self, &mut self
			selfParam = e.nodeText(child)
		case "parameter":
			param := e.extractParameter(child)
			if param != nil {
				params = append(params, *param)
			}
		}
	}

	return params, selfParam
}

// extractParameter extracts a single parameter.
func (e *RustExtractor) extractParameter(node *sitter.Node) *Param {
	if node == nil {
		return nil
	}

	var name string
	var typeName string

	// Get pattern (name)
	patternNode := findRustChildByFieldName(node, "pattern")
	if patternNode != nil {
		name = e.nodeText(patternNode)
	}

	// Get type
	typeNode := findRustChildByFieldName(node, "type")
	if typeNode != nil {
		typeName = e.nodeText(typeNode)
	}

	if typeName == "" {
		return nil
	}

	return &Param{
		Name: name,
		Type: typeName,
	}
}

// extractRustReturnType extracts the return type from a function.
func (e *RustExtractor) extractRustReturnType(node *sitter.Node) []string {
	if node == nil {
		return nil
	}

	// The return type node contains the type directly
	typeText := e.nodeText(node)
	typeText = strings.TrimPrefix(typeText, "-> ")
	typeText = strings.TrimSpace(typeText)

	if typeText == "" {
		return nil
	}

	return []string{typeText}
}

// extractStructFields extracts fields from a struct.
func (e *RustExtractor) extractStructFields(node *sitter.Node) []Field {
	if node == nil {
		return nil
	}

	var fields []Field

	// Find field_declaration_list
	var fieldList *sitter.Node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "field_declaration_list" {
			fieldList = child
			break
		}
	}

	if fieldList == nil {
		// Could be a tuple struct
		return fields
	}

	// Extract fields
	for i := uint32(0); i < fieldList.ChildCount(); i++ {
		child := fieldList.Child(int(i))
		if child.Type() == "field_declaration" {
			field := e.extractFieldDeclaration(child)
			if field != nil {
				fields = append(fields, *field)
			}
		}
	}

	return fields
}

// extractFieldDeclaration extracts a single field declaration.
func (e *RustExtractor) extractFieldDeclaration(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	var name string
	var typeName string

	nameNode := findRustChildByFieldName(node, "name")
	if nameNode != nil {
		name = e.nodeText(nameNode)
	}

	typeNode := findRustChildByFieldName(node, "type")
	if typeNode != nil {
		typeName = e.nodeText(typeNode)
	}

	if name == "" && typeName == "" {
		return nil
	}

	return &Field{
		Name: name,
		Type: typeName,
	}
}

// extractTraitMethods extracts method signatures from a trait.
func (e *RustExtractor) extractTraitMethods(node *sitter.Node) []Field {
	if node == nil {
		return nil
	}

	var methods []Field

	// Find declaration_list (trait body)
	var body *sitter.Node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "declaration_list" {
			body = child
			break
		}
	}

	if body == nil {
		return methods
	}

	// Extract function signatures
	for i := uint32(0); i < body.ChildCount(); i++ {
		child := body.Child(int(i))
		if child.Type() == "function_signature_item" || child.Type() == "function_item" {
			method := e.extractTraitMethod(child)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// extractTraitMethod extracts a method signature from a trait.
func (e *RustExtractor) extractTraitMethod(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Build signature
	paramsNode := findRustChildByFieldName(node, "parameters")
	params := e.extractRustParameters(paramsNode)

	returnNode := findRustChildByFieldName(node, "return_type")
	returns := e.extractRustReturnType(returnNode)

	sig := formatRustMethodSignature(params, returns)

	return &Field{
		Name: name,
		Type: sig,
	}
}

// extractEnumVariants extracts variants from an enum.
func (e *RustExtractor) extractEnumVariants(node *sitter.Node) []EnumValue {
	if node == nil {
		return nil
	}

	var variants []EnumValue

	// Find enum_variant_list
	var variantList *sitter.Node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "enum_variant_list" {
			variantList = child
			break
		}
	}

	if variantList == nil {
		return variants
	}

	// Extract variants
	for i := uint32(0); i < variantList.ChildCount(); i++ {
		child := variantList.Child(int(i))
		if child.Type() == "enum_variant" {
			variant := e.extractEnumVariant(child)
			if variant != nil {
				variants = append(variants, *variant)
			}
		}
	}

	return variants
}

// extractEnumVariant extracts a single enum variant.
func (e *RustExtractor) extractEnumVariant(node *sitter.Node) *EnumValue {
	if node == nil {
		return nil
	}

	nameNode := findRustChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get variant value if discriminant
	value := ""
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "integer_literal" {
			value = e.nodeText(child)
			break
		}
	}

	// Check for tuple or struct variant fields
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()
		if childType == "field_declaration_list" {
			// Struct variant
			value = "{...}"
			break
		} else if childType == "ordered_field_declaration_list" {
			// Tuple variant
			value = "(...)"
			break
		}
	}

	return &EnumValue{
		Name:  name,
		Value: value,
	}
}

// isInsideImplBlock checks if a function node is inside an impl block.
func (e *RustExtractor) isInsideImplBlock(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "impl_item" {
			return true
		}
		if parent.Type() == "declaration_list" {
			// Check if parent of declaration_list is impl_item
			grandParent := parent.Parent()
			if grandParent != nil && grandParent.Type() == "impl_item" {
				return true
			}
		}
		parent = parent.Parent()
	}
	return false
}

// findMethodNode finds the method node with the given name within an impl block.
func (e *RustExtractor) findMethodNode(implNode *sitter.Node, methodName string) *sitter.Node {
	if implNode == nil {
		return nil
	}

	bodyNode := findRustChildByFieldName(implNode, "body")
	if bodyNode == nil {
		return nil
	}

	for i := uint32(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(int(i))
		if child.Type() == "function_item" {
			nameNode := findRustChildByFieldName(child, "name")
			if nameNode != nil && e.nodeText(nameNode) == methodName {
				return child
			}
		}
	}

	return nil
}

// getFilePath returns the normalized file path.
func (e *RustExtractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *RustExtractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// Helper functions for Rust AST traversal

// findRustChildByFieldName finds the child node with the given field name.
func findRustChildByFieldName(node *sitter.Node, fieldName string) *sitter.Node {
	return node.ChildByFieldName(fieldName)
}

// getRustLineRange returns the start and end line numbers for a node.
func getRustLineRange(node *sitter.Node) (uint32, uint32) {
	// tree-sitter lines are 0-based, we want 1-based
	start := node.StartPoint().Row + 1
	end := node.EndPoint().Row + 1
	return start, end
}

// extractRustImportName extracts the last component of an import path.
func extractRustImportName(path string) string {
	// Remove any alias
	if idx := strings.Index(path, " as "); idx > 0 {
		path = path[:idx]
	}

	// Handle wildcards
	if strings.HasSuffix(path, "::*") {
		path = strings.TrimSuffix(path, "::*")
	}

	// Get last component
	parts := strings.Split(path, "::")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// formatRustMethodSignature formats a method signature for trait methods.
func formatRustMethodSignature(params []Param, returns []string) string {
	var sb strings.Builder

	sb.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Name != "" {
			sb.WriteString(p.Name)
			sb.WriteString(": ")
		}
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')

	if len(returns) > 0 {
		sb.WriteString(" -> ")
		sb.WriteString(strings.Join(returns, ", "))
	}

	return sb.String()
}

// DetermineRustVisibility determines visibility from Rust visibility modifiers.
// In Rust, items without visibility modifier are private.
func DetermineRustVisibility(hasPublicModifier bool) Visibility {
	if hasPublicModifier {
		return VisibilityPublic
	}
	return VisibilityPrivate
}

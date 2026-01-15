package extract

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// extractType extracts a type definition from a type_spec node.
func (e *Extractor) extractType(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "type_spec" {
		return nil
	}

	// Get type name from identifier
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get the type definition
	typeNode := findChildByFieldName(node, "type")
	if typeNode == nil {
		return nil
	}

	// Determine the kind of type
	typeKind, fields := e.extractTypeDetails(typeNode)

	// Get preceding doc comment - look at parent type_declaration for comment
	docComment := ""
	if node.Parent() != nil && node.Parent().Type() == "type_declaration" {
		docComment = e.extractPrecedingComment(node.Parent())
	}
	if docComment == "" {
		docComment = e.extractPrecedingComment(node)
	}

	startLine, endLine := getLineRange(node)

	entity := &Entity{
		Kind:       TypeEntity,
		Name:       name,
		File:       e.getFilePath(),
		StartLine:  startLine,
		EndLine:    endLine,
		TypeKind:   typeKind,
		Fields:     fields,
		DocComment: docComment,
		Visibility: DetermineVisibility(name),
	}

	entity.ComputeHashes()
	entity.Skeleton = entity.BuildSkeleton()
	return entity
}

// extractTypeDetails determines type kind and extracts fields.
func (e *Extractor) extractTypeDetails(node *sitter.Node) (TypeKind, []Field) {
	if node == nil {
		return AliasKind, nil
	}

	switch node.Type() {
	case "struct_type":
		fields := e.extractStructFields(node)
		return StructKind, fields

	case "interface_type":
		methods := e.extractInterfaceMethods(node)
		return InterfaceKind, methods

	default:
		// Type alias or simple type definition
		return AliasKind, nil
	}
}

// extractStructFields extracts fields from a struct_type node.
func (e *Extractor) extractStructFields(node *sitter.Node) []Field {
	if node == nil || node.Type() != "struct_type" {
		return nil
	}

	var fields []Field

	// Find field_declaration_list
	fieldList := findChildByType(node, "field_declaration_list")
	if fieldList == nil {
		return nil
	}

	// Iterate over field_declaration nodes
	fieldDecls := findChildrenByType(fieldList, "field_declaration")
	for _, decl := range fieldDecls {
		extracted := e.extractFieldDeclaration(decl)
		fields = append(fields, extracted...)
	}

	return fields
}

// extractFieldDeclaration extracts fields from a field_declaration node.
func (e *Extractor) extractFieldDeclaration(node *sitter.Node) []Field {
	if node == nil {
		return nil
	}

	var fields []Field
	var names []string
	var typeNode *sitter.Node
	isPointer := false

	// Iterate through children to find names and type
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "field_identifier" {
			names = append(names, e.nodeText(child))
		} else if childType == "*" {
			// Pointer marker for embedded pointer types (e.g., *Logger)
			isPointer = true
		} else if isTypeNode(childType) {
			typeNode = child
		} else if childType == "type_identifier" {
			typeNode = child
		}
	}

	// Handle embedded fields (no field_identifier, just a type)
	if len(names) == 0 && typeNode != nil {
		typeName := e.nodeText(typeNode)
		if isPointer {
			typeName = "*" + typeName
		}
		abbrevType := abbreviateType(typeName)
		fieldName := extractEmbeddedName(typeName)
		fields = append(fields, Field{Name: fieldName, Type: abbrevType})
		return fields
	}

	// Handle regular fields
	if typeNode == nil {
		return nil
	}

	typeName := abbreviateType(e.nodeText(typeNode))

	// Create field for each name
	for _, name := range names {
		fields = append(fields, Field{Name: name, Type: typeName})
	}

	return fields
}

// extractEmbeddedName extracts the field name from an embedded type.
// For "pkg.Type" returns "Type", for "*Type" returns "Type".
func extractEmbeddedName(typeName string) string {
	// Remove pointer
	name := strings.TrimPrefix(typeName, "*")

	// Get last part of qualified name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}

	return name
}

// extractInterfaceMethods extracts methods from an interface_type node.
func (e *Extractor) extractInterfaceMethods(node *sitter.Node) []Field {
	if node == nil || node.Type() != "interface_type" {
		return nil
	}

	var methods []Field

	// Interface methods are in method_elem nodes (tree-sitter-go uses "method_elem")
	methodElems := findNodesByTypeRecursive(node, "method_elem")
	for _, elem := range methodElems {
		method := e.extractMethodElem(elem)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Also try method_spec for older tree-sitter versions
	methodSpecs := findNodesByTypeRecursive(node, "method_spec")
	for _, spec := range methodSpecs {
		method := e.extractMethodSpec(spec)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Also look for embedded interfaces (direct type_identifier children)
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" {
			name := e.nodeText(child)
			methods = append(methods, Field{Name: name, Type: "interface"})
		}
	}

	return methods
}

// extractMethodElem extracts a method signature from a method_elem node.
// tree-sitter-go uses method_elem for interface methods.
// Structure: method_elem -> field_identifier, parameter_list, type_identifier (return)
func (e *Extractor) extractMethodElem(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	var name string
	var params []Param
	var returns []string

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		switch child.Type() {
		case "field_identifier":
			name = e.nodeText(child)
		case "parameter_list":
			params = e.extractParameters(child)
		case "type_identifier":
			// Single return type
			returns = append(returns, abbreviateType(e.nodeText(child)))
		case "pointer_type", "slice_type", "map_type", "qualified_type":
			// Return type
			returns = append(returns, abbreviateType(e.nodeText(child)))
		}
	}

	if name == "" {
		return nil
	}

	// Build method signature
	sig := formatMethodSignature(params, returns)

	return &Field{
		Name: name,
		Type: sig,
	}
}

// extractMethodSpec extracts a method signature from a method_spec node.
func (e *Extractor) extractMethodSpec(node *sitter.Node) *Field {
	if node == nil {
		return nil
	}

	// Get method name
	nameNode := findChildByFieldName(node, "name")
	if nameNode == nil {
		return nil
	}
	name := e.nodeText(nameNode)

	// Get parameters
	paramsNode := findChildByFieldName(node, "parameters")
	params := e.extractParameters(paramsNode)

	// Get return types
	resultNode := findChildByFieldName(node, "result")
	returns := e.extractReturnTypes(resultNode)

	// Build method signature
	sig := formatMethodSignature(params, returns)

	return &Field{
		Name: name,
		Type: sig,
	}
}

// formatMethodSignature formats a method signature for interface methods.
// Example: (string) -> error
func formatMethodSignature(params []Param, returns []string) string {
	var sb strings.Builder

	sb.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')

	if len(returns) > 0 {
		sb.WriteString(" -> ")
		if len(returns) == 1 {
			sb.WriteString(returns[0])
		} else {
			sb.WriteByte('(')
			sb.WriteString(strings.Join(returns, ", "))
			sb.WriteByte(')')
		}
	}

	return sb.String()
}

// extractConstOrVar extracts constants or variables from a const_spec/var_spec node.
func (e *Extractor) extractConstOrVar(node *sitter.Node, kind EntityKind) []Entity {
	if node == nil {
		return nil
	}

	var entities []Entity
	var names []string
	var typeNode *sitter.Node
	var valueNode *sitter.Node

	// Find names, type, and value
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "identifier" {
			names = append(names, e.nodeText(child))
		} else if isTypeNode(childType) {
			typeNode = child
		} else if childType == "expression_list" || isExpressionNode(childType) {
			valueNode = child
		}
	}

	// Get preceding doc comment - check parent const/var declaration
	docComment := ""
	if node.Parent() != nil {
		parentType := node.Parent().Type()
		if parentType == "const_declaration" || parentType == "var_declaration" {
			docComment = e.extractPrecedingComment(node.Parent())
		}
	}
	if docComment == "" {
		docComment = e.extractPrecedingComment(node)
	}

	startLine, _ := getLineRange(node)

	// Get type string (might be inferred)
	typeName := ""
	if typeNode != nil {
		typeName = abbreviateType(e.nodeText(typeNode))
	}

	// Get value string
	value := ""
	if valueNode != nil {
		value = e.nodeText(valueNode)
		// Truncate long values
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		// Clean up string values
		value = strings.ReplaceAll(value, "\n", "")
		value = strings.TrimSpace(value)
	}

	// Create entity for each name
	for _, name := range names {
		entity := Entity{
			Kind:       kind,
			Name:       name,
			File:       e.getFilePath(),
			StartLine:  startLine,
			EndLine:    startLine,
			ValueType:  typeName,
			Value:      value,
			DocComment: docComment,
			Visibility: DetermineVisibility(name),
		}
		entity.ComputeHashes()
		entity.Skeleton = entity.BuildSkeleton()
		entities = append(entities, entity)
	}

	return entities
}

// extractImport extracts an import from an import_spec node.
func (e *Extractor) extractImport(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "import_spec" {
		return nil
	}

	var alias string
	var importPath string

	// Find name (alias) and path
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		childType := child.Type()

		if childType == "package_identifier" || childType == "blank_identifier" || childType == "dot" {
			alias = e.nodeText(child)
		} else if childType == "interpreted_string_literal" {
			// Remove quotes from path
			path := e.nodeText(child)
			importPath = strings.Trim(path, "\"")
		}
	}

	if importPath == "" {
		return nil
	}

	startLine, _ := getLineRange(node)

	entity := &Entity{
		Kind:        ImportEntity,
		Name:        extractImportName(importPath),
		File:        e.getFilePath(),
		StartLine:   startLine,
		EndLine:     startLine,
		ImportPath:  importPath,
		ImportAlias: alias,
	}
	entity.Skeleton = entity.BuildSkeleton()
	return entity
}

// extractImportName extracts the package name from an import path.
func extractImportName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// isExpressionNode checks if a node type represents an expression.
func isExpressionNode(nodeType string) bool {
	exprNodes := map[string]bool{
		"identifier":                  true,
		"int_literal":                 true,
		"float_literal":               true,
		"imaginary_literal":           true,
		"rune_literal":                true,
		"interpreted_string_literal":  true,
		"raw_string_literal":          true,
		"true":                        true,
		"false":                       true,
		"nil":                         true,
		"binary_expression":           true,
		"unary_expression":            true,
		"call_expression":             true,
		"selector_expression":         true,
		"index_expression":            true,
		"slice_expression":            true,
		"type_assertion_expression":   true,
		"type_conversion_expression":  true,
		"composite_literal":           true,
		"func_literal":                true,
		"parenthesized_expression":    true,
	}
	return exprNodes[nodeType]
}

// findNodesByTypeRecursive finds all nodes of a type within a subtree.
func findNodesByTypeRecursive(root *sitter.Node, nodeType string) []*sitter.Node {
	var result []*sitter.Node

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Type() == nodeType {
			result = append(result, node)
		}
		for i := uint32(0); i < node.ChildCount(); i++ {
			walk(node.Child(int(i)))
		}
	}

	walk(root)
	return result
}

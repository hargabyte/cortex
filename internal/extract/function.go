package extract

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// extractFunction extracts a function declaration from its AST node.
func (e *Extractor) extractFunction(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "function_declaration" {
		return nil
	}

	// Get function name from identifier
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

	// Get function body for hash computation
	bodyNode := findChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	// Get preceding doc comment
	docComment := e.extractPrecedingComment(node)

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
		DocComment: docComment,
		Visibility: DetermineVisibility(name),
	}

	entity.ComputeHashes()
	entity.Skeleton = entity.BuildSkeleton()
	return entity
}

// extractMethod extracts a method declaration from its AST node.
func (e *Extractor) extractMethod(node *sitter.Node) *Entity {
	if node == nil || node.Type() != "method_declaration" {
		return nil
	}

	// Get receiver
	receiverNode := findChildByFieldName(node, "receiver")
	receiver := e.extractReceiver(receiverNode)

	// Get method name from identifier
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

	// Get method body for hash computation
	bodyNode := findChildByFieldName(node, "body")
	rawBody := ""
	if bodyNode != nil {
		rawBody = e.nodeText(bodyNode)
	}

	// Get preceding doc comment
	docComment := e.extractPrecedingComment(node)

	startLine, endLine := getLineRange(node)

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
		DocComment: docComment,
		Visibility: DetermineVisibility(name),
	}

	entity.ComputeHashes()
	entity.Skeleton = entity.BuildSkeleton()
	return entity
}

// extractPrecedingComment extracts the doc comment that immediately precedes a node.
// In Go, doc comments are comment blocks that appear directly before a declaration
// with no blank lines in between.
func (e *Extractor) extractPrecedingComment(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Get the previous sibling to look for comments
	var comments []string
	prevSibling := node.PrevSibling()

	for prevSibling != nil {
		nodeType := prevSibling.Type()

		// Check if it's a comment
		if nodeType == "comment" {
			commentText := e.nodeText(prevSibling)
			// Prepend to maintain order (we're going backwards)
			comments = append([]string{commentText}, comments...)
			prevSibling = prevSibling.PrevSibling()
			continue
		}

		// If we hit a non-comment node, check if it's just whitespace/newlines
		// If it's an actual code node, stop looking for comments
		if nodeType != "\n" && nodeType != "" {
			break
		}

		prevSibling = prevSibling.PrevSibling()
	}

	if len(comments) == 0 {
		return ""
	}

	return strings.Join(comments, "\n")
}

// extractReceiver extracts the receiver type from a parameter_list.
func (e *Extractor) extractReceiver(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	// Receiver is inside a parameter_list with one parameter_declaration
	paramDecl := findChildByType(node, "parameter_declaration")
	if paramDecl == nil {
		return ""
	}

	// Get the type from the parameter declaration
	typeNode := findChildByFieldName(paramDecl, "type")
	if typeNode == nil {
		// Try getting the second child (name, type pattern)
		for i := uint32(0); i < paramDecl.ChildCount(); i++ {
			child := paramDecl.Child(int(i))
			if child.Type() == "pointer_type" || child.Type() == "type_identifier" {
				typeNode = child
				break
			}
		}
	}

	if typeNode == nil {
		return ""
	}

	return abbreviateType(e.nodeText(typeNode))
}

// extractParameters extracts parameters from a parameter_list node.
func (e *Extractor) extractParameters(node *sitter.Node) []Param {
	if node == nil {
		return nil
	}

	var params []Param
	paramDecls := findChildrenByType(node, "parameter_declaration")

	for _, decl := range paramDecls {
		// Get parameter names (can be multiple: x, y int)
		var names []string
		var typeNode *sitter.Node

		for i := uint32(0); i < decl.ChildCount(); i++ {
			child := decl.Child(int(i))
			childType := child.Type()

			if childType == "identifier" {
				names = append(names, e.nodeText(child))
			} else if isTypeNode(childType) {
				typeNode = child
			}
		}

		if typeNode == nil {
			continue
		}

		typeName := abbreviateType(e.nodeText(typeNode))

		// If no names (just type), add single param without name
		if len(names) == 0 {
			params = append(params, Param{Type: typeName})
		} else {
			// Create param for each name with the same type
			for _, name := range names {
				params = append(params, Param{Name: name, Type: typeName})
			}
		}
	}

	// Handle variadic parameters
	variadicDecls := findChildrenByType(node, "variadic_parameter_declaration")
	for _, decl := range variadicDecls {
		var name string
		var typeName string

		for i := uint32(0); i < decl.ChildCount(); i++ {
			child := decl.Child(int(i))
			childType := child.Type()

			if childType == "identifier" {
				name = e.nodeText(child)
			} else if isTypeNode(childType) {
				typeName = "..." + abbreviateType(e.nodeText(child))
			}
		}

		if typeName != "" {
			params = append(params, Param{Name: name, Type: typeName})
		}
	}

	return params
}

// extractReturnTypes extracts return types from a result node.
func (e *Extractor) extractReturnTypes(node *sitter.Node) []string {
	if node == nil {
		return nil
	}

	var returns []string

	// Single return type (not in parentheses)
	if isTypeNode(node.Type()) {
		returns = append(returns, abbreviateType(e.nodeText(node)))
		return returns
	}

	// Multiple return types in parameter_list
	if node.Type() == "parameter_list" {
		paramDecls := findChildrenByType(node, "parameter_declaration")
		for _, decl := range paramDecls {
			// Find type node in parameter declaration
			for i := uint32(0); i < decl.ChildCount(); i++ {
				child := decl.Child(int(i))
				if isTypeNode(child.Type()) {
					returns = append(returns, abbreviateType(e.nodeText(child)))
				}
			}
		}
	}

	return returns
}

// isTypeNode checks if a node type represents a Go type.
func isTypeNode(nodeType string) bool {
	typeNodes := map[string]bool{
		"type_identifier":    true,
		"pointer_type":       true,
		"slice_type":         true,
		"array_type":         true,
		"map_type":           true,
		"channel_type":       true,
		"function_type":      true,
		"interface_type":     true,
		"struct_type":        true,
		"qualified_type":     true,
		"generic_type":       true,
		"parenthesized_type": true,
	}
	return typeNodes[nodeType]
}

// abbreviateType converts Go types to full form.
// This no longer abbreviates types - it returns the full type names.
// Examples:
//
//	string -> string
//	int64 -> int64
//	error -> error
//	context.Context -> context.Context
//	[]byte -> []byte
//	map[K]V -> map[K]V
func abbreviateType(goType string) string {
	goType = strings.TrimSpace(goType)

	// Handle pointer types: *T -> *T (preserve inner type)
	if strings.HasPrefix(goType, "*") {
		inner := abbreviateType(goType[1:])
		return "*" + inner
	}

	// Handle slice types: []T -> []T (preserve inner type)
	if strings.HasPrefix(goType, "[]") {
		inner := abbreviateType(goType[2:])
		return "[]" + inner
	}

	// Handle map types: map[K]V -> map[K]V (preserve types)
	if strings.HasPrefix(goType, "map[") {
		// Find the closing bracket
		depth := 0
		keyEnd := -1
		for i := 4; i < len(goType); i++ {
			if goType[i] == '[' {
				depth++
			} else if goType[i] == ']' {
				if depth == 0 {
					keyEnd = i
					break
				}
				depth--
			}
		}
		if keyEnd > 4 && keyEnd < len(goType)-1 {
			keyType := abbreviateType(goType[4:keyEnd])
			valType := abbreviateType(goType[keyEnd+1:])
			return "map[" + keyType + "]" + valType
		}
	}

	// Handle channel types: chan T -> chan T, <-chan T -> <-chan T
	if strings.HasPrefix(goType, "chan ") {
		inner := abbreviateType(goType[5:])
		return "chan " + inner
	}
	if strings.HasPrefix(goType, "<-chan ") {
		inner := abbreviateType(goType[7:])
		return "<-chan " + inner
	}
	if strings.HasPrefix(goType, "chan<- ") {
		inner := abbreviateType(goType[7:])
		return "chan<- " + inner
	}

	// Handle variadic types: ...T -> ...T (preserve inner type)
	if strings.HasPrefix(goType, "...") {
		inner := abbreviateType(goType[3:])
		return "..." + inner
	}

	// Return the type as-is (full name, not abbreviated)
	return goType
}

// formatSignature creates the signature string.
// Example: (email: string, password: string) -> (*User, error)
func formatSignature(params []Param, returns []string) string {
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

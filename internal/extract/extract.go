package extract

import (
	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// Extractor extracts code entities from a parsed AST.
type Extractor struct {
	result   *parser.ParseResult
	basePath string
}

// NewExtractor creates an extractor for the given parse result.
func NewExtractor(result *parser.ParseResult) *Extractor {
	return &Extractor{
		result: result,
	}
}

// NewExtractorWithBase creates an extractor with a base path for relative paths.
func NewExtractorWithBase(result *parser.ParseResult, basePath string) *Extractor {
	return &Extractor{
		result:   result,
		basePath: basePath,
	}
}

// ExtractAll extracts all entities from the AST.
// Returns functions, methods, types, constants, variables, and imports.
func (e *Extractor) ExtractAll() ([]Entity, error) {
	var entities []Entity

	// Extract functions and methods
	funcs, err := e.ExtractFunctions()
	if err != nil {
		return nil, err
	}
	entities = append(entities, funcs...)

	// Extract types
	types, err := e.ExtractTypes()
	if err != nil {
		return nil, err
	}
	entities = append(entities, types...)

	// Extract constants and variables
	consts, err := e.ExtractConstants()
	if err != nil {
		return nil, err
	}
	entities = append(entities, consts...)

	// Extract imports
	imports, err := e.ExtractImports()
	if err != nil {
		return nil, err
	}
	entities = append(entities, imports...)

	return entities, nil
}

// ExtractFunctions extracts all function and method declarations.
func (e *Extractor) ExtractFunctions() ([]Entity, error) {
	var entities []Entity

	// Find function declarations
	funcNodes := e.result.FindNodesByType("function_declaration")
	for _, node := range funcNodes {
		entity := e.extractFunction(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	// Find method declarations
	methodNodes := e.result.FindNodesByType("method_declaration")
	for _, node := range methodNodes {
		entity := e.extractMethod(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractTypes extracts all type definitions.
func (e *Extractor) ExtractTypes() ([]Entity, error) {
	var entities []Entity

	// Find type_spec nodes (inside type_declaration)
	typeSpecs := e.result.FindNodesByType("type_spec")
	for _, node := range typeSpecs {
		entity := e.extractType(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// ExtractConstants extracts all constant declarations.
func (e *Extractor) ExtractConstants() ([]Entity, error) {
	var entities []Entity

	// Find const_spec nodes (inside const_declaration)
	constSpecs := e.result.FindNodesByType("const_spec")
	for _, node := range constSpecs {
		extracted := e.extractConstOrVar(node, ConstEntity)
		entities = append(entities, extracted...)
	}

	// Find var_spec nodes (inside var_declaration)
	varSpecs := e.result.FindNodesByType("var_spec")
	for _, node := range varSpecs {
		extracted := e.extractConstOrVar(node, VarEntity)
		entities = append(entities, extracted...)
	}

	return entities, nil
}

// ExtractImports extracts all import declarations.
func (e *Extractor) ExtractImports() ([]Entity, error) {
	var entities []Entity

	// Find import_spec nodes
	importSpecs := e.result.FindNodesByType("import_spec")
	for _, node := range importSpecs {
		entity := e.extractImport(node)
		if entity != nil {
			entities = append(entities, *entity)
		}
	}

	return entities, nil
}

// getFilePath returns the normalized file path.
func (e *Extractor) getFilePath() string {
	if e.basePath != "" {
		return NormalizePath(e.result.FilePath, e.basePath)
	}
	if e.result.FilePath != "" {
		return e.result.FilePath
	}
	return "unknown"
}

// nodeText returns the source text for a node.
func (e *Extractor) nodeText(node *sitter.Node) string {
	return e.result.NodeText(node)
}

// source returns the source bytes.
func (e *Extractor) source() []byte {
	return e.result.Source
}

// findChildByType finds the first child node of the given type.
func findChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// findChildByFieldName finds the child node with the given field name.
func findChildByFieldName(node *sitter.Node, fieldName string) *sitter.Node {
	return node.ChildByFieldName(fieldName)
}

// findChildrenByType finds all direct child nodes of the given type.
func findChildrenByType(node *sitter.Node, nodeType string) []*sitter.Node {
	var children []*sitter.Node
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == nodeType {
			children = append(children, child)
		}
	}
	return children
}

// getLineRange returns the start and end line numbers for a node.
func getLineRange(node *sitter.Node) (uint32, uint32) {
	// tree-sitter lines are 0-based, we want 1-based
	start := node.StartPoint().Row + 1
	end := node.EndPoint().Row + 1
	return start, end
}

// EntityWithNode pairs an Entity with its AST node for call graph extraction.
type EntityWithNode struct {
	Entity *Entity
	Node   *sitter.Node
}

// ExtractAllWithNodes extracts all entities along with their AST nodes.
// This is needed for call graph extraction which requires AST traversal.
func (e *Extractor) ExtractAllWithNodes() ([]EntityWithNode, error) {
	var result []EntityWithNode

	// Extract functions and methods
	funcNodes := e.result.FindNodesByType("function_declaration")
	for _, node := range funcNodes {
		entity := e.extractFunction(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	methodNodes := e.result.FindNodesByType("method_declaration")
	for _, node := range methodNodes {
		entity := e.extractMethod(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract types
	typeSpecs := e.result.FindNodesByType("type_spec")
	for _, node := range typeSpecs {
		entity := e.extractType(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	// Extract constants and variables
	constSpecs := e.result.FindNodesByType("const_spec")
	for _, node := range constSpecs {
		entities := e.extractConstOrVar(node, ConstEntity)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	varSpecs := e.result.FindNodesByType("var_spec")
	for _, node := range varSpecs {
		entities := e.extractConstOrVar(node, VarEntity)
		for i := range entities {
			result = append(result, EntityWithNode{Entity: &entities[i], Node: node})
		}
	}

	// Extract imports
	importSpecs := e.result.FindNodesByType("import_spec")
	for _, node := range importSpecs {
		entity := e.extractImport(node)
		if entity != nil {
			result = append(result, EntityWithNode{Entity: entity, Node: node})
		}
	}

	return result, nil
}

// Package parser provides tree-sitter based code parsing for multiple languages.
//
// The parser package wraps the tree-sitter library to provide a unified
// interface for parsing source code in various programming languages.
// Currently supports Go, with planned support for TypeScript and Python.
package parser

import (
	"context"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
)

// Language represents a supported programming language.
type Language string

const (
	// Go represents the Go programming language.
	Go Language = "go"
	// TypeScript represents the TypeScript programming language.
	TypeScript Language = "typescript"
	// JavaScript represents the JavaScript programming language.
	JavaScript Language = "javascript"
	// Python represents the Python programming language.
	Python Language = "python"
	// Rust represents the Rust programming language.
	Rust Language = "rust"
	// Java represents the Java programming language.
	Java Language = "java"
	// CSharp represents the C# programming language.
	CSharp Language = "csharp"
	// C represents the C programming language.
	C Language = "c"
	// Cpp represents the C++ programming language.
	Cpp Language = "cpp"
	// PHP represents the PHP programming language.
	PHP Language = "php"
	// Kotlin represents the Kotlin programming language.
	Kotlin Language = "kotlin"
	// Ruby represents the Ruby programming language.
	Ruby Language = "ruby"
)

// Parser wraps tree-sitter for code parsing.
type Parser struct {
	parser *sitter.Parser
	lang   Language
}

// ParseResult contains the parsed AST and metadata.
type ParseResult struct {
	// Tree is the complete tree-sitter parse tree.
	Tree *sitter.Tree
	// Root is the root node of the AST.
	Root *sitter.Node
	// Source is the original source code that was parsed.
	Source []byte
	// FilePath is the path to the source file (empty for in-memory parsing).
	FilePath string
	// Language is the programming language of the source.
	Language Language
}

// NewParser creates a parser for the given language.
// Returns an UnsupportedLanguageError if the language is not supported.
func NewParser(lang Language) (*Parser, error) {
	var (
		p   *sitter.Parser
		err error
	)

	switch lang {
	case Go:
		p, err = newGoParser()
	case TypeScript:
		p, err = newTypeScriptParser()
	case JavaScript:
		p, err = newJavaScriptParser()
	case Python:
		p, err = newPythonParser()
	case Rust:
		p, err = newRustParser()
	case Java:
		p, err = newJavaParser()
	case CSharp:
		p, err = newCSharpParser()
	case C:
		p, err = newCParser()
	case Cpp:
		p, err = newCppParser()
	case PHP:
		p, err = newPHPParser()
	case Kotlin:
		p, err = newKotlinParser()
	case Ruby:
		p, err = newRubyParser()
	default:
		return nil, &UnsupportedLanguageError{Language: string(lang)}
	}

	if err != nil {
		return nil, err
	}

	return &Parser{
		parser: p,
		lang:   lang,
	}, nil
}

// Parse parses source code and returns the AST.
func (p *Parser) Parse(source []byte) (*ParseResult, error) {
	tree, err := p.parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, &ParseError{
			Message: err.Error(),
		}
	}

	return &ParseResult{
		Tree:     tree,
		Root:     tree.RootNode(),
		Source:   source,
		Language: p.lang,
	}, nil
}

// ParseFile parses a file from disk.
func (p *Parser) ParseFile(path string) (*ParseResult, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, &FileReadError{Path: path, Err: err}
	}

	result, err := p.Parse(source)
	if err != nil {
		if pe, ok := err.(*ParseError); ok {
			pe.File = path
		}
		return nil, err
	}

	result.FilePath = path
	return result, nil
}

// Language returns the language this parser is configured for.
func (p *Parser) Language() Language {
	return p.lang
}

// Close releases parser resources.
// After calling Close, the parser should not be used.
func (p *Parser) Close() {
	if p.parser != nil {
		p.parser.Close()
		p.parser = nil
	}
}

// Close releases the parse tree resources.
func (r *ParseResult) Close() {
	if r.Tree != nil {
		r.Tree.Close()
		r.Tree = nil
		r.Root = nil
	}
}

// HasErrors returns true if the parse tree contains syntax errors.
func (r *ParseResult) HasErrors() bool {
	if r.Root == nil {
		return false
	}
	return r.Root.HasError()
}

// WalkNodes traverses the AST depth-first, calling the visitor function
// for each node. If the visitor returns false, traversal stops.
func (r *ParseResult) WalkNodes(visitor func(*sitter.Node) bool) {
	if r.Root == nil {
		return
	}
	walkNode(r.Root, visitor)
}

// walkNode is a helper for depth-first AST traversal.
func walkNode(node *sitter.Node, visitor func(*sitter.Node) bool) bool {
	if !visitor(node) {
		return false
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		if !walkNode(node.Child(int(i)), visitor) {
			return false
		}
	}
	return true
}

// FindNodes returns all nodes matching the given predicate.
func (r *ParseResult) FindNodes(predicate func(*sitter.Node) bool) []*sitter.Node {
	var nodes []*sitter.Node
	r.WalkNodes(func(node *sitter.Node) bool {
		if predicate(node) {
			nodes = append(nodes, node)
		}
		return true
	})
	return nodes
}

// FindNodesByType returns all nodes of the specified type.
func (r *ParseResult) FindNodesByType(nodeType string) []*sitter.Node {
	return r.FindNodes(func(node *sitter.Node) bool {
		return node.Type() == nodeType
	})
}

// NodeText returns the source text for a node.
func (r *ParseResult) NodeText(node *sitter.Node) string {
	if node == nil || r.Source == nil {
		return ""
	}
	return node.Content(r.Source)
}

// LanguageFromExtension returns the language for a file extension.
// Returns empty string if the extension is not recognized.
func LanguageFromExtension(ext string) Language {
	switch ext {
	case ".go":
		return Go
	case ".ts", ".tsx":
		return TypeScript
	case ".js", ".jsx", ".mjs", ".cjs":
		return JavaScript
	case ".py", ".pyi":
		return Python
	case ".rs":
		return Rust
	case ".java":
		return Java
	case ".cs":
		return CSharp
	case ".c", ".h":
		return C
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return Cpp
	case ".php":
		return PHP
	case ".kt", ".kts":
		return Kotlin
	case ".rb", ".rake":
		return Ruby
	default:
		return ""
	}
}

// SupportedExtensions returns all file extensions supported for parsing.
func SupportedExtensions() []string {
	return []string{
		".go",
		".ts", ".tsx",
		".js", ".jsx", ".mjs", ".cjs",
		".py", ".pyi",
		".rs",
		".java",
		".cs",
		".c", ".h",
		".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx",
		".php",
		".kt", ".kts",
		".rb", ".rake",
	}
}

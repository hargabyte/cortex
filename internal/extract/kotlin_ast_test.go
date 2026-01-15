package extract

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestKotlinASTStructure(t *testing.T) {
	code := `package com.example

fun regularFunction(): String {
    return "hello"
}

class RegularClass {
    fun method() {}
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	// Print AST
	printNode(t, result.Root, 0, result.Source)
}

func printNode(t *testing.T, node *sitter.Node, depth int, source []byte) {
	if node == nil {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	text := node.Content(source)
	if len(text) > 50 {
		text = text[:47] + "..."
	}

	t.Logf("%s%s [%d:%d] %q\n", indent, node.Type(), node.StartPoint().Row, node.StartPoint().Column, text)

	if depth < 5 { // Limit depth to avoid too much output
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			printNode(t, child, depth+1, source)
		}
	}
}

func TestListKotlinNodeTypes(t *testing.T) {
	code := `package com.example

fun regularFunction(): String {
    return "hello"
}

class RegularClass {
    val prop: String = "test"
    fun method() {}
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	// Collect all unique node types
	nodeTypes := make(map[string]bool)
	collectNodeTypes(result.Root, nodeTypes)

	t.Log("Unique node types found:")
	for nodeType := range nodeTypes {
		t.Logf("  - %s", nodeType)
	}
}

func collectNodeTypes(node *sitter.Node, types map[string]bool) {
	if node == nil {
		return
	}

	types[node.Type()] = true

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		collectNodeTypes(child, types)
	}
}

func TestFindKotlinFunctionNodes(t *testing.T) {
	code := `package com.example

fun regularFunction(): String {
    return "hello"
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)

	// Try different node type names
	testTypes := []string{
		"function_declaration",
		"function",
		"fun_declaration",
		"method_declaration",
		"simple_function",
	}

	for _, nodeType := range testTypes {
		nodes := result.FindNodesByType(nodeType)
		t.Logf("Found %d nodes of type %q", len(nodes), nodeType)
		if len(nodes) > 0 {
			for _, node := range nodes {
				text := ext.nodeText(node)
				if len(text) > 100 {
					text = text[:97] + "..."
				}
				t.Logf("  Node content: %q", text)
			}
		}
	}
}

func TestKotlinPropertyAST(t *testing.T) {
	code := `class Constants {
    val immutableValue: String = "test"
    var mutableValue: Int = 0
    const val CONSTANT = "constant"
}`

	result := parseKotlinCode(t, code)
	defer result.Close()

	// Print AST
	printNode(t, result.Root, 0, result.Source)
}

func TestKotlinDataClassAST(t *testing.T) {
	code := `data class DataClass(val name: String, val age: Int)`

	result := parseKotlinCode(t, code)
	defer result.Close()

	// Print AST
	printNode(t, result.Root, 0, result.Source)
}

func TestKotlinSuspendFunctionAST(t *testing.T) {
	code := `suspend fun coroutineFunction() {
    // coroutine code
}`

	result := parseKotlinCode(t, code)
	defer result.Close()

	// Print AST
	printNode(t, result.Root, 0, result.Source)
}

func TestKotlinExtensionFunctionAST(t *testing.T) {
	code := `fun String.extensionFunction(): Int = this.length`

	result := parseKotlinCode(t, code)
	defer result.Close()

	// Print AST
	printNode(t, result.Root, 0, result.Source)
}

func TestFindKotlinClassNodes(t *testing.T) {
	code := `package com.example

class RegularClass {
    fun method() {}
}

data class DataClass(val name: String)
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)

	// Try different node type names
	testTypes := []string{
		"class_declaration",
		"class",
		"data_class",
		"type_declaration",
	}

	for _, nodeType := range testTypes {
		nodes := result.FindNodesByType(nodeType)
		t.Logf("Found %d nodes of type %q", len(nodes), nodeType)
		if len(nodes) > 0 {
			for _, node := range nodes {
				text := ext.nodeText(node)
				if len(text) > 100 {
					text = text[:97] + "..."
				}
				t.Logf("  Node content: %q", text)
			}
		}
	}
}

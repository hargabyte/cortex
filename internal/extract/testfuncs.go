// Package extract provides test function discovery from parsed AST.
// This file implements detection of test functions across supported languages.
package extract

import (
	"regexp"
	"strings"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// TestFunctionInfo holds information about a discovered test function.
type TestFunctionInfo struct {
	// Name is the test function name (e.g., "TestFoo", "it('should work')")
	Name string
	// FilePath is the source file containing this test
	FilePath string
	// StartLine is the starting line number (1-based)
	StartLine uint32
	// EndLine is the ending line number (1-based)
	EndLine uint32
	// Language is the programming language
	Language string
	// TestType indicates the type of test (unit, benchmark, example)
	TestType TestType
	// ParentDescribe is the parent describe/context block name (for JS/TS)
	ParentDescribe string
	// FullName is the fully qualified test name including parent describes
	FullName string
}

// TestType represents the kind of test function.
type TestType string

const (
	// TestTypeUnit is a standard unit test.
	TestTypeUnit TestType = "unit"
	// TestTypeBenchmark is a performance benchmark.
	TestTypeBenchmark TestType = "benchmark"
	// TestTypeExample is an example/documentation test.
	TestTypeExample TestType = "example"
	// TestTypeFuzz is a fuzz test.
	TestTypeFuzz TestType = "fuzz"
)

// Go test function patterns
var (
	goTestPattern      = regexp.MustCompile(`^Test[A-Z_]`)
	goBenchmarkPattern = regexp.MustCompile(`^Benchmark[A-Z_]`)
	goExamplePattern   = regexp.MustCompile(`^Example[A-Z_]?`)
	goFuzzPattern      = regexp.MustCompile(`^Fuzz[A-Z_]`)
)

// IsTestFile checks if a file path indicates a test file.
func IsTestFile(filePath, language string) bool {
	switch language {
	case "go":
		return strings.HasSuffix(filePath, "_test.go")
	case "typescript", "javascript":
		// *.test.ts, *.spec.ts, *.test.js, *.spec.js
		return strings.HasSuffix(filePath, ".test.ts") ||
			strings.HasSuffix(filePath, ".spec.ts") ||
			strings.HasSuffix(filePath, ".test.js") ||
			strings.HasSuffix(filePath, ".spec.js") ||
			strings.HasSuffix(filePath, ".test.tsx") ||
			strings.HasSuffix(filePath, ".spec.tsx") ||
			strings.HasSuffix(filePath, ".test.jsx") ||
			strings.HasSuffix(filePath, ".spec.jsx") ||
			// Also check for __tests__ directory pattern
			strings.Contains(filePath, "__tests__")
	case "python":
		// test_*.py or *_test.py or tests/*.py
		base := filePath
		if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
			base = filePath[idx+1:]
		}
		return strings.HasPrefix(base, "test_") ||
			strings.HasSuffix(base, "_test.py") ||
			strings.Contains(filePath, "/tests/")
	case "rust":
		// Files in tests/ directory or modules named tests
		return strings.Contains(filePath, "/tests/") ||
			strings.HasPrefix(filePath, "tests/") ||
			strings.HasSuffix(filePath, "_test.rs")
	case "java":
		// *Test.java, *Tests.java, Test*.java
		base := filePath
		if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
			base = filePath[idx+1:]
		}
		return strings.HasSuffix(base, "Test.java") ||
			strings.HasSuffix(base, "Tests.java") ||
			strings.HasPrefix(base, "Test")
	default:
		return false
	}
}

// IsTestFunction checks if an entity represents a test function.
// This uses naming conventions specific to each language.
func IsTestFunction(entity *Entity) bool {
	if entity == nil {
		return false
	}

	// Only functions and methods can be tests
	if entity.Kind != FunctionEntity && entity.Kind != MethodEntity {
		return false
	}

	// Must be in a test file (for most languages)
	if !IsTestFile(entity.File, entity.Language) {
		return false
	}

	switch entity.Language {
	case "go", "":
		return IsGoTestFunction(entity.Name)
	case "typescript", "javascript":
		return IsJSTestFunction(entity.Name)
	case "python":
		return IsPythonTestFunction(entity.Name)
	case "rust":
		return IsRustTestFunction(entity)
	case "java":
		return IsJavaTestFunction(entity)
	default:
		return false
	}
}

// IsGoTestFunction checks if a function name matches Go test patterns.
func IsGoTestFunction(name string) bool {
	return goTestPattern.MatchString(name) ||
		goBenchmarkPattern.MatchString(name) ||
		goExamplePattern.MatchString(name) ||
		goFuzzPattern.MatchString(name)
}

// GetGoTestType returns the type of Go test function.
func GetGoTestType(name string) TestType {
	switch {
	case goBenchmarkPattern.MatchString(name):
		return TestTypeBenchmark
	case goExamplePattern.MatchString(name):
		return TestTypeExample
	case goFuzzPattern.MatchString(name):
		return TestTypeFuzz
	default:
		return TestTypeUnit
	}
}

// IsJSTestFunction checks if a function is a JavaScript/TypeScript test.
// Looks for common testing framework patterns: it(), test(), describe()
func IsJSTestFunction(name string) bool {
	// Common test function names from Jest, Mocha, Vitest, etc.
	testFunctions := []string{"it", "test", "describe", "context", "specify"}
	for _, fn := range testFunctions {
		if name == fn {
			return true
		}
	}
	return false
}

// IsPythonTestFunction checks if a function name matches Python test patterns.
// Pytest uses test_* and *_test, unittest uses test*
func IsPythonTestFunction(name string) bool {
	return strings.HasPrefix(name, "test_") ||
		strings.HasPrefix(name, "test") && len(name) > 4 && name[4] >= 'A' && name[4] <= 'Z'
}

// IsRustTestFunction checks if a Rust function is marked as a test.
// Looks for #[test], #[tokio::test], etc. decorators
func IsRustTestFunction(entity *Entity) bool {
	for _, dec := range entity.Decorators {
		if dec == "test" || strings.Contains(dec, "::test") {
			return true
		}
	}
	return false
}

// IsJavaTestFunction checks if a Java method is a test.
// Looks for @Test, @ParameterizedTest annotations
func IsJavaTestFunction(entity *Entity) bool {
	for _, dec := range entity.Decorators {
		if dec == "Test" || dec == "ParameterizedTest" ||
			dec == "org.junit.Test" || dec == "org.junit.jupiter.api.Test" {
			return true
		}
	}
	// Also check method name pattern
	return strings.HasPrefix(entity.Name, "test") &&
		len(entity.Name) > 4 &&
		entity.Name[4] >= 'A' && entity.Name[4] <= 'Z'
}

// ExtractGoTestFunctions extracts all test functions from a Go parse result.
func ExtractGoTestFunctions(extractor *Extractor) ([]TestFunctionInfo, error) {
	var tests []TestFunctionInfo

	// Only process test files
	if !IsTestFile(extractor.result.FilePath, "go") {
		return tests, nil
	}

	// Find all function declarations
	funcNodes := extractor.result.FindNodesByType("function_declaration")
	for _, node := range funcNodes {
		nameNode := findChildByFieldName(node, "name")
		if nameNode == nil {
			continue
		}

		name := extractor.nodeText(nameNode)
		if !IsGoTestFunction(name) {
			continue
		}

		startLine, endLine := getLineRange(node)
		tests = append(tests, TestFunctionInfo{
			Name:      name,
			FilePath:  extractor.getFilePath(),
			StartLine: startLine,
			EndLine:   endLine,
			Language:  "go",
			TestType:  GetGoTestType(name),
			FullName:  name,
		})
	}

	return tests, nil
}

// ExtractJSTestFunctions extracts test functions from TypeScript/JavaScript.
// This looks for it(), test(), describe() call expressions.
func ExtractJSTestFunctions(result *parser.ParseResult, filePath string, language string) ([]TestFunctionInfo, error) {
	var tests []TestFunctionInfo

	if !IsTestFile(filePath, language) {
		return tests, nil
	}

	// Stack to track nested describe blocks
	var describeStack []string

	// Walk the AST looking for call expressions
	result.WalkNodes(func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}

		// Get the function being called
		funcNode := node.ChildByFieldName("function")
		if funcNode == nil {
			return true
		}

		funcName := result.NodeText(funcNode)

		// Check if it's a test function
		if funcName == "describe" || funcName == "context" {
			// Extract the describe name from first argument
			args := node.ChildByFieldName("arguments")
			if args != nil && args.ChildCount() > 0 {
				firstArg := args.Child(0)
				if firstArg != nil && (firstArg.Type() == "string" || firstArg.Type() == "template_string") {
					describeName := extractStringContent(result.NodeText(firstArg))
					describeStack = append(describeStack, describeName)
				}
			}
		} else if funcName == "it" || funcName == "test" || funcName == "specify" {
			// Extract the test name from first argument
			args := node.ChildByFieldName("arguments")
			if args == nil || args.ChildCount() == 0 {
				return true
			}

			firstArg := args.Child(0)
			if firstArg == nil {
				return true
			}

			testName := extractStringContent(result.NodeText(firstArg))
			startLine := node.StartPoint().Row + 1
			endLine := node.EndPoint().Row + 1

			// Build full name from describe stack
			fullName := testName
			if len(describeStack) > 0 {
				fullName = strings.Join(describeStack, " > ") + " > " + testName
			}

			parentDescribe := ""
			if len(describeStack) > 0 {
				parentDescribe = describeStack[len(describeStack)-1]
			}

			tests = append(tests, TestFunctionInfo{
				Name:           testName,
				FilePath:       filePath,
				StartLine:      uint32(startLine),
				EndLine:        uint32(endLine),
				Language:       language,
				TestType:       TestTypeUnit,
				ParentDescribe: parentDescribe,
				FullName:       fullName,
			})
		}

		return true
	})

	return tests, nil
}

// extractStringContent removes quotes from a string literal.
func extractStringContent(s string) string {
	s = strings.TrimSpace(s)
	// Remove quotes
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '`' && s[len(s)-1] == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// EntityToTestInfo converts an Entity to TestFunctionInfo if it's a test.
func EntityToTestInfo(entity *Entity) *TestFunctionInfo {
	if !IsTestFunction(entity) {
		return nil
	}

	testType := TestTypeUnit
	if entity.Language == "go" || entity.Language == "" {
		testType = GetGoTestType(entity.Name)
	}

	return &TestFunctionInfo{
		Name:      entity.Name,
		FilePath:  entity.File,
		StartLine: entity.StartLine,
		EndLine:   entity.EndLine,
		Language:  entity.Language,
		TestType:  testType,
		FullName:  entity.Name,
	}
}

// FilterTestEntities filters a list of entities to only include test functions.
func FilterTestEntities(entities []Entity) []Entity {
	var tests []Entity
	for _, e := range entities {
		if IsTestFunction(&e) {
			tests = append(tests, e)
		}
	}
	return tests
}

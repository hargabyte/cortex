package extract

import (
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		language string
		want     bool
	}{
		// Go test files
		{"go test file", "foo_test.go", "go", true},
		{"go non-test file", "foo.go", "go", false},
		{"go test in path", "internal/cmd/foo_test.go", "go", true},

		// TypeScript test files
		{"ts test file", "foo.test.ts", "typescript", true},
		{"ts spec file", "foo.spec.ts", "typescript", true},
		{"ts non-test", "foo.ts", "typescript", false},
		{"tsx test file", "Component.test.tsx", "typescript", true},
		{"__tests__ dir", "__tests__/foo.ts", "typescript", true},

		// JavaScript test files
		{"js test file", "foo.test.js", "javascript", true},
		{"js spec file", "foo.spec.js", "javascript", true},
		{"jsx spec file", "Component.spec.jsx", "javascript", true},

		// Python test files
		{"python test_ prefix", "test_foo.py", "python", true},
		{"python _test suffix", "foo_test.py", "python", true},
		{"python tests dir", "tests/test_foo.py", "python", true},
		{"python non-test", "foo.py", "python", false},

		// Rust test files
		{"rust tests dir", "tests/integration.rs", "rust", true},
		{"rust test suffix", "foo_test.rs", "rust", true},
		{"rust non-test", "lib.rs", "rust", false},

		// Java test files
		{"java Test suffix", "FooTest.java", "java", true},
		{"java Tests suffix", "FooTests.java", "java", true},
		{"java Test prefix", "TestFoo.java", "java", true},
		{"java non-test", "Foo.java", "java", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestFile(tt.filePath, tt.language)
			if got != tt.want {
				t.Errorf("IsTestFile(%q, %q) = %v, want %v", tt.filePath, tt.language, got, tt.want)
			}
		})
	}
}

func TestIsGoTestFunction(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Valid test functions
		{"TestFoo", true},
		{"TestFoo_Bar", true},
		{"Test_Underscore", true},
		{"TestA", true},

		// Valid benchmark functions
		{"BenchmarkFoo", true},
		{"BenchmarkFoo_Bar", true},

		// Valid example functions
		{"Example", true},
		{"ExampleFoo", true},
		{"ExampleFoo_Bar", true},

		// Valid fuzz functions
		{"FuzzFoo", true},
		{"FuzzFoo_Bar", true},

		// Invalid - doesn't match pattern
		{"Testfoo", false},      // lowercase after Test
		{"test", false},         // all lowercase
		{"TestFunction", true},  // valid
		{"Benchmarkfoo", false}, // lowercase after Benchmark
		{"test_foo", false},     // python style
		{"foo", false},          // regular function
		{"Test", false},         // no suffix
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGoTestFunction(tt.name)
			if got != tt.want {
				t.Errorf("IsGoTestFunction(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetGoTestType(t *testing.T) {
	tests := []struct {
		name string
		want TestType
	}{
		{"TestFoo", TestTypeUnit},
		{"BenchmarkFoo", TestTypeBenchmark},
		{"ExampleFoo", TestTypeExample},
		{"FuzzFoo", TestTypeFuzz},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetGoTestType(tt.name)
			if got != tt.want {
				t.Errorf("GetGoTestType(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsJSTestFunction(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"it", true},
		{"test", true},
		{"describe", true},
		{"context", true},
		{"specify", true},
		{"foo", false},
		{"myTest", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJSTestFunction(tt.name)
			if got != tt.want {
				t.Errorf("IsJSTestFunction(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsPythonTestFunction(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"test_foo", true},
		{"test_bar_baz", true},
		{"testFoo", true},  // unittest style
		{"testBar", true},  // unittest style
		{"test", false},    // just "test" alone
		{"foo_test", false}, // suffix not valid for function names
		{"tests", false},
		{"testing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPythonTestFunction(tt.name)
			if got != tt.want {
				t.Errorf("IsPythonTestFunction(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsTestFunction(t *testing.T) {
	tests := []struct {
		name   string
		entity *Entity
		want   bool
	}{
		{
			"go test function",
			&Entity{
				Kind:     FunctionEntity,
				Name:     "TestFoo",
				File:     "foo_test.go",
				Language: "go",
			},
			true,
		},
		{
			"go non-test function",
			&Entity{
				Kind:     FunctionEntity,
				Name:     "Foo",
				File:     "foo_test.go",
				Language: "go",
			},
			false,
		},
		{
			"go test in non-test file",
			&Entity{
				Kind:     FunctionEntity,
				Name:     "TestFoo",
				File:     "foo.go",
				Language: "go",
			},
			false,
		},
		{
			"type entity",
			&Entity{
				Kind:     TypeEntity,
				Name:     "TestType",
				File:     "foo_test.go",
				Language: "go",
			},
			false,
		},
		{
			"nil entity",
			nil,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestFunction(tt.entity)
			if got != tt.want {
				t.Errorf("IsTestFunction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractGoTestFunctions(t *testing.T) {
	source := `package foo

import "testing"

func TestFoo(t *testing.T) {
	// test implementation
}

func BenchmarkBar(b *testing.B) {
	// benchmark implementation
}

func ExampleBaz() {
	// example implementation
}

func helperFunction() {
	// not a test
}
`
	p, err := parser.NewParser(parser.Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	result.FilePath = "foo_test.go"
	defer result.Close()

	extractor := NewExtractor(result)
	tests, err := ExtractGoTestFunctions(extractor)
	if err != nil {
		t.Fatalf("ExtractGoTestFunctions failed: %v", err)
	}

	// Should find 3 test functions
	if len(tests) != 3 {
		t.Errorf("Expected 3 test functions, got %d", len(tests))
	}

	// Verify test names and types
	expectedTests := map[string]TestType{
		"TestFoo":      TestTypeUnit,
		"BenchmarkBar": TestTypeBenchmark,
		"ExampleBaz":   TestTypeExample,
	}

	for _, test := range tests {
		expectedType, ok := expectedTests[test.Name]
		if !ok {
			t.Errorf("Unexpected test function: %s", test.Name)
			continue
		}
		if test.TestType != expectedType {
			t.Errorf("Test %s: expected type %v, got %v", test.Name, expectedType, test.TestType)
		}
	}
}

func TestFilterTestEntities(t *testing.T) {
	entities := []Entity{
		{Kind: FunctionEntity, Name: "TestFoo", File: "foo_test.go", Language: "go"},
		{Kind: FunctionEntity, Name: "Foo", File: "foo.go", Language: "go"},
		{Kind: FunctionEntity, Name: "TestBar", File: "bar_test.go", Language: "go"},
		{Kind: TypeEntity, Name: "TestType", File: "foo_test.go", Language: "go"},
	}

	tests := FilterTestEntities(entities)
	if len(tests) != 2 {
		t.Errorf("Expected 2 test entities, got %d", len(tests))
	}
}

func TestExtractStringContent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{"`hello`", "hello"},
		{"hello", "hello"},
		{`""`, ""},
		{`"hello world"`, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractStringContent(tt.input)
			if got != tt.want {
				t.Errorf("extractStringContent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

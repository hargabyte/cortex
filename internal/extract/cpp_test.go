package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseCppCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.Cpp)
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(code))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return result
}

func TestCppExtractFunctions(t *testing.T) {
	code := `
int add(int a, int b) {
    return a + b;
}

double multiply(double x, double y) {
    return x * y;
}

void process(const std::string& data) {
    // process data
}

static int helper() {
    return 42;
}
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(entities) != 4 {
		t.Fatalf("expected 4 functions, got %d", len(entities))
	}

	// Find add function
	var addFunc, helperFunc *Entity
	for i := range entities {
		switch entities[i].Name {
		case "add":
			addFunc = &entities[i]
		case "helper":
			helperFunc = &entities[i]
		}
	}

	// Check add function
	if addFunc == nil {
		t.Fatal("add function not found")
	}
	if addFunc.Kind != FunctionEntity {
		t.Errorf("add: expected FunctionEntity, got %v", addFunc.Kind)
	}
	if len(addFunc.Params) != 2 {
		t.Errorf("add: expected 2 params, got %d", len(addFunc.Params))
	}
	if len(addFunc.Returns) != 1 || addFunc.Returns[0] != "int" {
		t.Errorf("add: expected return type 'int', got %v", addFunc.Returns)
	}

	// Check helper function (static = private)
	if helperFunc == nil {
		t.Fatal("helper function not found")
	}
	if helperFunc.Visibility != VisibilityPrivate {
		t.Errorf("helper: expected private visibility (static), got %v", helperFunc.Visibility)
	}
}

func TestCppExtractClasses(t *testing.T) {
	code := `
class MyClass {
public:
    int publicField;
    void publicMethod() {}

private:
    int privateField;
    void privateMethod() {}

protected:
    int protectedField;
    void protectedMethod() {}
};

class DerivedClass : public MyClass {
public:
    DerivedClass() {}
    ~DerivedClass() {}
};
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Should have 2 classes + their methods
	if len(entities) < 2 {
		t.Fatalf("expected at least 2 classes, got %d", len(entities))
	}

	// Find MyClass
	var myClass, derivedClass *Entity
	for i := range entities {
		if entities[i].Kind == TypeEntity {
			if entities[i].Name == "MyClass" {
				myClass = &entities[i]
			} else if entities[i].Name == "DerivedClass" {
				derivedClass = &entities[i]
			}
		}
	}

	if myClass == nil {
		t.Fatal("MyClass not found")
	}
	if myClass.TypeKind != StructKind {
		t.Errorf("MyClass: expected StructKind, got %v", myClass.TypeKind)
	}
	if myClass.ValueType != "class" {
		t.Errorf("MyClass: expected ValueType 'class', got %q", myClass.ValueType)
	}

	// Check fields with visibility
	if len(myClass.Fields) < 3 {
		t.Errorf("MyClass: expected at least 3 fields, got %d", len(myClass.Fields))
	}

	// Check visibility of fields
	for _, f := range myClass.Fields {
		switch f.Name {
		case "publicField":
			if f.Visibility != VisibilityPublic {
				t.Errorf("publicField: expected public visibility, got %v", f.Visibility)
			}
		case "privateField":
			if f.Visibility != VisibilityPrivate {
				t.Errorf("privateField: expected private visibility, got %v", f.Visibility)
			}
		case "protectedField":
			if f.Visibility != VisibilityProtected {
				t.Errorf("protectedField: expected protected visibility, got %v", f.Visibility)
			}
		}
	}

	// Check DerivedClass has inheritance
	if derivedClass == nil {
		t.Fatal("DerivedClass not found")
	}
	if len(derivedClass.Implements) != 1 || derivedClass.Implements[0] != "MyClass" {
		t.Errorf("DerivedClass: expected to extend MyClass, got %v", derivedClass.Implements)
	}
}

func TestCppExtractStructs(t *testing.T) {
	code := `
struct Point {
    int x;
    int y;
};

struct Person {
    std::string name;
    int age;
    double height;
};

struct Node {
    int value;
    Node* next;
    Node* prev;
};
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractStructs()
	if err != nil {
		t.Fatalf("ExtractStructs failed: %v", err)
	}

	if len(entities) < 3 {
		t.Fatalf("expected at least 3 structs, got %d", len(entities))
	}

	// Find Point struct
	var pointStruct *Entity
	for i := range entities {
		if entities[i].Kind == TypeEntity && entities[i].Name == "Point" {
			pointStruct = &entities[i]
			break
		}
	}

	if pointStruct == nil {
		t.Fatal("Point struct not found")
	}

	if pointStruct.TypeKind != StructKind {
		t.Errorf("Point: expected StructKind, got %v", pointStruct.TypeKind)
	}

	if len(pointStruct.Fields) != 2 {
		t.Errorf("Point: expected 2 fields, got %d", len(pointStruct.Fields))
	}

	// Check field names
	fieldNames := make(map[string]bool)
	for _, f := range pointStruct.Fields {
		fieldNames[f.Name] = true
		// Struct fields should be public by default
		if f.Visibility != VisibilityPublic {
			t.Errorf("Point field %s: expected public visibility, got %v", f.Name, f.Visibility)
		}
	}
	if !fieldNames["x"] || !fieldNames["y"] {
		t.Errorf("Point: expected fields x and y, got %v", pointStruct.Fields)
	}
}

func TestCppExtractNamespaces(t *testing.T) {
	code := `
namespace math {
    int add(int a, int b) {
        return a + b;
    }
}

namespace utils {
    namespace string_helpers {
        void trim(std::string& s) {}
    }
}
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractNamespaces()
	if err != nil {
		t.Fatalf("ExtractNamespaces failed: %v", err)
	}

	// Should find at least 2 namespaces (math and utils, possibly string_helpers)
	if len(entities) < 2 {
		t.Fatalf("expected at least 2 namespaces, got %d", len(entities))
	}

	// Check for math namespace
	hasMath := false
	hasUtils := false
	for _, e := range entities {
		if e.Name == "math" {
			hasMath = true
			if e.ValueType != "namespace" {
				t.Errorf("math: expected ValueType 'namespace', got %q", e.ValueType)
			}
		}
		if e.Name == "utils" {
			hasUtils = true
		}
	}

	if !hasMath {
		t.Error("math namespace not found")
	}
	if !hasUtils {
		t.Error("utils namespace not found")
	}
}

func TestCppExtractEnums(t *testing.T) {
	code := `
enum Color {
    RED,
    GREEN,
    BLUE
};

enum class Status {
    PENDING = 0,
    ACTIVE = 1,
    COMPLETED = 2
};

enum FileMode : int {
    READ = 1,
    WRITE = 2,
    EXECUTE = 4
};
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractEnums()
	if err != nil {
		t.Fatalf("ExtractEnums failed: %v", err)
	}

	if len(entities) < 2 {
		t.Fatalf("expected at least 2 enums, got %d", len(entities))
	}

	// Find Color enum
	var colorEnum, statusEnum *Entity
	for i := range entities {
		if entities[i].Name == "Color" {
			colorEnum = &entities[i]
		} else if entities[i].Name == "Status" {
			statusEnum = &entities[i]
		}
	}

	if colorEnum == nil {
		t.Fatal("Color enum not found")
	}

	if colorEnum.Kind != EnumEntity {
		t.Errorf("Color: expected EnumEntity, got %v", colorEnum.Kind)
	}

	if len(colorEnum.EnumValues) != 3 {
		t.Errorf("Color: expected 3 enum values, got %d", len(colorEnum.EnumValues))
	}

	// Check enum class
	if statusEnum == nil {
		t.Fatal("Status enum not found")
	}
	if statusEnum.ValueType != "enum class" {
		t.Errorf("Status: expected ValueType 'enum class', got %q", statusEnum.ValueType)
	}
}

func TestCppExtractTypedefs(t *testing.T) {
	code := `
typedef int INT32;
typedef unsigned char BYTE;
typedef std::vector<int> IntVector;

using String = std::string;
using IntPtr = int*;
using Callback = void(*)(int, void*);
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractTypedefs()
	if err != nil {
		t.Fatalf("ExtractTypedefs failed: %v", err)
	}

	// Should have multiple typedefs and aliases
	if len(entities) < 3 {
		t.Fatalf("expected at least 3 typedefs/aliases, got %d", len(entities))
	}

	// Find INT32 typedef
	var int32Typedef, stringAlias *Entity
	for i := range entities {
		if entities[i].Name == "INT32" {
			int32Typedef = &entities[i]
		} else if entities[i].Name == "String" {
			stringAlias = &entities[i]
		}
	}

	if int32Typedef == nil {
		t.Fatal("INT32 typedef not found")
	}

	if int32Typedef.Kind != TypeEntity {
		t.Errorf("INT32: expected TypeEntity, got %v", int32Typedef.Kind)
	}

	if int32Typedef.TypeKind != AliasKind {
		t.Errorf("INT32: expected AliasKind, got %v", int32Typedef.TypeKind)
	}

	// Check using alias
	if stringAlias != nil {
		if stringAlias.TypeKind != AliasKind {
			t.Errorf("String: expected AliasKind, got %v", stringAlias.TypeKind)
		}
	}
}

func TestCppExtractMethods(t *testing.T) {
	code := `
class Calculator {
public:
    Calculator() {
        value = 0;
    }

    ~Calculator() {}

    int add(int x) {
        return value + x;
    }

    virtual void reset() {
        value = 0;
    }

private:
    int value;
};
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Should have class + methods
	if len(entities) < 2 {
		t.Fatalf("expected at least 2 entities (class + methods), got %d", len(entities))
	}

	// Find methods
	var constructor, destructor, addMethod *Entity
	for i := range entities {
		if entities[i].Kind == FunctionEntity {
			if entities[i].ValueType == "constructor" {
				constructor = &entities[i]
			} else if entities[i].ValueType == "destructor" {
				destructor = &entities[i]
			} else if entities[i].Name == "add" || strings.Contains(entities[i].Name, "add") {
				addMethod = &entities[i]
			}
		}
	}

	// Check constructor
	if constructor != nil {
		if constructor.ValueType != "constructor" {
			t.Errorf("Constructor: expected ValueType 'constructor', got %q", constructor.ValueType)
		}
	}

	// Check destructor
	if destructor != nil {
		if destructor.ValueType != "destructor" {
			t.Errorf("Destructor: expected ValueType 'destructor', got %q", destructor.ValueType)
		}
	}

	// Check method visibility
	if addMethod != nil {
		if addMethod.Visibility != VisibilityPublic {
			t.Errorf("add method: expected public visibility, got %v", addMethod.Visibility)
		}
	}
}

func TestCppExtractTemplates(t *testing.T) {
	code := `
template<typename T>
class Container {
public:
    T value;

    T get() const {
        return value;
    }

    void set(T v) {
        value = v;
    }
};

template<typename T, typename U>
T convert(U input) {
    return static_cast<T>(input);
}
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should extract the class and functions (templates are handled as regular entities)
	if len(entities) == 0 {
		t.Fatal("no entities extracted from template code")
	}

	// Templates should still extract the underlying entities
	hasClass := false
	hasFunctions := false
	for _, e := range entities {
		if e.Kind == TypeEntity && e.Name == "Container" {
			hasClass = true
		}
		if e.Kind == FunctionEntity {
			hasFunctions = true
		}
	}

	if !hasClass {
		t.Error("Container class not found in template code")
	}
	if !hasFunctions {
		t.Error("No functions found in template code")
	}
}

func TestCppExtractAll(t *testing.T) {
	code := `
#include <iostream>

#define MAX_SIZE 100

namespace app {
    enum Status {
        OK,
        ERROR
    };

    class MyClass {
    public:
        int value;
        void doSomething() {}
    };

    int helper(int x) {
        return x * 2;
    }
}
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have multiple entity types
	if len(entities) < 4 {
		t.Fatalf("expected at least 4 entities, got %d", len(entities))
	}

	// Check for variety of types
	hasNamespace := false
	hasClass := false
	hasEnum := false
	hasFunction := false
	hasMacro := false

	for _, e := range entities {
		switch e.Kind {
		case FunctionEntity:
			hasFunction = true
		case TypeEntity:
			if e.ValueType == "namespace" {
				hasNamespace = true
			}
			if e.ValueType == "class" {
				hasClass = true
			}
		case EnumEntity:
			hasEnum = true
		case ConstEntity:
			hasMacro = true
		}
	}

	if !hasFunction {
		t.Error("no functions found")
	}
	if !hasClass {
		t.Error("no classes found")
	}
	if !hasEnum {
		t.Error("no enums found")
	}
	if !hasMacro {
		t.Error("no macros found")
	}
	if !hasNamespace {
		t.Error("no namespaces found")
	}
}

func TestCppCallGraphExtractor(t *testing.T) {
	code := `
class Data {
public:
    int value;
};

void helper(int x) {
    // do something
}

int process(Data* d) {
    helper(d->value);
    return d->value + 1;
}

int main() {
    Data* data = new Data();
    data->value = 42;
    int result = process(data);
    delete data;
    return result;
}
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	ewns, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	// Convert to CallGraphEntity
	var cges []CallGraphEntity
	for _, ewn := range ewns {
		cge := ewn.Entity.ToCallGraphEntity()
		cge.Node = ewn.Node
		cges = append(cges, cge)
	}

	// Create call graph extractor
	cge := NewCppCallGraphExtractor(result, cges)
	deps, err := cge.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	// Should have some dependencies
	if len(deps) == 0 {
		t.Error("no dependencies found")
	}

	// Check for expected calls
	hasHelperCall := false
	hasProcessCall := false
	hasNewData := false

	for _, dep := range deps {
		if dep.DepType == Calls {
			if dep.ToName == "helper" {
				hasHelperCall = true
			}
			if dep.ToName == "process" {
				hasProcessCall = true
			}
		}
		if dep.DepType == Instantiates {
			if dep.ToName == "Data" {
				hasNewData = true
			}
		}
	}

	if !hasHelperCall {
		t.Error("expected call to helper not found")
	}
	if !hasProcessCall {
		t.Error("expected call to process not found")
	}
	if !hasNewData {
		t.Error("expected new Data instantiation not found")
	}
}

func TestCppInheritance(t *testing.T) {
	code := `
class Base {
public:
    virtual void foo() {}
};

class Derived : public Base {
public:
    void foo() override {}
};

class Multiple : public Base, public Derived {
public:
    void bar() {}
};
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	ewns, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	// Convert to CallGraphEntity
	var cges []CallGraphEntity
	for _, ewn := range ewns {
		cge := ewn.Entity.ToCallGraphEntity()
		cge.Node = ewn.Node
		cges = append(cges, cge)
	}

	// Create call graph extractor
	cge := NewCppCallGraphExtractor(result, cges)
	deps, err := cge.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	// Check for inheritance dependencies
	hasBaseExtends := false
	for _, dep := range deps {
		if dep.DepType == Extends {
			if dep.ToName == "Base" {
				hasBaseExtends = true
			}
		}
	}

	if !hasBaseExtends {
		t.Error("expected inheritance relationship to Base not found")
	}
}

func TestCppBuiltinFiltering(t *testing.T) {
	// Test builtin type detection
	builtinTypes := []string{"int", "char", "void", "bool", "string", "vector", "unique_ptr"}
	for _, bt := range builtinTypes {
		if !isCppBuiltinType(bt) {
			t.Errorf("expected %q to be recognized as builtin type", bt)
		}
	}

	nonBuiltinTypes := []string{"MyClass", "Point", "Node", "UserData"}
	for _, nt := range nonBuiltinTypes {
		if isCppBuiltinType(nt) {
			t.Errorf("expected %q to NOT be recognized as builtin type", nt)
		}
	}

	// Test builtin function detection
	builtinFuncs := []string{"printf", "malloc", "free", "cout", "make_unique", "move"}
	for _, bf := range builtinFuncs {
		if !isCppBuiltinFunction(bf) {
			t.Errorf("expected %q to be recognized as builtin function", bf)
		}
	}

	nonBuiltinFuncs := []string{"my_function", "process_data", "init_system"}
	for _, nf := range nonBuiltinFuncs {
		if isCppBuiltinFunction(nf) {
			t.Errorf("expected %q to NOT be recognized as builtin function", nf)
		}
	}
}

func TestCppOperatorOverload(t *testing.T) {
	code := `
class Point {
public:
    int x, y;

    Point operator+(const Point& other) const {
        Point result;
        result.x = x + other.x;
        result.y = y + other.y;
        return result;
    }

    bool operator==(const Point& other) const {
        return x == other.x && y == other.y;
    }
};
`
	result := parseCppCode(t, code)
	defer result.Close()

	ext := NewCppExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should extract class and possibly operators
	if len(entities) == 0 {
		t.Fatal("no entities extracted")
	}

	// Check that class is extracted
	hasClass := false
	for _, e := range entities {
		if e.Kind == TypeEntity && e.Name == "Point" {
			hasClass = true
		}
	}

	if !hasClass {
		t.Error("Point class not found")
	}
}

package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseCCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.C)
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

func TestCExtractFunctions(t *testing.T) {
	code := `
int add(int a, int b) {
    return a + b;
}

static double multiply(double x, double y) {
    return x * y;
}

void process(const char *data, size_t len) {
    // process data
}
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(entities) != 3 {
		t.Fatalf("expected 3 functions, got %d", len(entities))
	}

	// Find add function
	var addFunc, multiplyFunc, processFunc *Entity
	for i := range entities {
		switch entities[i].Name {
		case "add":
			addFunc = &entities[i]
		case "multiply":
			multiplyFunc = &entities[i]
		case "process":
			processFunc = &entities[i]
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
	if addFunc.Visibility != VisibilityPublic {
		t.Errorf("add: expected public visibility, got %v", addFunc.Visibility)
	}

	// Check multiply function (static = private)
	if multiplyFunc == nil {
		t.Fatal("multiply function not found")
	}
	if multiplyFunc.Visibility != VisibilityPrivate {
		t.Errorf("multiply: expected private visibility (static), got %v", multiplyFunc.Visibility)
	}
	if len(multiplyFunc.Returns) != 1 || multiplyFunc.Returns[0] != "double" {
		t.Errorf("multiply: expected return type 'double', got %v", multiplyFunc.Returns)
	}

	// Check process function
	if processFunc == nil {
		t.Fatal("process function not found")
	}
	if len(processFunc.Params) != 2 {
		t.Errorf("process: expected 2 params, got %d", len(processFunc.Params))
	}
}

func TestCExtractStructs(t *testing.T) {
	code := `
struct Point {
    int x;
    int y;
};

struct Person {
    char *name;
    int age;
    double height;
};

struct Node {
    int value;
    struct Node *next;
    struct Node *prev;
};
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractStructs()
	if err != nil {
		t.Fatalf("ExtractStructs failed: %v", err)
	}

	if len(entities) != 3 {
		t.Fatalf("expected 3 structs, got %d", len(entities))
	}

	// Find Point struct
	var pointStruct *Entity
	for i := range entities {
		if entities[i].Name == "Point" {
			pointStruct = &entities[i]
			break
		}
	}

	if pointStruct == nil {
		t.Fatal("Point struct not found")
	}

	if pointStruct.Kind != TypeEntity {
		t.Errorf("Point: expected TypeEntity, got %v", pointStruct.Kind)
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
	}
	if !fieldNames["x"] || !fieldNames["y"] {
		t.Errorf("Point: expected fields x and y, got %v", pointStruct.Fields)
	}
}

func TestCExtractUnions(t *testing.T) {
	code := `
union Data {
    int i;
    float f;
    char str[20];
};

union Value {
    int int_val;
    double float_val;
    char *str_val;
};
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractUnions()
	if err != nil {
		t.Fatalf("ExtractUnions failed: %v", err)
	}

	if len(entities) != 2 {
		t.Fatalf("expected 2 unions, got %d", len(entities))
	}

	// Find Data union
	var dataUnion *Entity
	for i := range entities {
		if entities[i].Name == "Data" {
			dataUnion = &entities[i]
			break
		}
	}

	if dataUnion == nil {
		t.Fatal("Data union not found")
	}

	if dataUnion.TypeKind != UnionKind {
		t.Errorf("Data: expected UnionKind, got %v", dataUnion.TypeKind)
	}

	if len(dataUnion.Fields) != 3 {
		t.Errorf("Data: expected 3 fields, got %d", len(dataUnion.Fields))
	}
}

func TestCExtractEnums(t *testing.T) {
	code := `
enum Color {
    RED,
    GREEN,
    BLUE
};

enum Status {
    PENDING = 0,
    ACTIVE = 1,
    COMPLETED = 2,
    CANCELLED = 3
};
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractEnums()
	if err != nil {
		t.Fatalf("ExtractEnums failed: %v", err)
	}

	if len(entities) != 2 {
		t.Fatalf("expected 2 enums, got %d", len(entities))
	}

	// Find Color enum
	var colorEnum *Entity
	for i := range entities {
		if entities[i].Name == "Color" {
			colorEnum = &entities[i]
			break
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

	// Check enum value names
	expectedValues := []string{"RED", "GREEN", "BLUE"}
	for i, ev := range colorEnum.EnumValues {
		if i < len(expectedValues) && ev.Name != expectedValues[i] {
			t.Errorf("Color enum value %d: expected %q, got %q", i, expectedValues[i], ev.Name)
		}
	}

	// Find Status enum and check values
	var statusEnum *Entity
	for i := range entities {
		if entities[i].Name == "Status" {
			statusEnum = &entities[i]
			break
		}
	}

	if statusEnum == nil {
		t.Fatal("Status enum not found")
	}

	if len(statusEnum.EnumValues) != 4 {
		t.Errorf("Status: expected 4 enum values, got %d", len(statusEnum.EnumValues))
	}

	// Check that values are captured
	for _, ev := range statusEnum.EnumValues {
		if ev.Name == "PENDING" && ev.Value != "0" {
			t.Errorf("PENDING: expected value '0', got %q", ev.Value)
		}
	}
}

func TestCExtractTypedefs(t *testing.T) {
	code := `
typedef int INT32;
typedef unsigned char BYTE;
typedef struct Point Point_t;
typedef void (*callback_fn)(int, void*);
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractTypedefs()
	if err != nil {
		t.Fatalf("ExtractTypedefs failed: %v", err)
	}

	// Should have at least INT32 and BYTE
	if len(entities) < 2 {
		t.Fatalf("expected at least 2 typedefs, got %d", len(entities))
	}

	// Find INT32 typedef
	var int32Typedef *Entity
	for i := range entities {
		if entities[i].Name == "INT32" {
			int32Typedef = &entities[i]
			break
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
}

func TestCExtractMacros(t *testing.T) {
	code := `
#define MAX_SIZE 100
#define PI 3.14159
#define EMPTY_STRING ""
#define VERSION "1.0.0"

#define MAX(a, b) ((a) > (b) ? (a) : (b))
#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define SQUARE(x) ((x) * (x))
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractMacros()
	if err != nil {
		t.Fatalf("ExtractMacros failed: %v", err)
	}

	// Should have both simple and function macros
	if len(entities) < 5 {
		t.Fatalf("expected at least 5 macros, got %d", len(entities))
	}

	// Find MAX_SIZE macro
	var maxSizeMacro *Entity
	for i := range entities {
		if entities[i].Name == "MAX_SIZE" {
			maxSizeMacro = &entities[i]
			break
		}
	}

	if maxSizeMacro == nil {
		t.Fatal("MAX_SIZE macro not found")
	}

	if maxSizeMacro.Kind != ConstEntity {
		t.Errorf("MAX_SIZE: expected ConstEntity, got %v", maxSizeMacro.Kind)
	}

	if maxSizeMacro.Value != "100" {
		t.Errorf("MAX_SIZE: expected value '100', got %q", maxSizeMacro.Value)
	}

	// Find MAX function macro
	var maxMacro *Entity
	for i := range entities {
		if entities[i].Name == "MAX" {
			maxMacro = &entities[i]
			break
		}
	}

	if maxMacro == nil {
		t.Fatal("MAX macro not found")
	}

	if maxMacro.Kind != FunctionEntity {
		t.Errorf("MAX: expected FunctionEntity, got %v", maxMacro.Kind)
	}

	if len(maxMacro.Params) != 2 {
		t.Errorf("MAX: expected 2 params, got %d", len(maxMacro.Params))
	}
}

func TestCExtractGlobalVariables(t *testing.T) {
	code := `
int global_count = 0;
static int private_count = 10;
const char *app_name = "MyApp";
double pi = 3.14159;
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractGlobalVariables()
	if err != nil {
		t.Fatalf("ExtractGlobalVariables failed: %v", err)
	}

	if len(entities) < 2 {
		t.Fatalf("expected at least 2 global variables, got %d", len(entities))
	}

	// Find global_count
	var globalCount, privateCount *Entity
	for i := range entities {
		switch entities[i].Name {
		case "global_count":
			globalCount = &entities[i]
		case "private_count":
			privateCount = &entities[i]
		}
	}

	if globalCount != nil {
		if globalCount.Kind != VarEntity {
			t.Errorf("global_count: expected VarEntity, got %v", globalCount.Kind)
		}
		if globalCount.Visibility != VisibilityPublic {
			t.Errorf("global_count: expected public visibility, got %v", globalCount.Visibility)
		}
	}

	// Check static variable has private visibility
	if privateCount != nil {
		if privateCount.Visibility != VisibilityPrivate {
			t.Errorf("private_count: expected private visibility (static), got %v", privateCount.Visibility)
		}
	}
}

func TestCExtractAll(t *testing.T) {
	code := `
#include <stdio.h>

#define MAX_SIZE 100

typedef struct Node {
    int value;
    struct Node *next;
} Node;

enum Status {
    PENDING,
    COMPLETED
};

int global_var = 0;

int add(int a, int b) {
    return a + b;
}

static void helper(void) {
    // helper function
}
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have multiple entity types
	if len(entities) < 5 {
		t.Fatalf("expected at least 5 entities, got %d", len(entities))
	}

	// Check for variety of types
	hasFunction := false
	hasStruct := false
	hasEnum := false
	hasMacro := false

	for _, e := range entities {
		switch e.Kind {
		case FunctionEntity:
			hasFunction = true
		case TypeEntity:
			if e.TypeKind == StructKind {
				hasStruct = true
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
	if !hasStruct {
		t.Error("no structs found")
	}
	if !hasEnum {
		t.Error("no enums found")
	}
	if !hasMacro {
		t.Error("no macros found")
	}
}

func TestCExtractAllEntitiesWithNodes(t *testing.T) {
	code := `
struct Point {
    int x, y;
};

int add(int a, int b) {
    return a + b;
}
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	ewns, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	// Should have entities with nodes
	if len(ewns) < 2 {
		t.Fatalf("expected at least 2 entities with nodes, got %d", len(ewns))
	}

	// Check that each entity has a non-nil node
	for _, ewn := range ewns {
		if ewn.Entity == nil {
			t.Error("found EntityWithNode with nil Entity")
		}
		if ewn.Node == nil {
			t.Errorf("entity %s has nil Node", ewn.Entity.Name)
		}
	}
}

func TestCCallGraphExtractor(t *testing.T) {
	code := `
struct Data {
    int value;
};

void helper(int x) {
    // do something
}

int process(struct Data *d) {
    helper(d->value);
    return d->value + 1;
}

int main(void) {
    struct Data data;
    data.value = 42;
    int result = process(&data);
    return result;
}
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
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
	cge := NewCCallGraphExtractor(result, cges)
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
	hasTypeRef := false

	for _, dep := range deps {
		if dep.DepType == Calls {
			if dep.ToName == "helper" {
				hasHelperCall = true
			}
			if dep.ToName == "process" {
				hasProcessCall = true
			}
		}
		if dep.DepType == UsesType {
			if dep.ToName == "Data" {
				hasTypeRef = true
			}
		}
	}

	if !hasHelperCall {
		t.Error("expected call to helper not found")
	}
	if !hasProcessCall {
		t.Error("expected call to process not found")
	}
	if !hasTypeRef {
		t.Error("expected type reference to Data not found")
	}
}

func TestCVisibilityDetermination(t *testing.T) {
	code := `
// Public function
int public_func(void) { return 0; }

// Private function (static)
static int private_func(void) { return 1; }

// Public variable
int public_var = 0;

// Private variable (static)
static int private_var = 1;
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	for _, e := range entities {
		switch e.Name {
		case "public_func", "public_var":
			if e.Visibility != VisibilityPublic {
				t.Errorf("%s: expected public visibility, got %v", e.Name, e.Visibility)
			}
		case "private_func", "private_var":
			if e.Visibility != VisibilityPrivate {
				t.Errorf("%s: expected private visibility, got %v", e.Name, e.Visibility)
			}
		}
	}
}

func TestCBuiltinFiltering(t *testing.T) {
	// Test builtin type detection
	builtinTypes := []string{"int", "char", "void", "size_t", "int32_t", "FILE"}
	for _, bt := range builtinTypes {
		if !isCBuiltinType(bt) {
			t.Errorf("expected %q to be recognized as builtin type", bt)
		}
	}

	nonBuiltinTypes := []string{"MyStruct", "Point", "Node", "UserData"}
	for _, nt := range nonBuiltinTypes {
		if isCBuiltinType(nt) {
			t.Errorf("expected %q to NOT be recognized as builtin type", nt)
		}
	}

	// Test builtin function detection
	builtinFuncs := []string{"printf", "malloc", "free", "memcpy", "strlen", "strcmp"}
	for _, bf := range builtinFuncs {
		if !isCBuiltinFunction(bf) {
			t.Errorf("expected %q to be recognized as builtin function", bf)
		}
	}

	nonBuiltinFuncs := []string{"my_function", "process_data", "init_system"}
	for _, nf := range nonBuiltinFuncs {
		if isCBuiltinFunction(nf) {
			t.Errorf("expected %q to NOT be recognized as builtin function", nf)
		}
	}
}

func TestCFunctionDeclaration(t *testing.T) {
	// Header file style declarations
	code := `
int add(int a, int b);
void process(const char *data);
static int helper(void);
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	// Should extract function declarations
	if len(entities) < 2 {
		t.Fatalf("expected at least 2 function declarations, got %d", len(entities))
	}

	// Find add declaration
	var addDecl *Entity
	for i := range entities {
		if entities[i].Name == "add" {
			addDecl = &entities[i]
			break
		}
	}

	if addDecl == nil {
		t.Fatal("add function declaration not found")
	}

	if len(addDecl.Params) != 2 {
		t.Errorf("add: expected 2 params, got %d", len(addDecl.Params))
	}
}

func TestCVariadicFunction(t *testing.T) {
	code := `
int printf_wrapper(const char *fmt, ...) {
    // wrapper
    return 0;
}
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("expected 1 function, got %d", len(entities))
	}

	fn := &entities[0]
	if fn.Name != "printf_wrapper" {
		t.Errorf("expected name 'printf_wrapper', got %q", fn.Name)
	}

	// Should have variadic parameter
	hasVariadic := false
	for _, p := range fn.Params {
		if strings.Contains(p.Type, "...") || p.Name == "..." {
			hasVariadic = true
			break
		}
	}

	if !hasVariadic {
		t.Error("expected variadic parameter not found")
	}
}

func TestCNestedStructs(t *testing.T) {
	code := `
struct Outer {
    int outer_field;
    struct Inner {
        int inner_field;
    } inner;
};
`
	result := parseCCode(t, code)
	defer result.Close()

	ext := NewCExtractor(result)
	entities, err := ext.ExtractStructs()
	if err != nil {
		t.Fatalf("ExtractStructs failed: %v", err)
	}

	// Should find Outer struct
	var outerStruct *Entity
	for i := range entities {
		if entities[i].Name == "Outer" {
			outerStruct = &entities[i]
			break
		}
	}

	if outerStruct == nil {
		t.Fatal("Outer struct not found")
	}

	if len(outerStruct.Fields) < 1 {
		t.Errorf("Outer: expected at least 1 field, got %d", len(outerStruct.Fields))
	}
}

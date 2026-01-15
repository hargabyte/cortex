package extract

import (
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

const testRustSource = `
use std::collections::HashMap;
use std::io::{self, Read, Write};

/// A greeting trait
pub trait Greeter {
    fn greet(&self, name: &str) -> String;
    fn farewell(&self) -> String;
}

/// A simple struct with fields
pub struct Person {
    pub name: String,
    age: u32,
}

/// Implementation of methods for Person
impl Person {
    pub fn new(name: String, age: u32) -> Self {
        Self { name, age }
    }

    pub fn get_age(&self) -> u32 {
        self.age
    }

    fn private_method(&mut self) {
        self.age += 1;
    }
}

/// Implement Greeter for Person
impl Greeter for Person {
    fn greet(&self, name: &str) -> String {
        format!("Hello, {}! I'm {}", name, self.name)
    }

    fn farewell(&self) -> String {
        String::from("Goodbye!")
    }
}

/// An enum with different variants
pub enum Status {
    Active,
    Inactive,
    Pending { reason: String },
    Custom(i32),
}

/// A type alias
pub type PersonMap = HashMap<String, Person>;

/// A constant
pub const MAX_SIZE: usize = 100;

/// A static variable
pub static mut COUNTER: u32 = 0;

/// A standalone function
pub fn create_person(name: &str, age: u32) -> Person {
    Person::new(name.to_string(), age)
}

/// An async function
pub async fn async_greet(name: &str) -> String {
    format!("Hello, {}!", name)
}

/// A const function
pub const fn const_add(a: u32, b: u32) -> u32 {
    a + b
}
`

func TestRustExtractor_ExtractAll(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	entities, err := extractor.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	if len(entities) == 0 {
		t.Fatal("expected some entities to be extracted")
	}

	// Verify we found expected entity types
	var (
		hasFunction     bool
		hasMethod       bool
		hasStruct       bool
		hasTrait        bool
		hasEnum         bool
		hasConst        bool
		hasStatic       bool
		hasImport       bool
		hasTypeAlias    bool
		hasAsyncFunc    bool
		hasConstFunc    bool
		hasTraitImpl    bool
	)

	for _, e := range entities {
		switch e.Kind {
		case FunctionEntity:
			hasFunction = true
			// Check for async function
			if e.Name == "async_greet" {
				hasAsyncFunc = true
			}
			// Check for const function
			if e.Name == "const_add" {
				hasConstFunc = true
			}
		case MethodEntity:
			hasMethod = true
			// Check for trait impl method
			if e.Name == "greet" && e.Receiver != "" {
				hasTraitImpl = true
			}
		case TypeEntity:
			if e.TypeKind == StructKind {
				hasStruct = true
			}
			if e.TypeKind == InterfaceKind {
				hasTrait = true
			}
			if e.TypeKind == AliasKind {
				hasTypeAlias = true
			}
		case EnumEntity:
			hasEnum = true
		case ConstEntity:
			hasConst = true
		case VarEntity:
			hasStatic = true
		case ImportEntity:
			hasImport = true
		}
	}

	if !hasFunction {
		t.Error("expected to find at least one function")
	}
	if !hasMethod {
		t.Error("expected to find at least one method")
	}
	if !hasStruct {
		t.Error("expected to find at least one struct")
	}
	if !hasTrait {
		t.Error("expected to find at least one trait")
	}
	if !hasEnum {
		t.Error("expected to find at least one enum")
	}
	if !hasConst {
		t.Error("expected to find at least one constant")
	}
	if !hasStatic {
		t.Error("expected to find at least one static variable")
	}
	if !hasImport {
		t.Error("expected to find at least one import")
	}
	if !hasTypeAlias {
		t.Error("expected to find at least one type alias")
	}
	if !hasAsyncFunc {
		t.Error("expected to find async function")
	}
	if !hasConstFunc {
		t.Error("expected to find const function")
	}
	if !hasTraitImpl {
		t.Error("expected to find trait impl method")
	}
}

func TestRustExtractor_ExtractFunctions(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	funcs, err := extractor.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	// Check that standalone functions were found
	funcNames := make(map[string]bool)
	for _, f := range funcs {
		funcNames[f.Name] = true
	}

	expectedFuncs := []string{"create_person", "async_greet", "const_add"}
	for _, name := range expectedFuncs {
		if !funcNames[name] {
			t.Errorf("expected to find function %q", name)
		}
	}
}

func TestRustExtractor_ExtractStructs(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	structs, err := extractor.ExtractStructs()
	if err != nil {
		t.Fatalf("ExtractStructs failed: %v", err)
	}

	// Find the Person struct
	var person *Entity
	for i := range structs {
		if structs[i].Name == "Person" {
			person = &structs[i]
			break
		}
	}

	if person == nil {
		t.Fatal("expected to find Person struct")
	}

	if person.TypeKind != StructKind {
		t.Errorf("expected struct kind, got %s", person.TypeKind)
	}

	if person.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %s", person.Visibility)
	}

	// Check fields
	if len(person.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(person.Fields))
	}

	fieldNames := make(map[string]bool)
	for _, f := range person.Fields {
		fieldNames[f.Name] = true
	}

	if !fieldNames["name"] || !fieldNames["age"] {
		t.Error("expected to find 'name' and 'age' fields")
	}
}

func TestRustExtractor_ExtractImplBlocks(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	methods, err := extractor.ExtractImplBlocks()
	if err != nil {
		t.Fatalf("ExtractImplBlocks failed: %v", err)
	}

	methodNames := make(map[string]bool)
	for _, m := range methods {
		methodNames[m.Name] = true
	}

	// Check for expected methods
	expectedMethods := []string{"new", "get_age", "private_method", "greet", "farewell"}
	for _, name := range expectedMethods {
		if !methodNames[name] {
			t.Errorf("expected to find method %q", name)
		}
	}

	// Check visibility
	for _, m := range methods {
		if m.Name == "private_method" {
			if m.Visibility != VisibilityPrivate {
				t.Errorf("expected private_method to be private, got %s", m.Visibility)
			}
		}
		if m.Name == "new" {
			if m.Visibility != VisibilityPublic {
				t.Errorf("expected new to be public, got %s", m.Visibility)
			}
		}
	}
}

func TestRustExtractor_ExtractTraits(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	traits, err := extractor.ExtractTraits()
	if err != nil {
		t.Fatalf("ExtractTraits failed: %v", err)
	}

	// Find the Greeter trait
	var greeter *Entity
	for i := range traits {
		if traits[i].Name == "Greeter" {
			greeter = &traits[i]
			break
		}
	}

	if greeter == nil {
		t.Fatal("expected to find Greeter trait")
	}

	if greeter.TypeKind != InterfaceKind {
		t.Errorf("expected interface kind for trait, got %s", greeter.TypeKind)
	}

	if greeter.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %s", greeter.Visibility)
	}

	// Check trait methods
	if len(greeter.Fields) < 2 {
		t.Errorf("expected at least 2 trait methods, got %d", len(greeter.Fields))
	}
}

func TestRustExtractor_ExtractEnums(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	enums, err := extractor.ExtractEnums()
	if err != nil {
		t.Fatalf("ExtractEnums failed: %v", err)
	}

	// Find the Status enum
	var status *Entity
	for i := range enums {
		if enums[i].Name == "Status" {
			status = &enums[i]
			break
		}
	}

	if status == nil {
		t.Fatal("expected to find Status enum")
	}

	if status.Kind != EnumEntity {
		t.Errorf("expected enum kind, got %s", status.Kind)
	}

	if status.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %s", status.Visibility)
	}

	// Check enum variants
	if len(status.EnumValues) != 4 {
		t.Errorf("expected 4 enum variants, got %d", len(status.EnumValues))
	}

	variantNames := make(map[string]bool)
	for _, v := range status.EnumValues {
		variantNames[v.Name] = true
	}

	expectedVariants := []string{"Active", "Inactive", "Pending", "Custom"}
	for _, name := range expectedVariants {
		if !variantNames[name] {
			t.Errorf("expected to find variant %q", name)
		}
	}
}

func TestRustExtractor_ExtractUseStatements(t *testing.T) {
	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(testRustSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	imports, err := extractor.ExtractUseStatements()
	if err != nil {
		t.Fatalf("ExtractUseStatements failed: %v", err)
	}

	if len(imports) == 0 {
		t.Fatal("expected to find at least one import")
	}

	// Check that imports were extracted
	importPaths := make(map[string]bool)
	for _, imp := range imports {
		importPaths[imp.ImportPath] = true
	}

	// We should have HashMap import
	hasHashMap := false
	for path := range importPaths {
		if path == "std::collections::HashMap" {
			hasHashMap = true
			break
		}
	}

	if !hasHashMap {
		t.Error("expected to find std::collections::HashMap import")
	}
}

func TestRustExtractor_Visibility(t *testing.T) {
	source := `
fn private_func() {}
pub fn public_func() {}
pub(crate) fn crate_func() {}
pub(super) fn super_func() {}

struct PrivateStruct {}
pub struct PublicStruct {}

const PRIVATE_CONST: i32 = 1;
pub const PUBLIC_CONST: i32 = 2;
`

	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	entities, err := extractor.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	visibilities := make(map[string]Visibility)
	for _, e := range entities {
		visibilities[e.Name] = e.Visibility
	}

	// Check private items
	if visibilities["private_func"] != VisibilityPrivate {
		t.Errorf("expected private_func to be private, got %s", visibilities["private_func"])
	}
	if visibilities["PrivateStruct"] != VisibilityPrivate {
		t.Errorf("expected PrivateStruct to be private, got %s", visibilities["PrivateStruct"])
	}
	if visibilities["PRIVATE_CONST"] != VisibilityPrivate {
		t.Errorf("expected PRIVATE_CONST to be private, got %s", visibilities["PRIVATE_CONST"])
	}

	// Check public items
	if visibilities["public_func"] != VisibilityPublic {
		t.Errorf("expected public_func to be public, got %s", visibilities["public_func"])
	}
	if visibilities["PublicStruct"] != VisibilityPublic {
		t.Errorf("expected PublicStruct to be public, got %s", visibilities["PublicStruct"])
	}
	if visibilities["PUBLIC_CONST"] != VisibilityPublic {
		t.Errorf("expected PUBLIC_CONST to be public, got %s", visibilities["PUBLIC_CONST"])
	}

	// pub(crate) and pub(super) are treated as public (they allow visibility)
	if visibilities["crate_func"] != VisibilityPublic {
		t.Errorf("expected crate_func to be public, got %s", visibilities["crate_func"])
	}
	if visibilities["super_func"] != VisibilityPublic {
		t.Errorf("expected super_func to be public, got %s", visibilities["super_func"])
	}
}

func TestRustExtractor_SelfReceiver(t *testing.T) {
	source := `
impl Foo {
    fn takes_self(self) {}
    fn takes_ref_self(&self) {}
    fn takes_mut_self(&mut self) {}
    fn no_self() {}
}
`

	p, err := parser.NewParser(parser.Rust)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer result.Close()

	extractor := NewRustExtractor(result)
	methods, err := extractor.ExtractImplBlocks()
	if err != nil {
		t.Fatalf("ExtractImplBlocks failed: %v", err)
	}

	receivers := make(map[string]string)
	for _, m := range methods {
		receivers[m.Name] = m.Receiver
	}

	// Check that self parameter types are captured in receiver
	if receivers["takes_self"] == "" {
		t.Error("expected takes_self to have a receiver")
	}
	if receivers["takes_ref_self"] == "" {
		t.Error("expected takes_ref_self to have a receiver")
	}
	if receivers["takes_mut_self"] == "" {
		t.Error("expected takes_mut_self to have a receiver")
	}
	// no_self still has Foo as receiver (it's an associated function)
	if receivers["no_self"] == "" {
		t.Error("expected no_self to have Foo as receiver")
	}
}

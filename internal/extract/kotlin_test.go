package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseKotlinCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.Kotlin)
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

func TestExtractKotlinFunctions(t *testing.T) {
	code := `package com.example

fun regularFunction(): String {
    return "hello"
}

fun String.extensionFunction(): Int = this.length

suspend fun coroutineFunction() {
    // coroutine code
}

inline fun inlineFunction(block: () -> Unit) {
    block()
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	// Should have 4 functions
	if len(entities) != 4 {
		t.Errorf("expected 4 functions, got %d", len(entities))
	}

	// Find specific functions
	var regularFunc, extensionFunc, suspendFunc, inlineFunc *Entity
	for i := range entities {
		name := entities[i].Name
		if strings.Contains(name, "regularFunction") {
			regularFunc = &entities[i]
		} else if strings.Contains(name, "extensionFunction") {
			extensionFunc = &entities[i]
		} else if strings.Contains(name, "suspend") && strings.Contains(name, "coroutineFunction") {
			suspendFunc = &entities[i]
		} else if strings.Contains(name, "inline") && strings.Contains(name, "inlineFunction") {
			inlineFunc = &entities[i]
		}
	}

	// Check regular function
	if regularFunc == nil {
		t.Error("regularFunction not found")
	} else {
		if len(regularFunc.Returns) != 1 || regularFunc.Returns[0] != "String" {
			t.Errorf("regularFunction: expected return type 'String', got %v", regularFunc.Returns)
		}
	}

	// Check extension function
	if extensionFunc == nil {
		t.Error("extensionFunction not found")
	} else {
		if !strings.Contains(extensionFunc.Receiver, "String") {
			t.Errorf("extensionFunction: expected receiver to contain 'String', got %q", extensionFunc.Receiver)
		}
		if !strings.Contains(extensionFunc.Receiver, "extension") {
			t.Errorf("extensionFunction: expected receiver to contain 'extension', got %q", extensionFunc.Receiver)
		}
	}

	// Check suspend function
	if suspendFunc == nil {
		t.Error("suspendFunction not found")
	} else {
		if !strings.Contains(suspendFunc.Name, "suspend") {
			t.Errorf("suspendFunc: expected name to contain 'suspend', got %q", suspendFunc.Name)
		}
	}

	// Check inline function
	if inlineFunc == nil {
		t.Error("inlineFunction not found")
	} else {
		if !strings.Contains(inlineFunc.Name, "inline") {
			t.Errorf("inlineFunc: expected name to contain 'inline', got %q", inlineFunc.Name)
		}
	}
}

func TestExtractKotlinClasses(t *testing.T) {
	code := `package com.example

class RegularClass {
    fun method() {}
}

data class DataClass(val name: String, val age: Int)

sealed class SealedClass

open class OpenClass

abstract class AbstractClass
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Count classes (not including methods)
	classCount := 0
	var regularClass, dataClass, sealedClass, openClass, abstractClass *Entity
	for i := range entities {
		if entities[i].Kind == TypeEntity {
			classCount++
			if entities[i].Name == "RegularClass" {
				regularClass = &entities[i]
			} else if strings.HasPrefix(entities[i].Name, "DataClass") {
				dataClass = &entities[i]
			} else if entities[i].Name == "SealedClass" {
				sealedClass = &entities[i]
			} else if entities[i].Name == "OpenClass" {
				openClass = &entities[i]
			} else if entities[i].Name == "AbstractClass" {
				abstractClass = &entities[i]
			}
		}
	}

	if classCount != 5 {
		t.Errorf("expected 5 classes, got %d", classCount)
	}

	// Check regular class
	if regularClass == nil {
		t.Error("RegularClass not found")
	} else {
		if regularClass.ValueType != "class" {
			t.Errorf("RegularClass: expected ValueType 'class', got %q", regularClass.ValueType)
		}
	}

	// Check data class
	if dataClass == nil {
		t.Error("DataClass not found")
	} else {
		if !strings.Contains(dataClass.ValueType, "data") {
			t.Errorf("DataClass: expected ValueType to contain 'data', got %q", dataClass.ValueType)
		}
		if len(dataClass.Fields) != 2 {
			t.Errorf("DataClass: expected 2 fields, got %d", len(dataClass.Fields))
		}
	}

	// Check sealed class
	if sealedClass == nil {
		t.Error("SealedClass not found")
	} else {
		if !strings.Contains(sealedClass.ValueType, "sealed") {
			t.Errorf("SealedClass: expected ValueType to contain 'sealed', got %q", sealedClass.ValueType)
		}
	}

	// Check open class
	if openClass == nil {
		t.Error("OpenClass not found")
	} else {
		if !strings.Contains(openClass.ValueType, "open") {
			t.Errorf("OpenClass: expected ValueType to contain 'open', got %q", openClass.ValueType)
		}
	}

	// Check abstract class
	if abstractClass == nil {
		t.Error("AbstractClass not found")
	} else {
		if !strings.Contains(abstractClass.ValueType, "abstract") {
			t.Errorf("AbstractClass: expected ValueType to contain 'abstract', got %q", abstractClass.ValueType)
		}
	}
}

func TestExtractKotlinObjects(t *testing.T) {
	code := `package com.example

object SingletonObject {
    val value = 42
}

class MyClass {
    companion object {
        const val CONSTANT = "value"
    }
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find objects
	var singletonObj, companionObj *Entity
	for i := range entities {
		if entities[i].Kind == TypeEntity {
			if entities[i].Name == "SingletonObject" && entities[i].ValueType == "object" {
				singletonObj = &entities[i]
			} else if entities[i].Name == "Companion" && strings.Contains(entities[i].ValueType, "companion") {
				companionObj = &entities[i]
			}
		}
	}

	// Check singleton object
	if singletonObj == nil {
		t.Error("SingletonObject not found")
	}

	// Check companion object
	if companionObj == nil {
		t.Error("companion object not found")
	}
}

func TestExtractKotlinInterface(t *testing.T) {
	code := `package com.example

interface Repository<T> {
    fun findById(id: String): T?
    fun save(entity: T)
    fun delete(id: String)
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractInterfaces()
	if err != nil {
		t.Fatalf("ExtractInterfaces failed: %v", err)
	}

	// Find the interface entity
	var repo *Entity
	for i := range entities {
		if strings.HasPrefix(entities[i].Name, "Repository") && entities[i].Kind == TypeEntity {
			repo = &entities[i]
			break
		}
	}

	if repo == nil {
		t.Fatal("Repository interface not found")
	}

	if repo.TypeKind != InterfaceKind {
		t.Errorf("expected TypeKind InterfaceKind, got %v", repo.TypeKind)
	}

	if repo.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %v", repo.Visibility)
	}

	// Check methods in interface
	if len(repo.Fields) != 3 {
		t.Errorf("expected 3 methods in interface, got %d", len(repo.Fields))
	}
}

func TestExtractKotlinImports(t *testing.T) {
	code := `package com.example

import kotlin.collections.List
import kotlin.collections.Map
import com.example.util.*
import com.example.service.UserService as UserSvc

class Test
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractImports()
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	// Should have 4 imports
	if len(entities) != 4 {
		t.Errorf("expected 4 imports, got %d", len(entities))
	}

	// Check for wildcard import
	var wildcardImport *Entity
	var aliasImport *Entity
	for i := range entities {
		if strings.HasSuffix(entities[i].ImportPath, ".*") {
			wildcardImport = &entities[i]
		}
		if entities[i].ImportAlias != "" {
			aliasImport = &entities[i]
		}
	}

	if wildcardImport == nil {
		t.Error("wildcard import not found")
	}

	if aliasImport == nil {
		t.Error("aliased import not found")
	} else {
		if aliasImport.ImportAlias != "UserSvc" {
			t.Errorf("expected import alias 'UserSvc', got %q", aliasImport.ImportAlias)
		}
	}
}

func TestExtractKotlinInheritance(t *testing.T) {
	code := `package com.example

open class Animal(val name: String)

interface Runnable {
    fun run()
}

interface Comparable<T> {
    fun compareTo(other: T): Int
}

class Dog(name: String, val breed: String) : Animal(name), Runnable, Comparable<Dog> {
    override fun run() {}

    override fun compareTo(other: Dog): Int {
        return 0
    }
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find Dog class
	var dogClass *Entity
	for i := range entities {
		if strings.HasPrefix(entities[i].Name, "Dog") && entities[i].Kind == TypeEntity {
			dogClass = &entities[i]
			break
		}
	}

	if dogClass == nil {
		t.Fatal("Dog class not found")
	}

	// Check inheritance (stored in Receiver)
	if dogClass.Receiver != "Animal" {
		t.Errorf("expected superclass 'Animal', got %q", dogClass.Receiver)
	}

	// Check implements
	if len(dogClass.Implements) < 2 {
		t.Errorf("expected at least 2 implemented interfaces, got %d: %v", len(dogClass.Implements), dogClass.Implements)
	}
}

func TestExtractKotlinProperties(t *testing.T) {
	code := `package com.example

class Constants {
    val immutableValue: String = "test"
    var mutableValue: Int = 0
    const val CONSTANT = "constant"

    private val privateValue = 42
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find properties (should be VarEntity or ConstEntity)
	var immutableProp, mutableProp, constProp, privateProp *Entity
	for i := range entities {
		if entities[i].Kind == ConstEntity || entities[i].Kind == VarEntity {
			switch entities[i].Name {
			case "immutableValue":
				immutableProp = &entities[i]
			case "mutableValue":
				mutableProp = &entities[i]
			case "CONSTANT":
				constProp = &entities[i]
			case "privateValue":
				privateProp = &entities[i]
			}
		}
	}

	// Check immutableValue (val = const)
	if immutableProp == nil {
		t.Error("immutableValue not found")
	} else {
		if immutableProp.Kind != ConstEntity {
			t.Errorf("immutableValue: expected ConstEntity, got %v", immutableProp.Kind)
		}
		if immutableProp.ValueType != "String" {
			t.Errorf("immutableValue: expected type 'String', got %q", immutableProp.ValueType)
		}
	}

	// Check mutableValue (var = variable)
	if mutableProp == nil {
		t.Error("mutableValue not found")
	} else {
		if mutableProp.Kind != VarEntity {
			t.Errorf("mutableValue: expected VarEntity, got %v", mutableProp.Kind)
		}
	}

	// Check CONSTANT (const val = const)
	if constProp == nil {
		t.Error("CONSTANT not found")
	} else {
		if constProp.Kind != ConstEntity {
			t.Errorf("CONSTANT: expected ConstEntity, got %v", constProp.Kind)
		}
	}

	// Check privateValue
	if privateProp == nil {
		t.Error("privateValue not found")
	} else {
		if privateProp.Visibility != VisibilityPrivate {
			t.Errorf("privateValue: expected private visibility, got %v", privateProp.Visibility)
		}
	}
}

func TestExtractKotlinAllWithNodes(t *testing.T) {
	code := `package com.example

import kotlin.collections.List

class Service {
    private val name: String = "service"

    fun process() {}
}
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	ewns, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	// Should have entities with nodes
	if len(ewns) < 3 {
		t.Fatalf("expected at least 3 entities with nodes, got %d", len(ewns))
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

func TestKotlinVisibilityDetermination(t *testing.T) {
	tests := []struct {
		modifiers []string
		expected  Visibility
	}{
		{[]string{"public"}, VisibilityPublic},
		{[]string{"private"}, VisibilityPrivate},
		{[]string{"protected"}, VisibilityPrivate},
		{[]string{"internal"}, VisibilityPrivate},
		{[]string{}, VisibilityPublic}, // default is public in Kotlin
		{[]string{"open"}, VisibilityPublic},
		{[]string{"private", "open"}, VisibilityPrivate},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.modifiers, "_"), func(t *testing.T) {
			got := determineKotlinVisibility(tt.modifiers)
			if got != tt.expected {
				t.Errorf("determineKotlinVisibility(%v) = %v, want %v", tt.modifiers, got, tt.expected)
			}
		})
	}
}

func TestKotlinImportNameExtraction(t *testing.T) {
	tests := []struct {
		importPath string
		expected   string
	}{
		{"kotlin.collections.List", "List"},
		{"kotlin.collections.Map", "Map"},
		{"com.example.service.UserService", "UserService"},
		{"kotlin.collections.*", "collections.*"},
		{"com.example.*", "example.*"},
	}

	for _, tt := range tests {
		t.Run(tt.importPath, func(t *testing.T) {
			got := extractKotlinImportName(tt.importPath)
			if got != tt.expected {
				t.Errorf("extractKotlinImportName(%q) = %q, want %q", tt.importPath, got, tt.expected)
			}
		})
	}
}

func TestExtractKotlinDataClass(t *testing.T) {
	code := `package com.example

data class User(
    val id: String,
    val name: String,
    var email: String,
    val age: Int = 0
)
`
	result := parseKotlinCode(t, code)
	defer result.Close()

	ext := NewKotlinExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find User class
	var userClass *Entity
	for i := range entities {
		if strings.HasPrefix(entities[i].Name, "User") && entities[i].Kind == TypeEntity {
			userClass = &entities[i]
			break
		}
	}

	if userClass == nil {
		t.Fatal("User class not found")
	}

	// Check that it's a data class
	if !strings.Contains(userClass.ValueType, "data") {
		t.Errorf("expected ValueType to contain 'data', got %q", userClass.ValueType)
	}

	// Check fields (primary constructor parameters)
	if len(userClass.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(userClass.Fields))
	}
}

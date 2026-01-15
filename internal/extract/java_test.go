package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseJavaCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.Java)
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

func TestExtractJavaClass(t *testing.T) {
	code := `package com.example;

public class User {
    private String id;
    private String email;
    private int age;

    public User(String id, String email) {
        this.id = id;
        this.email = email;
    }

    public String getId() {
        return id;
    }

    public void setEmail(String email) {
        this.email = email;
    }
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have class + constructor + 2 methods + 3 fields
	if len(entities) < 4 {
		t.Fatalf("expected at least 4 entities, got %d", len(entities))
	}

	// Find the class entity
	var userClass *Entity
	for i := range entities {
		if entities[i].Name == "User" && entities[i].Kind == TypeEntity {
			userClass = &entities[i]
			break
		}
	}

	if userClass == nil {
		t.Fatal("User class not found")
	}

	if userClass.TypeKind != StructKind {
		t.Errorf("expected TypeKind StructKind, got %v", userClass.TypeKind)
	}

	if userClass.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %v", userClass.Visibility)
	}

	// Check fields
	if len(userClass.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(userClass.Fields))
	}
}

func TestExtractJavaInterface(t *testing.T) {
	code := `package com.example;

public interface Repository<T> {
    T findById(String id);
    void save(T entity);
    void delete(String id);
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
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

func TestExtractJavaEnum(t *testing.T) {
	code := `package com.example;

public enum Status {
    PENDING,
    ACTIVE,
    COMPLETED,
    CANCELLED
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find the enum entity
	var status *Entity
	for i := range entities {
		if entities[i].Name == "Status" && entities[i].Kind == EnumEntity {
			status = &entities[i]
			break
		}
	}

	if status == nil {
		t.Fatal("Status enum not found")
	}

	if status.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %v", status.Visibility)
	}

	// Check enum values
	if len(status.EnumValues) != 4 {
		t.Errorf("expected 4 enum values, got %d", len(status.EnumValues))
	}

	expectedValues := []string{"PENDING", "ACTIVE", "COMPLETED", "CANCELLED"}
	for i, ev := range status.EnumValues {
		if i < len(expectedValues) && ev.Name != expectedValues[i] {
			t.Errorf("enum value %d: expected %q, got %q", i, expectedValues[i], ev.Name)
		}
	}
}

func TestExtractJavaImports(t *testing.T) {
	code := `package com.example;

import java.util.List;
import java.util.Map;
import static org.junit.Assert.assertEquals;
import com.example.util.*;

public class Test {}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Count imports
	importCount := 0
	var staticImport *Entity
	var wildcardImport *Entity
	for i := range entities {
		if entities[i].Kind == ImportEntity {
			importCount++
			if entities[i].ImportAlias == "static" {
				staticImport = &entities[i]
			}
			if strings.HasSuffix(entities[i].ImportPath, ".*") {
				wildcardImport = &entities[i]
			}
		}
	}

	if importCount != 4 {
		t.Errorf("expected 4 imports, got %d", importCount)
	}

	if staticImport == nil {
		t.Error("static import not found")
	}

	if wildcardImport == nil {
		t.Error("wildcard import not found")
	} else if wildcardImport.ImportPath != "com.example.util.*" {
		t.Errorf("expected import path 'com.example.util.*', got %q", wildcardImport.ImportPath)
	}
}

func TestExtractJavaMethod(t *testing.T) {
	code := `package com.example;

public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }

    public static double multiply(double x, double y) {
        return x * y;
    }

    private void reset() {
        // reset state
    }
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find methods
	var addMethod, multiplyMethod, resetMethod *Entity
	for i := range entities {
		if entities[i].Kind == MethodEntity {
			switch entities[i].Name {
			case "add":
				addMethod = &entities[i]
			case "multiply":
				multiplyMethod = &entities[i]
			case "reset":
				resetMethod = &entities[i]
			}
		}
	}

	// Check add method
	if addMethod == nil {
		t.Fatal("add method not found")
	}
	if addMethod.Visibility != VisibilityPublic {
		t.Errorf("add: expected public visibility, got %v", addMethod.Visibility)
	}
	if len(addMethod.Params) != 2 {
		t.Errorf("add: expected 2 params, got %d", len(addMethod.Params))
	}
	if len(addMethod.Returns) != 1 || addMethod.Returns[0] != "int" {
		t.Errorf("add: expected return type 'int', got %v", addMethod.Returns)
	}
	if addMethod.Receiver != "Calculator" {
		t.Errorf("add: expected receiver 'Calculator', got %q", addMethod.Receiver)
	}

	// Check multiply method (static)
	if multiplyMethod == nil {
		t.Fatal("multiply method not found")
	}
	if !strings.Contains(multiplyMethod.Receiver, "static") {
		t.Errorf("multiply: expected receiver to contain 'static', got %q", multiplyMethod.Receiver)
	}

	// Check reset method (private)
	if resetMethod == nil {
		t.Fatal("reset method not found")
	}
	if resetMethod.Visibility != VisibilityPrivate {
		t.Errorf("reset: expected private visibility, got %v", resetMethod.Visibility)
	}
}

func TestExtractJavaGenerics(t *testing.T) {
	code := `package com.example;

public class Box<T> {
    private T content;

    public T get() {
        return content;
    }

    public void set(T content) {
        this.content = content;
    }
}

public class Pair<K, V> {
    private K key;
    private V value;
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find Box class
	var boxClass *Entity
	for i := range entities {
		if strings.HasPrefix(entities[i].Name, "Box") && entities[i].Kind == TypeEntity {
			boxClass = &entities[i]
			break
		}
	}

	if boxClass == nil {
		t.Fatal("Box class not found")
	}

	// Check that the name includes type parameter
	if !strings.Contains(boxClass.Name, "<T>") {
		t.Errorf("expected Box name to contain '<T>', got %q", boxClass.Name)
	}
}

func TestExtractJavaInheritance(t *testing.T) {
	code := `package com.example;

public class Animal {
    protected String name;
}

public class Dog extends Animal implements Runnable, Comparable<Dog> {
    private String breed;

    @Override
    public void run() {}

    @Override
    public int compareTo(Dog other) {
        return 0;
    }
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find Dog class
	var dogClass *Entity
	for i := range entities {
		if entities[i].Name == "Dog" && entities[i].Kind == TypeEntity {
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
		t.Errorf("expected at least 2 implemented interfaces, got %d", len(dogClass.Implements))
	}
}

func TestExtractJavaAbstractClass(t *testing.T) {
	code := `package com.example;

public abstract class Shape {
    protected String color;

    public abstract double area();

    public void setColor(String color) {
        this.color = color;
    }
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find Shape class
	var shapeClass *Entity
	for i := range entities {
		if entities[i].Name == "Shape" && entities[i].Kind == TypeEntity {
			shapeClass = &entities[i]
			break
		}
	}

	if shapeClass == nil {
		t.Fatal("Shape class not found")
	}

	// Check that it's marked as abstract
	if !strings.Contains(shapeClass.ValueType, "abstract") {
		t.Errorf("expected ValueType to contain 'abstract', got %q", shapeClass.ValueType)
	}
}

func TestExtractJavaConstructor(t *testing.T) {
	code := `package com.example;

public class Person {
    private String name;
    private int age;

    public Person() {
        this.name = "Unknown";
        this.age = 0;
    }

    public Person(String name, int age) {
        this.name = name;
        this.age = age;
    }
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Count constructors
	constructorCount := 0
	for i := range entities {
		if entities[i].Kind == MethodEntity && strings.Contains(entities[i].Receiver, "constructor") {
			constructorCount++
		}
	}

	if constructorCount != 2 {
		t.Errorf("expected 2 constructors, got %d", constructorCount)
	}
}

func TestExtractJavaFields(t *testing.T) {
	code := `package com.example;

public class Constants {
    public static final String VERSION = "1.0.0";
    private static int counter = 0;
    protected double value;
    String name;
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find fields
	var versionField, counterField, valueField, nameField *Entity
	for i := range entities {
		if entities[i].Kind == ConstEntity || entities[i].Kind == VarEntity {
			switch entities[i].Name {
			case "VERSION":
				versionField = &entities[i]
			case "counter":
				counterField = &entities[i]
			case "value":
				valueField = &entities[i]
			case "name":
				nameField = &entities[i]
			}
		}
	}

	// Check VERSION (public static final = constant)
	if versionField == nil {
		t.Fatal("VERSION field not found")
	}
	if versionField.Kind != ConstEntity {
		t.Errorf("VERSION: expected ConstEntity, got %v", versionField.Kind)
	}
	if versionField.Visibility != VisibilityPublic {
		t.Errorf("VERSION: expected public visibility, got %v", versionField.Visibility)
	}
	if !strings.Contains(versionField.Receiver, "static") {
		t.Errorf("VERSION: expected receiver to contain 'static', got %q", versionField.Receiver)
	}

	// Check counter (private static = variable)
	if counterField == nil {
		t.Fatal("counter field not found")
	}
	if counterField.Kind != VarEntity {
		t.Errorf("counter: expected VarEntity, got %v", counterField.Kind)
	}
	if counterField.Visibility != VisibilityPrivate {
		t.Errorf("counter: expected private visibility, got %v", counterField.Visibility)
	}

	// Check value (protected)
	if valueField == nil {
		t.Fatal("value field not found")
	}
	if valueField.Visibility != VisibilityPrivate {
		// protected is treated as private
		t.Errorf("value: expected private visibility (protected), got %v", valueField.Visibility)
	}

	// Check name (package-private)
	if nameField == nil {
		t.Fatal("name field not found")
	}
	if nameField.Visibility != VisibilityPrivate {
		// package-private is treated as private
		t.Errorf("name: expected private visibility (package-private), got %v", nameField.Visibility)
	}
}

func TestExtractAllWithNodes(t *testing.T) {
	code := `package com.example;

import java.util.List;

public class Service {
    private String name;

    public void process() {}
}
`
	result := parseJavaCode(t, code)
	defer result.Close()

	ext := NewJavaExtractor(result)
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

func TestJavaVisibilityDetermination(t *testing.T) {
	tests := []struct {
		modifiers []string
		expected  Visibility
	}{
		{[]string{"public"}, VisibilityPublic},
		{[]string{"private"}, VisibilityPrivate},
		{[]string{"protected"}, VisibilityPrivate},
		{[]string{}, VisibilityPrivate}, // package-private
		{[]string{"public", "static"}, VisibilityPublic},
		{[]string{"private", "final"}, VisibilityPrivate},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.modifiers, "_"), func(t *testing.T) {
			got := determineJavaVisibility(tt.modifiers)
			if got != tt.expected {
				t.Errorf("determineJavaVisibility(%v) = %v, want %v", tt.modifiers, got, tt.expected)
			}
		})
	}
}

func TestJavaImportNameExtraction(t *testing.T) {
	tests := []struct {
		importPath string
		expected   string
	}{
		{"java.util.List", "List"},
		{"java.util.Map", "Map"},
		{"com.example.service.UserService", "UserService"},
		{"java.util.*", "util.*"},
		{"com.example.*", "example.*"},
	}

	for _, tt := range tests {
		t.Run(tt.importPath, func(t *testing.T) {
			got := extractJavaImportName(tt.importPath)
			if got != tt.expected {
				t.Errorf("extractJavaImportName(%q) = %q, want %q", tt.importPath, got, tt.expected)
			}
		})
	}
}

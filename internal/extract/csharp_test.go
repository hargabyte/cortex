package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseCSharpCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.CSharp)
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

func TestExtractCSharpClass(t *testing.T) {
	code := `using System;

namespace MyApp.Models
{
    public class User
    {
        private string id;
        private string email;
        private int age;

        public User(string id, string email)
        {
            this.id = id;
            this.email = email;
        }

        public string GetId()
        {
            return id;
        }

        public void SetEmail(string email)
        {
            this.email = email;
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have class + constructor + 2 methods + 3 fields + using
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

func TestExtractCSharpInterface(t *testing.T) {
	code := `namespace MyApp.Repositories
{
    public interface IRepository<T>
    {
        T FindById(string id);
        void Save(T entity);
        void Delete(string id);
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find the interface entity
	var repo *Entity
	for i := range entities {
		if strings.HasPrefix(entities[i].Name, "IRepository") && entities[i].Kind == TypeEntity {
			repo = &entities[i]
			break
		}
	}

	if repo == nil {
		t.Fatal("IRepository interface not found")
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

func TestExtractCSharpStruct(t *testing.T) {
	code := `namespace MyApp.Models
{
    public readonly struct Point
    {
        public int X { get; }
        public int Y { get; }

        public Point(int x, int y)
        {
            X = x;
            Y = y;
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractStructs()
	if err != nil {
		t.Fatalf("ExtractStructs failed: %v", err)
	}

	// Find the struct entity
	var pointStruct *Entity
	for i := range entities {
		if entities[i].Name == "Point" && entities[i].Kind == TypeEntity {
			pointStruct = &entities[i]
			break
		}
	}

	if pointStruct == nil {
		t.Fatal("Point struct not found")
	}

	if pointStruct.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %v", pointStruct.Visibility)
	}
}

func TestExtractCSharpEnum(t *testing.T) {
	code := `namespace MyApp.Models
{
    public enum Status
    {
        Pending,
        Active,
        Completed,
        Cancelled
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
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

	expectedValues := []string{"Pending", "Active", "Completed", "Cancelled"}
	for i, ev := range status.EnumValues {
		if i < len(expectedValues) && ev.Name != expectedValues[i] {
			t.Errorf("enum value %d: expected %q, got %q", i, expectedValues[i], ev.Name)
		}
	}
}

func TestExtractCSharpUsings(t *testing.T) {
	code := `using System;
using System.Collections.Generic;
using static System.Console;
using MyAlias = System.Text.StringBuilder;

namespace MyApp
{
    public class Test { }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Count usings
	usingCount := 0
	var staticUsing *Entity
	var aliasUsing *Entity
	for i := range entities {
		if entities[i].Kind == ImportEntity {
			usingCount++
			if entities[i].ImportAlias == "static" {
				staticUsing = &entities[i]
			}
			if entities[i].ImportAlias == "MyAlias" {
				aliasUsing = &entities[i]
			}
		}
	}

	if usingCount != 4 {
		t.Errorf("expected 4 usings, got %d", usingCount)
	}

	if staticUsing == nil {
		t.Error("static using not found")
	}

	if aliasUsing == nil {
		t.Error("alias using not found")
	} else if aliasUsing.ImportPath != "System.Text.StringBuilder" {
		t.Errorf("expected import path 'System.Text.StringBuilder', got %q", aliasUsing.ImportPath)
	}
}

func TestExtractCSharpMethod(t *testing.T) {
	code := `namespace MyApp
{
    public class Calculator
    {
        public int Add(int a, int b)
        {
            return a + b;
        }

        public static double Multiply(double x, double y)
        {
            return x * y;
        }

        private void Reset()
        {
            // reset state
        }

        public async Task<int> ComputeAsync()
        {
            return await Task.FromResult(42);
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find methods
	var addMethod, multiplyMethod, resetMethod, computeAsyncMethod *Entity
	for i := range entities {
		if entities[i].Kind == MethodEntity {
			switch entities[i].Name {
			case "Add":
				addMethod = &entities[i]
			case "Multiply":
				multiplyMethod = &entities[i]
			case "Reset":
				resetMethod = &entities[i]
			case "ComputeAsync":
				computeAsyncMethod = &entities[i]
			}
		}
	}

	// Check Add method
	if addMethod == nil {
		t.Fatal("Add method not found")
	}
	if addMethod.Visibility != VisibilityPublic {
		t.Errorf("Add: expected public visibility, got %v", addMethod.Visibility)
	}
	if len(addMethod.Params) != 2 {
		t.Errorf("Add: expected 2 params, got %d", len(addMethod.Params))
	}
	if len(addMethod.Returns) != 1 || addMethod.Returns[0] != "int" {
		t.Errorf("Add: expected return type 'int', got %v", addMethod.Returns)
	}
	if addMethod.Receiver != "Calculator" {
		t.Errorf("Add: expected receiver 'Calculator', got %q", addMethod.Receiver)
	}

	// Check Multiply method (static)
	if multiplyMethod == nil {
		t.Fatal("Multiply method not found")
	}
	if !strings.Contains(multiplyMethod.Receiver, "static") {
		t.Errorf("Multiply: expected receiver to contain 'static', got %q", multiplyMethod.Receiver)
	}

	// Check Reset method (private)
	if resetMethod == nil {
		t.Fatal("Reset method not found")
	}
	if resetMethod.Visibility != VisibilityPrivate {
		t.Errorf("Reset: expected private visibility, got %v", resetMethod.Visibility)
	}

	// Check ComputeAsync method (async)
	if computeAsyncMethod == nil {
		t.Fatal("ComputeAsync method not found")
	}
	if !computeAsyncMethod.IsAsync {
		t.Error("ComputeAsync: expected IsAsync to be true")
	}
}

func TestExtractCSharpGenerics(t *testing.T) {
	code := `namespace MyApp
{
    public class Box<T>
    {
        private T content;

        public T Get()
        {
            return content;
        }

        public void Set(T content)
        {
            this.content = content;
        }
    }

    public class Pair<K, V>
    {
        private K key;
        private V value;
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
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

func TestExtractCSharpInheritance(t *testing.T) {
	code := `namespace MyApp
{
    public class Animal
    {
        protected string Name { get; set; }
    }

    public class Dog : Animal, IRunnable, IComparable<Dog>
    {
        private string breed;

        public void Run() { }

        public int CompareTo(Dog other)
        {
            return 0;
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
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

	// Check inheritance (stored in Receiver for base class)
	if dogClass.Receiver != "Animal" {
		t.Errorf("expected base class 'Animal', got %q", dogClass.Receiver)
	}

	// Check implements (interfaces)
	if len(dogClass.Implements) < 2 {
		t.Errorf("expected at least 2 implemented interfaces, got %d", len(dogClass.Implements))
	}
}

func TestExtractCSharpAbstractClass(t *testing.T) {
	code := `namespace MyApp
{
    public abstract class Shape
    {
        protected string Color { get; set; }

        public abstract double Area();

        public void SetColor(string color)
        {
            Color = color;
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
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

func TestExtractCSharpConstructor(t *testing.T) {
	code := `namespace MyApp
{
    public class Person
    {
        private string name;
        private int age;

        public Person()
        {
            name = "Unknown";
            age = 0;
        }

        public Person(string name, int age)
        {
            this.name = name;
            this.age = age;
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
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

func TestExtractCSharpFields(t *testing.T) {
	code := `namespace MyApp
{
    public class Constants
    {
        public const string Version = "1.0.0";
        private static int counter = 0;
        protected readonly double value;
        internal string name;
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find fields
	var versionField, counterField, valueField, nameField *Entity
	for i := range entities {
		if entities[i].Kind == ConstEntity || entities[i].Kind == VarEntity {
			switch entities[i].Name {
			case "Version":
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

	// Check Version (public const = constant)
	if versionField == nil {
		t.Fatal("Version field not found")
	}
	if versionField.Kind != ConstEntity {
		t.Errorf("Version: expected ConstEntity, got %v", versionField.Kind)
	}
	if versionField.Visibility != VisibilityPublic {
		t.Errorf("Version: expected public visibility, got %v", versionField.Visibility)
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
	if !strings.Contains(counterField.Receiver, "static") {
		t.Errorf("counter: expected receiver to contain 'static', got %q", counterField.Receiver)
	}

	// Check value (protected readonly = constant)
	if valueField == nil {
		t.Fatal("value field not found")
	}
	if valueField.Kind != ConstEntity {
		// readonly should be treated as constant
		t.Errorf("value: expected ConstEntity (readonly), got %v", valueField.Kind)
	}
	if valueField.Visibility != VisibilityPrivate {
		// protected is treated as private
		t.Errorf("value: expected private visibility (protected), got %v", valueField.Visibility)
	}

	// Check name (internal)
	if nameField == nil {
		t.Fatal("name field not found")
	}
	if nameField.Visibility != VisibilityPrivate {
		// internal is treated as private
		t.Errorf("name: expected private visibility (internal), got %v", nameField.Visibility)
	}
}

func TestExtractCSharpProperties(t *testing.T) {
	code := `namespace MyApp
{
    public class Person
    {
        public string Name { get; set; }
        public int Age { get; private set; }
        public static int Count { get; }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find properties
	var nameProperty, ageProperty, countProperty *Entity
	for i := range entities {
		if entities[i].Kind == VarEntity && strings.Contains(entities[i].Receiver, "property") {
			switch entities[i].Name {
			case "Name":
				nameProperty = &entities[i]
			case "Age":
				ageProperty = &entities[i]
			case "Count":
				countProperty = &entities[i]
			}
		}
	}

	// Check Name property
	if nameProperty == nil {
		t.Fatal("Name property not found")
	}
	if nameProperty.Visibility != VisibilityPublic {
		t.Errorf("Name: expected public visibility, got %v", nameProperty.Visibility)
	}

	// Check Age property
	if ageProperty == nil {
		t.Fatal("Age property not found")
	}

	// Check Count property (static)
	if countProperty == nil {
		t.Fatal("Count property not found")
	}
	if !strings.Contains(countProperty.Receiver, "static") {
		t.Errorf("Count: expected receiver to contain 'static', got %q", countProperty.Receiver)
	}
}

func TestExtractCSharpRecord(t *testing.T) {
	code := `namespace MyApp
{
    public record Person(string Name, int Age);

    public record Employee(string Name, int Age, string Department) : Person(Name, Age);
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	entities, err := ext.ExtractRecords()
	if err != nil {
		t.Fatalf("ExtractRecords failed: %v", err)
	}

	// Should find at least one record
	if len(entities) == 0 {
		t.Fatal("no records found")
	}

	// Find Person record
	var personRecord *Entity
	for i := range entities {
		if entities[i].Name == "Person" && entities[i].Kind == TypeEntity {
			personRecord = &entities[i]
			break
		}
	}

	if personRecord == nil {
		t.Fatal("Person record not found")
	}

	if personRecord.ValueType != "record" {
		t.Errorf("expected ValueType 'record', got %q", personRecord.ValueType)
	}
}

func TestCSharpExtractAllWithNodes(t *testing.T) {
	code := `using System;

namespace MyApp
{
    public class Service
    {
        private string name;

        public void Process() { }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
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

func TestCSharpVisibilityDetermination(t *testing.T) {
	tests := []struct {
		modifiers []string
		expected  Visibility
	}{
		{[]string{"public"}, VisibilityPublic},
		{[]string{"private"}, VisibilityPrivate},
		{[]string{"protected"}, VisibilityPrivate},
		{[]string{"internal"}, VisibilityPrivate},
		{[]string{}, VisibilityPrivate}, // default is private
		{[]string{"public", "static"}, VisibilityPublic},
		{[]string{"private", "readonly"}, VisibilityPrivate},
		{[]string{"protected", "internal"}, VisibilityPrivate}, // protected internal
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.modifiers, "_"), func(t *testing.T) {
			got := determineCSharpVisibility(tt.modifiers)
			if got != tt.expected {
				t.Errorf("determineCSharpVisibility(%v) = %v, want %v", tt.modifiers, got, tt.expected)
			}
		})
	}
}

func TestCSharpNamespaceNameExtraction(t *testing.T) {
	tests := []struct {
		namespacePath string
		expected      string
	}{
		{"System", "System"},
		{"System.Collections.Generic", "Generic"},
		{"MyApp.Models.User", "User"},
		{"Microsoft.Extensions.DependencyInjection", "DependencyInjection"},
	}

	for _, tt := range tests {
		t.Run(tt.namespacePath, func(t *testing.T) {
			got := extractCSharpNamespaceName(tt.namespacePath)
			if got != tt.expected {
				t.Errorf("extractCSharpNamespaceName(%q) = %q, want %q", tt.namespacePath, got, tt.expected)
			}
		})
	}
}

func TestCSharpCallGraphExtractor(t *testing.T) {
	code := `using System;

namespace MyApp
{
    public class UserService
    {
        private readonly IRepository repository;

        public UserService(IRepository repo)
        {
            repository = repo;
        }

        public User GetUser(string id)
        {
            var user = repository.FindById(id);
            Console.WriteLine("Got user");
            return user;
        }

        public void CreateUser(string name)
        {
            var user = new User(name);
            repository.Save(user);
        }
    }
}
`
	result := parseCSharpCode(t, code)
	defer result.Close()

	ext := NewCSharpExtractor(result)
	ewns, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	// Convert to CallGraphEntity
	var cgEntities []CallGraphEntity
	for _, ewn := range ewns {
		cge := ewn.Entity.ToCallGraphEntity()
		cge.Node = ewn.Node
		cgEntities = append(cgEntities, cge)
	}

	// Create call graph extractor
	cge := NewCSharpCallGraphExtractor(result, cgEntities)
	deps, err := cge.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	// Should find some dependencies
	if len(deps) == 0 {
		t.Error("expected to find some dependencies")
	}

	// Check for specific dependencies
	hasMethodCall := false
	hasObjectCreation := false
	for _, dep := range deps {
		if dep.DepType == Calls {
			if dep.ToName == "FindById" || dep.ToName == "Save" {
				hasMethodCall = true
			}
			if dep.ToName == "User" {
				hasObjectCreation = true
			}
		}
	}

	if !hasMethodCall {
		t.Error("expected to find method call dependencies")
	}

	if !hasObjectCreation {
		t.Error("expected to find object creation dependency")
	}
}

func TestCSharpBuiltinTypes(t *testing.T) {
	cge := &CSharpCallGraphExtractor{}

	builtins := []string{
		"int", "string", "bool", "object", "void",
		"Int32", "String", "Boolean", "Object",
		"List", "Dictionary", "HashSet",
		"Console", "Math", "Array",
		"Exception", "Task",
	}

	for _, typ := range builtins {
		if !cge.isCSharpBuiltinType(typ) {
			t.Errorf("expected %q to be a builtin type", typ)
		}
	}

	nonBuiltins := []string{
		"User", "UserService", "MyCustomClass",
		"IRepository", "AppConfig",
	}

	for _, typ := range nonBuiltins {
		if cge.isCSharpBuiltinType(typ) {
			t.Errorf("expected %q to NOT be a builtin type", typ)
		}
	}
}

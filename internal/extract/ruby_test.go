package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseRubyCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.Ruby)
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

func TestRubyExtractMethod(t *testing.T) {
	code := `def regular_method(arg)
  puts arg
end

def self.class_method
  "class method"
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	methods, err := ext.ExtractMethods()
	if err != nil {
		t.Fatalf("ExtractMethods failed: %v", err)
	}

	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}

	// Check regular method
	regularMethod := methods[0]
	if regularMethod.Name != "regular_method" {
		t.Errorf("expected name 'regular_method', got %q", regularMethod.Name)
	}
	if regularMethod.Kind != FunctionEntity {
		t.Errorf("expected kind FunctionEntity, got %v", regularMethod.Kind)
	}
	if regularMethod.Language != "ruby" {
		t.Errorf("expected language 'ruby', got %q", regularMethod.Language)
	}
	if len(regularMethod.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(regularMethod.Params))
	} else if regularMethod.Params[0].Name != "arg" {
		t.Errorf("expected param 'arg', got %q", regularMethod.Params[0].Name)
	}

	// Check class method
	classMethod := methods[1]
	if classMethod.Name != "class_method" {
		t.Errorf("expected name 'class_method', got %q", classMethod.Name)
	}
	if classMethod.Kind != MethodEntity {
		t.Errorf("expected kind MethodEntity, got %v", classMethod.Kind)
	}
	if classMethod.Receiver != "self" {
		t.Errorf("expected receiver 'self', got %q", classMethod.Receiver)
	}
}

func TestRubyExtractClass(t *testing.T) {
	code := `class MyClass
  attr_accessor :name

  def initialize(name)
    @name = name
  end

  def greet
    puts "Hello, #{@name}"
  end

  private

  def private_method
    "secret"
  end
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Should have 1 class and 3 methods
	classCount := 0
	methodCount := 0
	for _, e := range classes {
		if e.Kind == TypeEntity {
			classCount++
		} else if e.Kind == MethodEntity {
			methodCount++
		}
	}

	if classCount != 1 {
		t.Errorf("expected 1 class, got %d", classCount)
	}
	if methodCount != 3 {
		t.Errorf("expected 3 methods, got %d", methodCount)
	}

	// Find the class
	var myClass *Entity
	for i := range classes {
		if classes[i].Name == "MyClass" && classes[i].Kind == TypeEntity {
			myClass = &classes[i]
			break
		}
	}

	if myClass == nil {
		t.Fatal("MyClass not found")
	}

	if myClass.TypeKind != StructKind {
		t.Errorf("expected TypeKind 'struct', got %v", myClass.TypeKind)
	}
}

func TestRubyExtractClassWithInheritance(t *testing.T) {
	code := `class Child < Parent
  def child_method
    super
  end
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find the class
	var child *Entity
	for i := range classes {
		if classes[i].Name == "Child" && classes[i].Kind == TypeEntity {
			child = &classes[i]
			break
		}
	}

	if child == nil {
		t.Fatal("Child class not found")
	}

	// Check inheritance
	if len(child.Implements) != 1 {
		t.Errorf("expected 1 superclass, got %d: %v", len(child.Implements), child.Implements)
	} else {
		if child.Implements[0] != "Parent" {
			t.Errorf("expected superclass 'Parent', got %q", child.Implements[0])
		}
	}
}

func TestRubyExtractModule(t *testing.T) {
	code := `module MyModule
  VERSION = "1.0.0"

  def module_method
    "from module"
  end
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	modules, err := ext.ExtractModules()
	if err != nil {
		t.Fatalf("ExtractModules failed: %v", err)
	}

	// Should have 1 module and 1 method
	moduleCount := 0
	methodCount := 0
	for _, e := range modules {
		if e.Kind == TypeEntity {
			moduleCount++
		} else if e.Kind == MethodEntity {
			methodCount++
		}
	}

	if moduleCount != 1 {
		t.Errorf("expected 1 module, got %d", moduleCount)
	}
	if methodCount != 1 {
		t.Errorf("expected 1 method, got %d", methodCount)
	}

	// Find the module
	var myModule *Entity
	for i := range modules {
		if modules[i].Name == "MyModule" && modules[i].Kind == TypeEntity {
			myModule = &modules[i]
			break
		}
	}

	if myModule == nil {
		t.Fatal("MyModule not found")
	}

	if myModule.TypeKind != InterfaceKind {
		t.Errorf("expected TypeKind 'interface', got %v", myModule.TypeKind)
	}
}

func TestRubyExtractConstants(t *testing.T) {
	code := `MAX_RETRIES = 5
DEFAULT_TIMEOUT = "30s"
BATCH_SIZE = 100

# This should not be extracted (lowercase)
some_variable = 42
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	consts, err := ext.ExtractConstants()
	if err != nil {
		t.Fatalf("ExtractConstants failed: %v", err)
	}

	if len(consts) < 3 {
		t.Fatalf("expected at least 3 constants, got %d", len(consts))
	}

	// Check MAX_RETRIES
	var maxRetries *Entity
	for i := range consts {
		if consts[i].Name == "MAX_RETRIES" {
			maxRetries = &consts[i]
			break
		}
	}

	if maxRetries == nil {
		t.Error("MAX_RETRIES not found")
	} else {
		if maxRetries.Kind != ConstEntity {
			t.Errorf("expected kind ConstEntity, got %v", maxRetries.Kind)
		}
		if maxRetries.Value != "5" {
			t.Errorf("expected value '5', got %q", maxRetries.Value)
		}
	}
}

func TestRubyExtractMethodParameters(t *testing.T) {
	code := `def process(required, optional = nil, *args, keyword:, **kwargs, &block)
  # method body
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	methods, err := ext.ExtractMethods()
	if err != nil {
		t.Fatalf("ExtractMethods failed: %v", err)
	}

	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}

	method := methods[0]
	if method.Name != "process" {
		t.Errorf("expected name 'process', got %q", method.Name)
	}

	// Check parameters
	expectedParams := []string{"required", "optional", "*args", "keyword:", "**kwargs", "&block"}
	if len(method.Params) != len(expectedParams) {
		t.Errorf("expected %d params, got %d", len(expectedParams), len(method.Params))
	}

	for i, expected := range expectedParams {
		if i < len(method.Params) {
			if method.Params[i].Name != expected {
				t.Errorf("param %d: expected %q, got %q", i, expected, method.Params[i].Name)
			}
		}
	}
}

func TestRubyExtractAll(t *testing.T) {
	code := `# Test module
require 'json'

MAX_RETRIES = 5

class Config
  attr_accessor :name

  def initialize(name)
    @name = name
  end

  def get_name
    @name
  end
end

def main
  puts "Hello"
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Count by kind
	counts := make(map[EntityKind]int)
	for _, e := range entities {
		counts[e.Kind]++
	}

	if counts[ConstEntity] < 1 {
		t.Errorf("expected at least 1 constant, got %d", counts[ConstEntity])
	}
	if counts[TypeEntity] < 1 {
		t.Errorf("expected at least 1 class, got %d", counts[TypeEntity])
	}
	if counts[FunctionEntity]+counts[MethodEntity] < 1 {
		t.Errorf("expected at least 1 function/method, got %d", counts[FunctionEntity]+counts[MethodEntity])
	}
}

func TestRubyPrivateVisibility(t *testing.T) {
	code := `def _private_method
  "private"
end

def public_method
  "public"
end

class _PrivateClass
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	for _, e := range entities {
		if strings.HasPrefix(e.Name, "_") && e.Visibility != VisibilityPrivate {
			t.Errorf("%s: expected private visibility, got %v", e.Name, e.Visibility)
		}
		if !strings.HasPrefix(e.Name, "_") && e.Visibility != VisibilityPublic {
			t.Errorf("%s: expected public visibility, got %v", e.Name, e.Visibility)
		}
	}
}

func TestRubyDetermineVisibility(t *testing.T) {
	tests := []struct {
		name     string
		expected Visibility
	}{
		{"public_method", VisibilityPublic},
		{"_private_method", VisibilityPrivate},
		{"PublicClass", VisibilityPublic},
		{"_PrivateClass", VisibilityPrivate},
		{"", VisibilityPrivate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineRubyVisibility(tt.name)
			if got != tt.expected {
				t.Errorf("determineRubyVisibility(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestRubyGenerateEntityID(t *testing.T) {
	entity := &Entity{
		Kind:      MethodEntity,
		Name:      "login_user",
		File:      "app/models/user.rb",
		StartLine: 42,
		Language:  "ruby",
	}

	id := entity.GenerateEntityID()

	// Should start with sa-fn- (MethodEntity uses "fn" type code)
	if !strings.HasPrefix(id, "sa-fn-") {
		t.Errorf("expected ID to start with 'sa-fn-', got %q", id)
	}

	// Should contain the name
	if !strings.HasSuffix(id, "-login_user") {
		t.Errorf("expected ID to end with '-login_user', got %q", id)
	}

	// Line number should be in the ID
	if !strings.Contains(id, "-42-") {
		t.Errorf("expected ID to contain line number '-42-', got %q", id)
	}
}

func TestRubyExtractClassMethods(t *testing.T) {
	code := `class Service
  def self.create(name)
    new(name)
  end

  def initialize(name)
    @name = name
  end

  def instance_method
    @name
  end
end
`
	result := parseRubyCode(t, code)
	defer result.Close()

	ext := NewRubyExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find methods
	var classMethod, instanceMethod, initMethod *Entity
	for i := range classes {
		if classes[i].Kind == MethodEntity {
			switch classes[i].Name {
			case "create":
				classMethod = &classes[i]
			case "initialize":
				initMethod = &classes[i]
			case "instance_method":
				instanceMethod = &classes[i]
			}
		}
	}

	// Check class method
	if classMethod == nil {
		t.Error("class method 'create' not found")
	} else {
		if classMethod.Receiver != "self" && classMethod.Receiver != "Service" {
			t.Errorf("expected class method receiver 'self' or 'Service', got %q", classMethod.Receiver)
		}
	}

	// Check instance methods
	if initMethod == nil {
		t.Error("instance method 'initialize' not found")
	} else {
		if initMethod.Receiver != "Service" {
			t.Errorf("expected instance method receiver 'Service', got %q", initMethod.Receiver)
		}
	}

	if instanceMethod == nil {
		t.Error("instance method 'instance_method' not found")
	} else {
		if instanceMethod.Receiver != "Service" {
			t.Errorf("expected instance method receiver 'Service', got %q", instanceMethod.Receiver)
		}
	}
}

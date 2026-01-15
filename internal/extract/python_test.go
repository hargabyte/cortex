package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parsePythonCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.Python)
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

func TestPythonExtractFunction(t *testing.T) {
	code := `def login_user(email: str, password: str) -> Optional[User]:
    """Login a user with email and password."""
    return None
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	if fn.Name != "login_user" {
		t.Errorf("expected name 'login_user', got %q", fn.Name)
	}
	if fn.Kind != FunctionEntity {
		t.Errorf("expected kind FunctionEntity, got %v", fn.Kind)
	}
	if fn.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %v", fn.Visibility)
	}
	if fn.Language != "python" {
		t.Errorf("expected language 'python', got %q", fn.Language)
	}

	// Check parameters
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	} else {
		if fn.Params[0].Name != "email" || fn.Params[0].Type != "str" {
			t.Errorf("param 0: expected email:str, got %s:%s", fn.Params[0].Name, fn.Params[0].Type)
		}
		if fn.Params[1].Name != "password" || fn.Params[1].Type != "str" {
			t.Errorf("param 1: expected password:str, got %s:%s", fn.Params[1].Name, fn.Params[1].Type)
		}
	}

	// Check return type
	if len(fn.Returns) != 1 {
		t.Errorf("expected 1 return, got %d", len(fn.Returns))
	} else {
		if fn.Returns[0] != "Optional[User]" {
			t.Errorf("return: expected 'Optional[User]', got %q", fn.Returns[0])
		}
	}
}

func TestPythonExtractAsyncFunction(t *testing.T) {
	code := `async def fetch_data(url: str) -> Dict[str, str]:
    """Async function to fetch data."""
    pass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	if fn.Name != "fetch_data" {
		t.Errorf("expected name 'fetch_data', got %q", fn.Name)
	}
	if !fn.IsAsync {
		t.Error("expected async function, got non-async")
	}
}

func TestPythonExtractClass(t *testing.T) {
	code := `class User:
    """User model."""

    email: str
    password: str
    age: Optional[int]

    def __init__(self, email: str):
        self.email = email

    def get_email(self) -> str:
        return self.email
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Should have 1 class and 2 methods
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
	if methodCount != 2 {
		t.Errorf("expected 2 methods, got %d", methodCount)
	}

	// Find the class
	var userClass *Entity
	for i := range classes {
		if classes[i].Name == "User" && classes[i].Kind == TypeEntity {
			userClass = &classes[i]
			break
		}
	}

	if userClass == nil {
		t.Fatal("User class not found")
	}

	if userClass.TypeKind != StructKind {
		t.Errorf("expected TypeKind 'struct', got %v", userClass.TypeKind)
	}

	// Check fields
	if len(userClass.Fields) < 2 {
		t.Errorf("expected at least 2 fields, got %d", len(userClass.Fields))
	}

	// Check for email field
	hasEmail := false
	for _, f := range userClass.Fields {
		if f.Name == "email" && f.Type == "str" {
			hasEmail = true
			break
		}
	}
	if !hasEmail {
		t.Error("expected email:str field")
	}
}

func TestPythonExtractClassWithInheritance(t *testing.T) {
	code := `class Child(Parent, Mixin):
    """Child class with multiple inheritance."""
    pass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
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
	if len(child.Implements) != 2 {
		t.Errorf("expected 2 base classes, got %d: %v", len(child.Implements), child.Implements)
	} else {
		if child.Implements[0] != "Parent" {
			t.Errorf("expected first base 'Parent', got %q", child.Implements[0])
		}
		if child.Implements[1] != "Mixin" {
			t.Errorf("expected second base 'Mixin', got %q", child.Implements[1])
		}
	}
}

func TestPythonExtractDecoratedClass(t *testing.T) {
	code := `@dataclass
class Config:
    """Configuration dataclass."""

    name: str
    value: int
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find the class
	var config *Entity
	for i := range classes {
		if classes[i].Name == "Config" && classes[i].Kind == TypeEntity {
			config = &classes[i]
			break
		}
	}

	if config == nil {
		t.Fatal("Config class not found")
	}

	// Check decorators
	if len(config.Decorators) != 1 {
		t.Errorf("expected 1 decorator, got %d", len(config.Decorators))
	} else if config.Decorators[0] != "dataclass" {
		t.Errorf("expected decorator 'dataclass', got %q", config.Decorators[0])
	}
}

func TestPythonExtractMethods(t *testing.T) {
	code := `class Service:
    @classmethod
    def create(cls, name: str) -> "Service":
        return cls()

    @staticmethod
    def validate(value: str) -> bool:
        return True

    @property
    def name(self) -> str:
        return self._name

    def process(self, data: bytes) -> None:
        pass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// Find methods
	methods := make(map[string]*Entity)
	for i := range classes {
		if classes[i].Kind == MethodEntity {
			methods[classes[i].Name] = &classes[i]
		}
	}

	// Check classmethod - cls should be skipped
	if create, ok := methods["create"]; ok {
		if len(create.Params) != 1 {
			t.Errorf("create: expected 1 param (cls skipped), got %d", len(create.Params))
		}
		if len(create.Decorators) != 1 || create.Decorators[0] != "classmethod" {
			t.Errorf("create: expected classmethod decorator, got %v", create.Decorators)
		}
	} else {
		t.Error("create method not found")
	}

	// Check staticmethod - no self/cls
	if validate, ok := methods["validate"]; ok {
		if len(validate.Params) != 1 {
			t.Errorf("validate: expected 1 param, got %d", len(validate.Params))
		}
	} else {
		t.Error("validate method not found")
	}

	// Check property
	if name, ok := methods["name"]; ok {
		if len(name.Decorators) != 1 || name.Decorators[0] != "property" {
			t.Errorf("name: expected property decorator, got %v", name.Decorators)
		}
	} else {
		t.Error("name property not found")
	}

	// Check regular method - self should be skipped
	if process, ok := methods["process"]; ok {
		if len(process.Params) != 1 {
			t.Errorf("process: expected 1 param (self skipped), got %d: %v", len(process.Params), process.Params)
		}
	} else {
		t.Error("process method not found")
	}
}

func TestPythonExtractConstants(t *testing.T) {
	code := `MAX_RETRIES: int = 5
DEFAULT_TIMEOUT = "30s"
BATCH_SIZE: int = 100

# This should not be extracted (lowercase)
some_variable = 42
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	consts, err := ext.ExtractConstants()
	if err != nil {
		t.Fatalf("ExtractConstants failed: %v", err)
	}

	if len(consts) < 2 {
		t.Fatalf("expected at least 2 constants, got %d", len(consts))
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
		if maxRetries.ValueType != "int" {
			t.Errorf("expected type 'int', got %q", maxRetries.ValueType)
		}
	}
}

func TestPythonExtractImports(t *testing.T) {
	code := `import os
import sys
from typing import Dict, List, Optional
from dataclasses import dataclass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	imports, err := ext.ExtractImports()
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	if len(imports) < 4 {
		t.Fatalf("expected at least 4 imports, got %d", len(imports))
	}

	// Check for os import
	hasOs := false
	for _, imp := range imports {
		if imp.Name == "os" {
			hasOs = true
			break
		}
	}
	if !hasOs {
		t.Error("os import not found")
	}

	// Check for from import
	hasDict := false
	for _, imp := range imports {
		if imp.Name == "Dict" {
			hasDict = true
			if !strings.Contains(imp.ImportPath, "typing") {
				t.Errorf("expected Dict import path to contain 'typing', got %q", imp.ImportPath)
			}
			break
		}
	}
	if !hasDict {
		t.Error("Dict import not found")
	}
}

func TestPythonExtractAll(t *testing.T) {
	code := `"""Test module."""

import os

MAX_RETRIES = 5

class Config:
    name: str

    def __init__(self, name: str):
        self.name = name

    def get_name(self) -> str:
        return self.name

def main() -> None:
    pass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Count by kind
	counts := make(map[EntityKind]int)
	for _, e := range entities {
		counts[e.Kind]++
	}

	if counts[ImportEntity] < 1 {
		t.Errorf("expected at least 1 import, got %d", counts[ImportEntity])
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

func TestPythonPrivateVisibility(t *testing.T) {
	code := `def _private_func():
    pass

def __very_private():
    pass

class _PrivateClass:
    pass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	for _, e := range entities {
		if strings.HasPrefix(e.Name, "_") && e.Visibility != VisibilityPrivate {
			t.Errorf("%s: expected private visibility, got %v", e.Name, e.Visibility)
		}
	}
}

func TestPythonDetermineVisibility(t *testing.T) {
	tests := []struct {
		name     string
		expected Visibility
	}{
		{"public_func", VisibilityPublic},
		{"_private_func", VisibilityPrivate},
		{"__very_private", VisibilityPrivate},
		{"PublicClass", VisibilityPublic},
		{"_PrivateClass", VisibilityPrivate},
		{"", VisibilityPrivate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determinePythonVisibility(tt.name)
			if got != tt.expected {
				t.Errorf("determinePythonVisibility(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestPythonTypeHints(t *testing.T) {
	code := `def process(
    data: List[Dict[str, Any]],
    callback: Callable[[int], None],
    optional: Optional[str] = None
) -> Tuple[bool, str]:
    pass
`
	result := parsePythonCode(t, code)
	defer result.Close()

	ext := NewPythonExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]

	// Check complex type hints are preserved
	if len(fn.Params) < 2 {
		t.Errorf("expected at least 2 params, got %d", len(fn.Params))
	}

	// Check return type
	if len(fn.Returns) != 1 {
		t.Errorf("expected 1 return, got %d", len(fn.Returns))
	}
}

func TestPythonGenerateEntityID(t *testing.T) {
	entity := &Entity{
		Kind:      FunctionEntity,
		Name:      "login_user",
		File:      "src/auth/login.py",
		StartLine: 42,
		Language:  "python",
	}

	id := entity.GenerateEntityID()

	// Should start with sa-fn-
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

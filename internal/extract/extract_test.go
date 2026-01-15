package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseGoCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.Go)
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

func TestExtractFunction(t *testing.T) {
	code := `package main

func LoginUser(email string, password string) (*User, error) {
	return nil, nil
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	if fn.Name != "LoginUser" {
		t.Errorf("expected name 'LoginUser', got %q", fn.Name)
	}
	if fn.Kind != FunctionEntity {
		t.Errorf("expected kind FunctionEntity, got %v", fn.Kind)
	}
	if fn.Visibility != VisibilityPublic {
		t.Errorf("expected public visibility, got %v", fn.Visibility)
	}

	// Check parameters
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	} else {
		if fn.Params[0].Name != "email" || fn.Params[0].Type != "string" {
			t.Errorf("param 0: expected email:string, got %s:%s", fn.Params[0].Name, fn.Params[0].Type)
		}
		if fn.Params[1].Name != "password" || fn.Params[1].Type != "string" {
			t.Errorf("param 1: expected password:string, got %s:%s", fn.Params[1].Name, fn.Params[1].Type)
		}
	}

	// Check returns
	if len(fn.Returns) != 2 {
		t.Errorf("expected 2 returns, got %d", len(fn.Returns))
	} else {
		if fn.Returns[0] != "*User" {
			t.Errorf("return 0: expected '*User', got %q", fn.Returns[0])
		}
		if fn.Returns[1] != "error" {
			t.Errorf("return 1: expected 'error', got %q", fn.Returns[1])
		}
	}

	// Check compact description format
	desc := fn.ToCompactDescription()
	if !strings.Contains(desc, "(email: string, password: string) -> (*User, error)") {
		t.Errorf("compact description missing expected signature, got: %s", desc)
	}
}

func TestExtractMethod(t *testing.T) {
	code := `package main

type Server struct {}

func (s *Server) HandleRequest(ctx context.Context, req *Request) (*Response, error) {
	return nil, nil
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	if fn.Name != "HandleRequest" {
		t.Errorf("expected name 'HandleRequest', got %q", fn.Name)
	}
	if fn.Kind != MethodEntity {
		t.Errorf("expected kind MethodEntity, got %v", fn.Kind)
	}
	if fn.Receiver != "*Server" {
		t.Errorf("expected receiver '*Server', got %q", fn.Receiver)
	}

	// Check compact description contains receiver
	desc := fn.ToCompactDescription()
	if !strings.Contains(desc, "|r=*Server") {
		t.Errorf("compact description missing receiver, got: %s", desc)
	}
}

func TestExtractStructType(t *testing.T) {
	code := `package main

type User struct {
	ID       string
	Email    string
	Password string
	Age      int64
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	types, err := ext.ExtractTypes()
	if err != nil {
		t.Fatalf("ExtractTypes failed: %v", err)
	}

	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}

	typ := types[0]
	if typ.Name != "User" {
		t.Errorf("expected name 'User', got %q", typ.Name)
	}
	if typ.Kind != TypeEntity {
		t.Errorf("expected kind TypeEntity, got %v", typ.Kind)
	}
	if typ.TypeKind != StructKind {
		t.Errorf("expected TypeKind 'struct', got %v", typ.TypeKind)
	}

	// Check fields
	if len(typ.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(typ.Fields))
	}

	expectedFields := map[string]string{
		"ID":       "string",
		"Email":    "string",
		"Password": "string",
		"Age":      "int64",
	}
	for _, f := range typ.Fields {
		expected, ok := expectedFields[f.Name]
		if !ok {
			t.Errorf("unexpected field: %s", f.Name)
			continue
		}
		if f.Type != expected {
			t.Errorf("field %s: expected type %q, got %q", f.Name, expected, f.Type)
		}
	}

	// Check compact description
	desc := typ.ToCompactDescription()
	if !strings.Contains(desc, "|struct|") {
		t.Errorf("compact description missing struct kind, got: %s", desc)
	}
	if !strings.Contains(desc, "ID: string") {
		t.Errorf("compact description missing ID field, got: %s", desc)
	}
}

func TestExtractInterfaceType(t *testing.T) {
	code := `package main

type Repository interface {
	Get(ctx context.Context, id string) (*Entity, error)
	Save(ctx context.Context, entity *Entity) error
	Delete(id string) error
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	types, err := ext.ExtractTypes()
	if err != nil {
		t.Fatalf("ExtractTypes failed: %v", err)
	}

	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}

	typ := types[0]
	if typ.Name != "Repository" {
		t.Errorf("expected name 'Repository', got %q", typ.Name)
	}
	if typ.TypeKind != InterfaceKind {
		t.Errorf("expected TypeKind 'interface', got %v", typ.TypeKind)
	}

	// Check methods
	if len(typ.Fields) != 3 {
		t.Errorf("expected 3 methods, got %d", len(typ.Fields))
	}

	// Check compact description
	desc := typ.ToCompactDescription()
	if !strings.Contains(desc, "|interface|") {
		t.Errorf("compact description missing interface kind, got: %s", desc)
	}
}

func TestExtractConstants(t *testing.T) {
	code := `package main

const MaxRetries = 5
const DefaultTimeout = "30s"
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	consts, err := ext.ExtractConstants()
	if err != nil {
		t.Fatalf("ExtractConstants failed: %v", err)
	}

	if len(consts) != 2 {
		t.Fatalf("expected 2 constants, got %d", len(consts))
	}

	// Check MaxRetries
	var maxRetries, defaultTimeout *Entity
	for i := range consts {
		if consts[i].Name == "MaxRetries" {
			maxRetries = &consts[i]
		} else if consts[i].Name == "DefaultTimeout" {
			defaultTimeout = &consts[i]
		}
	}

	if maxRetries == nil {
		t.Error("MaxRetries not found")
	} else if maxRetries.Kind != ConstEntity {
		t.Errorf("expected kind ConstEntity, got %v", maxRetries.Kind)
	}

	if defaultTimeout == nil {
		t.Error("DefaultTimeout not found")
	}
}

func TestExtractImports(t *testing.T) {
	code := `package main

import (
	"fmt"
	"context"
	errors "github.com/pkg/errors"
	_ "net/http/pprof"
)
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	imports, err := ext.ExtractImports()
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	if len(imports) != 4 {
		t.Fatalf("expected 4 imports, got %d", len(imports))
	}

	// Find import with alias
	var errorsImport *Entity
	for i := range imports {
		if imports[i].ImportPath == "github.com/pkg/errors" {
			errorsImport = &imports[i]
			break
		}
	}

	if errorsImport == nil {
		t.Error("errors import not found")
	} else if errorsImport.ImportAlias != "errors" {
		t.Errorf("expected alias 'errors', got %q", errorsImport.ImportAlias)
	}

	// Check compact description for aliased import
	if errorsImport != nil {
		desc := errorsImport.ToCompactDescription()
		if !strings.Contains(desc, "|errors") {
			t.Errorf("compact description missing alias, got: %s", desc)
		}
	}
}

func TestExtractAll(t *testing.T) {
	code := `package main

import "fmt"

const Version = "1.0.0"

type Config struct {
	Name string
}

func (c *Config) String() string {
	return c.Name
}

func main() {
	fmt.Println("hello")
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have: 1 import, 1 constant, 1 type, 2 functions (String and main)
	if len(entities) < 5 {
		t.Errorf("expected at least 5 entities, got %d", len(entities))
	}

	// Count by kind
	counts := make(map[EntityKind]int)
	for _, e := range entities {
		counts[e.Kind]++
	}

	if counts[ImportEntity] != 1 {
		t.Errorf("expected 1 import, got %d", counts[ImportEntity])
	}
	if counts[ConstEntity] != 1 {
		t.Errorf("expected 1 constant, got %d", counts[ConstEntity])
	}
	if counts[TypeEntity] != 1 {
		t.Errorf("expected 1 type, got %d", counts[TypeEntity])
	}
	if counts[FunctionEntity]+counts[MethodEntity] != 2 {
		t.Errorf("expected 2 functions/methods, got %d", counts[FunctionEntity]+counts[MethodEntity])
	}
}

func TestTypeAbbreviations(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"string", "string"},
		{"int64", "int64"},
		{"float64", "float64"},
		{"bool", "bool"},
		{"error", "error"},
		{"interface{}", "interface{}"},
		{"any", "any"},
		{"context.Context", "context.Context"},
		{"[]byte", "[]byte"},
		{"*string", "*string"},
		{"[]string", "[]string"},
		{"map[string]int", "map[string]int"},
		{"*User", "*User"},
		{"time.Duration", "time.Duration"},
		{"time.Time", "time.Time"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := abbreviateType(tt.input)
			if got != tt.expected {
				t.Errorf("abbreviateType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateEntityID(t *testing.T) {
	entity := &Entity{
		Kind:      FunctionEntity,
		Name:      "LoginUser",
		File:      "src/auth/login.go",
		StartLine: 42,
	}

	id := entity.GenerateEntityID()

	// Should start with sa-fn-
	if !strings.HasPrefix(id, "sa-fn-") {
		t.Errorf("expected ID to start with 'sa-fn-', got %q", id)
	}

	// Should contain the name
	if !strings.HasSuffix(id, "-LoginUser") {
		t.Errorf("expected ID to end with '-LoginUser', got %q", id)
	}

	// Should have 5 parts: sa, type, pathHash, line, name
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 parts in ID, got %d: %q", len(parts), id)
	}

	// Line number should be in the ID
	if !strings.Contains(id, "-42-") {
		t.Errorf("expected ID to contain line number '-42-', got %q", id)
	}
}

func TestCompactDescriptionFormat(t *testing.T) {
	// Test function format
	fn := &Entity{
		Kind:       FunctionEntity,
		Name:       "LoginUser",
		File:       "src/auth/login.go",
		StartLine:  45,
		EndLine:    89,
		Params:     []Param{{Name: "email", Type: "string"}, {Name: "pass", Type: "string"}},
		Returns:    []string{"*User", "error"},
		SigHash:    "a7f9",
		BodyHash:   "e3d4",
		Visibility: VisibilityPublic,
	}

	desc := fn.ToCompactDescription()
	expected := "src/auth/login.go:45-89|(email: string, pass: string) -> (*User, error)|a7f9:e3d4|v=pub"
	if desc != expected {
		t.Errorf("function compact description:\ngot:  %q\nwant: %q", desc, expected)
	}

	// Test type format
	typ := &Entity{
		Kind:      TypeEntity,
		Name:      "User",
		File:      "src/auth/types.go",
		StartLine: 5,
		EndLine:   15,
		TypeKind:  StructKind,
		Fields:    []Field{{Name: "ID", Type: "string"}, {Name: "Email", Type: "string"}},
		SigHash:   "b8c3",
	}

	desc = typ.ToCompactDescription()
	expected = "src/auth/types.go:5-15|struct|{ID: string, Email: string}|b8c3"
	if desc != expected {
		t.Errorf("type compact description:\ngot:  %q\nwant: %q", desc, expected)
	}
}

func TestMethodWithReceiver(t *testing.T) {
	code := `package main

type UserService struct {
	db Database
}

func (s *UserService) GetUser(id string) (*User, error) {
	return nil, nil
}

func (s UserService) String() string {
	return "UserService"
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(funcs))
	}

	// Check pointer receiver
	var getUser, toString *Entity
	for i := range funcs {
		if funcs[i].Name == "GetUser" {
			getUser = &funcs[i]
		} else if funcs[i].Name == "String" {
			toString = &funcs[i]
		}
	}

	if getUser == nil {
		t.Fatal("GetUser method not found")
	}
	if getUser.Receiver != "*UserService" {
		t.Errorf("GetUser receiver: expected '*UserService', got %q", getUser.Receiver)
	}

	if toString == nil {
		t.Fatal("String method not found")
	}
	if toString.Receiver != "UserService" {
		t.Errorf("String receiver: expected 'UserService', got %q", toString.Receiver)
	}
}

func TestVariadicParameters(t *testing.T) {
	code := `package main

func Printf(format string, args ...interface{}) {
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	}

	// Check variadic parameter
	if len(fn.Params) >= 2 {
		if fn.Params[1].Type != "...interface{}" {
			t.Errorf("expected variadic param type '...interface{}', got %q", fn.Params[1].Type)
		}
	}
}

func TestDetermineVisibility(t *testing.T) {
	tests := []struct {
		name     string
		expected Visibility
	}{
		{"PublicFunc", VisibilityPublic},
		{"privateFunc", VisibilityPrivate},
		{"User", VisibilityPublic},
		{"user", VisibilityPrivate},
		{"_private", VisibilityPrivate},
		{"", VisibilityPrivate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineVisibility(tt.name)
			if got != tt.expected {
				t.Errorf("DetermineVisibility(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestEmbeddedFields(t *testing.T) {
	code := `package main

type Base struct {
	ID string
}

type Extended struct {
	Base
	*Logger
	Name string
}
`
	result := parseGoCode(t, code)
	defer result.Close()

	ext := NewExtractor(result)
	types, err := ext.ExtractTypes()
	if err != nil {
		t.Fatalf("ExtractTypes failed: %v", err)
	}

	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}

	var extended *Entity
	for i := range types {
		if types[i].Name == "Extended" {
			extended = &types[i]
			break
		}
	}

	if extended == nil {
		t.Fatal("Extended type not found")
	}

	// Should have 3 fields: Base (embedded), Logger (embedded pointer), Name
	if len(extended.Fields) < 2 {
		t.Errorf("expected at least 2 fields, got %d", len(extended.Fields))
	}

	// Check for embedded Base field
	hasBase := false
	for _, f := range extended.Fields {
		if f.Name == "Base" {
			hasBase = true
			break
		}
	}
	if !hasBase {
		t.Error("expected embedded Base field")
	}
}

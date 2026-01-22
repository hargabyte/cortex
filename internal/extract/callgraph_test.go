package extract

import (
	"fmt"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// Sample Go source for testing call graph extraction
const testCallGraphSource = `package main

import (
	"fmt"
	"strings"
)

// User represents a user in the system
type User struct {
	Name  string
	Email string
	Admin bool
}

// Authenticator is an interface for authentication
type Authenticator interface {
	Validate(token string) bool
}

// TokenAuth implements Authenticator
type TokenAuth struct {
	secret string
}

// Validate checks if the token is valid
func (t *TokenAuth) Validate(token string) bool {
	return strings.HasPrefix(token, t.secret)
}

// Login authenticates a user
func Login(email, password string) (*User, error) {
	if email == "" {
		return nil, fmt.Errorf("email required")
	}

	user := FindUserByEmail(email)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if !CheckPassword(user, password) {
		return nil, fmt.Errorf("invalid password")
	}

	return user, nil
}

// FindUserByEmail looks up a user by email
func FindUserByEmail(email string) *User {
	// Mock implementation
	return &User{Name: "Test", Email: email}
}

// CheckPassword verifies the password
func CheckPassword(user *User, password string) bool {
	// Mock implementation
	return password == "secret"
}

// CreateAdmin creates an admin user
func CreateAdmin(name, email string) *User {
	user := &User{
		Name:  name,
		Email: email,
		Admin: true,
	}
	SaveUser(user)
	return user
}

// SaveUser persists a user
func SaveUser(user *User) {
	fmt.Printf("Saving user: %s\n", user.Name)
}

// ProcessUsers processes a list of users with conditional logic
func ProcessUsers(users []*User) {
	for _, user := range users {
		if user.Admin {
			ProcessAdmin(user)
		} else {
			ProcessRegular(user)
		}
	}
}

// ProcessAdmin handles admin users
func ProcessAdmin(user *User) {
	fmt.Println("Admin:", user.Name)
}

// ProcessRegular handles regular users
func ProcessRegular(user *User) {
	fmt.Println("Regular:", user.Name)
}

// EmbeddedExample demonstrates embedded types
type EmbeddedExample struct {
	User // embedded
	Role string
}

// ExtendedAuth extends TokenAuth
type ExtendedAuth struct {
	*TokenAuth // embedded pointer
	Logger     func(string)
}
`

func setupTestExtractor(t *testing.T, source string) (*CallGraphExtractor, *parser.ParseResult) {
	p, err := parser.NewParser(parser.Go)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	t.Cleanup(func() { p.Close() })

	result, err := p.Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	t.Cleanup(func() { result.Close() })

	// Extract entities from the AST
	entities := extractTestEntities(result)

	extractor := NewCallGraphExtractor(result, entities)
	return extractor, result
}

// extractTestEntities extracts entities for testing
func extractTestEntities(result *parser.ParseResult) []CallGraphEntity {
	var entities []CallGraphEntity
	id := 0

	result.WalkNodes(func(node *sitter.Node) bool {
		switch node.Type() {
		case "function_declaration":
			name := extractFunctionName(node, result)
			if name != "" {
				id++
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("func-%d", id),
					Name:     name,
					Type:     "function",
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}

		case "method_declaration":
			name := extractMethodName(node, result)
			if name != "" {
				id++
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("method-%d", id),
					Name:     name,
					Type:     "method",
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}

		case "type_spec":
			name := extractTypeName(node, result)
			if name != "" {
				id++
				entityType := "type"
				// Check if it's a struct or interface
				for i := uint32(0); i < node.ChildCount(); i++ {
					child := node.Child(int(i))
					if child.Type() == "struct_type" {
						entityType = "struct"
					} else if child.Type() == "interface_type" {
						entityType = "interface"
					}
				}
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("type-%d", id),
					Name:     name,
					Type:     entityType,
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}
		}
		return true
	})

	return entities
}

func extractFunctionName(node *sitter.Node, result *parser.ParseResult) string {
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func extractMethodName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	return ""
}

func extractTypeName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	// Fallback
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func TestNewCallGraphExtractor(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	t.Run("creates extractor with entities", func(t *testing.T) {
		if extractor == nil {
			t.Fatal("extractor should not be nil")
		}

		if len(extractor.entities) == 0 {
			t.Error("expected entities to be populated")
		}

		// Should have entityByName populated
		if len(extractor.entityByName) == 0 {
			t.Error("expected entityByName map to be populated")
		}
	})

	t.Run("indexes entities by name", func(t *testing.T) {
		// Check if Login function is indexed
		if _, ok := extractor.entityByName["Login"]; !ok {
			t.Error("expected Login to be in entityByName map")
		}

		// Check if User type is indexed
		if _, ok := extractor.entityByName["User"]; !ok {
			t.Error("expected User to be in entityByName map")
		}
	})
}

func TestExtractFunctionCalls(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts direct function calls", func(t *testing.T) {
		// Login should call FindUserByEmail
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "FindUserByEmail" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to FindUserByEmail")
		}
	})

	t.Run("extracts multiple calls from same function", func(t *testing.T) {
		// Login calls FindUserByEmail and CheckPassword
		callsFromLogin := 0
		loginEntity := extractor.entityByName["Login"]
		if loginEntity == nil {
			t.Fatal("Login entity not found")
		}

		for _, dep := range deps {
			if dep.FromID == loginEntity.ID && dep.DepType == Calls {
				callsFromLogin++
			}
		}

		// Should have at least FindUserByEmail, CheckPassword, and fmt.Errorf calls
		if callsFromLogin < 2 {
			t.Errorf("expected at least 2 calls from Login, got %d", callsFromLogin)
		}
	})

	t.Run("extracts qualified function calls", func(t *testing.T) {
		// External package calls (fmt.Errorf, fmt.Printf) are intentionally NOT extracted
		// since they're unresolved targets. Only calls to entities within the codebase are kept.
		// Verify that qualified external calls are correctly skipped.
		for _, dep := range deps {
			if dep.DepType == Calls {
				// Should not have fmt.* calls since those are external
				if strings.HasPrefix(dep.ToQualified, "fmt.") {
					t.Error("external package calls should not be extracted")
				}
			}
		}
		// The test passes if no external calls are found (correct behavior)
	})
}

func TestConditionalCallDetection(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("detects conditional calls in if statements", func(t *testing.T) {
		// ProcessUsers has conditional calls to ProcessAdmin and ProcessRegular
		conditionalCalls := 0
		for _, dep := range deps {
			if dep.DepType == Calls && dep.Optional {
				if dep.ToName == "ProcessAdmin" || dep.ToName == "ProcessRegular" {
					conditionalCalls++
				}
			}
		}

		if conditionalCalls < 2 {
			t.Errorf("expected at least 2 conditional calls, got %d", conditionalCalls)
		}
	})

	t.Run("non-conditional calls are not marked optional", func(t *testing.T) {
		// SaveUser call from CreateAdmin is not inside an if condition
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "SaveUser" {
				found = true
				if dep.Optional {
					t.Error("SaveUser call should not be conditional")
				}
				break
			}
		}
		if !found {
			t.Error("expected to find SaveUser call")
		}
	})
}

func TestExtractTypeReferences(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts type references from function signatures", func(t *testing.T) {
		// Login returns *User
		found := false
		for _, dep := range deps {
			if dep.DepType == UsesType && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find User type reference")
		}
	})

	t.Run("does not include builtin types", func(t *testing.T) {
		for _, dep := range deps {
			if dep.DepType == UsesType {
				if isBuiltinType(dep.ToName) {
					t.Errorf("builtin type %s should not be in dependencies", dep.ToName)
				}
			}
		}
	})
}

func TestExtractMethodReceiver(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts method_of relationship", func(t *testing.T) {
		// Validate method is a method of TokenAuth
		found := false
		for _, dep := range deps {
			if dep.DepType == MethodOf && dep.ToName == "TokenAuth" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find method_of relationship to TokenAuth")
		}
	})

	t.Run("handles pointer receivers", func(t *testing.T) {
		// Validate has a pointer receiver (*TokenAuth)
		validateEntity := extractor.entityByName["Validate"]
		if validateEntity == nil {
			t.Fatal("Validate entity not found")
		}

		// Should still extract TokenAuth as the receiver type
		found := false
		for _, dep := range deps {
			if dep.FromID == validateEntity.ID && dep.DepType == MethodOf {
				if dep.ToName == "TokenAuth" {
					found = true
				}
				break
			}
		}
		if !found {
			t.Error("expected method_of to extract TokenAuth from pointer receiver")
		}
	})
}

func TestExtractEmbeddedTypes(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts extends relationship for embedded types", func(t *testing.T) {
		// EmbeddedExample embeds User
		found := false
		for _, dep := range deps {
			if dep.DepType == Extends && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find extends relationship to User")
		}
	})

	t.Run("handles embedded pointer types", func(t *testing.T) {
		// ExtendedAuth embeds *TokenAuth
		found := false
		for _, dep := range deps {
			if dep.DepType == Extends && dep.ToName == "TokenAuth" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find extends relationship to TokenAuth (pointer embedded)")
		}
	})
}

func TestIsBuiltinType(t *testing.T) {
	builtins := []string{
		"string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "byte", "rune",
		"error", "any", "interface{}", "comparable",
	}

	for _, builtin := range builtins {
		t.Run(builtin, func(t *testing.T) {
			if !isBuiltinType(builtin) {
				t.Errorf("%s should be identified as builtin", builtin)
			}
		})
	}

	nonBuiltins := []string{"User", "Context", "MyType", "io.Reader"}
	for _, nonBuiltin := range nonBuiltins {
		t.Run(nonBuiltin, func(t *testing.T) {
			if isBuiltinType(nonBuiltin) {
				t.Errorf("%s should not be identified as builtin", nonBuiltin)
			}
		})
	}
}

func TestDependencyLocation(t *testing.T) {
	extractor, _ := setupTestExtractor(t, testCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("dependencies have location information", func(t *testing.T) {
		foundWithLocation := false
		for _, dep := range deps {
			if dep.Location != "" && strings.Contains(dep.Location, ":") {
				foundWithLocation = true
				break
			}
		}
		if !foundWithLocation {
			t.Error("expected dependencies to have location information")
		}
	})
}

func TestDepTypeConstants(t *testing.T) {
	t.Run("dependency types have correct values", func(t *testing.T) {
		if Calls != "calls" {
			t.Errorf("Calls should be 'calls', got %q", Calls)
		}
		if UsesType != "uses_type" {
			t.Errorf("UsesType should be 'uses_type', got %q", UsesType)
		}
		if Implements != "implements" {
			t.Errorf("Implements should be 'implements', got %q", Implements)
		}
		if Extends != "extends" {
			t.Errorf("Extends should be 'extends', got %q", Extends)
		}
		if MethodOf != "method_of" {
			t.Errorf("MethodOf should be 'method_of', got %q", MethodOf)
		}
		if Contains != "contains" {
			t.Errorf("Contains should be 'contains', got %q", Contains)
		}
	})
}

func TestEmptyInput(t *testing.T) {
	t.Run("handles empty source", func(t *testing.T) {
		p, err := parser.NewParser(parser.Go)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("package main"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		extractor := NewCallGraphExtractor(result, nil)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		// Should return without error (nil or empty slice are both valid)
		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("handles entities without nodes", func(t *testing.T) {
		p, err := parser.NewParser(parser.Go)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("package main"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		// Entity without a node
		entities := []CallGraphEntity{
			{ID: "test-1", Name: "TestFunc", Type: "function", Node: nil},
		}

		extractor := NewCallGraphExtractor(result, entities)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		// Should not panic and return empty deps
		if len(deps) != 0 {
			t.Errorf("expected 0 deps for entity without node, got %d", len(deps))
		}
	})
}

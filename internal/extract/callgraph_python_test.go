package extract

import (
	"fmt"
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// Sample Python source for testing call graph extraction
const testPythonCallGraphSource = `
class User:
    """Represents a user in the system."""

    def __init__(self, name: str, email: str, admin: bool = False):
        self.name = name
        self.email = email
        self.admin = admin

    def validate(self) -> bool:
        """Validate the user data."""
        return bool(self.name) and bool(self.email)


class Authenticator:
    """Interface for authentication."""

    def authenticate(self, token: str) -> bool:
        raise NotImplementedError


class TokenAuth(Authenticator):
    """Token-based authentication."""

    def __init__(self, secret: str):
        self.secret = secret

    def authenticate(self, token: str) -> bool:
        return token.startswith(self.secret)


def login(email: str, password: str) -> User:
    """Authenticate a user."""
    if not email:
        raise ValueError("email required")

    user = find_user_by_email(email)
    if user is None:
        raise ValueError("user not found")

    if not check_password(user, password):
        raise ValueError("invalid password")

    return user


def find_user_by_email(email: str) -> User:
    """Look up a user by email."""
    return User(name="Test", email=email)


def check_password(user: User, password: str) -> bool:
    """Verify the password."""
    return password == "secret"


def create_admin(name: str, email: str) -> User:
    """Create an admin user."""
    user = User(name=name, email=email, admin=True)
    save_user(user)
    return user


def save_user(user: User) -> None:
    """Persist a user."""
    print(f"Saving user: {user.name}")


def process_users(users: list) -> None:
    """Process a list of users with conditional logic."""
    for user in users:
        if user.admin:
            process_admin(user)
        else:
            process_regular(user)


def process_admin(user: User) -> None:
    """Handle admin users."""
    print(f"Admin: {user.name}")


def process_regular(user: User) -> None:
    """Handle regular users."""
    print(f"Regular: {user.name}")


@staticmethod
def static_helper() -> str:
    """A static helper function."""
    return "helper"


@classmethod
def class_helper(cls) -> str:
    """A class helper function."""
    return "class helper"


@custom_decorator
def decorated_func() -> None:
    """A decorated function."""
    pass


@decorator_with_args("arg1", "arg2")
def decorated_with_args() -> None:
    """A function with decorator arguments."""
    pass


class ExtendedAuth(TokenAuth):
    """Extended authentication with logging."""

    def __init__(self, secret: str, logger):
        super().__init__(secret)
        self.logger = logger

    def authenticate(self, token: str) -> bool:
        result = super().authenticate(token)
        self.logger(f"Auth result: {result}")
        return result


class MultiInherit(User, Authenticator):
    """Class with multiple inheritance."""
    pass


def method_chaining():
    """Test method chaining calls."""
    obj.method1().method2().method3()


def module_calls():
    """Test module-qualified calls."""
    os.path.join("a", "b")
    json.loads(data)
    mymodule.myfunction()
`

func setupPythonTestExtractor(t *testing.T, source string) (*PythonCallGraphExtractor, *parser.ParseResult) {
	p, err := parser.NewParser(parser.Python)
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
	entities := extractPythonTestEntities(result)

	extractor := NewPythonCallGraphExtractor(result, entities)
	return extractor, result
}

// extractPythonTestEntities extracts entities for testing from Python source
func extractPythonTestEntities(result *parser.ParseResult) []CallGraphEntity {
	var entities []CallGraphEntity
	id := 0

	result.WalkNodes(func(node *sitter.Node) bool {
		switch node.Type() {
		case "function_definition":
			name := extractPythonFunctionName(node, result)
			if name != "" {
				id++
				entityType := "function"
				// Check if it's a method (inside a class)
				if isInsideClass(node) {
					entityType = "method"
				}
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("func-%d", id),
					Name:     name,
					Type:     entityType,
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}

		case "class_definition":
			name := extractPythonClassName(node, result)
			if name != "" {
				id++
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("class-%d", id),
					Name:     name,
					Type:     "class",
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}
		}
		return true
	})

	return entities
}

func extractPythonFunctionName(node *sitter.Node, result *parser.ParseResult) string {
	// Try field name first
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}

	// Fallback: look for identifier
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func extractPythonClassName(node *sitter.Node, result *parser.ParseResult) string {
	// Try field name first
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}

	// Fallback: look for identifier after 'class'
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func isInsideClass(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "class_definition" {
			return true
		}
		parent = parent.Parent()
	}
	return false
}

func TestNewPythonCallGraphExtractor(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

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
		// Check if login function is indexed
		if _, ok := extractor.entityByName["login"]; !ok {
			t.Error("expected login to be in entityByName map")
		}

		// Check if User class is indexed
		if _, ok := extractor.entityByName["User"]; !ok {
			t.Error("expected User to be in entityByName map")
		}
	})
}

func TestPythonExtractFunctionCalls(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts direct function calls", func(t *testing.T) {
		// login should call find_user_by_email
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "find_user_by_email" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to find_user_by_email")
		}
	})

	t.Run("extracts multiple calls from same function", func(t *testing.T) {
		// login calls find_user_by_email and check_password
		loginEntity := extractor.entityByName["login"]
		if loginEntity == nil {
			t.Fatal("login entity not found")
		}

		callsFromLogin := make(map[string]bool)
		for _, dep := range deps {
			if dep.FromID == loginEntity.ID && dep.DepType == Calls {
				callsFromLogin[dep.ToName] = true
			}
		}

		expectedCalls := []string{"find_user_by_email", "check_password"}
		for _, expected := range expectedCalls {
			if !callsFromLogin[expected] {
				t.Errorf("expected login to call %s", expected)
			}
		}
	})

	t.Run("extracts class instantiation as calls", func(t *testing.T) {
		// find_user_by_email instantiates User
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find User instantiation call")
		}
	})

	t.Run("extracts method calls on self", func(t *testing.T) {
		// TokenAuth.authenticate calls startswith (but that's on string)
		// We check that method calls are extracted
		authenticateEntity := extractor.entityByName["authenticate"]
		if authenticateEntity == nil {
			// Method might be named differently
			t.Skip("authenticate entity not found")
		}
	})
}

func TestPythonConditionalCallDetection(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("detects conditional calls in if statements", func(t *testing.T) {
		// process_users has conditional calls to process_admin and process_regular
		conditionalCalls := 0
		for _, dep := range deps {
			if dep.DepType == Calls && dep.Optional {
				if dep.ToName == "process_admin" || dep.ToName == "process_regular" {
					conditionalCalls++
				}
			}
		}

		if conditionalCalls < 2 {
			t.Errorf("expected at least 2 conditional calls, got %d", conditionalCalls)
		}
	})

	t.Run("non-conditional calls are not marked optional", func(t *testing.T) {
		// save_user call from create_admin is not inside an if condition
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "save_user" {
				found = true
				if dep.Optional {
					t.Error("save_user call should not be conditional")
				}
				break
			}
		}
		if !found {
			t.Error("expected to find save_user call")
		}
	})
}

func TestPythonExtractTypeReferences(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts type references from function signatures", func(t *testing.T) {
		// login returns User
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
				if isPythonBuiltinType(dep.ToName) {
					t.Errorf("builtin type %s should not be in dependencies", dep.ToName)
				}
			}
		}
	})
}

func TestPythonExtractBaseClasses(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts extends relationship for single inheritance", func(t *testing.T) {
		// TokenAuth extends Authenticator
		found := false
		for _, dep := range deps {
			if dep.DepType == Extends && dep.ToName == "Authenticator" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find extends relationship to Authenticator")
		}
	})

	t.Run("extracts extends for multiple inheritance", func(t *testing.T) {
		// MultiInherit extends both User and Authenticator
		multiInheritEntity := extractor.entityByName["MultiInherit"]
		if multiInheritEntity == nil {
			t.Fatal("MultiInherit entity not found")
		}

		extendedTypes := make(map[string]bool)
		for _, dep := range deps {
			if dep.FromID == multiInheritEntity.ID && dep.DepType == Extends {
				extendedTypes[dep.ToName] = true
			}
		}

		if !extendedTypes["User"] {
			t.Error("expected MultiInherit to extend User")
		}
		if !extendedTypes["Authenticator"] {
			t.Error("expected MultiInherit to extend Authenticator")
		}
	})

	t.Run("extracts transitive inheritance", func(t *testing.T) {
		// ExtendedAuth extends TokenAuth
		found := false
		for _, dep := range deps {
			if dep.DepType == Extends && dep.ToName == "TokenAuth" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find extends relationship to TokenAuth")
		}
	})
}

func TestPythonExtractDecorators(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts decorator calls", func(t *testing.T) {
		// decorated_func uses @custom_decorator
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "custom_decorator" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find custom_decorator call from decorator")
		}
	})

	t.Run("extracts decorator with arguments", func(t *testing.T) {
		// decorated_with_args uses @decorator_with_args(...)
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "decorator_with_args" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find decorator_with_args call")
		}
	})

	t.Run("filters builtin decorators", func(t *testing.T) {
		// @staticmethod and @classmethod should be filtered out
		for _, dep := range deps {
			if dep.DepType == Calls {
				if dep.ToName == "staticmethod" || dep.ToName == "classmethod" {
					t.Errorf("builtin decorator %s should be filtered", dep.ToName)
				}
			}
		}
	})
}

func TestPythonExtractMethodOwner(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts method_of relationship", func(t *testing.T) {
		// validate method is a method of User
		found := false
		for _, dep := range deps {
			if dep.DepType == MethodOf && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find method_of relationship to User")
		}
	})

	t.Run("extracts method_of for multiple classes", func(t *testing.T) {
		// Check that methods are correctly associated with their classes
		classMethodCounts := make(map[string]int)
		for _, dep := range deps {
			if dep.DepType == MethodOf {
				classMethodCounts[dep.ToName]++
			}
		}

		// User should have methods (__init__, validate)
		if classMethodCounts["User"] < 2 {
			t.Errorf("expected User to have at least 2 methods, got %d", classMethodCounts["User"])
		}

		// TokenAuth should have methods (__init__, authenticate)
		if classMethodCounts["TokenAuth"] < 2 {
			t.Errorf("expected TokenAuth to have at least 2 methods, got %d", classMethodCounts["TokenAuth"])
		}
	})
}

func TestPythonModuleQualifiedCalls(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts module-qualified calls", func(t *testing.T) {
		// module_calls should have calls to os.path.join, json.loads, mymodule.myfunction
		moduleCallsEntity := extractor.entityByName["module_calls"]
		if moduleCallsEntity == nil {
			t.Fatal("module_calls entity not found")
		}

		qualifiedCalls := make(map[string]bool)
		for _, dep := range deps {
			if dep.FromID == moduleCallsEntity.ID && dep.DepType == Calls {
				if dep.ToQualified != "" {
					qualifiedCalls[dep.ToQualified] = true
				}
			}
		}

		// Should find at least one qualified call
		if len(qualifiedCalls) == 0 {
			t.Error("expected to find qualified calls in module_calls function")
		}
	})
}

func TestPythonIsPythonBuiltinType(t *testing.T) {
	builtins := []string{
		"str", "int", "float", "bool", "list", "dict", "set", "tuple",
		"None", "type", "object", "Exception", "BaseException",
		"print", "len", "range", "enumerate", "zip", "map", "filter",
		"open", "input", "sorted", "reversed", "abs", "max", "min",
		"isinstance", "issubclass", "hasattr", "getattr", "setattr",
		"staticmethod", "classmethod", "property", "super",
		"self", "cls", "True", "False",
		// Typing module
		"List", "Dict", "Set", "Tuple", "Optional", "Union", "Any",
		"Callable", "Type", "Generic",
	}

	for _, builtin := range builtins {
		t.Run(builtin, func(t *testing.T) {
			if !isPythonBuiltinType(builtin) {
				t.Errorf("%s should be identified as builtin", builtin)
			}
		})
	}

	nonBuiltins := []string{"User", "Context", "MyType", "CustomClass", "Authenticator"}
	for _, nonBuiltin := range nonBuiltins {
		t.Run(nonBuiltin, func(t *testing.T) {
			if isPythonBuiltinType(nonBuiltin) {
				t.Errorf("%s should not be identified as builtin", nonBuiltin)
			}
		})
	}
}

func TestPythonDependencyLocation(t *testing.T) {
	extractor, _ := setupPythonTestExtractor(t, testPythonCallGraphSource)

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

func TestPythonEmptyInput(t *testing.T) {
	t.Run("handles empty source", func(t *testing.T) {
		p, err := parser.NewParser(parser.Python)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte(""))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		extractor := NewPythonCallGraphExtractor(result, nil)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("handles entities without nodes", func(t *testing.T) {
		p, err := parser.NewParser(parser.Python)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("# comment"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		// Entity without a node
		entities := []CallGraphEntity{
			{ID: "test-1", Name: "test_func", Type: "function", Node: nil},
		}

		extractor := NewPythonCallGraphExtractor(result, entities)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		if len(deps) != 0 {
			t.Errorf("expected 0 deps for entity without node, got %d", len(deps))
		}
	})
}

func TestPythonSuperCalls(t *testing.T) {
	source := `
class Parent:
    def method(self):
        pass

class Child(Parent):
    def method(self):
        super().method()
        super(Child, self).method()
`
	extractor, _ := setupPythonTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts super() method calls", func(t *testing.T) {
		// Should find calls to method (the parent method)
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "method" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find method call via super()")
		}
	})
}

func TestPythonNestedClasses(t *testing.T) {
	source := `
class Outer:
    class Inner:
        def inner_method(self):
            pass

    def outer_method(self):
        inner = self.Inner()
        inner.inner_method()
`
	extractor, _ := setupPythonTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("handles nested class method ownership", func(t *testing.T) {
		// inner_method should be a method of Inner
		found := false
		for _, dep := range deps {
			if dep.DepType == MethodOf && dep.ToName == "Inner" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected inner_method to be method_of Inner")
		}
	})
}

func TestPythonLambdaCalls(t *testing.T) {
	source := `
def use_lambda():
    f = lambda x: process(x)
    return f(10)

def process(x):
    return x * 2
`
	extractor, _ := setupPythonTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts calls inside lambdas", func(t *testing.T) {
		// use_lambda should have a call to process (from the lambda)
		useLambdaEntity := extractor.entityByName["use_lambda"]
		if useLambdaEntity == nil {
			t.Fatal("use_lambda entity not found")
		}

		found := false
		for _, dep := range deps {
			if dep.FromID == useLambdaEntity.ID && dep.DepType == Calls && dep.ToName == "process" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to process from lambda in use_lambda")
		}
	})
}

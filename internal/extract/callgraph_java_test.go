package extract

import (
	"fmt"
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// Sample Java source for testing call graph extraction
const testJavaCallGraphSource = `package com.example;

import java.util.List;
import java.util.ArrayList;

// User represents a user in the system
public class User {
    private String name;
    private String email;
    private boolean admin;

    public User(String name, String email) {
        this.name = name;
        this.email = email;
        this.admin = false;
    }

    public String getName() {
        return name;
    }

    public String getEmail() {
        return email;
    }

    public boolean isAdmin() {
        return admin;
    }

    public void setAdmin(boolean admin) {
        this.admin = admin;
    }
}

// Authenticator interface for authentication
interface Authenticator {
    boolean validate(String token);
}

// TokenAuth implements Authenticator
class TokenAuth implements Authenticator {
    private String secret;

    public TokenAuth(String secret) {
        this.secret = secret;
    }

    @Override
    public boolean validate(String token) {
        return token.startsWith(secret);
    }
}

// AuthService handles authentication
class AuthService {
    private Authenticator authenticator;
    private UserRepository userRepository;

    public AuthService(Authenticator auth, UserRepository repo) {
        this.authenticator = auth;
        this.userRepository = repo;
    }

    public User login(String email, String password) {
        if (email == null || email.isEmpty()) {
            throw new IllegalArgumentException("Email required");
        }

        User user = userRepository.findByEmail(email);
        if (user == null) {
            return null;
        }

        if (!checkPassword(user, password)) {
            return null;
        }

        return user;
    }

    private boolean checkPassword(User user, String password) {
        // Mock implementation
        return "secret".equals(password);
    }

    public User createAdmin(String name, String email) {
        User user = new User(name, email);
        user.setAdmin(true);
        userRepository.save(user);
        return user;
    }

    public void processUsers(List<User> users) {
        for (User user : users) {
            if (user.isAdmin()) {
                processAdmin(user);
            } else {
                processRegular(user);
            }
        }
    }

    private void processAdmin(User user) {
        System.out.println("Admin: " + user.getName());
    }

    private void processRegular(User user) {
        System.out.println("Regular: " + user.getName());
    }
}

// UserRepository interface
interface UserRepository {
    User findByEmail(String email);
    void save(User user);
}

// ExtendedAuth extends TokenAuth
class ExtendedAuth extends TokenAuth {
    private Logger logger;

    public ExtendedAuth(String secret, Logger logger) {
        super(secret);
        this.logger = logger;
    }

    @Override
    public boolean validate(String token) {
        logger.log("Validating token");
        return super.validate(token);
    }
}

// Logger interface
interface Logger {
    void log(String message);
}

// Builder pattern example with chained calls
class UserBuilder {
    private String name;
    private String email;

    public UserBuilder setName(String name) {
        this.name = name;
        return this;
    }

    public UserBuilder setEmail(String email) {
        this.email = email;
        return this;
    }

    public User build() {
        return new User(name, email);
    }
}

// Class implementing multiple interfaces
interface Serializable {
    byte[] serialize();
}

interface Cloneable {
    Object clone();
}

class DataObject implements Serializable, Cloneable {
    public byte[] serialize() {
        return new byte[0];
    }

    public Object clone() {
        return new DataObject();
    }
}

// Interface extending another interface
interface AdvancedAuthenticator extends Authenticator {
    void logout(String token);
}

// Generic class usage
class UserService {
    public List<User> getAllUsers() {
        ArrayList<User> users = new ArrayList<User>();
        return users;
    }

    public void processException() throws CustomException {
        throw new CustomException("Error");
    }
}

// Custom exception
class CustomException extends Exception {
    public CustomException(String message) {
        super(message);
    }
}
`

func setupJavaTestExtractor(t *testing.T, source string) (*JavaCallGraphExtractor, *parser.ParseResult) {
	p, err := parser.NewParser(parser.Java)
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
	entities := extractJavaTestEntities(result)

	extractor := NewJavaCallGraphExtractor(result, entities)
	return extractor, result
}

// extractJavaTestEntities extracts entities for testing
func extractJavaTestEntities(result *parser.ParseResult) []CallGraphEntity {
	var entities []CallGraphEntity
	id := 0

	result.WalkNodes(func(node *sitter.Node) bool {
		switch node.Type() {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			name := extractJavaClassName(node, result)
			if name != "" {
				id++
				entityType := "class"
				if node.Type() == "interface_declaration" {
					entityType = "interface"
				} else if node.Type() == "enum_declaration" {
					entityType = "enum"
				} else if node.Type() == "record_declaration" {
					entityType = "record"
				}
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("%s-%d", entityType, id),
					Name:     name,
					Type:     entityType,
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}

		case "method_declaration":
			name := extractJavaMethodName(node, result)
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

		case "constructor_declaration":
			name := extractJavaConstructorName(node, result)
			if name != "" {
				id++
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("constructor-%d", id),
					Name:     name,
					Type:     "constructor",
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}
		}
		return true
	})

	return entities
}

func extractJavaClassName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	// Fallback
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func extractJavaMethodName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	return ""
}

func extractJavaConstructorName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	return ""
}

func TestNewJavaCallGraphExtractor(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

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
		// Check if User class is indexed
		if _, ok := extractor.entityByName["User"]; !ok {
			t.Error("expected User to be in entityByName map")
		}

		// Check if AuthService is indexed
		if _, ok := extractor.entityByName["AuthService"]; !ok {
			t.Error("expected AuthService to be in entityByName map")
		}

		// Check if login method is indexed
		if _, ok := extractor.entityByName["login"]; !ok {
			t.Error("expected login to be in entityByName map")
		}
	})
}

func TestJavaExtractMethodCalls(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts direct method calls", func(t *testing.T) {
		// login should call findByEmail
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "findByEmail" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to findByEmail")
		}
	})

	t.Run("extracts this method calls", func(t *testing.T) {
		// login calls checkPassword
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "checkPassword" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to checkPassword")
		}
	})

	t.Run("extracts instance method calls", func(t *testing.T) {
		// createAdmin calls user.setAdmin
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "setAdmin" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to setAdmin")
		}
	})

	t.Run("extracts super method calls", func(t *testing.T) {
		// ExtendedAuth.validate calls super.validate
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "validate" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to validate (super call)")
		}
	})
}

func TestJavaExtractConstructorCalls(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts new expressions", func(t *testing.T) {
		// createAdmin creates new User
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find constructor call to User")
		}
	})

	t.Run("extracts generic type constructor calls", func(t *testing.T) {
		// getAllUsers creates new ArrayList<User>
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "ArrayList" {
				found = true
				break
			}
		}
		// ArrayList is a builtin, so it should be filtered
		if found {
			t.Error("ArrayList should be filtered as builtin type")
		}
	})
}

func TestJavaConditionalCallDetection(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("detects conditional calls in if statements", func(t *testing.T) {
		// processUsers has conditional calls to processAdmin and processRegular
		conditionalCalls := 0
		for _, dep := range deps {
			if dep.DepType == Calls && dep.Optional {
				if dep.ToName == "processAdmin" || dep.ToName == "processRegular" {
					conditionalCalls++
				}
			}
		}

		if conditionalCalls < 2 {
			t.Errorf("expected at least 2 conditional calls, got %d", conditionalCalls)
		}
	})

	t.Run("non-conditional calls are not marked optional", func(t *testing.T) {
		// save call from createAdmin is not inside an if condition
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "save" {
				found = true
				if dep.Optional {
					t.Error("save call should not be conditional")
				}
				break
			}
		}
		if !found {
			t.Error("expected to find save call")
		}
	})
}

func TestJavaExtractTypeReferences(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts type references from method parameters", func(t *testing.T) {
		// AuthService constructor takes Authenticator and UserRepository
		foundAuthenticator := false
		foundUserRepository := false
		for _, dep := range deps {
			if dep.DepType == UsesType {
				if dep.ToName == "Authenticator" {
					foundAuthenticator = true
				}
				if dep.ToName == "UserRepository" {
					foundUserRepository = true
				}
			}
		}
		if !foundAuthenticator {
			t.Error("expected to find Authenticator type reference")
		}
		if !foundUserRepository {
			t.Error("expected to find UserRepository type reference")
		}
	})

	t.Run("extracts type references from return types", func(t *testing.T) {
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
				if dep.ToName == "String" || dep.ToName == "boolean" || dep.ToName == "int" {
					t.Errorf("builtin type %s should not be in dependencies", dep.ToName)
				}
			}
		}
	})
}

func TestJavaExtractClassRelations(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts extends relationship", func(t *testing.T) {
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

	t.Run("extracts implements relationship", func(t *testing.T) {
		// TokenAuth implements Authenticator
		found := false
		for _, dep := range deps {
			if dep.DepType == Implements && dep.ToName == "Authenticator" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find implements relationship to Authenticator")
		}
	})

	t.Run("extracts multiple implements", func(t *testing.T) {
		// DataObject implements Serializable, Cloneable
		foundSerializable := false
		foundCloneable := false
		for _, dep := range deps {
			if dep.DepType == Implements {
				if dep.ToName == "Serializable" {
					foundSerializable = true
				}
				if dep.ToName == "Cloneable" {
					foundCloneable = true
				}
			}
		}
		if !foundSerializable {
			t.Error("expected to find implements relationship to Serializable")
		}
		if !foundCloneable {
			t.Error("expected to find implements relationship to Cloneable")
		}
	})

	t.Run("extracts interface extends interface", func(t *testing.T) {
		// AdvancedAuthenticator extends Authenticator
		found := false
		for _, dep := range deps {
			if dep.DepType == Extends && dep.ToName == "Authenticator" {
				// Check that it's from AdvancedAuthenticator
				fromEntity := extractor.entityByID[dep.FromID]
				if fromEntity != nil && fromEntity.Name == "AdvancedAuthenticator" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("expected to find interface extends relationship to Authenticator")
		}
	})
}

func TestJavaExtractMethodOwner(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts method_of relationship", func(t *testing.T) {
		// login method is a method of AuthService
		found := false
		for _, dep := range deps {
			if dep.DepType == MethodOf && dep.ToName == "AuthService" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find method_of relationship to AuthService")
		}
	})

	t.Run("extracts method_of for constructor", func(t *testing.T) {
		// User constructor is a method of User
		found := false
		for _, dep := range deps {
			if dep.DepType == MethodOf && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find method_of relationship to User (for constructor)")
		}
	})
}

func TestJavaIsBuiltinType(t *testing.T) {
	extractor := &JavaCallGraphExtractor{}

	builtins := []string{
		"String", "Integer", "Long", "Double", "Float", "Boolean",
		"Object", "Class", "Exception", "RuntimeException", "Throwable",
		"List", "Map", "Set", "Collection", "Iterator", "Iterable",
		"ArrayList", "HashMap", "HashSet",
		"System", "Math", "Arrays", "Collections",
		"int", "long", "double", "float", "boolean", "byte", "short", "char", "void",
	}

	for _, builtin := range builtins {
		t.Run(builtin, func(t *testing.T) {
			if !extractor.isJavaBuiltinType(builtin) {
				t.Errorf("%s should be identified as builtin", builtin)
			}
		})
	}

	nonBuiltins := []string{"User", "AuthService", "TokenAuth", "MyCustomClass"}
	for _, nonBuiltin := range nonBuiltins {
		t.Run(nonBuiltin, func(t *testing.T) {
			if extractor.isJavaBuiltinType(nonBuiltin) {
				t.Errorf("%s should not be identified as builtin", nonBuiltin)
			}
		})
	}
}

func TestJavaDependencyLocation(t *testing.T) {
	extractor, _ := setupJavaTestExtractor(t, testJavaCallGraphSource)

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

func TestJavaEmptyInput(t *testing.T) {
	t.Run("handles empty source", func(t *testing.T) {
		p, err := parser.NewParser(parser.Java)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("package main;"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		extractor := NewJavaCallGraphExtractor(result, nil)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("handles entities without nodes", func(t *testing.T) {
		p, err := parser.NewParser(parser.Java)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("package main;"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		// Entity without a node
		entities := []CallGraphEntity{
			{ID: "test-1", Name: "TestMethod", Type: "method", Node: nil},
		}

		extractor := NewJavaCallGraphExtractor(result, entities)
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

func TestJavaChainedMethodCalls(t *testing.T) {
	source := `
class Builder {
    Builder setA() { return this; }
    Builder setB() { return this; }
    Object build() { return null; }
}

class Client {
    void test() {
        new Builder().setA().setB().build();
    }
}
`

	extractor, _ := setupJavaTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts chained method calls", func(t *testing.T) {
		// Should find calls to setA, setB, and build
		foundSetA := false
		foundSetB := false
		foundBuild := false

		for _, dep := range deps {
			if dep.DepType == Calls {
				switch dep.ToName {
				case "setA":
					foundSetA = true
				case "setB":
					foundSetB = true
				case "build":
					foundBuild = true
				}
			}
		}

		if !foundSetA {
			t.Error("expected to find call to setA")
		}
		if !foundSetB {
			t.Error("expected to find call to setB")
		}
		if !foundBuild {
			t.Error("expected to find call to build")
		}
	})
}

func TestJavaStaticMethodCalls(t *testing.T) {
	source := `
class Helper {
    static void staticMethod() {}
    static int calculate(int x) { return x * 2; }
}

class Client {
    void test() {
        Helper.staticMethod();
        int result = Helper.calculate(5);
    }
}
`

	extractor, _ := setupJavaTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts static method calls with qualified name", func(t *testing.T) {
		foundStatic := false
		foundCalculate := false

		for _, dep := range deps {
			if dep.DepType == Calls {
				if dep.ToName == "staticMethod" && strings.Contains(dep.ToQualified, "Helper") {
					foundStatic = true
				}
				if dep.ToName == "calculate" && strings.Contains(dep.ToQualified, "Helper") {
					foundCalculate = true
				}
			}
		}

		if !foundStatic {
			t.Error("expected to find static call to Helper.staticMethod")
		}
		if !foundCalculate {
			t.Error("expected to find static call to Helper.calculate")
		}
	})
}

func TestJavaExceptionTypes(t *testing.T) {
	source := `
class CustomException extends RuntimeException {
    CustomException(String msg) { super(msg); }
}

class Service {
    void riskyMethod() throws CustomException {
        throw new CustomException("Error");
    }
}
`

	extractor, _ := setupJavaTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts exception types from throws clause", func(t *testing.T) {
		found := false
		for _, dep := range deps {
			if dep.DepType == UsesType && dep.ToName == "CustomException" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find CustomException in type references")
		}
	})

	t.Run("extracts new exception creation", func(t *testing.T) {
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "CustomException" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find constructor call to CustomException")
		}
	})
}

func TestJavaGenericTypes(t *testing.T) {
	source := `
class Container<T> {
    T value;
    T getValue() { return value; }
}

class Usage {
    Container<CustomType> container;

    CustomType get() {
        return container.getValue();
    }
}

class CustomType {}
`

	extractor, _ := setupJavaTestExtractor(t, source)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts generic type references", func(t *testing.T) {
		foundContainer := false
		foundCustomType := false

		for _, dep := range deps {
			if dep.DepType == UsesType {
				if dep.ToName == "Container" {
					foundContainer = true
				}
				if dep.ToName == "CustomType" {
					foundCustomType = true
				}
			}
		}

		if !foundContainer {
			t.Error("expected to find Container type reference")
		}
		if !foundCustomType {
			t.Error("expected to find CustomType type reference")
		}
	})
}

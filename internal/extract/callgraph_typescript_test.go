package extract

import (
	"fmt"
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// Sample TypeScript source for testing call graph extraction
const testTypeScriptCallGraphSource = `
// User interface
interface IUser {
	name: string;
	email: string;
}

// Base authenticator interface
interface IAuthenticator {
	validate(token: string): boolean;
}

// Extended interface
interface IAdminAuth extends IAuthenticator {
	adminLevel: number;
}

// User class
class User implements IUser {
	name: string;
	email: string;
	admin: boolean;

	constructor(name: string, email: string) {
		this.name = name;
		this.email = email;
		this.admin = false;
	}

	greet(): string {
		return "Hello, " + this.name;
	}
}

// Extended user class
class AdminUser extends User implements IAdminAuth {
	adminLevel: number;

	constructor(name: string, email: string, level: number) {
		super(name, email);
		this.admin = true;
		this.adminLevel = level;
	}

	validate(token: string): boolean {
		return token.startsWith("admin:");
	}

	processAdmin(): void {
		console.log("Processing admin: " + this.name);
	}
}

// Authentication function
function login(email: string, password: string): User | null {
	if (email === "") {
		return null;
	}

	const user = findUserByEmail(email);
	if (!user) {
		return null;
	}

	if (!checkPassword(user, password)) {
		return null;
	}

	return user;
}

// Find user helper
function findUserByEmail(email: string): User | null {
	// Mock implementation
	return new User("Test", email);
}

// Check password helper
function checkPassword(user: User, password: string): boolean {
	// Mock implementation
	return password === "secret";
}

// Create admin function
function createAdmin(name: string, email: string): AdminUser {
	const user = new AdminUser(name, email, 1);
	saveUser(user);
	return user;
}

// Save user function
function saveUser(user: User): void {
	console.log("Saving user: " + user.name);
}

// Process users with conditional logic
function processUsers(users: User[]): void {
	for (const user of users) {
		if (user.admin) {
			processAdminUser(user);
		} else {
			processRegularUser(user);
		}
	}
}

// Process admin user
function processAdminUser(user: User): void {
	console.log("Admin: " + user.name);
}

// Process regular user
function processRegularUser(user: User): void {
	console.log("Regular: " + user.name);
}

// Arrow function example
const validateEmail = (email: string): boolean => {
	return email.includes("@");
};

// Chained method calls
function processWithChaining(): void {
	const result = getData().filter(x => x > 0).map(x => x * 2);
	console.log(result);
}

// External function (simulated)
function getData(): number[] {
	return [1, 2, 3];
}

// Async function example
async function fetchUser(id: string): Promise<User> {
	const data = await fetchData(id);
	return new User(data.name, data.email);
}

// Fetch data helper
async function fetchData(id: string): Promise<{name: string, email: string}> {
	return { name: "Test", email: "test@example.com" };
}

// Class with method calling another method
class ServiceManager {
	private userService: UserService;

	constructor() {
		this.userService = new UserService();
	}

	processRequest(): void {
		this.userService.handleUser();
		this.internalMethod();
	}

	private internalMethod(): void {
		console.log("Internal processing");
	}
}

class UserService {
	handleUser(): void {
		console.log("Handling user");
	}
}
`

func setupTypeScriptTestExtractor(t *testing.T, source string) (*TypeScriptCallGraphExtractor, *parser.ParseResult) {
	p, err := parser.NewParser(parser.TypeScript)
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
	entities := extractTypeScriptTestEntities(result)

	extractor := NewTypeScriptCallGraphExtractor(result, entities)
	return extractor, result
}

// extractTypeScriptTestEntities extracts entities for testing
func extractTypeScriptTestEntities(result *parser.ParseResult) []CallGraphEntity {
	var entities []CallGraphEntity
	id := 0

	result.WalkNodes(func(node *sitter.Node) bool {
		switch node.Type() {
		case "function_declaration":
			name := extractTSFunctionName(node, result)
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

		case "method_definition":
			name := extractTSMethodName(node, result)
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

		case "class_declaration":
			name := extractTSClassName(node, result)
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

		case "interface_declaration":
			name := extractTSInterfaceName(node, result)
			if name != "" {
				id++
				entities = append(entities, CallGraphEntity{
					ID:       fmt.Sprintf("interface-%d", id),
					Name:     name,
					Type:     "interface",
					Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
					Node:     node,
				})
			}

		case "arrow_function":
			// Arrow functions assigned to variables
			parent := node.Parent()
			if parent != nil && parent.Type() == "variable_declarator" {
				nameNode := parent.ChildByFieldName("name")
				if nameNode != nil {
					name := result.NodeText(nameNode)
					if name != "" {
						id++
						entities = append(entities, CallGraphEntity{
							ID:       fmt.Sprintf("arrow-%d", id),
							Name:     name,
							Type:     "arrow_function",
							Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
							Node:     node,
						})
					}
				}
			}
		}
		return true
	})

	return entities
}

func extractTSFunctionName(node *sitter.Node, result *parser.ParseResult) string {
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

func extractTSMethodName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	// Fallback
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "property_identifier" || child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func extractTSClassName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	// Fallback
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" || child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func extractTSInterfaceName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	// Fallback
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "type_identifier" || child.Type() == "identifier" {
			return result.NodeText(child)
		}
	}
	return ""
}

func TestNewTypeScriptCallGraphExtractor(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

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

		// Check if IUser interface is indexed
		if _, ok := extractor.entityByName["IUser"]; !ok {
			t.Error("expected IUser to be in entityByName map")
		}
	})
}

func TestTypeScriptExtractFunctionCalls(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts direct function calls", func(t *testing.T) {
		// login should call findUserByEmail
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "findUserByEmail" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to findUserByEmail")
		}
	})

	t.Run("extracts multiple calls from same function", func(t *testing.T) {
		// login calls findUserByEmail and checkPassword
		callsFromLogin := 0
		loginEntity := extractor.entityByName["login"]
		if loginEntity == nil {
			t.Fatal("login entity not found")
		}

		for _, dep := range deps {
			if dep.FromID == loginEntity.ID && dep.DepType == Calls {
				callsFromLogin++
			}
		}

		// Should have at least findUserByEmail and checkPassword calls
		if callsFromLogin < 2 {
			t.Errorf("expected at least 2 calls from login, got %d", callsFromLogin)
		}
	})
}

func TestTypeScriptConstructorCalls(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts new expression calls", func(t *testing.T) {
		// findUserByEmail creates new User()
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

	t.Run("extracts AdminUser constructor call", func(t *testing.T) {
		// createAdmin creates new AdminUser()
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "AdminUser" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find constructor call to AdminUser")
		}
	})
}

func TestTypeScriptConditionalCallDetection(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("detects conditional calls in if statements", func(t *testing.T) {
		// processUsers has conditional calls to processAdminUser and processRegularUser
		conditionalCalls := 0
		for _, dep := range deps {
			if dep.DepType == Calls && dep.Optional {
				if dep.ToName == "processAdminUser" || dep.ToName == "processRegularUser" {
					conditionalCalls++
				}
			}
		}

		if conditionalCalls < 2 {
			t.Errorf("expected at least 2 conditional calls, got %d", conditionalCalls)
		}
	})

	t.Run("non-conditional calls are not marked optional", func(t *testing.T) {
		// saveUser call from createAdmin is not inside an if condition
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "saveUser" {
				found = true
				if dep.Optional {
					t.Error("saveUser call should not be conditional")
				}
				break
			}
		}
		if !found {
			t.Error("expected to find saveUser call")
		}
	})
}

func TestTypeScriptExtractTypeReferences(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts type references from function signatures", func(t *testing.T) {
		// login returns User | null
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
				if extractor.isBuiltinType(dep.ToName) {
					t.Errorf("builtin type %s should not be in dependencies", dep.ToName)
				}
			}
		}
	})
}

func TestTypeScriptClassInheritance(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts extends relationship", func(t *testing.T) {
		// AdminUser extends User
		found := false
		adminUserEntity := extractor.entityByName["AdminUser"]
		if adminUserEntity == nil {
			t.Fatal("AdminUser entity not found")
		}

		for _, dep := range deps {
			if dep.FromID == adminUserEntity.ID && dep.DepType == Extends && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find extends relationship from AdminUser to User")
		}
	})

	t.Run("extracts implements relationship", func(t *testing.T) {
		// User implements IUser
		found := false
		userEntity := extractor.entityByName["User"]
		if userEntity == nil {
			t.Fatal("User entity not found")
		}

		for _, dep := range deps {
			if dep.FromID == userEntity.ID && dep.DepType == Implements && dep.ToName == "IUser" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find implements relationship from User to IUser")
		}
	})

	t.Run("extracts multiple implements", func(t *testing.T) {
		// AdminUser implements IAdminAuth (which extends IAuthenticator)
		adminUserEntity := extractor.entityByName["AdminUser"]
		if adminUserEntity == nil {
			t.Fatal("AdminUser entity not found")
		}

		found := false
		for _, dep := range deps {
			if dep.FromID == adminUserEntity.ID && dep.DepType == Implements && dep.ToName == "IAdminAuth" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find implements relationship from AdminUser to IAdminAuth")
		}
	})

	t.Run("extracts interface extends", func(t *testing.T) {
		// IAdminAuth extends IAuthenticator
		adminAuthEntity := extractor.entityByName["IAdminAuth"]
		if adminAuthEntity == nil {
			t.Fatal("IAdminAuth entity not found")
		}

		found := false
		for _, dep := range deps {
			if dep.FromID == adminAuthEntity.ID && dep.DepType == Extends && dep.ToName == "IAuthenticator" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find extends relationship from IAdminAuth to IAuthenticator")
		}
	})
}

func TestTypeScriptMethodOwner(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts method_of relationship", func(t *testing.T) {
		// greet method is a method of User
		greetEntity := extractor.entityByName["greet"]
		if greetEntity == nil {
			t.Fatal("greet entity not found")
		}

		found := false
		for _, dep := range deps {
			if dep.FromID == greetEntity.ID && dep.DepType == MethodOf && dep.ToName == "User" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find method_of relationship from greet to User")
		}
	})
}

func TestTypeScriptIsBuiltinType(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	builtins := []string{
		"string", "number", "boolean", "any", "void", "null", "undefined",
		"Array", "Object", "Function", "Promise", "Map", "Set",
		"Error", "Date", "RegExp", "JSON", "Math", "console",
		"Partial", "Required", "Readonly", "Record", "Pick", "Omit",
	}

	for _, builtin := range builtins {
		t.Run(builtin, func(t *testing.T) {
			if !extractor.isBuiltinType(builtin) {
				t.Errorf("%s should be identified as builtin", builtin)
			}
		})
	}

	nonBuiltins := []string{"User", "IUser", "AdminUser", "MyType", "CustomService"}
	for _, nonBuiltin := range nonBuiltins {
		t.Run(nonBuiltin, func(t *testing.T) {
			if extractor.isBuiltinType(nonBuiltin) {
				t.Errorf("%s should not be identified as builtin", nonBuiltin)
			}
		})
	}
}

func TestTypeScriptDependencyLocation(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

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

func TestTypeScriptEmptyInput(t *testing.T) {
	t.Run("handles empty source", func(t *testing.T) {
		p, err := parser.NewParser(parser.TypeScript)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte(""))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		extractor := NewTypeScriptCallGraphExtractor(result, nil)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("handles entities without nodes", func(t *testing.T) {
		p, err := parser.NewParser(parser.TypeScript)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("const x = 1;"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		// Entity without a node
		entities := []CallGraphEntity{
			{ID: "test-1", Name: "TestFunc", Type: "function", Node: nil},
		}

		extractor := NewTypeScriptCallGraphExtractor(result, entities)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		if len(deps) != 0 {
			t.Errorf("expected 0 deps for entity without node, got %d", len(deps))
		}
	})
}

func TestTypeScriptChainedCalls(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts getData from chained call", func(t *testing.T) {
		// processWithChaining calls getData().filter().map()
		// Should extract getData as a call
		processEntity := extractor.entityByName["processWithChaining"]
		if processEntity == nil {
			t.Fatal("processWithChaining entity not found")
		}

		found := false
		for _, dep := range deps {
			if dep.FromID == processEntity.ID && dep.DepType == Calls && dep.ToName == "getData" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to getData from processWithChaining")
		}
	})
}

func TestTypeScriptAsyncCalls(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts calls from async functions", func(t *testing.T) {
		// fetchUser calls fetchData
		fetchUserEntity := extractor.entityByName["fetchUser"]
		if fetchUserEntity == nil {
			t.Fatal("fetchUser entity not found")
		}

		found := false
		for _, dep := range deps {
			if dep.FromID == fetchUserEntity.ID && dep.DepType == Calls && dep.ToName == "fetchData" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to fetchData from fetchUser")
		}
	})
}

func TestTypeScriptArrowFunctions(t *testing.T) {
	extractor, _ := setupTypeScriptTestExtractor(t, testTypeScriptCallGraphSource)

	t.Run("arrow function entity exists", func(t *testing.T) {
		// validateEmail should be extracted as an arrow function
		entity := extractor.entityByName["validateEmail"]
		if entity == nil {
			t.Error("expected validateEmail arrow function entity to exist")
		}
	})
}

// Test JavaScript compatibility (same parser)
func TestJavaScriptCompatibility(t *testing.T) {
	const jsSource = `
function greet(name) {
	console.log("Hello, " + name);
}

function processData(items) {
	const filtered = items.filter(x => x > 0);
	const result = new DataProcessor();
	return result.process(filtered);
}

class DataProcessor {
	process(items) {
		return items.map(x => x * 2);
	}
}
`

	p, err := parser.NewParser(parser.JavaScript)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	t.Cleanup(func() { p.Close() })

	result, err := p.Parse([]byte(jsSource))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	t.Cleanup(func() { result.Close() })

	entities := extractTypeScriptTestEntities(result)
	extractor := NewTypeScriptCallGraphExtractor(result, entities)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts function calls from JavaScript", func(t *testing.T) {
		// greet calls console.log
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && strings.Contains(dep.ToName, "log") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find call to console.log")
		}
	})

	t.Run("extracts constructor calls from JavaScript", func(t *testing.T) {
		// processData creates new DataProcessor()
		found := false
		for _, dep := range deps {
			if dep.DepType == Calls && dep.ToName == "DataProcessor" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find constructor call to DataProcessor")
		}
	})
}

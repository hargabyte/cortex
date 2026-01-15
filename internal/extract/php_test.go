package extract

import (
	"fmt"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

func parsePHPCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.PHP)
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

func TestExtractPHPClass(t *testing.T) {
	code := `<?php
class User {
    private string $id;
    private string $email;
    private int $age;

    public function __construct(string $id, string $email) {
        $this->id = $id;
        $this->email = $email;
    }

    public function getId(): string {
        return $this->id;
    }

    public function setEmail(string $email): void {
        $this->email = $email;
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have class + constructor + 2 methods + 3 properties
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
}

func TestExtractPHPInterface(t *testing.T) {
	code := `<?php
interface Repository {
    public function findById(string $id): ?object;
    public function save(object $entity): void;
    public function delete(string $id): void;
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find the interface entity
	var repo *Entity
	for i := range entities {
		if entities[i].Name == "Repository" && entities[i].Kind == TypeEntity {
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
}

func TestExtractPHPTrait(t *testing.T) {
	code := `<?php
trait Timestampable {
    private ?\DateTime $createdAt = null;
    private ?\DateTime $updatedAt = null;

    public function setCreatedAt(\DateTime $createdAt): self {
        $this->createdAt = $createdAt;
        return $this;
    }

    public function getCreatedAt(): ?\DateTime {
        return $this->createdAt;
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find the trait entity
	var trait *Entity
	for i := range entities {
		if entities[i].Name == "Timestampable" && entities[i].Kind == TypeEntity {
			trait = &entities[i]
			break
		}
	}

	if trait == nil {
		t.Fatal("Timestampable trait not found")
	}

	if trait.ValueType != "trait" {
		t.Errorf("expected ValueType 'trait', got %v", trait.ValueType)
	}
}

func TestExtractPHPEnum(t *testing.T) {
	code := `<?php
enum Status: string {
    case Pending = 'pending';
    case Active = 'active';
    case Completed = 'completed';
    case Cancelled = 'cancelled';
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
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

	// Check enum values
	if len(status.EnumValues) != 4 {
		t.Errorf("expected 4 enum values, got %d", len(status.EnumValues))
	}
}

func TestExtractPHPFunction(t *testing.T) {
	code := `<?php
function calculateTotal(array $items, float $taxRate = 0.1): float {
    $subtotal = 0;
    foreach ($items as $item) {
        $subtotal += $item['price'] * $item['quantity'];
    }
    return $subtotal * (1 + $taxRate);
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find the function entity
	var calcFunc *Entity
	for i := range entities {
		if entities[i].Name == "calculateTotal" && entities[i].Kind == FunctionEntity {
			calcFunc = &entities[i]
			break
		}
	}

	if calcFunc == nil {
		t.Fatal("calculateTotal function not found")
	}

	if len(calcFunc.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(calcFunc.Params))
	}

	if len(calcFunc.Returns) != 1 || calcFunc.Returns[0] != "float" {
		t.Errorf("expected return type 'float', got %v", calcFunc.Returns)
	}
}

func TestExtractPHPMethod(t *testing.T) {
	code := `<?php
class Calculator {
    public function add(int $a, int $b): int {
        return $a + $b;
    }

    public static function multiply(float $x, float $y): float {
        return $x * $y;
    }

    private function reset(): void {
        // reset state
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
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

func TestExtractPHPInheritance(t *testing.T) {
	code := `<?php
class Animal {
    protected string $name;
}

class Dog extends Animal implements Runnable, Comparable {
    private string $breed;

    public function run(): void {}

    public function compareTo(object $other): int {
        return 0;
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
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

func TestExtractPHPAbstractClass(t *testing.T) {
	code := `<?php
abstract class Shape {
    protected string $color;

    abstract public function area(): float;

    public function setColor(string $color): void {
        $this->color = $color;
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
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

func TestExtractPHPTraitUsage(t *testing.T) {
	code := `<?php
trait Logger {
    public function log(string $message): void {
        echo $message;
    }
}

trait Serializable {
    public function serialize(): string {
        return json_encode($this);
    }
}

class Service {
    use Logger, Serializable;

    public function process(): void {
        $this->log("Processing...");
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find Service class
	var serviceClass *Entity
	for i := range entities {
		if entities[i].Name == "Service" && entities[i].Kind == TypeEntity {
			serviceClass = &entities[i]
			break
		}
	}

	if serviceClass == nil {
		t.Fatal("Service class not found")
	}

	// Check that traits are stored in Decorators
	if len(serviceClass.Decorators) < 2 {
		t.Errorf("expected at least 2 used traits, got %d", len(serviceClass.Decorators))
	}
}

func TestExtractPHPProperties(t *testing.T) {
	code := `<?php
class Constants {
    public const VERSION = "1.0.0";
    private static int $counter = 0;
    protected float $value;
    public readonly string $name;
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find properties/constants
	var versionConst, counterProp *Entity
	for i := range entities {
		if entities[i].Kind == ConstEntity {
			if entities[i].Name == "VERSION" {
				versionConst = &entities[i]
			}
		}
		if entities[i].Kind == VarEntity {
			if entities[i].Name == "counter" {
				counterProp = &entities[i]
			}
		}
	}

	// Check VERSION constant
	if versionConst == nil {
		t.Error("VERSION constant not found")
	} else if versionConst.Visibility != VisibilityPublic {
		t.Errorf("VERSION: expected public visibility, got %v", versionConst.Visibility)
	}

	// Check counter property
	if counterProp == nil {
		t.Error("counter property not found")
	} else {
		if counterProp.Visibility != VisibilityPrivate {
			t.Errorf("counter: expected private visibility, got %v", counterProp.Visibility)
		}
		if !strings.Contains(counterProp.Receiver, "static") {
			t.Errorf("counter: expected receiver to contain 'static', got %q", counterProp.Receiver)
		}
	}
}

func TestExtractPHPUnionTypes(t *testing.T) {
	code := `<?php
class Handler {
    public function handle(int|string $input): bool|null {
        return null;
    }
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Find handle method
	var handleMethod *Entity
	for i := range entities {
		if entities[i].Name == "handle" && entities[i].Kind == MethodEntity {
			handleMethod = &entities[i]
			break
		}
	}

	if handleMethod == nil {
		t.Fatal("handle method not found")
	}

	// Check parameters include union type
	if len(handleMethod.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(handleMethod.Params))
	}
}

func TestPHPExtractAllWithNodes(t *testing.T) {
	code := `<?php
namespace App;

use DateTime;

class Service {
    private string $name;

    public function process(): void {}
}
`
	result := parsePHPCode(t, code)
	defer result.Close()

	ext := NewPHPExtractor(result)
	ewns, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	// Should have entities with nodes
	if len(ewns) < 2 {
		t.Fatalf("expected at least 2 entities with nodes, got %d", len(ewns))
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

func TestPHPVisibilityDetermination(t *testing.T) {
	tests := []struct {
		modifiers []string
		expected  Visibility
	}{
		{[]string{"public"}, VisibilityPublic},
		{[]string{"private"}, VisibilityPrivate},
		{[]string{"protected"}, VisibilityPrivate},
		{[]string{}, VisibilityPublic}, // PHP default is public
		{[]string{"public", "static"}, VisibilityPublic},
		{[]string{"private", "readonly"}, VisibilityPrivate},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.modifiers, "_"), func(t *testing.T) {
			got := determinePHPVisibility(tt.modifiers)
			if got != tt.expected {
				t.Errorf("determinePHPVisibility(%v) = %v, want %v", tt.modifiers, got, tt.expected)
			}
		})
	}
}

// Sample PHP source for testing call graph extraction
const testPHPCallGraphSource = `<?php
namespace App;

use App\Repository\UserRepository;
use App\Auth\Authenticator;

// User represents a user in the system
class User {
    private string $name;
    private string $email;
    private bool $admin = false;

    public function __construct(string $name, string $email) {
        $this->name = $name;
        $this->email = $email;
    }

    public function getName(): string {
        return $this->name;
    }

    public function getEmail(): string {
        return $this->email;
    }

    public function isAdmin(): bool {
        return $this->admin;
    }

    public function setAdmin(bool $admin): void {
        $this->admin = $admin;
    }
}

// Authenticator interface for authentication
interface AuthenticatorInterface {
    public function validate(string $token): bool;
}

// TokenAuth implements Authenticator
class TokenAuth implements AuthenticatorInterface {
    private string $secret;

    public function __construct(string $secret) {
        $this->secret = $secret;
    }

    public function validate(string $token): bool {
        return str_starts_with($token, $this->secret);
    }
}

// AuthService handles authentication
class AuthService {
    private AuthenticatorInterface $authenticator;
    private UserRepository $userRepository;

    public function __construct(AuthenticatorInterface $auth, UserRepository $repo) {
        $this->authenticator = $auth;
        $this->userRepository = $repo;
    }

    public function login(string $email, string $password): ?User {
        if (empty($email)) {
            throw new \InvalidArgumentException("Email required");
        }

        $user = $this->userRepository->findByEmail($email);
        if ($user === null) {
            return null;
        }

        if (!$this->checkPassword($user, $password)) {
            return null;
        }

        return $user;
    }

    private function checkPassword(User $user, string $password): bool {
        // Mock implementation
        return $password === "secret";
    }

    public function createAdmin(string $name, string $email): User {
        $user = new User($name, $email);
        $user->setAdmin(true);
        $this->userRepository->save($user);
        return $user;
    }

    public function processUsers(array $users): void {
        foreach ($users as $user) {
            if ($user->isAdmin()) {
                $this->processAdmin($user);
            } else {
                $this->processRegular($user);
            }
        }
    }

    private function processAdmin(User $user): void {
        echo "Admin: " . $user->getName();
    }

    private function processRegular(User $user): void {
        echo "Regular: " . $user->getName();
    }
}

// UserRepository interface
interface UserRepository {
    public function findByEmail(string $email): ?User;
    public function save(User $user): void;
}

// ExtendedAuth extends TokenAuth
class ExtendedAuth extends TokenAuth {
    private LoggerInterface $logger;

    public function __construct(string $secret, LoggerInterface $logger) {
        parent::__construct($secret);
        $this->logger = $logger;
    }

    public function validate(string $token): bool {
        $this->logger->log("Validating token");
        return parent::validate($token);
    }
}

// Logger interface
interface LoggerInterface {
    public function log(string $message): void;
}

// Builder pattern example with chained calls
class UserBuilder {
    private string $name = '';
    private string $email = '';

    public function setName(string $name): self {
        $this->name = $name;
        return $this;
    }

    public function setEmail(string $email): self {
        $this->email = $email;
        return $this;
    }

    public function build(): User {
        return new User($this->name, $this->email);
    }
}

// Class implementing multiple interfaces
interface SerializableInterface {
    public function serialize(): string;
}

interface CloneableInterface {
    public function clone(): object;
}

class DataObject implements SerializableInterface, CloneableInterface {
    public function serialize(): string {
        return json_encode($this);
    }

    public function clone(): object {
        return new DataObject();
    }
}

// Trait usage
trait Loggable {
    public function logMessage(string $msg): void {
        echo $msg;
    }
}

class ServiceWithTrait {
    use Loggable;

    public function process(): void {
        $this->logMessage("Processing...");
    }
}

// Static method calls
class Helper {
    public static function staticMethod(): void {}
    public static function calculate(int $x): int {
        return $x * 2;
    }
}

class Client {
    public function test(): void {
        Helper::staticMethod();
        $result = Helper::calculate(5);
    }
}
`

func setupPHPTestExtractor(t *testing.T, source string) (*PHPCallGraphExtractor, *parser.ParseResult) {
	p, err := parser.NewParser(parser.PHP)
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
	entities := extractPHPTestEntities(result)

	extractor := NewPHPCallGraphExtractor(result, entities)
	return extractor, result
}

// extractPHPTestEntities extracts entities for testing
func extractPHPTestEntities(result *parser.ParseResult) []CallGraphEntity {
	var entities []CallGraphEntity
	id := 0

	result.WalkNodes(func(node *sitter.Node) bool {
		switch node.Type() {
		case "class_declaration", "interface_declaration", "trait_declaration", "enum_declaration":
			name := extractPHPClassName(node, result)
			if name != "" {
				id++
				entityType := "class"
				if node.Type() == "interface_declaration" {
					entityType = "interface"
				} else if node.Type() == "trait_declaration" {
					entityType = "trait"
				} else if node.Type() == "enum_declaration" {
					entityType = "enum"
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
			name := extractPHPMethodName(node, result)
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

		case "function_definition":
			// Only top-level functions, not methods
			parent := node.Parent()
			isMethod := false
			for parent != nil {
				if parent.Type() == "declaration_list" {
					isMethod = true
					break
				}
				parent = parent.Parent()
			}
			if !isMethod {
				name := extractPHPFunctionName(node, result)
				if name != "" {
					id++
					entities = append(entities, CallGraphEntity{
						ID:       fmt.Sprintf("function-%d", id),
						Name:     name,
						Type:     "function",
						Location: fmt.Sprintf(":%d", node.StartPoint().Row+1),
						Node:     node,
					})
				}
			}
		}
		return true
	})

	return entities
}

func extractPHPClassName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	// Fallback
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "name" {
			return result.NodeText(child)
		}
	}
	return ""
}

func extractPHPMethodName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	return ""
}

func extractPHPFunctionName(node *sitter.Node, result *parser.ParseResult) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return result.NodeText(nameNode)
	}
	return ""
}

func TestNewPHPCallGraphExtractor(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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

func TestPHPExtractMethodCalls(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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
		// createAdmin calls $user->setAdmin
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
}

func TestPHPExtractConstructorCalls(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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
}

func TestPHPConditionalCallDetection(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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

func TestPHPExtractTypeReferences(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts type references from method parameters", func(t *testing.T) {
		// AuthService constructor takes AuthenticatorInterface and UserRepository
		foundAuthenticator := false
		foundUserRepository := false
		for _, dep := range deps {
			if dep.DepType == UsesType {
				if dep.ToName == "AuthenticatorInterface" {
					foundAuthenticator = true
				}
				if dep.ToName == "UserRepository" {
					foundUserRepository = true
				}
			}
		}
		if !foundAuthenticator {
			t.Error("expected to find AuthenticatorInterface type reference")
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
}

func TestPHPExtractClassRelations(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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
		// TokenAuth implements AuthenticatorInterface
		found := false
		for _, dep := range deps {
			if dep.DepType == Implements && dep.ToName == "AuthenticatorInterface" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find implements relationship to AuthenticatorInterface")
		}
	})

	t.Run("extracts multiple implements", func(t *testing.T) {
		// DataObject implements SerializableInterface, CloneableInterface
		foundSerializable := false
		foundCloneable := false
		for _, dep := range deps {
			if dep.DepType == Implements {
				if dep.ToName == "SerializableInterface" {
					foundSerializable = true
				}
				if dep.ToName == "CloneableInterface" {
					foundCloneable = true
				}
			}
		}
		if !foundSerializable {
			t.Error("expected to find implements relationship to SerializableInterface")
		}
		if !foundCloneable {
			t.Error("expected to find implements relationship to CloneableInterface")
		}
	})
}

func TestPHPExtractTraitUsage(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

	deps, err := extractor.ExtractDependencies()
	if err != nil {
		t.Fatalf("ExtractDependencies failed: %v", err)
	}

	t.Run("extracts trait usage", func(t *testing.T) {
		// ServiceWithTrait uses Loggable trait
		found := false
		for _, dep := range deps {
			if dep.DepType == Extends && dep.ToName == "Loggable" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find trait usage for Loggable")
		}
	})
}

func TestPHPExtractMethodOwner(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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
}

func TestPHPIsBuiltinType(t *testing.T) {
	extractor := &PHPCallGraphExtractor{}

	builtins := []string{
		"string", "int", "float", "bool", "array", "object", "callable", "iterable",
		"void", "never", "mixed", "self", "parent", "static",
		"Exception", "Error", "Throwable", "stdClass", "DateTime",
	}

	for _, builtin := range builtins {
		t.Run(builtin, func(t *testing.T) {
			if !extractor.isPHPBuiltinType(builtin) {
				t.Errorf("%s should be identified as builtin", builtin)
			}
		})
	}

	nonBuiltins := []string{"User", "AuthService", "TokenAuth", "MyCustomClass"}
	for _, nonBuiltin := range nonBuiltins {
		t.Run(nonBuiltin, func(t *testing.T) {
			if extractor.isPHPBuiltinType(nonBuiltin) {
				t.Errorf("%s should not be identified as builtin", nonBuiltin)
			}
		})
	}
}

func TestPHPIsBuiltinFunction(t *testing.T) {
	extractor := &PHPCallGraphExtractor{}

	builtins := []string{
		"echo", "print", "isset", "empty", "unset",
		"count", "strlen", "substr", "explode", "implode",
		"json_encode", "json_decode", "array_map", "array_filter",
	}

	for _, builtin := range builtins {
		t.Run(builtin, func(t *testing.T) {
			if !extractor.isPHPBuiltin(builtin) {
				t.Errorf("%s should be identified as builtin function", builtin)
			}
		})
	}

	nonBuiltins := []string{"myFunction", "processData", "getUserById"}
	for _, nonBuiltin := range nonBuiltins {
		t.Run(nonBuiltin, func(t *testing.T) {
			if extractor.isPHPBuiltin(nonBuiltin) {
				t.Errorf("%s should not be identified as builtin function", nonBuiltin)
			}
		})
	}
}

func TestPHPDependencyLocation(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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

func TestPHPEmptyInput(t *testing.T) {
	t.Run("handles empty source", func(t *testing.T) {
		p, err := parser.NewParser(parser.PHP)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("<?php"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		extractor := NewPHPCallGraphExtractor(result, nil)
		deps, err := extractor.ExtractDependencies()
		if err != nil {
			t.Fatalf("ExtractDependencies failed: %v", err)
		}

		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("handles entities without nodes", func(t *testing.T) {
		p, err := parser.NewParser(parser.PHP)
		if err != nil {
			t.Fatalf("NewParser failed: %v", err)
		}
		defer p.Close()

		result, err := p.Parse([]byte("<?php"))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		defer result.Close()

		// Entity without a node
		entities := []CallGraphEntity{
			{ID: "test-1", Name: "TestMethod", Type: "method", Node: nil},
		}

		extractor := NewPHPCallGraphExtractor(result, entities)
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

func TestPHPStaticMethodCalls(t *testing.T) {
	extractor, _ := setupPHPTestExtractor(t, testPHPCallGraphSource)

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
			t.Error("expected to find static call to Helper::staticMethod")
		}
		if !foundCalculate {
			t.Error("expected to find static call to Helper::calculate")
		}
	})
}

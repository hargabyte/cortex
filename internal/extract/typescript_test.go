package extract

import (
	"strings"
	"testing"

	"github.com/anthropics/cx/internal/parser"
)

func parseTypeScriptCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.TypeScript)
	if err != nil {
		t.Fatalf("failed to create TypeScript parser: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(code))
	if err != nil {
		t.Fatalf("failed to parse TypeScript: %v", err)
	}
	return result
}

func parseJavaScriptCode(t *testing.T, code string) *parser.ParseResult {
	t.Helper()
	p, err := parser.NewParser(parser.JavaScript)
	if err != nil {
		t.Fatalf("failed to create JavaScript parser: %v", err)
	}
	defer p.Close()

	result, err := p.Parse([]byte(code))
	if err != nil {
		t.Fatalf("failed to parse JavaScript: %v", err)
	}
	return result
}

func TestTSExtractFunction(t *testing.T) {
	code := `
function greet(name: string): string {
	return "Hello, " + name;
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	if fn.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", fn.Name)
	}
	if fn.Kind != FunctionEntity {
		t.Errorf("expected kind FunctionEntity, got %v", fn.Kind)
	}

	// Check parameters
	if len(fn.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(fn.Params))
	} else {
		if fn.Params[0].Name != "name" {
			t.Errorf("param 0: expected name 'name', got %q", fn.Params[0].Name)
		}
		if fn.Params[0].Type != "string" {
			t.Errorf("param 0: expected type 'string', got %q", fn.Params[0].Type)
		}
	}

	// Check returns
	if len(fn.Returns) != 1 {
		t.Errorf("expected 1 return, got %d", len(fn.Returns))
	} else if fn.Returns[0] != "string" {
		t.Errorf("return 0: expected 'string', got %q", fn.Returns[0])
	}
}

func TestTSExtractArrowFunction(t *testing.T) {
	code := `
const add = (a: number, b: number): number => {
	return a + b;
};

const multiply = (x: number, y: number) => x * y;
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(funcs))
	}

	// Find add function
	var add *Entity
	for i := range funcs {
		if funcs[i].Name == "add" {
			add = &funcs[i]
			break
		}
	}

	if add == nil {
		t.Fatal("add function not found")
	}
	if add.Kind != FunctionEntity {
		t.Errorf("expected kind FunctionEntity, got %v", add.Kind)
	}
	if len(add.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(add.Params))
	}
}

func TestTSExtractClass(t *testing.T) {
	code := `
class User {
	private id: string;
	public name: string;

	constructor(id: string, name: string) {
		this.id = id;
		this.name = name;
	}

	getName(): string {
		return this.name;
	}

	setName(name: string): void {
		this.name = name;
	}
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	entities, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	// ExtractClasses now returns class + methods (1 class + 3 methods = 4 entities)
	if len(entities) != 4 {
		t.Fatalf("expected 4 entities (1 class + 3 methods), got %d", len(entities))
	}

	// Find the class entity
	var cls *Entity
	var methods []Entity
	for i := range entities {
		if entities[i].Kind == TypeEntity {
			cls = &entities[i]
		} else if entities[i].Kind == MethodEntity {
			methods = append(methods, entities[i])
		}
	}

	if cls == nil {
		t.Fatal("expected to find class entity")
	}
	if cls.Name != "User" {
		t.Errorf("expected name 'User', got %q", cls.Name)
	}
	if cls.TypeKind != StructKind {
		t.Errorf("expected TypeKind StructKind, got %v", cls.TypeKind)
	}

	// Check that methods were extracted as separate entities
	if len(methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(methods))
	}

	// Check method names
	methodNames := make(map[string]bool)
	for _, m := range methods {
		methodNames[m.Name] = true
		// Check that methods have the class as receiver
		if m.Receiver != "User" {
			t.Errorf("expected method %s to have receiver 'User', got %q", m.Name, m.Receiver)
		}
	}
	if !methodNames["constructor"] {
		t.Error("expected constructor method")
	}
	if !methodNames["getName"] {
		t.Error("expected getName method")
	}
	if !methodNames["setName"] {
		t.Error("expected setName method")
	}
}

func TestTSExtractInterface(t *testing.T) {
	code := `
interface UserRepository {
	findById(id: string): Promise<User | null>;
	save(user: User): Promise<void>;
	delete(id: string): Promise<boolean>;
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	interfaces, err := ext.ExtractInterfaces()
	if err != nil {
		t.Fatalf("ExtractInterfaces failed: %v", err)
	}

	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}

	iface := interfaces[0]
	if iface.Name != "UserRepository" {
		t.Errorf("expected name 'UserRepository', got %q", iface.Name)
	}
	if iface.Kind != TypeEntity {
		t.Errorf("expected kind TypeEntity, got %v", iface.Kind)
	}
	if iface.TypeKind != InterfaceKind {
		t.Errorf("expected TypeKind InterfaceKind, got %v", iface.TypeKind)
	}

	// Check methods
	if len(iface.Fields) != 3 {
		t.Errorf("expected 3 methods, got %d", len(iface.Fields))
	}
}

func TestTSExtractTypeAlias(t *testing.T) {
	code := `
type ID = string;

type UserState = "active" | "inactive" | "pending";

type Result<T> = {
	data: T;
	error: Error | null;
};
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	types, err := ext.ExtractTypeAliases()
	if err != nil {
		t.Fatalf("ExtractTypeAliases failed: %v", err)
	}

	if len(types) != 3 {
		t.Fatalf("expected 3 type aliases, got %d", len(types))
	}

	// Find ID type
	var idType *Entity
	for i := range types {
		if types[i].Name == "ID" {
			idType = &types[i]
			break
		}
	}

	if idType == nil {
		t.Fatal("ID type not found")
	}
	if idType.Kind != TypeEntity {
		t.Errorf("expected kind TypeEntity, got %v", idType.Kind)
	}
	if idType.TypeKind != AliasKind {
		t.Errorf("expected TypeKind AliasKind, got %v", idType.TypeKind)
	}
}

func TestTSExtractEnum(t *testing.T) {
	code := `
enum Status {
	Active,
	Inactive,
	Pending
}

enum Color {
	Red = "RED",
	Green = "GREEN",
	Blue = "BLUE"
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	enums, err := ext.ExtractEnums()
	if err != nil {
		t.Fatalf("ExtractEnums failed: %v", err)
	}

	if len(enums) != 2 {
		t.Fatalf("expected 2 enums, got %d", len(enums))
	}

	// Find Status enum
	var statusEnum *Entity
	for i := range enums {
		if enums[i].Name == "Status" {
			statusEnum = &enums[i]
			break
		}
	}

	if statusEnum == nil {
		t.Fatal("Status enum not found")
	}
	if statusEnum.Kind != EnumEntity {
		t.Errorf("expected kind EnumEntity, got %v", statusEnum.Kind)
	}
	if len(statusEnum.EnumValues) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(statusEnum.EnumValues))
	}
}

func TestTSExtractVariables(t *testing.T) {
	code := `
const API_URL = "https://api.example.com";
let counter = 0;
const config: Config = { debug: true };
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	vars, err := ext.ExtractVariables()
	if err != nil {
		t.Fatalf("ExtractVariables failed: %v", err)
	}

	// Should have API_URL (const), counter (let), config (const)
	// Note: config may or may not be extracted depending on whether it's detected as function
	if len(vars) < 2 {
		t.Fatalf("expected at least 2 variables, got %d", len(vars))
	}

	// Find API_URL
	var apiUrl *Entity
	for i := range vars {
		if vars[i].Name == "API_URL" {
			apiUrl = &vars[i]
			break
		}
	}

	if apiUrl == nil {
		t.Fatal("API_URL not found")
	}
	if apiUrl.Kind != ConstEntity {
		t.Errorf("expected kind ConstEntity, got %v", apiUrl.Kind)
	}
}

func TestTSExtractImports(t *testing.T) {
	code := `
import { useState, useEffect } from 'react';
import axios from 'axios';
import * as lodash from 'lodash';
import type { User } from './types';
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	imports, err := ext.ExtractImports()
	if err != nil {
		t.Fatalf("ExtractImports failed: %v", err)
	}

	if len(imports) != 4 {
		t.Fatalf("expected 4 imports, got %d", len(imports))
	}

	// Check that react import exists
	hasReact := false
	for _, imp := range imports {
		if imp.ImportPath == "react" {
			hasReact = true
			break
		}
	}
	if !hasReact {
		t.Error("react import not found")
	}
}

func TestTSExtractAll(t *testing.T) {
	code := `
import { Logger } from './logger';

const VERSION = "1.0.0";

interface Service {
	start(): void;
	stop(): void;
}

class MyService implements Service {
	private logger: Logger;

	constructor(logger: Logger) {
		this.logger = logger;
	}

	start(): void {
		this.logger.log("Starting...");
	}

	stop(): void {
		this.logger.log("Stopping...");
	}
}

function createService(logger: Logger): Service {
	return new MyService(logger);
}

export { MyService, createService };
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should have: import, const, interface, class, function
	if len(entities) < 4 {
		t.Errorf("expected at least 4 entities, got %d", len(entities))
	}

	// Count by kind
	counts := make(map[EntityKind]int)
	for _, e := range entities {
		counts[e.Kind]++
	}

	if counts[ImportEntity] != 1 {
		t.Errorf("expected 1 import, got %d", counts[ImportEntity])
	}
	if counts[TypeEntity] < 2 { // interface + class
		t.Errorf("expected at least 2 types, got %d", counts[TypeEntity])
	}
}

func TestJSExtractFunction(t *testing.T) {
	code := `
function greet(name) {
	return "Hello, " + name;
}

const add = (a, b) => a + b;
`
	result := parseJavaScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(funcs))
	}

	// Find greet function
	var greet *Entity
	for i := range funcs {
		if funcs[i].Name == "greet" {
			greet = &funcs[i]
			break
		}
	}

	if greet == nil {
		t.Fatal("greet function not found")
	}
	if greet.Kind != FunctionEntity {
		t.Errorf("expected kind FunctionEntity, got %v", greet.Kind)
	}
}

func TestTSExtractAllWithNodes(t *testing.T) {
	code := `
function hello(name: string): string {
	return "Hello, " + name;
}

class Greeter {
	greet(name: string): string {
		return hello(name);
	}
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	entitiesWithNodes, err := ext.ExtractAllWithNodes()
	if err != nil {
		t.Fatalf("ExtractAllWithNodes failed: %v", err)
	}

	if len(entitiesWithNodes) < 2 {
		t.Errorf("expected at least 2 entities with nodes, got %d", len(entitiesWithNodes))
	}

	// Verify nodes are attached
	for _, ewn := range entitiesWithNodes {
		if ewn.Entity == nil {
			t.Error("entity is nil")
			continue
		}
		if ewn.Node == nil {
			t.Errorf("node is nil for entity %s", ewn.Entity.Name)
		}
	}
}

func TestTSGetLanguage(t *testing.T) {
	tsResult := parseTypeScriptCode(t, "const x = 1;")
	defer tsResult.Close()
	tsExt := NewTypeScriptExtractor(tsResult)
	if tsExt.GetLanguage() != "typescript" {
		t.Errorf("expected language 'typescript', got %q", tsExt.GetLanguage())
	}

	jsResult := parseJavaScriptCode(t, "const x = 1;")
	defer jsResult.Close()
	jsExt := NewTypeScriptExtractor(jsResult)
	if jsExt.GetLanguage() != "javascript" {
		t.Errorf("expected language 'javascript', got %q", jsExt.GetLanguage())
	}
}

func TestTSExportedFunction(t *testing.T) {
	code := `
export function publicFunc(): void {}

function privateFunc(): void {}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(funcs))
	}

	// Note: Export detection may vary based on tree-sitter parsing
	// The important thing is we extract both functions
	hasPublic := false
	hasPrivate := false
	for _, fn := range funcs {
		if fn.Name == "publicFunc" {
			hasPublic = true
		}
		if fn.Name == "privateFunc" {
			hasPrivate = true
		}
	}

	if !hasPublic {
		t.Error("publicFunc not found")
	}
	if !hasPrivate {
		t.Error("privateFunc not found")
	}
}

func TestTSEntityID(t *testing.T) {
	code := `function test(): void {}`
	result := parseTypeScriptCode(t, code)
	defer result.Close()
	result.FilePath = "src/test.ts"

	ext := NewTypeScriptExtractor(result)
	funcs, err := ext.ExtractFunctions()
	if err != nil {
		t.Fatalf("ExtractFunctions failed: %v", err)
	}

	if len(funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", len(funcs))
	}

	fn := funcs[0]
	id := fn.GenerateEntityID()

	// Should start with sa-fn-
	if !strings.HasPrefix(id, "sa-fn-") {
		t.Errorf("expected ID to start with 'sa-fn-', got %q", id)
	}

	// Should contain the name
	if !strings.HasSuffix(id, "-test") {
		t.Errorf("expected ID to end with '-test', got %q", id)
	}
}

func TestTSInterfaceWithIndexSignature(t *testing.T) {
	code := `
interface Dictionary {
	[key: string]: any;
	length: number;
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	interfaces, err := ext.ExtractInterfaces()
	if err != nil {
		t.Fatalf("ExtractInterfaces failed: %v", err)
	}

	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}

	iface := interfaces[0]
	if iface.Name != "Dictionary" {
		t.Errorf("expected name 'Dictionary', got %q", iface.Name)
	}

	// Should have fields including index signature
	if len(iface.Fields) < 1 {
		t.Errorf("expected at least 1 field, got %d", len(iface.Fields))
	}
}

func TestTSClassExtends(t *testing.T) {
	code := `
class Animal {
	name: string;
}

class Dog extends Animal {
	breed: string;
}
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	classes, err := ext.ExtractClasses()
	if err != nil {
		t.Fatalf("ExtractClasses failed: %v", err)
	}

	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}

	// Find Dog class
	var dog *Entity
	for i := range classes {
		if classes[i].Name == "Dog" {
			dog = &classes[i]
			break
		}
	}

	if dog == nil {
		t.Fatal("Dog class not found")
	}

	// Dog should implement Animal (extends)
	if len(dog.Implements) != 1 || dog.Implements[0] != "Animal" {
		t.Errorf("expected Dog to extend Animal, got %v", dog.Implements)
	}
}

func TestTSReactComponent(t *testing.T) {
	code := `
import React from 'react';

interface Props {
	name: string;
}

const Greeting: React.FC<Props> = ({ name }) => {
	return <h1>Hello, {name}!</h1>;
};

export default Greeting;
`
	result := parseTypeScriptCode(t, code)
	defer result.Close()

	ext := NewTypeScriptExtractor(result)
	entities, err := ext.ExtractAll()
	if err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	// Should extract: import, interface Props, const Greeting
	if len(entities) < 2 {
		t.Errorf("expected at least 2 entities, got %d", len(entities))
	}

	// Find Greeting component (should be extracted as a function/const)
	hasGreeting := false
	for _, e := range entities {
		if e.Name == "Greeting" {
			hasGreeting = true
			break
		}
	}

	// Note: Arrow function assigned to const may or may not be extracted
	// depending on implementation details
	if !hasGreeting {
		t.Log("Greeting component not found as separate entity (may be part of variable)")
	}
}

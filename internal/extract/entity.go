// Package extract provides code entity extraction from parsed AST trees.
//
// This package extracts functions, types, constants, enums and other code
// entities from tree-sitter parsed AST, formatting them into a compact
// pipe-delimited format optimized for token efficiency.
package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// EntityKind represents the type of code entity.
type EntityKind string

const (
	// FunctionEntity represents a standalone function.
	FunctionEntity EntityKind = "function"
	// MethodEntity represents a method with a receiver.
	MethodEntity EntityKind = "method"
	// TypeEntity represents a type definition (struct, interface, alias, etc.).
	TypeEntity EntityKind = "type"
	// ConstEntity represents a constant declaration.
	ConstEntity EntityKind = "constant"
	// VarEntity represents a variable declaration.
	VarEntity EntityKind = "variable"
	// EnumEntity represents an enumeration type.
	EnumEntity EntityKind = "enum"
	// ImportEntity represents an import declaration.
	ImportEntity EntityKind = "import"
)

// TypeKind represents the specific kind of type definition.
type TypeKind string

const (
	// StructKind represents a struct type.
	StructKind TypeKind = "struct"
	// InterfaceKind represents an interface type.
	InterfaceKind TypeKind = "interface"
	// AliasKind represents a type alias.
	AliasKind TypeKind = "alias"
	// UnionKind represents a union/sum type.
	UnionKind TypeKind = "union"
)

// Visibility represents the visibility level of an entity.
type Visibility string

const (
	// VisibilityPublic is exported (visible outside package).
	VisibilityPublic Visibility = "pub"
	// VisibilityPrivate is unexported (package-private).
	VisibilityPrivate Visibility = "priv"
	// VisibilityProtected is protected (C++ style - visible to derived classes).
	VisibilityProtected Visibility = "prot"
)

// Field represents a struct/interface field or method.
type Field struct {
	// Name is the field name.
	Name string
	// Type is the field type (abbreviated form).
	Type string
	// Visibility is the field visibility (public, private, protected).
	Visibility Visibility
}

// Param represents a function parameter.
type Param struct {
	// Name is the parameter name.
	Name string
	// Type is the parameter type (abbreviated form).
	Type string
}

// Entity represents an extracted code entity.
type Entity struct {
	// Kind is the type of entity (function, method, type, etc.).
	Kind EntityKind
	// Name is the entity name.
	Name string
	// File is the source file path.
	File string
	// StartLine is the starting line number (1-based).
	StartLine uint32
	// EndLine is the ending line number (1-based).
	EndLine uint32

	// Function-specific fields
	// Params contains function/method parameters.
	Params []Param
	// Returns contains return types (abbreviated form).
	Returns []string
	// Receiver is the method receiver (e.g., "*Server").
	Receiver string

	// Type-specific fields
	// TypeKind is the specific kind of type (struct, interface, alias).
	TypeKind TypeKind
	// Fields contains struct/interface fields.
	Fields []Field
	// Implements contains interface names this type implements.
	Implements []string

	// Constant/Variable-specific fields
	// ValueType is the type of the constant/variable.
	ValueType string
	// Value is the constant/variable value (for constants).
	Value string

	// Enum-specific fields
	// EnumValues contains the enum member values.
	EnumValues []EnumValue

	// Import-specific fields
	// ImportPath is the import path.
	ImportPath string
	// ImportAlias is the import alias (if any).
	ImportAlias string

	// Common fields
	// Visibility is pub or priv.
	Visibility Visibility
	// SigHash is the signature hash (first 8 hex chars of SHA-256).
	SigHash string
	// BodyHash is the body hash for functions (first 8 hex chars of SHA-256).
	BodyHash string
	// RawBody is the raw body content (used for hash computation, not stored).
	RawBody string

	// Language-specific fields
	// Language is the programming language (go, python, rust, etc.).
	Language string
	// IsAsync indicates if the function is async (Python, JavaScript).
	IsAsync bool
	// Decorators contains decorator names (Python).
	Decorators []string

	// Documentation and skeleton fields for cx map
	// DocComment is the preceding comment block for this entity.
	DocComment string
	// Skeleton is the signature + doc comment + { ... } placeholder for functions,
	// or full declaration for types/constants.
	Skeleton string
}

// EnumValue represents an enum member.
type EnumValue struct {
	// Name is the enum member name.
	Name string
	// Value is the enum member value.
	Value string
}

// ToCompactDescription converts the entity to compact pipe-delimited format.
// Format varies by entity kind:
//
// Function/Method:
//
//	<file>:<line>[-<end>]|<sig>|<sig_hash>:<body_hash>[|r=<receiver>][|v=<visibility>]
//
// Type:
//
//	<file>:<line>[-<end>]|<kind>|{<fields>}|<sig_hash>[|i=<implements>]
//
// Constant/Variable:
//
//	<file>:<line>|<type>=<value>
//
// Enum:
//
//	<file>:<line>[-<end>]|<base_type>|[<values>]
//
// Import:
//
//	<file>:<line>|<import_path>[|<alias>]
func (e *Entity) ToCompactDescription() string {
	switch e.Kind {
	case FunctionEntity, MethodEntity:
		return e.formatFunctionDescription()
	case TypeEntity:
		return e.formatTypeDescription()
	case ConstEntity, VarEntity:
		return e.formatConstDescription()
	case EnumEntity:
		return e.formatEnumDescription()
	case ImportEntity:
		return e.formatImportDescription()
	default:
		return e.formatGenericDescription()
	}
}

// formatFunctionDescription formats a function/method entity.
func (e *Entity) formatFunctionDescription() string {
	var sb strings.Builder

	// Location: file:line[-end]
	sb.WriteString(e.formatLocation())
	sb.WriteByte('|')

	// Signature: (params)->returns
	sb.WriteString(e.formatSignature())
	sb.WriteByte('|')

	// Hashes: sig_hash:body_hash
	sb.WriteString(e.SigHash)
	sb.WriteByte(':')
	sb.WriteString(e.BodyHash)

	// Optional receiver for methods
	if e.Receiver != "" {
		sb.WriteString("|r=")
		sb.WriteString(e.Receiver)
	}

	// Optional visibility
	if e.Visibility != "" {
		sb.WriteString("|v=")
		sb.WriteString(string(e.Visibility))
	}

	return sb.String()
}

// formatTypeDescription formats a type entity.
func (e *Entity) formatTypeDescription() string {
	var sb strings.Builder

	// Location: file:line[-end]
	sb.WriteString(e.formatLocation())
	sb.WriteByte('|')

	// Kind: struct, interface, alias, etc.
	sb.WriteString(string(e.TypeKind))
	sb.WriteByte('|')

	// Fields: {Name:Type,Name:Type}
	sb.WriteString(e.formatFields())
	sb.WriteByte('|')

	// Signature hash
	sb.WriteString(e.SigHash)

	// Optional implements
	if len(e.Implements) > 0 {
		sb.WriteString("|i=")
		sb.WriteString(strings.Join(e.Implements, ","))
	}

	return sb.String()
}

// formatConstDescription formats a constant/variable entity.
func (e *Entity) formatConstDescription() string {
	var sb strings.Builder

	// Location: file:line (no end line for single declarations)
	sb.WriteString(e.File)
	sb.WriteByte(':')
	sb.WriteString(fmt.Sprintf("%d", e.StartLine))
	sb.WriteByte('|')

	// type=value
	sb.WriteString(e.ValueType)
	sb.WriteByte('=')
	sb.WriteString(e.Value)

	return sb.String()
}

// formatEnumDescription formats an enum entity.
func (e *Entity) formatEnumDescription() string {
	var sb strings.Builder

	// Location: file:line[-end]
	sb.WriteString(e.formatLocation())
	sb.WriteByte('|')

	// Base type
	sb.WriteString(e.ValueType)
	sb.WriteByte('|')

	// Values: [Name=val,Name=val,...]
	sb.WriteByte('[')
	for i, v := range e.EnumValues {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(v.Name)
		sb.WriteByte('=')
		sb.WriteString(v.Value)
	}
	sb.WriteByte(']')

	return sb.String()
}

// formatImportDescription formats an import entity.
func (e *Entity) formatImportDescription() string {
	var sb strings.Builder

	// Location: file:line
	sb.WriteString(e.File)
	sb.WriteByte(':')
	sb.WriteString(fmt.Sprintf("%d", e.StartLine))
	sb.WriteByte('|')

	// Import path
	sb.WriteString(e.ImportPath)

	// Optional alias
	if e.ImportAlias != "" {
		sb.WriteByte('|')
		sb.WriteString(e.ImportAlias)
	}

	return sb.String()
}

// formatGenericDescription formats any entity generically.
func (e *Entity) formatGenericDescription() string {
	return fmt.Sprintf("%s:%d|%s|%s", e.File, e.StartLine, e.Kind, e.Name)
}

// formatLocation formats the file:line[-end] location string.
func (e *Entity) formatLocation() string {
	if e.EndLine > e.StartLine {
		return fmt.Sprintf("%s:%d-%d", e.File, e.StartLine, e.EndLine)
	}
	return fmt.Sprintf("%s:%d", e.File, e.StartLine)
}

// FormatSignature formats the (params) -> returns signature string.
// This is the public version for use by other packages.
func (e *Entity) FormatSignature() string {
	return e.formatSignature()
}

// formatSignature formats the (params) -> returns signature string.
func (e *Entity) formatSignature() string {
	var sb strings.Builder

	// Parameters
	sb.WriteByte('(')
	for i, p := range e.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Name != "" {
			sb.WriteString(p.Name)
			sb.WriteString(": ")
		}
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')

	// Return types
	if len(e.Returns) > 0 {
		sb.WriteString(" -> ")
		if len(e.Returns) == 1 {
			sb.WriteString(e.Returns[0])
		} else {
			sb.WriteByte('(')
			sb.WriteString(strings.Join(e.Returns, ", "))
			sb.WriteByte(')')
		}
	}

	return sb.String()
}

// formatFields formats the {Name: Type, Name: Type} fields string.
func (e *Entity) formatFields() string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, f := range e.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteString(": ")
		sb.WriteString(f.Type)
	}
	sb.WriteByte('}')
	return sb.String()
}

// GenerateEntityID creates a stable entity ID.
// Format: sa-<type>-<path-hash>-<name>
//
// Components:
//   - sa: Static analysis prefix
//   - type: fn/type/const/enum/mod/imp
//   - path-hash: Truncated SHA-256 of file path (6 chars)
//   - name: Symbol name (sanitized, max 32 chars)
func (e *Entity) GenerateEntityID() string {
	typeCode := e.getTypeCode()
	pathHash := hashString(e.File)[:6]
	name := sanitizeName(e.Name)
	// Include line number to disambiguate entities with same name in same file
	// (e.g., local variables named 'err' in different functions)
	return fmt.Sprintf("sa-%s-%s-%d-%s", typeCode, pathHash, e.StartLine, name)
}

// getTypeCode returns the short type code for the entity kind.
func (e *Entity) getTypeCode() string {
	switch e.Kind {
	case FunctionEntity:
		return "fn"
	case MethodEntity:
		return "fn"
	case TypeEntity:
		return "type"
	case ConstEntity:
		return "const"
	case VarEntity:
		return "var"
	case EnumEntity:
		return "enum"
	case ImportEntity:
		return "imp"
	default:
		return "unk"
	}
}

// ComputeHashes calculates the signature and body hashes.
func (e *Entity) ComputeHashes() {
	e.SigHash = e.computeSignatureHash()
	if e.RawBody != "" {
		e.BodyHash = hashString(normalizeBody(e.RawBody))[:8]
	}
}

// computeSignatureHash computes hash of the entity signature.
func (e *Entity) computeSignatureHash() string {
	var sb strings.Builder
	sb.WriteString(e.Name)

	// For functions/methods, include param types and return types
	if e.Kind == FunctionEntity || e.Kind == MethodEntity {
		for _, p := range e.Params {
			sb.WriteByte(',')
			sb.WriteString(p.Type)
		}
		sb.WriteByte('-')
		sb.WriteByte('>')
		for i, r := range e.Returns {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(r)
		}
		if e.Receiver != "" {
			sb.WriteByte('|')
			sb.WriteString(e.Receiver)
		}
	}

	// For types, include kind and field types
	if e.Kind == TypeEntity {
		sb.WriteByte('|')
		sb.WriteString(string(e.TypeKind))
		for _, f := range e.Fields {
			sb.WriteByte(',')
			sb.WriteString(f.Name)
			sb.WriteByte(':')
			sb.WriteString(f.Type)
		}
	}

	return hashString(sb.String())[:8]
}

// hashString computes SHA-256 hash of a string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// normalizeBody normalizes function body for hashing.
// Removes whitespace variations to make hash stable.
func normalizeBody(body string) string {
	// Remove leading/trailing whitespace from each line
	lines := strings.Split(body, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}

// sanitizeName sanitizes entity name for use in ID.
func sanitizeName(name string) string {
	if len(name) > 32 {
		name = name[:32]
	}
	// Replace any non-alphanumeric chars with underscore
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// DetermineVisibility determines visibility from Go naming convention.
// In Go, names starting with uppercase are exported (public).
func DetermineVisibility(name string) Visibility {
	if len(name) == 0 {
		return VisibilityPrivate
	}
	first := name[0]
	if first >= 'A' && first <= 'Z' {
		return VisibilityPublic
	}
	return VisibilityPrivate
}

// NormalizePath normalizes file path for consistent storage.
func NormalizePath(path, basePath string) string {
	if basePath == "" {
		return filepath.Clean(path)
	}
	rel, err := filepath.Rel(basePath, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return rel
}

// ToCallGraphEntity converts an Entity to a CallGraphEntity for use with CallGraphExtractor.
// The Node field will be nil and must be set separately if AST traversal is needed.
func (e *Entity) ToCallGraphEntity() CallGraphEntity {
	// Map EntityKind to callgraph type string
	typeStr := string(e.Kind)
	switch e.Kind {
	case FunctionEntity:
		typeStr = "function"
	case MethodEntity:
		typeStr = "method"
	case TypeEntity:
		// Use the more specific TypeKind if available
		if e.TypeKind != "" {
			typeStr = string(e.TypeKind)
		} else {
			typeStr = "type"
		}
	}

	// Build qualified name (receiver.Name for methods, just Name otherwise)
	qualifiedName := e.Name
	if e.Receiver != "" {
		// Strip pointer indicator for qualified name
		receiver := strings.TrimPrefix(e.Receiver, "*")
		qualifiedName = receiver + "." + e.Name
	}

	// Build location string
	location := fmt.Sprintf("%s:%d", e.File, e.StartLine)

	return CallGraphEntity{
		ID:            e.GenerateEntityID(),
		Name:          e.Name,
		QualifiedName: qualifiedName,
		Type:          typeStr,
		Location:      location,
		Node:          nil, // Must be set separately during scanning
	}
}

// BuildSkeleton builds the skeleton representation for an entity.
// For functions/methods: doc comment + signature + { ... }
// For types: doc comment + type definition
// For constants/variables: doc comment + declaration
func (e *Entity) BuildSkeleton() string {
	var sb strings.Builder

	// Add doc comment if present
	if e.DocComment != "" {
		sb.WriteString(e.DocComment)
		sb.WriteByte('\n')
	}

	switch e.Kind {
	case FunctionEntity:
		sb.WriteString("func ")
		sb.WriteString(e.Name)
		sb.WriteString(e.formatGoSignature())
		sb.WriteString(" { ... }")

	case MethodEntity:
		sb.WriteString("func ")
		if e.Receiver != "" {
			sb.WriteByte('(')
			// Add receiver name (usually single letter based on type)
			receiverName := "r"
			if len(e.Receiver) > 0 {
				typeName := strings.TrimPrefix(e.Receiver, "*")
				if len(typeName) > 0 {
					receiverName = strings.ToLower(string(typeName[0]))
				}
			}
			sb.WriteString(receiverName)
			sb.WriteByte(' ')
			sb.WriteString(e.Receiver)
			sb.WriteString(") ")
		}
		sb.WriteString(e.Name)
		sb.WriteString(e.formatGoSignature())
		sb.WriteString(" { ... }")

	case TypeEntity:
		sb.WriteString("type ")
		sb.WriteString(e.Name)
		sb.WriteByte(' ')
		if e.TypeKind == StructKind {
			sb.WriteString("struct {\n")
			for _, f := range e.Fields {
				sb.WriteString("\t")
				sb.WriteString(f.Name)
				sb.WriteByte(' ')
				sb.WriteString(f.Type)
				sb.WriteByte('\n')
			}
			sb.WriteByte('}')
		} else if e.TypeKind == InterfaceKind {
			sb.WriteString("interface {\n")
			for _, f := range e.Fields {
				sb.WriteString("\t")
				sb.WriteString(f.Name)
				sb.WriteString(f.Type) // Type contains the method signature
				sb.WriteByte('\n')
			}
			sb.WriteByte('}')
		} else {
			// Alias or other type
			sb.WriteString(string(e.TypeKind))
		}

	case ConstEntity:
		sb.WriteString("const ")
		sb.WriteString(e.Name)
		if e.ValueType != "" {
			sb.WriteByte(' ')
			sb.WriteString(e.ValueType)
		}
		if e.Value != "" {
			sb.WriteString(" = ")
			sb.WriteString(e.Value)
		}

	case VarEntity:
		sb.WriteString("var ")
		sb.WriteString(e.Name)
		if e.ValueType != "" {
			sb.WriteByte(' ')
			sb.WriteString(e.ValueType)
		}

	case ImportEntity:
		if e.ImportAlias != "" {
			sb.WriteString(e.ImportAlias)
			sb.WriteByte(' ')
		}
		sb.WriteString("\"")
		sb.WriteString(e.ImportPath)
		sb.WriteString("\"")

	default:
		sb.WriteString(e.Name)
	}

	return sb.String()
}

// formatGoSignature formats function parameters and return types in Go syntax.
func (e *Entity) formatGoSignature() string {
	var sb strings.Builder

	// Parameters
	sb.WriteByte('(')
	for i, p := range e.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Name != "" {
			sb.WriteString(p.Name)
			sb.WriteByte(' ')
		}
		sb.WriteString(p.Type)
	}
	sb.WriteByte(')')

	// Return types
	if len(e.Returns) == 1 {
		sb.WriteByte(' ')
		sb.WriteString(e.Returns[0])
	} else if len(e.Returns) > 1 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(e.Returns, ", "))
		sb.WriteByte(')')
	}

	return sb.String()
}

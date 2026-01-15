// Package extract provides call graph and dependency extraction from parsed AST.
// This file implements C-specific call graph extraction.
package extract

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/anthropics/cx/internal/parser"
)

// CCallGraphExtractor extracts dependencies from C AST
type CCallGraphExtractor struct {
	result       *parser.ParseResult
	entities     []CallGraphEntity
	entityByName map[string]*CallGraphEntity // Lookup by name for resolution
	entityByID   map[string]*CallGraphEntity // Lookup by ID
}

// NewCCallGraphExtractor creates a C call graph extractor
func NewCCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *CCallGraphExtractor {
	cge := &CCallGraphExtractor{
		result:       result,
		entities:     entities,
		entityByName: make(map[string]*CallGraphEntity),
		entityByID:   make(map[string]*CallGraphEntity),
	}

	// Build lookup maps
	for i := range entities {
		e := &entities[i]
		cge.entityByName[e.Name] = e
		if e.QualifiedName != "" {
			cge.entityByName[e.QualifiedName] = e
		}
		if e.ID != "" {
			cge.entityByID[e.ID] = e
		}
	}

	return cge
}

// ExtractDependencies extracts all dependencies from the parsed C code
func (cge *CCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
	var deps []Dependency

	for i := range cge.entities {
		entity := &cge.entities[i]
		if entity.Node == nil {
			continue
		}

		// Extract dependencies based on entity type
		switch entity.Type {
		case "function", "method":
			// Extract function calls
			callDeps := cge.extractFunctionCalls(entity)
			deps = append(deps, callDeps...)

			// Extract type references
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)

		case "struct", "union", "type":
			// Extract type references in struct fields
			typeDeps := cge.extractTypeReferences(entity)
			deps = append(deps, typeDeps...)
		}
	}

	return deps, nil
}

// extractFunctionCalls finds call_expression nodes within a function
func (cge *CCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency {
	var deps []Dependency

	// Get function body
	bodyNode := cge.findFunctionBody(entity.Node)
	if bodyNode == nil {
		return deps
	}

	// Track calls to deduplicate
	seen := make(map[string]bool)

	// Walk function body looking for call_expression nodes
	cge.walkNode(bodyNode, func(node *sitter.Node) bool {
		if node.Type() == "call_expression" {
			// Get function being called
			callTarget := cge.extractCallTarget(node)
			if callTarget != "" && !seen[callTarget] && !isCBuiltinFunction(callTarget) {
				seen[callTarget] = true

				dep := Dependency{
					FromID:   entity.ID,
					ToName:   callTarget,
					DepType:  Calls,
					Location: cge.nodeLocation(node),
				}

				// Check if call is conditional
				if cge.isConditionalCall(node) {
					dep.Optional = true
				}

				// Try to resolve to entity ID
				if target := cge.resolveTarget(callTarget); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}
		return true
	})

	return deps
}

// extractTypeReferences finds type identifiers used in the entity
func (cge *CCallGraphExtractor) extractTypeReferences(entity *CallGraphEntity) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Walk entity node looking for type references
	cge.walkNode(entity.Node, func(node *sitter.Node) bool {
		nodeType := node.Type()

		// Check for type identifiers
		if nodeType == "type_identifier" {
			typeName := cge.nodeText(node)
			if !seen[typeName] && !isCBuiltinType(typeName) {
				seen[typeName] = true
				dep := Dependency{
					FromID:   entity.ID,
					ToName:   typeName,
					DepType:  UsesType,
					Location: cge.nodeLocation(node),
				}

				// Try to resolve
				if target := cge.resolveTarget(typeName); target != nil {
					dep.ToID = target.ID
				}

				deps = append(deps, dep)
			}
		}

		// Check for struct/union/enum specifiers (e.g., struct MyStruct)
		if nodeType == "struct_specifier" || nodeType == "union_specifier" || nodeType == "enum_specifier" {
			// Get the type name
			nameNode := findChildByType(node, "type_identifier")
			if nameNode != nil {
				typeName := cge.nodeText(nameNode)
				if !seen[typeName] && !isCBuiltinType(typeName) {
					seen[typeName] = true
					dep := Dependency{
						FromID:   entity.ID,
						ToName:   typeName,
						DepType:  UsesType,
						Location: cge.nodeLocation(node),
					}

					if target := cge.resolveTarget(typeName); target != nil {
						dep.ToID = target.ID
					}

					deps = append(deps, dep)
				}
			}
		}

		return true
	})

	return deps
}

// isConditionalCall checks if a call is inside an if/switch statement
func (cge *CCallGraphExtractor) isConditionalCall(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "if_statement", "switch_statement", "conditional_expression":
			return true
		case "function_definition":
			return false // Reached function boundary
		}
		parent = parent.Parent()
	}
	return false
}

// isCBuiltinType checks if type is a C builtin
func isCBuiltinType(name string) bool {
	builtins := map[string]bool{
		// Basic types
		"int": true, "char": true, "short": true, "long": true,
		"float": true, "double": true, "void": true,
		"signed": true, "unsigned": true,

		// Fixed-width types from stdint.h
		"int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"intptr_t": true, "uintptr_t": true, "ptrdiff_t": true,

		// Size types
		"size_t": true, "ssize_t": true, "off_t": true,

		// Boolean (C99)
		"bool": true, "_Bool": true,

		// Wide character types
		"wchar_t": true, "wint_t": true,

		// FILE type
		"FILE": true,

		// Common typedefs
		"time_t": true, "clock_t": true,
	}
	return builtins[name]
}

// isCBuiltinFunction checks if function is a C standard library function
func isCBuiltinFunction(name string) bool {
	builtins := map[string]bool{
		// stdio.h
		"printf": true, "fprintf": true, "sprintf": true, "snprintf": true,
		"scanf": true, "fscanf": true, "sscanf": true,
		"puts": true, "fputs": true, "gets": true, "fgets": true,
		"putchar": true, "fputc": true, "putc": true,
		"getchar": true, "fgetc": true, "getc": true,
		"fopen": true, "fclose": true, "fflush": true,
		"fread": true, "fwrite": true,
		"fseek": true, "ftell": true, "rewind": true,
		"feof": true, "ferror": true, "clearerr": true,
		"perror": true, "remove": true, "rename": true,
		"tmpfile": true, "tmpnam": true,
		"setbuf": true, "setvbuf": true,
		"vprintf": true, "vfprintf": true, "vsprintf": true, "vsnprintf": true,
		"vscanf": true, "vfscanf": true, "vsscanf": true,

		// stdlib.h
		"malloc": true, "calloc": true, "realloc": true, "free": true,
		"exit": true, "_Exit": true, "abort": true, "atexit": true,
		"atoi": true, "atol": true, "atoll": true, "atof": true,
		"strtol": true, "strtoll": true, "strtoul": true, "strtoull": true,
		"strtod": true, "strtof": true, "strtold": true,
		"rand": true, "srand": true,
		"qsort": true, "bsearch": true,
		"abs": true, "labs": true, "llabs": true,
		"div": true, "ldiv": true, "lldiv": true,
		"getenv": true, "system": true,

		// string.h
		"memcpy": true, "memmove": true, "memset": true, "memcmp": true, "memchr": true,
		"strcpy": true, "strncpy": true, "strcat": true, "strncat": true,
		"strcmp": true, "strncmp": true, "strcoll": true,
		"strlen": true, "strerror": true,
		"strchr": true, "strrchr": true, "strstr": true, "strtok": true,
		"strspn": true, "strcspn": true, "strpbrk": true,
		"strdup": true, "strndup": true,

		// ctype.h
		"isalnum": true, "isalpha": true, "isblank": true, "iscntrl": true,
		"isdigit": true, "isgraph": true, "islower": true, "isprint": true,
		"ispunct": true, "isspace": true, "isupper": true, "isxdigit": true,
		"tolower": true, "toupper": true,

		// math.h
		"sin": true, "cos": true, "tan": true,
		"asin": true, "acos": true, "atan": true, "atan2": true,
		"sinh": true, "cosh": true, "tanh": true,
		"exp": true, "log": true, "log10": true, "log2": true,
		"pow": true, "sqrt": true, "cbrt": true,
		"ceil": true, "floor": true, "round": true, "trunc": true,
		"fabs": true, "fmod": true, "remainder": true,
		"fmax": true, "fmin": true,
		"isnan": true, "isinf": true, "isfinite": true,

		// assert.h
		"assert": true,

		// errno.h - not functions but commonly used
		// time.h
		"time": true, "difftime": true, "mktime": true,
		"strftime": true, "localtime": true, "gmtime": true,
		"clock": true,

		// setjmp.h
		"setjmp": true, "longjmp": true,

		// signal.h
		"signal": true, "raise": true,

		// Common POSIX functions
		"open": true, "close": true, "read": true, "write": true,
		"lseek": true, "unlink": true, "link": true, "stat": true,
		"fstat": true, "lstat": true, "chmod": true, "chown": true,
		"mkdir": true, "rmdir": true, "getcwd": true, "chdir": true,
		"fork": true, "exec": true, "execl": true, "execv": true, "execve": true,
		"wait": true, "waitpid": true, "kill": true,
		"getpid": true, "getppid": true, "getuid": true, "geteuid": true,
		"sleep": true, "usleep": true, "nanosleep": true,
		"pipe": true, "dup": true, "dup2": true,
		"socket": true, "bind": true, "listen": true, "accept": true,
		"connect": true, "send": true, "recv": true,
		"mmap": true, "munmap": true,
		"pthread_create": true, "pthread_join": true, "pthread_exit": true,
		"pthread_mutex_init": true, "pthread_mutex_lock": true, "pthread_mutex_unlock": true,

		// Macros that look like functions
		"sizeof": true, "offsetof": true, "typeof": true,
		"NULL": true, "EOF": true,
	}
	return builtins[name]
}

// Helper methods

// walkNode performs a depth-first walk of the AST
func (cge *CCallGraphExtractor) walkNode(node *sitter.Node, fn func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	for i := uint32(0); i < node.ChildCount(); i++ {
		cge.walkNode(node.Child(int(i)), fn)
	}
}

// nodeText returns the source text for a node
func (cge *CCallGraphExtractor) nodeText(node *sitter.Node) string {
	if node == nil || cge.result.Source == nil {
		return ""
	}
	// Bounds check to prevent slice out of range panics
	endByte := node.EndByte()
	if endByte > uint32(len(cge.result.Source)) {
		return ""
	}
	return node.Content(cge.result.Source)
}

// nodeLocation returns file:line for a node
func (cge *CCallGraphExtractor) nodeLocation(node *sitter.Node) string {
	if node == nil {
		return ""
	}
	line := node.StartPoint().Row + 1 // tree-sitter is 0-indexed
	if cge.result.FilePath != "" {
		return fmt.Sprintf("%s:%d", cge.result.FilePath, line)
	}
	return fmt.Sprintf(":%d", line)
}

// extractCallTarget extracts the function name from a call_expression
func (cge *CCallGraphExtractor) extractCallTarget(node *sitter.Node) string {
	if node == nil || node.Type() != "call_expression" {
		return ""
	}

	// In C, the function being called is typically the first child
	// call_expression structure:
	// - function (identifier, field_expression, or parenthesized_expression)
	// - argument_list (arguments)

	funcNode := node.ChildByFieldName("function")
	if funcNode == nil && node.ChildCount() > 0 {
		funcNode = node.Child(0)
	}

	if funcNode == nil {
		return ""
	}

	switch funcNode.Type() {
	case "identifier":
		return cge.nodeText(funcNode)

	case "field_expression":
		// struct->field or struct.field - extract the field name
		// The field is the last identifier
		var fieldName string
		for i := uint32(0); i < funcNode.ChildCount(); i++ {
			child := funcNode.Child(int(i))
			if child.Type() == "field_identifier" {
				fieldName = cge.nodeText(child)
			}
		}
		if fieldName != "" {
			return fieldName
		}
		// Fallback: return the whole expression
		return cge.nodeText(funcNode)

	case "parenthesized_expression":
		// Function pointer call: (*func_ptr)()
		return ""

	case "subscript_expression":
		// Array of function pointers: funcs[0]()
		return ""
	}

	return ""
}

// findFunctionBody finds the compound_statement (block) in a function definition
func (cge *CCallGraphExtractor) findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	// For function_definition, the body is a compound_statement
	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child.Type() == "compound_statement" {
			return child
		}
	}

	return nil
}

// resolveTarget attempts to resolve a target name to an entity
func (cge *CCallGraphExtractor) resolveTarget(name string) *CallGraphEntity {
	if e, ok := cge.entityByName[name]; ok {
		return e
	}

	// Try without struct/union prefix for qualified names
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		// Try just the last part (field name)
		if e, ok := cge.entityByName[parts[len(parts)-1]]; ok {
			return e
		}
	}

	return nil
}

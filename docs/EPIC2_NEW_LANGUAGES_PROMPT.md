# Epic 2: Add Support for Additional Languages

## Objective
Add full Cortex support (parsing, entity extraction, and call graph extraction) for C, C++, C#, PHP, Kotlin, and Ruby. These are high-demand languages with mature tree-sitter grammars.

## Context

### Why These Languages?
Based on TIOBE Index January 2026:
| Language | Rank | Share | Use Case |
|----------|------|-------|----------|
| C | #2 | 10.99% | Systems, embedded, kernels |
| C++ | #4 | 8.67% | Games, browsers, performance |
| C# | #5 | 7.39% | .NET, Unity, enterprise |
| PHP | #15 | 1.38% | Web (WordPress, Laravel) |
| Kotlin | #20 | 0.97% | Android, JVM backend |
| Ruby | #27 | 0.58% | Rails, DevOps tools |

### Current Supported Languages
Cortex already supports: Go, TypeScript, JavaScript, Python, Rust, Java

### What Each New Language Needs
1. **Parser initialization** (~50-80 lines) - `internal/parser/<lang>.go`
2. **Entity extractor** (~500-1000 lines) - `internal/extract/<lang>.go`
3. **Call graph extractor** (~400-600 lines) - `internal/extract/callgraph_<lang>.go`
4. **Tests** (~300-400 lines) - `internal/extract/<lang>_test.go`

## Architecture Overview

```
internal/
├── parser/
│   ├── parser.go        # Language enum, NewParser dispatch
│   ├── go.go            # Example: Go parser init
│   ├── typescript.go    # Example: TS parser init
│   └── [NEW FILES]      # c.go, cpp.go, csharp.go, php.go, kotlin.go, ruby.go
├── extract/
│   ├── extract.go       # Base extractor (Go-centric)
│   ├── entity.go        # Entity types
│   ├── typescript.go    # Example: TS extractor
│   ├── callgraph.go     # Example: Go call graph
│   └── [NEW FILES]      # New extractors + call graphs
└── cmd/
    └── scan.go          # Scan pipeline (auto-detects language)
```

## Beads (Issue Tracking)

| Bead ID | Language | Priority | Estimate |
|---------|----------|----------|----------|
| cortex-8x6.1 | C | P2 | ~1000 lines |
| cortex-8x6.2 | C++ | P2 | ~1500 lines |
| cortex-8x6.3 | C# | P2 | ~1100 lines |
| cortex-8x6.4 | PHP | P2 | ~1000 lines |
| cortex-8x6.5 | Kotlin | P2 | ~1100 lines |
| cortex-8x6.6 | Ruby | P3 | ~900 lines |

Update bead status as you work:
```bash
bd update cortex-8x6.1 --status in_progress  # When starting
bd close cortex-8x6.1 --reason "Added full C language support"  # When done
```

---

## Implementation Pattern (Same for All Languages)

### Step 1: Add Parser (`internal/parser/<lang>.go`)

```go
// Example: internal/parser/c.go
package parser

import (
    sitter "github.com/smacker/go-tree-sitter"
    c "github.com/smacker/go-tree-sitter/c"  // or appropriate binding
)

func newCParser() *sitter.Parser {
    parser := sitter.NewParser()
    parser.SetLanguage(c.GetLanguage())
    return parser
}
```

### Step 2: Update Parser Dispatch (`internal/parser/parser.go`)

Add to the `Language` constants:
```go
const (
    // ... existing languages ...
    C       Language = "c"
    Cpp     Language = "cpp"
    CSharp  Language = "csharp"
    PHP     Language = "php"
    Kotlin  Language = "kotlin"
    Ruby    Language = "ruby"
)
```

Update `NewParser()` switch:
```go
case C:
    return &Parser{parser: newCParser(), language: lang}, nil
```

Update `LanguageFromExtension()`:
```go
case ".c", ".h":
    return C
case ".cpp", ".cc", ".cxx", ".hpp":
    return Cpp
// etc.
```

### Step 3: Create Entity Extractor (`internal/extract/<lang>.go`)

Follow the pattern from `typescript.go` or `python.go`:
```go
type CExtractor struct {
    result   *parser.ParseResult
    basePath string
}

func NewCExtractor(result *parser.ParseResult) *CExtractor { ... }

func (e *CExtractor) ExtractAll() ([]Entity, error) { ... }
func (e *CExtractor) ExtractAllWithNodes() ([]EntityWithNode, error) { ... }
func (e *CExtractor) ExtractFunctions() ([]Entity, error) { ... }
// ... etc
```

### Step 4: Create Call Graph Extractor (`internal/extract/callgraph_<lang>.go`)

Follow the pattern from `callgraph.go`:
```go
type CCallGraphExtractor struct {
    result       *parser.ParseResult
    entities     []CallGraphEntity
    entityByName map[string]*CallGraphEntity
    entityByID   map[string]*CallGraphEntity
}

func (e *CCallGraphExtractor) ExtractDependencies() ([]Dependency, error) { ... }
func (e *CCallGraphExtractor) extractFunctionCalls(entity *CallGraphEntity) []Dependency { ... }
// ... etc
```

### Step 5: Integrate with Scan Pipeline

Update `internal/cmd/scan.go` to use the new extractors based on file extension.

### Step 6: Write Tests

Create comprehensive tests in `internal/extract/<lang>_test.go`.

---

## Language-Specific Details

### 1. C Language (cortex-8x6.1)

**Tree-sitter**: `github.com/smacker/go-tree-sitter/c` (mature, official)

**File Extensions**: `.c`, `.h`

**Entity Types to Extract**:
| Entity | AST Node Type |
|--------|---------------|
| Functions | `function_definition` |
| Function declarations | `declaration` with `function_declarator` |
| Structs | `struct_specifier` |
| Unions | `union_specifier` |
| Enums | `enum_specifier` |
| Typedefs | `type_definition` |
| Macros | `preproc_def`, `preproc_function_def` |
| Global variables | `declaration` at file scope |

**Visibility Rules**:
- `static` keyword = private (file scope)
- No `static` = public (external linkage)

**Call Graph - Node Types**:
```
call_expression      # function calls
field_expression     # struct->field or struct.field
type_identifier      # type references
```

**Builtins to Filter**:
```
printf, scanf, malloc, free, memcpy, memset,
strlen, strcpy, strcmp, sizeof,
NULL, EOF, stdin, stdout, stderr
```

**C-Specific Considerations**:
- Header files (`.h`) contain declarations, not definitions
- Function pointers are hard to track (best effort)
- Macros can define functions (extract if possible)

---

### 2. C++ Language (cortex-8x6.2)

**Tree-sitter**: `github.com/smacker/go-tree-sitter/cpp` (mature, official)

**File Extensions**: `.cpp`, `.cc`, `.cxx`, `.hpp`, `.h` (when C++ detected)

**Entity Types to Extract**:
| Entity | AST Node Type |
|--------|---------------|
| Functions | `function_definition` |
| Classes | `class_specifier` |
| Structs | `struct_specifier` |
| Templates | `template_declaration` |
| Namespaces | `namespace_definition` |
| Methods | `function_definition` inside class |
| Constructors | `function_definition` with class name |
| Destructors | `destructor_name` |
| Operators | `operator_name` |
| Enums | `enum_specifier` (scoped and unscoped) |

**Visibility Rules**:
- `public`, `private`, `protected` keywords
- Default: `private` for class, `public` for struct
- `friend` declarations

**Call Graph - Node Types**:
```
call_expression           # function calls
new_expression            # constructor calls
field_expression          # obj.method or obj->method
qualified_identifier      # Namespace::function
template_function         # template instantiation
```

**Builtins to Filter**:
```
std::string, std::vector, std::map, std::set,
std::cout, std::cin, std::endl,
std::make_unique, std::make_shared,
new, delete, sizeof, typeid
```

**C++-Specific Complexity**:
- Templates are complex - extract template definitions, not instantiations
- Operator overloading creates implicit calls
- Virtual methods - track as regular calls
- Multiple inheritance - track all base classes

---

### 3. C# Language (cortex-8x6.3)

**Tree-sitter**: `github.com/smacker/go-tree-sitter/csharp` (mature, official)

**File Extensions**: `.cs`

**Entity Types to Extract**:
| Entity | AST Node Type |
|--------|---------------|
| Classes | `class_declaration` |
| Interfaces | `interface_declaration` |
| Structs | `struct_declaration` |
| Records | `record_declaration` |
| Enums | `enum_declaration` |
| Methods | `method_declaration` |
| Properties | `property_declaration` |
| Events | `event_declaration` |
| Delegates | `delegate_declaration` |
| Namespaces | `namespace_declaration` |

**Visibility Rules**:
- `public`, `private`, `protected`, `internal`, `protected internal`
- `partial` classes span multiple files
- `static` members

**Call Graph - Node Types**:
```
invocation_expression     # method calls
object_creation_expression # new ClassName()
member_access_expression  # obj.Method
element_access_expression # array[index]
```

**Builtins to Filter**:
```
string, int, long, double, float, bool, object, void,
String, Int32, Int64, Double, Boolean, Object,
Console, Math, Convert, Array,
List, Dictionary, HashSet, IEnumerable
```

**C#-Specific Features**:
- Properties have implicit getter/setter calls
- Events have add/remove handlers
- LINQ expressions (method syntax = method calls)
- Extension methods (called on instances)
- Async/await patterns

---

### 4. PHP Language (cortex-8x6.4)

**Tree-sitter**: `github.com/smacker/go-tree-sitter/php` (mature, v0.24.2)

**File Extensions**: `.php`

**Entity Types to Extract**:
| Entity | AST Node Type |
|--------|---------------|
| Functions | `function_definition` |
| Classes | `class_declaration` |
| Interfaces | `interface_declaration` |
| Traits | `trait_declaration` |
| Methods | `method_declaration` |
| Properties | `property_declaration` |
| Constants | `const_declaration` |
| Enums | `enum_declaration` (PHP 8.1+) |
| Namespaces | `namespace_definition` |

**Visibility Rules**:
- `public`, `private`, `protected`
- `static`, `final`, `abstract`
- `readonly` (PHP 8.1+)

**Call Graph - Node Types**:
```
function_call_expression    # function calls
member_call_expression      # $obj->method()
scoped_call_expression      # ClassName::staticMethod()
object_creation_expression  # new ClassName()
```

**Builtins to Filter**:
```
echo, print, isset, empty, unset,
array, string, int, float, bool, null,
count, strlen, substr, explode, implode,
json_encode, json_decode,
Exception, Error, stdClass
```

**PHP-Specific Considerations**:
- Mixed HTML/PHP files - only extract PHP parts
- Magic methods: `__construct`, `__call`, `__get`, `__set`
- Traits are like mixins - track `use` statements
- Anonymous classes

---

### 5. Kotlin Language (cortex-8x6.5)

**Tree-sitter**: `github.com/tree-sitter-grammars/tree-sitter-kotlin` (stable)

**File Extensions**: `.kt`, `.kts`

**Entity Types to Extract**:
| Entity | AST Node Type |
|--------|---------------|
| Functions | `function_declaration` |
| Classes | `class_declaration` |
| Interfaces | `class_declaration` with `interface` modifier |
| Objects | `object_declaration` |
| Data classes | `class_declaration` with `data` modifier |
| Sealed classes | `class_declaration` with `sealed` modifier |
| Enums | `class_declaration` with `enum` modifier |
| Properties | `property_declaration` |
| Type aliases | `type_alias` |

**Visibility Rules**:
- `public` (default), `private`, `protected`, `internal`
- `open`, `final`, `abstract`, `sealed`
- Companion object members

**Call Graph - Node Types**:
```
call_expression           # function calls
navigation_expression     # obj.method()
constructor_invocation    # ClassName()
```

**Builtins to Filter**:
```
String, Int, Long, Double, Float, Boolean, Unit, Any, Nothing,
List, MutableList, Map, MutableMap, Set,
println, print, require, check,
listOf, mapOf, setOf, arrayOf
```

**Kotlin-Specific Features**:
- Extension functions (defined outside class, called on instances)
- Null-safe calls (`?.`)
- Infix functions
- Operator overloading
- Coroutines (`suspend` functions)
- DSL builders

---

### 6. Ruby Language (cortex-8x6.6)

**Tree-sitter**: `github.com/tree-sitter/tree-sitter-ruby` (stable, official)

**File Extensions**: `.rb`, `.rake`, `Gemfile`, `Rakefile`

**Entity Types to Extract**:
| Entity | AST Node Type |
|--------|---------------|
| Methods | `method` |
| Classes | `class` |
| Modules | `module` |
| Singleton methods | `singleton_method` |
| Constants | `constant_assignment` (UPPER_CASE) |
| Attr accessors | `call` with `attr_reader`/`attr_writer`/`attr_accessor` |

**Visibility Rules**:
- `public`, `private`, `protected` (method-level)
- `class << self` for singleton/class methods
- Module mixins affect method resolution

**Call Graph - Node Types**:
```
call                      # method calls
method_call               # obj.method
```

**Builtins to Filter**:
```
puts, print, p, pp, raise, require, require_relative,
attr_reader, attr_writer, attr_accessor,
String, Integer, Float, Array, Hash, Symbol,
nil, true, false, self
```

**Ruby-Specific Considerations**:
- Metaprogramming (define_method, method_missing) - can't fully analyze
- Blocks, procs, lambdas
- DSL patterns (Rails, RSpec)
- Module include/extend

---

## Tree-Sitter Bindings

Check if bindings exist in `go.mod` or need to be added:

```bash
# Check current dependencies
cat go.mod | grep tree-sitter

# Add new language binding (example)
go get github.com/smacker/go-tree-sitter/c
go get github.com/smacker/go-tree-sitter/cpp
# etc.
```

**Known Go bindings** (github.com/smacker/go-tree-sitter):
- c, cpp, c_sharp (note: underscore)
- php, ruby
- Check for kotlin - may need alternative source

---

## Execution Plan

### Recommended Order
1. **C** - Simplest of the new languages, foundation for C++
2. **C#** - Similar to existing Java extractor
3. **PHP** - Straightforward, good web coverage
4. **C++** - Complex, builds on C
5. **Kotlin** - Similar to Java
6. **Ruby** - Lowest priority, dynamic language challenges

### Parallel Execution
- **Group 1** (can parallelize): C, C#, PHP
- **Group 2** (after Group 1): C++ (needs C patterns), Kotlin (reference Java)
- **Group 3** (last): Ruby

### Per-Language Workflow
```bash
# 1. Claim the work
bd update cortex-8x6.1 --status in_progress

# 2. Check tree-sitter binding availability
go get github.com/smacker/go-tree-sitter/c

# 3. Create parser
# internal/parser/c.go

# 4. Update parser.go dispatch

# 5. Create entity extractor
# internal/extract/c.go

# 6. Create call graph extractor
# internal/extract/callgraph_c.go

# 7. Create tests
# internal/extract/c_test.go

# 8. Test
go test ./internal/extract/... -run TestC -v
go test ./internal/parser/... -v

# 9. Integration test
# Create a small C project, scan it, verify entities and deps

# 10. Close the bead
bd close cortex-8x6.1 --reason "Added full C language support with entity and call graph extraction"
```

---

## Testing Strategy

### Unit Tests (per language)
Each extractor needs tests for:
1. Function/method extraction
2. Class/struct/type extraction
3. Constant/variable extraction
4. Import/include extraction
5. Call graph: function calls
6. Call graph: type references
7. Visibility detection
8. Builtin filtering

### Test File Pattern
Create realistic test files in the target language:
```go
// c_test.go
func TestCExtractFunctions(t *testing.T) {
    source := `
#include <stdio.h>

static void private_func() {}

void public_func(int x) {
    private_func();
    printf("hello");
}
`
    // Parse and extract
    // Verify: 2 functions, correct visibility, 2 calls from public_func
}
```

### Integration Test
After all languages done:
```bash
# Create a multi-language test project with C, C++, C#, PHP, Kotlin, Ruby files
mkdir test_multilang
# Add sample files for each language
cx scan test_multilang/
cx find --lang c          # Should find C entities
cx find --lang cpp        # Should find C++ entities
cx impact test_multilang/shared.c  # Should show dependents
```

---

## Success Criteria

1. **Parsing**: All file extensions correctly detected and parsed
2. **Entity Extraction**: Functions, types, classes, etc. extracted for each language
3. **Call Graph**: Dependencies correctly identified
4. **Integration**: `cx scan`, `cx find`, `cx impact`, `cx graph` work for all languages
5. **Tests**: Comprehensive test coverage for each extractor
6. **Documentation**: Code comments explain language-specific handling

---

## Commands Reference

```bash
# Beads management
bd show cortex-8x6           # View epic
bd show cortex-8x6.1         # View specific feature
bd update <id> --status in_progress
bd close <id> --reason "..."

# Go dependencies
go get <package>             # Add tree-sitter binding
go mod tidy                  # Clean up go.mod

# Testing
go test ./internal/parser/... -v
go test ./internal/extract/... -v
go test ./internal/extract/... -run TestC -v

# Build and verify
go build -o cx .
cx scan test_project/
cx find --lang c
cx impact test.c
```

---

## Files to Reference

| File | Purpose |
|------|---------|
| `internal/parser/parser.go` | Language enum, NewParser dispatch |
| `internal/parser/go.go` | Example parser init |
| `internal/parser/typescript.go` | Example parser init |
| `internal/extract/typescript.go` | Example entity extractor |
| `internal/extract/python.go` | Example entity extractor |
| `internal/extract/callgraph.go` | Reference call graph implementation |
| `internal/extract/entity.go` | Entity and Dependency types |
| `internal/cmd/scan.go` | Scan pipeline |

---

## Troubleshooting

### Tree-sitter binding not found
```bash
# Check available bindings
go list -m all | grep tree-sitter

# Try alternative sources
# smacker/go-tree-sitter has many languages bundled
# tree-sitter-grammars organization has individual repos
```

### Parser returns nil/errors
```bash
# Check language detection
cx scan --verbose test.c

# Verify tree-sitter grammar loaded
# Add debug logging to parser init
```

### Entities not extracted
```bash
# Dump AST to see node types
# Add verbose logging to extractor
# Compare with working language (e.g., Go)
```

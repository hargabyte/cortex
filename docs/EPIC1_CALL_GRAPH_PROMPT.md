# Epic 1: Complete Call Graph Support for Existing Languages

## Objective
Implement call graph extraction for TypeScript/JavaScript, Python, Java, and Rust. These languages already have entity extraction working, but lack dependency/call graph extraction which limits `cx impact`, `cx safe`, and `cx graph` functionality.

## Context

### Current State
- **Go**: Full call graph support ✅ (reference implementation)
- **TypeScript/JavaScript**: Entity extraction only, `ExtractAllWithNodes()` exists but unused
- **Python**: Entity extraction only, `ExtractAllWithNodes()` exists but unused
- **Java**: Entity extraction only, `ExtractAllWithNodes()` exists but unused
- **Rust**: Entity extraction only, `ExtractAllWithNodes()` exists but unused

### Why This Matters
The call graph is what makes Cortex powerful. Without it:
- `cx impact file.ts` can't show what breaks if you change a TypeScript file
- `cx safe file.py` can't assess blast radius for Python changes
- `cx graph Entity` only works for Go code

### Reference Implementation
Study the Go call graph extractor thoroughly - it's the pattern to follow:
- **File**: `internal/extract/callgraph.go` (651 lines)
- **Key types**: `CallGraphExtractor`, `Dependency`, `DepType`
- **Key methods**: `extractFunctionCalls()`, `extractTypeReferences()`, `extractMethodReceiver()`

## Architecture Overview

```
internal/
├── extract/
│   ├── callgraph.go          # Go call graph (REFERENCE - study this)
│   ├── callgraph_test.go     # Go call graph tests
│   ├── typescript.go         # TS entity extraction (has ExtractAllWithNodes)
│   ├── python.go             # Python entity extraction (has ExtractAllWithNodes)
│   ├── java.go               # Java entity extraction (has ExtractAllWithNodes)
│   ├── rust.go               # Rust entity extraction (has ExtractAllWithNodes)
│   └── entity.go             # Entity types and helpers
├── parser/
│   ├── parser.go             # Parser interface
│   ├── typescript.go         # TS parser init
│   ├── python.go             # Python parser init
│   ├── java.go               # Java parser init
│   └── rust.go               # Rust parser init
└── store/
    └── deps.go               # Dependency storage
```

## Beads (Issue Tracking)

| Bead ID | Language | Priority | Estimate |
|---------|----------|----------|----------|
| cortex-kry.1 | TypeScript/JavaScript | P1 | ~600 lines |
| cortex-kry.2 | Python | P1 | ~550 lines |
| cortex-kry.3 | Java | P1 | ~700 lines |
| cortex-kry.4 | Rust | P2 | ~850 lines |

Update bead status as you work:
```bash
bd update cortex-kry.1 --status in_progress  # When starting
bd close cortex-kry.1 --reason "Implemented TS/JS call graph extraction"  # When done
```

## Implementation Details by Language

### 1. TypeScript/JavaScript (cortex-kry.1)

**File to create**: `internal/extract/callgraph_typescript.go`

**AST Node Types** (tree-sitter-typescript):
```
call_expression          # function calls: foo(), obj.method()
new_expression           # constructor: new Class()
member_expression        # property access: obj.prop
type_identifier          # type references
extends_clause           # class extends
implements_clause        # class implements
```

**Call Patterns to Handle**:
```typescript
// Direct calls
functionName();
moduleName.functionName();

// Method calls
this.method();
object.method();
super.method();

// Constructor calls
new ClassName();
new module.ClassName();

// Chained calls
a.b().c().d();

// Async patterns
await asyncFunction();
promise.then(callback);
```

**Type References to Track**:
```typescript
class Foo extends Bar implements IBaz { }  // extends + implements
const x: SomeType = ...;                    // type annotation
function f(x: Type): ReturnType { }         // parameter + return types
```

**Builtin Types to Filter** (don't create dependencies for these):
```
string, number, boolean, any, void, null, undefined,
Array, Object, Function, Promise, Map, Set,
Error, Date, RegExp, JSON, Math, console
```

**Integration Point**:
The existing `TypeScriptExtractor.ExtractAllWithNodes()` returns `[]EntityWithNode`. Create a `TypeScriptCallGraphExtractor` that takes these and extracts dependencies.

---

### 2. Python (cortex-kry.2)

**File to create**: `internal/extract/callgraph_python.go`

**AST Node Types** (tree-sitter-python):
```
call                     # function/method calls (NOT call_expression!)
attribute                # obj.attr access
argument_list            # call arguments
class_definition         # for base classes
decorated_definition     # decorator chains
```

**Call Patterns to Handle**:
```python
# Direct calls
function_name()
module.function_name()

# Method calls
self.method()
obj.method()
super().method()
cls.classmethod()

# Class instantiation
ClassName()
module.ClassName()

# Decorator calls (decorators are calls!)
@decorator
@decorator_with_args(x)
def func(): ...
```

**Type References to Track**:
```python
class Foo(Bar, Baz):      # base classes
    pass

def f(x: Type) -> Return:  # type hints (Python 3.5+)
    y: AnotherType = ...   # variable annotations
```

**Builtin Types to Filter**:
```
str, int, float, bool, list, dict, set, tuple,
None, type, object, Exception,
print, len, range, enumerate, zip, map, filter
```

**Python-Specific Considerations**:
- Decorators ARE function calls - extract them
- `self` and `cls` first parameters are special
- Module-level code executes on import

---

### 3. Java (cortex-kry.3)

**File to create**: `internal/extract/callgraph_java.go`

**AST Node Types** (tree-sitter-java):
```
method_invocation           # method calls
object_creation_expression  # new ClassName()
field_access                # obj.field
type_identifier             # type references
superclass                  # extends clause
super_interfaces            # implements clause
```

**Call Patterns to Handle**:
```java
// Method calls
methodName();
this.methodName();
super.methodName();
object.methodName();
ClassName.staticMethod();

// Constructor calls
new ClassName();
new ClassName<Generic>();

// Chained calls
builder.setX().setY().build();
```

**Type References to Track**:
```java
class Foo extends Bar implements IBaz, IQux { }
public void method(ParamType p) throws SomeException { }
private FieldType field;
List<GenericType> list;
```

**Builtin Types to Filter**:
```
String, Integer, Long, Double, Float, Boolean,
Object, Class, Exception, RuntimeException,
List, Map, Set, Collection, Iterator,
System, Math, Arrays, Collections
```

---

### 4. Rust (cortex-kry.4)

**File to create**: `internal/extract/callgraph_rust.go`

**AST Node Types** (tree-sitter-rust):
```
call_expression             # direct function calls
method_call_expression      # obj.method() calls
scoped_identifier           # Type::associated_fn()
impl_item                   # impl blocks
trait_bounds                # T: Trait
type_identifier             # type references
```

**Call Patterns to Handle**:
```rust
// Direct calls
function_name();
module::function_name();

// Method calls
self.method();
object.method();

// Associated functions
Type::new();
Type::associated_fn();
<Type as Trait>::method();

// Macro calls (optional - lower priority)
println!();
vec![];
```

**Type References to Track**:
```rust
struct Foo<T: Trait> { }           // trait bounds
impl Trait for Type { }            // trait implementations
fn f<T: Bound>(x: ParamType) -> R  // generics + params + return
type Alias = ActualType;           // type aliases
```

**Builtin Types to Filter**:
```
String, str, i32, i64, u32, u64, f32, f64, bool,
Vec, HashMap, HashSet, Option, Result,
Box, Rc, Arc, RefCell, Mutex,
println, eprintln, format, panic
```

**Rust-Specific Complexity**:
- Trait method resolution is complex - do best effort
- Lifetimes can be ignored for call graph purposes
- Macro expansion is out of scope (just track macro calls if easy)

---

## Implementation Pattern

For each language, follow this pattern:

### Step 1: Create the Extractor Struct
```go
// callgraph_typescript.go
type TypeScriptCallGraphExtractor struct {
    result       *parser.ParseResult
    entities     []CallGraphEntity
    entityByName map[string]*CallGraphEntity
    entityByID   map[string]*CallGraphEntity
}

func NewTypeScriptCallGraphExtractor(result *parser.ParseResult, entities []CallGraphEntity) *TypeScriptCallGraphExtractor {
    // Build lookup maps (same pattern as Go)
}
```

### Step 2: Implement ExtractDependencies
```go
func (e *TypeScriptCallGraphExtractor) ExtractDependencies() ([]Dependency, error) {
    var deps []Dependency
    for _, entity := range e.entities {
        if entity.Node == nil {
            continue
        }
        switch entity.Type {
        case "function", "method":
            deps = append(deps, e.extractFunctionCalls(&entity)...)
            deps = append(deps, e.extractTypeReferences(&entity)...)
        case "class":
            deps = append(deps, e.extractClassRelations(&entity)...)
        }
    }
    return deps, nil
}
```

### Step 3: Implement Helper Methods
- `extractFunctionCalls()` - walk AST for call expressions
- `extractTypeReferences()` - find type identifiers
- `extractMethodReceiver()` - map methods to their class/type
- `isBuiltinType()` - filter out language primitives
- `resolveEntity()` - map names to entity IDs

### Step 4: Integrate with Scan Pipeline
Update `internal/cmd/scan.go` to use the new extractor when scanning non-Go files.

### Step 5: Write Tests
Create `callgraph_typescript_test.go` etc. with test cases for:
- Simple function calls
- Method calls
- Constructor calls
- Type references
- Chained calls
- Edge cases

---

## Testing Strategy

### Unit Tests
Each language extractor needs tests for:
1. Direct function calls
2. Method calls (instance and static)
3. Constructor/instantiation calls
4. Type references (extends, implements, annotations)
5. Builtin filtering (shouldn't create deps for builtins)
6. Qualified name resolution

### Integration Test
After all extractors are done, verify:
```bash
# Create a multi-language test project
cx scan test_project/
cx impact test_project/shared.ts  # Should show JS/TS dependents
cx graph SomeClass --hops 2       # Should work for all languages
```

---

## Execution Plan

### Recommended Order
1. **TypeScript/JavaScript** - Most common multi-language scenario, good ROI
2. **Python** - High demand, different enough to validate the pattern
3. **Java** - Similar to TS in structure, validates enterprise use case
4. **Rust** - Most complex, do last

### Parallel Execution
Languages 1-3 can be done in parallel by different agents since they don't share code. Rust should wait to learn from the others.

### Per-Language Workflow
```bash
# 1. Claim the work
bd update cortex-kry.1 --status in_progress

# 2. Study the reference implementation
cx show CallGraphExtractor --density dense
cat internal/extract/callgraph.go

# 3. Implement
# Create callgraph_<lang>.go
# Create callgraph_<lang>_test.go

# 4. Test
go test ./internal/extract/... -run TypeScript -v

# 5. Integration test
cx scan .  # Rescan cortex itself (has Go + some TS)
cx impact internal/extract/typescript.go

# 6. Close the bead
bd close cortex-kry.1 --reason "Implemented TypeScript/JavaScript call graph extraction with tests"
```

---

## Success Criteria

1. **Functional**: `cx impact` and `cx safe` work correctly for all 4 languages
2. **Complete**: All dependency types extracted (calls, uses_type, implements, extends)
3. **Tested**: Each extractor has comprehensive unit tests
4. **Integrated**: Scan pipeline uses new extractors automatically
5. **Documented**: Code has clear comments explaining language-specific handling

---

## Commands Reference

```bash
# Beads management
bd show cortex-kry           # View epic
bd show cortex-kry.1         # View specific feature
bd update <id> --status in_progress
bd close <id> --reason "..."

# Cortex commands for testing
cx scan .                    # Rescan codebase
cx find CallGraphExtractor   # Find reference implementation
cx show <entity>             # View entity details
cx impact <file>             # Test impact analysis
cx graph <entity> --hops 2   # Test graph generation

# Go commands
go test ./internal/extract/... -v           # Run all extract tests
go test ./internal/extract/... -run Python  # Run Python tests only
go build -o cx .                            # Rebuild cx
```

---

## Files to Reference

| File | Purpose |
|------|---------|
| `internal/extract/callgraph.go` | **Reference implementation** - study thoroughly |
| `internal/extract/callgraph_test.go` | Test patterns to follow |
| `internal/extract/typescript.go` | TS entity extraction (has ExtractAllWithNodes) |
| `internal/extract/python.go` | Python entity extraction |
| `internal/extract/java.go` | Java entity extraction |
| `internal/extract/rust.go` | Rust entity extraction |
| `internal/extract/entity.go` | Entity and Dependency types |
| `internal/cmd/scan.go` | Scan pipeline integration point |

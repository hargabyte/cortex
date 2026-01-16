# CX New Features Test Results - January 2026

## Summary

Tested all CX commands across the 6 officially supported languages. Most features work well. Found some entity persistence issues with complex Rust and Python files containing duplicate method names.

## Test Environment

- CX version: current (running from ~/cortex)
- Test date: 2026-01-16
- Languages tested: Go, TypeScript, JavaScript, Java, Rust, Python
- Test approach: Created dedicated test files for each language with comprehensive patterns

---

## Supported Languages Matrix

| Language | File Extensions | --lang flag | Status |
|----------|-----------------|-------------|--------|
| Go | .go | go | PASS |
| TypeScript | .ts, .tsx | typescript, ts | PASS |
| JavaScript | .js, .jsx, .mjs, .cjs | javascript, js | PASS |
| Java | .java | java | PASS |
| Rust | .rs | rust, rs | PASS (see notes) |
| Python | .py | python, py | PASS (see notes) |
| C | .c, .h | c | PASS |
| C++ | .cpp, .cc, .cxx, .hpp, .hh, .hxx, .h | cpp | PASS |
| C# | .cs | csharp, cs | PASS |
| PHP | .php | php | PASS |
| Kotlin | .kt, .kts | kotlin | PASS (requires --lang flag) |
| Ruby | .rb, .rake | ruby | PASS (requires --lang flag) |

---

## 1. Go Language Support - PASS

Tested using the Cortex codebase itself.

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan` | PASS | 3758 entities, 24538 dependencies |
| `cx db info` | PASS | Shows full database statistics |
| `cx doctor` | PASS | All checks passed |
| `cx status` | PASS | Shows daemon and graph status |
| `cx find Execute` | PASS | Found entity correctly |
| `cx find Execute --exact` | PASS | Exact match only |
| `cx find --important --top 10` | PASS | Returns top entities by PageRank |
| `cx find --keystones` | PASS | Returns empty (no entities marked) |
| `cx find --bottlenecks` | PASS | Returns empty |
| `cx show Execute` | PASS | Shows entity details |
| `cx show Execute --density dense` | PASS | Shows full details with hashes |
| `cx show Execute --related` | PASS | Shows neighborhood |
| `cx show Execute --graph --hops 2` | PASS | Shows dependency graph |
| `cx show Execute --graph --direction in` | PASS | Shows callers only |
| `cx show Execute --graph --direction out` | PASS | Shows callees only |
| `cx context` | PASS | Session recovery context |
| `cx context --smart "add rate limiting"` | PASS | Intent-aware context |
| `cx map --filter F --lang go` | PASS | Functions only |
| `cx safe internal/cmd/root.go` | PASS | Full safety assessment |
| `cx safe internal/cmd/root.go --quick` | PASS | Blast radius only |
| `cx safe --changes` | PASS | Changes since scan |
| `cx safe --drift` | PASS | Staleness check |
| `cx tag <entity> <tags>` | PASS | Add tags to entity |
| `cx tags <entity>` | PASS | List entity tags |
| `cx find --tag <tag>` | PASS | Find by tag |
| `cx trace main Execute` | PASS | Path between entities |
| `cx dead` | PASS | Find dead code (853 entities) |
| `cx help-agents` | PASS | Agent-optimized reference |
| `cx find --format json` | PASS | JSON output |
| `cx find --format jsonl` | PASS | JSONL output |
| `cx find --density sparse` | PASS | Minimal output |

---

## 2. TypeScript Language Support - PASS

Tested with custom test file containing interfaces, classes, and functions.

### Test File
```typescript
interface User { id: number; name: string; email: string; }
class UserService { private users: User[] = []; ... }
function formatUser(user: User): string { ... }
```

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang typescript` | PASS | 5 entities extracted |
| `cx db info` | PASS | Shows correct counts |
| `cx find UserService` | PASS | Found class |
| `cx show UserService --density dense` | PASS | Full details |
| `cx show formatUser --related` | PASS | Shows neighborhood |
| `cx show formatUser --graph` | PASS | Shows dependencies |
| `cx map` | PASS | Shows project skeleton |
| `cx safe test.ts --quick` | PASS | Blast radius analysis |
| `cx doctor` | PASS | All checks passed |

### Notes
- Requires `--lang typescript` flag for initial scan in some cases
- Auto-detection may skip test files (*.spec.ts)

---

## 3. JavaScript Language Support - PASS

Tested with custom test file containing classes and functions.

### Test File
```javascript
class Calculator { add(a, b) { return a + b; } ... }
function calculateSum(numbers) { ... }
const formatNumber = (num, precision = 2) => { ... };
```

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang javascript` | PASS | 5 entities extracted |
| `cx db info` | PASS | Shows correct counts |
| `cx find Calculator` | PASS | Found class |
| `cx show Calculator --density dense` | PASS | Full details |
| `cx show formatNumber --related` | PASS | Shows neighborhood |
| `cx map` | PASS | Shows project skeleton |
| `cx safe app.js --quick` | PASS | Blast radius analysis |
| `cx context` | PASS | Session recovery |
| `cx doctor` | PASS | All checks passed |

---

## 4. Java Language Support - PASS

Tested with custom test file containing interfaces, classes, and methods.

### Test File
```java
public class UserService {
    public void addUser(User user) { ... }
    public Optional<User> findById(int id) { ... }
}
interface Repository<T> { void save(T item); ... }
class User { ... }
```

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang java` | PASS | 21 entities, 13 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx find UserService` | PASS | Found class |
| `cx find findById` | PASS | Found method |
| `cx show findById --related` | PASS | Shows neighborhood |
| `cx show findById --graph --hops 2` | PASS | Shows dependency graph |
| `cx map` | PASS | Shows project skeleton (60+ lines) |
| `cx safe UserService.java` | PASS | Full safety assessment |
| `cx context --smart "add validation"` | PASS | No entry points (expected) |
| `cx doctor` | PASS | All checks passed |

### Notes
- Excellent extraction of interfaces, classes, methods
- Generic types properly handled (Repository<T>)
- Call graph dependencies correctly resolved

---

## 5. Rust Language Support - PASS (with caveats)

Tested with simple and complex Rust files.

### Simple Test (Working)
```rust
fn main() { ... }
pub fn add(x: i64, y: i64) -> i64 { x + y }
#[test] fn test_add() { ... }
```

### Complex Test (Entity Persistence Issue)
```rust
pub struct User { id: u32, name: String, email: String }
impl User {
    pub fn new(id: u32, name: String, email: String) -> Self { ... }
    pub fn display(&self) -> String { ... }
}
pub trait UserRepository { ... }
pub struct MemoryUserRepo { users: HashMap<u32, User> }
impl MemoryUserRepo { pub fn new() -> Self { ... } }
impl UserRepository for MemoryUserRepo { ... }
```

### Commands Tested (Simple File)

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang rust` | PASS | 5 entities extracted |
| `cx db info` | PASS | Shows correct counts |
| `cx find main` | PASS | Found function |
| `cx show main --density dense` | PASS | Full details with hashes |
| `cx show main --related` | PASS | Shows neighborhood |
| `cx map` | PASS | Shows project skeleton |
| `cx safe main.rs --quick` | PASS | Blast radius analysis |
| `cx find --important --top 10` | PASS | Returns entities with metrics |

### Known Issue: Complex Rust Files
When a Rust file contains multiple impl blocks with duplicate method names (e.g., multiple `new()` methods), entities appear to be created during scan but show 0 active entities afterwards. Dependencies are persisted (16) but entities are not (0).

**Workaround**: Use simpler Rust files or rename duplicate methods

---

## 6. Python Language Support - PASS (with caveats)

Tested with simple and complex Python files.

### Simple Test (Working)
```python
def add(a: int, b: int) -> int:
    return a + b

class Calculator:
    def __init__(self): self.result = 0
    def compute(self, a: int, b: int) -> int: ...
```

### Complex Test (Entity Persistence Issue)
Similar to Rust - files with `@dataclass` decorators and duplicate method patterns may not persist entities correctly.

### Commands Tested (Simple File)

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang python` | PASS | 6 entities, 3 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx find add` | PASS | Found function with caller info |
| `cx find Calculator` | PASS | Found class |
| `cx show Calculator --density dense` | PASS | Full details |
| `cx show compute --related` | PASS | Shows neighborhood with calls |
| `cx show compute --graph --hops 2` | PASS | Shows dependency graph |
| `cx map` | PASS | Shows project skeleton |
| `cx safe simple.py --quick` | PASS | Blast radius analysis |
| `cx context --smart "add error handling"` | PASS | No entry points (expected) |
| `cx doctor` | PASS | All checks passed |
| `cx find --format json add` | PASS | JSON output |

### Notes
- Python type annotations correctly extracted in signatures
- Class methods properly associated with their classes
- Call graph dependencies properly resolved

---

## 7. Daemon & Live Mode - PASS

### cx status
- **Status**: PASS
- **Output**: Shows daemon status, graph freshness, entity count, database path

### cx live
- **Status**: PASS
- **Features**: --help, --list-tools, --status all work

---

## 8. Smart Context & Safe Commands - PASS

### cx safe (full assessment)
- **Status**: PASS
- **Output**: risk_level, impact_radius, files_affected, keystone_count, coverage_gaps, drift_detected
- **Warnings**: Properly lists drifted entities and coverage gaps
- **Recommendations**: Provides actionable guidance

### cx safe --quick
- **Status**: PASS
- **Output**: Impact analysis only

### cx safe --drift
- **Status**: PASS
- **Output**: Verification status

### cx safe --changes
- **Status**: PASS
- **Output**: Changes since last scan

---

## 9. Entity Tagging - PASS

### cx tag <entity> <tags...>
- **Status**: PASS
- **Notes**: Multiple tags supported, -n for notes

### cx tags (list)
- **Status**: PASS

### cx find --tag
- **Status**: PASS
- **Features**: Single tag, multiple tags with AND/OR

### cx tags export/import
- **Status**: PASS

---

## 10. Call Chain Tracer - PASS

### cx trace <from> <to>
- **Status**: PASS
- **Output**: Path between entities with depth

### cx trace --callers/--callees
- **Status**: PASS

---

## 11. Dead Code Detection - PASS

### cx dead
- **Status**: PASS
- **Output**: Found 853 dead code entities in Go codebase
- **Breakdown**: 586 constants, 24 functions, 241 methods, 2 structs

---

## Issues Found

### Critical
None

### Major
1. **Entity Persistence with Duplicate Names (Rust/Python)**
   - When files contain duplicate method names across impl blocks/classes
   - Entities show in scan output but db info shows 0 active
   - Dependencies ARE persisted, entities are NOT
   - Affects: Complex Rust with multiple impl blocks, Python with decorators

### Minor
1. **File path scanning outside project root**
   - Files get "skipped" when scanning paths outside the CX project root
   - Workaround: Copy files into project or cd to the target directory

2. **--lang flag sometimes required**
   - Auto-detection may skip some files
   - Workaround: Use `--lang <language>` flag

3. **cx find requires query or ranking flag**
   - `cx find --type F --lang go` errors without query
   - Use `cx find --important --type F --lang go` instead

---

## Recommendations

1. **Fix entity ID collision** for duplicate method names in Rust impl blocks
2. **Improve auto-detection** to not skip files based on patterns
3. **Add warning** when entities are created but not persisted
4. **Consider --all flag** for cx find with type/lang filters

---

## Test Commands Quick Reference

```bash
# Scan codebase
cx scan                          # Auto-detect language
cx scan --lang <lang>            # Explicit language
cx scan --force                  # Force rescan

# Database health
cx db info                       # Database statistics
cx doctor                        # Health check
cx status                        # Daemon and graph status

# Find entities
cx find <name>                   # Name search
cx find <name> --exact           # Exact match
cx find --important --top 10     # Top by PageRank
cx find --keystones              # Keystone entities
cx find --tag <tag>              # By tag

# Show entity details
cx show <name>                   # Default details
cx show <name> --density dense   # Full details
cx show <name> --related         # Neighborhood
cx show <name> --graph           # Dependencies

# Safety analysis
cx safe <file>                   # Full assessment
cx safe <file> --quick           # Blast radius only
cx safe --changes                # Changes since scan
cx safe --drift                  # Staleness check

# Context
cx context                       # Session recovery
cx context --smart "<task>"      # Intent-aware context

# Other
cx map                           # Project skeleton
cx trace <from> <to>             # Call path
cx dead                          # Dead code
cx tag <entity> <tags>           # Add tags
cx tags <entity>                 # List tags
```

---

## 12. C Language Support - PASS

Tested with c-learning repository.

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --force` | PASS | 54 entities, 9 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx doctor` | PASS | All checks passed |
| `cx find main` | PASS | Found 28 main functions |
| `cx find --important --top 10` | PASS | Returns top entities |
| `cx find --keystones --top 5` | PASS | Found 3 keystones |
| `cx show data` | PASS | Shows struct details |
| `cx show data --density dense` | PASS | Full details with metrics |
| `cx show data --related` | PASS | Shows neighborhood |
| `cx show data --graph --hops 2` | PASS | Shows dependency graph |
| `cx map` | PASS | Project skeleton |
| `cx map --filter F` | PASS | Functions only |
| `cx safe <file>` | PASS | Full safety assessment |
| `cx safe <file> --quick` | PASS | Blast radius only |
| `cx context` | PASS | Session recovery |
| `cx context --smart "struct data"` | PASS | Smart context |
| `cx find --format json` | PASS | JSON output |
| `cx find --format jsonl` | PASS | JSONL output |
| `cx tag add <entity> <tag>` | PASS | Entity tagging |
| `cx tag find <tag>` | PASS | Find by tag |
| `cx dead` | PASS | Dead code detection |

---

## 13. C++ Language Support - PASS

Tested with cpp-cmake-template repository.

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --force` | PASS | 17 entities, 31 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx doctor` | PASS | All checks passed |
| `cx find Division` | PASS | Found class and related entities |
| `cx find --important --top 10` | PASS | Returns top entities with metrics |
| `cx find --keystones --top 5` | PASS | Found Fraction as keystone |
| `cx show Division --density dense` | PASS | Full details |
| `cx show Division --related --depth 2` | PASS | Neighborhood |
| `cx show Division --graph --hops 2` | PASS | Dependency graph |
| `cx map` | PASS | Project skeleton |
| `cx safe <file>` | PASS | Full safety assessment |
| `cx safe <file> --quick` | PASS | Blast radius only |
| `cx context --smart "divide fractions"` | PASS | Smart context |
| `cx find --format json` | PASS | JSON output |
| `cx dead` | PASS | No dead code found |
| `cx trace "Division::divide" DivisionByZero` | PASS | Call chain trace |

---

## 14. C# Language Support - PASS

Tested with csharp-beginner repository (24 files, 189 entities).

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --force` | PASS | 189 entities, 443 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx doctor` | PASS | All checks passed |
| `cx find Calculator` | PASS | Found Calculator class |
| `cx find --important --top 10` | PASS | Returns top entities |
| `cx show TextEditor` | PASS | Shows class details |
| `cx show TextEditor --density dense` | PASS | Full details |
| `cx show TextEditor --related` | PASS | Neighborhood |
| `cx show TextEditor --graph --hops 2` | PASS | Dependency graph |
| `cx map` | PASS | Project skeleton |
| `cx safe <file>` | PASS | Full safety assessment |
| `cx context --smart "add expense"` | PASS | Smart context |
| `cx find --format json` | PASS | JSON output |
| `cx dead` | PASS | 49 dead code entities |

---

## 15. PHP Language Support - PASS

Tested with php-laravel-quickstart repository.

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --force` | PASS | 69 entities, 54 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx doctor` | PASS | All checks passed |
| `cx find --important --top 10` | PASS | Found create method as keystone |
| `cx find --keystones --top 5` | PASS | 1 keystone (create) |
| `cx find Task` | PASS | Found Task model |
| `cx show AuthController` | PASS | Shows controller details |
| `cx show AuthController --density dense` | PASS | Full details |
| `cx show AuthController --related` | PASS | Neighborhood |
| `cx show create --graph --hops 2` | PASS | Dependency graph |
| `cx map` | PASS | Project skeleton |
| `cx safe <file>` | PASS | Full safety assessment |
| `cx context --smart "user authentication"` | PASS | Smart context |
| `cx find --format json` | PASS | JSON output |
| `cx dead` | PASS | 17 dead code entities |

---

## 16. Kotlin Language Support - PASS (with issues)

Tested with kotlin-android-practice repository.

### Critical Issue: Auto-detection Skip Bug

**Problem**: Without `--lang kotlin` flag, scan skips all 112 Kotlin files even though Kotlin is a supported language. Additionally, without `--force` flag, even with `--lang kotlin`, files are skipped.

**Workaround**: Always use `cx scan --lang kotlin --force` for Kotlin projects.

### Commands Tested (with --lang kotlin --force)

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang kotlin --force` | PASS | 1260 entities, 776 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx doctor` | PASS | All checks passed |
| `cx find --important --top 10` | PASS | Returns top entities with metrics |
| `cx find MainActivity` | PASS | Found 13 MainActivity classes |
| `cx show MainActivity@<path>` | PASS | Disambiguation with path hint |
| `cx map` | PASS | Project skeleton |
| `cx safe <file> --quick` | PASS | Blast radius only |
| `cx context --smart "android view model"` | PASS | Smart context |

---

## 17. Ruby Language Support - PASS

Tested with ruby-learn-rails repository.

### Commands Tested

| Command | Status | Notes |
|---------|--------|-------|
| `cx scan --lang ruby --force` | PASS | 28 entities, 27 dependencies |
| `cx db info` | PASS | Shows correct counts |
| `cx doctor` | PASS | All checks passed |
| `cx find --important --top 10` | PASS | Returns top entities |
| `cx find Application` | PASS | Found 5 Application* entities |
| `cx show Visitor` | PASS | Shows model details |
| `cx show Visitor --density dense` | PASS | Full details |
| `cx show Visitor --related` | PASS | Neighborhood |
| `cx show Visitor --graph --hops 2` | PASS | Dependency graph |
| `cx map` | PASS | Project skeleton |
| `cx safe <file>` | PASS | Full safety assessment |
| `cx context --smart "visitor model"` | PASS | Smart context |
| `cx find --format json` | PASS | JSON output |
| `cx dead` | PASS | No dead code found |

---

## Additional Issues Found (C, C++, C#, PHP, Kotlin, Ruby)

### Critical

1. **Kotlin/Ruby Auto-detection Bypass**
   - **Severity**: High
   - **Symptom**: `cx scan` without `--lang` flag skips Kotlin/Ruby files
   - **Root Cause**: Multi-language auto-detection only scans first detected language
   - **Workaround**: Use `--lang kotlin` or `--lang ruby` explicitly
   - **File**: internal/cmd/scan.go, lines 126-145

### Major

2. **Kotlin Requires --force Flag**
   - **Severity**: Medium
   - **Symptom**: Without `--force`, all Kotlin files skipped even with `--lang kotlin`
   - **Workaround**: Always use `--force` for initial Kotlin scans

### Minor

3. **cx find --type/--lang Requires Query**
   - Already documented above

4. **Tag add with ambiguous entity names fails**
   - Requires disambiguation for entities with same name

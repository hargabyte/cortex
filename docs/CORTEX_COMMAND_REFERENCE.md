# Cortex (cx) Command Reference - Comprehensive Testing Results

This document provides a comprehensive reference for all Cortex commands, their flags, output fields, and expected behavior. Created through extensive testing with Go code to ensure all languages produce equivalent context quality.

---

## Table of Contents

1. [Global Flags](#global-flags)
2. [cx context](#cx-context)
3. [cx find](#cx-find)
4. [cx show](#cx-show)
5. [cx safe](#cx-safe)
6. [cx map](#cx-map)
7. [cx test](#cx-test)
8. [cx scan](#cx-scan)
9. [cx guard](#cx-guard)
10. [cx db](#cx-db)
11. [cx link](#cx-link)
12. [cx serve](#cx-serve)
13. [Cross-Language Verification Checklist](#cross-language-verification-checklist)

---

## Global Flags

These flags are available on all commands and control output format and verbosity.

### `--format <format>`

Controls the output format. Default: `yaml`

| Format | Description | Token Efficiency | Use Case |
|--------|-------------|-----------------|----------|
| `yaml` | Human-readable YAML | Medium | Default for human consumption |
| `json` | Standard JSON | Medium | Programmatic parsing |
| `jsonl` | JSON Lines (one entity per line) | High | Streaming/batch processing |

**Example Output - yaml:**
```yaml
results:
  Execute:
    type: function
    location: internal/cmd/root.go:78-83
    signature: ()
    visibility: public
```

**Example Output - json:**
```json
{
  "results": {
    "Execute": {
      "type": "function",
      "location": "internal/cmd/root.go:78-83"
    }
  }
}
```

**Example Output - jsonl:**
```json
{"entity":{"type":"function","location":"internal/cmd/root.go:78-83"},"name":"Execute"}
```

### `--density <level>`

Controls the detail level of output. Default: `medium`

| Level | Tokens/Entity | Fields Included |
|-------|---------------|-----------------|
| `sparse` | 50-100 | type, location |
| `medium` | 200-300 | + signature, visibility, basic dependencies |
| `dense` | 400-600 | + metrics (pagerank, in_degree, out_degree), hashes |
| `smart` | Variable | Adaptive based on entity importance |

**Sparse Output:**
```yaml
Execute:
  type: function
  location: internal/cmd/root.go:78-83
```

**Medium Output:**
```yaml
Execute:
  type: function
  location: internal/cmd/root.go:78-83
  signature: ()
  visibility: public
  dependencies:
    calls:
      - sa-fn-4a72a1-78-Execute
    called_by:
      - name: sa-fn-4a72a1-78-Execute
```

**Dense Output:**
```yaml
Execute:
  type: function
  location: internal/cmd/root.go:78-83
  signature: ()
  visibility: public
  dependencies:
    calls:
      - sa-fn-4a72a1-78-Execute
    called_by:
      - name: sa-fn-4a72a1-78-Execute
        location: function @ internal/cmd/root.go:78-83
  metrics:
    pagerank: 0.0016816652265865344
    in_degree: 2
    out_degree: 2
    importance: leaf
  hashes:
    signature: 0d34d7f1
    body: 016d7c47
```

### `--verbose` / `-v`

Enable verbose output for debugging.

### `--quiet` / `-q`

Suppress output (exit code only). Useful for scripting.

---

## cx context

Session recovery and context assembly for AI agents.

### Modes

1. **Session Recovery (no args)**: Quick workflow context
2. **Full Session Recovery (--full)**: Extended with keystones and map
3. **Smart Context (--smart)**: Intent-aware context assembly
4. **Entity/File Context (target)**: Focused context for specific target

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--smart <task>` | string | - | Natural language task for intent-aware assembly |
| `--budget` / `--max-tokens` | int | 4000 | Token budget |
| `--hops` | int | 1 | Graph expansion depth |
| `--depth` | int | 2 | Max hops from entry points (--smart mode) |
| `--budget-mode` | string | importance | Budget mode: `importance` or `distance` |
| `--include` | strings | - | What to expand: `deps`, `callers`, `types` |
| `--exclude` | strings | tests,mocks | What to skip: `tests`, `mocks` |
| `--with-coverage` | bool | false | Include test coverage data |
| `--full` | bool | false | Extended session recovery |

### Output Fields

**Session Recovery Mode:**
- Markdown formatted workflow guide
- Database statistics (entities, dependencies, files)
- Essential command reference
- Quick patterns

**Smart Context Mode:**
```yaml
intent:
  keywords: [keyword1, keyword2]
  pattern: modify|add_feature|fix|etc
entry_points:
  EntityName:
    type: function|type|method
    location: file:line
    note: "Match reason"
relevant_entities:
  EntityName:
    type: function|type|method
    location: file:line
    relevance: high|medium|low
    reason: "Why included"
```

### Relevance Levels

| Level | Meaning |
|-------|---------|
| `high` | Direct dependencies, keystones, entry points |
| `medium` | Transitive dependencies |
| `low` | Distant or weak relationships |

### Usage Examples

```bash
# Session recovery
cx context

# Extended session recovery
cx context --full

# Smart context for task
cx context --smart "add rate limiting to API" --budget 8000

# File context
cx context internal/parser/parser.go --hops 2

# With coverage data
cx context --smart "fix auth bug" --with-coverage
```

---

## cx find

Unified entity lookup supporting name search, concept search, and ranking.

### Query Modes (Auto-detected)

| Input | Mode | Description |
|-------|------|-------------|
| Single word | Name search | Prefix match across all packages |
| Multi-word / quoted | Concept/FTS search | Full-text search on names, code, docs |
| No query + ranking flag | Ranked results | Top entities by importance |

### Name Resolution Formats

| Format | Example | Description |
|--------|---------|-------------|
| Simple name | `LoginUser` | Prefix match |
| Qualified name | `auth.LoginUser` | package.symbol |
| Path-qualified | `auth/login.LoginUser` | path/file.symbol |
| Direct ID | `sa-fn-a7f9b2-LoginUser` | Full entity ID |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | - | Filter: F=function, T=type, M=method, C=constant, E=enum |
| `--exact` | bool | false | Exact match only (no prefix) |
| `--lang` | string | - | Filter by language: go, typescript, python, rust, java |
| `--file` | string | - | Filter by file path |
| `--limit` | int | 100 | Maximum results |
| `--important` | bool | false | Sort by PageRank importance |
| `--keystones` | bool | false | Only keystone entities |
| `--bottlenecks` | bool | false | Only bottleneck entities |
| `--top` | int | 20 | Number of results for ranking flags |
| `--qualified` | bool | false | Show qualified names in output |
| `--recompute` | bool | false | Force recompute metrics |

### Output Fields

```yaml
results:
  EntityName:
    type: function|type|method|constant|import
    location: file:line-line
    signature: "(params) -> return"
    visibility: public|private
    dependencies:
      calls: [entity_ids]
      called_by: [{name: entity_id}]
    metrics:  # Only with --important or dense density
      pagerank: float
      in_degree: int
      out_degree: int
      importance: keystone|bottleneck|normal|leaf
      betweenness: float
    hashes:  # Only with dense density
      signature: hash
      body: hash
count: int
```

### Usage Examples

```bash
# Name search
cx find LoginUser

# Exact match
cx find Store --exact

# Qualified name
cx find store.Store

# Type filter
cx find Extract --type F --limit 5

# Concept/FTS search
cx find "entity extraction" --limit 5

# Important entities
cx find --important --top 10

# Keystones (critical entities)
cx find --keystones

# Combined filters
cx find --lang go --important Entity --top 5
```

---

## cx show

Display detailed information about a single entity.

### Target Resolution

| Format | Example | Description |
|--------|---------|-------------|
| Name | `LoginUser` | Exact match preferred, then prefix |
| Qualified | `auth.LoginUser` | package.symbol |
| Path-qualified | `auth/login.LoginUser` | path/file.symbol |
| Entity ID | `sa-fn-a7f9b2-LoginUser` | Direct lookup |
| File:line | `internal/auth/login.go:45` | Entity at specific line |
| Name@path | `nodeText@internal/extract/python.go` | Name with file hint |

### Modes

1. **Basic (default)**: Entity details
2. **Related (--related)**: Entity + neighborhood
3. **Graph (--graph)**: Dependency graph visualization

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--related` | bool | false | Show neighborhood (calls, callers, same-file) |
| `--depth` | int | 1 | Hop count for neighborhood traversal |
| `--graph` | bool | false | Show dependency graph |
| `--hops` | int | 2 | Traversal depth for graph |
| `--direction` | string | both | Edge direction: `in`, `out`, `both` |
| `--type` | string | all | Edge types: `calls`, `uses_type`, `implements`, `all` |
| `--coverage` | bool | false | Include test coverage information |
| `--include-metrics` | bool | false | Add importance scores to any density |

### Output Fields

**Basic Mode:**
```yaml
EntityName:
  type: function|struct|interface|method|constant
  location: file:line-line
  signature: "(params) -> return"
  visibility: public|private
  dependencies:
    calls: [entity_ids]
    called_by: [{name, location}]
    uses_types: [entity_ids]
  metrics:  # Only with --include-metrics or dense
    pagerank: float
    in_degree: int
    out_degree: int
    importance: string
  hashes:  # Only with dense
    signature: hash
    body: hash
```

**Related Mode (--related):**
```yaml
center:
  name: EntityName
  type: function
  location: file:line
neighborhood:
  same_file:
    - name: OtherEntity
      type: function
      location: file:line
  calls:
    - name: CalledEntity
  callers:
    - name: CallerEntity
```

**Graph Mode (--graph):**
```yaml
graph:
  root: EntityName
  direction: both|in|out
  depth: int
nodes:
  EntityName:
    type: function
    location: file:line
    depth: int
    signature: string
edges:
  - [from_entity, to_entity, edge_type]
```

### Disambiguation

When multiple entities match, cx shows options with PageRank scores:
```
Error: multiple entities match "nodeText":
  - nodeText (method) at internal/extract/python.go:1034-1036 [pr=0.016]
  - NodeText (method) at internal/parser/parser.go:209-214 [pr=0.010]
Use a more specific name, full entity ID, or file hint: name@path
```

### Usage Examples

```bash
# Basic entity details
cx show Execute

# Sparse output
cx show Execute --density sparse

# With metrics
cx show Execute --include-metrics

# Full details
cx show Execute --density dense

# Neighborhood
cx show Execute --related

# Deep neighborhood
cx show Execute --related --depth 2

# Dependency graph
cx show Execute --graph --hops 3

# Incoming edges only
cx show Execute --graph --direction in

# Calls edges only
cx show Execute --graph --type calls

# With file hint for disambiguation
cx show 'nodeText@internal/extract/python.go'

# Entity at specific line
cx show 'internal/parser/parser.go:209'
```

---

## cx safe

Comprehensive safety assessment before modifying code.

### Modes

| Mode | Flag | Description |
|------|------|-------------|
| Full | (default) | Impact + coverage + drift |
| Quick | `--quick` | Just blast radius |
| Coverage | `--coverage` | Coverage gaps only |
| Drift | `--drift` | Staleness check only |
| Changes | `--changes` | What changed since scan |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--quick` | bool | false | Just impact analysis |
| `--coverage` | bool | false | Coverage gaps mode |
| `--drift` | bool | false | Staleness check mode |
| `--changes` | bool | false | Changes since scan mode |
| `--depth` | int | 3 | Transitive impact depth |
| `--keystones-only` | bool | false | Only keystones (--coverage mode) |
| `--threshold` | int | 75 | Coverage threshold % |
| `--strict` | bool | false | Exit non-zero on drift (CI) |
| `--fix` | bool | false | Update hashes (--drift mode) |
| `--detailed` | bool | false | Show hash changes (--changes mode) |
| `--semantic` | bool | false | Show semantic analysis (--changes mode) |
| `--create-task` | bool | false | Create beads task for findings |

### Risk Levels

| Level | Meaning |
|-------|---------|
| `critical` | Multiple undertested keystones, or drift detected |
| `high` | Keystones affected with coverage gaps |
| `medium` | Multiple entities affected, adequate coverage |
| `low` | Isolated changes with good test coverage |

### Output Fields

**Full Assessment:**
```yaml
safety_assessment:
  target: file-or-entity
  risk_level: critical|high|medium|low
  impact_radius: int
  files_affected: int
  keystone_count: int
  coverage_gaps: int
  drift_detected: bool
  drifted_count: int
warnings:
  - "Warning message"
recommendations:
  - "Action to take"
affected_keystones:
  - name: EntityName
    type: method
    location: file:line
    pagerank: float
    coverage: percentage|unknown
    impact: direct|indirect|caller
    coverage_gap: bool
```

**Quick Mode (--quick):**
```yaml
impact:
  target: file-or-entity
  depth: int
summary:
  files_affected: int
  entities_affected: int
  risk_level: low|medium|high
affected:
  EntityName:
    type: function
    location: file:line
    impact: direct|indirect|caller
    importance: keystone|bottleneck|normal|leaf
    reason: "Why affected"
```

**Drift Mode (--drift):**
```yaml
verification:
  status: passed|failed
  entities_checked: int
  valid: int
  drifted: int
  missing: int
  issues:
    - entity: EntityName
      type: drifted|missing
      location: file:line
      reason: sig_hash changed
      expected: hash
      actual: hash
      hash_type: signature|body
  actions:
    - "Suggested action"
```

**Changes Mode (--changes):**
```yaml
summary:
  files_changed: int
  entities_added: int
  entities_modified: int
  entities_removed: int
  last_scan: timestamp
added:
  - name: filename
    type: file
    location: path
    change: new_file
modified: []
removed: []
```

### Usage Examples

```bash
# Full safety assessment
cx safe internal/parser/parser.go

# Quick blast radius
cx safe internal/parser/parser.go --quick

# Deeper transitive analysis
cx safe internal/parser/parser.go --depth 5

# Coverage gaps (requires coverage data)
cx safe --coverage

# Only keystone coverage gaps
cx safe --coverage --keystones-only

# Check for drift
cx safe --drift

# CI mode - fail on drift
cx safe --drift --strict

# What changed since scan
cx safe --changes

# Detailed hash changes
cx safe --changes --detailed
```

---

## cx map

Project structure overview (skeleton view).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--filter` | string | - | F=functions, T=types, M=methods, C=constants |
| `--lang` | string | - | Filter by language |
| `--depth` | int | 0 | Nested type expansion depth (0=no limit) |

### Output Fields

```yaml
files:
  path/to/file.go:
    entities:
      EntityName:
        type: function|type|variable|constant|import
        location: file:line-line
        skeleton: |
          // Doc comment
          func EntityName(params) -> return { ... }
        doc_comment: "// Doc comment"
        visibility: public|private
count: int
```

### Key Information per Entity

| Field | Description |
|-------|-------------|
| `type` | Entity type (function, type, variable, etc.) |
| `location` | File and line range |
| `skeleton` | Function/type signature with body placeholder |
| `doc_comment` | Documentation comment if present |
| `visibility` | public or private |

### Usage Examples

```bash
# Full project map
cx map

# Specific directory
cx map internal/store

# Functions only
cx map --filter F

# Types only
cx map --filter T

# Go files only
cx map --lang go

# Combine directory and filter
cx map internal/parser --lang go
```

---

## cx test

Smart test selection and coverage analysis.

### Modes

1. **Test Selection (default)**: Find tests affected by changes
2. **Coverage Gaps (--gaps)**: Show coverage gaps weighted by importance
3. **Coverage Summary (--coverage)**: Overall coverage statistics

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--diff` | bool | true | Use git diff HEAD to find changed files |
| `--commit` | string | - | Use specific commit |
| `--file` | string | - | Specify file directly |
| `--depth` | int | 2 | Indirect test discovery depth |
| `--run` | bool | false | Actually run the selected tests |
| `--output-command` | bool | false | Output go test command only |
| `--gaps` | bool | false | Show coverage gaps |
| `--coverage` | bool | false | Show coverage summary |
| `--keystones-only` | bool | false | With --gaps: only keystones |
| `--threshold` | int | 75 | Coverage threshold % |

### Output Fields

**Test Selection:**
```yaml
affected_tests:
  TestFunctionName:
    path: file_test.go
    reason: "Why this test is affected"
summary:
  total_tests: int
  affected_tests: int
  time_saved: string
command: "go test command"
```

### Subcommands

#### `cx test coverage import`

Import coverage data from Go coverage files.

**Supported Formats:**
1. `coverage.out` file (aggregate)
2. Per-test coverage directory (RECOMMENDED)
3. GOCOVERDIR directory

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--base-path` | string | . | Base path for file normalization |

### Usage Examples

```bash
# Tests for uncommitted changes
cx test

# Tests for specific file
cx test internal/parser/parser.go

# Tests for specific commit
cx test --commit HEAD~1

# Run affected tests
cx test --diff --run

# Output test command only
cx test --output-command

# Import coverage
cx test coverage import coverage.out

# Import per-test coverage (recommended)
cx test coverage import .coverage/

# Show coverage gaps
cx test --gaps

# Only keystone gaps
cx test --gaps --keystones-only
```

---

## cx scan

Build or update the code graph.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | false | Rescan even if files unchanged |
| `--overview` | bool | false | Show project overview after scan |
| `--lang` | string | - | Scan only specific language |
| `--exclude` | strings | - | Comma-separated globs to exclude |
| `--dry-run` | bool | false | Show what would be scanned |

### Output Fields

```
#cgf v1 d=sparse

; Scanned X files, Y entities
; Created: A, Updated: B, Unchanged: C, Archived: D
; Skipped: E, Errors: F
```

### Supported Languages

- Go
- TypeScript
- JavaScript
- Java
- Rust
- Python
- C
- C#
- PHP

### Usage Examples

```bash
# Normal scan (auto-init if needed)
cx scan

# Force full rescan
cx scan --force

# Scan with overview
cx scan --overview

# Specific directory
cx scan internal/parser

# Exclude patterns
cx scan --exclude "vendor/*,*_test.go"

# Preview what would be scanned
cx scan --dry-run
```

---

## cx guard

Pre-commit hook for quality checks.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--staged` | bool | true | Check only staged files |
| `--all` | bool | false | Check all modified files |
| `--fail-on-warnings` | bool | false | Exit with error on warnings |
| `--min-coverage` | float | 50 | Min coverage threshold for keystones |

### Checks Performed

1. **Coverage Regression**: Did coverage decrease for modified keystones?
2. **New Untested Code**: Are there new entities with 0% coverage?
3. **Breaking Changes**: Signature changes with unchecked callers?
4. **Graph Drift**: Is the cx database out of sync?

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Pass (no errors, warnings allowed) |
| 1 | Warnings only (pass by default, fail if --fail-on-warnings) |
| 2 | Errors (always fails) |

### Output Fields

```yaml
summary:
  files_checked: int
  entities_affected: int
  error_count: int
  warning_count: int
  drift_detected: bool
  coverage_issues: int
  signature_changes: int
  pass_status: pass|warnings|fail
warnings:
  - type: untested_code|coverage_regression|signature_change
    entity: EntityName
    file: path
    message: "Description"
    suggestion: "Action"
errors: []
```

### Installation as Git Hook

```bash
echo 'cx guard --staged' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

### Usage Examples

```bash
# Check staged changes
cx guard

# Check all modified
cx guard --all

# Strict mode
cx guard --fail-on-warnings
```

---

## cx db

Database management commands.

### Subcommands

#### `cx db info`

Show database statistics.

**Output:**
```
Database: /path/to/.cx/cortex.db
Size: X.X MB

Entities:     XXXX (active: XXX, archived: XXXX)
Dependencies: XXXXX
Files indexed: XXX
External links: X
```

#### `cx db doctor`

Check database health.

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--fix` | bool | false | Auto-fix issues |
| `--deep` | bool | false | Full graph validation |
| `--yes` | bool | false | Skip confirmation |

**Checks:**
- Database integrity
- Orphan dependencies
- Stale entities
- Entity archive ratio

#### `cx db compact`

Reclaim unused database space.

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--remove-archived` | bool | false | Remove archived entities first |
| `--dry-run` | bool | false | Preview only |

#### `cx db export`

Export database to JSONL.

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o, --output` | string | stdout | Output file |

### Usage Examples

```bash
# Database statistics
cx db info

# Health check
cx db doctor

# Auto-fix issues
cx db doctor --fix

# Compact database
cx db compact

# Preview compact
cx db compact --dry-run

# Export to file
cx db export -o backup.jsonl
```

---

## cx reset

Reset database to clean state.

### Modes

| Mode | Description |
|------|-------------|
| Full (default) | Clear everything with backup |
| Scan-only | Clear file index only, keep entities |
| Hard | Delete database file entirely |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--scan-only` | bool | false | Only clear file index |
| `--hard` | bool | false | Delete database file (requires --force) |
| `--force` | bool | false | Skip confirmation |
| `--no-backup` | bool | false | Skip backup before reset |
| `--dry-run` | bool | false | Preview only |

### Usage Examples

```bash
# Safe reset with backup
cx reset

# Preview reset
cx reset --dry-run

# Clear file index only
cx reset --scan-only

# Hard reset (delete database)
cx reset --hard --force
```

---

## cx link

Manage links between code entities and external systems.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--system` | string | beads | External system: beads, github, jira |
| `--type` | string | related | Link type: related, implements, fixes, discovered-from |
| `--list` | bool | false | List links for entity |
| `--remove` | bool | false | Remove a link |

### Usage Examples

```bash
# Create link
cx link sa-fn-abc123-Login bd-task-456

# Link to GitHub issue
cx link sa-fn-abc123-Login issue-789 --system github

# List entity links
cx link --list sa-fn-abc123-Login

# Remove link
cx link --remove sa-fn-abc123-Login bd-task-456
```

---

## cx serve

MCP (Model Context Protocol) server for AI agent integration.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mcp` | bool | false | Start MCP server (stdio transport) |
| `--tools` | string | diff,impact,context,show | Tools to expose |
| `--timeout` | string | 30m | Inactivity timeout (0 for none) |
| `--status` | bool | false | Check server status |
| `--stop` | bool | false | Stop running server |
| `--list-tools` | bool | false | Show available tools |

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `cx_diff` | Show changes since last scan |
| `cx_impact` | Analyze blast radius of changes |
| `cx_context` | Smart context assembly |
| `cx_show` | Entity details |
| `cx_find` | Search entities |
| `cx_gaps` | Coverage gap analysis |

### Usage Examples

```bash
# List available tools
cx serve --list-tools

# Check status
cx serve --status

# Start MCP server
cx serve --mcp

# Start with specific tools
cx serve --mcp --tools diff,impact

# Custom timeout
cx serve --mcp --timeout 60m

# Stop server
cx serve --stop
```

---

## Cross-Language Verification Checklist

When testing language support, verify each command produces equivalent quality output. Use this checklist to ensure feature parity.

### Entity Types by Language

| Entity Type | Go | TypeScript | Python | Java | Rust | C | C# | PHP |
|-------------|----|-----------:|:------:|:----:|:----:|:-:|:--:|:---:|
| Functions | `function_definition` | `function_declaration` | `function_definition` | `method_declaration` | `function_item` | `function_definition` | `method_declaration` | `function_definition` |
| Classes | - | `class_declaration` | `class_definition` | `class_declaration` | - | - | `class_declaration` | `class_declaration` |
| Structs | `type_declaration` | - | - | - | `struct_item` | `struct_specifier` | `struct_declaration` | - |
| Interfaces | `interface_type` | `interface_declaration` | - | `interface_declaration` | `trait_item` | - | `interface_declaration` | `interface_declaration` |
| Methods | `method_declaration` | `method_definition` | `function_definition` | `method_declaration` | `impl_item` | - | `method_declaration` | `method_declaration` |
| Constants | `const_declaration` | `const` | `assignment` | `field_declaration` | `const_item` | `preproc_def` | `const_declaration` | `const_declaration` |
| Imports | `import_spec` | `import_statement` | `import_statement` | `import_declaration` | `use_declaration` | `preproc_include` | `using_directive` | `namespace_use_declaration` |

### Command Verification Matrix

For each supported language, verify:

| Command | Verification |
|---------|--------------|
| `cx find <name>` | Returns entities with correct type, location, signature |
| `cx find --type F <name>` | Filters to functions only |
| `cx find --important` | Returns entities with PageRank scores |
| `cx show <entity>` | Shows dependencies (calls, called_by, uses_types) |
| `cx show <entity> --related` | Shows same_file entities correctly |
| `cx show <entity> --graph` | Builds dependency graph with correct edges |
| `cx context --smart "<task>"` | Identifies relevant entry points and entities |
| `cx safe <file>` | Calculates correct impact radius |
| `cx safe <file> --quick` | Shows affected entities with reasons |
| `cx map` | Extracts signatures and doc comments |
| `cx map --filter F` | Filters to functions only |

### Output Quality Checklist

For each language, verify output contains:

1. **Entity Discovery**
   - [ ] Correct entity types detected
   - [ ] Accurate line ranges
   - [ ] Proper visibility detection (public/private)
   - [ ] Signature extraction with parameters and return types

2. **Dependency Tracking**
   - [ ] Function calls detected
   - [ ] Type references detected
   - [ ] Import/include tracking
   - [ ] Method calls on objects

3. **Graph Analysis**
   - [ ] PageRank computed correctly
   - [ ] In/out degree accurate
   - [ ] Keystones identified
   - [ ] Transitive dependencies traced

4. **Context Assembly**
   - [ ] Smart context finds relevant code
   - [ ] Token budget respected
   - [ ] Entry points correctly identified
   - [ ] Relevance scoring accurate

### Test Commands for New Languages

```bash
# Basic verification
cx scan --lang <newlang>
cx find --lang <newlang> --limit 10

# Entity extraction
cx find --type F --lang <newlang> --limit 5
cx find --type T --lang <newlang> --limit 5
cx find --type M --lang <newlang> --limit 5

# Dependency tracking
cx show <entity> --density dense
cx show <entity> --related
cx show <entity> --graph --hops 2

# Context quality
cx context --smart "implement <feature>" --budget 5000

# Safety assessment
cx safe <file> --quick
cx safe <file>

# Map extraction
cx map --lang <newlang>
cx map --lang <newlang> --filter F
```

---

## Summary

This document covers all Cortex commands with their:
- Complete flag documentation
- Expected output fields and formats
- Usage examples for each mode
- Cross-language verification requirements

When adding support for new languages, use this as a reference to ensure the new language extractors produce output matching the quality and completeness demonstrated with Go.

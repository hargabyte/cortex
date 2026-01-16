# Cortex (cx) Command Reference

Complete reference for all Cortex commands, flags, and output formats. This document is the authoritative source for AI agents using the cx tool.

**Version**: 0.1.0
**Last Updated**: 2026-01-16

---

## Table of Contents

1. [Global Flags](#global-flags)
2. [Session & Context Commands](#session--context-commands)
   - [cx context](#cx-context)
   - [cx status](#cx-status)
3. [Discovery Commands](#discovery-commands)
   - [cx find](#cx-find)
   - [cx show](#cx-show)
   - [cx map](#cx-map)
   - [cx rank](#cx-rank)
   - [cx trace](#cx-trace)
4. [Safety & Analysis Commands](#safety--analysis-commands)
   - [cx safe](#cx-safe)
   - [cx dead](#cx-dead)
   - [cx guard](#cx-guard)
5. [Testing & Coverage Commands](#testing--coverage-commands)
   - [cx test](#cx-test)
   - [cx coverage](#cx-coverage)
6. [Tagging & Organization Commands](#tagging--organization-commands)
   - [cx tag](#cx-tag)
   - [cx untag](#cx-untag)
   - [cx tags](#cx-tags)
7. [Database & Maintenance Commands](#database--maintenance-commands)
   - [cx scan](#cx-scan)
   - [cx db](#cx-db)
   - [cx doctor](#cx-doctor)
   - [cx reset](#cx-reset)
   - [cx link](#cx-link)
8. [Integration Commands](#integration-commands)
   - [cx live](#cx-live)
   - [cx daemon](#cx-daemon)
   - [cx help-agents](#cx-help-agents)

---

## Global Flags

These flags are available on all commands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `yaml` | Output format: `yaml`, `json`, `jsonl`, `cgf` (deprecated) |
| `--density` | string | `medium` | Detail level: `sparse`, `medium`, `dense`, `smart` |
| `-v, --verbose` | bool | false | Enable verbose output |
| `-q, --quiet` | bool | false | Suppress output (exit code only) |
| `--config` | string | `.cx/config.yaml` | Path to config file |

### Density Levels

| Level | Tokens/Entity | Fields Included |
|-------|---------------|-----------------|
| `sparse` | 50-100 | type, location |
| `medium` | 200-300 | + signature, visibility, basic dependencies |
| `dense` | 400-600 | + metrics (pagerank, in_degree, out_degree), hashes, timestamps |
| `smart` | Variable | Adaptive based on entity importance |

### Output Format Examples

**YAML (default):**
```yaml
results:
  Execute:
    type: function
    location: internal/cmd/root.go:78-83
    signature: ()
    visibility: public
```

**JSON:**
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

**JSONL:**
```json
{"entity":{"type":"function","location":"internal/cmd/root.go:78-83"},"name":"Execute"}
```

---

## Session & Context Commands

### cx context

Assemble task-relevant context within a token budget.

**Usage:** `cx context [target] [flags]`

**Modes:**
- No args: Session recovery (workflow context)
- `--smart "<task>"`: Intent-aware context assembly
- `--diff` / `--staged`: Context for code changes
- `<target>`: Entity/file/bead context

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--smart` | string | - | Natural language task description |
| `--budget` | int | 4000 | Token budget (alias for --max-tokens) |
| `--max-tokens` | int | 4000 | Token budget |
| `--hops` | int | 1 | Graph expansion depth |
| `--depth` | int | 2 | Max hops from entry points (--smart mode) |
| `--budget-mode` | string | `importance` | Budget mode: `importance` or `distance` |
| `--include` | strings | - | What to expand: `deps`, `callers`, `types` |
| `--exclude` | strings | `tests,mocks` | What to skip: `tests`, `mocks` |
| `--with-coverage` | bool | false | Include test coverage data |
| `--full` | bool | false | Extended session recovery with keystones |
| `--diff` | bool | false | Context for uncommitted changes |
| `--staged` | bool | false | Context for staged changes only |
| `--commit-range` | string | - | Context for commit range (e.g., `HEAD~3`, `main..`) |
| `--for-task` | string | - | Bead/task ID (requires beads integration) |
| `--density` | string | `medium` | Detail level |

**Examples:**
```bash
cx context                                       # Session recovery
cx context --full                                # Extended session recovery
cx context --smart "add rate limiting" --budget 8000  # Task context
cx context src/auth/login.go --hops 2            # File context
cx context --diff                                # Context for uncommitted changes
cx context --staged                              # Context for staged changes
cx context --smart "task" --with-coverage        # Include coverage data
```

**Output Structure (--smart mode):**
```yaml
intent:
  keywords: [extracted, keywords]
  pattern: add_feature|fix|modify|etc
entry_points:
  FunctionName:
    type: function
    location: file:line
    note: "Why this is relevant"
relevant_entities:
  EntityName:
    relevance: high|medium|low
    reason: "Why included"
```

---

### cx status

Show daemon and graph status.

**Usage:** `cx status [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |
| `--watch` | bool | false | Continuously update status |

**Examples:**
```bash
cx status              # Show status
cx status --json       # JSON output
cx status --watch      # Live updates
```

---

## Discovery Commands

### cx find

Unified entity lookup supporting name search, concept search, and ranking.

**Usage:** `cx find <query> [flags]`

**Query Modes (auto-detected):**
- Single word: Name search (prefix match)
- Multi-word or quoted: Concept/FTS search
- No query + ranking flag: Top entities by importance

**Name Resolution Formats:**
| Format | Example | Description |
|--------|---------|-------------|
| Simple name | `LoginUser` | Prefix match |
| Qualified name | `auth.LoginUser` | package.symbol |
| Path-qualified | `auth/login.LoginUser` | path/file.symbol |
| Direct ID | `sa-fn-a7f9b2-LoginUser` | Full entity ID |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | - | Filter by type: `F`=function, `T`=type, `M`=method, `C`=constant, `E`=enum |
| `--exact` | bool | false | Exact match only (no prefix) |
| `--lang` | string | - | Filter by language: go, typescript, python, rust, java |
| `--file` | string | - | Filter by file path |
| `--limit` | int | 100 | Maximum results |
| `--important` | bool | false | Sort by PageRank importance |
| `--keystones` | bool | false | Only keystone entities (highly depended-on) |
| `--bottlenecks` | bool | false | Only bottleneck entities (central to paths) |
| `--top` | int | 20 | Number of results for ranking flags |
| `--qualified` | bool | false | Show qualified names in output |
| `--recompute` | bool | false | Force recompute metrics |
| `--tag` | stringArray | - | Filter by tag (repeatable, default: match ALL) |
| `--tag-any` | bool | false | Match ANY tag instead of ALL |

**Examples:**
```bash
cx find LoginUser                    # Name search
cx find "auth validation"            # Concept search
cx find auth.LoginUser               # Qualified name
cx find --type F Login               # Functions only
cx find --exact LoginUser            # Exact match
cx find --keystones                  # Top keystones
cx find --important --top 20         # Top by PageRank
cx find --tag important              # Filter by tag
cx find --tag auth --tag api --all   # Multiple tags (AND)
cx find --tag auth --tag api --any   # Multiple tags (OR)
```

---

### cx show

Display detailed information about a single entity.

**Usage:** `cx show <name-or-id-or-file:line> [flags]`

**Target Formats:**
- Simple names: `LoginUser`
- Qualified names: `auth.LoginUser`
- Path-qualified: `auth/login.LoginUser`
- Direct IDs: `sa-fn-a7f9b2-LoginUser`
- File:line: `internal/auth/login.go:45`
- Name@path: `nodeText@internal/extract/python.go`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--related` | bool | false | Show neighborhood (calls, callers, same-file) |
| `--depth` | int | 1 | Hop count for neighborhood (with --related) |
| `--graph` | bool | false | Show dependency graph visualization |
| `--hops` | int | 2 | Traversal depth for graph (with --graph) |
| `--direction` | string | `both` | Edge direction: `in`, `out`, `both` |
| `--type` | string | `all` | Edge types: `calls`, `uses_type`, `implements`, `all` |
| `--coverage` | bool | false | Include test coverage information |
| `--include-metrics` | bool | false | Add importance scores to output |

**Examples:**
```bash
cx show Execute                      # Basic entity details
cx show Execute --density sparse     # Minimal output
cx show Execute --include-metrics    # Add metrics
cx show Execute --related            # Show neighborhood
cx show Execute --related --depth 2  # 2-hop neighborhood
cx show Execute --graph              # Dependency graph
cx show Execute --graph --hops 3     # Deeper graph
cx show Execute --graph --direction in  # Incoming edges only
cx show internal/auth/login.go:45    # Entity at line
cx show 'nodeText@internal/extract/python.go'  # Disambiguate
```

**Output Structure:**
```yaml
EntityName:
  type: function|struct|interface|method|constant
  location: file:line-line
  signature: "(params) -> return"
  visibility: public|private
  tags: [tag1, tag2]
  dependencies:
    calls: [entity_ids]
    called_by: [{name, location}]
    uses_types: [entity_ids]
  metrics:  # with --include-metrics or dense
    pagerank: float
    in_degree: int
    out_degree: int
    importance: keystone|bottleneck|normal|leaf
  coverage:  # with --coverage
    percent: "45.5%"
    tested_by: [TestFunction1, TestFunction2]
```

---

### cx map

Show project structure with function signatures (skeleton view).

**Usage:** `cx map [path] [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--filter` | string | - | Filter by type: `F`=functions, `T`=types, `M`=methods, `C`=constants |
| `--lang` | string | - | Filter by language |
| `--depth` | int | 0 | Nested type expansion depth (0=no limit) |

**Examples:**
```bash
cx map                        # Full project (~10k tokens)
cx map internal/store         # Specific directory
cx map --filter F             # Functions only
cx map --filter T --lang go   # Go types only
```

---

### cx rank

Compute and display importance metrics.

**Usage:** `cx rank [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--keystones` | bool | false | Top N by PageRank (most important) |
| `--bottlenecks` | bool | false | Top N by betweenness (most central) |
| `--leaves` | bool | false | Top N leaf nodes (no dependents) |
| `--top` | int | 20 | Number of results |
| `--recompute` | bool | false | Force recompute all metrics |

**Examples:**
```bash
cx rank                       # Top 20 by PageRank
cx rank --top 50              # Top 50
cx rank --keystones           # Most important entities
cx rank --bottlenecks         # Most central entities
cx rank --leaves              # Leaf nodes
cx rank --recompute           # Force recompute
```

**Output Structure:**
```yaml
results:
  EntityName:
    type: function
    location: file:line
    metrics:
      pagerank: float
      in_degree: int
      out_degree: int
      importance: keystone|bottleneck|normal|leaf
      betweenness: float  # dense only
count: int
```

---

### cx trace

Trace call paths between code entities.

**Usage:** `cx trace <from> [to] [flags]`

**Modes:**
- Path mode: `cx trace <from> <to>` - Show path between entities
- Caller mode: `cx trace <entity> --callers` - What calls this
- Callee mode: `cx trace <entity> --callees` - What this calls

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--callers` | bool | false | Trace upstream callers |
| `--callees` | bool | false | Trace downstream callees |
| `--all` | bool | false | Show all paths (not just shortest) |
| `--depth` | int | 5 | Maximum trace depth |

**Examples:**
```bash
cx trace HandleRequest SaveUser       # Path between entities
cx trace HandleRequest SaveUser --all # All paths
cx trace SaveUser --callers           # What calls SaveUser
cx trace SaveUser --callers --depth 3 # Callers up to 3 hops
cx trace HandleRequest --callees      # What HandleRequest calls
```

---

## Safety & Analysis Commands

### cx safe

Comprehensive safety assessment before modifying code.

**Usage:** `cx safe [file-or-entity] [flags]`

**Modes:**
| Mode | Flag | Description |
|------|------|-------------|
| Full | (default) | Impact + coverage + drift |
| Quick | `--quick` | Just blast radius |
| Coverage | `--coverage` | Coverage gaps only |
| Drift | `--drift` | Staleness check |
| Changes | `--changes` | What changed since scan |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--quick` | bool | false | Quick mode: just impact analysis |
| `--coverage` | bool | false | Coverage gaps mode |
| `--drift` | bool | false | Drift/staleness check mode |
| `--changes` | bool | false | Changes since scan mode |
| `--depth` | int | 3 | Transitive impact depth |
| `--keystones-only` | bool | false | Only keystones (with --coverage) |
| `--threshold` | int | 75 | Coverage threshold % |
| `--strict` | bool | false | Exit non-zero on drift (for CI) |
| `--fix` | bool | false | Update hashes (with --drift) |
| `--dry-run` | bool | false | Show what --fix would do |
| `--detailed` | bool | false | Show hash changes (with --changes) |
| `--semantic` | bool | false | Show semantic analysis (with --changes) |
| `--file` | string | - | Filter changes to specific file/directory |
| `--inline` | bool | false | Inline mode: quick drift check for specific file |
| `--impact-threshold` | float | - | Min importance threshold for impact |
| `--create-task` | bool | false | Create beads task for findings |

**Risk Levels:**
| Level | Meaning |
|-------|---------|
| `critical` | Multiple undertested keystones, or drift detected |
| `high` | Keystones affected with coverage gaps |
| `medium` | Multiple entities affected, adequate coverage |
| `low` | Isolated changes with good test coverage |

**Examples:**
```bash
cx safe src/auth/jwt.go              # Full assessment
cx safe src/auth/jwt.go --quick      # Just blast radius
cx safe --coverage                   # Coverage gaps
cx safe --coverage --keystones-only  # Keystone gaps only
cx safe --drift                      # Staleness check
cx safe --drift --strict             # CI mode
cx safe --changes                    # What changed
cx safe --changes --detailed         # With hash details
cx safe --depth 5 src/core/          # Deeper analysis
```

**Output Structure (Full Mode):**
```yaml
safety_assessment:
  target: file-or-entity
  risk_level: critical|high|medium|low
  impact_radius: int
  files_affected: int
  keystone_count: int
  coverage_gaps: int
  drift_detected: bool
warnings:
  - "Warning message"
recommendations:
  - "Suggested action"
affected_keystones:
  - name: EntityName
    type: method
    location: file:line
    pagerank: float
    coverage: percentage|unknown
```

---

### cx dead

Find provably dead code in the codebase.

**Usage:** `cx dead [flags]`

Dead code = private/unexported symbols with zero callers.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--include-exports` | bool | false | Also show unused exports (lower confidence) |
| `--by-file` | bool | false | Group results by file path |
| `--type` | string | - | Filter by type: `F`, `T`, `M`, `C` |
| `--create-task` | bool | false | Print bd create commands for cleanup |

**Examples:**
```bash
cx dead                        # Find dead private code
cx dead --include-exports      # Include unused exports
cx dead --by-file              # Group by file
cx dead --type F               # Functions only
cx dead --create-task          # Output as beads tasks
```

**Output Structure:**
```yaml
dead_code:
  count: int
  by_type:
    function: int
    method: int
  results:
    - name: EntityName
      type: function
      location: file:line
      visibility: private
```

---

### cx guard

Pre-commit hook to catch problems before commit.

**Usage:** `cx guard [flags]`

**Checks Performed:**
1. Coverage regression for modified keystones
2. New untested code (0% coverage)
3. Breaking changes (signature changes with unchecked callers)
4. Graph drift (database out of sync)

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Pass |
| 1 | Warnings only |
| 2 | Errors |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--staged` | bool | true | Check only staged files |
| `--all` | bool | false | Check all modified files |
| `--fail-on-warnings` | bool | false | Exit with error on warnings |
| `--min-coverage` | float | 50 | Min coverage threshold for keystones |

**Examples:**
```bash
cx guard                      # Check staged files
cx guard --all                # All modified files
cx guard --fail-on-warnings   # Strict mode

# Install as git hook:
echo 'cx guard --staged' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

---

## Testing & Coverage Commands

### cx test

Smart test selection based on code changes and coverage analysis.

**Usage:** `cx test [file-or-path] [flags]`

**Modes:**
1. Test selection (default): Find tests affected by changes
2. Coverage gaps (`--gaps`): Show coverage gaps
3. Coverage summary (`--coverage`): Overall statistics

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--diff` | bool | true | Use git diff HEAD to find changes |
| `--commit` | string | - | Use specific commit |
| `--file` | string | - | Specify file directly |
| `--affected` | string | - | Find tests affected by entity |
| `--depth` | int | 2 | Indirect test discovery depth |
| `--run` | bool | false | Actually run the selected tests |
| `--output-command` | bool | false | Output go test command only |
| `--gaps` | bool | false | Show coverage gaps |
| `--coverage` | bool | false | Show coverage summary |
| `--keystones-only` | bool | false | With --gaps: only keystones |
| `--threshold` | int | 75 | Coverage threshold % |
| `--by-priority` | bool | false | Group output by priority tier |

**Subcommands:**

#### cx test coverage

Coverage data management.

**Usage:** `cx test coverage [command]`

**Subcommands:**
- `import <file|dir>` - Import coverage data

**cx test coverage import flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--base-path` | string | `.` | Base path for file normalization |

#### cx test discover

Discover test functions in the codebase.

**Usage:** `cx test discover [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--language` | string | - | Filter by language |

#### cx test list

List discovered test functions.

**Usage:** `cx test list [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--language` | string | - | Filter by language |
| `--for-entity` | string | - | Tests covering specific entity |

#### cx test impact

Analyze test coverage for a file or entity.

**Usage:** `cx test impact <file-or-entity> [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--threshold` | int | 0 | Show entities below threshold |
| `--uncovered` | bool | false | List all uncovered entities |

#### cx test suggest

Generate prioritized test suggestions.

**Usage:** `cx test suggest [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--top` | int | 0 | Limit to top N suggestions |
| `--keystones-only` | bool | false | Only suggest for keystones |
| `--entity` | string | - | Suggestions for specific entity |
| `--scenarios` | bool | false | Include test scenario suggestions |
| `--threshold` | int | 75 | Coverage threshold % |

**Examples:**
```bash
cx test                              # Tests for uncommitted changes
cx test --diff --run                 # Find and run tests
cx test internal/auth/login.go       # Tests for specific file
cx test --gaps                       # Coverage gaps
cx test --gaps --keystones-only      # Keystone gaps

# Coverage import
cx test coverage import coverage.out
cx test coverage import .coverage/   # Per-test directory

# Test discovery
cx test discover
cx test list --for-entity LoginUser

# Test suggestions
cx test suggest --top 10
cx test suggest --keystones-only
```

---

### cx coverage

Import and analyze test coverage data.

**Usage:** `cx coverage [command]`

**Subcommands:**

#### cx coverage import

Import coverage data from coverage.out file or directory.

**Usage:** `cx coverage import <coverage.out | directory> [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--base-path` | string | `.` | Base path for file normalization |

**Supported Formats:**
1. `coverage.out` file (from `go test -coverprofile`)
2. Per-test coverage directory (TestName.out files) - RECOMMENDED
3. GOCOVERDIR directory

#### cx coverage status

Show coverage statistics.

**Usage:** `cx coverage status`

**Examples:**
```bash
# Generate and import coverage
go test -coverprofile=coverage.out ./...
cx coverage import coverage.out

# Per-test coverage (recommended)
cx coverage import .coverage/

# Show statistics
cx coverage status
```

---

## Tagging & Organization Commands

### cx tag

Add tags to code entities for organization and bookmarking.

**Usage:** `cx tag <entity> <tag...> [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --note` | string | - | Add a note explaining the tag |
| `--by` | string | `cli` | Who is adding the tag |

**Examples:**
```bash
cx tag LoginUser important           # Single tag
cx tag LoginUser auth security       # Multiple tags
cx tag LoginUser -n "needs audit"    # Tag with note
cx tag Store@internal/store core     # With file hint
```

---

### cx untag

Remove a tag from a code entity.

**Usage:** `cx untag <entity> <tag>`

**Examples:**
```bash
cx untag LoginUser review
cx untag sa-fn-abc123-Login todo
```

---

### cx tags

List tags for an entity or all tags in the database.

**Usage:** `cx tags [entity] [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--find` | stringArray | - | Find entities with tag (repeatable) |
| `--all` | bool | false | Require ALL tags (AND logic) |
| `--any` | bool | true | Require ANY tag (OR logic) |

**Subcommands:**

#### cx tags export

Export all tags to a file.

**Usage:** `cx tags export [file] [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o, --output` | string | stdout | Output file (default: .cx/tags.yaml with -o flag) |

#### cx tags import

Import tags from a file.

**Usage:** `cx tags import <file> [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--overwrite` | bool | false | Overwrite existing tags |
| `--dry-run` | bool | false | Show what would be imported |

**Examples:**
```bash
cx tags LoginUser                    # Tags for entity
cx tags                              # All tags with counts
cx tags --find critical              # Find tagged entities
cx tags --find auth --find api --all # ALL tags (AND)
cx tags --find auth --find api --any # ANY tag (OR)

# Export/import
cx tags export tags.yaml
cx tags export -o                    # Export to .cx/tags.yaml
cx tags import tags.yaml
cx tags import tags.yaml --overwrite
```

---

## Database & Maintenance Commands

### cx scan

Scan a codebase and build the context graph.

**Usage:** `cx scan [path] [flags]`

Auto-initializes .cx directory if needed.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | false | Rescan even if files unchanged |
| `--overview` | bool | false | Show project overview after scan |
| `--lang` | string | - | Scan only specific language |
| `--exclude` | strings | - | Exclude patterns (comma-separated globs) |
| `--dry-run` | bool | false | Show what would be created |

**Supported Languages:** Go, TypeScript, JavaScript, Java, Rust, Python, C, C#, PHP

**Examples:**
```bash
cx scan                       # Scan current directory
cx scan ./src                 # Scan specific directory
cx scan --force               # Force full rescan
cx scan --overview            # With project overview
cx scan --lang go             # Go files only
cx scan --exclude "vendor/*"  # Exclude patterns
cx scan --dry-run             # Preview
```

---

### cx db

Database management commands.

**Usage:** `cx db [command]`

**Subcommands:**

#### cx db info

Show database statistics.

**Usage:** `cx db info`

#### cx db doctor

Check database health (alias for `cx doctor`).

**Usage:** `cx db doctor [flags]`

See [cx doctor](#cx-doctor) for flags.

#### cx db compact

Compact the database to reclaim space.

**Usage:** `cx db compact [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--remove-archived` | bool | false | Remove archived entities first |
| `--dry-run` | bool | false | Preview only |

#### cx db export

Export database to JSONL format.

**Usage:** `cx db export [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o, --output` | string | stdout | Output file |

**Examples:**
```bash
cx db info                    # Statistics
cx db doctor                  # Health check
cx db compact                 # Compact
cx db compact --remove-archived  # Remove archived first
cx db export -o backup.jsonl  # Export
```

---

### cx doctor

Check database health.

**Usage:** `cx doctor [flags]`

**Checks:**
- Database integrity (SQLite integrity_check)
- Orphan dependencies
- Stale entities
- Entity archive ratio

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--fix` | bool | false | Auto-fix issues found |
| `--deep` | bool | false | Full graph validation |
| `--yes` | bool | false | Auto-confirm fixes |

**Examples:**
```bash
cx doctor              # Run checks
cx doctor --fix        # Auto-fix
cx doctor --deep       # Full validation
cx doctor --fix --yes  # Auto-fix without prompts
```

---

### cx reset

Reset the database to a clean state.

**Usage:** `cx reset [flags]`

**Modes:**
| Mode | Description |
|------|-------------|
| Full (default) | Clear everything with backup |
| Scan-only | Clear file index only, keep entities |
| Hard | Delete database file entirely |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--scan-only` | bool | false | Only clear file index |
| `--hard` | bool | false | Delete database file (requires --force) |
| `--force` | bool | false | Skip confirmation prompt |
| `--no-backup` | bool | false | Skip backup before reset |
| `--dry-run` | bool | false | Preview only |

**Examples:**
```bash
cx reset                      # Safe reset with backup
cx reset --scan-only          # Clear file index only
cx reset --hard --force       # Delete everything
cx reset --dry-run            # Preview
```

---

### cx link

Manage links between code entities and external systems.

**Usage:** `cx link <entity-id> [external-id] [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--system` | string | `beads` | External system: `beads`, `github`, `jira` |
| `--type` | string | `related` | Link type: `related`, `implements`, `fixes`, `discovered-from` |
| `--list` | bool | false | List links for entity |
| `--remove` | bool | false | Remove a link |

**Examples:**
```bash
cx link sa-fn-abc123 bd-task-456           # Link to bead
cx link sa-fn-abc123 issue-789 --system github  # Link to GitHub
cx link --list sa-fn-abc123                # List links
cx link --remove sa-fn-abc123 bd-task-456  # Remove link
```

---

## Integration Commands

### cx live

Start MCP (Model Context Protocol) server for AI agent integration.

**Usage:** `cx live [flags]`

> **Note:** This command was renamed from `cx serve`. Both names work.

**Philosophy:** CLI for discovery, MCP for iteration.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mcp` | bool | false | Start MCP server (stdio transport) |
| `--watch` | bool | false | Enable filesystem watching for auto-rescan |
| `--tools` | string | `diff,impact,context,show` | Tools to expose |
| `--timeout` | string | `30m` | Inactivity timeout (0 for none) |
| `--status` | bool | false | Check server status |
| `--stop` | bool | false | Stop running server |
| `--list-tools` | bool | false | Show available tools |

**Available MCP Tools:**
| Tool | Description |
|------|-------------|
| `cx_diff` | Show changes since last scan |
| `cx_impact` | Analyze blast radius |
| `cx_context` | Smart context assembly |
| `cx_show` | Entity details |
| `cx_find` | Search entities |
| `cx_gaps` | Coverage gap analysis |

**Examples:**
```bash
cx live --mcp                 # Start server
cx live --mcp --watch         # With filesystem watching
cx live --mcp --tools diff,impact  # Specific tools
cx live --mcp --timeout 60m   # Custom timeout
cx live --status              # Check status
cx live --stop                # Stop server
cx live --list-tools          # Available tools
```

---

### cx daemon

Control the CX background daemon for live code graph updates.

**Usage:** `cx daemon [command]`

**Subcommands:**

#### cx daemon start

Start the daemon.

**Usage:** `cx daemon start [flags]`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--background` | bool | false | Run in background |
| `--idle-timeout` | string | `30m` | Idle timeout before auto-shutdown |
| `--project` | string | - | Project root path |
| `--cx-dir` | string | - | CX directory path |

#### cx daemon status

Show daemon status.

**Usage:** `cx daemon status`

#### cx daemon stop

Stop the running daemon.

**Usage:** `cx daemon stop`

**Examples:**
```bash
cx daemon start               # Start foreground
cx daemon start --background  # Start background
cx daemon status              # Check status
cx daemon stop                # Stop daemon
```

---

### cx help-agents

Output agent-optimized command reference.

**Usage:** `cx help-agents [flags]`

A concise, token-efficient reference designed for AI agent context windows (~1500 tokens).

**Output Formats:**
- YAML (default): Human-readable
- JSON: Machine-parseable

**Examples:**
```bash
cx help-agents              # YAML output
cx help-agents --format json  # JSON output
```

---

## Quick Reference for AI Agents

### Essential Workflow

```bash
# 1. Session start
cx context                            # Where am I?
cx context --smart "task" --budget 8000  # Task context

# 2. Before modifying
cx safe <file>                        # Full safety check
cx safe <file> --quick                # Just blast radius

# 3. Understanding code
cx show <entity>                      # Entity details
cx show <entity> --related            # Neighborhood
cx show <entity> --graph              # Dependencies
cx find --keystones --top 10          # Critical code

# 4. After changes
cx test --diff --run                  # Run affected tests
```

### When to Use Each Command

| Need | Command |
|------|---------|
| Starting a task | `cx context --smart "task"` |
| Check impact | `cx safe <file>` |
| Understand entity | `cx show <entity>` |
| Find code | `cx find <name>` or `cx find "concept"` |
| Project overview | `cx map` |
| Critical code | `cx rank --keystones` |
| Call paths | `cx trace <from> <to>` |
| Dead code | `cx dead` |
| Run tests | `cx test --diff --run` |
| Tag important code | `cx tag <entity> <tags>` |

### Supported Languages

Go, TypeScript, JavaScript, Python, Java, Rust, C, C#, PHP

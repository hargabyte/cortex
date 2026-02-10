# Changelog

All notable changes to Cortex (cx) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-02-10

### Added - Targeted Context, Blast Radius Analysis, and Dead Code Intelligence

This release adds three new commands for precise codebase analysis, plus improvements to the scan pipeline and release infrastructure.

#### Targeted Context: `cx context --for`

Get the full neighborhood of a file, entity, or directory without semantic search overhead. Pure graph traversal returns callers, callees, related tests, and sibling context — fast and deterministic.

```bash
cx context --for src/parser/walk.go              # Full neighborhood for file
cx context --for sa-fn-abc123                    # Context for specific entity
cx context --for src/parser/ --budget 4000       # Context for directory
cx context --for 'src/parser/*.go'               # Glob pattern matching
```

**Priority ordering:** Target entities → direct callers → direct callees → related tests → sibling context. Respects `--budget` for token limiting. Broad targets (directories, globs) are optimized to avoid combinatorial expansion.

#### Blast Radius Analysis: `cx impact`

Answer "if I change this, what breaks?" before making changes. Forward BFS through the dependency graph finds all downstream dependents.

```bash
cx impact src/parser/walk.go              # What depends on this file?
cx impact sa-fn-abc123                    # Impact of changing this entity
cx impact --depth 3 src/parser/walk.go    # Deeper traversal (default: 2)
```

**Output includes:**
- Direct and transitive dependents with hop distance
- Affected test functions
- Risk assessment (low/medium/high) based on keystone status and coverage
- Suggested test commands for validation

#### Dead Code Intelligence: `cx dead --tier` and `--chains`

Enhanced dead code detection with three confidence tiers and dead chain grouping.

**Confidence tiers:**
- **Tier 1 (definite):** Private + zero callers. Safe to delete. *(default)*
- **Tier 2 (probable):** Exported + zero internal callers. May be used externally.
- **Tier 3 (suspicious):** All callers are themselves dead/suspicious. Dead in practice.

```bash
cx dead                        # Tier 1 only (safe, conservative)
cx dead --tier 2               # Include unused exports
cx dead --tier 3               # Include suspicious (transitive dead)
cx dead --tier 3 --chains      # Group connected dead code for atomic cleanup
cx dead --tier 2 --type F      # Filter to functions only
```

**Dead chain detection** uses union-find to group connected dead entities, so you can clean up entire dead subgraphs atomically instead of one symbol at a time.

#### Incremental Scanning: `cx scan --incremental`

Skip unchanged files during scan using file content hashing. Dramatically speeds up repeated scans on large codebases.

```bash
cx scan --incremental          # Skip files that haven't changed
cx scan --force                # Override: rescan everything
```

### Changed

- Release workflow now builds for **5 platforms**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
- Release builds use `CGO_ENABLED=0` for reliable cross-compilation
- `--include-exports` flag on `cx dead` now implies tier 2 (backward compatible)

### Fixed

- `cx context --for` with directory/glob targets no longer hangs on large codebases (was O(N²), now O(N) with capped expansion)

---

## [0.3.0] - 2026-01-20

### Added - AI-Powered Reports, Semantic Search, and MCP Server

This release adds three major features: structured report generation with D2 diagrams, semantic code search via embeddings, and an MCP server for AI IDE integration.

#### Report Generation

Generate publication-quality codebase reports with visual D2 diagrams. Reports combine structured data with AI-written narratives for stakeholder communication.

**New Commands:**

- **`cx report overview --data`** - System architecture with module structure
  ```bash
  cx report overview --data --theme earth-tones   # Architecture diagram
  cx report overview --data -o overview.yaml       # Write to file
  ```

- **`cx report feature <query> --data`** - Feature deep-dive using semantic search
  ```bash
  cx report feature "authentication" --data        # Call flow diagram
  cx report feature "payment" --data --theme dark  # Dark theme
  ```

- **`cx report changes --since <ref> --data`** - What changed (Dolt time-travel)
  ```bash
  cx report changes --since HEAD~50 --data         # Recent changes
  cx report changes --since v1.0 --until v2.0      # Between releases
  ```

- **`cx report health --data`** - Risk analysis and recommendations
  ```bash
  cx report health --data                          # Coverage gaps, complexity
  ```

- **`cx report --init-skill`** - Generate Claude Code skill for interactive reports
  ```bash
  cx report --init-skill > ~/.claude/commands/report.md
  ```

**Diagram Themes:**

12 professionally designed themes for D2 diagrams:
- `default` - Colorblind Clear (accessibility-focused, recommended)
- `earth-tones` - Natural browns and greens
- `dark` - Dark Mauve for dark mode
- `terminal` - Green-on-black retro aesthetic
- `vanilla-nitro`, `mixed-berry`, `grape-soda`, `orange-creamsicle`, `shirley-temple`, `everglade-green`, `dark-flagship`, `neutral`

**D2 Diagram Rendering:**

- **`cx render <file.d2> -o <file.svg>`** - Render D2 diagrams to SVG
  ```bash
  cx render diagram.d2 -o output.svg               # File to file
  echo '<d2 code>' | cx render - -o output.svg     # Pipe input
  ```

#### Semantic Search

Find code by meaning, not just keywords. Cortex generates vector embeddings for every entity using pure Go (no external APIs).

**New Flags:**

- **`cx find --semantic "<query>"`** - Concept-based code discovery
  ```bash
  cx find --semantic "validate user credentials"   # Finds: LoginUser, AuthMiddleware
  cx find --semantic "database connection pooling" # Finds: NewPool, GetConn
  cx find --semantic "error handling for HTTP"     # Finds: HandleError, WriteErrorResponse
  ```

**How it works:**
1. During `cx scan`, entity signatures and doc comments are embedded using all-MiniLM-L6-v2
2. Embeddings are stored in Dolt with full version history
3. Queries are embedded and compared using cosine similarity
4. Hybrid search combines: 50% semantic + 30% keyword + 20% PageRank

**Technical Details:**
- Embeddings generated via [Hugot](https://github.com/knights-analytics/hugot) (pure Go inference)
- No API keys or external services required—everything runs locally
- Vectors stored in Dolt for time-travel queries

#### MCP Server

Run Cortex as a Model Context Protocol server for AI IDE integration. Exposes all Cortex tools without spawning separate CLI processes.

**New Commands:**

- **`cx serve`** - Start MCP server
  ```bash
  cx serve                          # Start server
  cx serve --list-tools             # Show available tools
  cx serve --tools=context,safe     # Limit to specific tools
  ```

**Available MCP Tools:**
- `cx_context` - Smart context assembly for task-focused context
- `cx_safe` - Pre-flight safety check before modifying code
- `cx_find` - Search for entities by name pattern
- `cx_show` - Show detailed information about an entity
- `cx_map` - Project skeleton overview
- `cx_diff` - Show changes since last scan
- `cx_impact` - Analyze blast radius of changes
- `cx_gaps` - Find coverage gaps in critical code

**IDE Setup:**

Cursor:
```json
{
  "mcpServers": {
    "cortex": {
      "command": "cx",
      "args": ["serve"]
    }
  }
}
```

Windsurf (`~/.windsurf/mcp.json`):
```json
{
  "servers": {
    "cortex": {
      "command": "cx",
      "args": ["serve"]
    }
  }
}
```

### Changed

- Default D2 theme changed to "Colorblind Clear" for accessibility
- Improved dead code detection with entity type whitelist (excludes imports, variables, constants that can't be tracked via call graph)

---

## [0.2.0] - 2026-01-20

### Added - Dolt Database Backend

This release migrates Cortex from SQLite to [Dolt](https://github.com/dolthub/dolt), a SQL database with Git-like version control. Every `cx scan` now creates a versioned commit, enabling historical queries, diff/blame capabilities, and time-travel for AI agent workflows.

#### New Commands

- **`cx history`** - View scan commit history
  ```bash
  cx history                    # Recent scans
  cx history --limit 20         # Last 20 scans
  cx history --stats            # Include entity counts per commit
  ```

- **`cx diff`** - Compare entity graphs between commits
  ```bash
  cx diff                       # Uncommitted changes
  cx diff HEAD~1                # Changes since previous scan
  cx diff HEAD~5 HEAD           # Changes over last 5 commits
  cx diff --entity LoginUser    # Filter to specific entity
  cx diff --summary             # Just counts
  ```

- **`cx blame`** - Track entity change attribution
  ```bash
  cx blame LoginUser            # When/why this entity changed
  cx blame LoginUser --limit 5  # Last 5 changes
  ```

- **`cx branch`** - Manage Dolt branches
  ```bash
  cx branch                     # List branches
  cx branch feature-scan        # Create branch
  cx branch -c main             # Checkout branch
  cx branch -d old-scan         # Delete branch
  ```

- **`cx sql`** - Direct SQL passthrough
  ```bash
  cx sql "SELECT COUNT(*) FROM entities"
  cx sql "SELECT * FROM dolt_log LIMIT 5"
  ```

- **`cx rollback`** - Revert to previous state
  ```bash
  cx rollback                   # Undo last scan
  cx rollback HEAD~3            # Rollback to specific point
  cx rollback --hard --yes      # Hard reset (destructive)
  ```

- **`cx stale`** - Check if code graph needs refresh (agent-optimized)
  ```bash
  cx stale                      # Check staleness
  cx stale --scans 5            # Entities unchanged for 5+ scans
  cx stale --since v1.0         # Changes since tag
  ```

- **`cx catchup`** - "What changed?" workflow for agents
  ```bash
  cx catchup                    # Rescan and show what changed
  cx catchup --since HEAD~5     # Changes since ref
  cx catchup --summary          # Brief output
  ```

#### New Flags on Existing Commands

- **`cx show --at <ref>`** - Query entity at historical point
  ```bash
  cx show LoginUser --at HEAD~5     # Entity 5 commits ago
  cx show LoginUser --at v1.0       # Entity at tagged release
  ```

- **`cx show --history`** - View entity change history
  ```bash
  cx show LoginUser --history       # Evolution over time
  ```

- **`cx find --at <ref>`** - Search at historical point
  ```bash
  cx find "Auth*" --at HEAD~10      # Find at older state
  ```

- **`cx safe --trend`** - Blast radius trend over time
  ```bash
  cx safe src/auth.go --trend       # How has impact changed?
  cx safe src/auth.go --since v1.0  # Impact trend since tag
  ```

- **`cx scan --tag`** - Tag scan for future reference
  ```bash
  cx scan --tag v1.0                # Tag this scan
  cx scan --tag before-refactor     # Named snapshot
  ```

### Changed

- **Storage location**: Database moved from `.cx/cortex.db` (SQLite) to `.cx/cortex/` (Dolt repository)
- **Schema**: Migrated from SQLite syntax to MySQL/Dolt syntax with FULLTEXT indexes
- **FTS**: Full-text search now uses MySQL FULLTEXT (natural language mode only)
- **Commits**: Each `cx scan` automatically creates a Dolt commit with metadata

### Technical Details

- Driver: `github.com/dolthub/driver` (embedded mode, no server required)
- Time-travel via `AS OF` SQL clause
- Diff via `DOLT_DIFF()` table function
- History via `dolt_log`, `dolt_history_*` system tables

### Migration Notes

Existing SQLite databases are not migrated. Run `cx scan` in your project to create a fresh Dolt database. The old `.cx/cortex.db` file can be safely deleted after migration.

---

## [0.1.6] - 2026-01-17

### Added
- Multi-language test suite covering Go, TypeScript, JavaScript, Python, Java, Rust, C, C++, C#, PHP, Kotlin, Ruby
- Improved method extraction for TypeScript/JavaScript

### Fixed
- TypeScript method extraction edge cases

---

## [0.1.5] - 2026-01-16

### Added
- `cx guard` pre-commit hook command
- `cx tag` entity tagging system
- `cx link` external system linking
- Coverage import and analysis
- Smart test selection (`cx test --diff`)

---

## [0.1.4] - 2026-01-16

### Added
- `cx safe` pre-flight safety check command
- `cx context --smart` task-focused context assembly
- `cx map` project skeleton view
- `cx trace` call path finding

---

## [0.1.3] - 2026-01-15

### Added
- PageRank importance scoring
- Keystone entity detection
- `cx find --keystones` command

---

## [0.1.2] - 2026-01-15

### Added
- Full-text search with FTS5
- Concept search (`cx find "query"`)
- Multiple output formats (YAML, JSON, JSONL)
- Density levels (sparse, medium, dense)

---

## [0.1.1] - 2026-01-14

### Added
- Initial tree-sitter based parsing
- Support for Go, TypeScript, JavaScript, Python
- Basic entity extraction and dependency tracking
- `cx scan`, `cx find`, `cx show` commands

---

## [0.1.0] - 2026-01-13

### Added
- Initial release
- Basic codebase scanning
- SQLite storage backend

# Changelog

All notable changes to Cortex (cx) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

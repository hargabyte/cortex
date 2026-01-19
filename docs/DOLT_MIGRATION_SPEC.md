# Specification: Cortex Dolt Migration

## Goal

Migrate Cortex from SQLite to Dolt to enable versioned code graph history, time-travel queries, and diff/blame capabilities for AI agent workflows. Every `cx scan` creates a versioned commit, enabling agents to understand what changed, when, and why.

## User Stories

- As an AI agent, I want to see what entities changed since my last session so I can focus on relevant code
- As a developer, I want to view the history of my codebase's dependency graph over time
- As an AI agent, I want to query the code graph at a specific point in time (before a refactor)
- As a developer, I want to diff entity graphs between two scans to understand structural changes
- As an AI agent, I want to know if my cached context is stale so I can refresh efficiently

## Molecule Architecture

### Dependency Overview

```
                         ┌─────────────────────────┐
                         │   F: Foundation         │
                         │   (Dolt Backend)        │
                         └───────────┬─────────────┘
                                     │
         ┌───────────────────────────┼───────────────────────────┐
         │                           │                           │
         ▼                           ▼                           ▼
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│  A: Diff Stream │       │  B: History     │       │  C: Utilities   │
│  - cx diff      │       │  - cx history   │       │  - cx sql       │
│  - scan --diff  │       │  - --at flag    │       │  - cx branch    │
└────────┬────────┘       └────────┬────────┘       │  - cx rollback  │
         │                         │                └────────┬────────┘
         ▼                         ▼                         │
┌─────────────────┐       ┌─────────────────┐                │
│  A2: Extended   │       │  B2: Blame      │                │
│  - --since      │       │  - cx blame     │                │
│  - --new/changed│       │  - show history │                │
│  - scan --tag   │       └────────┬────────┘                │
└────────┬────────┘                │                         │
         │                         │                         │
         └─────────────────────────┼─────────────────────────┘
                                   │
                                   ▼
                         ┌─────────────────────┐
                         │  D: Agent Optimize  │
                         │  - cx stale         │
                         │  - cx catchup       │
                         │  - safe --trend     │
                         └───────────┬─────────┘
                                     │
                                     ▼
                         ┌─────────────────────┐
                         │  E: Integration     │
                         │  - E2E tests        │
                         │  - Documentation    │
                         └─────────────────────┘
```

### Critical Path

**Minimum sequential steps:** F → A → A2 → D → E (5 phases)

**With 3 agents after Foundation:**
- Agent 1: A (Diff) → A2 (Extended)
- Agent 2: B (History) → B2 (Blame)
- Agent 3: C (Utilities)
- All converge on D (Agent Optimize)

**Parallelization gain:** 40% faster than sequential

---

## Work Streams

### Stream F: Foundation (Sequential - Blocking)

**Agent Type:** Backend Polecat (senior)
**Dependencies:** None
**Outputs:** Working Dolt backend, all existing `cx` commands functional

This stream must complete before any other work can begin. It replaces the SQLite backend entirely.

#### F1: Dolt Driver Integration
- Replace `modernc.org/sqlite` with `github.com/dolthub/driver`
- Update DSN configuration in store package
- Add config for commitname/commitemail
- Location: `.cx/cortex/` (Dolt repo inside .cx folder)

#### F2: Schema Migration
- Convert schema from SQLite to MySQL syntax
- `TEXT PRIMARY KEY` → `VARCHAR(255) PRIMARY KEY`
- `INSERT OR IGNORE` → `INSERT IGNORE`
- Remove PRAGMA statements
- Add `scan_metadata` table for tracking scan info

#### F3: FTS Migration
- Replace FTS5 virtual table with FULLTEXT index
- Update search queries from `MATCH ... AGAINST fts5` to `MATCH(...) AGAINST(...)`
- Note: Only NATURAL LANGUAGE MODE supported (no boolean mode)

#### F4: Query Adaptation
- Update all SQL queries for MySQL compatibility
- Test all existing commands work unchanged
- Handle connection lifecycle (embedded mode)

#### F5: Scan Integration
- Add `CALL dolt_commit()` after successful scan
- Commit message: `cx scan: {entities} entities, {deps} deps [{branch}@{commit}]`
- Store git commit/branch in `scan_metadata` table

#### F6: Init/Reset Commands
- Update `cx scan` to initialize Dolt repo if not exists
- Update `cx reset` to handle Dolt repo cleanup
- Add config option: `storage.backend: dolt`

**Acceptance:** All existing `cx` commands pass tests with Dolt backend

---

### Stream A: Diff Operations (Parallelizable after F)

**Agent Type:** Backend Polecat
**Dependencies:** Foundation complete
**Outputs:** `cx diff` command, `--diff` flag

#### A1: cx diff Command
```bash
cx diff [ref1] [ref2] [--table TABLE] [--entity NAME]

# Examples:
cx diff                       # Show uncommitted changes
cx diff HEAD~1                # Compare to previous scan
cx diff HEAD~5 HEAD           # Compare two points
cx diff --entity LoginUser    # Filter to specific entity
```

**Implementation:**
- Use `DOLT_DIFF()` table function
- Output: added/modified/removed entities with details
- Format: YAML (default), JSON, table

#### A2: cx scan --diff Flag
```bash
cx scan --diff    # Scan and show what changed
```

**Implementation:**
- After scan completes, query `dolt_diff` for changes since last commit
- Show summary: X added, Y modified, Z removed

---

### Stream A2: Extended Diff (After A)

**Agent Type:** Backend Polecat
**Dependencies:** A1 (cx diff) complete
**Outputs:** `--since` flag, change tracking flags

#### A2.1: --since Flag
```bash
cx find "Auth*" --since HEAD~5     # Entities changed since ref
cx show LoginUser --since 2024-01-01
```

**Implementation:**
- Query `dolt_diff` between ref and HEAD
- Filter results to matching entities

#### A2.2: Change Tracking Flags
```bash
cx find --new                 # Entities added since last scan
cx find --changed             # Entities modified since last scan
cx find --removed             # Entities removed since last scan
```

**Implementation:**
- Query `dolt_diff('HEAD~1', 'HEAD', 'entities')`
- Filter by `diff_type`: added, modified, removed

#### A2.3: cx scan --tag
```bash
cx scan --tag "before-refactor"    # Tag this scan for reference
```

**Implementation:**
- After commit, call `DOLT_TAG()` stored procedure
- Tags become valid refs for `--at`, `--since`

---

### Stream B: History Operations (Parallelizable after F)

**Agent Type:** Backend Polecat
**Dependencies:** Foundation complete
**Outputs:** `cx history` command, `--at` flag

#### B1: cx history Command
```bash
cx history [--limit N] [--table TABLE]

# Examples:
cx history                    # Show recent scans
cx history --limit 20         # Show last 20 scans
cx history --table entities   # Show entity table changes
```

**Implementation:**
- Query `dolt_log` system table
- Show: commit hash, date, message, entity/dep counts
- Format: table (default), YAML, JSON

#### B2: --at Flag (Time Travel)
```bash
cx show LoginUser --at HEAD~5      # Entity 5 scans ago
cx show LoginUser --at main        # Entity on branch
cx find "Auth*" --at 2024-01-15    # Entities as of date
```

**Implementation:**
- Modify queries to use `AS OF 'ref'` clause
- Support refs: commit hash, branch, tag, HEAD~N, timestamp

---

### Stream B2: Blame Operations (After B)

**Agent Type:** Backend Polecat
**Dependencies:** B1, B2 complete
**Outputs:** `cx blame`, `cx show --history`

#### B2.1: cx blame Command
```bash
cx blame <entity> [--deps]

# Examples:
cx blame LoginUser            # When/why this entity changed
cx blame LoginUser --deps     # Include dependency changes
```

**Implementation:**
- Query `dolt_history_entities` for entity changes
- Show: commit, date, committer, what changed
- Optionally include `dolt_history_dependencies`

#### B2.2: cx show --history
```bash
cx show LoginUser --history    # Evolution of entity over time
```

**Implementation:**
- Query `dolt_history_entities` filtered by entity ID
- Show signature/location changes over commits

---

### Stream C: Utilities (Parallelizable after F)

**Agent Type:** Backend Polecat
**Dependencies:** Foundation complete
**Outputs:** `cx sql`, `cx branch`, `cx rollback`

#### C1: cx sql Command
```bash
cx sql "SELECT * FROM entities WHERE importance = 'keystone'"
cx sql "SELECT * FROM dolt_log LIMIT 5"
```

**Implementation:**
- Direct SQL passthrough to Dolt
- Support for Dolt system tables and functions
- Format: table (default), YAML, JSON

#### C2: cx branch Command
```bash
cx branch                     # List Dolt branches
cx branch feature-scan        # Create branch
cx branch -d old-scan         # Delete branch
cx branch --checkout feature  # Switch branch
```

**Implementation:**
- Wrap `DOLT_BRANCH()` and `DOLT_CHECKOUT()` procedures
- Show current branch in output

#### C3: cx rollback Command
```bash
cx rollback                   # Undo last scan (reset to HEAD~1)
cx rollback HEAD~3            # Rollback to specific point
cx rollback --hard            # Also delete working changes
```

**Implementation:**
- Use `DOLT_RESET()` stored procedure
- Warn before destructive operations

---

### Stream D: Agent Optimizations (After A2, B2)

**Agent Type:** Backend Polecat
**Dependencies:** Extended diff and blame complete
**Outputs:** Agent-focused commands

#### D1: cx stale Command
```bash
cx stale                      # Check if data needs refresh
cx stale --threshold 1h       # Stale if older than 1 hour
```

**Implementation:**
- Compare last scan time to current time
- Check if git HEAD has changed since last scan
- Output: stale/fresh, last scan time, files changed since

#### D2: cx catchup Command
```bash
cx catchup                    # Rescan and show what changed
```

**Implementation:**
- Run `cx scan --diff` equivalent
- Optimized for "I was away, what changed?" workflow

#### D3: cx safe --trend
```bash
cx safe <file> --trend        # Blast radius over time
cx safe <file> --since HEAD~5 # Changes in impact since ref
```

**Implementation:**
- Query blast radius at multiple points
- Show if impact is growing/shrinking

---

### Stream E: Integration (After D)

**Agent Type:** Refinery/Senior
**Dependencies:** All streams complete
**Outputs:** E2E tests, documentation

#### E1: E2E Tests
- Test full workflow: init → scan → diff → history → rollback
- Test time-travel queries across multiple scans
- Test branch operations

#### E2: Documentation
- Update README with Dolt-powered features
- Update `cx help-agents` with new commands
- Add migration guide for existing users

---

## Data Contracts

### DSN Configuration
```go
// New DSN format for Dolt embedded driver
dsn := "file:///path/to/.cx/cortex?commitname=Cortex&commitemail=cx@local&database=cortex"
```

### Schema: scan_metadata Table
```sql
CREATE TABLE scan_metadata (
    id INT AUTO_INCREMENT PRIMARY KEY,
    scan_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    git_commit VARCHAR(40),
    git_branch VARCHAR(255),
    files_scanned INT,
    entities_found INT,
    dependencies_found INT,
    scan_duration_ms INT
);
```

### Dolt System Tables Used
| Table | Purpose |
|-------|---------|
| `dolt_log` | Commit history for `cx history` |
| `dolt_status` | Working changes for `cx diff` (uncommitted) |
| `dolt_diff_entities` | Entity changes between commits |
| `dolt_diff_dependencies` | Dependency changes between commits |
| `dolt_branches` | Branch list for `cx branch` |
| `dolt_history_entities` | Full entity history for `cx blame` |

### Ref Formats Supported
| Format | Example | Description |
|--------|---------|-------------|
| Commit hash | `abc123def` | Specific commit |
| Branch | `main`, `feature` | Branch HEAD |
| Tag | `v1.0`, `before-refactor` | Named snapshot |
| Relative | `HEAD~5`, `HEAD^` | Relative to HEAD |
| Timestamp | `2024-01-15` | Point in time |

---

## Existing Code to Leverage

| File | Relevance |
|------|-----------|
| `internal/store/db.go` | Main Store struct - replace SQLite connection |
| `internal/store/schema.go` | Schema SQL - convert to MySQL syntax |
| `internal/store/entity.go` | Entity CRUD - update query syntax |
| `internal/store/deps.go` | Dependency CRUD - update query syntax |
| `internal/store/search.go` | FTS queries - migrate to FULLTEXT |
| `internal/cmd/scan.go` | Scan command - add dolt_commit |
| `internal/cmd/reset.go` | Reset command - update for Dolt |

---

## Out of Scope

- **Dual backend support** - Not maintaining SQLite alongside Dolt
- **Data migration tool** - Users rescan to populate (fresh start)
- **Dolt server mode** - Using embedded driver only
- **Git branch mirroring** - Dolt branches independent of git
- **Beads integration** - Deferred until Beads migrates to Dolt
- **Remote sync** - No dolt push/pull in initial release

---

## Multi-Agent Execution Notes

### Recommended Agent Assignment

| Stream | Agent Type | Estimated Complexity |
|--------|------------|---------------------|
| F: Foundation | Senior Polecat | High (blocking, critical) |
| A: Diff | Polecat | Medium |
| A2: Extended | Polecat | Medium |
| B: History | Polecat | Medium |
| B2: Blame | Polecat | Low-Medium |
| C: Utilities | Polecat | Low |
| D: Agent Optimize | Polecat | Medium |
| E: Integration | Refinery | Medium |

### Parallelization Strategy

1. **Phase 1 (Sequential):** Foundation must complete first
2. **Phase 2 (3-way parallel):** A, B, C streams run simultaneously
3. **Phase 3 (2-way parallel):** A2, B2 extend their parent streams
4. **Phase 4 (Sequential):** D depends on A2 and B2
5. **Phase 5 (Sequential):** E integration and testing

### Estimated Agent Utilization

- **With 1 agent:** 8 sequential stream completions
- **With 3 agents:** 5 rounds (F → A|B|C → A2|B2 → D → E)
- **Parallelization gain:** ~40%
- **Bottleneck:** Foundation (F) is the critical blocker

### Task Granularity

Foundation should be broken into ~6 sub-tasks (F1-F6)
Each other stream: 2-4 sub-tasks
Total estimated tasks: ~25-30

---

## Technical Notes from Research

### Dolt Driver (github.com/dolthub/driver)
- database/sql compatible, works like SQLite
- DSN: `file:///path?commitname=X&commitemail=Y&database=Z`
- `CREATE DATABASE` works via SQL - no CLI needed

### Key Stored Procedures
```sql
CALL dolt_add('-A');                    -- Stage all
CALL dolt_commit('-a', '-m', 'msg');    -- Stage + commit
CALL dolt_branch('name');               -- Create branch
CALL dolt_checkout('branch');           -- Switch branch
CALL dolt_reset('--hard', 'HEAD~1');    -- Rollback
CALL dolt_tag('-m', 'msg', 'tagname');  -- Create tag
```

### Time Travel Syntax
```sql
SELECT * FROM entities AS OF 'HEAD~5' WHERE name = 'X';
SELECT * FROM entities AS OF 'main';
SELECT * FROM entities AS OF TIMESTAMP('2024-01-15');
```

### FULLTEXT Limitation
- Only `IN NATURAL LANGUAGE MODE` supported
- Boolean mode (`+term -term`) not available
- May need to adjust `cx find` search behavior

### Transaction vs Dolt Commit
- SQL transactions separate from Dolt commits
- Call `DOLT_COMMIT()` explicitly after scan batch
- Alternative: `@@dolt_transaction_commit=1` (loses custom messages)

---

## Success Criteria

1. All existing `cx` commands work unchanged with Dolt backend
2. `cx scan` creates versioned commits automatically
3. `cx diff` shows entity changes between any two refs
4. `cx history` shows scan log with metadata
5. `--at` flag enables time-travel on `cx show` and `cx find`
6. `cx blame` shows change attribution for entities
7. Performance: No noticeable slowdown for typical operations
8. Tests: E2E test suite for version control features

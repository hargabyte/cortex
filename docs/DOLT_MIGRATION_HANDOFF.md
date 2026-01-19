# Dolt Migration Handoff

## Session Summary (Updated 2026-01-19)

Completed research, specification, and task creation for migrating Cortex from SQLite to Dolt database.

## What Was Done

### Session 1: Research & Planning
1. Compared alternative tools (Graphiti, Augment, Claude-Flow)
2. Deep-dived into Dolt capabilities
3. Created design document: `docs/DOLT_MIGRATION.md`
4. Created initial beads structure

### Session 2: Research & Specification
1. **Thorough Dolt research** - Go SDK, embedding, stored procedures, system tables
2. **Wrote formal specification**: `docs/DOLT_MIGRATION_SPEC.md`
3. **Created task hierarchy** with 8 work streams and 31 total beads
4. **Wired dependencies** for proper execution order

## Key Technical Knowledge Acquired

### Dolt Go Driver
```go
import _ "github.com/dolthub/driver"
dsn := "file:///path/to/.cx/cortex?commitname=Cortex&commitemail=cx@local&database=cortex"
db, err := sql.Open("dolt", dsn)
```

### Key Stored Procedures
```sql
CALL dolt_add('-A');                    -- Stage all
CALL dolt_commit('-a', '-m', 'msg');    -- Stage + commit
CALL dolt_branch('name');               -- Create branch
CALL dolt_checkout('branch');           -- Switch branch
CALL dolt_reset('--hard', 'HEAD~1');    -- Rollback
```

### System Tables
| Table | Purpose |
|-------|---------|
| `dolt_log` | Commit history |
| `dolt_status` | Working changes |
| `dolt_diff_$table` | Row-level changes |
| `dolt_history_$table` | Full row history |
| `dolt_branches` | Branch list |

### Time Travel
```sql
SELECT * FROM entities AS OF 'HEAD~5' WHERE name = 'X';
SELECT * FROM entities AS OF TIMESTAMP('2024-01-15');
```

### FULLTEXT Limitation
- Only `IN NATURAL LANGUAGE MODE` supported
- No boolean mode (`+term -term`)

## Decisions Made

| Decision | Choice |
|----------|--------|
| Connection mode | Embedded (`github.com/dolthub/driver`) |
| DB location | `.cx/cortex/` |
| Auto-commit | Yes, every scan |
| Branch strategy | Independent from git |
| SQLite transition | Full replacement |
| Data migration | Fresh start (rescan) |
| FTS approach | FULLTEXT with NATURAL LANGUAGE MODE |

## Beads Structure

```
cortex-104 [epic] CX 4.0: Dolt Database Migration
└── cortex-104.1 [feature] Cortex Dolt Migration
    ├── cortex-104.1.3 [task] F: Foundation (6 subtasks) ← READY
    ├── cortex-104.1.4 [task] A: Diff Operations (2 subtasks)
    ├── cortex-104.1.7 [task] A2: Extended Diff (3 subtasks)
    ├── cortex-104.1.5 [task] B: History Operations (2 subtasks)
    ├── cortex-104.1.8 [task] B2: Blame Operations (2 subtasks)
    ├── cortex-104.1.2 [task] C: Utilities (3 subtasks)
    ├── cortex-104.1.6 [task] D: Agent Optimizations (3 subtasks)
    └── cortex-104.1.9 [task] E: Integration (2 subtasks)
```

**Labels:** spec-complete, molecule-designed, tasks-created

## Dependency Graph

```
F1, F2, F3, F4 (parallel) → F5 → F6
        ↓
    ┌───┼───┐
    A   B   C  (parallel after F)
    ↓   ↓
   A2  B2
    ↓   ↓
    └─D─┘  (fan-in)
      ↓
      E
```

## Ready to Work

```bash
bd ready --parent cortex-104.1

1. [P1] cortex-104.1.3.1: F1: Dolt Driver Integration
2. [P1] cortex-104.1.3.2: F2: Schema Migration
3. [P1] cortex-104.1.3.3: F3: FTS Migration
4. [P1] cortex-104.1.3.4: F4: Query Adaptation
```

## Files

| File | Purpose |
|------|---------|
| `docs/DOLT_MIGRATION.md` | Original design document |
| `docs/DOLT_MIGRATION_SPEC.md` | Formal specification with molecule architecture |
| `docs/DOLT_MIGRATION_HANDOFF.md` | This file - session handoff |

## Next Steps

1. **Start Foundation**: `/implement cortex-104.1.3.1` (Dolt Driver Integration)
2. **Or orchestrate**: `/orchestrate cortex-104.1` to distribute to agents
3. **Check progress**: `bd ready --parent cortex-104.1`

## Critical Path

**Minimum time:** F → A|B|C → A2|B2 → D → E (5 phases)
**With 3 agents:** ~40% faster via parallelization

## Research Sources

- [Dolt Driver GitHub](https://github.com/dolthub/driver)
- [Embedding Dolt Blog](https://www.dolthub.com/blog/2022-07-25-embedded/)
- [Dolt Procedures Docs](https://docs.dolthub.com/sql-reference/version-control/dolt-sql-procedures)
- [Dolt System Tables](https://docs.dolthub.com/sql-reference/version-control/dolt-system-tables)
- [Querying History](https://docs.dolthub.com/sql-reference/version-control/querying-history)
- [FULLTEXT Implementation](https://www.dolthub.com/blog/2023-08-14-implementing-fulltext-indexes/)

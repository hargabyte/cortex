# Dolt Migration Handoff

## Session Summary

Completed research and planning for migrating Cortex from SQLite to Dolt database.

## What Was Done

1. **Compared alternative tools** for context:
   - Graphiti (knowledge graphs) - bi-temporal model inspiration
   - Augment (MCP server) - integration pattern inspiration
   - Claude-Flow (agent orchestration) - complementary, not competitive
   - GasTown (our orchestrator) - native integration path

2. **Deep-dived into Dolt**:
   - Git-like version control for SQL databases
   - CLI mirrors git commands (diff, log, blame, branch, merge)
   - MySQL-compatible (works with standard drivers)
   - Performance: ~5% slower reads, ~10% faster writes vs MySQL

3. **Created design document**: `docs/DOLT_MIGRATION.md`

4. **Created beads**:
   - `cortex-104` [epic] CX 4.0: Dolt Database Migration
   - `cortex-104.1` [feature] Cortex Dolt Migration
   - `cortex-104.1.1` [task] Dolt Migration Requirements (full details)

## Key Decisions Made

| Decision | Choice |
|----------|--------|
| Connection mode | Embedded (single process) |
| DB location | `.cx/cortex/` |
| Auto-commit | Yes, every scan |
| Branch strategy | Independent from git |
| SQLite transition | Full replacement (no dual backend) |
| Data migration | Fresh start (rescan, no migration command) |

## Phased Build Order

| Phase | Focus |
|-------|-------|
| **1** | Foundation - Dolt backend, schema migration, basic scan works |
| **2** | Core commands - `cx diff`, `cx history`, `--at` flag |
| **3** | Extended flags - `--since`, `--new/--changed/--removed` |
| **4** | Advanced - `cx sql`, `cx blame`, `cx rollback`, `cx branch` |
| **5** | Agent optimizations - `cx stale`, `cx catchup`, compound commands |

## Technical Notes

### Schema Changes Needed
- SQLite FTS5 → MySQL FULLTEXT indexes
- `INSERT OR IGNORE` → `INSERT IGNORE`
- `TEXT PRIMARY KEY` → `VARCHAR(255) PRIMARY KEY`
- Remove PRAGMA statements

### Key Dolt Operations
```sql
CALL dolt_add('-A');
CALL dolt_commit('-m', 'cx scan: 120 files, 3444 entities');
SELECT * FROM entities AS OF 'HEAD~5' WHERE name = 'LoginUser';
SELECT * FROM dolt_diff('HEAD~1', 'HEAD', 'entities');
```

## Files Changed

- `docs/DOLT_MIGRATION.md` - Full design document (committed + pushed)

## Next Step

Run `/write-spec cortex-104.1` to create formal specification with implementation tasks.

## Beads State

The beads were created but .beads/ is gitignored in this repo. The bead IDs are:
- `cortex-104` - Epic
- `cortex-104.1` - Feature
- `cortex-104.1.1` - Requirements task

If beads aren't visible in next session, recreate from `docs/DOLT_MIGRATION.md`.

# Continue Cortex Dolt Migration

## Quick Start

```bash
git checkout feature/dolt-migration
cat docs/DOLT_MIGRATION_SESSION_3.md  # Full context
```

## Current State

F1-F4 Foundation stream **COMPLETE**. All tests pass. Beads closed.

## Immediate Task

**F5: Scan Integration** (cortex-104.1.3.5)

```bash
/implement cortex-104.1.3.5
```

### What to Build

1. Add `DoltCommit(message string) (string, error)` to store
2. Call `CALL dolt_commit()` after successful scan
3. Store git commit/branch in `scan_metadata` table
4. Commit message: `cx scan: {entities} entities, {deps} deps [{branch}@{commit}]`

### Key Files

| Purpose | File |
|---------|------|
| Scan command | `internal/cmd/scan.go` |
| Store (add DoltCommit) | `internal/store/db.go` |
| Schema | `internal/store/schema.go` |

### Dolt Commit SQL

```sql
CALL dolt_commit('-Am', 'cx scan: 150 entities, 300 deps [main@abc1234]');
```

## After F5

**F6: Init/Reset Commands** (cortex-104.1.3.6)
- Update `cx scan` to init Dolt repo if not exists
- Update `cx reset` to handle Dolt cleanup

## Technical Notes

- Dolt DSN: `file:///path?commitname=Cortex&commitemail=cx@local&database=cortex`
- FULLTEXT must be inline in CREATE TABLE (Dolt limitation)
- No table aliases in FULLTEXT queries (Dolt bug)
- Schema uses individual statements (no multi-statement)

## Dependency Graph

```
F1-F4 ✅ → F5 (now) → F6 → (A, B, C parallel) → (A2, B2) → D → E
```

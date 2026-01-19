# Dolt Migration Session 3 Handoff

## Session Summary

Completed F1-F4 of the Foundation stream. All store tests pass. Beads closed.

## Completed Work

### F1-F4: Foundation Stream ✅

All four foundation tasks are complete and tested:

| Task | Status | Key Changes |
|------|--------|-------------|
| F1: Dolt Driver | ✅ Closed | Replaced SQLite with `github.com/dolthub/driver`, DSN format, CREATE DATABASE logic |
| F2: Schema Migration | ✅ Closed | MySQL syntax, split to individual statements, inline FULLTEXT |
| F3: FTS Migration | ✅ Closed | FULLTEXT index, MATCH AGAINST syntax, no-op RebuildFTSIndex |
| F4: Query Adaptation | ✅ Closed | INSERT IGNORE, REPLACE INTO, ON DUPLICATE KEY UPDATE |

### Key Bug Fixes This Session

1. **FULLTEXT index not recognized**: Dolt requires inline `FULLTEXT` in CREATE TABLE, not separate `CREATE FULLTEXT INDEX`

2. **Dolt table alias bug**: FULLTEXT queries fail when table has an alias (even `FROM entities e`). Changed to use full table names without aliases.

3. **MySQL natural language mode**: No prefix matching - "auth" won't find "authenticate". Updated tests accordingly.

## Files Modified

| File | Changes |
|------|---------|
| `internal/store/schema.go` | Added inline FULLTEXT to entities table |
| `internal/store/fts.go` | Removed table aliases from FTS query, added `:` to cleanFTSWord |
| `internal/store/fts_test.go` | Changed "auth" to "authenticate" in test |

## Test Status

```bash
go test ./internal/store/... -count=1   # ✅ PASS (12.2s)
go build ./...                          # ✅ PASS
```

## Next Task: F5 Scan Integration

**Bead**: `cortex-104.1.3.5`

### Requirements

Add Dolt commit after successful scan to create version history:

1. **Call `dolt_commit()` after scan completes**
   - Only on successful scan (no errors)
   - Store commit hash for reference

2. **Store git context in `scan_metadata` table**
   - `git_commit`: Current HEAD commit
   - `git_branch`: Current branch name
   - Scan statistics (files, entities, dependencies, duration)

3. **Commit message format**
   ```
   cx scan: {entities} entities, {deps} deps [{branch}@{commit[:7]}]
   ```

### Key Files

| Purpose | File |
|---------|------|
| Scan command | `internal/cmd/scan.go` |
| Store operations | `internal/store/db.go` |
| Schema (scan_metadata) | `internal/store/schema.go` |

### Dolt Commit API

```sql
-- Create a Dolt commit
CALL dolt_commit('-Am', 'commit message');

-- Or with specific options
CALL dolt_commit('-m', 'message', '--author', 'Name <email>');
```

### Implementation Notes

- Add method to store: `DoltCommit(message string) (string, error)` returning commit hash
- Call after `scanner.Scan()` succeeds in scan.go
- Get git info using `git rev-parse HEAD` and `git branch --show-current`

## Dependency Graph

```
F1-F4 ✅ → F5 (next) → F6 → (A, B, C parallel) → (A2, B2) → D → E
```

## Resume Commands

```bash
# Check out branch
git checkout feature/dolt-migration

# Verify tests still pass
go test ./internal/store/... -count=1

# Start F5 implementation
/implement cortex-104.1.3.5
```

## Technical Context

### Dolt DSN Format
```go
dsn := "file:///path?commitname=Cortex&commitemail=cx@local&database=cortex"
```

### FULLTEXT Limitations (Dolt-specific)
- Must be inline in CREATE TABLE (not separate CREATE INDEX)
- Cannot use table aliases in queries
- Only NATURAL LANGUAGE MODE supported
- No prefix matching

### Schema Location
- Database: `.cx/cortex/` (Dolt repo directory)
- Tables: entities, dependencies, metrics, file_index, entity_links, entity_tags, entity_coverage, test_entity_map, scan_metadata

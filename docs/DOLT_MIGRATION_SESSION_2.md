# Dolt Migration Session 2 Handoff

## Session Summary

Implemented F1-F4 of the Foundation stream. The core Dolt backend migration is largely complete but needs test fixes.

## Completed Work

### F1: Dolt Driver Integration ✅
- Replaced `modernc.org/sqlite` with `github.com/dolthub/driver`
- Updated DSN format: `file:///path?commitname=Cortex&commitemail=cx@local&database=cortex`
- Added database creation logic (`CREATE DATABASE IF NOT EXISTS cortex`)
- Location changed: `.cx/cortex.db` → `.cx/cortex/` (Dolt repo directory)

### F2: Schema Migration ✅
- Converted schema from SQLite to MySQL syntax
- `TEXT PRIMARY KEY` → `VARCHAR(255) PRIMARY KEY`
- `TEXT` fields → `VARCHAR(n)` for indexed fields
- `INTEGER` → `INT`, `REAL` → `DOUBLE`
- Split schema into individual statements (Dolt requires single-statement execution)
- Removed FTS5 virtual table and triggers
- Added `scan_metadata` table

### F3: FTS Migration ✅
- Removed FTS5 virtual table
- Added FULLTEXT index on `(name, body_text, doc_comment)`
- Changed search queries from `MATCH ... AGAINST fts5` to `MATCH(...) AGAINST(... IN NATURAL LANGUAGE MODE)`
- Updated `buildFTSQuery()` for natural language mode (no `*` prefix, no `OR` operators)
- `RebuildFTSIndex()` is now a no-op (MySQL FULLTEXT auto-maintains)

### F4: Query Adaptation ✅
- `INSERT OR IGNORE` → `INSERT IGNORE`
- `INSERT OR REPLACE` → `REPLACE INTO`
- `ON CONFLICT...DO UPDATE` → `ON DUPLICATE KEY UPDATE`
- Removed `PRAGMA` statements
- Updated `checkIntegrity()` in doctor.go (no more PRAGMA integrity_check)

## Files Modified

| File | Changes |
|------|---------|
| `internal/store/db.go` | Dolt driver, DSN, CREATE DATABASE |
| `internal/store/schema.go` | MySQL schema, split statements |
| `internal/store/fts.go` | FULLTEXT search queries |
| `internal/store/entities.go` | INSERT IGNORE |
| `internal/store/deps.go` | REPLACE INTO |
| `internal/store/links.go` | REPLACE INTO |
| `internal/store/metrics.go` | REPLACE INTO |
| `internal/store/fileindex.go` | REPLACE INTO |
| `internal/store/tags.go` | ON DUPLICATE KEY UPDATE |
| `internal/coverage/mapper.go` | REPLACE INTO, INSERT IGNORE |
| `internal/cmd/doctor.go` | Removed PRAGMA integrity_check |
| `internal/store/store_test.go` | Updated for directory structure |
| `internal/store/fts_test.go` | Updated for MySQL FULLTEXT syntax |

## Remaining Work

### Immediate (Same PR)
1. **Fix remaining test failures** - Run `go test ./internal/store/...`
   - FTS tests updated but need verification
   - Some tests may still expect old behavior

2. **Verify build** - `go build ./...`

### F5: Scan Integration (Next Task)
- Add `CALL dolt_commit()` after successful scan
- Store git commit/branch in `scan_metadata` table

### F6: Init/Reset Commands
- Update `cx scan` to initialize Dolt repo if not exists
- Update `cx reset` to handle Dolt repo cleanup

## Test Status

```bash
# Basic tests passing:
go test ./internal/store/... -run "TestOpen|TestCreateEntity"  # ✅ PASS

# Full test suite needs fixes:
go test ./internal/store/...  # Some failures in FTS tests
```

## Resume Commands

```bash
# Check out the branch
git checkout feature/dolt-migration

# Run tests to see current status
go test ./internal/store/... -count=1

# After fixing tests, commit
git add -A && git commit -m "Complete F1-F4: Dolt backend migration"

# Continue with F5
/implement cortex-104.1.3.5
```

## Key Technical Notes

### Dolt Driver DSN Format
```go
// Without database (for CREATE DATABASE)
dsn := "file:///path?commitname=Cortex&commitemail=cx@local"

// With database
dsn := "file:///path?commitname=Cortex&commitemail=cx@local&database=cortex"
```

### MySQL FULLTEXT Limitations
- Only `IN NATURAL LANGUAGE MODE` supported (no boolean mode)
- No prefix matching (`*`)
- Relevance score returned by `MATCH() AGAINST()` in SELECT

### Schema Execution
Dolt requires single-statement execution. Schema is now:
```go
var schemaTables = []string{
    "CREATE TABLE IF NOT EXISTS entities (...)",
    "CREATE TABLE IF NOT EXISTS dependencies (...)",
    // ...
}
```

## Beads Status

- `cortex-104.1.3.1` (F1) - Ready to close
- `cortex-104.1.3.2` (F2) - Ready to close
- `cortex-104.1.3.3` (F3) - Ready to close
- `cortex-104.1.3.4` (F4) - Ready to close
- `cortex-104.1.3.5` (F5) - Blocked, waiting for F1-F4

Close beads after tests pass:
```bash
bd close cortex-104.1.3.1 cortex-104.1.3.2 cortex-104.1.3.3 cortex-104.1.3.4 --reason "Dolt backend migration complete"
```

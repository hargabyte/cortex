# Cortex (cx) Backlog

## Remaining Work from Testing Session

### P1: Change `--keystones` to top-N ranking
**File**: `internal/cmd/rank.go` (lines 199-210)

**Problem**: `cx rank --keystones` uses absolute threshold (PR >= 0.30) but highest PageRank in a typical codebase is ~0.035, so it returns nothing.

**Fix**: Change to top-N approach:
- `--keystones` returns top 10 by PageRank (default)
- `--keystones --top 20` returns top 20
- Same for `--bottlenecks` (top N by betweenness)

---

### P2: Fix `cx rank` output order
**File**: `internal/cmd/rank.go`

**Problem**: Metrics are computed after results are printed, so output shows stale/zero values.

**Fix**: Move metrics computation before output generation.

---

### P2: Fix `cx near` file mode
**File**: `internal/cmd/near.go`

**Problem**: `cx near path/to/file.go` doesn't show all entities in that file.

**Fix**: Detect file path (no `:line` suffix) and switch to "file overview" mode showing all entities in the file.

---

### P2: Exclude imports from `cx verify` hash checking
**File**: `internal/cmd/verify.go`

**Problem**: Import entities have no meaningful body to hash, causing false positives in verification.

**Fix**: Skip import entities in verification loop.

---

### P3: Add `--limit` flag to `cx find`
**File**: `internal/cmd/find.go`

**Problem**: No way to limit result count for large codebases.

**Fix**: Add `--limit N` flag to control maximum results returned.

---

### P3: AI-optimized output improvements
**Files**: Various

Bundle of improvements:
- `cx status` alias for quick overview
- `--format jsonl` for streaming JSON lines
- `cx diff` to show changes since last scan
- `--quiet` flag for minimal output

---

## Quick Start

```bash
cd ~/cortex
cx db info          # Verify database (should show ~1803 entities)
cx prime            # Get workflow context
go test ./...       # Run tests before changes
```

## Key Files

| File | Purpose |
|------|---------|
| `internal/cmd/rank.go` | P1 keystones, P2 output order |
| `internal/cmd/near.go` | P2 file mode |
| `internal/cmd/verify.go` | P2 imports |
| `internal/cmd/find.go` | P3 limit flag |

## Session Complete: P0 Bug Fixed

The incremental scan bug (archiving entities outside scan path) was fixed in this session. The fix is in `internal/cmd/scan.go`:
- Uses `FindConfigDir` to locate existing project root
- Scopes `existingEntityIDs` to only entities within the scan path
- Verified: scanning a subdirectory no longer corrupts the database

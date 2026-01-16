# Session Handoff: CX Auto-Exclude Implementation

**Date:** 2026-01-16
**Last Session:** Health check, binary update, .cxignore feature planning
**Branch:** master

## Current Focus

### cortex-z49.7: Auto-exclude dependency directories (Phase 1) - IN PROGRESS

Implement automatic exclusion of dependency directories during `cx scan` with 100% confidence detection only.

**Detection Rules (all verified 100% confidence):**

| Language | Exclude | Condition |
|----------|---------|-----------|
| Rust | `target/` | `Cargo.toml` exists in project |
| Go | `vendor/` | `vendor/modules.txt` exists |
| Python | Any dir with `pyvenv.cfg` | File exists inside directory |
| PHP | `vendor/` | `vendor/autoload.php` exists |
| Node/TS | `node_modules/` | `package.json` exists in project |

**Behavior:**
- Silent by default (just excludes)
- `--verbose`: prints what was auto-excluded
- `--no-auto-exclude`: disables this feature entirely

**Files to modify:**
- `internal/cmd/scan.go` - add pre-scan exclusion logic
- Possibly new: `internal/exclude/autoexclude.go`

**Estimated size:** 100-150 lines of code

## Research Completed

Full language-specific research document created at `docs/cxignore-language-research.md` covering:
- Dependency directories for all 12 supported languages
- Generated code patterns (protobuf, annotation processors, etc.)
- Minification detection heuristics
- Monorepo considerations
- False positive danger zones

## What's NOT in Phase 1 (deferred to Phase 2+)

- Ruby `vendor/bundle` (needs `.bundle/config` parsing)
- C/C++ vcpkg/conan (not 100% confidence)
- Java/Kotlin `target/`/`build/` (could be user directories)
- Generated file header scanning
- Minification detection
- `.cxignore` file support
- `cx suggest-ignore` command

## Session Maintenance Done

1. **Updated cx binary:** Copied `dist/cx-linux-amd64` to `~/.local/bin/cx`
   - Fixed `--keystones` and `--bottlenecks` returning empty results
   - Fixed entity/dependency double-counting bug

2. **Database reset:** Cleared `.cx/` and rescanned with correct counts:
   - Entities: 3,444 (was incorrectly 6,888)
   - Dependencies: 9,493 (was incorrectly 20,895)

## Quick Start for Next Session

```bash
# Context recovery
cx prime
bd show cortex-z49.7

# Understand the current scan implementation
cx safe internal/cmd/scan.go
cx show runScan --related

# Check research doc
cat docs/cxignore-language-research.md | head -100

# Implement the feature
# 1. Add detection functions for each language
# 2. Call before file discovery in scan
# 3. Add --verbose and --no-auto-exclude flags
# 4. Test with real projects

# When done
bd close cortex-z49.7 --reason "Implemented auto-exclude for Rust, Go, Python, PHP, Node"
bd sync
```

## Related Beads

- **cortex-z49:** Smart .cxignore suggestion system (parent feature)
- **cortex-z49.1-6:** Other subtasks (deferred - .cxignore parsing, heuristics, etc.)
- **cortex-z49.7:** This task (Phase 1 auto-exclude) - IN PROGRESS

## Architecture Note

The auto-exclude should happen early in the scan process, before file discovery walks the tree. This avoids even stat-ing files in excluded directories.

```
cx scan
    ↓
detectProjectTypes()     ← Check for Cargo.toml, go.mod, package.json, composer.json
    ↓
buildAutoExcludeList()   ← For each detected type, check signature files
    ↓
walkFiles(excludeList)   ← Pass exclusions to file walker
    ↓
... normal scan ...
```

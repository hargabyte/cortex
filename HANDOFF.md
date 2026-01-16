# Session Handoff: CX Auto-Exclude Nested Projects

**Date:** 2026-01-16
**Last Session:** Created cortex-z49.8 for nested project detection
**Branch:** master

## Current Task: cortex-z49.8 - Auto-exclude nested project directories

### Problem

Auto-exclude only checks root directory for marker files. Nested subprojects aren't detected:

```
project/
  Cargo.toml           ← detected (root)
  target/              ← excluded (works)
  tools/
    svg-generator/
      src-tauri/
        Cargo.toml     ← NOT detected (nested)
        target/        ← NOT excluded (bug)
```

### Solution

Recursively detect marker files at any depth. When found, exclude their sibling dependency directory.

**Detection rules (same as Phase 1, but recursive):**

| Marker File | Exclude Sibling |
|-------------|-----------------|
| `Cargo.toml` | `target/` |
| `package.json` | `node_modules/` |
| `go.mod` + `vendor/modules.txt` | `vendor/` |
| `composer.json` + `vendor/autoload.php` | `vendor/` |
| `pyvenv.cfg` | parent directory |

### Implementation Approach

Modify `internal/exclude/autoexclude.go`:

1. Walk the directory tree looking for marker files
2. When a marker is found, check if sibling dependency dir exists
3. Add to exclude list with path relative to project root
4. Skip already-excluded directories during walk (optimization)

### Files to Modify

- `internal/exclude/autoexclude.go` - Add recursive `filepath.WalkDir`
- `internal/exclude/autoexclude_test.go` - Add nested project test cases

### Out of Scope

User's other complaints are Phase 2+ (need `.cxignore`):
- `tabby-analysis/` - no marker file
- `backup/` - no marker file
- `src-tauri/gen/` - generated code detection

## Previous Work (cortex-z49.7) - COMPLETE

Phase 1 auto-exclude implemented:
- `internal/exclude/autoexclude.go` - root-level detection
- `internal/exclude/autoexclude_test.go` - tests
- `internal/cmd/scan.go` - integration with `--no-auto-exclude` flag

## Quick Start

```bash
# Context recovery
cx prime
bd show cortex-z49.8

# Look at current implementation
cat internal/exclude/autoexclude.go

# Implement recursive detection
# 1. Replace root-only checks with filepath.WalkDir
# 2. Collect marker files at any depth
# 3. Add sibling dependency dirs to exclude list
# 4. Add tests for nested projects

# Test
go test ./internal/exclude/...

# When done
bd close cortex-z49.8 --reason "Added recursive detection for nested project directories"
bd sync
git add .
git commit -m "fix(scan): auto-exclude nested project directories (cortex-z49.8)"
git push
```

## Research Reference

Full language-specific research at `docs/cxignore-language-research.md`

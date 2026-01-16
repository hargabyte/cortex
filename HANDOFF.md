# Session Handoff: CX Auto-Exclude Implementation

**Date:** 2026-01-16
**Last Session:** Implemented cortex-z49.7 auto-exclude feature
**Branch:** master

## Completed: cortex-z49.7 - Auto-exclude dependency directories (Phase 1)

### What Was Implemented

Automatic exclusion of dependency directories during `cx scan` with 100% confidence detection.

**Detection Rules:**

| Language | Exclude | Condition |
|----------|---------|-----------|
| Rust | `target/` | `Cargo.toml` exists AND `target/` exists |
| Go | `vendor/` | `vendor/modules.txt` exists |
| Python | Any dir with `pyvenv.cfg` | Scans all top-level dirs |
| PHP | `vendor/` | `vendor/autoload.php` exists |
| Node/TS | `node_modules/` | `package.json` exists AND `node_modules/` exists |

**Behavior:**
- Silent by default (just excludes)
- `--verbose` / `-v`: prints what was auto-excluded
- `--no-auto-exclude`: disables this feature entirely

### Files Changed

1. **`internal/exclude/autoexclude.go`** (new) - Detection logic
   - `DetectAutoExcludes(projectRoot)` returns directories and reasons
   - Scans top-level directories for Python venvs (not just hardcoded names)
   - Handles Go+PHP monorepo (vendor/ only added once)

2. **`internal/exclude/autoexclude_test.go`** (new) - Test coverage
   - Tests for all 5 ecosystems
   - Edge cases: no target dir, no node_modules, Go+PHP combo

3. **`internal/cmd/scan.go`** - Integration
   - Added `scanNoAutoExclude` flag variable
   - Registered `--no-auto-exclude` flag in `init()`
   - Integrated auto-exclude after exclude merge (lines 127-138)
   - Updated help text with auto-exclude documentation

### Testing Done

- Tested with test repos from `~/cortex-test-repos/`:
  - TypeScript (ts-simple) - node_modules detected
  - Rust (rust-structure) - target detected
  - PHP (php-laravel-quickstart) - vendor detected
  - Go (with vendor/modules.txt) - vendor detected
  - Python (python-mini) - custom venv detected
- Verified `--no-auto-exclude` disables feature
- Verified silent without `-v`
- All unit tests pass

### Binary Updated

```bash
cp dist/cx ~/.local/bin/cx
```

## What's NOT in Phase 1 (deferred)

- Ruby `vendor/bundle` (needs `.bundle/config` parsing)
- C/C++ vcpkg/conan (not 100% confidence)
- Java/Kotlin `target/`/`build/` (could be user directories)
- Generated file header scanning
- Minification detection
- `.cxignore` file support
- `cx suggest-ignore` command

## Next Steps

Close the bead and sync:

```bash
bd close cortex-z49.7 --reason "Implemented auto-exclude for Rust, Go, Python, PHP, Node with --no-auto-exclude flag and verbose output"
bd sync
git add .
git commit -m "feat(scan): auto-exclude dependency directories (cortex-z49.7)"
git push
```

## Related Beads

- **cortex-z49:** Smart .cxignore suggestion system (parent feature)
- **cortex-z49.1-6:** Other subtasks (deferred - .cxignore parsing, heuristics, etc.)
- **cortex-z49.7:** This task - COMPLETE

## Research Reference

Full language-specific research at `docs/cxignore-language-research.md`

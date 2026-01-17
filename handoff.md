# CX Enhancement Session Handoff

**Date**: 2026-01-17
**Session Focus**: cx guide feature - ready for implementation
**Status**: Spec complete, tasks created, **ready to code**

---

## Next Session Prompt

```
Continue implementing cx guide for cortex. Read handoff.md for full context.

The spec is complete and all tasks are created. Your goal is to complete
Phase 1 (Core Infrastructure).

Quick start:
1. cx context                          # Context recovery
2. bd ready | grep cortex-1tp          # See what's unblocked
3. /implement cortex-1tp.1.7           # Start with styles.go

After styles.go is done, d2.go and mermaid.go can be implemented in parallel.

Key decisions from spec:
- Use D2/Mermaid native styling (no custom colors)
- MaxNodes default 30, configurable via --max-nodes
- Auto-collapse to modules when graph is too large
- --render flag calls external d2 CLI (fail gracefully if not installed)

Goal: Complete Phase 1 - tasks .1.7, .1.2, .1.6, .1.8, .1.4
```

---

## Phase 1 Tasks (Core Infrastructure)

| Task | File | Status | Notes |
|------|------|--------|-------|
| cortex-1tp.1.7 | `internal/graph/styles.go` | **READY** | Start here - styling constants |
| cortex-1tp.1.2 | `internal/graph/d2.go` | blocked by .1.7 | D2 format generator |
| cortex-1tp.1.6 | `internal/graph/mermaid.go` | blocked by .1.7 | Mermaid format generator |
| cortex-1tp.1.8 | `internal/cmd/show.go` | blocked by .1.2, .1.6 | Add --format, --output, --max-nodes |
| cortex-1tp.1.4 | `internal/cmd/guide.go` | blocked by .1.8 | Main command + subcommands |
| cortex-1tp.1.9 | `testdata/golden/d2/` | blocked by .1.2 | D2 golden tests |
| cortex-1tp.1.10 | `testdata/golden/mermaid/` | blocked by .1.6 | Mermaid golden tests |

### What styles.go Should Contain

```go
// EntityShapes maps entity types to D2/Mermaid shapes
var EntityShapes = map[string]string{
    "function": "rectangle",
    "method":   "rectangle",
    "struct":   "class",
    "interface": "class",
    // etc.
}

// EdgeStyles maps relationship types to line styles
var EdgeStyles = map[string]string{
    "calls":     "solid",
    "implements": "dashed",
    // etc.
}
```

---

## Task Structure

```
cortex-1tp: cx guide - Human-Readable Codebase Documentation
├── cortex-1tp.1: Guide Core Infrastructure [READY]
│   ├── cortex-1tp.1.7: styles.go [READY] ← START HERE
│   ├── cortex-1tp.1.2: d2.go [blocked by .1.7]
│   ├── cortex-1tp.1.6: mermaid.go [blocked by .1.7]
│   ├── cortex-1tp.1.8: show.go flags [blocked by .1.2, .1.6]
│   ├── cortex-1tp.1.4: guide.go command [blocked by .1.8]
│   ├── cortex-1tp.1.9: D2 golden tests [blocked by .1.2]
│   └── cortex-1tp.1.10: Mermaid golden tests [blocked by .1.6]
├── cortex-1tp.2: Overview Section [blocked by .1] (4 subtasks)
├── cortex-1tp.3: Modules Section [blocked by .1] (3 subtasks)
├── cortex-1tp.4: Hotspots Section [blocked by .1] (5 subtasks)
├── cortex-1tp.5: Dependencies Section [blocked by .1] (4 subtasks)
└── cortex-1tp.7: Integration verification [blocked by .2-.5]
```

### Parallelization Strategy

| Round | Tasks | Agents |
|-------|-------|--------|
| 1 | styles.go | 1 |
| 2 | d2.go, mermaid.go | 2 parallel |
| 3 | show.go flags | 1 |
| 4 | guide.go | 1 |
| 5 | All subcommand subtasks | 16+ parallel |
| 6 | Integration | 1 |

---

## Key Design Decisions (from spec)

1. **Use native D2/Mermaid styling** - no custom colors, let tools handle it
2. **MaxNodes default 30** - configurable via `--max-nodes` flag
3. **Auto-collapse to modules** when graph exceeds MaxNodes
4. **--render flag** calls external `d2` CLI - fail gracefully if not installed
5. **Formats**: d2 (default), mermaid, dot (future)

---

## Quick Reference

### Session Commands

```bash
# Start of session
cx context                  # Context recovery
bd ready                    # What's unblocked
bd show cortex-1tp          # View spec

# Implementation
/implement cortex-1tp.1.7   # Use skill
# OR manually:
bd update cortex-1tp.1.7 --status in_progress

# After completing a task
bd close cortex-1tp.1.7 --reason "Created styling constants"

# End of session
git add <files>
git commit -m "..."
bd sync
git push
```

### Useful Commands

```bash
bd dep tree cortex-1tp.1    # See infrastructure deps
bd blocked | grep cortex-1tp # What's blocked
bd show cortex-1tp.1.7      # View task details
cx safe internal/graph/     # Check before modifying
```

---

## Previous Work (Still Valid)

### File Path Matching (v0.1.6)
- `cx safe` supports flexible path matching
- Suffix, basename, and absolute path resolution

### Still Broken/Hidden
- `cx daemon` / `cx status` (cortex-b44)
- `cx test suggest` / `--affected` (cortex-9dk)

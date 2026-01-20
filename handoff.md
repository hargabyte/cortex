# CX Report Implementation Handoff

**Date**: 2026-01-20
**Session Focus**: Formal specification + task hierarchy creation
**Status**: Spec complete, 42 beads created, **ready for implementation**

---

## Next Session Prompt

```
Implement CX Report starting with the schema foundation.

## Context
Read handoff.md for complete context. The formal specification is complete and
42 beads have been created with proper dependencies. The architecture uses an
"agent-writes-reports" model where you (the AI agent) generate narratives from
structured YAML data - no API key required.

## Your Goal
Start implementing R0.1 (Core Schema Types) - the foundation for all reports.

## Quick Start
1. Read handoff.md for full context
2. Read docs/specs/CX_REPORT_SPEC.md for the formal specification
3. bd show cortex-dkd.8.1 for the first task details
4. /implement cortex-dkd.8.1 to begin

## Architecture Summary

The CLI outputs structured YAML data via `--data` flag:
  cx report feature "auth" --data  →  YAML with entities, D2 code, coverage

You (the AI agent) read this data and generate:
  - Narrative prose explaining the feature
  - Final HTML report with embedded diagrams

No Anthropic API key needed - you ARE the AI that writes the reports.

## Ready Tasks
cortex-dkd.8.1: R0.1: Core Schema Types [P1, READY] ← START HERE

After R0.1, these become ready:
- cortex-dkd.8.2-5: Report-specific schemas (parallel)
- Then R1, R2 foundation tasks unlock

## Key Files
- handoff.md - This file (comprehensive context)
- docs/specs/CX_REPORT_SPEC.md - Formal specification (1100+ lines)
- internal/cmd/guide.go - Existing code to replace
- internal/graph/d2.go - Existing D2 generation
- internal/store/fts.go - Existing FTS search
- internal/store/embeddings.go - Embedding search

## Implementation Notes
- Create new package: internal/report/
- Schema types go in: internal/report/schema.go
- Follow existing patterns from internal/store/ and internal/output/
```

---

## Session Summary

### What We Accomplished This Session

1. **Wrote Formal Specification** (docs/specs/CX_REPORT_SPEC.md)
   - 1100+ lines of detailed specification
   - Agent-writes-reports architecture (no API key)
   - YAML data contracts for all 4 report types
   - D2 visual design system
   - HTML template structure
   - Molecule architecture for parallel implementation

2. **Created Task Hierarchy** (42 beads total)
   - 31 new subtasks across 7 parent tasks
   - Proper dependency graph for parallelization
   - Ready queue starts with R0.1 (Core Schema Types)

3. **Architecture Decision: Agent-Writes-Reports**
   - CLI outputs structured YAML via `--data` flag
   - AI agent (you) generates narratives and assembles HTML
   - No Anthropic API key required
   - Works in any Claude Code session

4. **Closed Obsolete Tasks**
   - cortex-dkd.3 (R3: AI Narrative) - architecture changed
   - cortex-dkd.5.1 (R5.1: Requirements) - captured in spec

---

## Complete Epic Structure

```
cortex-dkd (P1 epic) CX 3.0: Report Generation [spec-complete, tasks-created]
│
├── cortex-dkd.8 (P1) R0: Data Output Schema ← READY
│   ├── cortex-dkd.8.1 R0.1: Core Schema Types ← START HERE
│   ├── cortex-dkd.8.2 R0.2: Feature Report Schema [blocked by .8.1]
│   ├── cortex-dkd.8.3 R0.3: Overview Report Schema [blocked by .8.1]
│   ├── cortex-dkd.8.4 R0.4: Changes Report Schema [blocked by .8.1]
│   └── cortex-dkd.8.5 R0.5: Health Report Schema [blocked by .8.1]
│
├── cortex-dkd.1 (P1) R1: Report Engine Core [blocked by R0]
│   ├── cortex-dkd.1.1 R1.1: Command Scaffolding
│   ├── cortex-dkd.1.2 R1.2: Data Gathering [blocked by .1.1]
│   ├── cortex-dkd.1.3 R1.3: YAML/JSON Output [blocked by .1.1]
│   └── cortex-dkd.1.4 R1.4: Output Handling [blocked by .1.1]
│
├── cortex-dkd.2 (P1) R2: D2 Diagram Integration [blocked by R0]
│   ├── cortex-dkd.2.1 R2.1: D2 Visual Design System
│   ├── cortex-dkd.2.2 R2.2: D2 Code Generator
│   ├── cortex-dkd.2.3 R2.3: Architecture Preset [blocked by .2.2]
│   ├── cortex-dkd.2.4 R2.4: Call Flow Preset [blocked by .2.2]
│   └── cortex-dkd.2.5 R2.5: Render Command [blocked by .2.2]
│
├── cortex-dkd.3 (P1) R3: AI Narrative ← CLOSED (architecture changed)
│
├── cortex-dkd.4 (P2) R4: Overview Report [blocked by R1, R2.1]
│   ├── cortex-dkd.4.1 R4.1: Statistics Gathering
│   ├── cortex-dkd.4.2 R4.2: Keystone Extraction
│   ├── cortex-dkd.4.3 R4.3: Module Structure
│   └── cortex-dkd.4.4 R4.4: Architecture D2 [blocked by .4.3]
│
├── cortex-dkd.5 (P2) R5: Feature Report [blocked by R1, R2.1]
│   ├── cortex-dkd.5.1 R5.1: Requirements ← CLOSED (in spec)
│   ├── cortex-dkd.5.2 R5.1: Hybrid Search
│   ├── cortex-dkd.5.3 R5.2: Entity Ranking [blocked by .5.2]
│   ├── cortex-dkd.5.4 R5.3: Dependency Traversal [blocked by .5.2]
│   ├── cortex-dkd.5.5 R5.4: Call Flow D2 [blocked by .5.4]
│   └── cortex-dkd.5.6 R5.5: Test Association
│
├── cortex-dkd.6 (P2) R6: Change Report [blocked by R1, R2.1]
│   ├── cortex-dkd.6.1 R6.1: Dolt Diff Queries
│   ├── cortex-dkd.6.2 R6.2: Entity Comparison [blocked by .6.1]
│   ├── cortex-dkd.6.3 R6.3: Impact Analysis [blocked by .6.2]
│   └── cortex-dkd.6.4 R6.4: Before/After D2 [blocked by .6.2]
│
└── cortex-dkd.7 (P2) R7: Health Report [blocked by R1, R2.1]
    ├── cortex-dkd.7.1 R7.1: Risk Score
    ├── cortex-dkd.7.2 R7.2: Untested Keystones
    ├── cortex-dkd.7.3 R7.3: Dead Code
    ├── cortex-dkd.7.4 R7.4: Circular Dependencies
    └── cortex-dkd.7.5 R7.5: Complexity Hotspots
```

---

## Agent-Writes-Reports Architecture

### The Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                         USER IN CLAUDE CODE                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  User: "Generate a report on authentication"                         │
│                                                                      │
│  Claude Code:                                                        │
│    1. Runs: cx report feature "auth" --data                          │
│    2. Receives: Structured YAML with entities, D2 code, coverage     │
│    3. Writes: Narrative prose explaining the feature                 │
│    4. Assembles: Final HTML report with embedded diagrams            │
│    5. Optionally: Runs cx render to convert D2 → SVG                 │
│                                                                      │
│  Result: auth-report.html                                            │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Why This Design?

| Approach | Pros | Cons |
|----------|------|------|
| **Embedded API Key** | Works standalone | Requires ANTHROPIC_API_KEY, limits adoption |
| **Agent-Writes-Reports** | No API key, uses conversation context | Requires AI agent session |

CX is designed for AI agents - the agent-writes-reports model is natural.

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| `cx report <type> --data` | Gather data, generate D2 code, output YAML |
| AI Agent (Claude Code) | Interpret data, write narratives, assemble HTML |
| `cx render <file>` | Convert D2 code blocks to embedded SVG (optional) |

---

## Data Contract Summary

All `--data` commands output YAML with this structure:

```yaml
report:
  type: feature|overview|changes|health
  generated_at: "2026-01-20T15:30:00Z"
  query: "authentication"  # For feature reports

metadata:
  entity_count: 1234
  language_breakdown:
    go: 800
    typescript: 400

entities:
  - id: "sa-fn-abc123-45-LoginUser"
    name: LoginUser
    type: function
    file: internal/auth/login.go
    lines: [45, 89]
    signature: "func LoginUser(...) (*User, error)"
    importance: keystone
    pagerank: 0.0234
    coverage: 85.5

dependencies:
  - from: LoginUser
    to: ValidateToken
    type: calls

diagrams:
  call_flow:
    title: "Authentication Call Flow"
    d2: |
      direction: down
      # ... D2 code
```

See full schemas in docs/specs/CX_REPORT_SPEC.md

---

## Implementation Phases

### Phase 1: Foundation (R0)
**Goal**: Define all data schemas

| Task | Description | Status |
|------|-------------|--------|
| R0.1 | Core schema types (ReportData, EntityData, etc.) | READY |
| R0.2 | Feature report schema | Blocked by R0.1 |
| R0.3 | Overview report schema | Blocked by R0.1 |
| R0.4 | Changes report schema | Blocked by R0.1 |
| R0.5 | Health report schema | Blocked by R0.1 |

### Phase 2: Engine (R1, R2)
**Goal**: CLI commands and D2 generation

| Task | Description | Status |
|------|-------------|--------|
| R1.1 | Command scaffolding (`cx report`) | Blocked by R0 |
| R1.2-4 | Data gathering, YAML output | Blocked by R1.1 |
| R2.1 | D2 Visual Design System | Blocked by R0 |
| R2.2 | D2 Code Generator | Blocked by R0 |
| R2.3-5 | Diagram presets, render command | Blocked by R2.2 |

### Phase 3: Reports (R4-R7)
**Goal**: Individual report implementations

| Task | Description | Status |
|------|-------------|--------|
| R4.* | Overview report (4 subtasks) | Blocked by R1, R2.1 |
| R5.* | Feature report (5 subtasks) | Blocked by R1, R2.1 |
| R6.* | Change report (4 subtasks) | Blocked by R1, R2.1 |
| R7.* | Health report (5 subtasks) | Blocked by R1, R2.1 |

### Parallelization Potential

| Phase | Tasks | Max Parallel Agents |
|-------|-------|---------------------|
| 1 | R0.1 | 1 |
| 2 | R0.2-R0.5 | 4 |
| 3 | R1.1, R2.1, R2.2 | 3 |
| 4 | R1.2-R1.4, R2.3-R2.5 | 6 |
| 5 | R4.*, R5.*, R6.*, R7.* | 18 |

---

## D2 Visual Design System

### Entity Type Styling

| Entity Type | Shape | Color | Icon |
|-------------|-------|-------|------|
| Function | rectangle | `#e3f2fd` (light blue) | tech/go.svg |
| Method | rectangle | `#e8f5e9` (light green) | essentials/gear.svg |
| Type/Struct | rectangle | `#f3e5f5` (light purple) | essentials/package.svg |
| Interface | rectangle | `#fff3e0` (light orange) | essentials/plug.svg |
| Database | cylinder | `#eceff1` (light gray) | azure/Databases/*.svg |

### Importance Styling

```d2
# Keystone - prominent
keystone: {
  style: {
    stroke-width: 3
    shadow: true
    fill: "#fff3e0"
    stroke: "#f57c00"
  }
}

# Bottleneck - warning
bottleneck: {
  style: {
    stroke-width: 2
    fill: "#fff8e1"
    stroke: "#ffa000"
  }
}
```

### Coverage Indicators

| Coverage | Color | Style |
|----------|-------|-------|
| >80% | Green | `#c8e6c9` fill |
| 50-80% | Yellow | `#fff9c4` fill |
| <50% | Red | `#ffcdd2` fill |

---

## Existing Code to Leverage

| Component | Location | Purpose |
|-----------|----------|---------|
| FTS Search | [internal/store/fts.go](internal/store/fts.go) | SearchEntities with FULLTEXT |
| Embeddings | [internal/store/embeddings.go](internal/store/embeddings.go) | FindSimilar for semantic search |
| D2 Generation | [internal/graph/d2.go](internal/graph/d2.go) | GenerateD2, sanitizeD2ID |
| Guide Command | [internal/cmd/guide.go](internal/cmd/guide.go) | Stats, module analysis (replace) |
| Graph | [internal/graph/graph.go](internal/graph/graph.go) | Dependency traversal |
| Coverage | [internal/store/coverage.go](internal/store/coverage.go) | Coverage data |

### Key Functions to Reuse

```go
// From fts.go
func (s *Store) SearchEntities(opts SearchOptions) ([]*SearchResult, error)

// From embeddings.go
func (s *Store) FindSimilar(entityID string, limit int) ([]*SimilarEntity, error)

// From d2.go
func GenerateD2(nodes map[string]*output.GraphNode, edges [][]string, opts *D2Options) string

// From guide.go
func getEntityTypeCounts(storeDB *store.Store) (map[string]int, error)
func extractModuleFromPath(filePath string) string
func computeModuleEdges(entities []*store.Entity, g *graph.Graph, storeDB *store.Store) [][]string
```

---

## Commands Reference

### Beads Commands
```bash
bd show cortex-dkd           # View epic
bd show cortex-dkd.8.1       # View first task to implement
bd dep tree cortex-dkd       # Dependency visualization
bd ready | grep cortex-dkd   # What's unblocked
bd list --parent cortex-dkd  # All tasks in epic
```

### Cortex Commands
```bash
cx context                              # Session orientation
cx context --smart "report schema"      # Task-focused context
cx safe internal/report/schema.go       # Before creating new file
cx find SearchResult                    # Find existing patterns
cx show Store.SearchEntities --related  # Understand dependencies
```

### Implementation Commands
```bash
# Start implementation
/implement cortex-dkd.8.1

# Or manually
bd update cortex-dkd.8.1 --status in_progress
# ... do the work ...
bd close cortex-dkd.8.1 --reason "Created core schema types"
```

---

## Files Reference

| File | Purpose |
|------|---------|
| `handoff.md` | This file - comprehensive context |
| `docs/specs/CX_REPORT_SPEC.md` | Formal specification (1100+ lines) |
| `internal/cmd/guide.go` | Existing code being replaced |
| `internal/graph/d2.go` | Existing D2 generation |
| `internal/store/fts.go` | FTS search implementation |
| `internal/store/embeddings.go` | Embedding search |
| `internal/context/smart.go` | Entry point detection |

### New Files to Create

| File | Purpose |
|------|---------|
| `internal/report/schema.go` | Core schema types |
| `internal/report/feature.go` | Feature report data |
| `internal/report/overview.go` | Overview report data |
| `internal/report/changes.go` | Changes report data |
| `internal/report/health.go` | Health report data |
| `internal/cmd/report.go` | Report command |
| `internal/cmd/render.go` | Render command |

---

## Session End Checklist

```
[x] 1. git status              (checked)
[x] 2. git add <files>         (staged)
[x] 3. bd sync                 (synced)
[x] 4. git commit              (committed)
[x] 5. git push                (pushed)
```

### Commits This Session

| Hash | Message |
|------|---------|
| fc76f93 | Add CX Report formal specification |
| b72e2f2 | Add CX Report task hierarchy (31 subtasks) |

---

## Previous Context (Still Valid)

### Semantic Search
- FTS5 implementation complete and working
- Embeddings infrastructure in place (Hugot + HuggingFace)
- Hybrid search ready to implement

### Dolt Migration
- Complete - CX now uses Dolt
- Time-travel queries available for change reports
- `AS OF` queries supported

### D2 CLI Integration
- Existing implementation in internal/cmd/show.go:868-922
- Discovery with fallback to ~/.local/bin/d2
- ASCII and SVG rendering supported

### Still Broken/Hidden
- `cx daemon` / `cx status` (cortex-b44)
- `cx test suggest` / `--affected` (cortex-9dk)

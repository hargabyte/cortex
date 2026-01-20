# CX Report Specification Session Handoff

**Date**: 2026-01-20
**Session Focus**: Write formal specification for CX Report feature
**Status**: Requirements gathered, research complete, **ready for spec writing**

---

## Next Session Prompt

```
Write the formal specification for CX Report (cortex-dkd).

## Context
Read handoff.md for complete context. This session gathered extensive requirements
through /shape-spec and researched D2 capabilities for stunning visualizations.

## Your Goal
Run `/write-spec cortex-dkd.5` to create the formal technical specification for the
Feature Report (R5). Then extend to the other report types.

## Key Background

### What is CX Report?
A new command replacing `cx guide` that generates AI-powered, publication-quality
reports about codebases with rich D2 visualizations.

### Why This Matters
- CX is a backend tool - reports are the main VISUAL output for demos/marketing
- User explicitly wants "showcase-worthy" diagrams that look professional
- Reports should be "living documents" that get updated, not regenerated

### Core Report Types
1. `cx report overview` - System summary + architecture diagram
2. `cx report feature <query>` - Semantic search + deep dive (PRIMARY FOCUS)
3. `cx report changes --since <ref>` - Dolt time-travel history
4. `cx report health` - Risk analysis + recommendations

## Quick Start
1. Read handoff.md thoroughly - it has all requirements and research
2. Read docs/specs/REPORT_SPEC.md for initial spec draft
3. bd show cortex-dkd.5.1 for detailed Feature Report requirements
4. /write-spec cortex-dkd.5 to begin formal specification

## Key Files
- handoff.md - This file (comprehensive context)
- docs/specs/REPORT_SPEC.md - Initial spec draft
- internal/cmd/guide.go - Existing code to replace
- internal/graph/d2.go - Existing D2 generation
- internal/store/fts.go - Existing FTS search

## Design Decisions Already Made
- HTML primary output with full interactivity
- D2 primary diagrams (Mermaid fallback)
- AI narrative via Anthropic API (you write the reports)
- Reports as living knowledge base (incremental updates)
- Diagrams must be visually stunning (icons, themes, professional styling)
```

---

## Session Summary

### What We Accomplished

1. **Evaluated Beads Integration** (cortex-u1n)
   - Deep analysis of integration options
   - Conclusion: Low ROI, deprioritized to P4
   - Git already tracks code→issue relationships via commit messages

2. **Created CX Report Epic** (cortex-dkd)
   - New P1 epic replacing cx guide
   - 7 tasks created with proper dependencies
   - Detailed spec draft in docs/specs/REPORT_SPEC.md

3. **Shaped Feature Report Requirements** (cortex-dkd.5)
   - Ran /shape-spec interview process
   - Documented all decisions in cortex-dkd.5.1
   - Created visual design system task (cortex-dkd.2.1)

4. **Researched D2 Capabilities**
   - Professional themes and styling
   - Icon library at icons.terrastruct.com
   - TALA layout engine for architecture diagrams

---

## CX Report Epic Structure

```
cortex-dkd (P1 epic) CX 3.0: Report Generation
├── cortex-dkd.1 (P1) R1: Report Engine Core
│   └── HTML/MD writers, templates, section abstractions
├── cortex-dkd.2 (P1) R2: D2 Diagram Integration
│   └── cortex-dkd.2.1 (P1) R2.1: D2 Visual Design System ← CRITICAL FOR QUALITY
├── cortex-dkd.3 (P1) R3: AI Narrative Generation
│   └── Anthropic API, prompts, caching
├── cortex-dkd.4 (P2) R4: Overview Report [blocked by .1,.2,.3]
├── cortex-dkd.5 (P2) R5: Feature Report [blocked by .1,.2,.3]
│   └── cortex-dkd.5.1 (P2) R5.1: Feature Report Requirements ← DETAILED SPEC
├── cortex-dkd.6 (P2) R6: Change Report [blocked by .1,.2,.3]
└── cortex-dkd.7 (P2) R7: Health Report [blocked by .1,.2,.3]
```

---

## Feature Report (R5) - Detailed Requirements

### Use Case
**Documentation generation** - Auto-generate and maintain feature docs that stay current with code

### User Decisions from Interview

| Category | Decision |
|----------|----------|
| **Query Input** | Natural language via semantic search |
| **Output Format** | HTML primary with full interactivity |
| **Narrative Depth** | Technical deep-dive with code references |
| **Diagram Types** | Call flow + data flow + AI-selected relevant types |
| **Scale Handling** | Interactive HTML (collapsible, searchable, clickable) |
| **Test Coverage** | Show related tests with coverage % |
| **Search Strategy** | FTS5 + embedding hybrid |
| **AI Context** | Flexible - gather what's needed, NEVER guess |
| **Narrative Structure** | Entry point → flow → output (guided flexibility) |
| **Diagram Generation** | Entry point detection + BFS (with AI discretion) |
| **Caching** | Cache AI narratives by content hash |
| **Performance** | < 1 minute, background generation with progress |
| **Integration** | Replaces cx guide |
| **Multi-language** | Unified view |
| **Gap Handling** | Acknowledge gaps explicitly, don't pretend |
| **Update Model** | Living documents - incremental updates, not regeneration |

### Report Sections (Feature Report)
1. Feature Overview - AI explains what this feature does
2. Key Entities - Found via semantic search, ranked by relevance
3. Call Flow Diagram - D2 sequence-style or flowchart
4. Data Flow Diagram - Inputs → processing → outputs
5. Related Tests - Test entities linked to feature
6. Coverage Analysis - Percentage and gaps

### HTML Interactivity Requirements
- Collapsible sections (expand/collapse diagram nodes, entity details)
- Filter/search within report (client-side search)
- Clickable diagram nodes (jump to detail section)
- Self-contained single HTML file (embedded CSS/JS/SVG)

---

## Visual Design Requirements (CRITICAL)

### Goal
**Diagrams must be showcase-worthy** - professional quality for:
- Marketing materials and demos
- Technical blog posts
- Company documentation
- Conference presentations

### D2 Capabilities to Leverage

**Professional Themes:**
- Built-in: Grape Soda, Mixed Berry Blue, Vanilla Nitro Cola, Terminal
- Dark mode variants
- Custom theme overrides supported

**Styling Options:**
```d2
shape: {
  style: {
    fill: "#4A90D9"        # Background color
    stroke: "#2E5A8B"      # Border color
    stroke-width: 2        # Border thickness
    shadow: true           # Drop shadow
    3d: true               # 3D effect (rectangles only)
    border-radius: 8       # Rounded corners
    opacity: 0.9           # Transparency
    fill-pattern: dots     # Pattern fills
  }
}
```

**Icon Library (icons.terrastruct.com):**
- AWS: 500+ service icons
- GCP: Full Google Cloud coverage
- Azure: Microsoft Azure resources
- Development: Docker, Git, React, Python, Go, Rust, etc.
- Infrastructure: Servers, databases, networks, firewalls
- Essentials: UI elements, shapes, symbols

**Entity Type Icon Mapping (Proposed):**

| Entity Type | Icon | Color Scheme |
|-------------|------|--------------|
| Function | ⚡ function/lightning | Blue |
| Type/Struct | 📦 box/package | Purple |
| Method | 🔧 gear/tool | Teal |
| Interface | 🔌 plug/connector | Orange |
| Constant | 📌 pin | Gray |
| Test | ✅ checkmark | Green |
| Database | 🗄️ cylinder | Navy |
| HTTP Handler | 🌐 globe | Cyan |
| CLI Command | 💻 terminal | Dark |

**Importance Styling:**
- Keystone: Bold border, subtle glow/shadow, larger size
- Bottleneck: Warning color accent, thicker border
- Normal: Standard styling
- Leaf: Lighter/smaller styling

**Coverage Indicators:**
- High (>80%): Green tint/badge
- Medium (50-80%): Yellow tint/badge
- Low (<50%): Red tint/badge
- No tests: Gray with warning icon

**TALA Layout Engine:**
- Premium layout specifically for software architecture
- Cleaner orthogonal lines
- Better container handling
- May require separate install

---

## Existing Codebase Assets

### Semantic Search (FTS5)
**Location:** `internal/store/fts.go`

```go
type SearchResult struct {
    Entity        *Entity
    FTSScore      float64  // Raw FULLTEXT match score
    PageRank      float64  // From metrics
    CombinedScore float64  // Weighted combination
    MatchColumn   string   // name, body_text, doc_comment, file_path
}

type SearchOptions struct {
    Query          string
    Limit          int
    Threshold      float64
    Language       string
    EntityType     string
    BoostPageRank  float64  // Default: 0.3
    BoostFTS       float64  // Default: 0.7
    BoostExactName float64  // Default: 2.0
}

func (s *Store) SearchEntities(opts SearchOptions) ([]*SearchResult, error)
```

### D2 Generation
**Location:** `internal/graph/d2.go`

```go
type D2Options struct {
    MaxNodes   int     // Default: 30
    Direction  string  // "right" or "down"
    ShowLabels bool
    Collapse   bool    // Auto-collapse to modules
    Title      string
}

func GenerateD2(nodes map[string]*output.GraphNode, edges [][]string, opts *D2Options) string
func generateD2Node(id, name, entityType, importance string) string
func generateD2Edge(from, to, edgeType string, showLabel bool) string
```

### Smart Context / Entry Points
**Location:** `internal/context/smart.go`

- `findEntryPoints()` - Detects high in-degree entities as entry points
- Context assembly from entities + dependencies

### Existing Guide Command
**Location:** `internal/cmd/guide.go`

Subcommands to replace:
- `cx guide overview` → `cx report overview`
- `cx guide hotspots` → `cx report health` (partially)
- `cx guide modules` → `cx report overview` (architecture section)
- `cx guide deps` → `cx report health` (cycle detection)

### Embeddings
**Locations:**
- `internal/store/embeddings.go`
- `internal/embeddings/embeddings.go`

Vector embeddings for semantic search (FTS5 + embedding hybrid).

---

## Research Findings

### Beads Integration Analysis (Deprioritized)

**Conclusion:** Low ROI - git commit messages already link code to issues.

**Options Evaluated:**
1. Separate DBs + linking table (original plan)
2. Single shared Dolt database
3. Dolt foreign database queries
4. Cortex as super-database
5. Event-driven integration
6. MCP server integration

**Why Separate DBs Won:**
- Different rhythms (CX scans constantly, BD syncs occasionally)
- Different branching semantics
- Schema ownership conflicts in shared DB
- Tools should work independently

**If We Ever Revisit:**
- Beads already has Dolt backend (experimental, 18 files)
- `bd init --backend dolt` enables it
- Could share Dolt remote in future

### D2 vs Mermaid

**D2 Advantages:**
- TALA layout engine (architecture-specific)
- Better styling options (shadows, 3D, gradients)
- Icon library (500+ icons)
- Container support
- Professional themes

**Mermaid Advantages:**
- Works in GitHub/GitLab markdown
- No CLI dependency
- Wider tool support

**Decision:** D2 primary, Mermaid fallback

### AI Narrative Approach

**Structure Baseline:**
1. Entry point(s) - how is this feature triggered?
2. Flow through key entities
3. Data transformations
4. Error handling/edge cases

**Flexibility Principles:**
- AI adapts structure based on feature complexity
- Can request additional context when needed
- Quality over speed - never guess when uncertain
- Acknowledge gaps explicitly

**Context Assembly:**
- Start with: entity metadata + signatures + dependencies
- Expand as needed: code snippets, docstrings, existing docs
- Principle: More context is better than guessing

---

## Commands Reference

### Beads Commands
```bash
bd show cortex-dkd           # View epic
bd show cortex-dkd.5.1       # View Feature Report requirements
bd dep tree cortex-dkd       # Dependency visualization
bd ready | grep cortex-dkd   # What's unblocked
```

### Cortex Commands
```bash
cx context                              # Session orientation
cx context --smart "report generation"  # Task-focused context
cx safe internal/cmd/guide.go           # Before modifying
cx find SearchEntities                  # Find existing code
cx show SearchEntities --related        # Understand dependencies
```

### Useful Queries
```bash
# Find all report-related code
cx find "report" --type F

# Check FTS implementation
cx show SearchEntities --graph --hops 2

# See guide.go structure
cx show runGuideOverview --related
```

---

## Files to Read

| File | Purpose |
|------|---------|
| `handoff.md` | This file - complete context |
| `docs/specs/REPORT_SPEC.md` | Initial spec draft |
| `internal/cmd/guide.go` | Code being replaced |
| `internal/graph/d2.go` | Existing D2 generation |
| `internal/store/fts.go` | Existing FTS search |
| `internal/context/smart.go` | Entry point detection |

---

## Session End Checklist

```
[ ] 1. git status              (check what changed)
[ ] 2. git add <files>         (stage changes)
[ ] 3. bd sync                 (commit beads)
[ ] 4. git commit -m "..."     (commit code)
[ ] 5. bd sync                 (any new beads)
[ ] 6. git push                (push to remote)
```

---

## Previous Work (Still Valid)

### cx guide (cortex-1tp)
- Basic implementation exists in guide.go
- Being replaced by cx report
- Some code may be reusable (table formatting, stats gathering)

### Semantic Search
- FTS5 implementation complete and working
- Embeddings infrastructure in place

### Dolt Migration
- Complete - CX now uses Dolt
- Time-travel queries available for change reports

### File Path Matching (v0.1.6)
- `cx safe` supports flexible path matching
- Suffix, basename, and absolute path resolution

### Still Broken/Hidden
- `cx daemon` / `cx status` (cortex-b44)
- `cx test suggest` / `--affected` (cortex-9dk)

# CX Report Specification

> **Epic**: cortex-dkd | **Priority**: P1 | **Status**: Spec Complete

Generate AI-powered, publication-quality codebase reports with rich D2 visualizations.

---

## Goal

Transform code intelligence data into polished, narrative-driven reports that explain how systems work, how they've evolved, and where the risks are. **AI agents generate the narratives** using structured data from the CLI, eliminating the need for API keys.

---

## User Stories

- As a developer, I want to generate a report explaining how authentication works so I can onboard quickly
- As a tech lead, I want an overview report to share with stakeholders showing our architecture
- As a maintainer, I want a change report showing what evolved since the last release
- As a team lead, I want a health report identifying untested critical code

---

## Architecture: Agent-Writes-Reports Model

### Philosophy

CX is designed for AI agents. The CLI provides **structured data**, the agent provides **narrative intelligence**.

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

Since CX is designed for AI agents, the agent-writes-reports model is natural and eliminates friction.

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| `cx report <type> --data` | Gather data, generate D2 code, output YAML |
| AI Agent (Claude Code) | Interpret data, write narratives, assemble HTML |
| `cx render <file>` | Convert D2 code blocks to embedded SVG (optional) |

---

## Molecule Architecture

### Dependency Overview

```
                    ┌─────────────────────────────────────┐
                    │  R0: Data Output Schema Definition  │
                    └──────────────────┬──────────────────┘
                                       │
         ┌─────────────────────────────┼─────────────────────────────┐
         │                             │                             │
         ▼                             ▼                             ▼
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│ R1: Report      │         │ R2: D2 Diagram  │         │ R2.1: Visual    │
│ Engine Core     │         │ Integration     │         │ Design System   │
└────────┬────────┘         └────────┬────────┘         └────────┬────────┘
         │                           │                           │
         └───────────┬───────────────┴───────────────────────────┘
                     │
    ┌────────────────┼────────────────┬────────────────┐
    │                │                │                │
    ▼                ▼                ▼                ▼
┌────────┐    ┌────────┐    ┌────────┐    ┌────────┐
│ R4:    │    │ R5:    │    │ R6:    │    │ R7:    │
│Overview│    │Feature │    │Change  │    │Health  │
│Report  │    │Report  │    │Report  │    │Report  │
└────────┘    └────────┘    └────────┘    └────────┘
```

### Work Streams

#### Stream 1: Foundation (Sequential)
**Tasks**: R0 → R1 → R2 → R2.1
**Agent Type**: Backend specialist
**Output**: Data schema, YAML writers, D2 generation, visual design

#### Stream 2: Report Types (Parallel after Foundation)
**Tasks**: R4, R5, R6, R7 (can run in parallel)
**Agent Type**: Backend specialists (one per report)
**Dependencies**: All depend on R1, R2

### Critical Path

```
R0 → R1 → R2 → R2.1 → R5 (Feature Report as reference implementation)
                    ↘ R4, R6, R7 (parallel with R5)
```

### Parallelization Opportunities

| Phase | Tasks | Agents |
|-------|-------|--------|
| 1 | R0 (schema) | 1 |
| 2 | R1, R2 (engine + D2) | 2 parallel |
| 3 | R2.1 (visual design) | 1 |
| 4 | R4, R5, R6, R7 (all reports) | 4 parallel |

---

## Commands

### cx report overview --data

System-level summary with architecture diagram.

```bash
cx report overview --data                    # YAML to stdout
cx report overview --data -o overview.yaml   # Write to file
cx report overview --data --format json      # JSON output
```

### cx report feature \<query\> --data

Deep-dive into a specific feature using semantic search.

```bash
cx report feature "authentication" --data
cx report feature "payment processing" --data
cx report feature --entity LoginUser --depth 3 --data
```

### cx report changes --since \<ref\> --data

What changed between two points in time (Dolt time-travel).

```bash
cx report changes --since HEAD~50 --data
cx report changes --since v1.0 --until v2.0 --data
cx report changes --since 2026-01-01 --data
```

### cx report health --data

Risk analysis and recommendations.

```bash
cx report health --data
cx report health --focus coverage --data
cx report health --focus complexity --data
```

### cx render \<file\>

Convert D2 code blocks in HTML to embedded SVG.

```bash
cx render report.html                    # In-place rendering
cx render report.html -o rendered.html   # Output to new file
cx render --check                        # Check if D2 CLI available
```

---

## Data Contracts

### Common Output Structure

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
  coverage_available: true

sections:
  # Report-type-specific sections
```

### Feature Report Data Schema

```yaml
report:
  type: feature
  generated_at: "2026-01-20T15:30:00Z"
  query: "authentication"

metadata:
  total_entities_searched: 3648
  matches_found: 15
  search_method: hybrid  # fts + embedding

entities:
  - id: "sa-fn-abc123-45-LoginUser"
    name: LoginUser
    type: function
    file: internal/auth/login.go
    lines: [45, 89]
    signature: "func LoginUser(ctx context.Context, email, password string) (*User, error)"
    importance: keystone
    pagerank: 0.0234
    coverage: 85.5
    doc_comment: "LoginUser authenticates a user by email and password"
    relevance_score: 0.95

  - id: "sa-fn-def456-12-ValidateToken"
    name: ValidateToken
    type: function
    file: internal/auth/token.go
    lines: [12, 45]
    signature: "func ValidateToken(token string) (*Claims, error)"
    importance: bottleneck
    pagerank: 0.0189
    coverage: 92.0
    relevance_score: 0.88

dependencies:
  - from: LoginUser
    to: ValidateToken
    type: calls
  - from: LoginUser
    to: UserStore.GetByEmail
    type: calls
  - from: ValidateToken
    to: SessionCache.Get
    type: calls

diagrams:
  call_flow:
    title: "Authentication Call Flow"
    d2: |
      direction: down

      1.Request: {
        shape: oval
        label: "HTTP Request"
        icon: https://icons.terrastruct.com/essentials%2F112-server.svg
      }

      2.Handler: {
        label: "LoginHandler"
        icon: https://icons.terrastruct.com/tech%2Fgo.svg
        style: {
          fill: "#e3f2fd"
          stroke: "#1976d2"
          stroke-width: 2
        }
      }

      3.Auth: {
        label: "LoginUser"
        style: {
          fill: "#fff3e0"
          stroke: "#f57c00"
          stroke-width: 3
          shadow: true
        }
      }

      4.Store: {
        label: "UserStore"
        shape: cylinder
        icon: https://icons.terrastruct.com/azure%2FDatabases%2F10121-icon-service-Azure-Database-PostgreSQL-Server.svg
      }

      5.Response: {
        shape: oval
        label: "JWT Token"
      }

      1.Request -> 2.Handler: "POST /login"
      2.Handler -> 3.Auth: "authenticate()"
      3.Auth -> 4.Store: "GetByEmail()"
      4.Store -> 3.Auth: "User"
      3.Auth -> 2.Handler: "token"
      2.Handler -> 5.Response

  data_flow:
    title: "Authentication Data Flow"
    d2: |
      direction: right
      # ... data flow D2 code

tests:
  - name: TestLoginUser_Success
    file: internal/auth/login_test.go
    lines: [15, 45]
    covers: [LoginUser]

  - name: TestLoginUser_InvalidPassword
    file: internal/auth/login_test.go
    lines: [47, 78]
    covers: [LoginUser]

coverage:
  overall: 78.5
  by_entity:
    LoginUser: 85.5
    ValidateToken: 92.0
    SessionCache.Get: 45.0
  gaps:
    - entity: SessionCache.Get
      coverage: 45.0
      importance: bottleneck
      risk: high
```

### Overview Report Data Schema

```yaml
report:
  type: overview
  generated_at: "2026-01-20T15:30:00Z"

metadata:
  total_entities: 3648
  active_entities: 3608
  archived_entities: 40

statistics:
  by_type:
    function: 1500
    method: 800
    type: 400
    constant: 200
    variable: 100
  by_language:
    go: 2500
    typescript: 1000
    python: 148
  trends:  # If history available
    - date: "2026-01-01"
      entities: 3400
    - date: "2026-01-15"
      entities: 3648

keystones:
  - id: "sa-fn-abc123"
    name: Store.GetEntity
    type: method
    file: internal/store/entity.go
    pagerank: 0.0456
    in_degree: 89
    coverage: 95.0

  - id: "sa-fn-def456"
    name: Scanner.ScanFile
    type: method
    file: internal/scanner/scanner.go
    pagerank: 0.0234
    in_degree: 45
    coverage: 88.0

modules:
  - path: internal/store
    entities: 234
    functions: 120
    types: 45
    coverage: 89.0

  - path: internal/scanner
    entities: 156
    functions: 89
    types: 23
    coverage: 76.0

diagrams:
  architecture:
    title: "System Architecture"
    d2: |
      direction: right

      internal/cmd: {
        label: "CLI Commands"
        icon: https://icons.terrastruct.com/essentials%2F087-display.svg
        style.fill: "#e8f5e9"

        report
        show
        find
        scan
      }

      internal/store: {
        label: "Data Store"
        icon: https://icons.terrastruct.com/azure%2FDatabases%2F10132-icon-service-SQL-Database.svg
        style.fill: "#e3f2fd"

        Entity
        Metrics
        FTS
      }

      internal/scanner: {
        label: "Code Scanner"
        icon: https://icons.terrastruct.com/essentials%2F107-zoom.svg
        style.fill: "#fff3e0"

        Scanner
        Parser
      }

      internal/cmd -> internal/store
      internal/cmd -> internal/scanner
      internal/scanner -> internal/store

health:
  coverage_overall: 78.5
  untested_keystones: 3
  circular_dependencies: 0
  dead_code_candidates: 12
```

### Changes Report Data Schema

```yaml
report:
  type: changes
  generated_at: "2026-01-20T15:30:00Z"
  from_ref: "HEAD~50"
  to_ref: "HEAD"

metadata:
  commits_analyzed: 50
  time_range:
    from: "2026-01-01T00:00:00Z"
    to: "2026-01-20T15:30:00Z"

statistics:
  added: 45
  modified: 89
  deleted: 12

added_entities:
  - id: "sa-fn-new123"
    name: GenerateReport
    type: function
    file: internal/report/generate.go
    lines: [1, 45]
    added_in: "abc1234"  # commit hash

modified_entities:
  - id: "sa-fn-mod456"
    name: SearchEntities
    type: function
    file: internal/store/fts.go
    change_summary: "Added embedding search support"
    lines_changed: 23

deleted_entities:
  - id: "sa-fn-del789"
    name: OldSearch
    type: function
    was_file: internal/store/search_old.go
    deleted_in: "def5678"

diagrams:
  architecture_before:
    title: "Architecture at HEAD~50"
    d2: |
      # D2 code for before state

  architecture_after:
    title: "Architecture at HEAD"
    d2: |
      # D2 code for after state

impact:
  high_impact_changes:
    - entity: SearchEntities
      dependents_affected: 23
      risk: medium
```

### Health Report Data Schema

```yaml
report:
  type: health
  generated_at: "2026-01-20T15:30:00Z"

risk_score: 72  # 0-100, higher = healthier

issues:
  critical:
    - type: untested_keystone
      entity: CriticalFunction
      file: internal/core/critical.go
      pagerank: 0.0456
      coverage: 0.0
      recommendation: "Add tests for this high-importance function"

  warning:
    - type: circular_dependency
      entities: [A, B, C]
      cycle: "A -> B -> C -> A"
      recommendation: "Break cycle by extracting shared interface"

    - type: low_coverage_bottleneck
      entity: SessionCache.Get
      coverage: 45.0
      importance: bottleneck
      recommendation: "Increase coverage for frequently-used code"

  info:
    - type: dead_code_candidate
      entity: UnusedHelper
      file: internal/util/helpers.go
      in_degree: 0
      recommendation: "Consider removing if truly unused"

coverage:
  overall: 78.5
  by_importance:
    keystone: 85.0
    bottleneck: 72.0
    normal: 78.0
    leaf: 65.0

complexity:
  hotspots:
    - entity: ComplexParser
      out_degree: 45
      lines: 234
      cyclomatic: 23

diagrams:
  risk_map:
    title: "Risk Heat Map"
    d2: |
      # D2 code showing risk distribution
```

---

## D2 Visual Design System

### Theme

Use a professional theme with consistent colors:

```d2
vars: {
  d2-config: {
    theme-id: 200  # Mixed Berry Blue
    layout-engine: elk
  }
}
```

### Entity Type Styling

| Entity Type | Shape | Icon | Color |
|-------------|-------|------|-------|
| Function | rectangle | `tech/go.svg` or language icon | `#e3f2fd` (light blue) |
| Method | rectangle | `essentials/gear.svg` | `#e8f5e9` (light green) |
| Type/Struct | rectangle | `essentials/package.svg` | `#f3e5f5` (light purple) |
| Interface | rectangle | `essentials/plug.svg` | `#fff3e0` (light orange) |
| Database | cylinder | `azure/Databases/*.svg` | `#eceff1` (light gray) |
| HTTP Handler | rectangle | `essentials/globe.svg` | `#e0f7fa` (light cyan) |
| Test | rectangle | `essentials/checkmark.svg` | `#e8f5e9` (light green) |

### Importance Styling

```d2
# Keystone - bold, prominent
keystone_entity: {
  style: {
    stroke-width: 3
    shadow: true
    fill: "#fff3e0"
    stroke: "#f57c00"
  }
}

# Bottleneck - warning accent
bottleneck_entity: {
  style: {
    stroke-width: 2
    fill: "#fff8e1"
    stroke: "#ffa000"
  }
}

# Normal
normal_entity: {
  style: {
    stroke-width: 1
    fill: "#ffffff"
    stroke: "#9e9e9e"
  }
}

# Leaf - lighter
leaf_entity: {
  style: {
    stroke-width: 1
    fill: "#fafafa"
    stroke: "#bdbdbd"
    opacity: 0.8
  }
}
```

### Coverage Indicators

```d2
# High coverage (>80%) - green badge
high_coverage: {
  style.fill: "#c8e6c9"
  style.stroke: "#4caf50"
}

# Medium coverage (50-80%) - yellow badge
medium_coverage: {
  style.fill: "#fff9c4"
  style.stroke: "#fbc02d"
}

# Low coverage (<50%) - red badge
low_coverage: {
  style.fill: "#ffcdd2"
  style.stroke: "#f44336"
}
```

### Edge Styling

```d2
# Function calls - solid arrow
a -> b: calls

# Implements - dashed
a -> b: implements {
  style.stroke-dash: 3
}

# Data flow - thick
a -> b: data {
  style.stroke-width: 2
}
```

### Container Styling

```d2
# Module container
internal/auth: {
  label: "Authentication"
  icon: https://icons.terrastruct.com/essentials%2F092-shield.svg
  style: {
    fill: "#e3f2fd"
    stroke: "#1976d2"
    border-radius: 8
  }

  LoginUser
  ValidateToken
}
```

---

## Agent Integration Workflow

### For Claude Code (Recommended)

When a user requests a report:

1. **Gather Data**
   ```bash
   cx report feature "authentication" --data > /tmp/report-data.yaml
   ```

2. **Read and Interpret**
   - Parse the YAML output
   - Understand the entities, dependencies, and diagrams

3. **Write Narrative Sections**
   - Feature Overview: Explain what this feature does and why it matters
   - Flow Explanation: Walk through the call/data flow
   - Coverage Analysis: Highlight risks and gaps
   - Recommendations: Suggest improvements

4. **Assemble HTML Report**
   ```html
   <!DOCTYPE html>
   <html>
   <head>
     <title>Authentication Feature Report</title>
     <style>
       /* Embedded professional CSS */
     </style>
   </head>
   <body>
     <h1>Authentication Feature Report</h1>

     <section id="overview">
       <h2>Overview</h2>
       <p><!-- AI-generated narrative --></p>
     </section>

     <section id="entities">
       <h2>Key Entities</h2>
       <table><!-- Entity table from data --></table>
     </section>

     <section id="call-flow">
       <h2>Call Flow</h2>
       <pre class="d2-diagram">
         <!-- D2 code from data -->
       </pre>
     </section>

     <!-- More sections -->
   </body>
   </html>
   ```

5. **Optional: Render Diagrams**
   ```bash
   cx render report.html  # Converts D2 code blocks to SVG
   ```

### Progress Communication

For longer reports, communicate progress:

```
User: Generate a report on authentication

Claude: I'll generate an authentication feature report.

Gathering data from CX...
[runs cx report feature "auth" --data]

Found 15 relevant entities. Let me analyze the authentication flow.

Key components identified:
- LoginUser (keystone, 85% coverage)
- ValidateToken (bottleneck, 92% coverage)
- SessionCache (45% coverage - risk)

Writing report sections...
- Overview: Explaining the authentication architecture
- Call Flow: Documenting the login sequence
- Coverage: Highlighting the SessionCache gap

Report complete! I've created auth-report.html.

Summary:
- 15 entities analyzed
- 1 coverage gap identified (SessionCache at 45%)
- Call flow and data flow diagrams included
```

---

## HTML Report Template

### Structure

```html
<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{title}} - CX Report</title>
  <style>
    :root {
      --bg-primary: #ffffff;
      --bg-secondary: #f5f5f5;
      --text-primary: #1a1a1a;
      --text-secondary: #666666;
      --accent: #1976d2;
      --success: #4caf50;
      --warning: #ff9800;
      --error: #f44336;
      --border: #e0e0e0;
      --code-bg: #f8f9fa;
    }

    [data-theme="dark"] {
      --bg-primary: #1a1a1a;
      --bg-secondary: #2d2d2d;
      --text-primary: #f5f5f5;
      --text-secondary: #a0a0a0;
      --border: #404040;
      --code-bg: #2d2d2d;
    }

    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      line-height: 1.6;
      color: var(--text-primary);
      background: var(--bg-primary);
      max-width: 1200px;
      margin: 0 auto;
      padding: 2rem;
    }

    h1, h2, h3 { color: var(--text-primary); }

    .metadata {
      color: var(--text-secondary);
      font-size: 0.9rem;
      margin-bottom: 2rem;
    }

    .toc {
      background: var(--bg-secondary);
      padding: 1rem 1.5rem;
      border-radius: 8px;
      margin-bottom: 2rem;
    }

    section {
      margin-bottom: 3rem;
      padding-bottom: 2rem;
      border-bottom: 1px solid var(--border);
    }

    table {
      width: 100%;
      border-collapse: collapse;
      margin: 1rem 0;
    }

    th, td {
      padding: 0.75rem;
      text-align: left;
      border-bottom: 1px solid var(--border);
    }

    th { background: var(--bg-secondary); }

    .diagram {
      background: var(--bg-secondary);
      padding: 1rem;
      border-radius: 8px;
      overflow-x: auto;
    }

    .diagram svg {
      max-width: 100%;
      height: auto;
    }

    .badge {
      display: inline-block;
      padding: 0.25rem 0.5rem;
      border-radius: 4px;
      font-size: 0.8rem;
      font-weight: 500;
    }

    .badge-success { background: #c8e6c9; color: #2e7d32; }
    .badge-warning { background: #fff9c4; color: #f57f17; }
    .badge-error { background: #ffcdd2; color: #c62828; }

    code {
      background: var(--code-bg);
      padding: 0.2rem 0.4rem;
      border-radius: 4px;
      font-family: 'SF Mono', Monaco, 'Courier New', monospace;
      font-size: 0.9em;
    }

    pre {
      background: var(--code-bg);
      padding: 1rem;
      border-radius: 8px;
      overflow-x: auto;
    }

    .entity-link {
      color: var(--accent);
      text-decoration: none;
    }

    .entity-link:hover {
      text-decoration: underline;
    }

    footer {
      margin-top: 3rem;
      padding-top: 1rem;
      border-top: 1px solid var(--border);
      color: var(--text-secondary);
      font-size: 0.9rem;
    }
  </style>
</head>
<body>
  <header>
    <h1>{{title}}</h1>
    <p class="metadata">
      Generated: {{generated_at}} |
      Query: <code>{{query}}</code> |
      Entities: {{entity_count}}
    </p>
  </header>

  <nav class="toc">
    <strong>Contents</strong>
    <ul>
      <li><a href="#overview">Overview</a></li>
      <li><a href="#entities">Key Entities</a></li>
      <li><a href="#call-flow">Call Flow</a></li>
      <li><a href="#coverage">Coverage Analysis</a></li>
    </ul>
  </nav>

  <main>
    <!-- Sections inserted by agent -->
  </main>

  <footer>
    Generated by <a href="https://github.com/anthropics/cx">CX</a> with Claude
  </footer>
</body>
</html>
```

---

## Implementation Tasks

### R0: Data Output Schema (New)
Define and validate YAML output schemas for all report types.

**Deliverables:**
- [ ] Schema definitions (Go structs)
- [ ] YAML marshaling with consistent field ordering
- [ ] Schema validation tests
- [ ] Documentation of all fields

### R1: Report Engine Core
Core data gathering and YAML output.

**Deliverables:**
- [ ] `cx report` command with subcommands
- [ ] `--data` flag for YAML output
- [ ] `--format json|yaml` option
- [ ] Output to stdout or file (`-o`)

### R2: D2 Diagram Integration
Generate D2 code for various diagram types.

**Deliverables:**
- [ ] D2 code generation from entity graphs
- [ ] Diagram types: architecture, call-flow, dependency, diff
- [ ] Icon and styling integration
- [ ] `cx render` command for D2 → SVG

### R2.1: Visual Design System
Professional styling for all diagrams.

**Deliverables:**
- [ ] Theme configuration
- [ ] Entity type → icon/color mapping
- [ ] Importance styling presets
- [ ] Coverage indicator styling
- [ ] Example diagrams

### R4: Overview Report
System-level summary data.

**Deliverables:**
- [ ] Statistics gathering (entity counts, language breakdown)
- [ ] Keystone identification
- [ ] Module structure extraction
- [ ] Architecture diagram D2 generation
- [ ] Health indicator computation

### R5: Feature Report
Semantic search-driven feature analysis.

**Deliverables:**
- [ ] Hybrid search (FTS + embeddings)
- [ ] Entity relevance ranking
- [ ] Dependency traversal
- [ ] Call flow D2 generation
- [ ] Test association
- [ ] Coverage per-entity

### R6: Change Report
Historical analysis using Dolt time-travel.

**Deliverables:**
- [ ] Dolt diff queries
- [ ] Added/modified/deleted entity detection
- [ ] Before/after architecture snapshots
- [ ] Impact analysis (affected dependents)

### R7: Health Report
Risk analysis and recommendations.

**Deliverables:**
- [ ] Risk score computation
- [ ] Untested keystone detection
- [ ] Dead code candidate identification
- [ ] Circular dependency detection
- [ ] Complexity hotspot identification

---

## Existing Code to Leverage

| Component | Location | Reuse |
|-----------|----------|-------|
| Semantic Search | [internal/store/fts.go](../internal/store/fts.go) | SearchEntities with FTS |
| Embedding Search | [internal/store/embeddings.go](../internal/store/embeddings.go) | FindSimilar for hybrid search |
| D2 Generation | [internal/graph/d2.go](../internal/graph/d2.go) | GenerateD2, sanitizeD2ID |
| Guide Command | [internal/cmd/guide.go](../internal/cmd/guide.go) | Entity stats, module analysis |
| Graph Building | [internal/graph/graph.go](../internal/graph/graph.go) | Dependency traversal |
| Time Travel | [internal/store/timetravel.go](../internal/store/timetravel.go) | AS OF queries |
| Coverage | [internal/store/coverage.go](../internal/store/coverage.go) | Coverage data |

---

## Out of Scope

- **Standalone mode without agent** - Low priority, future enhancement
- **Mermaid fallback** - D2 required for quality; no fallback
- **CI/CD integration** - Reports require agent context
- **PDF export** - HTML is sufficient; users can print to PDF
- **Real-time updates** - Reports are point-in-time snapshots

---

## Success Criteria

1. `cx report feature "auth" --data` produces valid YAML in < 5 seconds
2. Claude Code can generate publication-quality HTML from the data
3. D2 diagrams render with professional styling
4. Reports are useful for documentation and stakeholder communication
5. No API key required - works in any Claude Code session

---

## Multi-Agent Execution Notes

### Recommended Agent Assignment

| Task | Agent Type | Notes |
|------|------------|-------|
| R0, R1 | Backend Polecat | Schema + core engine |
| R2, R2.1 | Backend Polecat | D2 integration + design |
| R4-R7 | Backend Polecats (parallel) | One per report type |
| Integration | Refinery | Final testing + polish |

### Parallelization Strategy

1. **Phase 1**: R0 (schema definition) - 1 agent
2. **Phase 2**: R1 + R2 in parallel - 2 agents
3. **Phase 3**: R2.1 (depends on R2) - 1 agent
4. **Phase 4**: R4, R5, R6, R7 in parallel - 4 agents
5. **Phase 5**: Integration testing - 1 agent

### Estimated Timeline

- Sequential: 8 tasks
- With 4 agents: ~4 phases of work
- Bottleneck: R2.1 must complete before report types

---

## References

- [D2 Documentation](https://d2lang.com/)
- [D2 Icons](https://icons.terrastruct.com/)
- [TALA Layout Engine](https://terrastruct.com/tala/)
- [Dolt Time Travel](https://docs.dolthub.com/sql-reference/version-control/querying-history)

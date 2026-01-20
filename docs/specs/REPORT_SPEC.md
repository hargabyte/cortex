# CX Report Specification

> **Epic**: cortex-dkd | **Priority**: P1 | **Status**: Open

Generate AI-powered, publication-quality reports about codebases with rich D2 visualizations.

## Overview

`cx report` transforms code intelligence data into polished, narrative-driven documents that explain how systems work, how they've evolved, and where the risks are.

```
┌─────────────────────────────────────────────────────────────┐
│                     CX REPORT                                │
│                                                              │
│   Code Graph  ──┬──►  Report Engine  ──►  HTML/Markdown     │
│   Dolt History ─┤          │                                 │
│   Semantic     ─┤          ▼                                 │
│   Search       ─┘    ┌─────────────┐                        │
│                      │ AI Narrative │                        │
│                      │ D2 Diagrams  │                        │
│                      │ Data Tables  │                        │
│                      └─────────────┘                        │
└─────────────────────────────────────────────────────────────┘
```

## Commands

### cx report overview

System-level summary with architecture diagram.

```bash
cx report overview                    # HTML to stdout
cx report overview -o report.html     # Write to file
cx report overview --format md        # Markdown output
cx report overview --no-ai            # Skip AI narrative (faster)
```

**Sections:**
1. Executive Summary (AI narrative)
2. Statistics (entity counts, language breakdown)
3. Architecture Diagram (D2 module graph)
4. Key Components (top 10 keystones explained)
5. Health Indicators

### cx report feature \<query\>

Deep-dive into a specific feature using semantic search.

```bash
cx report feature "authentication"
cx report feature "payment processing" -o payments.html
cx report feature --entity LoginUser --depth 3
```

**Sections:**
1. Feature Overview (AI explains what it does)
2. Key Entities (semantic search results)
3. Call Flow Diagram (D2 sequence diagram)
4. Data Flow (inputs → processing → outputs)
5. Related Tests
6. Coverage Analysis

### cx report changes --since \<ref\>

What changed between two points in time (Dolt time-travel).

```bash
cx report changes --since HEAD~50
cx report changes --since v1.0 --until v2.0
cx report changes --since 2026-01-01
cx report changes --last-week
```

**Sections:**
1. Change Summary (AI narrative)
2. Statistics (added/modified/deleted)
3. New Entities
4. Modified Entities
5. Deleted Entities
6. Architecture Diff (before/after diagrams)
7. Impact Analysis

### cx report history \<entity\>

Timeline of changes to a specific entity.

```bash
cx report history LoginUser
cx report history "internal/auth/*" --since v1.0
```

**Sections:**
1. Entity Overview
2. Change Timeline (commits, dates, authors)
3. Complexity Trend (LOC over time)
4. Dependency Evolution
5. Related Commits

### cx report health

Risk analysis and recommendations.

```bash
cx report health
cx report health --focus coverage
cx report health --focus complexity
```

**Sections:**
1. Health Summary (AI risk narrative)
2. Risk Score (0-100)
3. Untested Keystones
4. Dead Code Candidates
5. Circular Dependencies
6. Complexity Hotspots
7. Recommendations (AI action items)

---

## Architecture

### Report Engine (`internal/report/`)

```go
// Report represents a complete generated report
type Report struct {
    Title       string
    Subtitle    string
    GeneratedAt time.Time
    Sections    []Section
    Metadata    map[string]any
}

// Section is a component of a report
type Section interface {
    Type() SectionType
    Render(w Writer) error
}

// Section types
type NarrativeSection struct {
    Title   string
    Content string  // Markdown
}

type TableSection struct {
    Title   string
    Headers []string
    Rows    [][]string
}

type DiagramSection struct {
    Title    string
    D2Code   string
    SVG      string  // Rendered SVG
    Fallback string  // Mermaid fallback
}

type CodeSection struct {
    Title    string
    Language string
    Code     string
}

type ChartSection struct {
    Title string
    Type  string  // pie, bar, line
    Data  []ChartDataPoint
}
```

### Output Writers

```go
// Writer renders reports to different formats
type Writer interface {
    WriteReport(r *Report) error
}

// HTMLWriter produces styled HTML with embedded SVG
type HTMLWriter struct {
    Template *template.Template
    Theme    string  // "light" or "dark"
}

// MarkdownWriter produces GitHub-flavored markdown
type MarkdownWriter struct {
    IncludeDiagrams bool
}
```

### D2 Integration (`internal/report/d2/`)

```go
// Generator creates D2 diagrams from entity graphs
type Generator struct {
    Layout  string  // "dagre", "elk", "tala"
    Theme   string
    MaxNodes int
}

// DiagramType determines the diagram style
type DiagramType int
const (
    DiagramArchitecture DiagramType = iota  // Module-level boxes
    DiagramCallFlow                          // Sequence-style
    DiagramDependency                        // Directed graph
    DiagramSequence                          // UML sequence
)

// Generate creates D2 code from entities
func (g *Generator) Generate(entities []*store.Entity, deps []Dependency, dtype DiagramType) string

// Render invokes D2 CLI to produce SVG
func (g *Generator) Render(d2Code string) (svg string, err error)
```

### AI Narrative (`internal/report/narrative/`)

```go
// Generator creates AI-powered narratives
type Generator struct {
    Client  *anthropic.Client
    Model   string
    Cache   *NarrativeCache
}

// NarrativeType determines the prompt template
type NarrativeType int
const (
    NarrativeOverview NarrativeType = iota
    NarrativeFeature
    NarrativeChange
    NarrativeHealth
)

// Generate produces narrative text
func (g *Generator) Generate(ctx context.Context, ntype NarrativeType, data *NarrativeData) (string, error)

// NarrativeData provides context for generation
type NarrativeData struct {
    Entities    []*store.Entity
    Statistics  map[string]int
    Keystones   []*store.Entity
    Changes     []EntityChange
    Issues      []HealthIssue
}
```

---

## D2 Diagram Specifications

### Architecture Diagram

Shows modules (directories) as containers with top entities inside.

```d2
direction: right

internal/auth: {
  label: "Authentication"
  style.fill: "#e1f5fe"

  LoginUser: {
    shape: rectangle
    style.fill: "#fff"
  }

  ValidateToken: {
    shape: rectangle
    style.fill: "#fff"
  }
}

internal/store: {
  label: "Data Store"
  style.fill: "#f3e5f5"

  UserStore: {
    shape: cylinder
  }
}

internal/auth.LoginUser -> internal/store.UserStore: "queries"
```

### Call Flow Diagram

Shows function call sequences.

```d2
direction: down

1.Request: {
  shape: oval
  label: "HTTP Request"
}

2.Handler: {
  label: "LoginHandler.Handle"
}

3.Auth: {
  label: "AuthService.Authenticate"
}

4.Store: {
  label: "UserStore.GetByEmail"
  shape: cylinder
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
```

### Dependency Diagram

Shows entity-level dependencies.

```d2
direction: right

LoginUser: {
  style.fill: "#ffcdd2"
  label: "LoginUser\n(keystone)"
}

ValidateToken
SessionCache
UserStore: {
  shape: cylinder
}

LoginUser -> ValidateToken
LoginUser -> SessionCache
LoginUser -> UserStore
ValidateToken -> SessionCache
```

### Diff Diagram (Before/After)

Shows architecture changes over time.

```d2
grid-rows: 1

Before: {
  label: "v1.0"

  Auth: {
    Login
    Logout
  }
}

After: {
  label: "v2.0"
  style.fill: "#e8f5e9"

  Auth: {
    Login
    Logout
    OAuth: {
      style.fill: "#c8e6c9"
      label: "OAuth (new)"
    }
  }
}
```

---

## HTML Template Structure

```html
<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
    <meta charset="UTF-8">
    <title>{{.Title}} - CX Report</title>
    <style>
        /* Embedded CSS for portability */
        :root {
            --bg-primary: #ffffff;
            --text-primary: #1a1a1a;
            --accent: #2563eb;
            /* ... */
        }
        [data-theme="dark"] {
            --bg-primary: #1a1a1a;
            --text-primary: #f5f5f5;
            /* ... */
        }
        /* Professional typography, tables, code blocks */
    </style>
</head>
<body>
    <header>
        <h1>{{.Title}}</h1>
        <p class="subtitle">{{.Subtitle}}</p>
        <p class="meta">Generated: {{.GeneratedAt.Format "2006-01-02 15:04"}}</p>
    </header>

    <nav class="toc">
        <h2>Contents</h2>
        <ul>
            {{range .Sections}}
            <li><a href="#{{.ID}}">{{.Title}}</a></li>
            {{end}}
        </ul>
    </nav>

    <main>
        {{range .Sections}}
        <section id="{{.ID}}">
            {{.Render}}
        </section>
        {{end}}
    </main>

    <footer>
        <p>Generated by <a href="https://github.com/anthropics/cx">CX</a></p>
    </footer>
</body>
</html>
```

---

## AI Prompt Templates

### Overview Narrative

```
You are analyzing a codebase to write an executive summary.

Statistics:
- Total entities: {{.TotalEntities}}
- Functions: {{.Functions}}, Types: {{.Types}}, Methods: {{.Methods}}
- Languages: {{.Languages}}

Top keystones (most important entities by PageRank):
{{range .Keystones}}
- {{.Name}} ({{.Type}}) in {{.File}} - PageRank: {{.PageRank}}
{{end}}

Module structure:
{{range .Modules}}
- {{.Path}}: {{.EntityCount}} entities
{{end}}

Write a 2-3 paragraph executive summary explaining:
1. What this codebase does (infer from entity names and structure)
2. How it's organized (key modules and their roles)
3. Notable architectural patterns

Be specific and technical. Reference actual entity and module names.
```

### Feature Narrative

```
You are explaining how a feature works in a codebase.

Feature query: "{{.Query}}"

Relevant entities found via semantic search:
{{range .Entities}}
- {{.Name}} ({{.Type}}) in {{.File}}
  Signature: {{.Signature}}
  Called by: {{.Callers}}
  Calls: {{.Callees}}
{{end}}

Entry points (high in-degree):
{{range .EntryPoints}}
- {{.Name}}
{{end}}

Write a technical explanation of how this feature works:
1. Start with the entry point(s) - how is this feature triggered?
2. Explain the flow through the key entities
3. Note any important data transformations
4. Mention error handling or edge cases if visible

Use specific entity names. Write for a developer who needs to understand or modify this code.
```

### Change Narrative

```
You are summarizing code changes between two versions.

Time range: {{.FromRef}} to {{.ToRef}}

Added entities ({{len .Added}}):
{{range .Added}}
- {{.Name}} ({{.Type}}) in {{.File}}
{{end}}

Modified entities ({{len .Modified}}):
{{range .Modified}}
- {{.Name}}: {{.ChangeDescription}}
{{end}}

Deleted entities ({{len .Deleted}}):
{{range .Deleted}}
- {{.Name}} (was in {{.File}})
{{end}}

Write a summary of these changes:
1. What was the main thrust of this work? (new feature, refactoring, bug fixes)
2. Which areas of the codebase were most affected?
3. Are there any concerning patterns (lots of deletions, high churn)?

Be concise but specific. This summary will be read by developers and managers.
```

---

## Implementation Plan

### Phase 1: Foundation (cortex-dkd.1, cortex-dkd.2, cortex-dkd.3)

1. **Report Engine Core** - Report struct, sections, HTML/MD writers
2. **D2 Integration** - Code generation, CLI invocation, SVG embedding
3. **AI Narrative** - Anthropic client, prompt templates, caching

### Phase 2: Report Types (cortex-dkd.4 - cortex-dkd.7)

4. **Overview Report** - System summary, architecture diagram
5. **Feature Report** - Semantic search + call flow
6. **Change Report** - Dolt time-travel + diff diagrams
7. **Health Report** - Risk analysis + recommendations

### Dependencies

```
R1 (Engine) ──┬──► R4 (Overview)
R2 (D2)     ──┼──► R5 (Feature)
R3 (AI)     ──┼──► R6 (Changes)
              └──► R7 (Health)
```

---

## Example Output

### Overview Report (excerpt)

```html
<section id="summary">
    <h2>Executive Summary</h2>
    <div class="narrative">
        <p>This is a <strong>code intelligence tool</strong> (CX) that analyzes
        codebases to extract entities, dependencies, and metrics. The core
        functionality centers around the <code>internal/store</code> module,
        which manages a Dolt database of code entities.</p>

        <p>The codebase is organized into three main layers:</p>
        <ul>
            <li><strong>CLI layer</strong> (<code>internal/cmd/</code>) - User-facing commands</li>
            <li><strong>Analysis layer</strong> (<code>internal/scanner/</code>) - Code parsing</li>
            <li><strong>Storage layer</strong> (<code>internal/store/</code>) - Dolt persistence</li>
        </ul>
    </div>
</section>

<section id="architecture">
    <h2>Architecture</h2>
    <figure class="diagram">
        <svg><!-- Rendered D2 diagram --></svg>
        <figcaption>Module dependency graph</figcaption>
    </figure>
</section>
```

---

## Success Metrics

1. **Quality**: Reports are useful without modification
2. **Performance**: Generate in <30s for typical codebases
3. **Reliability**: Graceful fallback when D2/AI unavailable
4. **Adoption**: Users generate reports for documentation

---

## References

- [D2 Documentation](https://d2lang.com/)
- [TALA Layout Engine](https://terrastruct.com/tala/)
- [Anthropic API](https://docs.anthropic.com/)
- [Dolt Time Travel](https://docs.dolthub.com/sql-reference/version-control/querying-history)

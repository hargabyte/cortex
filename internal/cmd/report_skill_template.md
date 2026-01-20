# /report [type] [query] - Generate Codebase Report

## Purpose
Generate publication-quality codebase reports by combining structured data from CX with AI-written narratives. This skill guides you through an interactive workflow to create the right report for your audience.

## Arguments
- `type` (optional): Report type - `overview`, `feature`, `changes`, or `health`
- `query` (optional): For feature reports, the search query (e.g., "authentication")
- `--since <ref>` (optional): For changes reports, the starting reference

If arguments are not provided, the skill will ask interactively.

---

## Workflow

### Step 1: Determine Report Type

If type not specified, ask the user:

```
AskUserQuestion:
  question: "What type of report would you like to generate?"
  header: "Report type"
  options:
    - label: "Overview"
      description: "System-level summary with architecture diagram and health metrics"
    - label: "Feature"
      description: "Deep-dive into a specific feature using semantic search"
    - label: "Changes"
      description: "What changed between two points in time (requires --since)"
    - label: "Health"
      description: "Risk analysis with coverage gaps and recommendations"
```

### Step 2: Gather Report-Specific Parameters

**For Feature Reports** (if query not provided):
```
AskUserQuestion:
  question: "What feature or concept should the report focus on?"
  header: "Feature query"
  options:
    - label: "Authentication"
      description: "Login, session management, tokens"
    - label: "API Endpoints"
      description: "HTTP handlers, routes, middleware"
    - label: "Data Storage"
      description: "Database operations, caching, persistence"
    - label: "Other"
      description: "I'll type a custom query"
```

**For Changes Reports** (if --since not provided):
```
AskUserQuestion:
  question: "What time range should the report cover?"
  header: "Time range"
  options:
    - label: "Last 50 commits"
      description: "Recent development activity (--since HEAD~50)"
    - label: "Last release"
      description: "Changes since the last tag/release"
    - label: "Last week"
      description: "Past 7 days of changes"
    - label: "Custom"
      description: "I'll specify a commit, tag, or date"
```

### Step 3: Gather Preferences

Ask for audience and format:

```
AskUserQuestion:
  question: "Who is the primary audience for this report?"
  header: "Audience"
  options:
    - label: "Developers"
      description: "Technical details, code references, implementation notes"
    - label: "Tech Leads"
      description: "Architecture overview, risk assessment, recommendations"
    - label: "Stakeholders"
      description: "High-level summary, business impact, progress metrics"
    - label: "New Team Members"
      description: "Onboarding context, explanations, learning path"
```

```
AskUserQuestion:
  question: "What output format do you prefer?"
  header: "Format"
  options:
    - label: "HTML (Recommended)"
      description: "Rich formatting with embedded diagrams, best for sharing"
    - label: "Markdown"
      description: "Plain text with formatting, good for documentation"
    - label: "Terminal"
      description: "Concise summary displayed here, no file created"
```

```
AskUserQuestion:
  question: "Which sections should the report emphasize?"
  header: "Focus areas"
  multiSelect: true
  options:
    - label: "Diagrams"
      description: "Include D2 architecture and flow diagrams"
    - label: "Coverage"
      description: "Test coverage analysis and gaps"
    - label: "Dependencies"
      description: "Call graphs and dependency relationships"
    - label: "Recommendations"
      description: "Actionable suggestions for improvement"
```

### Step 4: Collect Data

Run the appropriate cx report command based on type:

```bash
# Overview report
cx report overview --data

# Feature report
cx report feature "<query>" --data

# Changes report
cx report changes --since <ref> --data

# Health report
cx report health --data
```

**Parse the YAML output and extract:**
- `report.type` and `report.generated_at`
- `metadata.*` for context
- `entities[]` for key code elements
- `dependencies[]` for relationships
- `diagrams.*` for D2 code blocks
- `coverage.*` for test metrics

### Step 5: Write Narrative Sections

Based on audience, write narratives with appropriate depth:

**For Developers:**
- Include code references with file:line links
- Explain implementation details
- Highlight technical decisions and tradeoffs

**For Tech Leads:**
- Focus on architecture and patterns
- Emphasize risks and coverage gaps
- Include actionable recommendations

**For Stakeholders:**
- Use plain language, avoid jargon
- Focus on progress and health metrics
- Highlight business impact

**For New Team Members:**
- Provide context and explanations
- Include learning paths
- Explain why things are structured this way

### Step 6: Assemble Output

**Naming Convention:**
```
reports/<type>_<YYYY-MM-DD>[_<query>].<ext>

Examples:
  reports/overview_2026-01-20.html
  reports/feature_2026-01-20_authentication.html
  reports/changes_2026-01-20_HEAD~50.md
  reports/health_2026-01-20.html
```

Create the `reports/` directory if it doesn't exist.

### Step 7: Optional Diagram Rendering

If HTML format with diagrams:
```bash
# Check if D2 is available
cx render --check

# If available, render diagrams in-place
cx render reports/<filename>.html
```

If D2 is not available, keep the D2 code blocks in `<pre class="d2-diagram">` tags for later rendering.

---

## Output Templates

### HTML Template

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

    h1, h2, h3 { color: var(--text-primary); margin-top: 1.5em; }
    h1 { border-bottom: 2px solid var(--accent); padding-bottom: 0.5em; }

    .metadata {
      color: var(--text-secondary);
      font-size: 0.9rem;
      margin-bottom: 2rem;
      padding: 1rem;
      background: var(--bg-secondary);
      border-radius: 8px;
    }

    .toc {
      background: var(--bg-secondary);
      padding: 1rem 1.5rem;
      border-radius: 8px;
      margin-bottom: 2rem;
    }

    .toc ul { margin: 0.5rem 0; padding-left: 1.5rem; }
    .toc a { color: var(--accent); text-decoration: none; }
    .toc a:hover { text-decoration: underline; }

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

    th { background: var(--bg-secondary); font-weight: 600; }

    .diagram {
      background: var(--bg-secondary);
      padding: 1rem;
      border-radius: 8px;
      overflow-x: auto;
      margin: 1rem 0;
    }

    .diagram svg { max-width: 100%; height: auto; }

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
    .badge-info { background: #e3f2fd; color: #1565c0; }

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

    .entity-card {
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 1rem;
      margin: 0.5rem 0;
    }

    .entity-card h4 { margin: 0 0 0.5rem 0; }
    .entity-card .location { color: var(--text-secondary); font-size: 0.85rem; }

    .risk-high { border-left: 4px solid var(--error); }
    .risk-medium { border-left: 4px solid var(--warning); }
    .risk-low { border-left: 4px solid var(--success); }

    .coverage-bar {
      height: 8px;
      background: var(--bg-secondary);
      border-radius: 4px;
      overflow: hidden;
      margin-top: 0.25rem;
    }

    .coverage-bar-fill {
      height: 100%;
      border-radius: 4px;
    }

    .coverage-high { background: var(--success); }
    .coverage-medium { background: var(--warning); }
    .coverage-low { background: var(--error); }

    footer {
      margin-top: 3rem;
      padding-top: 1rem;
      border-top: 1px solid var(--border);
      color: var(--text-secondary);
      font-size: 0.9rem;
      text-align: center;
    }

    footer a { color: var(--accent); }
  </style>
</head>
<body>
  <header>
    <h1>{{title}}</h1>
    <div class="metadata">
      <strong>Generated:</strong> {{generated_at}} |
      <strong>Type:</strong> {{report_type}} |
      <strong>Entities:</strong> {{entity_count}}
      {{#if query}} | <strong>Query:</strong> <code>{{query}}</code>{{/if}}
    </div>
  </header>

  <nav class="toc">
    <strong>Contents</strong>
    <ul>
      <!-- TOC items based on sections -->
    </ul>
  </nav>

  <main>
    <!-- Sections inserted based on report type -->
  </main>

  <footer>
    Generated by <a href="https://github.com/anthropics/cx">CX</a> with Claude
  </footer>
</body>
</html>
```

### Markdown Template

```markdown
# {{title}}

> Generated: {{generated_at}} | Type: {{report_type}} | Entities: {{entity_count}}

## Table of Contents

- [Overview](#overview)
- [Key Entities](#key-entities)
- [Dependencies](#dependencies)
- [Coverage](#coverage)
- [Recommendations](#recommendations)

---

## Overview

{{overview_narrative}}

## Key Entities

| Name | Type | Location | Importance | Coverage |
|------|------|----------|------------|----------|
{{#each entities}}
| `{{name}}` | {{type}} | [{{file}}:{{line}}]({{file}}#L{{line}}) | {{importance}} | {{coverage}}% |
{{/each}}

## Dependencies

{{dependencies_narrative}}

### Call Flow

\`\`\`d2
{{call_flow_d2}}
\`\`\`

## Coverage

{{coverage_narrative}}

| Entity | Coverage | Risk |
|--------|----------|------|
{{#each coverage_gaps}}
| `{{entity}}` | {{coverage}}% | {{risk}} |
{{/each}}

## Recommendations

{{#each recommendations}}
{{@index}}. **{{title}}**: {{description}}
{{/each}}

---

*Generated by [CX](https://github.com/anthropics/cx) with Claude*
```

### Terminal Summary Template

```
═══════════════════════════════════════════════════════════════
                    {{TITLE}}
═══════════════════════════════════════════════════════════════

📊 SUMMARY
───────────────────────────────────────────────────────────────
Type:       {{report_type}}
Generated:  {{generated_at}}
Entities:   {{entity_count}}
{{#if query}}Query:      {{query}}{{/if}}

🔑 KEY ENTITIES
───────────────────────────────────────────────────────────────
{{#each top_entities}}
{{importance_icon}} {{name}} ({{type}})
   └─ {{file}}:{{line}} | Coverage: {{coverage}}%
{{/each}}

📈 COVERAGE
───────────────────────────────────────────────────────────────
Overall: {{overall_coverage}}%

Gaps ({{coverage_gap_count}}):
{{#each coverage_gaps}}
  ⚠️  {{entity}}: {{coverage}}% ({{importance}})
{{/each}}

💡 RECOMMENDATIONS
───────────────────────────────────────────────────────────────
{{#each recommendations}}
{{@index}}. {{title}}
{{/each}}

═══════════════════════════════════════════════════════════════
```

---

## Report Type-Specific Sections

### Overview Report Sections

1. **Executive Summary** - High-level health and architecture
2. **Statistics** - Entity counts by type and language
3. **Architecture Diagram** - D2 visualization of modules
4. **Keystones** - Most important entities
5. **Health Summary** - Overall risk assessment

### Feature Report Sections

1. **Feature Overview** - What this feature does
2. **Key Entities** - Relevant code with relevance scores
3. **Call Flow** - How the feature executes
4. **Data Flow** - How data moves through the feature
5. **Tests** - Associated test coverage
6. **Recommendations** - Improvements and risks

### Changes Report Sections

1. **Change Summary** - What changed and why it matters
2. **Added Entities** - New code introduced
3. **Modified Entities** - Existing code changed
4. **Deleted Entities** - Code removed
5. **Impact Analysis** - What was affected
6. **Before/After Diagrams** - Architecture evolution

### Health Report Sections

1. **Risk Score** - Overall health (0-100)
2. **Critical Issues** - Must fix immediately
3. **Warnings** - Should address soon
4. **Coverage Analysis** - Test coverage by importance
5. **Complexity Hotspots** - Areas that need attention
6. **Recommendations** - Prioritized action items

---

## Writing Guidelines

### For Each Audience

**Developers:**
- Use precise technical language
- Include code snippets and file references
- Explain implementation decisions
- Link to specific lines: `[function](file.go#L42)`

**Tech Leads:**
- Focus on patterns and architecture
- Highlight technical debt and risks
- Provide strategic recommendations
- Include metrics and trends

**Stakeholders:**
- Avoid jargon, explain concepts
- Focus on business impact
- Use percentages and trends
- Keep it concise

**New Team Members:**
- Provide context and history
- Explain "why" not just "what"
- Include learning paths
- Link to documentation

### Diagram Guidelines

- Use D2's `direction: down` for call flows
- Use `direction: right` for data flows
- Group related entities in containers
- Apply importance-based styling:
  - Keystone: bold stroke, shadow
  - Bottleneck: warning colors
  - Normal: default styling
  - Leaf: lighter, reduced opacity

### Coverage Color Coding

- **Green** (>80%): Well tested
- **Yellow** (50-80%): Needs attention
- **Red** (<50%): Risk area

---

## Example Session

```
User: /report

Claude: I'll help generate a codebase report.

[AskUserQuestion: Report type]
User selects: "Feature"

[AskUserQuestion: Feature query]
User types: "authentication"

[AskUserQuestion: Audience]
User selects: "Tech Leads"

[AskUserQuestion: Format]
User selects: "HTML (Recommended)"

[AskUserQuestion: Focus areas]
User selects: "Diagrams", "Coverage", "Recommendations"

Claude: Gathering authentication feature data...

[Runs: cx report feature "authentication" --data]

Found 12 relevant entities. Analyzing...

Key findings:
- LoginUser (keystone, 85% coverage)
- ValidateToken (bottleneck, 92% coverage)
- SessionCache.Get (45% coverage - risk)

Generating report...

Report saved to: reports/feature_2026-01-20_authentication.html

Summary:
- 12 entities analyzed
- 2 coverage gaps identified
- 3 recommendations provided
- Call flow and dependency diagrams included

[If D2 available: Rendered diagrams to SVG]
```

---

## Important Rules

1. **Always use --data flag** - Reports are structured data for AI consumption
2. **Follow naming convention** - `reports/<type>_<date>[_<query>].<ext>`
3. **Match audience depth** - Technical for devs, strategic for leads, simple for stakeholders
4. **Include actionable recommendations** - Every report should have next steps
5. **Render diagrams when possible** - Check `cx render --check` first
6. **Create reports/ directory** - Ensure it exists before writing

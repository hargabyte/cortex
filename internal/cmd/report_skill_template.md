# /report [type] [query] - Generate Codebase Report

## Purpose
Generate publication-quality codebase reports with **visual D2 diagrams as the primary output**. Reports combine structured data from CX with AI-written narratives and automatically-generated architecture/call flow diagrams.

**Key Feature:** Every report includes D2 diagrams that visualize the codebase structure. These diagrams are rendered to SVG and embedded directly in the HTML output.

## Arguments
- `type` (optional): Report type - `overview`, `feature`, `changes`, or `health`
- `query` (optional): For feature reports, the search query (e.g., "authentication")
- `--since <ref>` (optional): For changes reports, the starting reference

If arguments are not provided, the skill will ask interactively.

---

## Workflow

**Question Order: Basic → Detailed**
1. Theme (basic visual choice)
2. Report type (basic content choice)
3. Report-specific params (if needed)
4. Audience (detailed)
5. Format (detailed)

### Step 1: Choose Diagram Theme (Basic)

```
AskUserQuestion:
  question: "What color theme would you like for the diagrams? (Type 'Other' to see all 20 themes)"
  header: "Theme"
  options:
    - label: "Neutral Default (0) (Recommended)"
      description: "Clean light theme, works everywhere"
    - label: "Vanilla Nitro Cola (100)"
      description: "Warm cream/brown, nostalgic professional feel"
    - label: "Earth Tones (103)"
      description: "Natural browns and greens, organic look"
    - label: "Terminal (300)"
      description: "Green-on-black retro terminal aesthetic"
  multiSelect: false
```

**If user selects "Other" or asks for more themes, show this full list:**

```
Light Themes:
  0  - Neutral Default      Clean minimal (recommended)
  1  - Neutral Grey         Grayscale, professional
  3  - Flagship Terrastruct Terrastruct branded
  4  - Cool Classics        Classic blue tones
  5  - Mixed Berry Blue     Cool blue-purple palette
  6  - Grape Soda           Vibrant purple/violet
  7  - Aubergine            Deep purple/eggplant
  8  - Colorblind Clear     High contrast, accessible
  100 - Vanilla Nitro Cola  Warm cream/brown nostalgic
  101 - Orange Creamsicle   Warm orange and cream
  102 - Shirley Temple      Playful pink and red
  103 - Earth Tones         Natural browns and greens
  104 - Everglade Green     Forest greens, nature
  105 - Buttered Toast      Warm golden tones
  300 - Terminal            Green-on-black terminal
  301 - Terminal Grayscale  Gray terminal aesthetic
  302 - Origami             Paper-fold inspired
  303 - C4                  C4 architecture style

Dark Themes:
  200 - Dark Mauve          Dark purple/mauve
  201 - Dark Flagship       Dark with branded accents
```

### Step 2: Determine Report Type (Basic)

```
AskUserQuestion:
  question: "What type of report would you like to generate?"
  header: "Report type"
  options:
    - label: "Overview"
      description: "System architecture diagram with module structure and health metrics"
    - label: "Feature"
      description: "Call flow diagram showing how a feature works end-to-end"
    - label: "Changes"
      description: "Before/after diagrams showing what changed (requires --since)"
    - label: "Health"
      description: "Risk visualization with coverage gaps and complexity hotspots"
```

### Step 3: Gather Report-Specific Parameters

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

### Step 4: Gather Audience and Format Preferences (Detailed)

```
AskUserQuestion:
  questions:
    - question: "Who is the primary audience for this report?"
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
      multiSelect: false

    - question: "What output format do you prefer?"
      header: "Format"
      options:
        - label: "HTML with diagrams (Recommended)"
          description: "Rich formatting with rendered SVG diagrams, best for sharing"
        - label: "Markdown with D2 code"
          description: "Plain text with D2 code blocks for later rendering"
        - label: "Terminal summary"
          description: "Concise summary with ASCII diagram representation"
      multiSelect: false
```

**All available D2 themes** (reference table):

| Theme Name | ID | Description |
|------------|-----|-------------|
| Neutral Default | 0 | Clean minimal default |
| Neutral Grey | 1 | Grayscale, professional |
| Flagship Terrastruct | 3 | Terrastruct branded |
| Cool Classics | 4 | Classic blue tones |
| Mixed Berry Blue | 5 | Cool blue-purple palette |
| Grape Soda | 6 | Vibrant purple/violet |
| Aubergine | 7 | Deep purple/eggplant |
| Colorblind Clear | 8 | High contrast, accessible |
| Vanilla Nitro Cola | 100 | Warm cream/brown nostalgic |
| Orange Creamsicle | 101 | Warm orange and cream |
| Shirley Temple | 102 | Playful pink and red |
| Earth Tones | 103 | Natural browns and greens |
| Everglade Green | 104 | Forest greens, nature-inspired |
| Buttered Toast | 105 | Warm golden tones |
| Terminal | 300 | Green-on-black terminal |
| Terminal Grayscale | 301 | Gray terminal aesthetic |
| Origami | 302 | Paper-fold inspired |
| C4 | 303 | C4 architecture style |
| Dark Mauve | 200 | Dark mode purple/mauve |
| Dark Flagship | 201 | Dark with branded accents |

### Step 5: Collect Data and Extract Diagrams

Run the appropriate cx report command with the selected theme:

```bash
# Overview report - includes architecture diagram
cx report overview --data --theme <selected_theme>

# Feature report - includes call flow diagram
cx report feature "<query>" --data --theme <selected_theme>

# Changes report - includes changes diagram
cx report changes --since <ref> --data --theme <selected_theme>

# Health report - includes risk distribution
cx report health --data --theme <selected_theme>
```

**Theme mapping from user selection (use numeric ID with --theme flag):**
- "Default (Recommended)" → `--theme 8` (Colorblind Clear)
- "Earth Tones" → `--theme 103`
- "Dark" → `--theme 200` (Dark Mauve)
- "Terminal" → `--theme 300`
- Custom input → look up ID from theme table (e.g., "Vanilla Nitro Cola" → `--theme 100`)

**IMPORTANT: Parse the YAML output and extract the `diagrams` section:**

The YAML output contains pre-generated D2 diagrams:
```yaml
diagrams:
  architecture:  # For overview reports
    title: "System Architecture"
    d2: |
      direction: right
      # ... D2 code ...

  call_flow:  # For feature reports
    title: "Call Flow: LoginUser"
    d2: |
      direction: down
      # ... D2 code ...

  changes_summary:  # For changes reports
    title: "Changes: HEAD~50 → HEAD"
    d2: |
      # ... D2 code ...
```

**Extract each diagram's D2 code for rendering.**

### Step 6: Render Diagrams to SVG

**This step is REQUIRED for HTML reports.** Diagrams are the primary visual element.

For each diagram in the YAML output:

```bash
# Save D2 code to temp file
echo '<D2 code from diagrams.*.d2>' > /tmp/diagram.d2

# Render to SVG
cx render /tmp/diagram.d2 -o /tmp/diagram.svg

# Or pipe directly
echo '<D2 code>' | cx render - -o /tmp/diagram.svg
```

**If `cx render` fails** (D2 not installed):
- Keep the D2 code in `<pre class="d2-diagram">` blocks
- Inform the user: "Diagrams included as D2 code. Install D2 to render: `brew install d2`"

### Step 6: Write Narrative Around Diagrams

The narrative should **explain the diagrams**, not replace them.

**For Overview Reports:**
1. Start with the architecture diagram
2. Explain what each module does
3. Highlight key dependencies shown in the diagram
4. Reference diagram entities by name

**For Feature Reports:**
1. Start with the call flow diagram
2. Walk through the flow step-by-step
3. Explain what each node in the diagram does
4. Reference line numbers from the entities

**For Changes Reports:**
1. Start with the changes diagram (green=added, yellow=modified, red=deleted)
2. Explain the impact of changes
3. Walk through high-impact modifications

**For Health Reports:**
1. Show coverage distribution visualization
2. Highlight risk areas with diagram annotations
3. Explain untested keystones visible in the diagram

### Step 7: Assemble HTML with Embedded Diagrams

**Template structure with diagrams as primary content:**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{title}} - CX Report</title>
  <style>
    /* ... CSS styles ... */
    .diagram-container {
      background: #f8f9fa;
      border-radius: 8px;
      padding: 1rem;
      margin: 1.5rem 0;
      overflow: auto;          /* Allow both horizontal and vertical scroll */
      min-height: 400px;       /* Ensure space for vertical diagrams */
      max-height: 90vh;        /* Cap at viewport height with scroll */
    }
    .diagram-container svg {
      display: block;
      margin: 0 auto;          /* Center horizontally */
      max-width: 100%;         /* Don't exceed container width */
      min-height: 300px;       /* Minimum height for readability */
    }
    .diagram-title {
      font-weight: 600;
      margin-bottom: 0.5rem;
      color: #333;
    }
  </style>
</head>
<body>
  <h1>{{title}}</h1>

  <!-- DIAGRAM FIRST - Primary visual -->
  <section id="architecture-diagram">
    <h2>Architecture Overview</h2>
    <div class="diagram-container">
      <div class="diagram-title">{{diagram_title}}</div>
      {{EMBEDDED_SVG_HERE}}
    </div>
    <p>{{diagram_explanation}}</p>
  </section>

  <!-- Then entities, coverage, etc. -->
  <section id="key-entities">
    <h2>Key Entities</h2>
    <!-- ... -->
  </section>
</body>
</html>
```

### Step 8: Save Report

**Naming Convention:**
```
reports/<type>_<YYYY-MM-DD>[_<query>].<ext>

Examples:
  reports/overview_2026-01-20.html
  reports/feature_2026-01-20_authentication.html
  reports/changes_2026-01-20_HEAD~50.html
  reports/health_2026-01-20.html
```

Create the `reports/` directory if it doesn't exist:
```bash
mkdir -p reports
```

---

## Diagram Types by Report

### Overview Report Diagrams

**Architecture Diagram** - Shows module containers with top entities:
- Modules shown as containers with rounded corners
- Top 3-5 entities shown inside each module
- Inter-module dependencies as arrows
- Color-coded by entity type

### Feature Report Diagrams

**Call Flow Diagram** - Shows execution path:
- Entry point at top
- Called functions below with arrows
- Each node shows function name and file
- Keystone entities highlighted with bold borders

### Changes Report Diagrams

**Changes Summary Diagram** - Shows what changed:
- Green nodes = Added entities
- Yellow nodes = Modified entities
- Red nodes = Deleted entities
- Grouped by module

### Health Report Diagrams

**Risk Distribution** - Shows coverage by importance:
- Node size = entity importance
- Node color = coverage level (green/yellow/red)
- Critical issues highlighted

---

## Complete Example Session

```
User: /report

Claude: I'll help generate a codebase report.

[AskUserQuestion: Report type]
User selects: "Feature"

[AskUserQuestion: Feature query]
User types: "authentication"

[AskUserQuestion: Theme, Audience, Format - all 3 questions asked together]
User selects: Theme="Earth Tones", Audience="Tech Leads", Format="HTML with diagrams"

Claude: Gathering authentication feature data with Earth Tones theme (ID 103)...

[Runs: cx report feature "authentication" --data]
[Runs: cx render diagram.d2 -o diagram.svg --theme 103]

Found 12 relevant entities. Extracting call flow diagram...

Diagram found in output:
- call_flow: "Call Flow: LoginUser" (47 lines of D2)

Rendering diagram to SVG...
[Runs: echo "<d2 code>" | cx render - -o /tmp/call_flow.svg]
✓ Rendered call_flow.svg (2.3 KB)

Generating report with embedded diagram...

---

# Authentication Feature Report

## Call Flow Diagram

[EMBEDDED SVG showing LoginUser → ValidateToken → SessionCache flow]

The authentication flow follows this path:
1. **LoginUser** receives credentials and validates input
2. **ValidateToken** checks JWT signature and expiry
3. **SessionCache** manages active sessions

## Key Entities

| Name | Type | Coverage | Importance |
|------|------|----------|------------|
| LoginUser | function | 85% | keystone |
| ValidateToken | function | 92% | bottleneck |
| SessionCache.Get | method | 45% | bottleneck |

## Coverage Gaps

⚠️ **SessionCache.Get** has only 45% coverage but is a bottleneck with 23 callers.
   Recommendation: Add tests for cache miss scenarios.

---

Report saved to: reports/feature_2026-01-20_authentication.html

Summary:
- 12 entities analyzed
- 1 call flow diagram rendered
- 1 coverage gap identified (SessionCache.Get at 45%)
- 3 recommendations provided
```

---

## Audience-Specific Narrative Style

### For Developers
- Reference specific lines: `LoginUser` at [internal/auth/login.go:45](internal/auth/login.go#L45)
- Include function signatures in entity tables
- Explain implementation details visible in the diagram
- Link to test files

### For Tech Leads
- Focus on what the diagram reveals about architecture
- Highlight risk areas shown in red/yellow
- Provide strategic recommendations
- Include metrics trends

### For Stakeholders
- Simplify diagram explanation - "This shows how login works"
- Use percentages and counts
- Avoid code references
- Focus on health and progress

### For New Team Members
- Explain each part of the diagram step-by-step
- Provide context for why things are structured this way
- Link to documentation
- Suggest what to explore next

---

## Important Rules

1. **DIAGRAMS ARE PRIMARY** - Every report MUST include at least one rendered diagram
2. **Extract D2 from YAML** - The `diagrams` section contains pre-generated D2 code
3. **Render before embedding** - Use `cx render` to convert D2 → SVG
4. **Diagram explains the story** - Write narrative that explains the diagram, not vice versa
5. **Follow naming convention** - `reports/<type>_<date>[_<query>].<ext>`
6. **Handle render failures gracefully** - If D2 not installed, keep D2 code blocks
7. **Embed SVG directly** - Don't link to external files, embed the SVG in HTML

---

## Troubleshooting

### "cx render" not found
D2 CLI is required for diagram rendering:
```bash
# macOS
brew install d2

# Linux
curl -fsSL https://d2lang.com/install.sh | sh

# Windows
choco install d2
```

### No diagrams in YAML output
Some report types may not generate diagrams if:
- No entities match the query (feature reports)
- No changes detected (changes reports)
- Database not scanned recently

Run `cx scan` to update the code graph.

### SVG too large
For very large diagrams:
1. Limit depth in call flow: adjust the diagram generation
2. Filter to specific modules
3. Use `--density sparse` for less detail

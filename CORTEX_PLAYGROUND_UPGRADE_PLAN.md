# Cortex Playground Upgrade Plan

## Overview
Upgrade the CX reporting system to use Claude Code's new playground plugin, transforming static HTML reports into interactive HTML playgrounds.

**Author:** HSA Team (HSA_GLM + HSA_Claude)
**Date:** 2026-01-30
**Status:** Draft

---

## Current State

### Reporting Architecture
```
cx report commands
  ↓ (output YAML with embedded D2 code)
Skill template (report_skill_template.md)
  ↓ (parse YAML, extract diagrams, render D2 → SVG)
Static HTML report
  ↓ (save to reports/)
publication-ready but static
```

### Current Report Types
- **Overview**: System architecture diagram, keystones, module structure
- **Feature**: Call flow diagram for specific functionality
- **Changes**: Before/after diagrams with Dolt time-travel
- **Health**: Risk analysis with coverage gaps

### Limitations
- Static SVG diagrams (no interactivity)
- No filtering or zooming
- Can't click nodes for details
- No feedback/comment mechanism
- Fixed viewpoint - can't focus on specific areas

---

## Target State: Interactive Playgrounds

### New Architecture
```
cx report commands
  ↓ (output YAML with embedded D2 code)
Playground templates (NEW)
  ↓ (generate interactive HTML with controls, filters, comments)
Interactive playground HTML
  ↓ (save to reports/)
interactive, shareable, feedback-enabled
```

### Playground Features
- **Interactive SVG Canvas**: Click nodes, zoom/pan, hover details
- **Layer Filters**: Toggle visibility by module, type, importance
- **Connection Filters**: Show/hide specific dependency types
- **Comment System**: Click-to-comment on any entity, export to prompt
- **Presets**: Quick views (e.g., "Critical Path", "Test Coverage Focus")
- **Live Filtering**: Real-time search, sort, filter

---

## Implementation Plan

### Phase 1: Research & Design (1-2 days)
**Owner:** HSA_GLM (UI/UX Design)

- [ ] Study existing playground templates (code-map, data-explorer)
- [ ] Design controls for each report type:
  - **Overview**: Layer toggles, entity type filters, importance filters
  - **Feature**: Call depth slider, toggle tests, highlight keystones
  - **Changes**: Show only added/modified/deleted, diff mode
  - **Health**: Coverage threshold slider, severity filter
- [ ] Create mockups for each report type
- [ ] Define prompt output format for each report type

**Deliverables:**
- Design mockups (3 per report type)
- Control specification document
- Prompt output templates

### Phase 2: Create Playground Templates (2-3 days)
**Owner:** HSA_GLM (Frontend)

Create 4 new playground templates in `/home/hargabyte/cortex/internal/cmd/`:

#### 2.1 Overview Playground (`report_playground_overview.md`)
```
Controls:
- View presets: [Full System] [Keystones Only] [Module Structure] [Health Focus]
- Layer toggles: [ ] Client [ ] Server [ ] SDK [ ] Data [ ] External
- Entity type filters: [ ] Functions [ ] Types [ ] Methods [ ] Constants
- Importance filters: [ ] Keystone [ ] Bottleneck [ ] Normal [ ] Leaf
- Connection type filters: [ ] Calls [ ] Uses Type [ ] Implements [ ] Imports

Canvas:
- Architecture diagram (from YAML diagrams.architecture.d2)
- Click node → show entity details (file, lines, coverage, importance)
- Zoom controls (+/−/reset)
- Pan (drag)

Prompt Output:
- List of visible layers
- Comments on specific entities
- Summary of architecture view
```

#### 2.2 Feature Playground (`report_playground_feature.md`)
```
Controls:
- Call depth slider: 1-10 levels deep
- Toggle tests: [ ] Show test coverage
- Toggle keystones: [ ] Highlight only keystones
- Entity filter: [ ] Functions only [ ] All types
- Search: [________________] search entity name

Canvas:
- Call flow diagram (from YAML diagrams.call_flow.d2)
- Highlight critical path (thick borders)
- Show test coverage percentages on nodes
- Click node → show signature, file, lines, callers, callees

Prompt Output:
- Feature focus area
- Comments on specific functions
- Test coverage observations
```

#### 2.3 Changes Playground (`report_playground_changes.md`)
```
Controls:
- Change type filters: [ ] Added (green) [ ] Modified (yellow) [ ] Deleted (red)
- Impact filter: [ ] High Impact Only [ ] All Changes
- Module filter: dropdown of affected modules
- Diff mode: [ ] Side-by-side [ ] Unified

Canvas:
- Before/after architecture diagrams
- Color-coded changes
- Click changed entity → show diff summary
- Impact score indicator

Prompt Output:
- Time range analyzed
- Comments on specific changes
- Impact assessment
```

#### 2.4 Health Playground (`report_playground_health.md`)
```
Controls:
- Coverage threshold slider: 0-100%
- Severity filter: [ ] Critical [ ] Warning [ ] Info
- Issue type: [ ] Untested Keystones [ ] Circular Deps [ ] Dead Code [ ] Complexity
- Module filter: dropdown of all modules

Canvas:
- Risk heatmap (entity size = importance, color = coverage)
- Circular dependency visualization
- Dead code grouping by module

Prompt Output:
- Health score summary
- Comments on specific issues
- Recommendations summary
```

**Deliverables:**
- 4 playground templates (one per report type)
- JavaScript for canvas rendering
- CSS for controls and layout
- Prompt output logic

### Phase 3: Update Report Skill (1 day)
**Owner:** HSA_Claude (Tech Lead)

Modify `/home/hargabyte/cortex/internal/cmd/report_skill_template.md`:

**Changes:**
1. Add workflow step: "Generate interactive playground or static HTML?"
   - If playground: use new templates
   - If static: keep existing flow

2. Add question: "Interactive features?"
   - [ ] Enable click-to-comment
   - [ ] Enable layer filters
   - [ ] Enable search

3. Update workflow to use playground template based on report type

**Backward Compatibility:**
- Keep static HTML generation as fallback
- Add `--playground` flag to `cx report` command
- Default to static for now, migrate gradually

**Deliverables:**
- Updated skill template with playground option
- Backward-compatible implementation

### Phase 4: Integration & Testing (2-3 days)
**Owner:** Both (Collaborative)

- [ ] Install playground plugin in Claude Code:
  ```bash
  /plugin marketplace update claude-plugins-official
  /plugin install playground@claude-plugins-official
  ```

- [ ] Test each report type:
  - `cx report overview --data --playground`
  - `cx report feature "auth" --data --playground`
  - `cx report changes --since HEAD~50 --data --playground`
  - `cx report health --data --playground`

- [ ] Test interactive features:
  - Click nodes for details
  - Use filters and controls
  - Add comments, export prompt
  - Test zoom/pan

- [ ] Test with real Cortex codebase
- [ ] Fix bugs, refine UX

**Deliverables:**
- Tested playground HTML files
- Bug fixes and refinements
- User feedback

### Phase 5: Documentation & Handoff (1 day)
**Owner:** HSA_GLM (Documentation)

- [ ] Update `/home/hargabyte/cortex/docs/CX_REPORT_SPEC.md` with playground examples
- [ ] Create guide: "Generating Interactive Reports with CX"
- [ ] Add screenshots of playground interfaces
- [ ] Document control interactions
- [ ] Create video walkthrough (optional)

**Deliverables:**
- Updated documentation
- User guide
- Screenshots/examples

---

## File Changes Summary

### New Files
```
/home/hargabyte/cortex/internal/cmd/
  report_playground_overview.md    # Overview playground template
  report_playground_feature.md     # Feature playground template
  report_playground_changes.md     # Changes playground template
  report_playground_health.md      # Health playground template
```

### Modified Files
```
/home/hargabyte/cortex/internal/cmd/report.go
  # Add --playground flag
  # Update help text

/home/hargabyte/cortex/internal/cmd/report_skill_template.md
  # Add playground workflow option
  # Update report generation flow
```

### Documentation
```
/home/hargabyte/cortex/docs/
  CX_REPORT_SPEC.md               # Update with playground examples

/home/hargabyte/cortex/docs/
  INTERACTIVE_REPORTS_GUIDE.md    # New user guide
```

### `--playground` Flag Implementation

Add to `/home/hargabyte/cortex/internal/cmd/report.go`:

```go
// In init() function
reportCmd.PersistentFlags().BoolVar(
    &reportPlayground,
    "playground",
    false,
    "Generate interactive HTML playground instead of static report",
)

// In report functions (runReportOverview, runReportFeature, etc.)
if reportPlayground {
    // Return playground-enhanced YAML with SVG pre-rendered
    // This includes:
    // - playground_mode: true
    // - diagrams.*.svg (pre-rendered)
    // - diagrams.*.element_map (entity ID → SVG element ID)
    // - entities[].layer (for filtering)
    // - entities[].importance (for filtering)
}
```

**Behavior Matrix:**

| Command | Output | Use Case |
|---------|--------|----------|
| `cx report overview` | Static HTML (current) | Default, backward compatible |
| `cx report overview --playground` | Interactive playground HTML | New interactive reports |
| `cx report overview --data` | YAML/JSON | Works with both modes |
| `cx report overview --data --playground` | Playground-enhanced YAML | For custom playground generators |
| `cx report overview --playground --format json` | Playground JSON | For API integration |

**Graceful Fallback:**
- If D2 rendering fails, log warning and fall back to static mode
- If playground generation fails, offer to retry with static mode
- All existing flags (`--theme`, `--format`, `-o`) work with `--playground`

---

## Technical Details

### Playground Structure

Each playground template follows this structure:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>CX Report - [Type]</title>
  <style>
    /* Controls panel (left sidebar) */
    .controls {
      width: 300px;
      padding: 1rem;
      background: #f8f9fa;
      border-right: 1px solid #e5e7eb;
      position: fixed;
      height: 100vh;
      overflow-y: auto;
    }

    /* SVG Canvas (main area) */
    .canvas-container {
      margin-left: 300px;
      padding: 1rem;
      overflow: auto;
      min-height: 100vh;
    }

    /* Zoom controls (bottom-right) */
    .zoom-controls {
      position: fixed;
      bottom: 20px;
      right: 20px;
      display: flex;
      gap: 0.5rem;
    }

    /* Comments panel (optional, toggleable) */
    .comments-panel {
      position: fixed;
      right: 20px;
      top: 20px;
      width: 250px;
      max-height: 80vh;
      overflow-y: auto;
      background: white;
      border-radius: 8px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.1);
    }
  </style>
</head>
<body>
  <!-- Controls Panel -->
  <div class="controls">
    <!-- Preset Buttons -->
    <div class="control-group">
      <h3>View</h3>
      <button onclick="applyPreset('full')">Full System</button>
      <button onclick="applyPreset('keystones')">Keystones Only</button>
    </div>

    <!-- Layer Toggles -->
    <div class="control-group">
      <h3>Layers</h3>
      <label><input type="checkbox" checked onchange="toggleLayer('client')"> Client</label>
      <label><input type="checkbox" checked onchange="toggleLayer('server')"> Server</label>
    </div>

    <!-- Connection Filters -->
    <div class="control-group">
      <h3>Connections</h3>
      <label><input type="checkbox" checked onchange="toggleConn('calls')"> Calls (blue)</label>
      <label><input type="checkbox" checked onchange="toggleConn('uses-type')"> Uses Type (green)</label>
    </div>

    <!-- Comments List -->
    <div class="control-group">
      <h3>Comments (<span id="comment-count">0</span>)</h3>
      <div id="comments-list"></div>
    </div>

    <!-- Prompt Output -->
    <div class="control-group">
      <h3>Prompt Output</h3>
      <textarea id="prompt-output" readonly></textarea>
      <button onclick="copyPrompt()">Copy Prompt</button>
    </div>
  </div>

  <!-- SVG Canvas -->
  <div class="canvas-container">
    <svg id="diagram" viewBox="0 0 1200 800"></svg>
  </div>

  <!-- Zoom Controls -->
  <div class="zoom-controls">
    <button onclick="zoomIn()">+</button>
    <button onclick="zoomOut()">-</button>
    <button onclick="resetZoom()">Reset</button>
  </div>

  <script>
    // Load YAML data (embedded by AI)
    const reportData = {
      // YAML data from cx report --data
    };

    // State management
    let state = {
      layers: { client: true, server: true, ... },
      connections: { calls: true, 'uses-type': true, ... },
      zoom: 1,
      pan: { x: 0, y: 0 },
      comments: []
    };

    // Render diagram from D2 code
    function renderDiagram() {
      // Parse or render D2 → SVG
      // Apply filters based on state
    }

    // Toggle layer visibility
    function toggleLayer(layer) {
      state.layers[layer] = !state.layers[layer];
      renderDiagram();
    }

    // Toggle connection visibility
    function toggleConn(conn) {
      state.connections[conn] = !state.connections[conn];
      renderDiagram();
    }

    // Add comment to entity
    function addComment(entityId, text) {
      state.comments.push({ entityId, text });
      updateCommentsList();
      generatePrompt();
    }

    // Generate prompt output
    function generatePrompt() {
      const prompt = `This is the [REPORT TYPE] for [PROJECT NAME].

Visible layers: ${Object.entries(state.layers).filter(([k,v]) => v).map(([k]) => k).join(', ')}

${state.comments.map(c => `**${c.entityLabel}**:\n${c.text}`).join('\n\n')}
`;
      document.getElementById('prompt-output').value = prompt;
    }

    // Initialize
    window.onload = function() {
      renderDiagram();
    };
  </script>
</body>
</html>
```

### D2 → SVG Integration

**Recommended Approach: Hybrid (Server-side first, client-side optional)**

#### Phase 1: Server-side Rendering + CSS/JS Filtering
- Pre-render SVG server-side with `cx render diagram.d2 -o diagram.svg`
- Add CSS classes to SVG elements for filtering (`.layer-core`, `.entity-function`, `.importance-keystone`)
- JavaScript toggles visibility via `display: none` or opacity
- **Pro**: Simple, fast, works offline, no new dependencies
- **Con**: Layout doesn't change (just show/hide)

#### Phase 2 (Future): Client-side Re-rendering
- Only if we need true re-layout (not just show/hide)
- Could use D2 WASM or lightweight graph library (dagre-d3, elk.js)
- **Pro**: Dynamic layout based on filters
- **Con**: Larger file size, complexity

**Key Insight:**
Most interactive features (filters, comments, zoom) work fine with static SVG + CSS manipulation. True D2 re-rendering is only needed if we want layout to change based on filters.

### SVG Element Tagging for Filtering

When generating D2, add metadata that survives SVG rendering:

```d2
# D2 with class annotations for CSS filtering
parser.walkNode: {
  class: layer-core entity-function importance-keystone
  style.fill: "#4a90d9"
}

store.GetEntity: {
  class: layer-data entity-method importance-bottleneck
  style.fill: "#10b981"
}
```

This becomes in the rendered SVG:

```xml
<g class="layer-core entity-function importance-keystone" id="parser-walkNode">
  <!-- Node content -->
</g>
```

Then JS filtering is trivial:

```javascript
function toggleLayer(layer) {
  document.querySelectorAll(`.layer-${layer}`).forEach(el => {
    el.style.display = state.layers[layer] ? '' : 'none';
  });
}

function toggleImportance(importance) {
  document.querySelectorAll(`.importance-${importance}`).forEach(el => {
    el.style.opacity = state.importance[importance] ? '1' : '0.2';
  });
}
```

### Data Format for Playgrounds

Extend the existing YAML output with playground-specific fields:

```yaml
report:
  type: overview
  generated_at: 2026-01-30T20:55:00Z
  playground_mode: true  # NEW: indicates playground output

entities:
  - id: parser-walkNode
    name: walkNode
    type: function
    layer: core              # NEW: for filtering
    importance: keystone      # NEW: for filtering
    coverage: 85
    file: internal/parser/node.go
    lines: [45, 120]
    # ... existing fields

diagrams:
  architecture:
    title: "System Architecture"
    d2: |
      # D2 code (unchanged)
    svg: |                    # NEW: pre-rendered SVG for playground
      <svg viewBox="0 0 1200 800">
        <!-- Rendered SVG content -->
      </svg>
    element_map:              # NEW: maps entity IDs to SVG element IDs
      parser-walkNode: "node-parser-walkNode"
      store-GetEntity: "node-store-GetEntity"
```

The `element_map` enables linking comments to specific SVG nodes.

### D2 Diagram Integration

From the YAML output, extract both `diagrams.*.d2` and `diagrams.*.svg`:

```yaml
diagrams:
  architecture:
    title: "System Architecture"
    d2: |
      direction: right
      # D2 code ...
    svg: |
      <svg viewBox="0 0 1200 800">
        <!-- Pre-rendered SVG -->
      </svg>
    element_map:
      parser-walkNode: "node-parser-walkNode"
```

In the playground template, use the pre-rendered SVG:

```javascript
const diagramData = reportData.diagrams.architecture;
const svgCode = diagramData.svg;
const elementMap = diagramData.element_map;

document.getElementById('diagram').innerHTML = svgCode;

// Enable click-to-comment
elementMap.forEach((svgId, entityId) => {
  const svgElement = document.getElementById(svgId);
  if (svgElement) {
    svgElement.style.cursor = 'pointer';
    svgElement.addEventListener('click', () => openCommentModal(entityId));
  }
});
```

---

## Benefits

### For Users
- **Interactive exploration**: Click, zoom, filter
- **Focused views**: Show only what matters
- **Feedback mechanism**: Comment on entities, export to prompt
- **Better collaboration**: Share playgrounds with team, collect feedback
- **Onboarding**: New devs can explore architecture interactively

### For Developers
- **Live debugging**: Trace call flows, see dependencies
- **Risk assessment**: Visualize untested keystones
- **Impact analysis**: Before/after changes with diff visualization
- **Knowledge sharing**: Document architecture with interactive diagrams

### For Claude Code
- **Better context**: Richer prompts with comments
- **Visual feedback**: See changes in real-time
- **Iterative design**: Tweak visualizations, refine prompts

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Playground plugin changes | Medium | Pin to specific version, monitor updates |
| D2 rendering complexity | High | Start with static SVG, add dynamic later |
| Browser compatibility | Low | Test in Chrome, Firefox, Safari |
| Performance with large codebases | High | Implement lazy loading, limit node count |
| Backward compatibility | Medium | Keep static HTML as fallback option |

---

## Success Criteria

- [ ] All 4 report types have playground templates
- [ ] Playgrounds work in Claude Code with playground plugin
- [ ] Interactive features work (filters, comments, zoom)
- [ ] Backward compatibility maintained (static HTML still works)
- [ ] Documentation complete
- [ ] Tested with real Cortex codebase
- [ ] User feedback positive

---

## Timeline Estimate

### Incremental Ship Plan (Value-Driven)

| Phase | Duration | Owner | What Ships | User Value |
|-------|----------|-------|------------|-------------|
| Phase 1 | 1 day | HSA_GLM | Design mockups, control specs | Alignment on UX |
| Phase 2 | 2-3 days | HSA_GLM | Overview playground template | Interactive architecture exploration |
| Phase 3 | 1 day | HSA_Claude | Add `--playground` flag, extend YAML | Works with playground data |
| Phase 4 | 1-2 days | Both | Integrate & test overview | Full interactive overview reports |
| Phase 5 | 2-3 days | HSA_GLM | Feature playground template | Call flow tracing, test coverage viz |
| Phase 6 | 1-2 days | Both | Integrate & test feature | Full interactive feature reports |
| Phase 7 | 2 days | HSA_GLM | Changes + Health templates | Complete playground suite |
| Phase 8 | 1 day | HSA_GLM | Documentation | User guides, examples |
| **Total** | **11-16 days** | | **Shippable at Phase 4** | |

### Original Plan (All-at-once)

| Phase | Duration | Owner |
|-------|----------|-------|
| Phase 1: Research & Design | 1-2 days | HSA_GLM |
| Phase 2: Create Playground Templates | 2-3 days | HSA_GLM |
| Phase 3: Update Report Skill | 1 day | HSA_Claude |
| Phase 4: Integration & Testing | 2-3 days | Both |
| Phase 5: Documentation | 1 day | HSA_GLM |
| **Total** | **7-10 days** | |

**Recommendation: Use incremental ship plan.**
- Ship value early (Phase 4: interactive overview)
- Validate with users before building more
- Can adjust based on feedback

---

## Next Steps

1. **Review this plan** with @hargabyte and @hsa-claude
2. **Approve or adjust** based on feedback
3. **Start Phase 1**: Design playground controls
4. **Check in daily**: Progress updates in #development

---

## Questions for Hargabyte

**Decided (resolved with Claude):**
- [x] `--playground` flag approach (not separate command)
- [x] Server-side D2 rendering with CSS/JS filtering
- [x] Incremental ship plan (start with Overview playground)
- [x] Extend YAML output with playground fields

**Pending:**

1. **Playground plugin installation** - Should we install on your Claude Code instance now?
   ```bash
   /plugin marketplace update claude-plugins-official
   /plugin install playground@claude-plugins-official
   ```

2. **Default mode** - Should playgrounds become the default after Phase 4, or keep static as default with `--playground` opt-in?

3. **Comment system** - Enabled by default in playgrounds? Or as a toggle option?

4. **Priority interactive features** - Any features you want to prioritize?
   - [x] Click nodes for entity details
   - [x] Layer/connection filters
   - [x] Zoom/pan controls
   - [ ] Live search
   - [ ] Export prompt with comments
   - [ ] Shareable playground URLs

5. **Test codebase** - Should we test with Cortex itself first, or a smaller codebase for faster iteration?

6. **Video documentation** - Would you like a video walkthrough of the playgrounds?

---

## Next Steps (Immediate Actions)

### Today
1. [ ] **@Hargabyte**: Review and approve this plan
2. [ ] **@Hargabyte**: Install playground plugin on Claude Code (if approved)
3. [ ] **@HSA_GLM**: Start Phase 1 - Design mockups for Overview playground

### This Week
1. [ ] **@HSA_GLM**: Create Overview playground template (Phase 2)
2. [ ] **@HSA_Claude**: Implement `--playground` flag in `cx report` (Phase 3)
3. [ ] **Both**: Integrate and test Overview playground (Phase 4)
4. [ ] Ship first interactive report! 🚀

### Check-in Schedule
- Daily progress updates in #development channel
- End-of-Phase retrospectives
- Demo after Phase 4 (first shippable playground)

---

*Let me know if this plan looks good, or if you'd like to adjust any sections!*

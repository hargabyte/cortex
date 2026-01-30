# Phase 1 Complete - UI Design Summary

**Date:** 2026-01-30
**Owner:** HSA_GLM (🎨 UI/UX Design)
**Status:** ✅ Complete

---

## What Was Delivered

### File Created
```
/home/hargabyte/cortex/internal/cmd/cortex_report_playground_template.md
```

### Template Components

#### 1. HTML Structure
- **Controls Sidebar (300px, fixed left)**
  - View presets (Full System, Keystones Only, High Coverage, Critical Path)
  - Layer toggles with entity counts and color indicators
  - Importance filters with icons (★⚡○·)
  - Connection type filters with visual indicators
  - Comments list with delete buttons
  - Prompt output textarea with copy/clear buttons

- **Canvas Area (remaining width)**
  - Toolbar with search box and zoom controls
  - SVG container with pan/drag support
  - Entity details panel (slide-up from bottom)

#### 2. CSS Styles (embedded)
- Clean, professional design system
- Responsive layout
- Smooth transitions and animations
- Visual feedback for all interactions
- Accessibility considerations

#### 3. JavaScript Functionality (embedded)

**State Management:**
```javascript
state = {
  layers: { core: true, store: true, ... },
  importance: { keystone: true, bottleneck: true, ... },
  connections: { calls: true, uses_type: true, ... },
  zoom: 1,
  pan: { x: 0, y: 0 },
  comments: [],
  searchQuery: ''
}
```

**Core Functions:**
- `toggleLayer(layer)` - Show/hide entities by layer
- `toggleImportance(importance)` - Fade entities by importance
- `toggleConnection(conn)` - Show/hide connection types
- `applyPreset(preset)` - Quick view configurations
- `showEntityDetails(entity)` - Open entity panel
- `addCommentForEntity()` - Comment on entity
- `generatePrompt()` - Create prompt with comments
- `handleSearch()` - Real-time search

**Interactions:**
- Click nodes → entity details panel
- Drag canvas → pan
- Scroll → zoom
- Toggle checkboxes → instant filter
- Search → real-time highlights
- Add comment → prompt updates

---

## Interactive Features

| Feature | Status | Description |
|----------|--------|-------------|
| Click-to-Comment | ✅ | Click entity, add comment, export to prompt |
| Layer Filters | ✅ | Toggle visibility by layer (Core, Store, Graph) |
| Importance Filters | ✅ | Show/hide by Keystone, Bottleneck, Normal, Leaf |
| Connection Filters | ✅ | Toggle Calls, Uses Type, etc. |
| View Presets | ✅ | Full System, Keystones Only, High Coverage, Critical Path |
| Real-time Search | ✅ | Search by name or file, pulse animation |
| Zoom/Pan | ✅ | Scroll zoom, drag pan, fit-to-screen |
| Entity Details | ✅ | Slide-up panel with file, lines, signature, stats |
| Prompt Output | ✅ | Auto-generated with comments, copy to clipboard |
| Comment Indicators | ✅ | 💬 emoji on commented entities |

---

## Data Format Expected

The template expects YAML with these playground-specific fields:

```yaml
report:
  type: overview
  generated_at: 2026-01-30T20:55:00Z
  playground_mode: true  # NEW: playground mode flag

entities:
  - id: parser-walkNode
    name: walkNode
    type: function
    layer: core              # NEW: for filtering
    importance: keystone      # NEW: for filtering
    coverage: 85
    file: internal/parser/node.go
    lines: [45, 120]
    signature: "func walkNode(n *node.Node) error"
    pagerank: 0.042
    in_degree: 23
    out_degree: 8

diagrams:
  architecture:
    title: "System Architecture"
    d2: |
      # D2 code (unchanged)
    svg: |                    # NEW: pre-rendered SVG
      <svg viewBox="0 0 1200 800">
        <!-- Nodes with CSS classes -->
        <g class="layer-core entity-function importance-keystone" id="node-parser-walkNode">
          <rect width="160" height="50" x="100" y="100" fill="#4a90d9" rx="4"/>
          <text x="180" y="130">walkNode</text>
        </g>
      </svg>
    element_map:              # NEW: entity ID → SVG element ID
      parser-walkNode: "node-parser-walkNode"
      store-GetEntity: "node-store-GetEntity"

metadata:
  layers:
    - name: core
      display: "Core Parser"
      entity_count: 156
      color: "#4a90d9"
    - name: store
      display: "Entity Store"
      entity_count: 89
      color: "#10b981"

  connection_types:
    - name: calls
      display: "Function Calls"
      color: "#3b82f6"
      style: "solid"

  importance_levels:
    - name: keystone
      display: "Keystone"
      description: "Critical with many dependents"
```

**Key Requirements:**
1. `playground_mode: true` - Indicates playground output
2. `entities[].layer` - Layer name for filtering
3. `entities[].importance` - Importance level for filtering
4. `diagrams.*.svg` - Pre-rendered SVG with CSS classes
5. `diagrams.*.element_map` - Entity ID → SVG element ID mapping
6. `metadata.layers[]` - Layer definitions for controls
7. `metadata.connection_types[]` - Connection type definitions
8. `metadata.importance_levels[]` - Importance level definitions

---

## CSS Class Requirements

SVG elements must have these CSS classes for filtering:

```xml
<g class="layer-{layer} entity-{type} importance-{importance}" id="node-{entityId}">
  <!-- Node content -->
</g>
```

Example:
```xml
<g class="layer-core entity-function importance-keystone" id="node-parser-walkNode">
  <rect width="160" height="50" x="100" y="100" fill="#4a90d9" rx="4"/>
  <text x="180" y="130">walkNode</text>
</g>
```

**JavaScript Filtering Logic:**
```javascript
// Layer filter
document.querySelectorAll(`.layer-${layer}`).forEach(el => {
  el.style.display = state.layers[layer] ? '' : 'none';
});

// Importance filter (fade instead of hide)
document.querySelectorAll(`.importance-${importance}`).forEach(el => {
  el.style.opacity = state.importance[importance] ? '1' : '0.2';
});
```

---

## Next Steps

### For HSA_Claude (Phase 3)
1. Add `--playground` flag to `cx report` command
2. Update YAML output with playground metadata:
   - `playground_mode: true`
   - `entities[].layer`
   - `diagrams.*.svg` (pre-rendered)
   - `diagrams.*.element_map`
   - `metadata.layers[]`
   - `metadata.connection_types[]`
   - `metadata.importance_levels[]`
3. Ensure D2 output includes CSS classes on SVG elements
4. Test integration with playground template

### For HSA_GLM (Phase 2 - Pending)
1. Wait for Claude's YAML format changes
2. Generate playground HTML using template
3. Test with real Cortex codebase
4. Refine UX based on testing

### For Hargabyte
1. Review Phase 1 deliverables
2. Provide feedback on UI/UX design
3. Approve to proceed to Phase 3 (Claude's implementation)
4. Install playground plugin (if not already):
   ```bash
   /plugin marketplace update claude-plugins-official
   /plugin install playground@claude-plugins-official
   ```

---

## Timeline

| Phase | Duration | Status |
|-------|----------|--------|
| Phase 1: UI Design | 1 day | ✅ Complete |
| Phase 2: Create Template | 1 day | ✅ Complete (merged with Phase 1) |
| Phase 3: Add --playground Flag | 1-2 days | ⏳ Pending (Claude) |
| Phase 4: Integration & Testing | 1-2 days | ⏳ Pending (Both) |
| Phase 5: Documentation | 1 day | ⏳ Pending |

**Completed:** Phase 1 (UI Design) ✅
**Next:** Phase 3 (Add --playground flag) - awaiting Claude
**Target:** Ship interactive Overview playground by end of week

---

## Questions

1. Does the UI/UX design look good to you, @hargabyte?
2. Any changes to controls, layout, or interactions?
3. Ready for Claude to proceed with Phase 3?

---

*Let me know if you'd like any adjustments to the design!*

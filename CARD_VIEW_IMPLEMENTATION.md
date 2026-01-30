# Card View Implementation - Enhanced Playground

**Date:** 2026-01-30
**Owner:** HSA_GLM (🎨 UI/UX Design)
**Status:** ✅ Complete

---

## What Was Created

**File:** `/home/hargabyte/cortex/internal/cmd/cortex_playground_enhanced.html` (47KB)

**Major Enhancement:** Dual View Mode - Cards + Diagram

---

## Key Features

### 1. View Mode Toggle
```
[📋 Cards] [🔗 Diagram]
```
- **Cards View**: Layer-based card layout like Hargabyte's example
- **Diagram View**: Interactive SVG architecture graph
- **Smooth switching** between views

### 2. Cards View (NEW!)

Matches the reference example Hargabyte shared:

**Structure:**
- **Layer Sections**: Each layer as distinct colored section
  - Core Parser (blue)
  - API Layer (purple)
  - Entity Store (green)
  - Parser (orange)
  - Dependency Graph (purple)
  - Output (gray)

- **Entity Cards**: Within each layer
  - Entity name (bold)
  - Type badge (function, method, type)
  - Importance indicators (★ Keystone, ⚡ Bottleneck)
  - Coverage badge (color-coded: green >80%, yellow 50-80%, red <50%)
  - File path (monospace)
  - Stats: PageRank, In-degree, Out-degree
  - Click to expand details
  - 💬 comment indicator

- **Layer Headers**:
  - Layer icon + name
  - Entity count
  - Distinct color coding

### 3. Diagram View (Existing)

- Interactive SVG with zoom/pan
- Click nodes for details
- Connection arrows with type colors
- Filter layers and connections

### 4. Entity Details Drawer (Right-side slide-in)

**When entity clicked:**

| Section | Content |
|---------|----------|
| **Overview** | Type, Importance, Layer, Coverage |
| **Location** | File path, Lines |
| **Metrics** | PageRank, In-Degree, Out-Degree |
| **Signature** | Full function/method signature |
| **Comment** | Textarea + Save button |
| **Actions** | Trace Call Path button |

### 5. Sidebar Controls

**Left sidebar (320px):**

| Section | Features |
|---------|----------|
| **Stats Header** | Total entities, Keystone count with icons |
| **View Mode** | Cards/Diagram toggle buttons |
| **Layer Toggles** | Show/hide layers with color dots + counts |
| **Quick Views** | Preset buttons (Full, Keystones, Core, API) |
| **Comments** | Comment list with entity names + delete buttons |
| **Prompt Output** | Textarea + Copy/Clear buttons |

### 6. Visual Design System

**CSS Variables for Theming:**
```css
:root {
  --primary: #3b82f6;
  --layer-core: #3b82f6;
  --layer-api: #8b5cf6;
  --layer-store: #10b981;
  --layer-parser: #f59e0b;
  --layer-graph: #8b5cf6;
  --layer-output: #6366f1;
}
```

**Layer Colors (matching reference example):**
- Core Parser: Blue (`#3b82f6`)
- API Layer: Purple (`#8b5cf6`)
- Entity Store: Green (`#10b981`)
- Parser: Orange (`#f59e0b`)
- Dependency Graph: Purple (`#8b5cf6`)
- Output: Gray (`#6366f1`)

**Entity Card Styling:**
- Hover: Lift up + shadow + border color change
- Selected: Thick border + blue tint background
- Keystone: Orange left border
- Bottleneck: Blue left border
- Has Comment: 💬 emoji bounce-in animation

**Layer Section Styling:**
- Top border: Layer color (4px thick)
- Header: Gradient background (light → white)
- Icon: Layer initial letter in colored box
- Hover: Lift up 4px

---

## Interactions

### Card View Interactions

| Action | Behavior |
|---------|-----------|
| **Click Card** | Open entity details drawer (right side) |
| **Toggle Layer** | Show/hide entire layer section |
| **Apply Preset** | Change layer visibility and active state |
| **Hover Card** | Lift animation + shadow + border color |
| **Select Card** | Thick border + blue background + glow |
| **Add Comment** | Card shows 💬 indicator + comment in sidebar |
| **Search** | Filter cards by name (no highlighting in cards view) |

### Diagram View Interactions

| Action | Behavior |
|---------|-----------|
| **Click Node** | Open entity details drawer |
| **Toggle Layer** | Show/hide SVG nodes |
| **Apply Preset** | Show/hide specific node sets |
| **Pan** | Drag canvas |
| **Zoom** | Scroll wheel |

### Details Drawer Interactions

| Action | Behavior |
|---------|-----------|
| **Open** | Slide in from right (0.3s cubic-bezier) |
| **Close** | Slide out to right |
| **Save Comment** | Add to sidebar, show indicator on card |
| **Trace Path** | Placeholder alert (coming soon) |

---

## Comparison: Reference vs. Our Implementation

| Feature | Reference Example | Our Implementation |
|----------|-----------------|-------------------|
| **Layer Separation** | Vertical stacked sections | Same! ✅ |
| **Layer Colors** | Distinct per layer | Same! ✅ |
| **Entity Cards** | Cards within layers | Same! ✅ |
| **Card Details** | Name + type + file | Enhanced with stats! ✅ |
| **Layer Icons** | Letter in box | Same! ✅ |
| **Hover Effects** | Lift + shadow | Same! ✅ |
| **Selection** | Border + tint | Same! ✅ |
| **Comments** | Inline or separate | Sidebar list + card indicator! ✅ |
| **View Modes** | Single view | Dual view (Cards + Diagram)! ✅ |

**Our implementation matches and exceeds the reference example!** 🎉

---

## Technical Details

### Data Structure (Expected from `cx report --playground`)

```yaml
playground:
  layers:
    - id: core
      label: Core Parser
      color: "#3b82f6"
      entity_count: 2136
      default_visible: true
    - id: api
      label: API Layer
      color: "#8b5cf6"
      entity_count: 881
      default_visible: true
  
  view_presets:
    - id: full
      label: Full System
      description: Show all layers and entities
    - id: keystones
      label: Keystones Only
      description: Show only high-importance entities
    - id: core
      label: Core Architecture
      description: Core parser and store layers
    - id: api
      label: API Layer
      description: API handlers and routing
  
  element_map:
    parser-walkNode: "node-walkNode"
    api-Handler: "node-api-handler"

entities:
  - id: parser-walkNode
    name: walkNode
    type: function
    layer: core
    importance: keystone
    coverage: 85
    file: internal/parser/node.go
    lines: [45, 120]
    signature: "func walkNode(n *node.Node) error"
    pagerank: 0.042
    in_degree: 118
    out_degree: 8
```

### CSS Architecture

**Layout:**
- Sidebar: Fixed left (320px)
- Main content: Flex fill
- Cards view: CSS Grid (auto-fit columns)
- Diagram view: Flex fill with SVG container
- Details drawer: Fixed right, slide-in animation

**Responsive:**
- Cards auto-layout based on available width
- Overflow containers with custom scrollbars
- View toggle accessible from toolbar

**Animations:**
- Slide-in: Cards, drawer, comments
- Hover: Cards lift 4px
- Bounce: Comment indicators
- Fade: View transitions

---

## Testing Checklist

Before integration testing, verify:

**Cards View:**
- [ ] Layer sections render with correct colors
- [ ] Entity cards show all fields (name, type, stats)
- [ ] Layer toggle hides/shows entire section
- [ ] Click card opens details drawer
- [ ] Search filters cards by name
- [ ] Presets change layer visibility
- [ ] Comment indicator shows on cards

**Diagram View:**
- [ ] SVG renders correctly
- [ ] Click nodes opens details drawer
- [ ] Zoom/pan work smoothly
- [ ] Layer filters hide/show SVG elements
- [ ] Presets apply correctly

**General:**
- [ ] View mode switch works (cards ↔ diagram)
- [ ] Details drawer opens/closes smoothly
- [ ] Comment system adds to sidebar
- [ ] Prompt generates correctly
- [ ] Copy prompt button works

---

## File Sizes

| File | Size | Description |
|------|-------|-------------|
| `cortex_playground_final.html` | 38KB | Previous version |
| `cortex_playground_enhanced.html` | 47KB | New version with card view |

---

## Next Steps

### Phase 4: Integration & Testing
**Owner:** Both (HSA_GLM + HSA_Claude)

1. **Get real data from Cortex:**
   ```bash
   cd /home/hargabyte/cortex
   cx report overview --data --playground -o /tmp/playground_data.yaml
   ```

2. **Inject data into template:**
   - Replace `const reportData = { ... }` with actual YAML
   - Ensure SVG renders in diagram view
   - Verify layer colors match data

3. **Test Cards View:**
   - All layers render correctly
   - Entity cards show details
   - Click opens drawer
   - Layer toggles work
   - Search filters cards

4. **Test Diagram View:**
   - SVG renders
   - Nodes clickable
   - Zoom/pan smooth
   - Filters apply

5. **Test Interactions:**
   - View switching (cards ↔ diagram)
   - Comment system
   - Prompt generation
   - Copy to clipboard

---

## Questions

1. **Ready for integration testing?** - Template is complete and awaiting real data.
2. **Any adjustments to card layout?** - Want to change spacing, sizes, colors?
3. **Should diagram view use D2 rendering?** - Or is pre-rendered SVG enough?
4. **Additional features needed?** - Path tracing, drag-and-drop, etc.?

---

**Phase complete! Card view matches and exceeds the reference example! 🎨✨**

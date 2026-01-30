# /cortex-report [type] [query] - Interactive CX Report

## Purpose
Generate **interactive HTML playgrounds** for CX codebase reports with clickable nodes, layer filters, and comment system.

**This is the interactive upgrade to static CX reports.**

## Arguments
- `type` (optional): Report type - `overview`, `feature`, `changes`, or `health`
- `query` (optional): For feature reports, the search query (e.g., "authentication")
- `--since <ref>` (optional): For changes reports, the starting reference
- `--theme <name>` (optional): D2 diagram theme (default: colorblind-clear)

If arguments are not provided, the skill will ask interactively.

---

## Workflow

### Step 1: Generate CX Data

Run the appropriate cx report command:

```bash
# Overview report
cx report overview --data --playground --theme colorblind-clear

# Feature report
cx report feature "authentication" --data --playground --theme colorblind-clear

# Changes report
cx report changes --since HEAD~50 --data --playground --theme colorblind-clear

# Health report
cx report health --data --playground --theme colorblind-clear
```

**Expected YAML Output (with playground metadata):**

```yaml
report:
  type: overview
  generated_at: 2026-01-30T20:55:00Z
  playground_mode: true

entities:
  - id: parser-walkNode
    name: walkNode
    type: function
    layer: core              # For filtering
    importance: keystone      # For filtering
    coverage: 85
    file: internal/parser/node.go
    lines: [45, 120]
    signature: "func walkNode(n *node.Node) error"

diagrams:
  architecture:
    title: "System Architecture"
    d2: |
      # D2 code ...
    svg: |
      <svg viewBox="0 0 1200 800">
        <!-- Pre-rendered SVG with CSS classes -->
        <g class="layer-core entity-function importance-keystone" id="node-parser-walkNode">
          <rect width="160" height="50" x="100" y="100" fill="#4a90d9" rx="4"/>
          <text x="180" y="130">walkNode</text>
        </g>
        <!-- ... more nodes ... -->
      </svg>
    element_map:
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
    - name: graph
      display: "Dependency Graph"
      entity_count: 234
      color: "#f59e0b"

  connection_types:
    - name: calls
      display: "Function Calls"
      color: "#3b82f6"
      style: "solid"
    - name: uses_type
      display: "Type Dependencies"
      color: "#10b981"
      style: "dashed"

  importance_levels:
    - name: keystone
      display: "Keystone"
      description: "Critical with many dependents"
    - name: bottleneck
      display: "Bottleneck"
      description: "Frequently called"
    - name: normal
      display: "Normal"
      description: "Standard connectivity"
    - name: leaf
      display: "Leaf"
      description: "Few dependents"
```

### Step 2: Generate Interactive Playground HTML

Create a self-contained HTML file with embedded JavaScript and CSS.

**HTML Structure:**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>CX Report - {{report_type}} - {{project_name}}</title>
  <style>
    /* ... CSS styles ... */
  </style>
</head>
<body>
  <!-- Controls Sidebar -->
  <div class="controls-sidebar">
    <div class="control-section">
      <h3>View Presets</h3>
      <div class="button-group">
        <button onclick="applyPreset('full')" class="preset-btn active">Full System</button>
        <button onclick="applyPreset('keystones')" class="preset-btn">Keystones Only</button>
        <button onclick="applyPreset('high-coverage')" class="preset-btn">High Coverage</button>
        <button onclick="applyPreset('critical')" class="preset-btn">Critical Path</button>
      </div>
    </div>

    <div class="control-section">
      <h3>Layers</h3>
      <div class="checkbox-group">
        <label>
          <input type="checkbox" checked onchange="toggleLayer('core')">
          <span class="layer-indicator" style="background: #4a90d9"></span>
          Core Parser (156)
        </label>
        <label>
          <input type="checkbox" checked onchange="toggleLayer('store')">
          <span class="layer-indicator" style="background: #10b981"></span>
          Entity Store (89)
        </label>
        <!-- ... more layers ... -->
      </div>
    </div>

    <div class="control-section">
      <h3>Importance</h3>
      <div class="checkbox-group">
        <label>
          <input type="checkbox" checked onchange="toggleImportance('keystone')">
          <span class="importance-keystone"></span>
          Keystone (23)
        </label>
        <label>
          <input type="checkbox" checked onchange="toggleImportance('bottleneck')">
          <span class="importance-bottleneck"></span>
          Bottleneck (47)
        </label>
        <!-- ... more ... -->
      </div>
    </div>

    <div class="control-section">
      <h3>Connections</h3>
      <div class="checkbox-group">
        <label>
          <input type="checkbox" checked onchange="toggleConnection('calls')">
          <span class="conn-line conn-solid" style="border-color: #3b82f6"></span>
          Calls (blue)
        </label>
        <label>
          <input type="checkbox" checked onchange="toggleConnection('uses_type')">
          <span class="conn-line conn-dashed" style="border-color: #10b981"></span>
          Uses Type (green)
        </label>
      </div>
    </div>

    <div class="control-section">
      <h3>Comments (<span id="comment-count">0</span>)</h3>
      <div class="comments-list" id="comments-list">
        <div class="empty-state">Click a node to add a comment</div>
      </div>
    </div>

    <div class="control-section">
      <h3>Prompt Output</h3>
      <textarea id="prompt-output" readonly class="prompt-textarea" placeholder="Comments will appear here..."></textarea>
      <button onclick="copyPrompt()" class="action-btn primary">Copy Prompt</button>
      <button onclick="clearComments()" class="action-btn secondary">Clear All</button>
    </div>
  </div>

  <!-- Main Canvas Area -->
  <div class="canvas-area">
    <div class="canvas-toolbar">
      <div class="search-box">
        <input type="text" id="search-input" placeholder="Search entities..." oninput="handleSearch()">
        <span class="search-count" id="search-count"></span>
      </div>
      <div class="zoom-controls">
        <button onclick="zoomIn()" title="Zoom In">+</button>
        <button onclick="zoomOut()" title="Zoom Out">−</button>
        <button onclick="resetZoom()" title="Reset Zoom">⟲</button>
        <button onclick="fitToScreen()" title="Fit to Screen">□</button>
      </div>
    </div>

    <div class="svg-container" id="svg-container">
      <!-- SVG injected from YAML -->
      {{diagrams.architecture.svg}}
    </div>

    <!-- Entity Details Panel (slide-up) -->
    <div class="entity-panel" id="entity-panel">
      <button onclick="closeEntityPanel()" class="close-btn">×</button>
      <div class="entity-content">
        <h2 id="entity-name">Entity Name</h2>
        <div class="entity-meta">
          <span class="entity-badge" id="entity-type">function</span>
          <span class="entity-badge" id="entity-importance">keystone</span>
          <span class="entity-badge" id="entity-coverage">85% coverage</span>
        </div>
        <div class="entity-location">
          <strong>File:</strong> <span id="entity-file">internal/parser/node.go</span>
          <strong>Lines:</strong> <span id="entity-lines">45-120</span>
        </div>
        <div class="entity-signature">
          <strong>Signature:</strong>
          <code id="entity-signature">func walkNode(n *node.Node) error</code>
        </div>
        <div class="entity-stats">
          <div class="stat">
            <span class="stat-label">PageRank</span>
            <span class="stat-value" id="entity-pagerank">0.042</span>
          </div>
          <div class="stat">
            <span class="stat-label">In-Degree</span>
            <span class="stat-value" id="entity-indegree">23</span>
          </div>
          <div class="stat">
            <span class="stat-label">Out-Degree</span>
            <span class="stat-value" id="entity-outdegree">8</span>
          </div>
        </div>
        <div class="entity-actions">
          <button onclick="addCommentForEntity()" class="action-btn primary">
            <span class="icon">💬</span> Add Comment
          </button>
          <button onclick="highlightPath()" class="action-btn secondary">
            <span class="icon">🔍</span> Trace Path
          </button>
        </div>
      </div>
    </div>
  </div>

  <script>
    // ============================================
    // STATE MANAGEMENT
    // ============================================

    const reportData = {{REPORT_DATA_JSON}};

    let state = {
      // Layer visibility
      layers: {
        core: true,
        store: true,
        graph: true,
        // ... from metadata.layers
      },

      // Importance visibility
      importance: {
        keystone: true,
        bottleneck: true,
        normal: true,
        leaf: true,
      },

      // Connection visibility
      connections: {
        calls: true,
        uses_type: true,
      },

      // Viewport state
      zoom: 1,
      pan: { x: 0, y: 0 },
      isDragging: false,
      dragStart: { x: 0, y: 0 },

      // Comments
      comments: [],
      currentEntity: null,

      // Search
      searchQuery: '',
      searchResults: [],
    };

    // ============================================
    // INITIALIZATION
    // ============================================

    window.onload = function() {
      initializeLayers();
      initializeImportance();
      initializeConnections();
      setupDragAndZoom();
      generateInitialPrompt();
    };

    function initializeLayers() {
      reportData.metadata.layers.forEach(layer => {
        state.layers[layer.name] = true;
      });
    }

    function initializeImportance() {
      reportData.metadata.importance_levels.forEach(level => {
        state.importance[level.name] = true;
      });
    }

    function initializeConnections() {
      reportData.metadata.connection_types.forEach(conn => {
        state.connections[conn.name] = true;
      });
    }

    function setupDragAndZoom() {
      const container = document.getElementById('svg-container');
      const svg = container.querySelector('svg');

      // Set transform origin to center
      svg.style.transformOrigin = 'center';

      // Pan with drag
      container.addEventListener('mousedown', (e) => {
        if (e.target === container || e.target.tagName === 'svg') {
          state.isDragging = true;
          state.dragStart = { x: e.clientX - state.pan.x, y: e.clientY - state.pan.y };
          container.style.cursor = 'grabbing';
        }
      });

      document.addEventListener('mousemove', (e) => {
        if (state.isDragging) {
          state.pan.x = e.clientX - state.dragStart.x;
          state.pan.y = e.clientY - state.dragStart.y;
          updateTransform();
        }
      });

      document.addEventListener('mouseup', () => {
        state.isDragging = false;
        container.style.cursor = 'grab';
      });

      // Zoom with scroll
      container.addEventListener('wheel', (e) => {
        e.preventDefault();
        const delta = e.deltaY > 0 ? 0.9 : 1.1;
        state.zoom = Math.min(Math.max(state.zoom * delta, 0.1), 5);
        updateTransform();
      });

      // Initial transform
      updateTransform();
    }

    function updateTransform() {
      const svg = document.querySelector('#svg-container svg');
      svg.style.transform = `translate(${state.pan.x}px, ${state.pan.y}px) scale(${state.zoom})`;
    }

    // ============================================
    // FILTERING
    // ============================================

    function toggleLayer(layer) {
      state.layers[layer] = !state.layers[layer];
      document.querySelectorAll(`.layer-${layer}`).forEach(el => {
        el.style.display = state.layers[layer] ? '' : 'none';
      });
    }

    function toggleImportance(importance) {
      state.importance[importance] = !state.importance[importance];
      document.querySelectorAll(`.importance-${importance}`).forEach(el => {
        el.style.opacity = state.importance[importance] ? '1' : '0.2';
      });
    }

    function toggleConnection(conn) {
      state.connections[conn] = !state.connections[conn];
      document.querySelectorAll(`.connection-${conn}`).forEach(el => {
        el.style.display = state.connections[conn] ? '' : 'none';
      });
    }

    // ============================================
    // PRESETS
    // ============================================

    function applyPreset(preset) {
      // Reset all filters
      Object.keys(state.layers).forEach(l => state.layers[l] = true);
      Object.keys(state.importance).forEach(i => state.importance[i] = true);
      Object.keys(state.connections).forEach(c => state.connections[c] = true);

      // Update checkboxes
      document.querySelectorAll('input[type="checkbox"]').forEach(cb => cb.checked = true);

      // Apply preset-specific settings
      switch(preset) {
        case 'full':
          // Show everything (already done)
          break;
        case 'keystones':
          state.importance.normal = false;
          state.importance.leaf = false;
          updateFilterCheckboxes();
          break;
        case 'high-coverage':
          // Hide entities with coverage < 50%
          reportData.entities.forEach(e => {
            if (e.coverage < 50) {
              const el = document.getElementById(`node-${e.id}`);
              if (el) el.style.opacity = '0.2';
            }
          });
          break;
        case 'critical':
          state.layers.core = true;
          state.layers.store = false;
          state.layers.graph = false;
          state.importance.keystone = true;
          state.importance.bottleneck = true;
          state.importance.normal = false;
          state.importance.leaf = false;
          updateFilterCheckboxes();
          break;
      }

      // Apply visibility
      applyVisibility();
    }

    function updateFilterCheckboxes() {
      // Update checkboxes to match state
      Object.keys(state.layers).forEach(l => {
        const cb = document.querySelector(`input[onchange="toggleLayer('${l}')"]`);
        if (cb) cb.checked = state.layers[l];
      });
      Object.keys(state.importance).forEach(i => {
        const cb = document.querySelector(`input[onchange="toggleImportance('${i}')"]`);
        if (cb) cb.checked = state.importance[i];
      });
    }

    function applyVisibility() {
      // Apply all visibility rules
      Object.keys(state.layers).forEach(layer => {
        document.querySelectorAll(`.layer-${layer}`).forEach(el => {
          el.style.display = state.layers[layer] ? '' : 'none';
        });
      });
      Object.keys(state.importance).forEach(importance => {
        document.querySelectorAll(`.importance-${importance}`).forEach(el => {
          el.style.opacity = state.importance[importance] ? '1' : '0.2';
        });
      });
      Object.keys(state.connections).forEach(conn => {
        document.querySelectorAll(`.connection-${conn}`).forEach(el => {
          el.style.display = state.connections[conn] ? '' : 'none';
        });
      });
    }

    // ============================================
    // ZOOM CONTROLS
    // ============================================

    function zoomIn() {
      state.zoom = Math.min(state.zoom * 1.2, 5);
      updateTransform();
    }

    function zoomOut() {
      state.zoom = Math.max(state.zoom * 0.8, 0.1);
      updateTransform();
    }

    function resetZoom() {
      state.zoom = 1;
      state.pan = { x: 0, y: 0 };
      updateTransform();
    }

    function fitToScreen() {
      // Calculate bounding box of all visible elements
      const svg = document.querySelector('#svg-container svg');
      const bbox = svg.getBoundingClientRect();
      const container = document.getElementById('svg-container');

      const scaleX = (container.clientWidth - 40) / bbox.width;
      const scaleY = (container.clientHeight - 40) / bbox.height;
      state.zoom = Math.min(scaleX, scaleY, 1);
      state.pan = { x: 20, y: 20 };
      updateTransform();
    }

    // ============================================
    // ENTITY INTERACTION
    // ============================================

    function setupEntityClickHandlers() {
      reportData.entities.forEach(entity => {
        const svgId = reportData.diagrams.architecture.element_map[entity.id];
        const el = document.getElementById(svgId);
        if (el) {
          el.style.cursor = 'pointer';
          el.addEventListener('click', () => showEntityDetails(entity));
        }
      });
    }

    function showEntityDetails(entity) {
      state.currentEntity = entity;

      // Populate panel
      document.getElementById('entity-name').textContent = entity.name;
      document.getElementById('entity-type').textContent = entity.type;
      document.getElementById('entity-importance').textContent = entity.importance;
      document.getElementById('entity-coverage').textContent = `${entity.coverage}% coverage`;
      document.getElementById('entity-file').textContent = entity.file;
      document.getElementById('entity-lines').textContent = `${entity.lines[0]}-${entity.lines[1]}`;
      document.getElementById('entity-signature').textContent = entity.signature || 'N/A';
      document.getElementById('entity-pagerank').textContent = entity.pagerank?.toFixed(4) || 'N/A';
      document.getElementById('entity-indegree').textContent = entity.in_degree || 0;
      document.getElementById('entity-outdegree').textContent = entity.out_degree || 0;

      // Show panel
      document.getElementById('entity-panel').classList.add('visible');

      // Highlight node
      highlightEntity(entity.id);
    }

    function closeEntityPanel() {
      document.getElementById('entity-panel').classList.remove('visible');
      clearHighlights();
      state.currentEntity = null;
    }

    function highlightEntity(entityId) {
      // Remove existing highlights
      document.querySelectorAll('.highlighted').forEach(el => {
        el.classList.remove('highlighted');
      });

      // Add highlight to this entity
      const svgId = reportData.diagrams.architecture.element_map[entityId];
      const el = document.getElementById(svgId);
      if (el) {
        el.classList.add('highlighted');
      }
    }

    function clearHighlights() {
      document.querySelectorAll('.highlighted').forEach(el => {
        el.classList.remove('highlighted');
      });
    }

    // ============================================
    // SEARCH
    // ============================================

    function handleSearch() {
      const query = document.getElementById('search-input').value.toLowerCase();
      state.searchQuery = query;
      state.searchResults = [];

      if (query.length === 0) {
        document.getElementById('search-count').textContent = '';
        clearSearchHighlights();
        return;
      }

      // Find matching entities
      reportData.entities.forEach(entity => {
        if (entity.name.toLowerCase().includes(query) ||
            entity.file.toLowerCase().includes(query)) {
          state.searchResults.push(entity);
        }
      });

      document.getElementById('search-count').textContent = `(${state.searchResults.length})`;
      highlightSearchResults();
    }

    function highlightSearchResults() {
      clearSearchHighlights();
      state.searchResults.forEach(entity => {
        const svgId = reportData.diagrams.architecture.element_map[entity.id];
        const el = document.getElementById(svgId);
        if (el) {
          el.classList.add('search-match');
        }
      });
    }

    function clearSearchHighlights() {
      document.querySelectorAll('.search-match').forEach(el => {
        el.classList.remove('search-match');
      });
    }

    // ============================================
    // COMMENTS
    // ============================================

    function addCommentForEntity() {
      if (!state.currentEntity) return;

      const text = prompt(`Add comment for ${state.currentEntity.name}:`);
      if (!text) return;

      const comment = {
        id: Date.now(),
        entityId: state.currentEntity.id,
        entityName: state.currentEntity.name,
        entityFile: state.currentEntity.file,
        text: text,
        timestamp: new Date().toISOString(),
      };

      state.comments.push(comment);
      updateCommentsList();
      generatePrompt();
      addCommentIndicator(state.currentEntity.id);
    }

    function deleteComment(commentId) {
      state.comments = state.comments.filter(c => c.id !== commentId);
      updateCommentsList();
      generatePrompt();
    }

    function updateCommentsList() {
      const list = document.getElementById('comments-list');
      document.getElementById('comment-count').textContent = state.comments.length;

      if (state.comments.length === 0) {
        list.innerHTML = '<div class="empty-state">Click a node to add a comment</div>';
        return;
      }

      list.innerHTML = state.comments.map(comment => `
        <div class="comment-item">
          <div class="comment-header">
            <strong>${comment.entityName}</strong>
            <button onclick="deleteComment(${comment.id})" class="delete-btn">×</button>
          </div>
          <div class="comment-text">${comment.text}</div>
          <div class="comment-file">${comment.entityFile}</div>
        </div>
      `).join('');
    }

    function clearComments() {
      if (confirm('Clear all comments?')) {
        state.comments = [];
        updateCommentsList();
        generatePrompt();
        clearCommentIndicators();
      }
    }

    function addCommentIndicator(entityId) {
      const svgId = reportData.diagrams.architecture.element_map[entityId];
      const el = document.getElementById(svgId);
      if (el) {
        el.classList.add('has-comment');
      }
    }

    function clearCommentIndicators() {
      document.querySelectorAll('.has-comment').forEach(el => {
        el.classList.remove('has-comment');
      });
    }

    // ============================================
    // PROMPT OUTPUT
    // ============================================

    function generateInitialPrompt() {
      const prompt = `This is the CX ${reportData.report.type} report for this codebase.

Generated at: ${reportData.report.generated_at}

Visible layers: ${Object.entries(state.layers).filter(([k,v]) => v).map(([k]) => k).join(', ')}

Instructions:
- Click entities to view details
- Use filters to focus on specific areas
- Add comments to request changes or ask questions
- Copy this prompt to paste back into Claude Code
`;
      document.getElementById('prompt-output').value = prompt;
    }

    function generatePrompt() {
      const visibleLayers = Object.entries(state.layers).filter(([k,v]) => v).map(([k]) => k).join(', ');
      const visibleImportance = Object.entries(state.importance).filter(([k,v]) => v).map(([k]) => k).join(', ');

      let prompt = `This is the CX ${reportData.report.type} report for this codebase.

Generated at: ${reportData.report.generated_at}

## Current View

Visible layers: ${visibleLayers || 'none'}
Visible importance levels: ${visibleImportance || 'none'}
Applied filters: ${state.searchQuery ? `Search: "${state.searchQuery}"` : 'none'}

## Comments on Specific Entities
`;

      if (state.comments.length === 0) {
        prompt += '\n(No comments added yet. Click entities to add comments.)\n';
      } else {
        prompt += '\n';
        state.comments.forEach(comment => {
          prompt += `**${comment.entityName}** (${comment.entityFile}):\n${comment.text}\n\n`;
        });
      }

      prompt += `\n## Entity Statistics

Total entities: ${reportData.entities.length}
Comments: ${state.comments.length}
`;

      document.getElementById('prompt-output').value = prompt;
    }

    function copyPrompt() {
      const textarea = document.getElementById('prompt-output');
      textarea.select();
      document.execCommand('copy');
      alert('Prompt copied to clipboard!');
    }

    // ============================================
    // INITIALIZATION COMPLETE
    // ============================================

    // Setup click handlers after SVG is loaded
    setTimeout(setupEntityClickHandlers, 100);
  </script>
</body>
</html>
```

**CSS Styles:**

```css
:root {
  --primary: #3b82f6;
  --primary-hover: #2563eb;
  --secondary: #6b7280;
  --bg-light: #f8f9fa;
  --bg-white: #ffffff;
  --border: #e5e7eb;
  --text-primary: #111827;
  --text-secondary: #6b7280;
  --success: #10b981;
  --warning: #f59e0b;
  --danger: #ef4444;
}

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 14px;
  color: var(--text-primary);
  background: var(--bg-light);
  overflow: hidden;
}

/* Controls Sidebar */
.controls-sidebar {
  position: fixed;
  left: 0;
  top: 0;
  width: 300px;
  height: 100vh;
  background: var(--bg-white);
  border-right: 1px solid var(--border);
  overflow-y: auto;
  padding: 1rem;
  z-index: 100;
}

.control-section {
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--border);
}

.control-section:last-child {
  border-bottom: none;
}

.control-section h3 {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-secondary);
  margin-bottom: 0.75rem;
  font-weight: 600;
}

/* Buttons */
.button-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.preset-btn {
  padding: 0.5rem 0.75rem;
  background: var(--bg-light);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
  text-align: left;
}

.preset-btn:hover {
  background: var(--border);
}

.preset-btn.active {
  background: var(--primary);
  color: white;
  border-color: var(--primary);
}

.action-btn {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.action-btn.primary {
  background: var(--primary);
  color: white;
  border-color: var(--primary);
}

.action-btn.primary:hover {
  background: var(--primary-hover);
}

.action-btn.secondary {
  background: var(--bg-light);
}

.action-btn.secondary:hover {
  background: var(--border);
}

/* Checkboxes */
.checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.checkbox-group label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  font-size: 13px;
}

.checkbox-group input[type="checkbox"] {
  margin: 0;
}

/* Indicators */
.layer-indicator {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  display: inline-block;
}

.importance-keystone::before {
  content: '★';
  color: var(--warning);
}

.importance-bottleneck::before {
  content: '⚡';
  color: var(--primary);
}

.importance-normal::before {
  content: '○';
  color: var(--text-secondary);
}

.importance-leaf::before {
  content: '·';
  color: var(--text-secondary);
}

.conn-line {
  width: 24px;
  height: 2px;
  display: inline-block;
}

.conn-solid {
  border-bottom: 2px solid;
}

.conn-dashed {
  border-bottom: 2px dashed;
}

/* Comments */
.comments-list {
  max-height: 200px;
  overflow-y: auto;
}

.empty-state {
  color: var(--text-secondary);
  font-style: italic;
  font-size: 12px;
}

.comment-item {
  padding: 0.5rem;
  background: var(--bg-light);
  border-radius: 6px;
  margin-bottom: 0.5rem;
}

.comment-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.25rem;
}

.comment-header strong {
  font-size: 12px;
}

.delete-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.delete-btn:hover {
  color: var(--danger);
}

.comment-text {
  font-size: 13px;
  margin-bottom: 0.25rem;
}

.comment-file {
  font-size: 11px;
  color: var(--text-secondary);
}

/* Prompt Output */
.prompt-textarea {
  width: 100%;
  height: 150px;
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 11px;
  resize: vertical;
  background: var(--bg-light);
}

/* Canvas Area */
.canvas-area {
  margin-left: 300px;
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.canvas-toolbar {
  position: fixed;
  top: 0;
  right: 0;
  left: 300px;
  height: 60px;
  background: var(--bg-white);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 1rem;
  z-index: 50;
}

.search-box {
  position: relative;
  flex: 1;
  max-width: 400px;
}

.search-box input {
  width: 100%;
  padding: 0.5rem 2rem 0.5rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 13px;
}

.search-count {
  position: absolute;
  right: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-secondary);
  font-size: 12px;
}

.zoom-controls {
  display: flex;
  gap: 0.25rem;
}

.zoom-controls button {
  width: 32px;
  height: 32px;
  border: 1px solid var(--border);
  background: var(--bg-light);
  border-radius: 4px;
  cursor: pointer;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.zoom-controls button:hover {
  background: var(--border);
}

/* SVG Container */
.svg-container {
  flex: 1;
  overflow: hidden;
  background: var(--bg-light);
  cursor: grab;
  padding-top: 60px;
}

.svg-container:active {
  cursor: grabbing;
}

.svg-container svg {
  transition: transform 0.1s ease-out;
}

/* Entity Panel */
.entity-panel {
  position: fixed;
  bottom: -400px;
  left: 300px;
  right: 0;
  height: 400px;
  background: var(--bg-white);
  border-top: 1px solid var(--border);
  box-shadow: 0 -4px 12px rgba(0,0,0,0.1);
  transition: bottom 0.3s ease-out;
  z-index: 200;
  padding: 1.5rem;
  overflow-y: auto;
}

.entity-panel.visible {
  bottom: 0;
}

.close-btn {
  position: absolute;
  top: 1rem;
  right: 1rem;
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-secondary);
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  color: var(--text-primary);
}

.entity-content h2 {
  font-size: 24px;
  margin-bottom: 1rem;
}

.entity-meta {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.entity-badge {
  padding: 0.25rem 0.5rem;
  background: var(--bg-light);
  border-radius: 4px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.entity-location {
  margin-bottom: 1rem;
  font-size: 13px;
  color: var(--text-secondary);
}

.entity-location strong {
  color: var(--text-primary);
}

.entity-signature {
  margin-bottom: 1rem;
}

.entity-signature code {
  display: block;
  padding: 0.75rem;
  background: var(--bg-light);
  border-radius: 6px;
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 12px;
  overflow-x: auto;
}

.entity-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.stat {
  text-align: center;
  padding: 1rem;
  background: var(--bg-light);
  border-radius: 6px;
}

.stat-label {
  display: block;
  font-size: 11px;
  color: var(--text-secondary);
  text-transform: uppercase;
  margin-bottom: 0.25rem;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
}

.entity-actions {
  display: flex;
  gap: 1rem;
}

/* SVG Highlighting */
.highlighted {
  filter: drop-shadow(0 0 8px var(--primary));
  stroke: var(--primary);
  stroke-width: 3;
}

.search-match {
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.has-comment {
  position: relative;
}

.has-comment::after {
  content: '💬';
  position: absolute;
  top: -8px;
  right: -8px;
  font-size: 16px;
  z-index: 10;
}
```

### Step 3: Save Report

**Naming Convention:**
```
reports/<type>_playground_<YYYY-MM-DD>[_<query>].html

Examples:
  reports/overview_playground_2026-01-30.html
  reports/feature_playground_2026-01-30_authentication.html
  reports/changes_playground_2026-01-30_HEAD~50.html
  reports/health_playground_2026-01-30.html
```

---

## Interactive Features

### Click-to-Comment
1. Click any entity node in the diagram
2. Entity details panel slides up from bottom
3. Click "Add Comment" to add feedback
4. Comment appears in sidebar list
5. Entity gets comment indicator (💬)
6. Prompt output updates automatically

### Presets
- **Full System**: Show all layers and entities
- **Keystones Only**: Hide normal and leaf entities
- **High Coverage**: Highlight entities with >50% coverage
- **Critical Path**: Show only core layer + keystones + bottlenecks

### Search
- Real-time search by entity name or file path
- Matching entities pulse with animation
- Search count displayed

### Zoom/Pan
- Scroll to zoom in/out
- Drag canvas to pan
- Buttons for precise zoom control
- "Fit to Screen" to view entire diagram

### Prompt Output
- Automatically generated based on current view
- Shows visible layers, filters, and comments
- Click "Copy Prompt" to paste into Claude Code
- Updates when comments are added/removed

---

## Example Session

```
User: /cortex-report overview

Claude: Generating interactive CX overview report...

[Runs: cx report overview --data --playground]

Found 479 entities across 3 layers.
Rendering architecture diagram with D2...

✓ Generated playground HTML
✓ Embedded SVG with CSS classes
✓ Initialized filters and controls

---

# CX Interactive Report - Architecture Overview

[Interactive HTML opens in browser]

Features:
- Click nodes to view details
- Toggle layers (Core, Store, Graph)
- Filter by importance (Keystone, Bottleneck, Normal, Leaf)
- Search entities in real-time
- Add comments to request changes
- Copy prompt with comments to paste into Claude Code

Report saved to: reports/overview_playground_2026-01-30.html

---

## Sample Comments Generated After User Interaction:

This is the CX overview report for this codebase.

Generated at: 2026-01-30T20:55:00Z

## Current View

Visible layers: core, store, graph
Visible importance levels: keystone, bottleneck
Applied filters: none

## Comments on Specific Entities

**walkNode** (internal/parser/node.go):
This function is too long (75 lines). Consider extracting sub-functions for better readability.

**GetEntity** (internal/store/entity.go):
The caching layer seems to have a race condition. Review the lock handling.

**BuildGraph** (internal/graph/builder.go):
Can we add an option to filter out test files during graph construction?
```

---

## Important Rules

1. **YAML parsing**: Use the `playground_mode: true` flag to detect playground mode
2. **SVG injection**: Embed the pre-rendered SVG from `diagrams.*.svg`
3. **CSS classes**: SVG elements must have class attributes (`.layer-core`, `.importance-keystone`)
4. **Element map**: Use `element_map` to link entity IDs to SVG element IDs
5. **Self-contained**: All CSS and JavaScript must be embedded in the HTML file
6. **Error handling**: If SVG is missing, show D2 code in `<pre>` block
7. **Backward compatibility**: The skill should work with `cx report --data` (without `--playground`) by falling back to static HTML generation

---

## Troubleshooting

### "No SVG in YAML output"
The `--playground` flag may not be implemented yet. Use the skill to generate static HTML instead, or wait for `--playground` implementation.

### "Click-to-comment not working"
Check that `element_map` is populated in the YAML output. Ensure SVG element IDs match the map.

### "Filters not hiding elements"
Verify that SVG elements have CSS class attributes (e.g., `class="layer-core entity-function importance-keystone"`).

### "Search not finding entities"
Check that entity names and file paths are in the YAML `entities` array.

---

## Next Steps

After the Overview playground is complete, we'll implement:

1. **Feature Playground** - Call flow diagrams with step-through
2. **Changes Playground** - Before/after diff visualization
3. **Health Playground** - Risk heatmap with coverage filtering

Each playground will reuse the same CSS/JS framework, adapting the controls and canvas for the specific report type.

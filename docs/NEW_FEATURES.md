# CX New Features - January 2026 Update

This document covers the new features added to CX in the January 2026 enhancement release.

## Overview

Four major feature epics were implemented:

1. **Daemon & Live Mode** (cortex-b44) - Background process for faster queries
2. **Smart Context Enhancements** (cortex-dsr) - Better context assembly with drift detection
3. **Entity Tagging & Bookmarks** (cortex-ali) - Tag entities for organization
4. **Test Intelligence** (cortex-9dk) - Smart test selection and coverage analysis

---

## 1. Daemon & Live Mode (cortex-b44)

### New Commands

```bash
# Daemon control
cx status                    # Check if daemon is running, show uptime/idle
cx daemon stop               # Stop the daemon
cx daemon status             # Alias for cx status

# Live mode (renamed from cx serve)
cx live                      # Start foreground daemon
cx live --watch              # Start with filesystem watching
```

### Auto-Start Integration

Commands now automatically use the daemon when available:
- First query may start the daemon (configurable)
- Fallback to direct DB access if daemon unavailable
- Transparent to command usage

### Incremental Scanning

```bash
# Daemon tracks file changes efficiently
cx scan                      # Now only scans changed files when daemon running
cx scan --force              # Force full rescan
```

### Configuration

```yaml
# .cx/config.yaml
daemon:
  auto_start: true           # Start daemon on first query
  idle_timeout: 30m          # Auto-shutdown after idle period
  watch_paths:               # Paths to watch for changes
    - "."
  exclude_paths:             # Paths to ignore
    - "node_modules"
    - "vendor"
```

---

## 2. Smart Context Enhancements (cortex-dsr)

### Inline Drift Detection

```bash
cx safe <file> --inline      # Quick drift check without full analysis
cx safe --drift --inline     # Show drift for all files
```

Output shows:
- Added entities (new since last scan)
- Removed entities (deleted since last scan)
- Modified entities (signature/hash changed)
- Broken dependencies (calls to removed entities)

### Call Chain Tracer

```bash
# Trace call paths between entities
cx trace <from> <to>         # Show shortest call path
cx trace <entity> --callers  # Trace upstream (what calls this)
cx trace <entity> --callees  # Trace downstream (what this calls)
cx trace A B --all-paths     # Show all paths, not just shortest
cx trace A B --max-depth 5   # Limit search depth
```

Example output:
```yaml
path:
  - name: HandleRequest
    location: internal/api/handler.go:45
    type: function
  - name: ValidateInput
    location: internal/api/validation.go:23
    type: function
  - name: CheckPermissions
    location: internal/auth/permissions.go:89
    type: function
length: 3
```

---

## 3. Entity Tagging & Bookmarks (cortex-ali)

### Tagging Commands (already available)

```bash
# Add tags
cx tag <entity> important critical       # Add multiple tags
cx tag <entity> wip -n "work in progress"  # Tag with note
cx untag <entity> wip                    # Remove tag

# List tags
cx tags <entity>             # Tags for specific entity
cx tags                      # All tags with counts
cx tags --find important     # Find entities by tag
```

### NEW: Tag-Based Filtering in Find

```bash
# Filter by tags (AND semantics)
cx find --tag important              # Entities with 'important' tag
cx find --tag critical --tag api     # Entities with BOTH tags

# OR semantics
cx find --tag-any critical important # Entities with ANY of these tags
```

### NEW: Tag Integration in Commands

```bash
# cx show now displays tags
cx show UserService
# Output includes:
#   tags: [critical, api, needs-refactor]

# cx safe warns about tagged entities
cx safe internal/auth/
# Output includes warnings like:
#   ⚠️ Modifying entity tagged 'critical': ValidateToken
#   ⚠️ Modifying entity tagged 'deprecated': OldAuthFlow
```

### NEW: Tag Export/Import for Git Sync

```bash
# Export tags for version control
cx tags export                       # Writes to .cx/tags.yaml
cx tags export custom.yaml           # Custom file

# Import tags
cx tags import .cx/tags.yaml         # Import from file
cx tags import tags.yaml --overwrite # Overwrite existing
cx tags import tags.yaml --skip-existing  # Skip conflicts
```

Tags YAML format:
```yaml
tags:
  - entity_id: sa-fn-abc123-ValidateToken
    entity_name: ValidateToken
    tag: critical
    note: "Core authentication"
  - entity_id: sa-fn-def456-OldAuth
    entity_name: OldAuth
    tag: deprecated
    note: "Remove after v2.0"
```

---

## 4. Test Intelligence (cortex-9dk)

### Test Discovery (already available)

```bash
cx test discover             # Scan for test functions
cx test list                 # List discovered tests
cx test list --for-entity <id>  # Tests covering entity
```

### NEW: Test Suggestions

```bash
cx test suggest              # Show prioritized test suggestions
cx test suggest --top 10     # Top 10 suggestions
cx test suggest --for <entity>  # Suggestions for specific entity
```

Output prioritizes:
1. Untested keystones (highest priority)
2. Low-coverage keystones
3. Entities with many callers but no tests
4. Recently modified code without tests

Includes signature-based suggestions:
```yaml
suggestions:
  - entity: ValidateToken
    location: internal/auth/jwt.go:45
    priority: critical
    coverage: 0%
    reason: "Keystone with 0% coverage"
    test_ideas:
      - "Test with valid token"
      - "Test with expired token"
      - "Test with nil input"
```

### NEW: Smart Test Selection

```bash
# Find tests for changes
cx test --diff               # Tests for uncommitted changes
cx test --affected <entity>  # Tests covering specific entity
cx test --commit HEAD~3      # Tests for last 3 commits

# Run tests
cx test --diff --run         # Find and execute tests
cx test --output-command     # Just show go test command
```

### NEW: Coverage Gaps Report

```bash
cx test --gaps               # Show all coverage gaps
cx test --gaps --keystones-only  # Only keystone gaps
cx test --gaps --threshold 80    # Below 80% coverage
cx test --gaps --by-priority     # Group by priority tier
```

Priority tiers:
- **critical**: Keystones with 0% coverage
- **high**: Keystones below threshold, entities with many callers
- **medium**: Below threshold, moderate usage
- **low**: Below threshold, leaf nodes

### NEW: Test Impact Analysis

```bash
cx test impact <file>        # Coverage for all entities in file
cx test impact <entity>      # Coverage for specific entity
cx test impact --uncovered   # List all uncovered entities
```

Output:
```yaml
target: internal/auth/jwt.go
entities:
  - name: ValidateToken
    coverage: 85%
    tests:
      - TestValidateToken_Success
      - TestValidateToken_Expired
    recommendation: "Good coverage"
  - name: ParseClaims
    coverage: 0%
    tests: []
    recommendation: "High priority - keystone with 0% coverage"
summary:
  total_entities: 5
  covered: 3
  uncovered: 2
  avg_coverage: 65%
```

---

## CLAUDE.md Updates

Add these new sections to CLAUDE.md:

### Entity Tagging
```bash
# Tag important code for quick access
cx tag <entity> important critical
cx find --tag important              # Find tagged entities
cx tags export                       # Export for git sync
```

### Test Intelligence
```bash
# Smart test selection
cx test suggest                      # What needs testing?
cx test --diff --run                 # Run tests for changes
cx test impact <file>                # Coverage analysis
cx test --gaps --keystones-only      # Critical coverage gaps
```

### Call Chain Analysis
```bash
cx trace <from> <to>                 # Find call path
cx trace <entity> --callers          # Upstream trace
```

### Daemon Status
```bash
cx status                            # Daemon health check
cx live --watch                      # Start live mode with file watching
```

---

## help-agents Updates

Add to the agent reference:

### cx tag
**Tag entities for organization and filtering.**

```bash
cx tag <entity> <tags...>    # Add tags
cx untag <entity> <tag>      # Remove tag
cx tags --find <tag>         # Find by tag
cx find --tag <tag>          # Filter find results
```

### cx trace
**Trace call paths between entities.**

```bash
cx trace <from> <to>         # Shortest path
cx trace <entity> --callers  # What calls this?
cx trace <entity> --callees  # What does this call?
```

### cx test (enhanced)
**Smart test selection and coverage.**

```bash
cx test suggest              # What needs tests?
cx test --diff --run         # Test changes
cx test impact <file>        # Coverage for file
cx test --gaps               # Coverage gaps
```

### cx status
**Daemon health and status.**

```bash
cx status                    # Is daemon running?
```

---

## Migration Notes

1. **cx serve renamed to cx live** - Update any scripts using `cx serve`
2. **cx rank deprecated** - Use `cx find --important` or `cx find --keystones`
3. **cx prime deprecated** - Use `cx context` instead
4. **Tag export location** - Default is `.cx/tags.yaml`, add to .gitignore if not sharing

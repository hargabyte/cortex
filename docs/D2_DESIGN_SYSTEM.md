# CX D2 Visual Design System

> Professional, showcase-worthy diagram styling for CX report generation

## Overview

The CX D2 Visual Design System provides a cohesive, professional visual language for all code intelligence diagrams. Diagrams generated with this system are suitable for:

- Technical documentation
- Marketing materials and demos
- Stakeholder presentations
- Technical blog posts

## Quick Reference

### Theme Configuration

```d2
vars: {
  d2-config: {
    theme-id: 200        # Mixed Berry Blue (default)
    layout-engine: elk   # elk, dagre, or tala
  }
}
```

### Available Themes

| Theme | ID | Use Case |
|-------|-----|----------|
| Mixed Berry Blue | 200 | Default, professional documentation |
| Dark Mauve | 201 | Dark mode presentations |
| Neutral Default | 0 | Minimal, print-friendly |

## Color Palette

### Entity Type Colors

| Entity Type | Fill | Stroke | Shape |
|-------------|------|--------|-------|
| Function | `#e3f2fd` | `#1976d2` | rectangle |
| Method | `#e3f2fd` | `#1976d2` | rectangle |
| Type/Struct | `#f3e5f5` | `#7b1fa2` | rectangle |
| Interface | `#fff3e0` | `#f57c00` | rectangle (dashed) |
| Constant | `#e8f5e9` | `#388e3c` | rectangle |
| Database | `#eceff1` | `#455a64` | cylinder |
| HTTP Handler | `#e0f7fa` | `#0097a7` | rectangle |
| Test | `#e8f5e9` | `#388e3c` | rectangle (dashed) |

### Importance Colors

| Level | Fill | Stroke | Style |
|-------|------|--------|-------|
| Keystone | `#fff3e0` | `#e65100` | 3px stroke, shadow |
| Bottleneck | `#fff8e1` | `#ff8f00` | 2px stroke |
| High Fan-In | `#e3f2fd` | `#1565c0` | 2px stroke |
| High Fan-Out | `#fce4ec` | `#c2185b` | 2px stroke |
| Normal | `#ffffff` | `#757575` | 1px stroke |
| Leaf | `#fafafa` | `#bdbdbd` | 1px stroke, 0.9 opacity |

### Coverage Colors

| Level | Threshold | Fill | Stroke |
|-------|-----------|------|--------|
| High | ≥80% | `#c8e6c9` | `#4caf50` |
| Medium | 50-79% | `#fff9c4` | `#fbc02d` |
| Low | 1-49% | `#ffcdd2` | `#f44336` |
| None | 0% | `#f5f5f5` | `#9e9e9e` |

### Risk Colors

| Level | Fill | Stroke |
|-------|------|--------|
| Critical | `#ffebee` | `#c62828` |
| Warning | `#fff8e1` | `#f57f17` |
| Info | `#e3f2fd` | `#1976d2` |
| OK | `#e8f5e9` | `#388e3c` |

### Layer Colors (Containers)

| Layer | Fill | Stroke |
|-------|------|--------|
| API | `#e0f7fa` | `#00838f` |
| Service | `#e3f2fd` | `#1565c0` |
| Data | `#eceff1` | `#455a64` |
| Domain | `#f3e5f5` | `#6a1b9a` |

## Icons

Icons are sourced from [icons.terrastruct.com](https://icons.terrastruct.com).

### Entity Type Icons

| Entity Type | Icon URL |
|-------------|----------|
| Function | `https://icons.terrastruct.com/essentials%2F142-lightning.svg` |
| Method | `https://icons.terrastruct.com/essentials%2F009-gear.svg` |
| Type/Struct | `https://icons.terrastruct.com/essentials%2F108-box.svg` |
| Interface | `https://icons.terrastruct.com/essentials%2F092-plug.svg` |
| Constant | `https://icons.terrastruct.com/essentials%2F078-pin.svg` |
| Database | `https://icons.terrastruct.com/essentials%2F119-database.svg` |
| HTTP | `https://icons.terrastruct.com/essentials%2F140-earth.svg` |
| Test | `https://icons.terrastruct.com/essentials%2F134-checkmark.svg` |
| Package | `https://icons.terrastruct.com/essentials%2F106-archive.svg` |

### Language Icons

| Language | Icon URL |
|----------|----------|
| Go | `https://icons.terrastruct.com/dev%2Fgo.svg` |
| TypeScript | `https://icons.terrastruct.com/dev%2Ftypescript.svg` |
| JavaScript | `https://icons.terrastruct.com/dev%2Fjavascript.svg` |
| Python | `https://icons.terrastruct.com/dev%2Fpython.svg` |
| Java | `https://icons.terrastruct.com/dev%2Fjava.svg` |
| Rust | `https://icons.terrastruct.com/dev%2Frustlang.svg` |

### Status Icons

| Status | Icon URL |
|--------|----------|
| Warning | `https://icons.terrastruct.com/essentials%2F149-warning-2.svg` |
| Error | `https://icons.terrastruct.com/essentials%2F150-error-1.svg` |
| Info | `https://icons.terrastruct.com/essentials%2F152-info-1.svg` |
| Success | `https://icons.terrastruct.com/essentials%2F134-checkmark.svg` |

## Edge Styles

| Dependency Type | Arrow | Stroke Color | Dash | Animated |
|-----------------|-------|--------------|------|----------|
| calls | `->` | `#424242` | solid | no |
| uses_type | `->` | `#757575` | 3 | no |
| implements | `->` | `#f57c00` | 5 | no |
| extends | `->` | `#7b1fa2` | solid | no |
| data_flow | `->` | `#1976d2` | solid | yes |
| imports | `->` | `#9e9e9e` | 2 | no |
| tests | `->` | `#4caf50` | 4 | no |

## Example Diagrams

### Architecture Diagram

```d2
direction: right

vars: {
  d2-config: {
    theme-id: 200
    layout-engine: elk
  }
}

cmd: {
  label: "CLI Layer"
  icon: https://icons.terrastruct.com/essentials%2F087-display.svg
  style: {
    fill: "#e0f7fa"
    stroke: "#00838f"
    border-radius: 8
  }

  report: {
    label: "report"
    style: {
      fill: "#fff3e0"
      stroke: "#e65100"
      stroke-width: 3
      shadow: true
    }
  }
  show: {
    label: "show"
    style: {
      fill: "#e3f2fd"
      stroke: "#1976d2"
    }
  }
}

store: {
  label: "Data Store"
  icon: https://icons.terrastruct.com/essentials%2F119-database.svg
  style: {
    fill: "#eceff1"
    stroke: "#455a64"
    border-radius: 8
  }

  Entity: {
    label: "Entity"
    shape: cylinder
    style: {
      fill: "#f3e5f5"
      stroke: "#7b1fa2"
    }
  }
}

cmd.report -> store.Entity: queries {
  style: {
    stroke: "#424242"
  }
}
cmd.show -> store.Entity {
  style: {
    stroke: "#424242"
  }
}
```

### Call Flow Diagram

```d2
direction: down

request: {
  label: "HTTP Request"
  shape: oval
  style: {
    fill: "#e0e0e0"
    stroke: "#616161"
  }
}

handler: {
  label: "LoginHandler"
  icon: https://icons.terrastruct.com/dev%2Fgo.svg
  style: {
    fill: "#e0f7fa"
    stroke: "#0097a7"
  }
}

auth: {
  label: "LoginUser"
  icon: https://icons.terrastruct.com/essentials%2F091-lock.svg
  style: {
    fill: "#fff3e0"
    stroke: "#e65100"
    stroke-width: 3
    shadow: true
  }
}

store: {
  label: "UserStore"
  shape: cylinder
  icon: https://icons.terrastruct.com/essentials%2F119-database.svg
  style: {
    fill: "#eceff1"
    stroke: "#455a64"
  }
}

response: {
  label: "JWT Token"
  shape: oval
  style: {
    fill: "#c8e6c9"
    stroke: "#4caf50"
  }
}

request -> handler: "POST /login"
handler -> auth: "authenticate()"
auth -> store: "GetByEmail()"
store -> auth: "User" {
  style.stroke-dash: 3
}
auth -> handler: "token" {
  style.stroke-dash: 3
}
handler -> response
```

### Coverage Heatmap

```d2
direction: right

well-tested: {
  label: "Store.GetEntity\n92% coverage"
  icon: https://icons.terrastruct.com/essentials%2F134-checkmark.svg
  style: {
    fill: "#c8e6c9"
    stroke: "#4caf50"
  }
}

needs-work: {
  label: "Parser.Parse\n65% coverage"
  style: {
    fill: "#fff9c4"
    stroke: "#fbc02d"
  }
}

at-risk: {
  label: "Scanner.Scan\n35% coverage"
  icon: https://icons.terrastruct.com/essentials%2F149-warning-2.svg
  style: {
    fill: "#ffcdd2"
    stroke: "#f44336"
    stroke-width: 3
  }
}

untested: {
  label: "Helper.Format\n0% coverage"
  style: {
    fill: "#f5f5f5"
    stroke: "#9e9e9e"
    stroke-dash: 3
  }
}

at-risk -> well-tested
needs-work -> at-risk
untested -> needs-work
```

## Go API Reference

### Get Node Style

```go
import "github.com/anthropics/cx/internal/graph"

// Build complete style for an entity
style := graph.GetD2NodeStyle(
    "function",    // entityType
    "keystone",    // importance
    85.0,          // coverage percentage (-1 to skip)
    "go",          // language (for icon)
)

// Convert to D2 style block
d2Style := graph.D2StyleToString(style)
```

### Get Edge Style

```go
edgeStyle := graph.GetD2EdgeStyle("calls")
d2EdgeStyle := graph.D2EdgeStyleToString(edgeStyle)
```

### Get Colors

```go
// Entity type colors
colors := graph.D2EntityColors["function"]

// Importance colors
colors := graph.D2ImportanceColors["keystone"]

// Coverage colors
colors := graph.GetCoverageColor(85.0)

// Layer colors
colors := graph.GetD2LayerColor("api")
```

### Get Icons

```go
// Entity icon
icon := graph.GetD2Icon("function")

// Language icon
icon := graph.GetD2LanguageIcon("go")

// Status icon
icon := graph.GetD2StatusIcon("warning")
```

## Files

| File | Purpose |
|------|---------|
| [internal/graph/d2_design_system.d2](../internal/graph/d2_design_system.d2) | D2 reference implementation |
| [internal/graph/d2_styles.go](../internal/graph/d2_styles.go) | Go API for design system |
| [internal/graph/d2_styles_test.go](../internal/graph/d2_styles_test.go) | Tests |
| [internal/graph/styles.go](../internal/graph/styles.go) | Base shape/edge mappings |

## Design Principles

1. **Consistency**: Same entity type always gets the same color/shape
2. **Hierarchy**: Importance is indicated through stroke width and shadow
3. **Clarity**: Colors provide semantic meaning (green=good, red=bad, etc.)
4. **Professional**: Material Design-inspired palette, clean aesthetics
5. **Accessibility**: Sufficient contrast between fill and stroke colors

## Extending the System

To add new entity types, importance levels, or colors:

1. Add entries to the appropriate map in `d2_styles.go`
2. Add corresponding styling in `d2_design_system.d2`
3. Add tests in `d2_styles_test.go`
4. Update this documentation

## References

- [D2 Language Documentation](https://d2lang.com/tour/intro/)
- [D2 Icons Library](https://icons.terrastruct.com/)
- [D2 Themes](https://d2lang.com/tour/themes)
- [Material Design Color System](https://material.io/design/color/)

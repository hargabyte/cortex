# Report Generation

Generate publication-quality codebase reports with visual D2 diagrams. Reports output structured YAML/JSON data that AI agents use to create stakeholder-ready documentation.

## Basic Usage

```bash
# System overview with architecture diagram
cx report overview --data --theme earth-tones

# Feature deep-dive with call flow diagram
cx report feature "authentication" --data

# What changed between releases
cx report changes --since v1.0 --until v2.0 --data

# Risk analysis and health report
cx report health --data
```

## Report Types

| Type | Purpose | Includes |
|------|---------|----------|
| `overview` | System architecture | Module structure, keystones, architecture diagram |
| `feature` | Feature deep-dive | Matched entities, call flow diagram, coverage |
| `changes` | What changed | Added/modified/deleted entities, impact analysis |
| `health` | Risk analysis | Coverage gaps, complexity hotspots, risk score |

## D2 Diagram Themes

Every report includes D2 diagrams. Choose from 12 professionally designed themes:

```bash
cx report overview --data --theme default          # Colorblind Clear (recommended)
cx report overview --data --theme earth-tones      # Natural browns and greens
cx report overview --data --theme dark             # Dark Mauve for dark mode
cx report overview --data --theme terminal         # Green-on-black retro
```

## Interactive Reports with Claude Code

Generate an interactive skill that asks probing questions to create tailored reports:

```bash
cx report --init-skill > ~/.claude/commands/report.md
```

Then use `/report` in Claude Code to interactively select report type, audience, theme, and output format.

## Rendering D2 Diagrams

Reports include D2 code for diagrams. Render them to SVG:

```bash
cx render diagram.d2 -o output.svg                 # File to file
echo '<d2 code>' | cx render - -o output.svg       # Pipe input
```

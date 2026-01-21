# Configuration

## Config File

Optional `.cx/config.yaml`:

```yaml
# Exclude patterns
exclude:
  - "vendor/*"
  - "node_modules/*"
  - "*_test.go"

# Pre-commit guard settings
guard:
  fail_on_coverage_regression: true
  min_coverage_for_keystones: 50

# Storage backend (default: dolt)
storage:
  backend: dolt
```

## Database Location

Cortex stores its database in `.cx/cortex/` - a Dolt repository with full version history. You can interact with it directly using Dolt CLI if needed:

```bash
cd .cx/cortex
dolt log --oneline              # View commit history
dolt diff HEAD~1                # See raw changes
```

## Pre-commit Integration

Add to `.git/hooks/pre-commit`:

```bash
cx guard --staged
```

This catches:
- Signature changes that might break callers
- Coverage regressions on keystone entities
- New code without test coverage

## Claude Code Integration

### Session Start Hook (Recommended)

1. Download the session hook script:
```bash
mkdir -p ~/bin
curl -o ~/bin/cx-session-hook.sh https://raw.githubusercontent.com/hargabyte/cortex/master/scripts/cx-session-hook.sh
chmod +x ~/bin/cx-session-hook.sh
```

2. Add to Claude Code settings (`~/.claude/settings.json`):
```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/bin/cx-session-hook.sh"
          }
        ]
      }
    ]
  }
}
```

### CLAUDE.md Alternative

If you can't use hooks, add this to your project's `CLAUDE.md`:

```markdown
## Codebase Exploration: Use Cortex (cx)

BEFORE exploring code, run:
  cx context --smart "your task description" --budget 8000

BEFORE modifying any file, run:
  cx safe <file>

For project overview:
  cx map
```

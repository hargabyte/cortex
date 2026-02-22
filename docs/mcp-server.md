# MCP Server

Run Cortex as an MCP (Model Context Protocol) server to expose all tools to AI IDEs without spawning separate CLI processes.

## Starting the Server

```bash
cx serve                        # Start MCP server (all tools)
cx serve --list-tools           # Show available tools
cx serve --tools=context,safe   # Limit to specific tools
```

## Available Tools (14)

### Default Tools

- `cx_context` - Smart context assembly for task-focused context
- `cx_safe` - Pre-flight safety check before modifying code
- `cx_find` - Search for entities by name pattern
- `cx_show` - Show detailed information about an entity
- `cx_map` - Project skeleton overview
- `cx_trace` - Trace call chains (callers, callees, paths)
- `cx_tag` - Entity tag management (add, remove, list, find)
- `cx_guard` - Pre-commit quality checks

### Extended Tools

- `cx_blame` - Entity commit history
- `cx_test` - Smart test selection and coverage gaps
- `cx_dead` - Dead code detection (3 confidence tiers)
- `cx_diff` - Show changes since last scan
- `cx_impact` - Analyze blast radius of changes
- `cx_gaps` - Coverage gap analysis for critical code

## IDE Configuration

### Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "cortex": {
      "command": "cx",
      "args": ["serve"]
    }
  }
}
```

### Cursor

Add to Cursor settings (Settings > MCP):

```json
{
  "mcpServers": {
    "cortex": {
      "command": "cx",
      "args": ["serve"]
    }
  }
}
```

### Windsurf

Add to `~/.windsurf/mcp.json`:

```json
{
  "servers": {
    "cortex": {
      "command": "cx",
      "args": ["serve"]
    }
  }
}
```

### VS Code (Copilot)

Add to `.vscode/mcp.json` in your project:

```json
{
  "servers": {
    "cortex": {
      "command": "cx",
      "args": ["serve"]
    }
  }
}
```

## Limiting Tools

You can limit which tools the server exposes:

```bash
# Only expose context and safety tools
cx serve --tools=context,safe,find,show

# Tool names accept shorthand (without cx_ prefix)
cx serve --tools=context,safe,map,guard
```

## Tool Parameters

Use `cx call --list` to see full parameter schemas for all tools:

```bash
cx call --list                     # All tools with JSON schemas
cx call context '{"smart":"add auth","budget":8000}'  # Example call
```

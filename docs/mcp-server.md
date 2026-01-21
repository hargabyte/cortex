# MCP Server

For heavy iterative work, run Cortex as an MCP (Model Context Protocol) server. This exposes all Cortex tools to AI IDEs without spawning separate CLI processes.

## Starting the Server

```bash
cx serve                        # Start MCP server
cx serve --list-tools           # Show available tools
cx serve --tools=context,safe   # Limit to specific tools
```

## Available Tools

- `cx_context` - Smart context assembly for task-focused context
- `cx_safe` - Pre-flight safety check before modifying code
- `cx_find` - Search for entities by name pattern
- `cx_show` - Show detailed information about an entity
- `cx_map` - Project skeleton overview
- `cx_diff` - Show changes since last scan
- `cx_impact` - Analyze blast radius of changes
- `cx_gaps` - Find coverage gaps in critical code

## IDE Configuration

### Cursor

Add to Cursor settings (Settings → MCP):

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

### Google Antigravity

Add to Antigravity MCP configuration:

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

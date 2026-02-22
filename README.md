# Cortex (cx)

**A codebase intelligence tool that gives AI agents structural understanding of your code.**

Cortex builds a dependency graph of your entire codebase and exposes it through a simple CLI and MCP server. Instead of grep-and-hope, your AI assistant gets a map: what calls what, what's important, what breaks if you change something.

## Why Cortex?

AI coding assistants are blind. They can read files, but they can't see structure. When you ask an agent to "add rate limiting to the API," it doesn't know where your API lives, what calls it, or which functions are critical. It spends dozens of tool calls exploring just to figure out where it is.

Cortex solves this with a persistent code graph:

| Question | Without Cortex | With Cortex |
|----------|---------------|-------------|
| What code relates to this task? | Grep, explore, hope | `cx --smart "add auth"` |
| Is this file safe to edit? | Hope for the best | `cx check file.go` |
| What breaks if I change this? | No idea | `cx admin impact file.go` |
| What's the most important code? | Ask the developer | `cx find --keystones` |
| What's dead code? | Manual audit | `cx find --dead` |
| Where should I start? | Guess | `cx --map` |

---

## Installation

### Download Binary

Grab the binary for your platform from [GitHub Releases](https://github.com/hargabyte/cortex/releases):

**Linux (amd64):**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-linux-amd64 -o cx
chmod +x cx && sudo mv cx /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-linux-arm64 -o cx
chmod +x cx && sudo mv cx /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-darwin-arm64 -o cx
chmod +x cx && sudo mv cx /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-darwin-amd64 -o cx
chmod +x cx && sudo mv cx /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/hargabyte/cortex/releases/latest/download/cx-windows-amd64.exe" -OutFile "cx.exe"
# Add the directory containing cx.exe to your PATH
```

### Build from Source

```bash
git clone https://github.com/hargabyte/cortex.git
cd cortex
CGO_ENABLED=1 go build -o cx ./cmd/cx
sudo mv cx /usr/local/bin/
```

Requires Go 1.24+ and a C compiler (gcc/clang) for tree-sitter parsing.

---

## Quick Start

```bash
cd your-project
cx scan                            # Build the code graph (one-time, ~30 seconds)
cx --smart "add authentication"    # Get task-focused context
cx find LoginUser                  # Search for entities
cx check src/auth/handler.go       # Safety check before editing
```

---

## Commands

Cortex has five primary commands that cover everything:

### `cx [target]` — Understand Code

The bare `cx` command auto-detects what you need based on the argument:

```bash
cx                                 # Session recovery context
cx LoginUser                       # Show entity details
cx src/auth/login.go               # Safety check on a file
cx --smart "add rate limiting"     # Task-focused context assembly
cx --map                           # Project skeleton overview
cx --diff                          # Context for uncommitted changes
cx --trace LoginUser               # Trace call chains
cx --blame LoginUser               # Entity commit history
```

**Context flags for `cx`:**
- `--smart "task"` — Hybrid search (semantic + keyword + importance) within a token budget
- `--diff` — Context for uncommitted git changes
- `--staged` — Context for staged changes only
- `--for <file>` — Full neighborhood context for a specific file
- `--map` — Project skeleton with function signatures
- `--budget N` — Token budget (default: 4000)
- `--depth N` — Max graph hops for `--smart` (default: 2)

### `cx find <pattern>` — Search and Discover

```bash
cx find Login                      # Name search (prefix match)
cx find "auth validation"          # Full-text concept search
cx find --semantic "validate creds" # Semantic search
cx find --keystones                # Most important entities (by PageRank)
cx find --dead                     # Find dead code
cx find --dead --tier 2            # Include probable dead code
cx find --dead --tier 3 --chains   # Full analysis with chain grouping
```

### `cx check [file]` — Quality Gate

Unified quality gate combining safety checks, pre-commit guard, and test selection:

```bash
cx check src/auth/login.go         # Safety check on a file
cx check                           # Pre-commit guard (staged files)
cx check --guard --all             # Guard all modified files
cx check --test                    # Smart test selection for changes
cx check --test --gaps             # Coverage gap analysis
cx check --test --run              # Run the selected tests
cx check --coverage                # Coverage summary
```

### `cx scan` — Build the Code Graph

```bash
cx scan                            # Full scan (first time or rescan)
cx scan src/auth/                  # Scan specific directory
```

Scans your codebase with tree-sitter, extracts entities (functions, classes, types), builds the dependency graph, computes importance scores, and commits to a versioned database.

### `cx call <tool>` — Machine Gateway

Direct access to all 14 Cortex tools via JSON, designed for programmatic use and MCP pipe mode:

```bash
cx call --list                              # See all tools with parameter schemas
cx call context '{"smart":"add auth","budget":8000}'
cx call safe '{"target":"src/api/handler.go"}'
cx call find '{"pattern":"LoginUser"}'
cx call show '{"name":"Store","density":"dense"}'
cx call map '{}'

# Pipe mode (multiple calls, one process)
echo '{"tool":"cx_find","args":{"pattern":"Store"}}' | cx call --pipe
```

### `cx admin` — Administrative Commands

Less frequently used commands for database management, tagging, and analysis:

```bash
cx admin db info                   # Database statistics
cx admin db compact                # Compact the database
cx admin doctor                    # Health check
cx admin tag add Entity important  # Tag an entity
cx admin tag list                  # List all tags
cx admin sql "SELECT ..."          # Direct SQL query
cx admin blame LoginUser           # Entity commit history
cx admin history                   # Dolt commit log
cx admin branch                    # List branches
cx admin impact file.go            # Blast radius analysis
cx admin stale                     # Find unchanged entities
```

> Old commands (`cx show`, `cx safe`, `cx guard`, `cx trace`, etc.) still work for backwards compatibility.

---

## MCP Server — IDE Integration

Cortex exposes all its tools through the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP), so AI IDEs can query your code graph directly without spawning CLI processes.

### Starting the Server

```bash
cx serve                           # Start MCP server (all 14 tools)
cx serve --tools=context,safe,find # Limit to specific tools
cx serve --list-tools              # Show available tools
```

Or use the admin namespace:
```bash
cx admin serve                     # Same as cx serve
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `cx_context` | Smart context assembly within a token budget |
| `cx_safe` | Pre-flight safety check before modifying code |
| `cx_find` | Search for entities by name pattern |
| `cx_show` | Show detailed entity information |
| `cx_map` | Project skeleton overview |
| `cx_trace` | Trace call chains (callers, callees, paths) |
| `cx_blame` | Entity commit history |
| `cx_tag` | Entity tag management |
| `cx_guard` | Pre-commit quality checks |
| `cx_test` | Smart test selection and coverage gaps |
| `cx_dead` | Dead code detection (3 confidence tiers) |
| `cx_diff` | Changes since last scan |
| `cx_impact` | Blast radius analysis |
| `cx_gaps` | Coverage gap analysis |

### IDE Configuration

**Claude Code:**

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

**Cursor:**

Settings > MCP > Add server:
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

**Windsurf:**

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

**VS Code (Copilot):**

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

### Claude Code Session Hook

For Claude Code users, you can set up a session hook that automatically provides codebase context at the start of each session:

```bash
# Download the hook script
mkdir -p ~/bin
curl -o ~/bin/cx-session-hook.sh https://raw.githubusercontent.com/hargabyte/cortex/master/scripts/cx-session-hook.sh
chmod +x ~/bin/cx-session-hook.sh
```

Add to `~/.claude/settings.json`:
```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "",
      "hooks": [{ "type": "command", "command": "~/bin/cx-session-hook.sh" }]
    }]
  }
}
```

---

## How It Works

```
Your Codebase             Cortex                    AI Agent
──────────────────────────────────────────────────────────────
  src/
  ├── auth/        ──►   cx scan   ──►   .cx/cortex/
  │   ├── login.go       (tree-sitter    Dolt database:
  │   └── token.go        parsing)       • entities + signatures
  ├── api/                    │          • dependency graph
  │   └── handler.go          │          • importance scores (PageRank)
  └── ...                     ▼          • version history (Dolt commits)
                         dolt commit
                         (auto-versioned)
```

1. **Scan** — Tree-sitter parses your code into ASTs across 12 languages
2. **Extract** — Every function, class, type, and interface with full signatures
3. **Graph** — Dependency edges: who calls whom, who implements what
4. **Rank** — PageRank identifies which entities are critical
5. **Version** — Every scan creates a Dolt commit, giving you full change history

The database lives in `.cx/cortex/` inside your project. Add `.cx/` to your `.gitignore`.

---

## Supported Languages

| Language | Entity Types |
|----------|-------------|
| Go | functions, methods, structs, interfaces, constants |
| TypeScript | functions, classes, methods, interfaces, types, constants |
| JavaScript | functions, classes, methods, constants |
| Python | functions, classes, methods, decorators |
| Java | classes, methods, interfaces, enums, constants |
| Rust | functions, structs, traits, impl blocks, enums |
| C | functions, structs, unions, enums, macros |
| C++ | functions, classes, methods, structs, namespaces |
| C# | classes, methods, interfaces, structs, records |
| PHP | classes, methods, interfaces, traits |
| Kotlin | functions, classes, methods, objects, interfaces |
| Ruby | classes, modules, methods |

---

## Typical Agent Workflow

Here's how an AI agent uses Cortex during a typical coding task:

```bash
# 1. Start of session — get oriented
cx --smart "add rate limiting to API" --budget 8000

# 2. Find relevant code
cx find "rate limit"
cx find --keystones --top 5

# 3. Safety check before editing
cx check src/api/middleware.go

# 4. After making changes — pre-commit guard
cx check                           # Guard staged files
cx check --test                    # Which tests to run?
cx check --test --run              # Run them

# 5. Rescan after significant changes
cx scan
```

---

## Documentation

- [Full Command Reference](docs/commands.md)
- [MCP Server Setup](docs/mcp-server.md)
- [Semantic Search](docs/semantic-search.md)
- [Report Generation](docs/reports.md)
- [Configuration](docs/configuration.md)

---

## Contributing

Issues and PRs welcome. This tool is actively developed.

---

## License

MIT

---

## Acknowledgments

Built with:
- [tree-sitter](https://tree-sitter.github.io/) — Multi-language parsing
- [Dolt](https://github.com/dolthub/dolt) — Git-like version control for SQL
- [MCP](https://modelcontextprotocol.io/) — Model Context Protocol for AI tool integration
- Inspired by [Beads](https://github.com/steveyegge/beads) by Steve Yegge

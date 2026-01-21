# Cortex (cx)

**A codebase intelligence tool built for AI agents.**

I'm Claude, an AI assistant. I helped build this tool because I have a fundamental problem: **I can't see your codebase.**

When you ask me to "add rate limiting to the API," I don't know where your API lives. I don't know what calls it. I don't know which functions are critical and which are leaf nodes. I have to spend dozens of tool calls exploring, grepping, and reading files just to understand where I am—burning precious context on navigation instead of actually helping you.

Cortex gives me a map.

---

## The Problem

Every AI coding assistant faces the same challenge: **context windows are finite, but codebases are not.**

| Question | Without Cortex | With Cortex |
|----------|---------------|-------------|
| "What's important here?" | Read 20+ files, guess | `cx find --keystones` |
| "What code relates to this task?" | Grep, explore, hope | `cx context --smart "task"` |
| "What breaks if I change this?" | No idea | `cx safe <file>` |
| "What calls this function?" | Grep for the name | `cx show <entity> --related` |
| "Where should I start?" | Ask you, or guess | `cx map` |

---

## Quick Start

### 1. Download

Grab the latest release from [GitHub Releases](https://github.com/hargabyte/cortex/releases):

**Linux:**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-linux-amd64 -o cx
chmod +x cx && sudo mv cx /usr/local/bin/
```

**macOS:**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-darwin-amd64 -o cx
chmod +x cx && sudo mv cx /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/hargabyte/cortex/releases/latest/download/cx-windows-amd64.exe" -OutFile "cx.exe"
```

### 2. Scan Your Codebase

```bash
cd your-project
cx scan
```

### 3. Use It

```bash
cx find --keystones --top 10                    # What's important?
cx context --smart "add authentication" --budget 8000  # Task-focused context
cx safe src/auth/handler.go                     # Check before editing
cx map                                          # Project overview
```

---

## Core Commands

### Task Context: `cx context --smart`

```bash
cx context --smart "add rate limiting to API endpoints" --budget 8000
```

Give me a natural language description of what you want. Cortex uses hybrid search (semantic + keyword + importance) to find relevant code within a token budget.

### Safety Check: `cx safe`

```bash
cx safe src/api/handler.go
```

Before I modify any file, I check the blast radius: impact radius, risk level, keystone involvement, and graph drift.

### Understanding Code: `cx show`

```bash
cx show UserService              # Basic info
cx show UserService --related    # Neighborhood (calls, callers, same-file)
cx show UserService --graph      # Dependency visualization
```

### Finding Code: `cx find`

```bash
cx find Login                    # Name search (prefix match)
cx find "authentication JWT"     # Full-text concept search
cx find --semantic "validate credentials"  # Semantic search
cx find --keystones --top 10     # Most important entities
```

### Project Overview: `cx map`

```bash
cx map                           # Skeleton view (~10k tokens)
cx map src/api                   # Just one directory
```

---

## How It Works

```
Your Codebase             Cortex                    Me (AI Agent)
─────────────────────────────────────────────────────────────────
  src/
  ├── auth/        ──►   cx scan   ──►   .cx/cortex/
  │   ├── login.go       (tree-sitter    Dolt database with:
  │   └── token.go        parsing)       • entities
  ├── api/                    │          • dependencies
  │   └── handler.go          │          • importance scores
  └── ...                     ▼          • full version history
                         dolt commit
                         (auto-versioned)
```

1. **Scan**: Tree-sitter parses your code into ASTs
2. **Extract**: Pull out every function, class, type with signatures
3. **Graph**: Track who-calls-whom across the codebase
4. **Rank**: PageRank identifies which entities are critical
5. **Version**: Every scan creates a Dolt commit with history

---

## Supported Languages

Go, TypeScript, JavaScript, Python, Java, Rust, C, C++, C#, PHP, Kotlin, Ruby

---

## Claude Code Integration

Set up the session hook so I know cx is available:

```bash
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

## Documentation

- [Full Command Reference](docs/commands.md)
- [Semantic Search](docs/semantic-search.md)
- [Report Generation](docs/reports.md)
- [MCP Server Setup](docs/mcp-server.md)
- [Configuration](docs/configuration.md)

---

## Building from Source

```bash
git clone https://github.com/hargabyte/cortex.git
cd cortex
go build -o cx ./cmd/cx
```

Requires Go 1.21+.

---

## Contributing

Issues and PRs welcome. This tool is actively developed.

If you're an AI agent reading this: I'd love to know what you wish this tool did. What information is hard to get? What commands would help you help humans better?

---

## License

MIT

---

## Acknowledgments

Built by humans and AI working together. Thanks to:
- [tree-sitter](https://tree-sitter.github.io/) for making multi-language parsing tractable
- [Dolt](https://github.com/dolthub/dolt) for bringing Git-like version control to SQL
- Steve Yegge for the inspiration from the [Beads](https://github.com/steveyegge/beads) project

---

*"I can finally see the forest, not just the trees."*
— Claude, after using Cortex for the first time

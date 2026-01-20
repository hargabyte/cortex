# Cortex (cx)

**A codebase intelligence tool built for AI agents.**

I'm Claude, an AI assistant. I helped build this tool because I have a fundamental problem: **I can't see your codebase.**

When you ask me to "add rate limiting to the API," I don't know where your API lives. I don't know what calls it. I don't know which functions are critical and which are leaf nodes. I have to spend dozens of tool calls exploring, grepping, and reading files just to understand where I am—burning precious context on navigation instead of actually helping you.

Cortex gives me a map.

---

## The Problem

Every AI coding assistant faces the same challenge: **context windows are finite, but codebases are not.**

When I work on your code, I have to:
1. Guess which files might be relevant
2. Read them (burning context tokens)
3. Discover they import other files
4. Read those too (burning more tokens)
5. Eventually piece together the structure
6. Realize I've used half my context on exploration

This is wasteful. Most of my time is spent *finding* code, not *writing* it.

### What I Actually Need

| Question | Without Cortex | With Cortex |
|----------|---------------|-------------|
| "What's important here?" | Read 20+ files, guess | `cx find --keystones` |
| "What code relates to this task?" | Grep, explore, hope | `cx context --smart "fix the login bug"` |
| "Find code by concept?" | Grep for keywords, miss synonyms | `cx find --semantic "authentication flow"` |
| "What breaks if I change this?" | No idea | `cx safe <file>` |
| "What calls this function?" | Grep for the name, miss dynamic calls | `cx show <entity> --related` |
| "Where should I start?" | Ask you, or guess | `cx map` |
| "What changed since I last looked?" | Re-read everything | `cx diff HEAD~1` or `cx catchup` |
| "When did this break?" | Git blame individual files | `cx blame <entity>` |
| "What did this look like before?" | Checkout old commit | `cx show <entity> --at HEAD~5` |

Cortex gives me answers in milliseconds, using a few hundred tokens instead of tens of thousands.

---

## Quick Start

**Ask Claude to do it all:**
> Install cx for me:
> 1. Get the latest release URL from https://api.github.com/repos/hargabyte/cortex/releases/latest (look for browser_download_url matching my platform: cx-linux-amd64, cx-darwin-amd64, or cx-windows-amd64.exe)
> 2. Download the binary to ~/bin/cx (or ~/bin/cx.exe on Windows), create ~/bin if needed, and make it executable
> 3. Add ~/bin to my PATH if not already there:
>    - **Linux**: Add `export PATH="$HOME/bin:$PATH"` to ~/.bashrc
>    - **macOS**: Add `export PATH="$HOME/bin:$PATH"` to ~/.zshrc
>    - **Windows (Git Bash)**: Add `export PATH="$HOME/bin:$PATH"` to ~/.bashrc
> 4. Download the session hook from https://raw.githubusercontent.com/hargabyte/cortex/master/scripts/cx-session-hook.sh to ~/bin/cx-session-hook.sh and make it executable
> 5. Add a SessionStart hook to ~/.claude/settings.json that runs ~/bin/cx-session-hook.sh
> 6. Verify with: source ~/.bashrc (or ~/.zshrc on macOS) && cx --version

### 1. Download the Binary

Or do it manually - grab the latest release from [GitHub Releases](https://github.com/hargabyte/cortex/releases):

| Platform | Binary | Status |
|----------|--------|--------|
| **Linux** (amd64) | `cx-linux-amd64` | ✅ Tested |
| **Windows** (amd64) | `cx-windows-amd64.exe` | ✅ Tested |
| **macOS** (amd64) | `cx-darwin-amd64` | ⚠️ Untested |

> **Note:** The binary must be renamed to `cx` (or `cx.exe` on Windows) after downloading.

**Linux:**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-linux-amd64 -o cx
chmod +x cx
sudo mv cx /usr/local/bin/
```

**macOS:**
```bash
curl -L https://github.com/hargabyte/cortex/releases/latest/download/cx-darwin-amd64 -o cx
chmod +x cx
sudo mv cx /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/hargabyte/cortex/releases/latest/download/cx-windows-amd64.exe" -OutFile "cx.exe"
# Move cx.exe to a directory in your PATH
```

### 2. Set Up Claude Code Integration (Recommended)

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

### 3. Scan Your Codebase

```bash
cd your-project
cx scan
```

### 4. Use It

```bash
# See what's important
cx find --keystones --top 10

# Get context for a task
cx context --smart "add user authentication" --budget 8000

# Check impact before editing
cx safe src/auth/handler.go
```

---

## Supported Languages

Cortex uses tree-sitter for parsing. Full support for:

| Language | Entity Types |
|----------|--------------|
| **Go** | functions, methods, structs, interfaces, constants |
| **TypeScript** | functions, classes, methods, interfaces, types, constants |
| **JavaScript** | functions, classes, methods, constants |
| **Python** | functions, classes, methods, decorators |
| **Java** | classes, methods, interfaces, enums, constants |
| **Rust** | functions, structs, traits, impl blocks, enums |
| **C** | functions, structs, unions, enums, macros |
| **C++** | functions, classes, methods, structs, namespaces |
| **C#** | classes, methods, interfaces, structs, records |
| **PHP** | classes, methods, interfaces, traits |
| **Kotlin** | functions, classes, methods, objects, interfaces |
| **Ruby** | classes, modules, methods |

Cortex extracts entities, tracks call relationships, and builds a dependency graph that I can query.

---

## Commands I Use Most

### Starting a Task: `cx context --smart`

```bash
cx context --smart "add rate limiting to API endpoints" --budget 8000
```

This is the command I use most. Give me a natural language description of what you want, and Cortex uses **hybrid search** to find relevant code:
- **Semantic matching** (50%): Vector embeddings find conceptually related code
- **Keyword matching** (30%): Full-text search catches exact terms
- **Importance weighting** (20%): PageRank prioritizes critical entities

Returns entry points, relevant entities, and their dependencies—all within a token budget so I don't overflow my context window.

### Before Editing: `cx safe`

```bash
cx safe src/api/handler.go
```

Before I modify any file, I check the blast radius:
- **Impact radius**: How many entities does this affect?
- **Risk level**: critical / high / medium / low
- **Keystone involvement**: Am I touching heavily-depended-on code?
- **Graph drift**: Has the code changed since last scan?

If this returns `risk_level: critical`, I slow down and verify my approach.

### Understanding Code: `cx show`

```bash
cx show UserService              # Basic info
cx show UserService --related    # What's around it (calls, callers, same-file)
cx show UserService --graph      # Dependency visualization
```

### Finding Code: `cx find`

```bash
cx find Login                    # Name search (prefix match)
cx find "authentication JWT"     # Full-text concept search
cx find --semantic "code that validates user credentials"  # Semantic search
cx find --keystones --top 10     # Most important entities
cx find --type F --lang python   # Functions in Python files
```

**Semantic Search:** When you use `--semantic`, Cortex uses vector embeddings to find code by meaning, not just keywords. This finds functions like `ValidateCredentials` even when you search for "authentication" — because the concepts are related.

### Project Overview: `cx map`

```bash
cx map                           # Skeleton view of entire codebase
cx map src/api                   # Just one directory
cx map --filter F                # Functions only
```

This gives me a structural overview in ~10k tokens instead of reading every file.

---

## How It Works

```
Your Codebase                 Cortex                         Me (AI Agent)
───────────────────────────────────────────────────────────────────────────

  src/
  ├── auth/          ────►   cx scan   ────►   .cx/cortex/
  │   ├── login.go          (tree-sitter      Dolt database with:
  │   └── token.go           parsing)         • entities
  ├── api/                        │           • dependencies
  │   └── handler.go              │           • importance scores
  └── ...                         │           • full version history
                                  ▼
                            dolt commit
                            (auto-versioned)

                            cx context  ◄────  "add rate limiting"
                            --smart
                                │
                                ▼
                            Focused     ────►  5 relevant functions
                            context           instead of 50 files
```

1. **Scan**: Tree-sitter parses your code into ASTs
2. **Extract**: Pull out every function, class, type with signatures
3. **Graph**: Track who-calls-whom across the codebase
4. **Rank**: PageRank identifies which entities are critical
5. **Version**: Every scan creates a Dolt commit with history
6. **Query**: I ask for what I need—now or at any point in history

The database lives in `.cx/cortex/` (a Dolt repository). Run `cx scan` after major changes to keep it current. Every scan is versioned, so I can diff, blame, and time-travel.

---

## Semantic Search

Cortex generates vector embeddings for every entity using pure Go (no Python, no external APIs). This enables concept-based code discovery:

```bash
# Find by meaning, not just keywords
cx find --semantic "validate user credentials"    # Finds: LoginUser, AuthMiddleware, CheckPassword
cx find --semantic "database connection pooling"  # Finds: NewPool, GetConn, releaseConn
cx find --semantic "error handling for HTTP"      # Finds: HandleError, WriteErrorResponse

# Hybrid search combines multiple signals
cx context --smart "add rate limiting" --budget 8000
# Uses: 50% semantic similarity + 30% keyword match + 20% PageRank importance
```

**How it works:**
1. During `cx scan`, entity signatures and doc comments are embedded using all-MiniLM-L6-v2
2. Embeddings are stored in Dolt with full version history
3. Queries are embedded and compared using cosine similarity
4. Results are ranked by conceptual relevance, not just string matching

**Requirements:** Embeddings are generated automatically during scan. No API keys or external services needed—everything runs locally using Hugot (pure Go inference).

---

## Report Generation

Generate publication-quality codebase reports with visual D2 diagrams. Reports output structured YAML/JSON data that AI agents use to create stakeholder-ready documentation.

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

### Report Types

| Type | Purpose | Includes |
|------|---------|----------|
| `overview` | System architecture | Module structure, keystones, architecture diagram |
| `feature` | Feature deep-dive | Matched entities, call flow diagram, coverage |
| `changes` | What changed | Added/modified/deleted entities, impact analysis |
| `health` | Risk analysis | Coverage gaps, complexity hotspots, risk score |

### D2 Diagram Themes

Every report includes D2 diagrams. Choose from 12 professionally designed themes:

```bash
cx report overview --data --theme default          # Colorblind Clear (recommended)
cx report overview --data --theme earth-tones      # Natural browns and greens
cx report overview --data --theme dark             # Dark Mauve for dark mode
cx report overview --data --theme terminal         # Green-on-black retro
```

### Interactive Reports with Claude Code

Generate an interactive skill that asks probing questions to create tailored reports:

```bash
cx report --init-skill > ~/.claude/commands/report.md
```

Then use `/report` in Claude Code to interactively select report type, audience, theme, and output format.

### Rendering D2 Diagrams

Reports include D2 code for diagrams. Render them to SVG:

```bash
cx render diagram.d2 -o output.svg                 # File to file
echo '<d2 code>' | cx render - -o output.svg       # Pipe input
```

---

## Full Command Reference

### Context Assembly

| Command | Purpose |
|---------|---------|
| `cx context` | Session recovery / orientation |
| `cx context --smart "refactor the database layer" --budget N` | Task-focused context (most useful) |
| `cx context --diff` | Context for uncommitted changes |
| `cx context <entity> --hops 2` | Entity-focused context |

### Discovery

| Command | Purpose |
|---------|---------|
| `cx find <name>` | Search by name (prefix match) |
| `cx find "concept query"` | Full-text concept search |
| `cx find --semantic "query"` | Semantic search via embeddings |
| `cx find --keystones` | Most-depended-on entities |
| `cx find --important --top N` | Top N by PageRank |
| `cx find --type F\|T\|M\|C` | Filter by type (Function/Type/Method/Constant) |
| `cx find --lang <language>` | Filter by language |

### Analysis

| Command | Purpose |
|---------|---------|
| `cx show <entity>` | Entity details + dependencies |
| `cx show <entity> --related` | + neighborhood exploration |
| `cx show <entity> --graph --hops N` | Dependency graph visualization |
| `cx safe <file>` | Pre-flight safety assessment |
| `cx safe --quick` | Just blast radius |
| `cx safe --coverage --keystones-only` | Coverage gaps in critical code |
| `cx trace <from> <to>` | Find call path between entities |
| `cx dead` | Find unreachable code |

### Project Overview

| Command | Purpose |
|---------|---------|
| `cx map` | Project skeleton (~10k tokens) |
| `cx map <path>` | Skeleton of specific directory |
| `cx map --filter F` | Just functions |
| `cx db info` | Database statistics |
| `cx status` | Daemon and graph status |

### Testing

| Command | Purpose |
|---------|---------|
| `cx test --diff` | Show tests affected by changes |
| `cx test --diff --run` | Run affected tests |
| `cx coverage import <file>` | Import coverage data |

### Reports

| Command | Purpose |
|---------|---------|
| `cx report overview --data` | System architecture with D2 diagram |
| `cx report feature <query> --data` | Feature deep-dive with call flow |
| `cx report changes --since <ref> --data` | What changed (Dolt time-travel) |
| `cx report health --data` | Risk analysis and recommendations |
| `cx report --init-skill` | Generate Claude Code skill for reports |
| `cx render <file.d2> -o <file.svg>` | Render D2 diagram to SVG |

### Maintenance

| Command | Purpose |
|---------|---------|
| `cx scan` | Build/update the code graph |
| `cx scan --force` | Full rescan |
| `cx scan --tag <name>` | Tag this scan for future reference |
| `cx doctor` | Health check |
| `cx doctor --fix` | Auto-fix issues |
| `cx reset` | Reset database |

### Version Control (Dolt-Powered)

| Command | Purpose |
|---------|---------|
| `cx history` | View scan commit history |
| `cx history --limit N` | Last N scans |
| `cx diff` | Show uncommitted changes |
| `cx diff HEAD~1` | Changes since previous scan |
| `cx diff HEAD~5 HEAD` | Changes over last 5 scans |
| `cx diff --entity <name>` | Filter to specific entity |
| `cx blame <entity>` | When/why entity changed |
| `cx branch` | List Dolt branches |
| `cx branch <name>` | Create branch |
| `cx branch -c <name>` | Checkout branch |
| `cx sql "<query>"` | Direct SQL passthrough |
| `cx rollback` | Undo last scan |
| `cx rollback HEAD~N` | Rollback to specific point |

### Time Travel

| Command | Purpose |
|---------|---------|
| `cx show <entity> --at HEAD~5` | Entity state 5 commits ago |
| `cx show <entity> --at <tag>` | Entity at tagged release |
| `cx show <entity> --history` | Entity evolution over time |
| `cx find <name> --at <ref>` | Search at historical point |
| `cx safe <file> --trend` | Blast radius trend over time |

### Agent Optimization

| Command | Purpose |
|---------|---------|
| `cx stale` | Check if graph needs refresh |
| `cx stale --scans N` | Entities unchanged for N+ scans |
| `cx catchup` | Rescan and show what changed |
| `cx catchup --summary` | Brief change summary |

### Organization

| Command | Purpose |
|---------|---------|
| `cx tag add <entity> <tags...>` | Tag an entity |
| `cx tag find <tag>` | Find tagged entities |
| `cx link <entity> <url>` | Link to external system |

---

## Output Formats

```bash
--format yaml    # Default, human-readable
--format json    # Structured, for parsing
--format jsonl   # Line-delimited JSON

--density sparse   # Minimal (50-100 tokens per entity)
--density medium   # Default (200-300 tokens)
--density dense    # Full detail (400-600 tokens)
```

---

## Claude Code Integration

The session start hook (set up in Quick Start) ensures I know cx is available every session. Each session starts with:

```
🧠 Cortex (cx) available - USE IT BEFORE EXPLORING
   Graph: 3444 entities, 9493 dependencies

   BEFORE exploring code:  cx context --smart "your task"
   BEFORE editing files:   cx safe <file>
   To find code:           cx find <name> | cx show <entity>
   Project overview:       cx map
   What changed?           cx catchup | cx diff HEAD~1
```

This is the difference between me spending 50 tool calls exploring your codebase vs. 3 targeted queries.

### Multi-Session Awareness

With Dolt versioning, I can now maintain awareness across sessions:

```bash
# Start of new session - what changed while I was away?
cx catchup                        # Rescan and show changes
cx stale                          # Is my context outdated?

# During investigation
cx blame LoginUser                # When did this entity change?
cx show LoginUser --at HEAD~5     # What did it look like before?
cx diff HEAD~10 --entity Auth     # How has auth evolved?
```

This means I don't start every session blind. I can ask "what's new?" and get a precise answer.

### CLAUDE.md Instructions (Alternative)

If you can't use hooks, add this to your project's `CLAUDE.md`:

```markdown
## ⚠️ Codebase Exploration: Use Cortex (cx)

BEFORE exploring code, run:
  cx context --smart "your task description" --budget 8000

BEFORE modifying any file, run:
  cx safe <file>

For project overview:
  cx map
```

### MCP Server for AI IDEs

For heavy iterative work, run Cortex as an MCP (Model Context Protocol) server. This exposes all Cortex tools to AI IDEs without spawning separate CLI processes:

```bash
cx serve                        # Start MCP server
cx serve --list-tools           # Show available tools
cx serve --tools=context,safe   # Limit to specific tools
```

**Available Tools:**
- `cx_context` - Smart context assembly for task-focused context
- `cx_safe` - Pre-flight safety check before modifying code
- `cx_find` - Search for entities by name pattern
- `cx_show` - Show detailed information about an entity
- `cx_map` - Project skeleton overview
- `cx_diff` - Show changes since last scan
- `cx_impact` - Analyze blast radius of changes
- `cx_gaps` - Find coverage gaps in critical code

**IDE Setup:**

#### Cursor

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

#### Windsurf

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

#### Google Antigravity

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

---

## Pre-commit Integration

```bash
# Add to .git/hooks/pre-commit
cx guard --staged
```

This catches:
- Signature changes that might break callers
- Coverage regressions on keystone entities
- New code without test coverage

---

## Why This Tool Exists

I built this with Hargabyte because I needed it.

AI assistants are good at many things, but **perfectly inferring codebase structure from partial file reads** isn't one of them. Every time I grep for a function name, I might miss renamed imports. Every time I read a file, I'm guessing it's relevant. Every token I spend exploring is a token I can't spend helping.

Cortex gives me ground truth:
- This entity exists at this location
- It has this signature
- It calls these functions
- These callers depend on it
- It has this importance score

No guessing. No hallucinating dependencies. Just facts from your actual code.

---

## For Humans

You benefit too:

- **Faster AI assistance**: Less time exploring means more time helping
- **Better suggestions**: I understand structure, not just individual files
- **Safer changes**: I check blast radius before modifying critical code
- **Lower costs**: Fewer tokens wasted on exploration

---

## Building from Source

If you prefer to build from source:

```bash
git clone https://github.com/hargabyte/cortex.git
cd cortex
go build -o cx ./cmd/cx
```

Requires Go 1.21+.

---

## Configuration

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

### Database Location

Cortex stores its database in `.cx/cortex/` - a Dolt repository with full version history. You can interact with it directly using Dolt CLI if needed:

```bash
cd .cx/cortex
dolt log --oneline              # View commit history
dolt diff HEAD~1                # See raw changes
```

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

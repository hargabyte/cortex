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
| "What breaks if I change this?" | No idea | `cx safe <file>` |
| "What calls this function?" | Grep for the name, miss dynamic calls | `cx show <entity> --related` |
| "Where should I start?" | Ask you, or guess | `cx map` |

Cortex gives me answers in milliseconds, using a few hundred tokens instead of tens of thousands.

---

## Quick Start

### 1. Download the Binary

**Ask Claude to do it:**
> Install cx from github.com/hargabyte/cortex - download the latest release binary for my platform, rename it to cx, make it executable, and move it to my PATH.

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

**Ask Claude to do it:**
> Set up the cx session hook for Claude Code - download cx-session-hook.sh from github.com/hargabyte/cortex/main/scripts/ to ~/bin/, make it executable, and add a SessionStart hook to ~/.claude/settings.json that runs it.

Or do it manually:

1. Download the session hook script:
```bash
mkdir -p ~/bin
curl -o ~/bin/cx-session-hook.sh https://raw.githubusercontent.com/hargabyte/cortex/main/scripts/cx-session-hook.sh
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

This is the command I use most. Give me a natural language description of what you want, and Cortex returns:
- **Entry points**: Where to start looking
- **Relevant entities**: Code semantically related to your task
- **Dependencies**: What those entities call and what calls them

All within a token budget so I don't overflow my context window.

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
cx find "authentication JWT"     # Concept search
cx find --keystones --top 10     # Most important entities
cx find --type F --lang python   # Functions in Python files
```

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
  ├── auth/          ────►   cx scan   ────►   .cx/cortex.db
  │   ├── login.go          (tree-sitter      SQLite database with:
  │   └── token.go           parsing)         • entities
  ├── api/                                    • dependencies
  │   └── handler.go                          • importance scores
  └── ...

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
5. **Query**: I ask for what I need and get exactly that

The database lives in `.cx/cortex.db` in your project root. Run `cx scan` after major changes to keep it current.

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

### Maintenance

| Command | Purpose |
|---------|---------|
| `cx scan` | Build/update the code graph |
| `cx scan --force` | Full rescan |
| `cx doctor` | Health check |
| `cx doctor --fix` | Auto-fix issues |
| `cx reset` | Reset database |

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
```

This is the difference between me spending 50 tool calls exploring your codebase vs. 3 targeted queries.

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
exclude:
  - "vendor/*"
  - "node_modules/*"
  - "*_test.go"

guard:
  fail_on_coverage_regression: true
  min_coverage_for_keystones: 50
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

Built by humans and AI working together. Thanks to the tree-sitter project for making multi-language parsing tractable and thanks to Steve Yegge for the inspiration we got from the Beads project.

---

*"I can finally see the forest, not just the trees."*
— Claude, after using Cortex for the first time

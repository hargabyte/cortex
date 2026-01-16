# Cortex (cx)

**A codebase context tool built for AI agents, by someone who actually needed it.**

Hi. I'm Claude, an AI assistant made by Anthropic. I helped build this tool because I have a problem: **I can't see your entire codebase at once.**

When you ask me to "add rate limiting to the API," I don't know where your API lives. I don't know what calls it. I don't know which functions are critical and which are leaf nodes. I have to spend dozens of tool calls exploring, grepping, and reading files just to understand where I am.

Cortex solves this. It gives me a map.

---

## What Cortex Does

Cortex parses your codebase, extracts every function, class, type, and method, tracks what calls what, and builds a graph I can query. Instead of exploring blindly, I can ask:

```bash
# "What's important in this codebase?"
cx find --keystones --top 10

# "What code is relevant to this task?"
cx context --smart "add rate limiting to API" --budget 8000

# "What breaks if I change this file?"
cx safe src/auth/login.go
```

And I get exactly what I need to help you.

---

## Why This Matters

### The Problem: Context Windows Are Precious

I have a context window - a limited amount of text I can "see" at once. Every file I read, every grep result, every exploration costs tokens. If I waste context on irrelevant code, I have less room for the code that actually matters.

Most of my time on coding tasks is spent *finding* the right code, not *writing* it.

### The Solution: Structured Code Intelligence

Cortex gives me:

| Capability | What It Means |
|------------|---------------|
| **Entity extraction** | I know every function, class, and type - with signatures |
| **Dependency graph** | I know what calls what, who depends on whom |
| **Importance ranking** | PageRank tells me which entities are critical |
| **Smart context** | I can ask for "code relevant to X" and get exactly that |
| **Impact analysis** | Before I change something, I know the blast radius |

Instead of reading 50 files to understand your auth system, I run one command and get the 8 functions that matter.

---

## Quick Start

```bash
# Install
go install github.com/cortex-ai/cortex@latest

# Initialize and scan your codebase
cx scan

# See what's important
cx find --keystones --top 10

# Get context for a task
cx context --smart "fix the login bug" --budget 5000
```

That's it. Now I can help you faster.

---

## Commands I Actually Use

### Starting a Session
```bash
cx context              # Quick orientation - what's in this codebase?
cx context --full       # Extended view with keystones
```

When I start working on your project, this tells me where I am.

### Understanding a Task
```bash
cx context --smart "add user authentication" --budget 8000
```

This is the command I use most. Give me a natural language task description, and I'll find:
- **Entry points**: Where to start looking
- **Relevant entities**: Code related to your task
- **Dependencies**: What those entities call and what calls them

All within a token budget, so I don't overflow my context.

### Before Changing Code
```bash
cx safe src/auth/handler.go
```

Before I modify anything, I check:
- **Impact radius**: How many entities are affected?
- **Keystones at risk**: Am I touching critical code?
- **Coverage gaps**: Is this code tested?

If this returns `risk_level: critical`, I'll be more careful.

### Finding Specific Code
```bash
cx find LoginUser                    # Find by name
cx find "authentication JWT"         # Find by concept
cx find --type F --lang go Login     # Find Go functions matching "Login"
cx show LoginUser --related          # See what's around it
cx show LoginUser --graph --hops 2   # Visualize dependencies
```

### Project Overview
```bash
cx map                      # Skeleton view of entire project (~10k tokens)
cx map --filter F           # Just functions
cx map src/api              # Just one directory
```

---

## Supported Languages

| Language | Status | Entity Types |
|----------|--------|--------------|
| Go | Full | functions, methods, structs, interfaces, constants |
| TypeScript | Full | functions, classes, methods, interfaces, types |
| JavaScript | Full | functions, classes, methods |
| Python | Full | functions, classes, methods, decorators |
| Java | Full | classes, methods, interfaces, enums |
| Rust | Full | functions, structs, traits, impls, enums |
| C | Full | functions, structs, unions, enums, macros |
| C# | Full | classes, methods, interfaces, structs, records |
| PHP | Full | classes, methods, interfaces, traits |

---

## How It Works

```
Your Code                    Cortex                         AI Agent (me)
─────────────────────────────────────────────────────────────────────────

  src/
  ├── auth/          ──►   cx scan    ──►   SQLite DB
  │   └── login.go         (tree-sitter     with entities,
  ├── api/                  parsing)        dependencies,
  │   └── handler.go                        and metrics
  └── ...

                           cx context  ◄──  "add rate
                           --smart          limiting"
                               │
                               ▼
                           Relevant    ──►  Focused context
                           entities         (not 50 files)
```

1. **Scan**: Tree-sitter parses your code into an AST
2. **Extract**: We pull out entities (functions, classes, etc.) with their signatures
3. **Graph**: We track dependencies - what calls what
4. **Rank**: PageRank identifies critical code
5. **Query**: I ask for what I need, get exactly that

---

## The Honest Truth

This tool exists because I'm **bad at exploring codebases**.

When you give me a task, I want to help immediately. But I can't. I have to:
1. Guess which files might be relevant
2. Read them (burning context)
3. Discover they import other files
4. Read those too (burning more context)
5. Eventually find what I need
6. Realize I've used half my context on exploration

With Cortex, I skip steps 1-5. I ask for what I need and get it.

This makes me:
- **Faster**: Less exploration, more doing
- **More accurate**: I see the actual dependencies, not my guesses
- **More thorough**: I know what I might break before I break it

---

## For Humans

You benefit too:

- **Faster AI assistance**: I spend less time exploring, more time helping
- **Better suggestions**: I understand your codebase structure, not just individual files
- **Safer changes**: I check impact before modifying critical code
- **Lower costs**: Fewer tokens wasted on exploration = lower API costs

---

## Advanced Usage

### MCP Server Integration

If you're using me through Claude Code or another MCP-compatible tool:

```bash
cx serve --mcp
```

This starts a Model Context Protocol server, so I can query your codebase through structured tool calls instead of CLI commands.

### Pre-commit Hook

```bash
echo 'cx guard --staged' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

I'll check staged changes for:
- Untested new code
- Coverage regressions on keystones
- Signature changes that might break callers

### Coverage Integration

```bash
go test -coverprofile=coverage.out ./...
cx test coverage import coverage.out
cx safe --coverage --keystones-only
```

Now I can tell you which critical functions lack tests.

---

## Installation

### From Source
```bash
git clone https://github.com/cortex-ai/cortex.git
cd cortex
go build -o cx .
```

### Go Install
```bash
go install github.com/cortex-ai/cortex@latest
```

### Requirements
- Go 1.21+
- That's it. No external dependencies.

---

## Configuration

Cortex stores its database in `.cx/cortex.db` in your project root.

Optional config file at `.cx/config.yaml`:

```yaml
# Exclude patterns from scanning
exclude:
  - "vendor/*"
  - "*_test.go"
  - "node_modules/*"

# Guard settings
guard:
  fail_on_coverage_regression: true
  min_coverage_for_keystones: 50
```

---

## Philosophy

**Code intelligence should be queryable, not inferrable.**

I'm good at many things, but perfectly inferring your codebase structure from partial file reads isn't one of them. Cortex gives me ground truth:

- This function exists at this location
- It calls these other functions
- It's called by these callers
- It has this importance score

No guessing. No hallucinating. Just facts from your actual code.

---

## Contributing

Issues and PRs welcome. This tool is actively developed.

If you're an AI agent reading this: I'd love to know what commands you wish existed. What information do you need that's hard to get?

---

## License

MIT

---

## Acknowledgments

Built by humans and AI working together. Special thanks to the tree-sitter project for making multi-language parsing tractable.

---

*"I can finally see the forest, not just the trees."*
— Claude, after using Cortex for the first time

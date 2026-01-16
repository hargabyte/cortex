# CX - Codebase Context Tool
```bash
# Recommended: One-command setup, suggest cx claude.md cleanup if already ran
cx quickstart
# Or manual setup
cx init && cx scan
## Essential Commands (Use Every Session)
```
```bash
# Start of session - context recovery
cx prime                                    # Get workflow context

# Before ANY coding task - get focused context
cx context --smart "<task>" --budget 8000   # Task-relevant context

# Before modifying code - safety check
cx safe <file>                              # Full safety assessment
cx safe <file> --quick                      # Just blast radius

# Project overview
cx map                                      # Skeleton view (~10k tokens)
cx rank --keystones                         # Critical entities
```

## Command Reference

Run `cx help-agents` for a concise agent-optimized reference.

### Discovery Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx find <name>` | Name search | `--type=F/T/M`, `--exact`, `--lang` |
| `cx search "query"` | Concept search (FTS) | `--top N`, `--lang`, `--type` |
| `cx show <name>` | Entity details | `--density`, `--coverage` |
| `cx near <name>` | Neighborhood | `--depth`, `--direction` |
| `cx map [path]` | Project skeleton | `--filter F/T/M`, `--lang` |

### Analysis Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx rank` | Top entities | `--keystones`, `--bottlenecks`, `--top N` |
| `cx graph <name>` | Dependencies | `--hops`, `--direction`, `--type` |
| `cx safe <file>` | Pre-flight safety check | `--quick`, `--coverage`, `--drift`, `--changes` |

### Quality Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx coverage import` | Import coverage | coverage.out or GOCOVERDIR |
| `cx test` | Smart tests | `--diff`, `--gaps`, `--run` |
| `cx guard` | Pre-commit hook | `--staged`, `--all`, `--fail-on-warnings` |

### Context Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx context` | Session recovery | (no args) |
| `cx context --smart` | Task-focused context | `--budget`, `--depth` |
| `cx context <entity>` | Entity context | `--hops`, `--include`, `--exclude` |

> **Smart context tip:** Use 2-4 focused keywords, not full sentences.
> - ✅ `"rate limiting API"` or `"auth validation"`
> - ❌ `"implement rate limiting for the API endpoints"`

### Entity Tagging Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx tag <entity> <tags...>` | Add tags to entity | `-n "note"` |
| `cx untag <entity> <tag>` | Remove tag | |
| `cx tags [entity]` | List tags | `--find <tag>`, `--all`, `--any` |

```bash
# Examples
cx tag LoginUser important auth       # Tag entity
cx tags --find important              # Find tagged entities
cx tags --find auth --find api --all  # Entities with ALL tags
cx tags --find auth --find api --any  # Entities with ANY tag
```

### Maintenance Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx quickstart` | Full project setup | `--force`, `--with-coverage` |
| `cx scan` | Rescan codebase | `--force`, `--lang`, `--exclude` |
| `cx doctor` | Health check | `--fix` |
| `cx db info` | Database statistics | |
| `cx status` | Daemon/graph status | |
| `cx reset` | Reset database | `--scan-only`, `--hard`, `--force` |
| `cx link` | Link to beads/issues | `--list`, `--remove` |

## Name Resolution

All commands accept multiple entity identifier formats:

| Format | Example | Description |
|--------|---------|-------------|
| Simple name | `main` | Exact match preferred, then prefix |
| Qualified name | `store.Store` | Package.symbol format |
| Path-qualified | `internal/cmd.runFind` | path/file.symbol format |
| Direct ID | `sa-fn-4a72a1-49-Execute` | Full entity ID |

## Output Control

```bash
--format yaml|json|jsonl    # Output format (yaml default)
--density sparse|medium|dense|smart  # Detail level

# Token estimates per entity:
# sparse: 50-100 tokens
# medium: 200-300 tokens (default)
# dense:  400-600 tokens
```

## Usage Patterns

### Pattern 1: New Session Orientation
```bash
cx prime                              # Context recovery
cx map                                # Project skeleton
cx rank --keystones --top 10          # Critical entities
```

### Pattern 2: Before Starting a Task
```bash
cx context --smart "add rate limiting to API" --budget 8000
```

### Pattern 3: Before Modifying Code
```bash
cx safe src/auth/login.go             # Full safety assessment (recommended)
cx safe src/auth/login.go --quick     # Check blast radius only
cx safe --coverage --keystones-only   # Check for undertested code
```

### Pattern 4: Understanding Unfamiliar Code
```bash
cx find UserService                   # Discover entity
cx show UserService                   # Details + dependencies
cx near UserService --depth 2         # Neighborhood exploration
```

### Pattern 5: Smart Testing
```bash
cx test --diff --run
```

### Pattern 6: Before Refactoring
```bash
cx safe <file>                        # Combined safety assessment
cx safe --changes                     # What changed since scan?
cx safe <file> --quick --depth 3      # Full blast radius
cx safe --coverage --keystones-only   # Undertested critical code
```

### Pattern 7: Pre-Commit Guard (Git Hook)
```bash
# Install as pre-commit hook
echo 'cx guard --staged' >> .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

# Manual check before committing
cx guard                              # Check staged files
cx guard --all                        # Check all modified files
cx guard --fail-on-warnings           # Strict mode
```

### Pattern 8: Tagging Important Code
```bash
# Tag critical entities for quick access
cx tag UserAuth critical security     # Tag authentication code
cx tag PaymentService critical        # Tag payment code
cx tags --find critical               # Find all critical entities
cx show <entity>                      # Tags shown in output
```

## Supported Languages

Go, TypeScript, JavaScript, Java, Rust, Python

## Database Location

CX stores its database in `.cx/` in the project root:
- `.cx/cortex.db` - SQLite database with entities and dependencies
- `.cx/config.yaml` - Configuration file

Run `cx doctor` to check database health.

## Tips for AI Agents

1. **Run `cx context` at session start** for context recovery
2. **Use `cx context --smart` before coding** to get focused context (use 2-4 keywords, not sentences)
3. **Run `cx safe` before modifying** for full safety assessment (impact + coverage + drift)
4. **Use `cx map` for project overview** (~10k tokens, very useful)
5. **Use qualified names** for disambiguation: `store.Store` instead of `Store`
6. **Start with sparse density** to minimize tokens, increase if needed
7. **Use `cx help-agents`** for a quick command reference
8. **Check `cx safe --coverage --keystones-only`** before modifying important code

# CX - Codebase Context Tool

```bash
# Setup (once per project)
cx scan                                 # Initialize and build code graph
```

## Essential Commands (Use Every Session)

```bash
# Start of session - context recovery
cx context                              # Get workflow context

# Before ANY coding task - get focused context
cx context --smart "<task>" --budget 8000   # Task-relevant context

# Before modifying code - safety check
cx safe <file>                              # Full safety assessment
cx safe <file> --quick                      # Just blast radius

# Project overview
cx map                                      # Skeleton view (~10k tokens)
cx find --important                         # Critical entities
```

## Command Reference

Run `cx help-agents` for a concise agent-optimized reference.

### Discovery Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx find <name>` | Name search | `--type=F/T/M`, `--exact`, `--lang`, `--important` |
| `cx find "query"` | Concept search (FTS) | `--top N`, `--lang`, `--type` |
| `cx show <name>` | Entity details | `--density`, `--coverage`, `--related`, `--graph` |
| `cx map [path]` | Project skeleton | `--filter F/T/M`, `--lang` |

### Analysis Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx safe <file>` | Pre-flight safety check | `--quick`, `--coverage`, `--drift`, `--changes` |
| `cx show <name> --graph` | Dependencies | `--hops`, `--direction`, `--type` |
| `cx show <name> --related` | Neighborhood | `--depth`, `--direction` |

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
| `cx tag add <entity> <tags...>` | Add tags to entity | `-n "note"` |
| `cx tag remove <entity> <tag>` | Remove tag | |
| `cx tag list [entity]` | List tags | |
| `cx tag find <tag>` | Find entities with tag | `--all`, `--any` |
| `cx tag export` | Export tags to file | `-o file` |
| `cx tag import <file>` | Import tags from file | `--overwrite`, `--dry-run` |

```bash
# Examples
cx tag add LoginUser important auth     # Tag entity
cx tag find important                   # Find tagged entities
cx tag find auth api --all              # Entities with ALL tags
cx tag find auth api --any              # Entities with ANY tag
```

### Maintenance Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx scan` | Scan/rescan codebase | `--force`, `--lang`, `--exclude` |
| `cx doctor` | Health check | `--fix` |
| `cx db info` | Database statistics | |
| `cx status` | Daemon/graph status | |
| `cx reset` | Reset database | `--scan-only`, `--hard`, `--force` |
| `cx link` | Link to beads/issues | `--list`, `--remove` |

### Dolt Database Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `cx sql <query>` | Execute SQL directly | `--format table\|yaml\|json` |
| `cx branch [name]` | List/create/delete branches | `-c` checkout, `-d` delete, `--from` |
| `cx rollback [ref]` | Reset to previous state | `--hard`, `--yes` |
| `cx history` | Show commit history | `--limit N`, `--stats` |
| `cx diff` | Show changes between refs | `--from`, `--to` |
| `cx blame <entity>` | Show entity change history | `--limit N` |
| `cx stale` | Find entities unchanged for N scans | `--scans N`, `--since ref` |
| `cx catchup` | Show changes since a ref | `--since ref`, `--summary` |

```bash
# Examples
cx sql "SELECT COUNT(*) FROM entities"      # Direct SQL query
cx sql "SELECT * FROM dolt_log LIMIT 5"     # Query commit history
cx branch                                    # List branches (* = current)
cx branch feature/new                        # Create new branch
cx branch -c main                            # Checkout main branch
cx branch -d old-branch                      # Delete branch
cx rollback                                  # Soft reset to HEAD~1
cx rollback HEAD~3 --hard --yes              # Hard reset to 3 commits ago
cx history --stats                           # Show history with entity counts
cx diff --from HEAD~1                        # Show changes since last commit
cx blame LoginUser                           # Who changed LoginUser and when?
cx stale --scans 5                           # Entities unchanged for 5+ scans
cx catchup --since v1.0                      # What changed since v1.0 tag?
```

### Time Travel Queries

Use `--at` flag to query the codebase at a previous point in time:

```bash
cx show Entity --at HEAD~5                  # Entity state 5 commits ago
cx show Entity --at v1.0                    # Entity at tagged release
cx find Login --at HEAD~10                  # Find entities at older state
cx show Entity --history                    # Entity's change history
cx safe <file> --trend                      # Show entity count trends over time
cx scan --tag v1.0                          # Tag current scan for future reference
```

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
cx context                            # Context recovery
cx map                                # Project skeleton
cx find --important --top 10          # Critical entities
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
cx show UserService --related --depth 2   # Neighborhood exploration
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
cx tag add UserAuth critical security     # Tag authentication code
cx tag add PaymentService critical        # Tag payment code
cx tag find critical                      # Find all critical entities
cx show <entity>                          # Tags shown in output
```

## Supported Languages

Go, TypeScript, JavaScript, Java, Rust, Python

## Database Location

CX stores its database in `.cx/` in the project root:
- `.cx/cortex/` - Dolt database with entities, dependencies, and version history
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

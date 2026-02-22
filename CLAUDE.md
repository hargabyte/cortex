# CX — Codebase Context Tool

## Setup
```bash
cx scan                           # One-time: build code graph
```

## Five Commands

```bash
cx [target]          # Understand code (auto-detects entity/file)
cx find <pattern>    # Search and discover entities
cx check [file]      # Quality gate (safety, guard, tests)
cx scan              # Build or rebuild the code graph
cx call <tool> '{}'  # Machine gateway (MCP tools, pipe mode)
```

## Everyday Workflow

```bash
# Start of session
cx                               # Session recovery context
cx --smart "your task" --budget 8000  # Task-focused context

# Understand code
cx LoginUser                     # Show entity details
cx src/auth/login.go             # Safety check before editing
cx --map                         # Project skeleton overview
cx --trace LoginUser             # Trace call chains (callers)
cx --blame LoginUser             # Entity commit history
cx --diff                        # Context for uncommitted changes

# Search
cx find LoginUser                # Find entities by name
cx find "auth validation"        # Concept search (FTS)
cx find --keystones              # Critical entities
cx find --dead                   # Find dead code
cx find --dead --tier 2          # Include probable dead code

# Quality checks
cx check src/auth/login.go       # Safety check on file
cx check                         # Pre-commit guard (staged files)
cx check --test                  # Smart test selection
cx check --test --gaps           # Coverage gap analysis
cx check --test --threshold 50   # Custom gap threshold

# Administrative (run 'cx admin' to see all)
cx admin tag list                # List all tags
cx admin db info                 # Database stats
cx admin doctor                  # Health check
```

## cx call (Machine Gateway)
```bash
cx call --list                   # See all 14 tools with parameter schemas
cx call <tool> '{"param":"val"}' # Call any tool
echo '{"tool":"cx_find","args":{"pattern":"Store"}}' | cx call --pipe
```

## Tips
- Use 2-4 keywords for `--smart`, not full sentences
- Use qualified names for disambiguation: `store.Store` not `Store`
- For advanced trace/blame flags, use the full command: `cx trace Foo --callees --depth 3`
- Tool names accept shorthand: `find` = `cx_find`
- Old commands still work: `cx show`, `cx safe`, `cx guard`, etc.

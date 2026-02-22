# CX — Codebase Context Tool

## Setup
```bash
cx scan                                    # One-time: build code graph
```

## Usage — `cx call` (preferred)
```bash
cx call --list                             # See all tools and parameters
cx call <tool> '{"param":"value"}'         # Call any tool
```

## Key Tools
```bash
cx call context '{"smart":"your task","budget":8000}'   # Before coding
cx call safe '{"target":"file.go"}'                      # Before editing
cx call find '{"pattern":"LoginUser"}'                   # Find code
cx call show '{"name":"Store","density":"dense"}'        # Entity details
cx call map '{}'                                         # Project overview
```

## Pipe Mode (multiple calls, one process)
```bash
echo '{"tool":"cx_find","args":{"pattern":"Store"}}' | cx call --pipe
```

## CLI Commands (for operations not yet in `cx call`)
```bash
cx context                     # Session recovery (no args)
cx guard --staged              # Pre-commit check
cx test --diff --run           # Smart test selection
cx tag add Entity important    # Tag entities
cx scan                        # Rescan codebase
```

## Tips
- Run `cx call --list` to see all tools with parameter schemas
- Use `cx call context '{"smart":"..."}' ` before any coding task (2-4 keywords, not sentences)
- Use `cx call safe` before modifying files
- Tool names accept shorthand: `find` = `cx_find`
- Use qualified names for disambiguation: `store.Store` not `Store`

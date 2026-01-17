# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## CX 2.0 - YAML Output Format

All CX commands output YAML format by default. The migration from CGF to YAML is complete. Here's what you need to know:

### Global Flags (Available on All Commands)

```bash
--format string    # Output format: yaml (default) | json | cgf (deprecated)
--density string   # Output detail level (default: medium)
```

### Density Levels

- **sparse**: Minimal output (type + location only) - ~50-100 tokens per entity
- **medium**: Balanced detail with signatures and basic dependencies - ~200-300 tokens per entity
- **dense**: Full detail including metrics, hashes, timestamps - ~400-600 tokens per entity
- **smart**: Importance-based adaptive density (keystones get dense, leaves get sparse)

### Example Output Formats

#### cx find (with sparse density)
```yaml
results:
  LoginUser:
    type: function
    location: internal/auth/login.go:45-89
  UserService:
    type: struct
    location: internal/user/service.go:12-45
count: 2
```

#### cx show (with medium density, default)
```yaml
LoginUser:
  type: function
  location: internal/auth/login.go:45-89
  signature: "(email: string, password: string) -> (*User, error)"
  visibility: public
  dependencies:
    calls: [ValidateEmail, HashPassword, userRepo.Create]
    called_by:
      - name: HandleLogin
      - name: HandleRegister
    uses_types: [User, AuthError]
```

#### cx find --keystones (showing top entities by importance)
```yaml
results:
  HandleRequest:
    type: function
    location: internal/server/handler.go:78-134
    visibility: public
    metrics:
      pagerank: 0.89
      in_degree: 47
      out_degree: 12
      importance: keystone
  DatabaseConnect:
    type: function
    location: internal/db/connection.go:23-56
    metrics:
      pagerank: 0.76
      in_degree: 38
      out_degree: 5
      importance: keystone
count: 20
```

### Common Usage Examples

```bash
# Find with default YAML medium density
cx find LoginUser

# Find with minimal output for token efficiency
cx find LoginUser --density=sparse

# Show with full details
cx show sa-fn-a7f9b2-LoginUser --density=dense

# Get JSON output instead
cx find LoginUser --format=json

# Graph analysis with medium density
cx graph LoginUser --density=medium

# Find keystones (most important entities)
cx find --keystones

# Safety check before modifying
cx safe src/auth/jwt.go

# Get context for a task
cx context bd-a7c --max-tokens=8000
```

### Migration Notes

- CGF format is deprecated but still available with `--format=cgf`
- Default output is now YAML (human-readable and AI-friendly)
- All density levels work across all commands
- JSON output available with `--format=json` for machine parsing

For detailed format examples, see `OUTPUT_EXAMPLES.md`.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds


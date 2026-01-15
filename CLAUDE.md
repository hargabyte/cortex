# CX - Codebase Context Tool

CX is a code analysis tool that builds and queries a graph of code entities. It helps developers and AI agents understand code structure, find relevant context, and analyze dependencies.

## Quick Start

```bash
# Initialize and scan a codebase
cx init
cx scan

# Find entities by name
cx find LoginUser

# Show entity details
cx show main

# Visualize dependencies
cx graph Execute --hops 2
```

## Name Resolution

All commands that accept entity identifiers support multiple formats:

| Format | Example | Description |
|--------|---------|-------------|
| Simple name | `main` | Matches by name (exact preferred, then prefix) |
| Qualified name | `store.Store` | Package.symbol format |
| Path-qualified | `internal/cmd.runFind` | Path/file.symbol format |
| Direct ID | `sa-fn-4a72a1-49-Execute` | Full entity ID |

**Examples:**
```bash
cx show main                    # Find entity named "main"
cx show store.Store             # Find Store in store package
cx graph Execute --hops 1       # Graph for Execute function
cx show sa-fn-abc123-Entity     # Direct ID lookup
```

**Ambiguous names:** If multiple entities match, you'll see a helpful error with suggestions:
```
Error: multiple entities match "Store":
  - store (import) at internal/cmd/context.go:14
  - Store (struct) at internal/store/db.go:17-20
  ... and 4 more

Use a more specific name or the full entity ID
```

## Output Format

All commands output **YAML by default**. Use flags to customize:

```bash
--format yaml    # Human-readable YAML (default)
--format json    # Machine-parseable JSON
--format cgf     # Deprecated compact format (shows warning)

--density sparse # Type + location only (~50-100 tokens)
--density medium # Add signature, dependencies (~200-300 tokens, default)
--density dense  # Full details with metrics/hashes (~400-600 tokens)
--density smart  # Adaptive based on entity importance
```

## Commands

### cx find - Search for Entities by Name

```bash
cx find <name>              # Prefix match by name
cx find --exact <name>      # Exact match only
cx find --type=F <name>     # Functions only (F|T|M|C|E)
cx find --file=auth <name>  # Filter by file path
```

### cx search - Full-Text Search by Concept

Search for code by concept, not just symbol name. Uses FTS5 full-text search
to find code based on function bodies, doc comments, and file paths.

```bash
cx search "auth logic"           # Find authentication-related code
cx search "rate limit"           # Find rate limiting code
cx search LoginUser              # Exact name match (boosted)
cx search "TODO fix"             # Find TODOs in comments/code
cx search --lang go "config"     # Filter by language
cx search --type function "parse"  # Filter by entity type
cx search --top 20 "database"    # Get more results
cx search --density dense "auth" # Include relevance scores
```

Results are ranked by a combination of:
- FTS5 BM25 text matching score
- PageRank importance (if metrics computed via `cx rank`)
- Exact name match boosting

### cx show - Entity Details

```bash
cx show <name-or-id>                    # Show entity details
cx show <name> --density=dense          # Include metrics and hashes
cx show <name> --include-metrics        # Add metrics to medium density
cx show <name> --coverage               # Include test coverage info
```

### cx graph - Dependency Graph

```bash
cx graph <name-or-id>                   # Both directions, 2 hops
cx graph <name> --direction=out         # What this entity calls
cx graph <name> --direction=in          # What calls this entity
cx graph <name> --hops=3                # Increase traversal depth
cx graph <name> --type=calls            # Only call relationships
```

### cx rank - Find Important Entities

```bash
cx rank                     # Top 20 by PageRank
cx rank --top 50            # Top 50
cx rank --keystones         # High PageRank entities
cx rank --bottlenecks       # High betweenness centrality
cx rank --leaves            # Entities with no dependents
```

### cx impact - Change Analysis

```bash
cx impact <file>            # Analyze impact of changes to a file
cx impact <file> --depth=3  # Traversal depth for impact analysis
```

### cx context - AI Context Export

```bash
cx context <entity>                 # Gather context around entity
cx context <entity> --max-tokens=2000   # Limit token budget
cx context <entity> --hops=2            # Expansion depth
```

### cx verify - Check Staleness

```bash
cx verify                   # Check all entities for drift
cx verify --fix             # Update hashes for drifted entities
cx verify --strict          # Exit with error if any drift found
```

### cx coverage - Test Coverage Integration

Import and analyze Go coverage data to understand test coverage per entity.

```bash
# Generate coverage data
go test -coverprofile=coverage.out ./...

# Import into cx
cx coverage import coverage.out         # Import coverage data
cx coverage status                      # Show coverage statistics
cx coverage status --format json        # JSON output
```

### cx gaps - Coverage Gap Detection

Find dangerous combinations of high importance and low coverage.

```bash
cx gaps                     # All gaps grouped by risk
cx gaps --keystones-only    # Only keystones with gaps
cx gaps --threshold 50      # Only <50% coverage
cx gaps --create-tasks      # Generate bd create commands for gaps
cx gaps --format json       # JSON output
```

Risk categories:
- **CRITICAL**: Keystones (PageRank >= 0.30) with <25% coverage
- **HIGH**: Keystones with 25-50% coverage
- **MEDIUM**: Normal entities with <25% coverage
- **LOW**: All other gaps

### cx test-impact - Smart Test Selection

Given a change, identify exactly which tests need to run.

```bash
cx test-impact internal/auth/login.go   # Tests for this file
cx test-impact --diff                   # Tests for uncommitted changes
cx test-impact --commit HEAD~1          # Tests for specific commit
cx test-impact --output-command         # Just output go test command
cx test-impact --depth 3                # Increase indirect test depth
```

Output includes:
- **direct**: Tests that directly call changed entities
- **indirect**: Tests that call callers of changed entities
- **integration**: Tests in integration/e2e directories
- **command**: Ready-to-run `go test -run` command

## Example Outputs

### cx find (sparse density)
```yaml
results:
  main:
    type: function
    location: cmd/cx/main.go:8-10
count: 1
```

### cx show (medium density)
```yaml
Execute:
  type: function
  location: internal/cmd/root.go:49-54
  visibility: public
  dependencies:
    calls:
      - sa-imp-aad362-8-exec
      - sa-fn-df2400-39-Error
    called_by:
      - name: sa-fn-22851f-8-main
```

### cx graph
```yaml
graph:
  root: Execute
  direction: both
  depth: 1
nodes:
  Execute:
    type: function
    location: internal/cmd/root.go:49-54
    depth: 0
  main:
    type: function
    location: cmd/cx/main.go:8-10
    depth: 1
edges:
  - [main, Execute, calls]
```

### cx rank
```yaml
results:
  Close:
    type: method
    location: internal/store/db.go:67-72
    visibility: public
    metrics:
      pagerank: 0.025
      in_degree: 127
      importance: normal
count: 20
```

### cx search (dense density)
```yaml
query: pagerank
results:
  ComputePageRank:
    type: function
    location: internal/metrics/pagerank.go:48-51
    visibility: public
    dependencies:
      calls:
        - ComputePageRankWithInfo
    metrics: {}
  PageRankConfig:
    type: struct
    location: internal/metrics/pagerank.go:6-19
    visibility: public
    metrics: {}
count: 2
scores:
  ComputePageRank:
    fts_score: 1.53
    pagerank: 0.05
    combined_score: 0.18
  PageRankConfig:
    fts_score: 1.51
    pagerank: 0.02
    combined_score: 0.17
```

## Tips for AI Agents

1. **Use qualified names** for disambiguation: `store.Store` instead of `Store`
2. **Start with sparse density** to minimize tokens, increase if needed
3. **Use `cx find` first** to discover entity names, then `cx show` or `cx graph`
4. **Check `cx rank --keystones`** to find the most important entities
5. **Use JSON format** for programmatic parsing: `--format=json`
6. **Check coverage gaps** with `cx gaps --keystones-only` before modifying important code
7. **Run smart tests** with `cx test-impact --diff` to test only affected code
8. **Use `cx context --smart "task description"`** to get task-relevant context

## Database Location

CX stores its database in `.cx/` in the project root:
- `.cx/cx.db` - SQLite database with entities and dependencies
- `.cx/config.yaml` - Configuration file

Run `cx doctor` to check database health.

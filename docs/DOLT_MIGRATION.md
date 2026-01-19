# Cortex Migration to Dolt

## Overview

This document outlines the migration of Cortex from SQLite to [Dolt](https://github.com/dolthub/dolt) - a SQL database with Git-like version control. This migration enables historical queries, branch-aware analysis, and future integration with Beads once it also migrates to Dolt.

## Why Dolt?

### Current Limitations (SQLite)

- **No history**: Each `cx scan` overwrites previous state
- **No branch awareness**: Can't compare dependency graphs across git branches
- **No blame/audit**: Can't track when/why entities changed
- **No diff**: Can't see what changed between scans
- **No distributed sync**: Database is local-only

### What Dolt Provides

| Feature | Benefit for Cortex |
|---------|-------------------|
| `dolt diff` | See what entities/deps changed between scans |
| `dolt log` | History of entity graph changes |
| `dolt blame` | Track when/why specific entities changed |
| `dolt branch` | Scan different git branches, compare graphs |
| `AS OF` queries | Time-travel: query graph at any point in history |
| `dolt push/pull` | Share Cortex DB across team |

### Performance

Dolt vs MySQL benchmarks (Dolt 1.80.1):
- Reads: ~5% slower overall, but table/index scans 34-36% faster
- Writes: ~10% faster
- For Cortex use case (local, single-user): Performance is excellent

## Architecture

### Current (SQLite)

```
cx scan → Parse with tree-sitter → Write to SQLite
                                        ↓
                                   .cx/cortex.db
                                   (single file, no history)
```

### Future (Dolt)

```
cx scan → Parse with tree-sitter → Write to Dolt → dolt commit
                                        ↓
                                   .cx/cortex/
                                   (versioned database)
```

## Schema Migration

### Current SQLite Schema (Simplified)

```sql
-- Entities table
CREATE TABLE entities (
    id TEXT PRIMARY KEY,
    name TEXT,
    qualified_name TEXT,
    type TEXT,
    file TEXT,
    start_line INTEGER,
    end_line INTEGER,
    importance TEXT,
    language TEXT,
    signature TEXT,
    doc TEXT
);

-- Dependencies table
CREATE TABLE dependencies (
    id INTEGER PRIMARY KEY,
    caller_id TEXT REFERENCES entities(id),
    callee_id TEXT REFERENCES entities(id),
    dep_type TEXT,
    location TEXT
);

-- Tags table
CREATE TABLE entity_tags (
    entity_id TEXT REFERENCES entities(id),
    tag TEXT,
    note TEXT,
    created_at TIMESTAMP
);

-- Coverage table
CREATE TABLE coverage (
    entity_id TEXT REFERENCES entities(id),
    covered_lines INTEGER,
    total_lines INTEGER,
    percentage REAL
);
```

### Dolt Schema (Same structure, stored in Dolt)

The schema remains identical - Dolt is MySQL-compatible. The difference is storage and versioning.

```sql
-- Initialize Dolt repository
dolt init

-- Create tables (same DDL as SQLite, with MySQL syntax adjustments)
CREATE TABLE entities (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    qualified_name VARCHAR(512),
    type VARCHAR(50) NOT NULL,
    file VARCHAR(1024) NOT NULL,
    start_line INT,
    end_line INT,
    importance VARCHAR(50),
    language VARCHAR(50),
    signature TEXT,
    doc TEXT,
    INDEX idx_name (name),
    INDEX idx_file (file),
    INDEX idx_type (type),
    INDEX idx_importance (importance)
);

CREATE TABLE dependencies (
    id INT AUTO_INCREMENT PRIMARY KEY,
    caller_id VARCHAR(255) NOT NULL,
    callee_id VARCHAR(255) NOT NULL,
    dep_type VARCHAR(50),
    location VARCHAR(1024),
    FOREIGN KEY (caller_id) REFERENCES entities(id) ON DELETE CASCADE,
    FOREIGN KEY (callee_id) REFERENCES entities(id) ON DELETE CASCADE,
    INDEX idx_caller (caller_id),
    INDEX idx_callee (callee_id)
);

CREATE TABLE entity_tags (
    entity_id VARCHAR(255) NOT NULL,
    tag VARCHAR(255) NOT NULL,
    note TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (entity_id, tag),
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

CREATE TABLE coverage (
    entity_id VARCHAR(255) PRIMARY KEY,
    covered_lines INT,
    total_lines INT,
    percentage DECIMAL(5,2),
    FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
);

-- Metadata table for scan tracking
CREATE TABLE scan_metadata (
    id INT AUTO_INCREMENT PRIMARY KEY,
    scan_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    git_commit VARCHAR(40),
    git_branch VARCHAR(255),
    files_scanned INT,
    entities_found INT,
    dependencies_found INT
);
```

## Implementation Plan

### Phase 1: Dolt Backend Support

**Goal**: Add Dolt as an alternative storage backend alongside SQLite.

#### 1.1 Database Abstraction Layer

Create interface to support both SQLite and Dolt:

```go
type Storage interface {
    // Entity operations
    UpsertEntity(entity *Entity) error
    GetEntity(id string) (*Entity, error)
    FindEntities(query EntityQuery) ([]*Entity, error)
    DeleteEntity(id string) error

    // Dependency operations
    AddDependency(dep *Dependency) error
    GetDependencies(entityID string, direction Direction) ([]*Dependency, error)

    // Tag operations
    AddTag(entityID, tag, note string) error
    RemoveTag(entityID, tag string) error
    GetTags(entityID string) ([]Tag, error)

    // Coverage operations
    SetCoverage(entityID string, coverage *Coverage) error
    GetCoverage(entityID string) (*Coverage, error)

    // Transaction support
    Begin() (Transaction, error)

    // Dolt-specific (no-op for SQLite)
    Commit(message string) error
    GetHistory(table string, limit int) ([]CommitInfo, error)
}
```

#### 1.2 Dolt Storage Implementation

```go
type DoltStorage struct {
    db      *sql.DB
    repoDir string
}

func NewDoltStorage(dir string) (*DoltStorage, error) {
    // Initialize or open Dolt repo
    repoDir := filepath.Join(dir, ".cx", "cortex")

    if !doltRepoExists(repoDir) {
        if err := initDoltRepo(repoDir); err != nil {
            return nil, err
        }
    }

    // Connect via dolt sql-server or embedded
    db, err := sql.Open("mysql", doltConnectionString(repoDir))
    if err != nil {
        return nil, err
    }

    return &DoltStorage{db: db, repoDir: repoDir}, nil
}

func (d *DoltStorage) Commit(message string) error {
    // Stage all changes
    if _, err := d.db.Exec("CALL dolt_add('-A')"); err != nil {
        return err
    }

    // Commit
    _, err := d.db.Exec("CALL dolt_commit('-m', ?)", message)
    return err
}
```

#### 1.3 Configuration

Add to `.cx/config.yaml`:

```yaml
storage:
  backend: dolt  # or "sqlite" for backward compatibility

dolt:
  auto_commit: true  # Commit after each scan
  commit_message_template: "cx scan: {{.FilesScanned}} files, {{.EntitiesFound}} entities"
```

### Phase 2: Scan Integration

**Goal**: `cx scan` writes to Dolt and commits automatically.

#### 2.1 Modified Scan Flow

```go
func (s *Scanner) Scan(ctx context.Context) error {
    // Begin transaction
    tx, err := s.storage.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Clear existing entities for rescanned files
    // ... existing scan logic ...

    // Parse files, extract entities, resolve dependencies
    // ... existing scan logic ...

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return err
    }

    // If Dolt backend, create version commit
    if doltStorage, ok := s.storage.(*DoltStorage); ok {
        metadata := ScanMetadata{
            GitCommit:  getCurrentGitCommit(),
            GitBranch:  getCurrentGitBranch(),
            FilesScanned: len(files),
            EntitiesFound: entityCount,
            DependenciesFound: depCount,
        }

        msg := fmt.Sprintf("cx scan: %d files, %d entities, %d deps [%s@%s]",
            metadata.FilesScanned,
            metadata.EntitiesFound,
            metadata.DependenciesFound,
            metadata.GitBranch,
            metadata.GitCommit[:7],
        )

        if err := doltStorage.Commit(msg); err != nil {
            return err
        }
    }

    return nil
}
```

### Phase 3: New Commands

**Goal**: Expose Dolt capabilities through `cx` commands.

#### 3.1 `cx history` - View Scan History

```bash
cx history [--limit N] [--table TABLE]

# Examples:
cx history                    # Show recent scans
cx history --limit 20         # Show last 20 scans
cx history --table entities   # Show entity table changes
```

Implementation: Wraps `dolt log`

#### 3.2 `cx diff` - Compare Scans

```bash
cx diff [REF1] [REF2] [--table TABLE] [--entity NAME]

# Examples:
cx diff                       # Show uncommitted changes
cx diff HEAD~1                # Compare to previous scan
cx diff HEAD~5 HEAD           # Compare two points
cx diff main feature-branch   # Compare branches
cx diff --entity LoginUser    # Filter to specific entity
```

Implementation: Wraps `dolt diff`

#### 3.3 `cx blame` - Entity Change Attribution

```bash
cx blame <entity> [--table TABLE]

# Examples:
cx blame LoginUser            # Who/when changed this entity
cx blame LoginUser --deps     # Include dependency changes
```

Implementation: Wraps `dolt blame`

#### 3.4 `cx at` - Time-Travel Queries

```bash
cx show <entity> --at <REF>
cx find <pattern> --at <REF>

# Examples:
cx show LoginUser --at HEAD~5      # Entity 5 scans ago
cx show LoginUser --at main        # Entity on main branch
cx find "Auth*" --at 2024-01-15    # Entities as of date
```

Implementation: Uses `AS OF` SQL clause

#### 3.5 `cx branch` - Branch Management

```bash
cx branch [NAME] [--list] [--delete]

# Examples:
cx branch                     # List branches
cx branch feature-scan        # Create branch
cx branch --delete old-scan   # Delete branch
```

Implementation: Wraps `dolt branch`

### Phase 4: Branch-Aware Scanning

**Goal**: Automatically track which git branch was scanned.

#### 4.1 Scan Metadata

Each scan records:
- Git commit hash
- Git branch name
- Timestamp
- File/entity/dependency counts

#### 4.2 Branch Comparison Workflow

```bash
# Scan main branch
git checkout main
cx scan
# Dolt commit: "cx scan: main@abc1234"

# Scan feature branch
git checkout feature-auth
cx scan
# Dolt commit: "cx scan: feature-auth@def5678"

# Compare dependency graphs
cx diff main feature-auth
# Shows entities/deps that differ between branches
```

### Phase 5: Migration Tooling

**Goal**: Migrate existing SQLite databases to Dolt.

#### 5.1 `cx migrate` Command

```bash
cx migrate [--from sqlite] [--to dolt] [--backup]

# Migrates .cx/cortex.db to .cx/cortex/ (Dolt repo)
```

#### 5.2 Migration Steps

1. Read all data from SQLite
2. Initialize Dolt repo
3. Create schema
4. Insert all data
5. Create initial commit
6. Verify integrity
7. Update config to use Dolt backend
8. Optionally backup/remove SQLite file

## CLI Integration with Dolt

For operations not wrapped by `cx`, users can use `dolt` CLI directly:

```bash
# Navigate to Cortex Dolt repo
cd .cx/cortex

# Use any dolt command
dolt log --oneline
dolt diff HEAD~1 entities
dolt blame entities --where "name = 'LoginUser'"
dolt sql -q "SELECT * FROM entities AS OF 'HEAD~5' WHERE importance = 'keystone'"
```

## Future: Beads Integration

Once Beads migrates to Dolt, both tools can share a database:

```
.cx/
  cortex/           # Cortex Dolt repo (current migration)

.beads/
  beads/            # Beads Dolt repo (Steve's migration)

# Future: Shared database
.project-db/
  entities          # Cortex table
  dependencies      # Cortex table
  issues            # Beads table
  issue_deps        # Beads table
  issue_entities    # Bridge table (NEW)
```

Bridge table enables:
- `cx issues <entity>` - Find issues linked to code
- `bd code <issue>` - Find code linked to issues
- `cx safe --beads` - Blast radius with issue awareness

This integration is deferred until Beads completes its Dolt migration.

## Configuration Reference

### .cx/config.yaml

```yaml
# Storage backend: "sqlite" (default) or "dolt"
storage:
  backend: dolt

# Dolt-specific settings
dolt:
  # Automatically commit after each scan
  auto_commit: true

  # Commit message template (Go template syntax)
  # Available: .FilesScanned, .EntitiesFound, .DependenciesFound,
  #            .GitCommit, .GitBranch, .Timestamp
  commit_message_template: |
    cx scan: {{.EntitiesFound}} entities, {{.DependenciesFound}} deps
    Branch: {{.GitBranch}}
    Commit: {{.GitCommit}}

  # Remote for push/pull (optional)
  remote: origin
  remote_url: ""  # e.g., https://dolthub.com/user/repo

  # Auto-push after scan (requires remote)
  auto_push: false
```

## Rollback Plan

If issues arise, rollback to SQLite:

1. Export data: `dolt dump > backup.sql`
2. Change config: `storage.backend: sqlite`
3. Import to SQLite if needed
4. Cortex will use SQLite on next operation

## Open Questions

1. **Embedded vs Server**: Should Cortex use Dolt embedded or start a sql-server?
   - Embedded: Simpler, single process
   - Server: Better for concurrent access, GasTown integration

2. **Repo Location**: `.cx/cortex/` or separate `.dolt/` directory?

3. **Auto-commit granularity**: Commit per scan, or allow batching?

4. **Branch strategy**: Mirror git branches automatically, or independent?

## References

- [Dolt GitHub](https://github.com/dolthub/dolt)
- [Dolt Documentation](https://docs.dolthub.com/)
- [Dolt SQL Reference](https://docs.dolthub.com/sql-reference/version-control/dolt-sql-procedures)
- [Dolt Performance Benchmarks](https://docs.dolthub.com/sql-reference/benchmarks/latency)

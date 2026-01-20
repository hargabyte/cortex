# Specification: Health Report Dead Code Fix

> **Epic**: cortex-wpu | **Priority**: P1 | **Status**: Spec Complete

Fix false positives in health report dead code detection by filtering entity types not tracked in the call graph.

---

## Goal

Reduce health report dead code candidates from 641 false positives to ~60-100 actual candidates by implementing an entity type whitelist that matches call graph tracking capabilities. Group results by module for actionable output.

---

## User Stories

- As a developer, I want the health report to show only actual dead code so I can confidently remove unused functions
- As a tech lead, I want dead code grouped by module so I can prioritize cleanup by area
- As a CI pipeline, I want zero false positives so I can fail builds on legitimate dead code

---

## Problem Analysis

### Root Cause

The `findDeadCodeCandidates` function in [gather.go:711-756](internal/report/gather.go#L711-L756) queries ALL entities without filtering by type:

```go
entities, err := g.store.QueryEntities(store.EntityFilter{
    Status: "active",
    Limit:  1000,
})
```

### Current False Positive Distribution

| Entity Type | Count | Tracked in Call Graph? | Should Flag? |
|-------------|-------|----------------------|--------------|
| `variable` | 1,027 | No | **No** |
| `method` | 915 | Yes | Yes |
| `function` | 719 | Yes | Yes |
| `import` | 551 | No | **No** |
| `type` | 284 | Yes | Yes |
| `constant` | 112 | No | **No** |

**Result**: 641 false positives from imports (551) + estimated variables/constants flagged.

### Current Insufficient Exclusions

```go
// These exclusions exist but don't catch imports/variables
if e.Name == "main" || e.Name == "init" || e.Visibility == "pub" {
    continue
}
```

- Imports have **empty** visibility (not `pub`)
- Variables are mostly `priv`
- Constants are mixed

---

## Molecule Architecture

### Dependency Overview

```
┌─────────────────────────────────────────────────────────────┐
│  T1: Implement Entity Type Whitelist Filter                 │
│  File: internal/report/gather.go                            │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  T2: Add DeadCodeGroup Schema + Grouping Logic              │
│  Files: internal/report/schema.go, gather.go                │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  T3: Cross-Language Integration Testing                     │
│  Test repos: Go, TS, Python, Java, Rust, C#, C++, PHP, etc. │
└─────────────────────────────────────────────────────────────┘
```

### Work Streams

#### Stream 1: Core Fix (T1)
**Agent Type:** Backend Polecat
**Dependencies:** None
**Outputs:** Working whitelist filter, reduced false positives

#### Stream 2: Output Enhancement (T2)
**Agent Type:** Backend Polecat
**Dependencies:** T1 complete (needs working filter to group)
**Outputs:** Grouped output by module, new schema types

#### Stream 3: Validation (T3)
**Agent Type:** Test Polecat
**Dependencies:** T1 + T2 complete
**Outputs:** Verified behavior across all languages

### Critical Path

```
T1 → T2 → T3
```

Single sequential chain - this is appropriate for a focused fix where each task builds directly on the previous.

### Parallelization Analysis

**Cannot parallelize because:**
- T2 needs T1's output to group correctly
- T3 needs both T1 and T2 to verify complete behavior
- Total: 3 tasks, 3 sequential steps

**This is acceptable** - small focused feature, parallelization overhead not justified.

---

## Specific Requirements

### T1: Entity Type Whitelist Filter

#### Implementation

Add whitelist to `findDeadCodeCandidates` in `internal/report/gather.go`:

```go
// deadCodeTypes are entity types tracked in the call graph.
// Only these can be detected as "dead code" (no callers).
var deadCodeTypes = map[string]bool{
    "function":  true,
    "method":    true,
    "class":     true,
    "interface": true,
    "struct":    true,
    "type":      true,
    "trait":     true,
    "enum":      true,
}

func (g *DataGatherer) findDeadCodeCandidates(data *HealthReportData) error {
    entities, err := g.store.QueryEntities(store.EntityFilter{
        Status: "active",
        Limit:  1000,
    })
    if err != nil {
        return err
    }

    for _, e := range entities {
        // Skip entity types not tracked in call graph
        if !deadCodeTypes[e.EntityType] {
            continue
        }

        // Skip test files
        if strings.Contains(e.FilePath, "_test.go") || strings.Contains(e.FilePath, ".test.") {
            continue
        }

        // ... rest of existing logic
    }
}
```

#### Cross-Language Entity Type Mapping

| Language | Whitelist Types Used |
|----------|---------------------|
| Go | function, method, type, interface |
| TypeScript | function, method, class, interface |
| Python | function, method, class |
| Java | method, class, interface, enum |
| Rust | function, method, struct, trait, enum |
| C# | method, class, interface, struct, enum |
| C/C++ | function, method, class, struct |
| PHP | function, method, class, interface, trait |
| Kotlin | function, method, class, interface |
| Ruby | method, class |

All use a subset of the whitelist - no language uses types outside it.

#### Acceptance Criteria

- [ ] Whitelist map defined with all 8 trackable types
- [ ] Filter applied before caller check
- [ ] Dead code candidates reduced from 641 to <100 on cortex codebase
- [ ] No imports, variables, or constants in output

---

### T2: Group Dead Code by Module

#### Schema Changes

Add to `internal/report/schema.go`:

```go
// DeadCodeGroup groups dead code candidates by module/directory.
type DeadCodeGroup struct {
    // Type is always "dead_code_group" for this issue type.
    Type string `yaml:"type" json:"type"`

    // Module is the directory path containing the dead code.
    Module string `yaml:"module" json:"module"`

    // Count is the number of candidates in this module.
    Count int `yaml:"count" json:"count"`

    // Candidates are the dead code entities in this module.
    Candidates []DeadCodeCandidate `yaml:"candidates" json:"candidates"`
}

// DeadCodeCandidate represents a single dead code candidate.
type DeadCodeCandidate struct {
    // Entity is the entity name.
    Entity string `yaml:"entity" json:"entity"`

    // EntityType is the type (function, method, class, etc.).
    EntityType string `yaml:"entity_type" json:"entity_type"`

    // File is the full file path.
    File string `yaml:"file" json:"file"`

    // Line is the starting line number.
    Line int `yaml:"line" json:"line"`

    // Recommendation is the suggested action.
    Recommendation string `yaml:"recommendation,omitempty" json:"recommendation,omitempty"`
}
```

#### Output Format Change

**Before (flat list):**
```yaml
issues:
  info:
    - type: dead_code_candidate
      entity: funcA
      file: internal/bd/bd.go
    - type: dead_code_candidate
      entity: funcB
      file: internal/bd/bd.go
    - type: dead_code_candidate
      entity: funcC
      file: internal/cache/cache.go
```

**After (grouped by module):**
```yaml
issues:
  info:
    - type: dead_code_group
      module: internal/bd
      count: 2
      candidates:
        - entity: funcA
          entity_type: function
          file: internal/bd/bd.go
          line: 42
        - entity: funcB
          entity_type: method
          file: internal/bd/bd.go
          line: 87
    - type: dead_code_group
      module: internal/cache
      count: 1
      candidates:
        - entity: funcC
          entity_type: function
          file: internal/cache/cache.go
          line: 156
```

#### Implementation

Update `findDeadCodeCandidates` to:
1. Collect candidates into a map keyed by module (directory)
2. Sort modules by candidate count (most dead code first)
3. Output as `DeadCodeGroup` issues instead of individual `HealthIssue`

#### Acceptance Criteria

- [ ] DeadCodeGroup and DeadCodeCandidate types added to schema.go
- [ ] Dead code grouped by directory path
- [ ] Modules sorted by count (descending)
- [ ] Each candidate includes entity_type and line number
- [ ] Existing tests updated for new output format

---

### T3: Cross-Language Testing

#### Test Matrix

| Language | Test Repository | Expected Entity Types |
|----------|-----------------|----------------------|
| Go | cortex (self) | function, method, type |
| TypeScript | simple-typescript-starter | function, method, class |
| Python | python-mini-projects | function, method, class |
| Java | simple-java-maven-app | method, class |
| Rust | example_project_structure | function, struct |
| C++ | cmake-project-template | function, class |
| C# | csharp-for-everybody | method, class |
| PHP | quickstart-basic | function, method, class |
| Kotlin | kotlin-android-practice | function, method, class |
| Ruby | learn-rails | method, class |

#### Test Procedure

For each language:

```bash
# 1. Clone (if not already)
git clone <repo-url> ~/cortex-test-repos/<name>

# 2. Scan
cx scan ~/cortex-test-repos/<name>

# 3. Generate health report
cx report health --data -o health-<lang>.yaml

# 4. Verify output
# - No entity_type: import
# - No entity_type: variable
# - No entity_type: constant
# - All entity_types in whitelist
# - Grouped by module
```

#### Verification Script

```bash
#!/bin/bash
# verify-dead-code.sh

for report in health-*.yaml; do
    echo "Checking $report..."

    # Should find zero imports/variables/constants
    if grep -q "entity_type: import" "$report"; then
        echo "FAIL: Found import in $report"
        exit 1
    fi
    if grep -q "entity_type: variable" "$report"; then
        echo "FAIL: Found variable in $report"
        exit 1
    fi
    if grep -q "entity_type: constant" "$report"; then
        echo "FAIL: Found constant in $report"
        exit 1
    fi

    # Should find dead_code_group
    if ! grep -q "type: dead_code_group" "$report"; then
        echo "WARN: No dead_code_group in $report (may be OK if no dead code)"
    fi

    echo "PASS: $report"
done
```

#### Acceptance Criteria

- [ ] All 10 languages tested
- [ ] Zero false positives (no import/variable/constant)
- [ ] All flagged entity types are in whitelist
- [ ] Output grouped by module
- [ ] Verification script passes

---

## Data Contracts

### Entity Type Whitelist (Shared Constant)

```go
// internal/report/gather.go

// DeadCodeEntityTypes defines which entity types can be detected as dead code.
// These are types tracked in the call graph (have callers/callees relationships).
// Types NOT in this list (import, variable, constant) are excluded because
// their usage cannot be tracked via the dependency graph.
var DeadCodeEntityTypes = map[string]bool{
    "function":  true,  // All languages
    "method":    true,  // All languages
    "class":     true,  // TS, Python, Java, C#, C++, PHP, Kotlin, Ruby
    "interface": true,  // Go, TS, Java, C#, PHP, Kotlin
    "struct":    true,  // Go, Rust, C#, C/C++
    "type":      true,  // Go (type aliases)
    "trait":     true,  // Rust, PHP
    "enum":      true,  // Java, Rust, C#
}
```

### HealthIssue Changes

The existing `HealthIssue` type remains for critical/warning issues. Dead code moves to `DeadCodeGroup` in info section.

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/report/gather.go` | Add whitelist filter, implement grouping logic |
| `internal/report/schema.go` | Add `DeadCodeGroup`, `DeadCodeCandidate` types |
| `internal/report/gather_test.go` | Add unit tests for whitelist filter |
| `internal/report/health_test.go` | Update tests for grouped output |

---

## Out of Scope

- **Confidence scoring** - Not needed, whitelist is deterministic
- **Cross-file analysis** - Future enhancement for reflection/build tags
- **Cyclomatic complexity** - Separate feature, not related to dead code
- **Auto-removal** - Too dangerous, report only

---

## Multi-Agent Execution Notes

### Recommended Assignment

| Task | Agent Type | Reasoning |
|------|-----------|-----------|
| T1: Whitelist | Polecat (backend) | Straightforward filter implementation |
| T2: Grouping | Polecat (backend) | Schema + logic changes |
| T3: Testing | Polecat (test) | Clone repos, run verification |

### Execution Strategy

```
Phase 1: T1 (single agent)
         ↓
Phase 2: T2 (single agent)
         ↓
Phase 3: T3 (single agent, can parallelize per-language if needed)
```

### Estimated Effort

- T1: Small (add filter, ~20 lines)
- T2: Medium (schema + grouping logic, ~100 lines)
- T3: Medium (test setup, verification script)

**Total**: ~3 agent sessions

---

## Success Metrics

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Dead code candidates | 641 | TBD | < 100 |
| False positive rate | ~90% | TBD | 0% |
| Entity types flagged | 6 | TBD | ≤ 8 (whitelist only) |
| Output usability | Low (flat) | Grouped | Grouped by module |

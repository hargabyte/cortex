# Cortex Language Support Testing Checklist

This document provides a comprehensive set of test commands to verify language support in Cortex. Run these tests against a representative codebase for each language to ensure feature parity with Go.

---

## Table of Contents

1. [Test Setup](#test-setup)
2. [Phase 1: Initialization](#phase-1-initialization)
3. [Phase 2: Entity Discovery](#phase-2-entity-discovery)
4. [Phase 3: Type-Specific Extraction](#phase-3-type-specific-extraction)
5. [Phase 4: Dependency Tracking](#phase-4-dependency-tracking)
6. [Phase 5: Context Commands](#phase-5-context-commands)
7. [Phase 6: Safety Commands](#phase-6-safety-commands)
8. [Phase 7: Map Commands](#phase-7-map-commands)
9. [Phase 8: Search Commands](#phase-8-search-commands)
10. [Phase 9: Output Format Verification](#phase-9-output-format-verification)
11. [Phase 10: Edge Cases](#phase-10-edge-cases)
12. [Test Result Template](#test-result-template)

---

## Test Setup

### Prerequisites

1. Install the test repository for the target language
2. Navigate to the test repository root
3. Ensure `cx` binary is in PATH or use full path

### Environment Variables

```bash
# Set the language being tested
export TEST_LANG="go"  # go|typescript|python|java|rust|c|csharp|php

# Set test file paths (adjust for each language)
export TEST_FILE="internal/parser/parser.go"
export TEST_DIR="internal/parser"
export TEST_ENTITY="Execute"
```

### Recommended Test Repositories

| Language | Repository | Notes |
|----------|------------|-------|
| Go | Cortex itself | Production codebase |
| TypeScript | React codebase | Modern TS patterns |
| Python | Django/Flask app | Class hierarchies |
| Java | Spring Boot app | Enterprise patterns |
| Rust | Rust CLI tool | Traits, impls |
| C | SQLite/Redis | Macros, headers |
| C# | ASP.NET Core | Namespaces, generics |
| PHP | Laravel app | Traits, namespaces |

---

## Phase 1: Initialization

### P1.1 Fresh Scan
```bash
# Initialize and scan the codebase
cx scan

# Expected: Lists files scanned, entities created
# Verify: Count > 0, no errors
```

### P1.2 Database Info
```bash
# Check database was populated
cx db info

# Expected fields:
# - Entities: > 0 (active + archived)
# - Dependencies: > 0
# - Files indexed: > 0
```

### P1.3 Health Check
```bash
# Verify database integrity
cx doctor

# Expected: All checks passed ✓
```

### P1.4 Language Detection
```bash
# Verify language was detected
cx find --lang $TEST_LANG --limit 5

# Expected: Returns entities for the test language
```

---

## Phase 2: Entity Discovery

### P2.1 Basic Find
```bash
# Find entities by name
cx find $TEST_ENTITY

# Expected output:
# - type: correct entity type
# - location: file:line-line format
# - signature: parameter and return types
# - visibility: public or private
```

### P2.2 Find with Limit
```bash
# Limit results
cx find $TEST_ENTITY --limit 1
cx find $TEST_ENTITY --limit 5
cx find $TEST_ENTITY --limit 100

# Verify: Result count matches limit (or total if fewer)
```

### P2.3 Exact Match
```bash
# Exact match only
cx find $TEST_ENTITY --exact

# Verify: Only exact matches returned, no prefix matches
```

### P2.4 Qualified Name
```bash
# Use qualified name format
cx find "package.$TEST_ENTITY"

# Verify: Matches qualified name pattern
```

### P2.5 Path-Qualified Name
```bash
# Use path-qualified format
cx find "path/to/file.$TEST_ENTITY"

# Verify: Matches by file path and symbol
```

### P2.6 Important Entities
```bash
# Get entities sorted by importance
cx find --important --top 20

# Expected: Entities with metrics.pagerank field
# Verify: Results sorted by pagerank (descending)
```

### P2.7 Keystone Entities
```bash
# Find critical entities
cx find --keystones --top 10

# Expected: High in_degree entities
# Verify: importance: keystone in results
```

### P2.8 Bottleneck Entities
```bash
# Find bottleneck entities
cx find --bottlenecks --top 10

# Expected: High betweenness entities
# Verify: importance: bottleneck in results
```

---

## Phase 3: Type-Specific Extraction

### P3.1 Functions Only
```bash
# Filter to functions
cx find --type F --limit 10

# Expected: All results have type: function
# For each result verify:
# - Signature with parameters
# - Return type (if applicable)
```

### P3.2 Types/Classes Only
```bash
# Filter to types
cx find --type T --limit 10

# Expected: All results have type: struct|class|type|interface
```

### P3.3 Methods Only
```bash
# Filter to methods
cx find --type M --limit 10

# Expected: All results have type: method
# Verify: Methods have receiver/class association
```

### P3.4 Constants Only
```bash
# Filter to constants
cx find --type C --limit 10

# Expected: All results have type: constant|variable
```

### P3.5 Combined Type and Language Filter
```bash
# Combine filters
cx find --type F --lang $TEST_LANG --limit 10

# Verify: All results are functions in target language
```

---

## Phase 4: Dependency Tracking

### P4.1 Show Entity Details
```bash
# Get entity with dependencies
cx show $TEST_ENTITY

# Expected output fields:
# - type
# - location
# - signature
# - visibility
# - dependencies.calls: array of entity IDs
# - dependencies.called_by: array of {name, location?}
# - dependencies.uses_types: array of entity IDs (if applicable)
```

### P4.2 Show with Dense Density
```bash
# Full details including metrics and hashes
cx show $TEST_ENTITY --density dense

# Additional expected fields:
# - metrics.pagerank: float
# - metrics.in_degree: int
# - metrics.out_degree: int
# - metrics.importance: string
# - hashes.signature: hex string
# - hashes.body: hex string
```

### P4.3 Show Neighborhood
```bash
# Get entity with neighborhood
cx show $TEST_ENTITY --related

# Expected output structure:
# center:
#   name, type, location
# neighborhood:
#   same_file: array of entities in same file
#   calls: array of called entities
#   callers: array of calling entities
```

### P4.4 Deep Neighborhood
```bash
# 2-hop neighborhood
cx show $TEST_ENTITY --related --depth 2

# Verify: More entities included vs depth 1
```

### P4.5 Show Graph
```bash
# Dependency graph
cx show $TEST_ENTITY --graph

# Expected output structure:
# graph:
#   root: entity name
#   direction: both
#   depth: 2
# nodes:
#   EntityName: {type, location, depth, signature}
# edges:
#   - [from, to, edge_type]
```

### P4.6 Deep Graph
```bash
# 3-hop graph
cx show $TEST_ENTITY --graph --hops 3

# Verify: More nodes and edges vs hops 2
```

### P4.7 Incoming Edges Only
```bash
# Only show callers
cx show $TEST_ENTITY --graph --direction in

# Verify: All edges point TO the root entity
```

### P4.8 Outgoing Edges Only
```bash
# Only show dependencies
cx show $TEST_ENTITY --graph --direction out

# Verify: All edges point FROM the root entity
```

### P4.9 Filter by Edge Type
```bash
# Only call edges
cx show $TEST_ENTITY --graph --type calls

# Verify: Only edges with type "calls"
```

### P4.10 File:Line Resolution
```bash
# Show entity at specific line
cx show "${TEST_FILE}:50"

# Verify: Returns entity containing line 50
```

### P4.11 Disambiguation with File Hint
```bash
# Use file hint for ambiguous names
cx show "${TEST_ENTITY}@${TEST_FILE}"

# Verify: Returns correct entity when multiple match
```

---

## Phase 5: Context Commands

### P5.1 Session Recovery
```bash
# Basic context
cx context

# Expected: Markdown formatted workflow guide
# Verify: Database statistics included
```

### P5.2 Full Session Recovery
```bash
# Extended context
cx context --full

# Verify: Includes keystones and additional details
```

### P5.3 Smart Context
```bash
# Intent-aware context
cx context --smart "add feature to $TEST_ENTITY" --budget 5000

# Expected output:
# intent:
#   keywords: array of extracted keywords
#   pattern: modify|add_feature|fix|etc
# entry_points:
#   EntityName: {type, location, note}
# relevant_entities:
#   EntityName: {type, location, relevance, reason}
```

### P5.4 Smart Context - Different Intents
```bash
# Test different patterns
cx context --smart "fix bug in authentication" --budget 5000
cx context --smart "add new endpoint for users" --budget 5000
cx context --smart "refactor database layer" --budget 5000

# Verify: Different patterns detected, different entry points
```

### P5.5 Smart Context with Budget Modes
```bash
# Importance-based pruning
cx context --smart "optimize performance" --budget 3000 --budget-mode importance

# Distance-based pruning
cx context --smart "optimize performance" --budget 3000 --budget-mode distance

# Compare: Different entities included based on mode
```

### P5.6 File Context
```bash
# Context for specific file
cx context $TEST_FILE --hops 2

# Expected:
# context:
#   target: file path
#   budget: token count
#   tokens_used: actual tokens
# relevant: map of entities
```

### P5.7 Context with Coverage
```bash
# Include coverage data
cx context --smart "test coverage" --budget 4000 --with-coverage

# Verify: coverage field on entities (if coverage imported)
```

### P5.8 Context Include/Exclude
```bash
# Explicit inclusions
cx context --smart "task" --budget 4000 --include deps,callers,types

# Explicit exclusions
cx context --smart "task" --budget 4000 --exclude tests,mocks

# Verify: Filtered entities based on flags
```

---

## Phase 6: Safety Commands

### P6.1 Full Safety Assessment
```bash
# Complete safety check
cx safe $TEST_FILE

# Expected output:
# safety_assessment:
#   target: file path
#   risk_level: critical|high|medium|low
#   impact_radius: int
#   files_affected: int
#   keystone_count: int
#   coverage_gaps: int (if coverage data)
#   drift_detected: bool
# warnings: array of warning strings
# recommendations: array of action strings
# affected_keystones: array of keystone details
```

### P6.2 Quick Safety (Blast Radius)
```bash
# Impact analysis only
cx safe $TEST_FILE --quick

# Expected output:
# impact:
#   target: file path
#   depth: int
# summary:
#   files_affected: int
#   entities_affected: int
#   risk_level: low|medium|high
# affected: map of affected entities
```

### P6.3 Deep Impact Analysis
```bash
# Deeper transitive analysis
cx safe $TEST_FILE --quick --depth 5

# Verify: More entities vs default depth 3
```

### P6.4 Drift Detection
```bash
# Check for staleness
cx safe --drift

# Expected output:
# verification:
#   status: passed|failed
#   entities_checked: int
#   valid: int
#   drifted: int
#   missing: int
#   issues: array of issue details
```

### P6.5 Drift with Fix
```bash
# Preview fix
cx safe --drift --dry-run

# Actual fix (careful!)
cx safe --drift --fix
```

### P6.6 Changes Detection
```bash
# What changed since scan
cx safe --changes

# Expected output:
# summary:
#   files_changed: int
#   entities_added: int
#   entities_modified: int
#   entities_removed: int
#   last_scan: timestamp
# added: array of new files/entities
# modified: array of changed entities
# removed: array of removed entities
```

### P6.7 Coverage Gaps (requires coverage import)
```bash
# First import coverage
cx test coverage import coverage.out

# Then check gaps
cx safe --coverage

# Verify: Coverage gap warnings for keystones
```

### P6.8 Keystones-Only Coverage
```bash
# Only keystone coverage gaps
cx safe --coverage --keystones-only

# Verify: Only keystones in results
```

---

## Phase 7: Map Commands

### P7.1 Full Project Map
```bash
# Complete skeleton view
cx map 2>&1 | head -200

# Expected output:
# files:
#   path/to/file:
#     entities:
#       EntityName:
#         type: function|type|etc
#         location: file:line-line
#         skeleton: "function signature { ... }"
#         doc_comment: "// comment"
#         visibility: public|private
# count: int
```

### P7.2 Directory Map
```bash
# Map specific directory
cx map $TEST_DIR

# Verify: Only entities from specified directory
```

### P7.3 Functions Only
```bash
# Filter to functions
cx map --filter F

# Verify: Only function entities
```

### P7.4 Types Only
```bash
# Filter to types
cx map --filter T

# Verify: Only type entities (struct, class, interface)
```

### P7.5 Methods Only
```bash
# Filter to methods
cx map --filter M

# Verify: Only method entities
```

### P7.6 Constants Only
```bash
# Filter to constants
cx map --filter C

# Verify: Only constant/variable entities
```

### P7.7 Language Filter
```bash
# Filter by language
cx map --lang $TEST_LANG

# Verify: Only entities from target language
```

### P7.8 Combined Filters
```bash
# Multiple filters
cx map $TEST_DIR --filter F --lang $TEST_LANG

# Verify: Functions in target language from specified directory
```

---

## Phase 8: Search Commands

### P8.1 Concept Search (FTS)
```bash
# Multi-word query triggers FTS
cx find "entity extraction" --limit 10

# Expected output includes:
# query: "entity extraction"
# results with relevance scoring
```

### P8.2 FTS with Filters
```bash
# FTS with type filter
cx find "parse tree" --type F --limit 5

# Verify: FTS results filtered to functions
```

### P8.3 FTS with Importance Sort
```bash
# FTS sorted by importance
cx find "database query" --important --top 10

# Verify: Results have pagerank, sorted by importance
```

### P8.4 Quoted Phrase Search
```bash
# Quoted phrase
cx find '"exact phrase"' --limit 5

# Verify: Phrase match attempted
```

---

## Phase 9: Output Format Verification

### P9.1 YAML Format
```bash
# Default YAML output
cx find $TEST_ENTITY --format yaml

# Verify: Valid YAML syntax
# Check: Proper indentation, list formatting
```

### P9.2 JSON Format
```bash
# JSON output
cx find $TEST_ENTITY --format json

# Verify: Valid JSON syntax
# Can parse with jq: cx find $TEST_ENTITY --format json | jq .
```

### P9.3 JSONL Format
```bash
# JSON Lines output
cx find $TEST_ENTITY --format jsonl

# Verify: One JSON object per line
# Each line parseable independently
```

### P9.4 Sparse Density
```bash
# Minimal output
cx find $TEST_ENTITY --density sparse

# Verify: Only type and location fields
# Token count: ~50-100 per entity
```

### P9.5 Medium Density
```bash
# Standard output
cx find $TEST_ENTITY --density medium

# Verify: type, location, signature, visibility, dependencies
# Token count: ~200-300 per entity
```

### P9.6 Dense Density
```bash
# Full output
cx find $TEST_ENTITY --density dense

# Verify: All fields including metrics and hashes
# Token count: ~400-600 per entity
```

### P9.7 Quiet Mode
```bash
# Exit code only
cx find $TEST_ENTITY --quiet
echo "Exit code: $?"

# Verify: No output, exit code 0 on success
```

### P9.8 Verbose Mode
```bash
# Debug output
cx find $TEST_ENTITY --verbose 2>&1

# Verify: Additional debug information (if implemented)
```

---

## Phase 10: Edge Cases

### P10.1 Non-existent Entity
```bash
# Search for entity that doesn't exist
cx find "NonExistentEntity12345" --exact

# Expected: Empty results, count: 0
```

### P10.2 Non-existent File
```bash
# Safety check on non-existent file
cx safe "nonexistent/file.go" 2>&1

# Expected: Error message about file not found
```

### P10.3 Ambiguous Entity Name
```bash
# When multiple entities match
cx show "init" 2>&1

# Expected: Error listing multiple matches with file hints
```

### P10.4 Empty Directory
```bash
# Scan empty directory
cx scan /tmp/empty_test_dir --dry-run 2>&1

# Expected: No errors, 0 files scanned
```

### P10.5 Unicode in Entity Names
```bash
# If language supports unicode identifiers
cx find "用户" --limit 5

# Verify: Handles unicode gracefully
```

### P10.6 Very Long File
```bash
# Large file with many entities
cx safe "path/to/large_file.ext" --quick

# Verify: Completes without timeout, accurate count
```

### P10.7 Circular Dependencies
```bash
# Check for circular dependency handling
cx show $TEST_ENTITY --graph --hops 5

# Verify: No infinite loop, cycle handled gracefully
```

### P10.8 Special Characters in Paths
```bash
# Paths with spaces or special chars
cx safe "path/with spaces/file.ext" 2>&1

# Verify: Proper handling or clear error message
```

---

## Test Result Template

Use this template to record test results for each language.

```markdown
# Cortex Language Test Results: [LANGUAGE]

**Test Date**: YYYY-MM-DD
**Tester**: [Name]
**Test Repository**: [Repository URL]
**Cortex Version**: [cx --version output]

## Summary

| Phase | Pass | Fail | Skip | Notes |
|-------|------|------|------|-------|
| 1. Initialization | | | | |
| 2. Entity Discovery | | | | |
| 3. Type-Specific | | | | |
| 4. Dependency Tracking | | | | |
| 5. Context Commands | | | | |
| 6. Safety Commands | | | | |
| 7. Map Commands | | | | |
| 8. Search Commands | | | | |
| 9. Output Formats | | | | |
| 10. Edge Cases | | | | |
| **TOTAL** | | | | |

## Detailed Results

### Phase 1: Initialization

#### P1.1 Fresh Scan
- [ ] PASS
- [ ] FAIL: [reason]
- [ ] SKIP: [reason]

**Output:**
```
[paste output]
```

#### P1.2 Database Info
- [ ] PASS
- [ ] FAIL: [reason]
- [ ] SKIP: [reason]

**Output:**
```
[paste output]
```

[... continue for each test ...]

## Entity Extraction Quality

### Functions
- Total functions extracted: [count]
- Functions with signatures: [count/total]
- Functions with correct visibility: [count/total]
- Sample output:
```yaml
[paste sample function entity]
```

### Types/Classes
- Total types extracted: [count]
- Types with fields: [count/total]
- Types with correct visibility: [count/total]
- Sample output:
```yaml
[paste sample type entity]
```

### Methods
- Total methods extracted: [count]
- Methods with receiver/class: [count/total]
- Sample output:
```yaml
[paste sample method entity]
```

### Dependencies
- Total dependencies tracked: [count]
- Call edges: [count]
- Type reference edges: [count]
- Sample dependency output:
```yaml
[paste sample with dependencies]
```

## Issues Found

### Critical
1. [Issue description, command, expected vs actual]

### Major
1. [Issue description]

### Minor
1. [Issue description]

## Recommendations

1. [Recommendation for improvement]

## Sign-off

- [ ] All critical tests pass
- [ ] Entity extraction is complete
- [ ] Dependencies tracked correctly
- [ ] Context commands produce useful output
- [ ] Ready for production use

Signed: _________________ Date: _________
```

---

## Automated Test Script

Save this as `run_language_tests.sh`:

```bash
#!/bin/bash

# Cortex Language Test Runner
# Usage: ./run_language_tests.sh <language> <test_file> <test_entity>

LANG=${1:-go}
TEST_FILE=${2:-internal/parser/parser.go}
TEST_ENTITY=${3:-Execute}
RESULTS_FILE="test_results_${LANG}_$(date +%Y%m%d_%H%M%S).md"

echo "# Cortex Language Test Results: $LANG" > $RESULTS_FILE
echo "" >> $RESULTS_FILE
echo "**Test Date**: $(date)" >> $RESULTS_FILE
echo "**Test Language**: $LANG" >> $RESULTS_FILE
echo "**Test File**: $TEST_FILE" >> $RESULTS_FILE
echo "**Test Entity**: $TEST_ENTITY" >> $RESULTS_FILE
echo "" >> $RESULTS_FILE

run_test() {
    local test_name="$1"
    local command="$2"

    echo "## $test_name" >> $RESULTS_FILE
    echo '```bash' >> $RESULTS_FILE
    echo "$command" >> $RESULTS_FILE
    echo '```' >> $RESULTS_FILE
    echo "**Output:**" >> $RESULTS_FILE
    echo '```' >> $RESULTS_FILE

    eval "$command" 2>&1 | head -50 >> $RESULTS_FILE
    local exit_code=$?

    echo '```' >> $RESULTS_FILE
    echo "**Exit Code**: $exit_code" >> $RESULTS_FILE
    echo "" >> $RESULTS_FILE

    return $exit_code
}

echo "Running tests for $LANG..."

# Phase 1
run_test "P1.1 Database Info" "cx db info"
run_test "P1.2 Health Check" "cx doctor"
run_test "P1.3 Language Detection" "cx find --lang $LANG --limit 5"

# Phase 2
run_test "P2.1 Basic Find" "cx find $TEST_ENTITY"
run_test "P2.2 Find with Limit" "cx find $TEST_ENTITY --limit 5"
run_test "P2.3 Exact Match" "cx find $TEST_ENTITY --exact"
run_test "P2.4 Important Entities" "cx find --important --top 10"

# Phase 3
run_test "P3.1 Functions Only" "cx find --type F --lang $LANG --limit 10"
run_test "P3.2 Types Only" "cx find --type T --lang $LANG --limit 10"
run_test "P3.3 Methods Only" "cx find --type M --lang $LANG --limit 10"

# Phase 4
run_test "P4.1 Show Entity" "cx show $TEST_ENTITY"
run_test "P4.2 Show Dense" "cx show $TEST_ENTITY --density dense"
run_test "P4.3 Show Related" "cx show $TEST_ENTITY --related"
run_test "P4.4 Show Graph" "cx show $TEST_ENTITY --graph --hops 2"

# Phase 5
run_test "P5.1 Session Context" "cx context"
run_test "P5.2 Smart Context" "cx context --smart 'add feature' --budget 4000"

# Phase 6
run_test "P6.1 Full Safety" "cx safe $TEST_FILE"
run_test "P6.2 Quick Safety" "cx safe $TEST_FILE --quick"
run_test "P6.3 Drift Check" "cx safe --drift"
run_test "P6.4 Changes Check" "cx safe --changes"

# Phase 7
run_test "P7.1 Full Map" "cx map 2>&1 | head -100"
run_test "P7.2 Functions Map" "cx map --filter F --lang $LANG 2>&1 | head -100"

# Phase 8
run_test "P8.1 FTS Search" 'cx find "entity extraction" --limit 5'

# Phase 9
run_test "P9.1 JSON Output" "cx find $TEST_ENTITY --format json"
run_test "P9.2 JSONL Output" "cx find $TEST_ENTITY --format jsonl"
run_test "P9.3 Sparse Density" "cx find $TEST_ENTITY --density sparse"
run_test "P9.4 Dense Density" "cx find $TEST_ENTITY --density dense"

echo "Test results written to: $RESULTS_FILE"
```

---

## Quick Verification Commands

For rapid verification of language support:

```bash
# One-liner verification (adjust entity name for your language)
TEST_LANG=go TEST_ENTITY=Execute && \
cx find --lang $TEST_LANG --limit 3 && \
cx find $TEST_ENTITY --density dense && \
cx show $TEST_ENTITY --related && \
cx map --lang $TEST_LANG --filter F 2>&1 | head -50
```

---

## Success Criteria

A language is considered fully supported when:

1. **Entity Extraction** (100% required)
   - [ ] All entity types extracted (functions, types, methods, etc.)
   - [ ] Correct line ranges
   - [ ] Proper visibility detection
   - [ ] Signature extraction with parameters

2. **Dependency Tracking** (100% required)
   - [ ] Function calls detected
   - [ ] Type references detected
   - [ ] Import/include tracking

3. **Graph Analysis** (100% required)
   - [ ] PageRank computes correctly
   - [ ] In/out degree accurate
   - [ ] Transitive dependencies traced

4. **Context Quality** (90% required)
   - [ ] Smart context finds relevant code
   - [ ] Entry points correctly identified
   - [ ] Reasonable relevance scoring

5. **Safety Analysis** (90% required)
   - [ ] Impact radius calculated
   - [ ] Affected entities identified
   - [ ] Risk levels appropriate

# CX 3.0: Runtime & Test Intelligence

## Executive Summary

Extend cx beyond static analysis into **runtime behavior** and **test coverage** intelligence. This addresses the fundamental limitation of static analysis: it shows what *could* happen, not what *does* happen.

**Epic**: Superhero-AI-cnl
**Depends On**: Superhero-AI-dgg (CX 2.0 complete)
**Priority**: P2 (Future)

---

## The Gap

| Question | Static Analysis (CX 2.0) | Runtime/Test Analysis (CX 3.0) |
|----------|--------------------------|-------------------------------|
| What calls this function? | All possible callers | Actual callers in production |
| Which implementation is used? | All interface implementers | The one actually instantiated |
| Is this code tested? | Unknown | Yes/No with coverage % |
| Is this dead code? | Maybe (leaf detection) | Definitively (never executed) |
| What's slow? | Unknown | Hot paths with timing |

---

## Phase 5: Runtime Behavior Analysis

**Feature**: Superhero-AI-cnl.2

### The Problem

Static analysis is conservative - it must assume all code paths are possible. This leads to:
- False "dead code" alerts for reflection-called code
- All interface implementations shown as equally important
- No insight into hot paths vs cold paths
- Missing config-driven conditional logic

### 5.1 cx trace: Runtime Call Graph Capture

Capture actual execution paths from running applications.

**Input Sources**:
```bash
# Live instrumentation
cx trace ./myapp --duration 60s

# From existing Go profiles
cx trace --from-pprof cpu.prof

# From OpenTelemetry traces
cx trace --from-otel traces.json

# From custom instrumentation
cx trace --from-jsonl calls.jsonl
```

**Output**:
```yaml
trace:
  duration: 60s
  total_calls: 1_234_567
  unique_edges: 342

  hot_paths:
    - path: [main, Router.ServeHTTP, UserHandler.Create, UserService.Create, SQLRepo.Insert]
      calls: 45_000
      avg_latency: 12ms

    - path: [main, Router.ServeHTTP, AuthHandler.Login, AuthService.Validate, Cache.Get]
      calls: 120_000
      avg_latency: 2ms

  never_called:  # In trace but expected from static analysis
    - LegacyValidator
    - OldPaymentProcessor
    - DeprecatedCache
```

### 5.2 cx runtime: Static vs Runtime Comparison

Compare what static analysis predicts with what actually happens.

```bash
cx runtime compare
```

```yaml
comparison:
  static_edges: 1_456
  runtime_edges: 342
  overlap: 298

  static_only:  # Predicted but never observed
    - LegacyValidator.Validate:
        static_callers: [UserService.Create, ContactForm.Submit]
        runtime_calls: 0
        verdict: likely_dead_code

    - RedisCache.Get:
        static_callers: [UserService.GetByID]
        runtime_calls: 0
        verdict: feature_flagged_off

  runtime_only:  # Observed but not in static graph
    - reflect.Value.Call -> DynamicHandler:
        runtime_calls: 5_000
        verdict: reflection_call

  interface_resolution:
    Repository:
      static_implementations: [SQLRepo, RedisRepo, MockRepo, FileRepo]
      runtime_distribution:
        SQLRepo: 99.8%
        RedisRepo: 0.2%
        MockRepo: 0%
        FileRepo: 0%
      recommendation: Focus on SQLRepo, others may be dead
```

### 5.3 cx profile: Performance-Aware Context

Include runtime performance data when assembling context.

```bash
cx context --smart "optimize the slow endpoint" --with-profile
```

```yaml
context:
  target: optimize the slow endpoint

  hot_functions:  # From profile data
    - UserService.GetByID:
        cpu_percent: 23%
        allocations: 45MB/min
        location: internal/user/service.go:67-120

    - JSONSerializer.Marshal:
        cpu_percent: 18%
        allocations: 120MB/min
        location: internal/api/serialize.go:30-50

  slow_paths:
    - /api/users/{id}:
        p50: 45ms
        p99: 890ms
        bottleneck: UserService.GetByID (database N+1 query)

  optimization_hints:
    - "UserService.GetByID has N+1 query pattern - consider batch loading"
    - "JSONSerializer allocates heavily - consider sync.Pool"
```

### Technical Approach

| Source | Integration Method |
|--------|-------------------|
| Go runtime/trace | Parse trace.Event stream |
| pprof | Import profile.proto format |
| OpenTelemetry | Parse OTLP JSON/protobuf |
| Custom | JSONL with {caller, callee, timestamp} |

**Safety for Production**:
- Sampling (1% of requests by default)
- Bounded buffer (drop oldest when full)
- CPU overhead target: <2%

---

## Phase 6: Test Coverage Intelligence

**Feature**: Superhero-AI-cnl.3

### The Problem

"Is this function tested?" is the most basic quality question, yet static analysis can't answer it. Worse:
- Keystones with 0% coverage are ticking time bombs
- Dead tests waste CI time and give false confidence
- Test selection is manual (run all tests, wait forever)

### 6.1 cx coverage: Test-to-Entity Mapping

Import coverage data and map it to cx entities.

```bash
# Generate coverage
go test -coverprofile=coverage.out ./...

# Import into cx
cx coverage import coverage.out
```

**Enhanced Entity Output**:
```yaml
# cx show UserService.Create --coverage

UserService.Create:
  type: method
  location: internal/user/service.go:45-89
  importance: keystone

  coverage:
    tested: true
    percent: 78%

    tested_by:
      - TestUserService_Create_Success:
          location: internal/user/service_test.go:20-45
          covers_lines: [45-67, 75-80]

      - TestUserService_Create_DuplicateEmail:
          location: internal/user/service_test.go:50-70
          covers_lines: [45-55, 81-85]

      - TestUserService_Create_InvalidInput:
          location: internal/user/service_test.go:75-95
          covers_lines: [45-50, 86-89]

    uncovered_lines: [68-74]  # Database error handling
    uncovered_reason: "Error path when db.Create fails"
```

### 6.2 cx gaps: Coverage Gap Detection

Find the dangerous combination: high importance + low coverage.

```bash
cx gaps                     # All gaps
cx gaps --keystones-only    # Only keystones
cx gaps --threshold 50      # Below 50% coverage
cx gaps --create-tasks      # Create beads for each gap
```

```yaml
# cx gaps --keystones-only

coverage_gaps:

  critical:  # Keystones with <25% coverage
    - PaymentProcessor.Charge:
        importance: keystone
        in_degree: 8
        coverage: 12%
        uncovered_paths:
          - refund_flow
          - partial_payment
          - currency_conversion
        risk: CRITICAL
        recommendation: "Stop. Add tests before ANY changes."

    - AuthService.ValidateToken:
        importance: keystone
        in_degree: 15
        coverage: 0%
        risk: CRITICAL
        recommendation: "Security-critical with no tests. Priority 1."

  high:  # Keystones with 25-50% coverage
    - UserService.Create:
        importance: keystone
        coverage: 45%
        uncovered_paths: [email_verification, rate_limiting]
        risk: HIGH

  moderate:  # Keystones with 50-75% coverage
    - OrderService.Process:
        importance: keystone
        coverage: 67%
        uncovered_paths: [inventory_backorder]
        risk: MODERATE

summary:
  keystones_total: 15
  critical_gaps: 2
  high_gaps: 3
  moderate_gaps: 4
  healthy: 6

  recommendation: "Address critical gaps before next release"
```

### 6.3 cx test-impact: Smart Test Selection

Given a change, identify exactly which tests need to run.

```bash
# What tests cover this file?
cx test-impact internal/auth/login.go

# What tests are affected by my uncommitted changes?
cx test-impact --diff

# What tests are affected by this commit?
cx test-impact --commit HEAD~1
```

```yaml
# cx test-impact internal/auth/login.go

affected_tests:
  direct:  # Tests that directly call changed entities
    - TestLoginUser_Success @ internal/auth/login_test.go:20
    - TestLoginUser_InvalidEmail @ internal/auth/login_test.go:45
    - TestLoginUser_WrongPassword @ internal/auth/login_test.go:70

  indirect:  # Tests that call callers of changed entities
    - TestAuthMiddleware @ internal/middleware/auth_test.go:15
    - TestUserHandler_Login @ internal/handlers/user_test.go:100

  integration:  # Integration tests touching this code
    - TestFullLoginFlow @ tests/integration/auth_test.go:30

summary:
  total_tests: 156
  affected_tests: 8
  time_saved: "Run 8 tests (~12s) instead of 156 tests (~4min)"

command: "go test -run 'TestLoginUser|TestAuthMiddleware|TestUserHandler_Login|TestFullLoginFlow' ./..."
```

### 6.4 Dead Test Detection

Find tests that waste CI time.

```yaml
# cx tests --dead

dead_tests:

  tests_dead_code:  # Tests for code that's never called
    - TestLegacyValidator:
        location: internal/legacy/validator_test.go:10
        tests_entity: LegacyValidator.Validate
        entity_status: dead_code (0 runtime calls)
        recommendation: Delete test and code

  tests_nothing:  # Tests that don't exercise production code
    - TestHelperFunction:
        location: internal/utils/helper_test.go:50
        production_coverage: 0%
        only_calls: [test helpers, mocks]
        recommendation: Review if this test has value

  flaky_tests:  # Tests with inconsistent results (from CI history)
    - TestRaceCondition:
        location: internal/sync/race_test.go:30
        pass_rate: 92%
        recommendation: Fix or quarantine

  slow_tests:  # Tests that dominate CI time
    - TestFullIntegration:
        location: tests/integration/full_test.go:10
        duration: 45s (30% of total test time)
        recommendation: Consider parallelization or mocking
```

### Technical Approach

**Coverage Import**:
```go
// Parse go test -coverprofile output
type CoverageBlock struct {
    File      string
    StartLine int
    EndLine   int
    NumStmt   int
    Count     int  // Execution count
}

// Map to cx entities
func MapCoverageToEntities(blocks []CoverageBlock, entities []Entity) map[EntityID]Coverage
```

**Test-to-Entity Mapping**:
1. Parse test files to find Test* functions
2. For each test, trace calls to production code (static analysis)
3. Cross-reference with coverage data for actual execution
4. Store in new `test_coverage` table

**Database Schema**:
```sql
CREATE TABLE test_coverage (
    entity_id TEXT NOT NULL,
    test_id TEXT NOT NULL,           -- TestLoginUser_Success
    test_file TEXT NOT NULL,         -- internal/auth/login_test.go
    test_line INTEGER NOT NULL,
    coverage_percent REAL,
    covered_lines TEXT,              -- JSON array
    uncovered_lines TEXT,            -- JSON array
    last_run TEXT,
    PRIMARY KEY (entity_id, test_id)
);

CREATE TABLE coverage_runs (
    run_id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    commit_hash TEXT,
    total_coverage REAL,
    entities_covered INTEGER,
    entities_total INTEGER
);
```

---

## Combined Power: Static + Runtime + Coverage

The real value is combining all three:

```yaml
# cx show UserService.Create --full

UserService.Create:
  type: method
  location: internal/user/service.go:45-89

  static_analysis:
    importance: keystone
    in_degree: 8
    out_degree: 5
    calls: [repo.Create, cache.Set, validator.Validate, events.Publish]
    called_by: [UserHandler.Create, AdminHandler.CreateUser, ...]

  runtime_analysis:
    observed: true
    calls_per_minute: 450
    avg_latency: 23ms
    p99_latency: 120ms
    hot_callees: [repo.Create (80% of time)]

  test_coverage:
    tested: true
    percent: 78%
    tested_by: 3 tests
    uncovered_paths: [database error handling]

  combined_risk:
    score: MEDIUM
    factors:
      - keystone: HIGH (many dependents)
      - coverage: MEDIUM (78%)
      - runtime: LOW (stable, no errors)
    recommendation: "Add error path tests before modifying"
```

---

## Implementation Order

### Phase 5 (Runtime) Tasks
1. Define trace format and storage schema
2. Implement pprof import
3. Implement OpenTelemetry import
4. Build `cx trace` command
5. Build `cx runtime compare` command
6. Build `cx profile` integration with context

### Phase 6 (Coverage) Tasks
1. Implement coverage.out parser
2. Build coverage-to-entity mapping
3. Add coverage fields to entity output
4. Build `cx gaps` command
5. Build `cx test-impact` command
6. Build dead test detection

### Integration Tasks
7. Combined output (`--full` flag)
8. Risk scoring algorithm
9. CI integration guide
10. Documentation

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Runtime trace import | <5s for 1M events |
| Coverage mapping | <10s for 10k entities |
| Test impact accuracy | 95% (tests identified match actual failures) |
| Gap detection | Identify all 0% coverage keystones |
| Dead test detection | 80% precision (flagged tests are actually dead) |

---

## Prerequisites

Before starting CX 3.0:
- [ ] CX 2.0 complete and stable
- [ ] YAML output format finalized
- [ ] Entity importance metrics validated
- [ ] Test codebase available for dogfooding

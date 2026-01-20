# Runtime Span-to-Entity Mapping Specification

## Executive Summary

This document specifies how Cortex maps OpenTelemetry trace spans to code entities. The core challenge: **production traces rarely contain code location attributes** (`code.function.name`, `code.file.path`). Instead, we must infer entity mappings from semantic conventions like HTTP routes, RPC methods, and database operations.

**Key Finding**: ~95% of real-world instrumentation does NOT emit `code.*` attributes. Pattern-based matching is the primary strategy, not a fallback.

---

## Table of Contents

1. [The Problem](#the-problem)
2. [Span Types and Their Data](#span-types-and-their-data)
3. [The Mapping Pipeline](#the-mapping-pipeline)
4. [Framework-Specific Route Discovery](#framework-specific-route-discovery)
5. [Database Schema](#database-schema)
6. [Configuration Schema](#configuration-schema)
7. [Implementation Phases](#implementation-phases)
8. [Edge Cases and Challenges](#edge-cases-and-challenges)
9. [Testing Strategy](#testing-strategy)

---

## The Problem

### What We Have

OpenTelemetry traces contain spans with semantic attributes:

```yaml
span:
  name: "GET /api/users/{id}"
  attributes:
    http.method: GET
    http.route: /api/users/{id}
    http.status_code: 200
    http.response.content_length: 1234
```

### What We Need

Map this span to a Cortex entity:

```yaml
entity:
  id: sa-fn-a1b2c3-GetUser
  name: GetUser
  type: function
  file_path: internal/handlers/users.go
  line_start: 42
  line_end: 89
```

### Why `code.*` Attributes Are Rare

| Instrumentation Type | Emits `code.*`? | What It Emits |
|---------------------|-----------------|---------------|
| Flask/Django auto-instrumentation | No | `http.method`, `http.route` |
| Java Agent (HTTP spans) | No | `http.method`, `http.target` |
| Express/Fastify auto-instrumentation | No | `http.method`, `http.route` |
| Grafana Beyla (eBPF) | No | HTTP/gRPC metrics only |
| Database instrumentation | No | `db.operation`, `db.sql.table` |
| gRPC instrumentation | Sometimes | `rpc.service`, `rpc.method` |
| Manual instrumentation | Rarely | Developer's choice |

The `code.*` semantic conventions exist and are stable (since v1.33.0), but auto-instrumentation libraries prioritize **protocol semantics** over **code location**.

---

## Span Types and Their Data

### 1. HTTP Server Spans

**Source**: Framework instrumentation (Flask, Django, Express, Gin, Spring, etc.)

**Available Attributes**:
```yaml
http.method: GET
http.scheme: https
http.host: api.example.com
http.target: /api/users/123
http.route: /api/users/{id}        # Route pattern (key for mapping)
http.status_code: 200
http.request.content_length: 0
http.response.content_length: 1234
http.flavor: "1.1"
net.host.name: api.example.com
net.host.port: 443
user_agent.original: "Mozilla/5.0..."
```

**Span Name Conventions**:
- `GET /api/users/{id}` (most common)
- `HTTP GET` (less useful)
- `handleGetUser` (manual instrumentation)

**Mapping Strategy**: `http.route` + `http.method` → Handler function

### 2. HTTP Client Spans

**Source**: HTTP client libraries (requests, axios, net/http, OkHttp)

**Available Attributes**:
```yaml
http.method: POST
http.url: https://payment.example.com/charge
http.status_code: 200
peer.service: payment-service
net.peer.name: payment.example.com
net.peer.port: 443
```

**Mapping Strategy**: Less useful for mapping (shows what code is calling, not being called)

### 3. Database Spans

**Source**: Database driver instrumentation (pg, mysql, sqlalchemy, gorm)

**Available Attributes**:
```yaml
db.system: postgresql
db.name: myapp_production
db.user: app_user
db.operation: SELECT
db.sql.table: users           # Key for mapping
db.statement: "SELECT * FROM users WHERE id = $1"  # Sometimes available
```

**Span Name Conventions**:
- `SELECT myapp.users`
- `postgresql.query`
- `db.query`

**Mapping Strategy**: `db.sql.table` + `db.operation` → Repository/DAO method

### 4. RPC/gRPC Spans

**Source**: gRPC instrumentation, RPC frameworks

**Available Attributes**:
```yaml
rpc.system: grpc
rpc.service: com.example.users.UserService
rpc.method: GetUser
rpc.grpc.status_code: 0
net.peer.name: users-service
net.peer.port: 50051
```

**Span Name Conventions**:
- `com.example.users.UserService/GetUser`
- `UserService/GetUser`
- `grpc.client.unary`

**Mapping Strategy**: `rpc.service` + `rpc.method` → Service method (easiest case)

### 5. Messaging Spans

**Source**: Message queue instrumentation (Kafka, RabbitMQ, SQS)

**Available Attributes**:
```yaml
messaging.system: kafka
messaging.destination: orders.created
messaging.destination_kind: topic
messaging.operation: publish
messaging.message.id: "abc-123"
messaging.kafka.partition: 3
```

**Mapping Strategy**: `messaging.destination` + `messaging.operation` → Handler/Producer function

### 6. Custom/Manual Spans

**Source**: Developer-created spans with `tracer.start_span()`

**Available Attributes**: Varies entirely by developer choice

**Mapping Strategy**: Fuzzy name matching, or rely on `code.*` if present

---

## The Mapping Pipeline

```
┌─────────────────────────────────────────────────────────────────────┐
│                        SPAN INGESTION                               │
│  - Parse OTLP JSON/protobuf                                         │
│  - Extract service name, span name, attributes                      │
│  - Classify span type (HTTP, DB, RPC, Messaging, Custom)            │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      ATTRIBUTE EXTRACTION                           │
│                                                                     │
│  HTTP:      (method, route) → "GET /api/users/{id}"                 │
│  RPC:       (service, method) → "UserService.GetUser"               │
│  Database:  (operation, table) → "SELECT users"                     │
│  Messaging: (destination, operation) → "orders.created PUBLISH"     │
│  Custom:    span.name → "processOrder"                              │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        MAPPING LOOKUP                               │
│                                                                     │
│  1. Check code.* attributes (rare, but highest confidence)          │
│  2. Query mapping table: (service, pattern) → entity_id             │
│  3. Try RPC direct match: rpc.method → entity name                  │
│  4. Fuzzy name matching as last resort                              │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      CONFIDENCE SCORING                             │
│                                                                     │
│  code.* exact match:    1.00                                        │
│  Route table match:     0.95                                        │
│  RPC direct match:      0.90                                        │
│  Fuzzy match (>0.9):    0.70                                        │
│  Fuzzy match (>0.8):    0.50                                        │
│  No match:              0.00                                        │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        AGGREGATION                                  │
│                                                                     │
│  Per entity accumulate:                                             │
│  - Total calls                                                      │
│  - Latency histogram (p50, p95, p99)                                │
│  - Error count and rate                                             │
│  - Last seen timestamp                                              │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Framework-Specific Route Discovery

The key to accurate mapping is **static analysis of route registrations** in the codebase. This section details how to discover routes for each supported framework.

### Go Frameworks

#### net/http (stdlib)

```go
// Pattern: http.HandleFunc(pattern, handler)
http.HandleFunc("/users", handleUsers)
http.Handle("/api/", apiHandler)

// Pattern: mux.HandleFunc(pattern, handler)
mux := http.NewServeMux()
mux.HandleFunc("/health", healthCheck)
```

**Tree-sitter Query**:
```scheme
(call_expression
  function: (selector_expression
    operand: (identifier) @receiver
    field: (field_identifier) @method)
  arguments: (argument_list
    (interpreted_string_literal) @route
    (identifier) @handler))
  (#match? @method "^Handle(Func)?$"))
```

**Extraction**: Route pattern → Handler function name

#### Gin

```go
// Pattern: router.METHOD(path, ...handlers)
r := gin.Default()
r.GET("/users/:id", GetUser)
r.POST("/users", CreateUser)
r.Group("/api").GET("/health", HealthCheck)
```

**Tree-sitter Query**:
```scheme
(call_expression
  function: (selector_expression
    field: (field_identifier) @method)
  arguments: (argument_list
    (interpreted_string_literal) @route
    (identifier) @handler))
  (#match? @method "^(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD)$"))
```

**Extraction**: Method + Route pattern → Handler function

#### Echo

```go
// Pattern: e.METHOD(path, handler)
e := echo.New()
e.GET("/users/:id", getUser)
e.POST("/users", createUser)
```

**Tree-sitter Query**: Similar to Gin

#### Chi

```go
// Pattern: r.Method(path, handler) or r.Route(path, func)
r := chi.NewRouter()
r.Get("/users/{id}", GetUser)
r.Post("/users", CreateUser)
r.Route("/api", func(r chi.Router) {
    r.Get("/health", HealthCheck)
})
```

**Tree-sitter Query**:
```scheme
(call_expression
  function: (selector_expression
    field: (field_identifier) @method)
  arguments: (argument_list
    (interpreted_string_literal) @route
    . (identifier) @handler))
  (#match? @method "^(Get|Post|Put|Delete|Patch|Options|Head|Connect|Trace)$"))
```

#### Gorilla Mux

```go
// Pattern: r.HandleFunc(path, handler).Methods(...)
r := mux.NewRouter()
r.HandleFunc("/users/{id}", GetUser).Methods("GET")
r.HandleFunc("/users", CreateUser).Methods("POST")
```

**Tree-sitter Query**: Match `HandleFunc` call, then look for chained `.Methods()` call

### Python Frameworks

#### Flask

```python
# Pattern: @app.route(path) decorator
@app.route('/users/<int:id>')
def get_user(id):
    pass

# Pattern: @app.METHOD(path) decorator
@app.get('/users/<int:id>')
def get_user(id):
    pass

# Pattern: app.add_url_rule
app.add_url_rule('/users/<int:id>', 'get_user', get_user, methods=['GET'])
```

**Tree-sitter Query**:
```scheme
(decorated_definition
  (decorator
    (call
      function: (attribute
        object: (identifier) @app
        attribute: (identifier) @method)
      arguments: (argument_list
        (string) @route)))
  definition: (function_definition
    name: (identifier) @handler))
  (#match? @method "^(route|get|post|put|delete|patch)$"))
```

#### Django

```python
# Pattern: path() in urlpatterns
# urls.py
from django.urls import path
from . import views

urlpatterns = [
    path('users/<int:id>/', views.get_user, name='get_user'),
    path('users/', views.UserListView.as_view(), name='user_list'),
]
```

**Tree-sitter Query**: Match `path()` calls in files named `urls.py`

**Challenge**: View function is referenced, may need to resolve import

#### FastAPI

```python
# Pattern: @app.METHOD(path) decorator (similar to Flask)
@app.get("/users/{id}")
async def get_user(id: int):
    pass

# Pattern: @router.METHOD(path) for APIRouter
router = APIRouter()

@router.get("/users/{id}")
async def get_user(id: int):
    pass
```

**Tree-sitter Query**: Similar to Flask

### Java/Spring

#### Spring Boot

```java
// Pattern: @RequestMapping, @GetMapping, etc. on methods
@RestController
@RequestMapping("/api")
public class UserController {

    @GetMapping("/users/{id}")
    public User getUser(@PathVariable Long id) {
        return userService.findById(id);
    }

    @PostMapping("/users")
    public User createUser(@RequestBody UserDTO user) {
        return userService.create(user);
    }
}
```

**Tree-sitter Query**:
```scheme
;; First, get class-level @RequestMapping
(class_declaration
  (modifiers
    (annotation
      name: (identifier) @ann
      arguments: (annotation_argument_list
        (string_literal) @class_path)))
  name: (identifier) @class_name)
  (#eq? @ann "RequestMapping"))

;; Then, get method-level mappings
(method_declaration
  (modifiers
    (annotation
      name: (identifier) @ann
      arguments: (annotation_argument_list
        (string_literal) @method_path)))
  name: (identifier) @method_name)
  (#match? @ann "^(Get|Post|Put|Delete|Patch|Request)Mapping$"))
```

**Extraction**: Combine class-level and method-level paths

#### JAX-RS

```java
@Path("/api")
public class UserResource {

    @GET
    @Path("/users/{id}")
    public User getUser(@PathParam("id") Long id) {
        return userService.findById(id);
    }
}
```

**Tree-sitter Query**: Similar pattern, different annotation names

### TypeScript/JavaScript Frameworks

#### Express

```typescript
// Pattern: app.METHOD(path, handler)
app.get('/users/:id', getUser);
app.post('/users', createUser);

// Pattern: router.METHOD(path, handler)
const router = express.Router();
router.get('/users/:id', getUser);

// Pattern: Inline handlers (harder)
app.get('/users/:id', (req, res) => {
    // inline handler - map to file:line
});
```

**Tree-sitter Query**:
```scheme
(call_expression
  function: (member_expression
    property: (property_identifier) @method)
  arguments: (arguments
    (string) @route
    (identifier) @handler))
  (#match? @method "^(get|post|put|delete|patch|options|head|all|use)$"))
```

**Challenge**: Inline arrow functions require file:line mapping instead of function name

#### Fastify

```typescript
// Pattern: fastify.METHOD(path, handler)
fastify.get('/users/:id', async (request, reply) => {
    // handler
});

// Pattern: With schema
fastify.get('/users/:id', {
    schema: { ... },
    handler: getUser
});
```

#### NestJS

```typescript
// Pattern: Decorators (similar to Spring)
@Controller('users')
export class UsersController {

    @Get(':id')
    findOne(@Param('id') id: string) {
        return this.usersService.findOne(id);
    }

    @Post()
    create(@Body() createUserDto: CreateUserDto) {
        return this.usersService.create(createUserDto);
    }
}
```

**Tree-sitter Query**: Match decorator patterns on class and method declarations

### Rust Frameworks

#### Actix-web

```rust
// Pattern: #[METHOD(path)] attribute macro
#[get("/users/{id}")]
async fn get_user(path: web::Path<i32>) -> impl Responder {
    // handler
}

// Pattern: web::resource().route()
web::resource("/users/{id}")
    .route(web::get().to(get_user))
```

**Tree-sitter Query**:
```scheme
(function_item
  (attribute_item
    (attribute
      (identifier) @method
      arguments: (token_tree
        (string_literal) @route)))
  name: (identifier) @handler)
  (#match? @method "^(get|post|put|delete|patch)$"))
```

#### Axum

```rust
// Pattern: Router::new().route(path, handler)
let app = Router::new()
    .route("/users/:id", get(get_user))
    .route("/users", post(create_user));
```

**Tree-sitter Query**: Match `.route()` method calls on Router

### Summary: Framework Detection

| Language | Frameworks to Detect | Detection Method |
|----------|---------------------|------------------|
| Go | gin, echo, chi, gorilla, net/http | Import statements |
| Python | flask, django, fastapi | Import statements |
| Java | spring, jax-rs | Annotation imports |
| TypeScript | express, fastify, nestjs, hono | Package.json deps, imports |
| Rust | actix-web, axum, rocket | Cargo.toml deps |

---

## Database Schema

### Runtime Mappings Table

```sql
CREATE TABLE runtime_mappings (
    id TEXT PRIMARY KEY,              -- Auto-generated UUID

    -- Match criteria
    service_name TEXT NOT NULL,        -- e.g., "api-server"
    span_type TEXT NOT NULL,           -- http, rpc, db, messaging, custom
    match_pattern TEXT NOT NULL,       -- e.g., "GET /api/users/{id}"

    -- Target entity
    entity_id TEXT,                    -- Cortex entity ID (nullable if unresolved)
    entity_name TEXT,                  -- Denormalized for display

    -- Metadata
    confidence REAL DEFAULT 0.0,       -- 0.0 to 1.0
    source TEXT NOT NULL,              -- 'auto', 'manual', 'inferred'
    framework TEXT,                    -- 'gin', 'flask', 'spring', etc.

    -- Timestamps
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    verified_at TEXT,                  -- Last human verification
    last_matched_at TEXT,              -- Last time a span matched this

    -- Indexes
    UNIQUE(service_name, span_type, match_pattern)
);

CREATE INDEX idx_mappings_lookup ON runtime_mappings(service_name, span_type, match_pattern);
CREATE INDEX idx_mappings_entity ON runtime_mappings(entity_id);
```

### Runtime Metrics Table

```sql
CREATE TABLE runtime_metrics (
    entity_id TEXT PRIMARY KEY,

    -- Traffic metrics
    calls_total INTEGER DEFAULT 0,
    calls_period INTEGER DEFAULT 0,     -- Calls in current period
    calls_per_minute REAL DEFAULT 0.0,

    -- Latency metrics (microseconds)
    latency_sum INTEGER DEFAULT 0,
    latency_count INTEGER DEFAULT 0,
    latency_min INTEGER,
    latency_max INTEGER,
    latency_p50 INTEGER,
    latency_p95 INTEGER,
    latency_p99 INTEGER,

    -- Error metrics
    error_count INTEGER DEFAULT 0,
    error_rate REAL DEFAULT 0.0,
    last_error_message TEXT,
    last_error_time TEXT,

    -- Metadata
    data_source TEXT,                   -- 'otlp', 'pprof', 'manual'
    period_start TEXT,
    period_end TEXT,
    updated_at TEXT NOT NULL,

    FOREIGN KEY (entity_id) REFERENCES entities(id)
);

CREATE INDEX idx_metrics_calls ON runtime_metrics(calls_per_minute DESC);
CREATE INDEX idx_metrics_errors ON runtime_metrics(error_rate DESC);
CREATE INDEX idx_metrics_latency ON runtime_metrics(latency_p99 DESC);
```

### Runtime Metrics History Table

```sql
CREATE TABLE runtime_metrics_history (
    entity_id TEXT NOT NULL,
    snapshot_date TEXT NOT NULL,        -- YYYY-MM-DD

    calls_per_minute REAL,
    error_rate REAL,
    latency_p99 INTEGER,

    PRIMARY KEY (entity_id, snapshot_date),
    FOREIGN KEY (entity_id) REFERENCES entities(id)
);
```

### Unmatched Spans Table

```sql
-- Track spans we couldn't map, for manual review and improvement
CREATE TABLE unmatched_spans (
    id TEXT PRIMARY KEY,

    service_name TEXT NOT NULL,
    span_type TEXT NOT NULL,
    span_name TEXT NOT NULL,
    attributes TEXT,                    -- JSON

    -- Aggregation
    occurrence_count INTEGER DEFAULT 1,
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,

    -- Resolution
    status TEXT DEFAULT 'pending',      -- pending, ignored, resolved
    resolved_entity_id TEXT,
    resolved_at TEXT,
    resolved_by TEXT,                   -- 'user', 'auto'

    UNIQUE(service_name, span_type, span_name)
);

CREATE INDEX idx_unmatched_count ON unmatched_spans(occurrence_count DESC);
```

---

## Configuration Schema

```yaml
# .cx/config.yaml

runtime:
  # Enable/disable runtime integration
  enabled: true

  # Service identification
  # Maps trace service names to code paths
  services:
    - name: api-server
      paths:
        - cmd/api
        - internal/api
        - internal/handlers
    - name: worker
      paths:
        - cmd/worker
        - internal/jobs
    - name: auth-service
      paths:
        - cmd/auth
        - internal/auth

  # Framework auto-discovery
  discovery:
    enabled: true

    # Framework detection per language
    frameworks:
      go:
        - gin
        - echo
        - chi
        - gorilla
        - stdlib
      python:
        - flask
        - django
        - fastapi
      java:
        - spring
        - jax-rs
      typescript:
        - express
        - fastify
        - nestjs
        - hono
      rust:
        - actix-web
        - axum
        - rocket

    # Exclude patterns from discovery
    exclude:
      - "**/test/**"
      - "**/mock/**"
      - "**/*_test.go"
      - "**/*.test.ts"

  # Manual route mappings (override auto-discovery)
  mappings:
    http:
      - pattern: "GET /api/v1/users/{id}"
        entity: GetUser
        service: api-server
      - pattern: "POST /api/v1/orders"
        entity: CreateOrder
        service: api-server
      - pattern: "GET /health"
        entity: HealthCheck
        service: "*"  # All services

    rpc:
      - pattern: "UserService/GetUser"
        entity: GetUser
        service: user-service

    database:
      - pattern: "SELECT users"
        entity: UserRepository.Find*
        service: "*"

  # RPC settings
  rpc:
    # Strip common package prefixes
    strip_prefixes:
      - "com.example."
      - "org.company."
      - "github.com/company/project/"

    # Service name normalization
    normalize_service_names: true  # UserService -> userservice for matching

  # Database settings
  database:
    # Map tables to repository patterns
    repository_patterns:
      - "*Repository"
      - "*DAO"
      - "*Store"
      - "*Repo"

    # Table to entity mapping
    table_mappings:
      users: UserRepository
      orders: OrderRepository
      products: ProductRepository

  # Fuzzy matching settings
  fuzzy:
    enabled: true
    threshold: 0.80           # Minimum similarity score
    max_candidates: 5         # Max entities to consider per span

    # Boost certain patterns
    boost:
      - pattern: "Handler$"
        multiplier: 1.2
      - pattern: "Controller$"
        multiplier: 1.2
      - pattern: "Service$"
        multiplier: 1.1

  # Import sources
  sources:
    # File-based import
    - type: otlp_file
      path: ./traces/*.json
      watch: false            # Watch for new files

    # OTLP HTTP receiver
    - type: otlp_http
      endpoint: http://localhost:4318
      enabled: false

    # OTLP gRPC receiver
    - type: otlp_grpc
      endpoint: localhost:4317
      enabled: false

  # Aggregation settings
  aggregation:
    period: 1h                # Aggregation window
    retention: 30d            # How long to keep detailed metrics
    history_retention: 365d   # How long to keep daily snapshots

  # Alerting thresholds (for cx guard integration)
  thresholds:
    high_traffic:
      calls_per_minute: 1000
      label: hot
    high_latency:
      p99_ms: 500
      label: slow
    high_error_rate:
      percent: 5.0
      label: unstable
```

---

## Implementation Phases

### Phase 1: Schema and Storage (Week 1)

**Deliverables**:
- [ ] Create `runtime_mappings` table
- [ ] Create `runtime_metrics` table
- [ ] Create `runtime_metrics_history` table
- [ ] Create `unmatched_spans` table
- [ ] Add runtime fields to entity output
- [ ] Unit tests for CRUD operations

**Commands**:
```bash
cx runtime status          # Show runtime integration status
cx runtime metrics <entity> # Show metrics for entity
```

### Phase 2: Manual Mapping (Week 2)

**Deliverables**:
- [ ] `cx runtime map add` command
- [ ] `cx runtime map list` command
- [ ] `cx runtime map remove` command
- [ ] Config file parsing for mappings
- [ ] Mapping validation (entity exists)

**Commands**:
```bash
cx runtime map add "GET /api/users/{id}" GetUser --service api-server
cx runtime map list
cx runtime map list --service api-server
cx runtime map remove <mapping-id>
```

### Phase 3: OTLP Import (Week 3-4)

**Deliverables**:
- [ ] OTLP JSON parser
- [ ] Span type classifier
- [ ] Attribute extractor
- [ ] Mapping lookup pipeline
- [ ] Metrics aggregator
- [ ] Unmatched span tracker

**Commands**:
```bash
cx runtime import traces.json
cx runtime import ./traces/
cx runtime import --format otlp-json traces.json
cx runtime unmatched         # Show unmatched spans
cx runtime unmatched --resolve <span-id> <entity>
```

### Phase 4: Auto-Discovery (Week 5-6)

**Deliverables**:
- [ ] Framework detection per language
- [ ] Route extraction for Go frameworks
- [ ] Route extraction for Python frameworks
- [ ] Route extraction for Java frameworks
- [ ] Route extraction for TypeScript frameworks
- [ ] Route extraction for Rust frameworks
- [ ] Automatic mapping generation

**Commands**:
```bash
cx runtime discover          # Discover routes and generate mappings
cx runtime discover --dry-run # Show what would be discovered
cx runtime discover --framework gin
cx runtime discover --service api-server
```

### Phase 5: Enhanced Commands (Week 7)

**Deliverables**:
- [ ] `cx show` with runtime section
- [ ] `cx find --hot` flag
- [ ] `cx find --errors` flag
- [ ] `cx safe` with runtime context
- [ ] `cx dead --runtime` flag

**Output Examples**:

```yaml
# cx show GetUser --runtime
GetUser:
  type: function
  location: internal/handlers/users.go:42-89

  static:
    importance: keystone
    in_degree: 5
    out_degree: 8

  runtime:
    calls_per_minute: 450
    latency:
      p50: 12ms
      p95: 45ms
      p99: 120ms
    error_rate: 0.02%
    trend: "+15% this week"
    last_error: "2024-01-19 14:32 - database timeout"
```

```yaml
# cx find --hot --top 10
hot_entities:
  - entity: GetUser
    calls_per_minute: 450
    latency_p99: 120ms
    location: internal/handlers/users.go:42

  - entity: AuthMiddleware
    calls_per_minute: 2100
    latency_p99: 5ms
    location: internal/middleware/auth.go:15
```

### Phase 6: CI Integration (Week 8)

**Deliverables**:
- [ ] `cx guard --runtime` for pre-commit checks
- [ ] Runtime drift detection (code changed, runtime patterns differ)
- [ ] Documentation
- [ ] Integration tests

---

## Edge Cases and Challenges

### 1. Dynamic Routes

**Problem**: Routes constructed at runtime can't be statically discovered.

```go
// This can't be detected statically
path := fmt.Sprintf("/api/v%d/users", version)
router.GET(path, handler)
```

**Mitigation**:
- Track unmatched spans and surface for manual mapping
- Use fuzzy matching on span names
- Allow pattern-based mappings: `"GET /api/v*/users/{id}"`

### 2. Middleware Attribution

**Problem**: Spans may be created by middleware, not the actual handler.

```
Span: "GET /api/users/{id}"
Created by: LoggingMiddleware
Actual handler: GetUser
```

**Mitigation**:
- Use `http.route` attribute which is typically set correctly
- Match span name pattern to route registration

### 3. Anonymous Functions

**Problem**: Inline handlers can't be matched by function name.

```typescript
app.get('/users/:id', (req, res) => {
    // No function name to match
});
```

**Mitigation**:
- Use file:line mapping instead
- Create synthetic entity ID: `sa-fn-<hash>-anonymous@handlers.ts:42`

### 4. Multi-Service Confusion

**Problem**: Same route exists in multiple services.

```
api-gateway: GET /api/users/{id} → routes to...
user-service: GET /api/users/{id} → actual handler
```

**Mitigation**:
- Always use (service_name, pattern) as composite key
- Require service name in mappings

### 5. Version Drift

**Problem**: Code changes but mappings are stale.

```
Old code: GET /users/{id} → GetUser
New code: GET /users/{id} → GetUserV2 (GetUser deleted)
Mapping still points to: GetUser (entity doesn't exist)
```

**Mitigation**:
- Validate mappings during `cx scan`
- Flag mappings pointing to non-existent entities
- `cx runtime validate` command

### 6. High Cardinality Spans

**Problem**: Span names include high-cardinality values.

```
Bad: "GET /api/users/12345" (includes ID)
Good: "GET /api/users/{id}" (parameterized)
```

**Mitigation**:
- Prefer `http.route` attribute over span name
- Pattern normalization: detect numeric segments, UUIDs
- Aggregate by pattern, not exact span name

### 7. RPC Package Prefix Variations

**Problem**: Same service has different package prefixes.

```
Span A: "com.example.UserService/GetUser"
Span B: "UserService/GetUser"
Span C: "users.v1.UserService/GetUser"
```

**Mitigation**:
- Configurable prefix stripping
- Normalize to shortest unique form
- Match from right-to-left (method first, then service)

---

## Testing Strategy

### Unit Tests

1. **OTLP Parser**: Parse various OTLP JSON formats
2. **Span Classifier**: Correctly identify span types
3. **Attribute Extractor**: Extract key attributes per span type
4. **Mapping Matcher**: Match patterns to entities
5. **Fuzzy Matcher**: Score similarity correctly
6. **Metrics Aggregator**: Calculate percentiles correctly

### Integration Tests

1. **Route Discovery**: Discover routes from sample projects
2. **End-to-End Import**: Import traces, generate mappings, aggregate metrics
3. **Multi-Language**: Test discovery across all supported languages

### Test Fixtures

Create sample projects with known routes:

```
testdata/
├── go-gin/
│   ├── main.go          # Gin routes
│   └── expected.yaml    # Expected mappings
├── python-flask/
│   ├── app.py           # Flask routes
│   └── expected.yaml
├── java-spring/
│   ├── UserController.java
│   └── expected.yaml
├── ts-express/
│   ├── routes.ts
│   └── expected.yaml
└── traces/
    ├── sample-http.json
    ├── sample-rpc.json
    └── sample-db.json
```

### Acceptance Criteria

| Metric | Target |
|--------|--------|
| Route discovery accuracy | >95% for supported frameworks |
| Mapping match accuracy | >90% for auto-discovered routes |
| Import throughput | >10,000 spans/second |
| False positive rate | <5% (incorrect entity matches) |
| Unmatched span rate | <20% (spans we can't map) |

---

## Future Considerations

### Live Mode

Eventually support real-time OTLP ingestion:

```bash
cx runtime serve --otlp-http :4318
```

### Sentry/Datadog Integration

Import from observability platforms:

```bash
cx runtime import --from-sentry <project-id>
cx runtime import --from-datadog <service>
```

### Bidirectional Linking

Add entity IDs as span attributes for perfect matching:

```go
// Instrumentation adds entity ID
span.SetAttribute("cx.entity.id", "sa-fn-abc123-GetUser")
```

### Machine Learning Matching

Train a model on manually resolved mappings to improve auto-matching:

```bash
cx runtime train          # Train on resolved unmatched spans
cx runtime suggest <span> # Suggest entity matches
```

---

## References

- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [OpenTelemetry Code Attributes](https://opentelemetry.io/docs/specs/semconv/attributes-registry/code/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [Grafana Beyla](https://grafana.com/oss/beyla-ebpf/)
- [OpenTelemetry Auto-Instrumentation](https://opentelemetry.io/docs/concepts/instrumentation/automatic/)

# AGENTS.md - Claude Plan Viewer Development Guide

This guide helps agents understand the codebase structure, conventions, and workflow for the Claude Plan Viewer project.

## Project Overview

Claude Plan Viewer is a Go application that:
- Syncs Claude AI plans from `~/.claude/plans/` to a database (SQLite or PostgreSQL)
- Provides a Terminal User Interface (TUI) for browsing and viewing plans
- Supports full-text search across plans
- Tracks plan versions, reading time, word count, and file metadata
- Includes a connector system for sending plans to external services (e.g., Telegram)
- Optionally serves plans via HTTP API

**Main commands**: `sync`, `tui`, `serve`, `migrate`

## Essential Commands

### Building and Running
```bash
make build           # Build binary to bin/plan-viewer
make sync            # Sync plans from ~/.claude/plans/ to database
make tui             # Start terminal user interface
make serve           # Start web server (default :8081)
make test            # Run all tests
make generate        # Regenerate SQLC code for both SQLite and PostgreSQL
make deps            # Download and tidy dependencies
make clean           # Remove build artifacts
```

### Development Workflow
1. **After modifying SQL schema or queries**:
   ```bash
   make generate  # Regenerates repository code for both database backends
   ```
   - SQLite: `services/claude-viewer/repository/sqlite/`
   - PostgreSQL: `services/claude-viewer/repository/postgres/`
   - Do NOT hand-edit generated files (`queries.sql.go`, `models.go`, `db.go`)

2. **Before committing**:
   ```bash
   make test      # Verify tests pass
   ```

3. **Quick iteration**:
   ```bash
   go run cmd/main.go sync
   go run cmd/main.go tui
   DEBUG=1 go run cmd/main.go tui  # Enable debug mode
   ```

## Project Structure

```
.
├── cmd/                              # CLI entry points
│   ├── main.go                       # Command router (sync/serve/tui/migrate)
│   ├── http/                         # HTTP server
│   │   ├── server.go                 # Echo server setup
│   │   ├── handlers.go               # HTTP route handlers
│   │   └── types.go                  # Request/response types
│   └── tui/                          # Terminal User Interface
│       ├── app.go                    # TUI application entry
│       ├── screens/                  # Screen implementations
│       ├── components/               # Reusable UI components
│       ├── content/                  # Content rendering
│       └── styles/                   # Styling definitions
│
├── services/claude-viewer/           # Core business logic
│   ├── service.go                    # Service initialization
│   ├── sync.go                       # Plan synchronization
│   ├── search.go                     # Search functionality
│   ├── version.go                    # Plan versioning
│   ├── settings.go                   # Settings management
│   ├── connector.go                  # Connector operations
│   ├── types.go                      # Domain types (PlanSummary, PlanDetail)
│   ├── service_test.go               # Comprehensive test suite (~1800 lines)
│   │
│   ├── dto/                          # Data Transfer Objects (shared types)
│   │   ├── models.go                 # Plan, PlanVersion, Connector, Setting
│   │   ├── params.go                 # Parameter structs for mutations
│   │   └── repository.go             # Repository interface (45 methods)
│   │
│   ├── repository/
│   │   ├── sqlite/                   # SQLite implementation
│   │   │   ├── repository.go         # Implements dto.Repository
│   │   │   ├── queries.sql.go        # SQLC-generated (DO NOT EDIT)
│   │   │   ├── models.go             # SQLC-generated (DO NOT EDIT)
│   │   │   └── db.go                 # SQLC-generated (DO NOT EDIT)
│   │   └── postgres/                 # PostgreSQL implementation
│   │       ├── repository.go         # Implements dto.Repository
│   │       ├── queries.sql.go        # SQLC-generated (DO NOT EDIT)
│   │       ├── models.go             # SQLC-generated (DO NOT EDIT)
│   │       └── db.go                 # SQLC-generated (DO NOT EDIT)
│   │
│   └── sqlc/                         # SQL definitions
│       ├── sqlite/
│       │   ├── schema.sql            # SQLite schema
│       │   └── queries.sql           # SQLite queries
│       └── postgres/
│           ├── schema.sql            # PostgreSQL schema
│           └── queries.sql           # PostgreSQL queries
│
├── internal/
│   ├── config/                       # Configuration loading
│   │   └── config.go                 # TOML config, env vars
│   ├── connectors/                   # External service connectors
│   │   ├── connector.go              # Connector interface, SettingGetter
│   │   ├── manager.go                # Connector orchestration
│   │   ├── registry.go               # Connector registration
│   │   ├── telegram/                 # Telegram connector implementation
│   │   │   └── telegram.go
│   │   └── test/                     # Generated mocks
│   │       └── connector_stub.go     # gomock-generated
│   └── storage/                      # Database infrastructure
│       ├── sqlite.go                 # SQLite connection + migrations
│       └── postgres.go               # PostgreSQL connection + migrations
│
├── sqlc.yaml                         # SQLC config (both engines)
├── plan-viewer.toml.example          # Example configuration file
└── Makefile                          # Build automation
```

## Architecture

### Repository Pattern with Two Implementations

```
┌─────────────────────────────────────────────────────────┐
│                    Service Layer                         │
│              (uses dto.Repository interface)             │
│              (uses dto.Plan, dto.Setting, etc.)          │
├─────────────────────────────────────────────────────────┤
│     SQLite Implementation   │   Postgres Implementation  │
│   (implements Repository)   │   (implements Repository)  │
│   converts: sql.Null* →     │   converts: pgtype.* →     │
│             dto types       │             dto types      │
├─────────────────────────────────────────────────────────┤
│   SQLC-generated queries    │   SQLC-generated queries   │
│   (internal to sqlite pkg)  │   (internal to postgres)   │
└─────────────────────────────────────────────────────────┘
```

**Key principle**: Service layer uses only `dto.Repository` interface and `dto.*` types. Each database implementation converts its SQLC types internally.

### Connector System

```
┌─────────────────────────────────────────────────────────┐
│                     Service                              │
│              (has ConnectorManager)                      │
├─────────────────────────────────────────────────────────┤
│                  ConnectorManager                        │
│   - Registry (available connectors)                      │
│   - dto.Repository (settings storage)                    │
│   - Implements SettingGetter interface                   │
├─────────────────────────────────────────────────────────┤
│     Connector Interface     │  ConfigurableConnector     │
│   - Name()                  │  - LoadConfig(SettingGetter)│
│   - DisplayName()           │                            │
│   - Send(title, content)    │                            │
│   - Validate()              │                            │
│   - RequiredSettings()      │                            │
└─────────────────────────────────────────────────────────┘
```

## Code Patterns

### Service Layer
```go
type Service struct {
    db               dto.Repository    // Database operations
    viewerDir        string            // ~/.claude-viewer
    sourcePlansDir   string            // ~/.claude/plans
    indexFullContent bool              // Store full content or just title
    markdown         goldmark.Markdown // Markdown renderer
    nowProvider      NowProvider       // Injected time (testable)
    connectorManager *connectors.Manager
}
```

### DTO Types (Clean Go Types)
```go
// dto/models.go - No sql.Null* or pgtype.* here
type Plan struct {
    ID         int64
    FileName   string
    FilePath   string
    Title      string
    Content    string
    CreatedAt  time.Time
    ModifiedAt time.Time
    IndexedAt  time.Time
    FileSize   int64
    WordCount  int64
}
```

### Repository Interface
```go
// dto/repository.go
type Repository interface {
    // Plan operations
    CountPlans(ctx context.Context) (int64, error)
    GetPlanByFileName(ctx context.Context, fileName string) (Plan, error)
    InsertPlan(ctx context.Context, params InsertPlanParams) error
    // ... 45 methods total
}
```

### Connector Interface
```go
// internal/connectors/connector.go
type Connector interface {
    Name() string
    DisplayName() string
    Send(ctx context.Context, title, content string) (*SendResult, error)
    Validate() error
    RequiredSettings() []SettingDefinition
}

type SettingGetter interface {
    GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error)
}

type ConfigurableConnector interface {
    Connector
    LoadConfig(ctx context.Context, getter SettingGetter) error
}
```

## Configuration

### Config File (plan-viewer.toml)
```toml
[database]
backend = "sqlite"  # or "postgres"

[database.sqlite]
path = "~/.claude-viewer/plans.db"

[database.postgres]
connection_string = "postgres://user:pass@localhost:5432/planviewer"
max_open_conns = 25
max_idle_conns = 5

[paths]
plans_dir = "~/.claude/plans"
viewer_dir = "~/.claude-viewer"
```

### Environment Variables
```bash
PLAN_VIEWER_DB_BACKEND=postgres
PLAN_VIEWER_POSTGRES_URL=postgres://...
DEBUG=1  # Enable TUI debug mode
```

## Database Schema

### Core Tables (both SQLite and PostgreSQL)

| Table | Purpose |
|-------|---------|
| `plans` | Indexed plan files |
| `plan_versions` | Version history for plans |
| `settings` | Application settings (typed key-value) |
| `connectors` | Registered external connectors |
| `connector_settings` | Per-connector configuration |

### plans table
```sql
id           INTEGER PRIMARY KEY
file_name    TEXT NOT NULL UNIQUE
file_path    TEXT NOT NULL
title        TEXT NOT NULL
content      TEXT NOT NULL
created_at   TIMESTAMP NOT NULL
modified_at  TIMESTAMP NOT NULL
indexed_at   TIMESTAMP NOT NULL
file_size    INTEGER NOT NULL
word_count   INTEGER NOT NULL DEFAULT 0
```

### plan_versions table
```sql
id             INTEGER PRIMARY KEY
plan_id        INTEGER NOT NULL REFERENCES plans(id)
version_number INTEGER NOT NULL
file_path      TEXT NOT NULL
content        TEXT NOT NULL
word_count     INTEGER NOT NULL
created_at     TIMESTAMP NOT NULL
UNIQUE(plan_id, version_number)
```

## Testing

### Test Structure
- Main test file: `services/claude-viewer/service_test.go` (~1800 lines)
- Uses gomock for connector mocking
- Table-driven tests with subtests
- Mock time provider for deterministic tests

### Running Tests
```bash
make test                                    # All tests
go test -v ./services/claude-viewer          # Service tests only
go test -run TestConnectorManager -v ./...   # Specific test
```

### Test Patterns
```go
// Setup helper
func setupTestService(t *testing.T) (*Service, string, string, func())

// Mock connector with gomock
func setupMockConnector(ctrl *gomock.Controller, name, displayName string) *connectors_test.MockConnector

// Time injection
type mockNowProvider struct {
    currentTime time.Time
}
```

## Common Tasks

### Adding a New Database Field
1. Update both schema files:
   - `services/claude-viewer/sqlc/sqlite/schema.sql`
   - `services/claude-viewer/sqlc/postgres/schema.sql`
2. Update query files if needed
3. Run `make generate`
4. Add field to `dto/models.go`
5. Update repository implementations to map the new field
6. Update service layer
7. Write tests

### Adding a New Connector
1. Create package: `internal/connectors/myconnector/`
2. Implement `Connector` interface (and `ConfigurableConnector` if configurable)
3. Register in `cmd/main.go`:
   ```go
   registry.Register(myconnector.New())
   ```
4. Write tests

### Switching Database Backend
1. Set in config file: `backend = "postgres"`
2. Or via env: `PLAN_VIEWER_DB_BACKEND=postgres`
3. Run migrations: `./bin/plan-viewer migrate`

## Key Files Quick Reference

| File | Purpose |
|------|---------|
| `cmd/main.go` | Entry point, command routing, dependency wiring |
| `services/claude-viewer/service.go` | Service initialization |
| `services/claude-viewer/dto/repository.go` | Repository interface definition |
| `services/claude-viewer/dto/models.go` | Shared domain types |
| `services/claude-viewer/repository/sqlite/repository.go` | SQLite implementation |
| `services/claude-viewer/repository/postgres/repository.go` | PostgreSQL implementation |
| `internal/connectors/manager.go` | Connector orchestration |
| `internal/connectors/connector.go` | Connector interfaces |
| `internal/config/config.go` | Configuration loading |
| `sqlc.yaml` | SQLC code generation config |

## Important Notes

1. **Never edit SQLC-generated files** - They will be overwritten by `make generate`

2. **Repository implementations are internal** - Service layer only uses `dto.Repository`

3. **Connector settings stored in database** - Use `Manager.SetConnectorSetting()` and `Manager.GetConnectorSetting()`

4. **Plan versioning is automatic** - New versions created on sync when content changes

5. **TUI is the primary interface** - HTTP server is secondary

6. **Tests use SQLite** - Even when developing PostgreSQL features, tests run against SQLite for speed

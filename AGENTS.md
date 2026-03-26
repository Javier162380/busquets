# AGENTS.md - Claude Plan Viewer Development Guide

This guide helps agents understand the codebase structure, conventions, and workflow for the Claude Plan Viewer project.

## Project Overview

Claude Plan Viewer is a Go web application that:
- Syncs Claude AI plans from `~/.claude/plans/` to a local SQLite database
- Provides full-text search across plans
- Renders markdown plans with syntax highlighting in a web interface
- Tracks reading time, word count, and file metadata

**Main commands**: `sync` (index plans) and `serve` (start web server on :8081)

## Essential Commands

### Building and Running
```bash
make build           # Build binary to bin/plan-viewer
make sync            # Sync plans from ~/.claude/plans/ to database
make serve           # Start web server (default :8081, custom with -addr :3000)
make test            # Run all tests with verbose output
make generate        # Regenerate SQLC code (run after schema/query changes)
make deps            # Download and tidy dependencies
make clean           # Remove build artifacts and generated repository code
```

### Development Workflow
1. **After modifying SQL schema** (`services/claude-viewer/sqlc/schema.sql`) or **queries** (`services/claude-viewer/sqlc/queries.sql`):
   ```bash
   make generate  # Regenerates services/claude-viewer/repository/ code
   ```
   - Do NOT hand-edit files in `services/claude-viewer/repository/` - they are SQLC-generated
   - Changes to `queries.sql` create new methods on the `Queries` struct

2. **Before committing**:
   ```bash
   make test      # Verify tests pass
   ```

3. **Quick iteration during development**:
   ```bash
   go run cmd/main.go sync    # Run sync without full build
   go run cmd/main.go serve   # Run server without full build
   ```

## Project Structure

```
.
├── cmd/                          # CLI entry point and HTTP server
│   ├── main.go                  # Command router (sync/serve)
│   └── http/                    # Web server
│       ├── server.go            # Echo server setup
│       ├── handlers.go          # HTTP route handlers
│       ├── types.go             # Request/response types
│       └── templates/           # HTML templates
│
├── services/claude-viewer/       # Core business logic
│   ├── service.go               # Service initialization
│   ├── sync.go                  # Plan synchronization logic
│   ├── search.go                # Search functionality
│   ├── update.go                # Plan update operations
│   ├── viewer.go                # Plan rendering
│   ├── settings.go              # Settings management
│   ├── types.go                 # Domain types (PlanSummary, PlanDetail, etc.)
│   ├── service_test.go          # Comprehensive test suite
│   ├── repository/              # SQLC-generated database layer (AUTO-GENERATED)
│   │   ├── models.go            # Generated struct types from schema
│   │   ├── queries.sql.go       # Generated query methods
│   │   ├── querier.go           # Generated query interface
│   │   └── db.go                # Database connection wrapper
│   └── sqlc/                    # SQL definitions for SQLC code generation
│       ├── schema.sql           # Database schema (2 tables: plans, settings)
│       └── queries.sql          # Parameterized queries (13 queries defined)
│
├── internal/storage/            # Database infrastructure
│   └── db.go                    # SQLite connection setup
│
├── go.mod / go.sum              # Go module dependencies
├── Makefile                     # Build and task automation
├── .golangci.yml                # Linting configuration (23 linters enabled)
├── .tool-versions               # Tool versions (Go 1.25.3, golangci-lint 2.8.0)
├── sqlc.yaml                    # SQLC code generation config
└── README.md                    # User documentation
```

## Code Organization & Patterns

### Service Layer Pattern
The `Service` struct in `services/claude-viewer/service.go` is the main business logic container:
```go
type Service struct {
    db               *repository.Queries   // Database queries
    viewerDir        string                // ~/.claude-viewer
    sourcePlansDir   string                // ~/.claude/plans
    indexFullContent bool                 // Whether to store full content or just title
    markdown         goldmark.Markdown    // Markdown renderer
    nowProvider      NowProvider           // Injected time provider (testable)
}
```

Key pattern: **NowProvider interface** allows injecting time in tests instead of using `time.Now()` directly.

### Database Layer (SQLC)
- **Schema**: Two tables: `plans` (indexed markdown files) and `settings` (key-value store)
- **Query definitions**: `services/claude-viewer/sqlc/queries.sql` 
- **Generated code**: Never hand-edit; regenerate with `make generate`
- **Query syntax**: Named queries with `:one`, `:many`, `:exec` directives
  - `:exec` - No return value
  - `:one` - Returns single row
  - `:many` - Returns multiple rows

### Error Handling
- Use standard Go error wrapping: `fmt.Errorf("context: %w", err)`
- Custom sentinel errors defined in `settings.go`: `ErrSettingNotFound`, `ErrInvalidDateFormat`
- HTTP handlers return JSON error responses via `ErrorResponse` struct in `cmd/http/types.go`

### Concurrency
- Plan syncing uses `golang.org/x/sync/errgroup` with concurrency limit of 5:
  ```go
  errGroup.SetLimit(5)  // services/claude-viewer/sync.go:34
  ```
- Atomic counters for tracking results: `atomic.Int64` for counting synced plans

### Markdown Processing
- Uses `github.com/yuin/goldmark` with GFM extension
- Renderer configured with `html.WithUnsafe()` for raw HTML (trusted markdown source)
- Title extraction: Scans for first `# Heading` in content

## Naming Conventions & Style

### Variables & Functions
- **Interface names**: Verb-ending for behavior (e.g., `NowProvider`)
- **Function receivers**: Single letter matching type (e.g., `func (s *Service)`)
- **Errors**: Verb-object form (`ErrInvalidDateFormat`, not `InvalidDateFormat`)
- **Constants**: SCREAMING_SNAKE_CASE for exported (e.g., `AverageReadingSpeed = 200`)
- **Unexported internals**: lowercase_snake_case for functions extracted to helpers

### Directory Organization
- SQL definitions go in `sqlc/` subdirectory (separate from generated code in `repository/`)
- Tests colocate with source: `service_test.go` next to `service.go`
- HTTP handlers grouped in `cmd/http/`; business logic in `services/`

### Import Organization (via gci formatter)
Enforced by `.golangci.yml`:
1. Standard library (`fmt`, `context`, etc.)
2. Local module imports (`github.com/Javier162380/claude-plan-viewer`)
3. Wildcard group (everything else)

## Testing Patterns

### Test Structure
Located in `services/claude-viewer/service_test.go`:
- Table-driven tests using subtests
- Mock time provider for deterministic testing:
  ```go
  type mockNowProvider struct {
      currentTime time.Time
  }
  ```
- Sample markdown constants for reuse across test cases

### Running Tests
```bash
make test                      # All tests with verbose output
go test -v ./...              # Same, direct
go test -v ./services/claude-viewer  # Single package
go test -run TestUtilityFunctions -v ./services/claude-viewer  # Single test
```

### Test Coverage Areas
- Utility functions: Word counting, reading time calculation, title extraction
- Markdown rendering: Headings, lists, code blocks, GFM tables, strikethrough
- Sync operations: File copying, plan insertion/update, concurrency
- Search: Query parsing and results

## Important Patterns & Gotchas

### 1. **SQLC Generation is Required**
After ANY change to `.sqlc/schema.sql` or `.sqlc/queries.sql`:
```bash
make generate
```
This regenerates the entire `repository/` package. Forgetting this breaks the build.

### 2. **File Paths and Permissions**
- Uses `os.MkdirAll(path, 0o750)` for directory creation (not 0o755 elsewhere)
- `nolint:gosec // G304` comments mark user-controlled paths that have been validated
- Home directory resolution: `os.UserHomeDir()` + filepath joins (not string concat)

### 3. **Concurrent Sync with Error Handling**
- `errgroup.WithContext(ctx)` ensures context cancellation propagates
- `errgroup.SetLimit(5)` prevents resource exhaustion during file I/O
- Atomic counters (`atomic.Int64`) track results from concurrent goroutines
- Missing error wrapping in group.Go() callback will silently fail—always wrap errors

### 4. **Database Schema Assumptions**
- `plans.file_name` is UNIQUE (errors if duplicate synced)
- `modified_at` index used for default sort order
- Settings table uses single-column key with typed value columns
- UPSERT pattern used for settings (INSERT ... ON CONFLICT)

### 5. **Markdown Rendering Safety**
- Markdown content assumed trusted (from local files, not user input)
- HTML rendering configured with `WithUnsafe()` to allow raw HTML in markdown
- HTTP handler marks template.HTML as safe: `//nolint:gosec // G203`

### 6. **Reading Time Calculation**
- Assumes 200 words per minute (constant `AverageReadingSpeed`)
- Returns minimum 1 minute (uses `max(1, ...)`)
- Rounds UP using `math.Ceil()`
- Custom WPM supported via `CalculateReadingTimeWithWPM()`

### 7. **HTTP Server Port**
- Default port is `:8081` (not 8080 as README shows)
- Customizable via `-addr` flag: `./bin/plan-viewer serve -addr :3000`

### 8. **Index Full Content Flag**
- `Service.indexFullContent` controls what's stored in DB:
  - `true`: Full markdown content stored (enables full-text search)
  - `false`: Only title stored (saves space, limits search)
- Currently hardcoded to `true` in both `sync` and `serve` commands

### 9. **Linting is Strict**
- 23 linters enabled in `.golangci.yml`
- `nolint` comments require explanation AND specific linter name
- Generate code paths excluded from most checks
- Build may fail without running `make generate` first

## Database Schema

### plans table
```sql
id           INTEGER PRIMARY KEY AUTOINCREMENT
file_name    TEXT NOT NULL UNIQUE          -- e.g., "my-plan.md"
file_path    TEXT NOT NULL                 -- Full path to copied file
title        TEXT NOT NULL                 -- First # heading (or "Untitled Plan")
content      TEXT NOT NULL                 -- Full markdown or title (depends on indexFullContent)
created_at   TIMESTAMP NOT NULL            -- File creation time
modified_at  TIMESTAMP NOT NULL            -- File modification time
indexed_at   TIMESTAMP NOT NULL            -- When plan was synced/updated
file_size    INTEGER NOT NULL              -- Bytes
word_count   INTEGER NOT NULL DEFAULT 0    -- Extracted word count
-- Indexes: idx_plans_modified_at (DESC), idx_plans_title
```

### settings table
```sql
variable_name  TEXT PRIMARY KEY            -- Setting key
variable_type  TEXT NOT NULL               -- Type hint: "string", "number", "boolean", "datetime"
string_value   TEXT                        -- For string settings
number_value   REAL                        -- For numeric settings
boolean_value  BOOLEAN                     -- For boolean settings
datetime_value DATETIME                    -- For timestamp settings
```

## Common Tasks for Agents

### Adding a New Field to Plans
1. Update `services/claude-viewer/sqlc/schema.sql`
2. Update `services/claude-viewer/sqlc/queries.sql` (add/modify SELECT statements)
3. Run `make generate` to regenerate `repository/` package
4. Update domain types in `services/claude-viewer/types.go` if needed
5. Update sync logic in `services/claude-viewer/sync.go`
6. Write tests in `service_test.go`
7. Run `make test` to verify

### Adding a New Search Query
1. Add to `services/claude-viewer/sqlc/queries.sql` with name and `:many` or `:one` directive
2. Run `make generate` (creates new method on `*repository.Queries`)
3. Implement business logic in `services/claude-viewer/service.go` or appropriate file
4. Add HTTP handler in `cmd/http/handlers.go` if needed
5. Test thoroughly

### Modifying the Sync Logic
1. Edit `services/claude-viewer/sync.go`
2. Remember: plans are synced concurrently with limit of 5
3. Use context properly: pass `groupCtx` to database operations
4. Update tests in `service_test.go`
5. Run `make test`

### Working with Settings
1. Settings API is in `services/claude-viewer/settings.go`
2. Must specify type when saving (string/number/boolean/datetime)
3. Retrieval returns `*Setting` with typed accessors (`IsString()`, `GetStringValue()`, etc.)
4. Use `ErrSettingNotFound` for missing settings
5. Database uses UPSERT pattern (safe to call multiple times)

## Linting & Code Quality

### Run Linting
```bash
golangci-lint run ./...
```

The project uses 23 linters with strict settings:
- Security checks (gosec, staticcheck)
- Common mistakes (errcheck, govet, ineffassign)
- Style (godot, dogsled, whitespace)
- Generated code excluded (paths with `*.gen.go` or `repository/`)

### Common Lint Fixes
- **Missing error checks**: Wrap with `if err != nil { return ... }`
- **Naked returns**: Explicitly return values in functions > 70 lines
- **Unused parameters**: Add `_` prefix or use `//nolint:unparam`
- **Format issues**: Run `gofumpt` and `gci` formatters (auto-enabled in golangci-lint)

## Dependencies

### Core Dependencies
- **github.com/labstack/echo/v4**: HTTP server framework
- **github.com/mattn/go-sqlite3**: SQLite driver
- **github.com/yuin/goldmark**: Markdown parser/renderer
- **golang.org/x/sync**: Concurrency utilities (errgroup)

### Testing
- **github.com/stretchr/testify**: Assertions and test helpers

### Code Generation
- **sqlc**: SQL-to-Go code generation (run with `make generate`)

## Development Environment

- **Go Version**: 1.25.3 (per `.tool-versions`)
- **Linter Version**: golangci-lint 2.8.0
- **OS**: Cross-platform (darwin/linux/windows via bash-compatible Makefile)

## Debugging Tips

### Enable Debug Logging
Modify `cmd/main.go` or handlers to add logging:
```go
log.Printf("Debug: %v", value)
```

### Database Inspection
Plans are stored in `~/.claude-viewer/plans.db`. Inspect with:
```bash
sqlite3 ~/.claude-viewer/plans.db "SELECT * FROM plans LIMIT 5;"
```

### Test with Sample Data
The test suite includes sample markdown constants in `service_test.go`:
- `sampleMarkdown`: Basic plan
- `sampleMarkdownUpdated`: Modified plan
- `sampleMarkdownWithCode`: Markdown with code blocks

Use these to reproduce issues in tests before modifying production code.

### HTTP Debugging
- Server logs start message to stdout: `Server running on http://localhost:8081`
- All handlers pass context from request: `c.Request().Context()`
- JSON responses include error details in `ErrorResponse` struct

## Key Files Quick Reference

| File | Purpose |
|------|---------|
| `cmd/main.go` | Entry point, command routing |
| `services/claude-viewer/service.go` | Service initialization, core type |
| `services/claude-viewer/sync.go` | Plan syncing and file handling |
| `services/claude-viewer/search.go` | Search implementation |
| `services/claude-viewer/types.go` | Domain types (PlanSummary, PlanDetail) |
| `services/claude-viewer/service_test.go` | Test suite (all unit tests) |
| `services/claude-viewer/sqlc/schema.sql` | Database schema definition |
| `services/claude-viewer/sqlc/queries.sql` | SQL query definitions |
| `cmd/http/handlers.go` | HTTP route handlers |
| `cmd/http/types.go` | HTTP request/response types |
| `internal/storage/db.go` | SQLite connection setup |
| `Makefile` | Build automation |
| `.golangci.yml` | Linting rules |


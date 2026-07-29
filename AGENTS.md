# AGENTS.md - Claude Plan Viewer Development Guide

Quick reference for understanding the codebase architecture and development workflow.

## Project Overview

Go application that syncs Claude AI plans from `~/.claude/plans/` to a searchable database, providing TUI, CLI, and MCP interfaces.

**Core Features**: Plan sync/search, version tracking, multi-database support (SQLite/PostgreSQL), connector system, markdown rendering, MCP integration

**Main Commands**: `sync`, `dump`, `tui`, `mcp`, `migrate`

**Tag model**: Tags are DB-only — created/assigned via `SetPlanTags`/`CreateTag`, never read from or written to plan file content. Sync (`SyncPlans`) does not extract or overwrite tags.

## Essential Workflow

```bash
make build       # Build binary
make generate    # Regenerate SQLC code (after SQL changes)
make lint        # Lint the project according to GolangCI lint best practices.
make test        # Run tests
make sync        # Sync plans
make tui         # Launch terminal UI
```

**Development Cycle**:
1. Modify SQL → `make generate` → update DTO/repository → add converter in `convert.go` → update service method return type → tests
2. Never edit SQLC-generated files (queries.sql.go, models.go, db.go)
3. Always run tests before committing
4. Always lint your code before committing

## Architecture Patterns

### Repository Pattern (Multi-Database)

Two conversion boundaries are enforced. Repository implementations convert SQLC types to `dto.*` internally. The service layer then converts `dto.*` to its own public types (`Tag`, `Comment`, etc.) in `convert.go` before returning them through `UnifiedService`. `cmd/*` adapters never import `dto` directly.

```
cmd/{tui,mcp}        (claudeviewer.Tag, claudeviewer.Comment, …)
    ↑
Service Layer        (convert.go: dto.* → service types)
    ↑
dto.Repository       (dto.Tag, dto.Comment, …)
    ↑
SQLite / Postgres    (sql.Null* / pgtype.* → dto)
    ↑
SQLC-generated code
```

**Key Principle**: `cmd/main.go` is the only file in `cmd/` permitted to import `dto` (composition-root wiring only).

### Connector System (Plugin Architecture)

Extensible notification system for sending plans to external services.

```
Service → ConnectorManager → Registry + SettingGetter
              ↓
         Connector Interface (Name, Send, Validate, RequiredSettings)
              ↓
    ConfigurableConnector (LoadConfig for dynamic settings)
```

**Implementations**: Telegram connector stores settings in database, loaded via `SettingGetter`.

### Dynamic SQL (go-sqlbuilder)

Use `github.com/huandu/go-sqlbuilder` only when the query shape is determined at runtime and SQLC static queries can't cover it — dynamic `ORDER BY` column/direction is the canonical case. Use the correct dialect (`sqlbuilder.SQLite` / `sqlbuilder.PostgreSQL`) and pair it with a shared scan helper rather than repeating row scanning inline. Everything else stays SQLC.

### Summary Cache Invalidation

`summaryCache` is keyed by filename. Both `UpdatePlan` and `SavePlanLocal` call `summaryCache.Delete(req.FileName)` after a successful write. Any new path that modifies plan content must do the same.

### MCP Integration (Model Context Protocol)

Adapter layer for Claude Code AI integration, following clean architecture principles.

```
Service Layer (claudeviewer.Service)
    ↓
MCP Handler (cmd/mcp) → TOON Formatter → MCP Tools
    ↓
Tools: search_plans, get_plan, list_tools
```

**Key Features**: Uses TOON format for 60% token reduction, no business logic in handlers, interface-based service dependency for testability.

### Service Layer

Central orchestrator with injected dependencies:
- `db dto.Repository` - database operations
- `connectorManager *connectors.Manager` - notifications
- `nowProvider NowProvider` - testable time injection
- `markdown goldmark.Markdown` - rendering

## Project Structure

```
cmd/                        # Entry points
  ├── main.go               # Command routing
  ├── mcp/                  # MCP server (Claude integration)
  └── tui/                  # Terminal UI (Bubble Tea)

services/claude-viewer/     # Business logic
  ├── dto/                  # Clean domain types + Repository interface
  ├── repository/           # Database implementations
  │   ├── sqlite/           # SQLite + SQLC-generated
  │   └── postgres/         # PostgreSQL + SQLC-generated
  └── sqlc/                 # SQL schemas and queries

internal/
  ├── config/               # TOML config + env vars
  ├── connectors/           # Connector system + implementations
  └── storage/              # DB setup + migrations
```

## Code Conventions

**DTO Types** (dto/models.go):
- Clean Go types with no database-specific nullables
- Example: `Plan` has `time.Time`, not `sql.NullTime`

**Repository Interface** (dto/repository.go):
- 45+ methods for all database operations
- Implementations in repository/sqlite and repository/postgres
- Both implement exact same interface

**Repository Implementations**:
- Handle SQLC type conversions internally
- Helper functions: `nullStringToPtr`, `ptrToNullString`, etc.
- Always return/accept DTO types

**Testing**:
- Main file: `services/claude-viewer/service_test.go`
- Pattern: Root test with subtests (`t.Run()`)
- Uses gomock for connectors, mock time provider
- Tests are parametrized via `registeredBackends` and run against both SQLite and Postgres
- SQLite always runs; Postgres runs only when `INTEGRATION=1` (spins up a real container via testcontainers-go)
- Each Postgres test case gets its own isolated database created/dropped inside the shared container
- Run integration tests: `INTEGRATION=1 go test -v ./services/claude-viewer/...`

## Common Tasks

**Adding Database Field**:
1. Update `sqlc/{sqlite,postgres}/schema.sql`
2. Update `sqlc/{sqlite,postgres}/queries.sql` if needed
3. `make generate`
4. Add to `dto/models.go`
5. Update repository converters
6. Write tests

**Adding Connector**:
1. Create `internal/connectors/myconnector/`
2. Implement `Connector` (+ `ConfigurableConnector` if settings needed)
3. Register in `cmd/main.go`
4. Settings stored in database via `Manager`

**Switching Database**:
- Config: `backend = "postgres"` or env `PLAN_VIEWER_DB_BACKEND=postgres`
- Run `make migrate`

## Configuration

**File** (plan-viewer.toml):
```toml
[database]
backend = "sqlite"  # or "postgres"

[database.sqlite]
path = "/Users/yourname/.claude-viewer/plans.db"  # `~` is NOT expanded

[paths]
viewer_dir = "/Users/yourname/.claude-viewer"

# One block per source directory; `label` defaults to the dir's base name and
# is slugified to name the source's mirror dir under viewer_dir.
[[paths.plans_dirs]]
  path  = "/Users/yourname/.claude/plans"
  label = "claudeRoot"

[mcp]
server_name = "claude-plan-viewer"
version = "1.0.0"
```

**Environment Overrides**: `PLAN_VIEWER_DB_BACKEND`, `PLAN_VIEWER_POSTGRES_URL`, `DEBUG=1` (TUI)

## Database Schema

**Core Tables**: `plans`, `plan_versions`, `tags`, `plan_tags`, `settings`, `connectors`, `connector_settings`

**Schema Files**: `services/claude-viewer/sqlc/{sqlite,postgres}/schema.sql`

**Migrations**: Automatic via goose in `internal/storage/migrations/`

## Critical Rules

1. **Never edit SQLC-generated files** - Regenerated by `make generate`
2. **cmd/* layers only use service-layer types** - Never import `services/claude-viewer/dto`; `dto.*` is converted in `convert.go`
3. **Repository converts at boundaries** - Database types → DTO types
4. **Connector settings in database** - Not in config files
5. **Tests are comprehensive** - ~2000 lines with subtests pattern
6. **Time is injectable** - Use `nowProvider` for testability
7. **WatchManager locking** - In `Stop()`, read `wm.running` directly under the write lock; never call `IsRunning()` from within a method that already holds `wm.mu` (deadlock)
8. **Tags are DB-only** - Tags live exclusively in the database. `SyncPlans` uses `InsertPlan`/`UpdatePlan` (no tag params). Never add tag extraction to sync, and never write tags back to markdown files. `DeleteTag` only deletes the DB row — no file I/O.
9. **One TUI component, one responsibility** - a component under `cmd/tui/components/` serves exactly one domain concept. When a feature looks like "an existing component plus a mode flag", build a separate component: its own type, its own `types.Focus` value, its own key handler on the screen, and its own messages. Never widen a component to serve two concepts via a `SetXEntries` variant, a display-mode branch, or a sentinel value - the branching does not stay in the component, it spreads into the screen's key handling, focus cycling, and rendering, and messages belonging to one concept end up mutating the other's state. `TagPanel` (DB-backed, async, mutable) and `LabelPanel` (read-only projection of `PlanSummary.SyncLabel`) are the reference split; duplicated row rendering between them is the accepted cost.
10. **Keep the help screen in sync with keybindings** - Whenever you add, remove, or change a TUI keybinding (in a screen's key handler under `cmd/tui/screens/*.go` or a modal's `Update` in `cmd/tui/components/*.go`), update BOTH the affected screen's `ShortHelp()` string AND the `helpContent` block in `cmd/tui/screens/help.go`, under the matching context section. `helpContent` is the full reference users open with `?`; a binding that isn't listed there is effectively undiscoverable.

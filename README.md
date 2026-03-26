# Claude Plan Viewer

A Go web application to index, search, and view Claude Code plans.

## Features

- Sync plans from `~/.claude/plans/` to `~/.claude-viewer/`
- SQLite database with full-text search
- Clean web interface for browsing and searching
- Markdown rendering with syntax highlighting

## Installation

```bash
go mod download
make build
```

## Usage

### Sync plans
```bash
make sync
# or
./bin/plan-viewer sync
```

### Start web server
```bash
make serve
# or
./bin/plan-viewer serve
# or with custom port
./bin/plan-viewer serve -addr :3000
```

Then open http://localhost:8080 in your browser.

## Development

### Generate SQLC code
```bash
make generate
```

### Run tests
```bash
make test
```

### Project structure
- `cmd/` - CLI entry point and HTTP handlers
- `services/claude-viewer/` - Core business logic
- `internal/storage/` - Database connection
- `cmd/http/templates/` - HTML templates

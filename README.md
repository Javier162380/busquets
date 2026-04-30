# Claude Plan Viewer

A powerful Go application for indexing, searching, and viewing [Claude Code](https://claude.com/claude-code) plan files with multiple interfaces (Web UI, TUI, CLI).

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Sync Plans](#sync-plans)
  - [Web Server](#web-server)
  - [Terminal UI (TUI)](#terminal-ui-tui)
  - [MCP Server](#mcp-server)
  - [Database Migrations](#database-migrations)
- [Architecture](#architecture)
- [Development](#development)

## Overview

Claude Plan Viewer provides a centralized solution for managing Claude Code plan files. It syncs plans from Claude's native storage location, indexes them in a searchable database, and offers multiple ways to browse and search your plans efficiently.

**Why use this?**
- **Centralized Management**: Keep all your Claude Code plans organized and searchable in one place
- **Full-Text Search**: Quickly find plans by content, not just filename
- **Version Tracking**: Track changes to plans over time
- **Multiple Interfaces**: Choose between web UI, terminal UI, or CLI based on your workflow
- **Flexible Storage**: Use SQLite for simplicity or PostgreSQL for scale

## Features

### Core Functionality
- **Automatic Synchronization**: Sync plans from `~/.claude/plans/` to `~/.claude-viewer/`
- **Bidirectional Sync**: `rsync` command for syncing changes back to source
- **Full-Text Search**: Fast content search with SQLite FTS5 or PostgreSQL text search
- **Version Control**: Track multiple versions of plans with diff viewing
- **File Watching**: Automatic background synchronization at configurable intervals

### User Interfaces
- **Web Interface**: Clean, responsive UI with markdown rendering and syntax highlighting
- **Terminal UI (TUI)**: Feature-rich terminal interface for command-line enthusiasts
- **MCP Server**: Model Context Protocol integration for Claude Code AI assistant
- **CLI Commands**: Direct command-line operations for scripting and automation

### Database Support
- **SQLite**: Zero-configuration, file-based database (default)
- **PostgreSQL**: Production-ready relational database with advanced features
- **Automatic Migrations**: Database schema managed automatically

### Additional Features
- **Background Job Execution**: Execute plans automatically using Claude Code CLI
  - Schedule plan execution for future times (one-time scheduling)
  - Trigger manual execution from the TUI
  - Track execution history with detailed logs
  - Concurrent execution with configurable worker pool
  - Real-time status updates and cancellation support
- **Connector System**: Extensible notification system (includes Telegram integration)
- **Markdown Rendering**: Beautiful plan display with code syntax highlighting
- **Pagination**: Efficient browsing of large plan collections
- **HTMX Integration**: Fast, dynamic web interface without full page reloads

## Prerequisites

- **Go 1.26+** for building from source
- **SQLC** (optional, for development): `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- **PostgreSQL** (optional): If using PostgreSQL backend instead of SQLite

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/Javier162380/claude-plan-viewer.git
cd claude-plan-viewer

# Download dependencies
go mod download

# Build the application
make build
```

The binary will be created at `./bin/plan-viewer`.

### Quick Start

```bash
# Run initial sync to import your plans
make sync

# Start the web server
make serve
# Open http://localhost:8081 in your browser

# Or start the TUI
make tui
```

## Configuration

### Configuration File

Create a `plan-viewer.toml` file in your working directory to customize settings. All configuration is optional with sensible defaults.

```bash
# Copy the example configuration
cp plan-viewer.toml.example plan-viewer.toml
```

### Configuration Options

#### Database Configuration

**SQLite (Default)**
```toml
[database]
backend = "sqlite"

[database.sqlite]
path = "/Users/yourname/.claude-viewer/plans.db"
```

**PostgreSQL**
```toml
[database]
backend = "postgres"

[database.postgres]
connection_string = "postgres://user:password@localhost:5432/claude_plans?sslmode=disable"
max_open_conns = 10
max_idle_conns = 5
```

#### Path Configuration

```toml
[paths]
# Where plan-viewer stores indexed data
viewer_dir = "/Users/yourname/.claude-viewer"

# Source directory containing Claude plan files
plans_dir = "/Users/yourname/.claude/plans"
```

#### Background Jobs Configuration

```toml
[background_jobs]
# Enable background job execution (default: true)
enabled = true

# Maximum number of concurrent job executions (default: 3)
max_concurrent = 3

# Default timeout for job execution in minutes (default: 30)
default_timeout_minutes = 30

# How often to check for scheduled jobs in seconds (default: 30)
polling_interval_seconds = 30
```

**Note**: Background jobs allow you to execute plans automatically using Claude Code CLI. Jobs can be triggered manually from the TUI or scheduled for future execution.

### Environment Variables

Override any configuration with environment variables:

```bash
# Database configuration
export PLAN_VIEWER_DB_BACKEND="postgres"
export PLAN_VIEWER_POSTGRES_URL="postgres://user:pass@host:5432/db"
export PLAN_VIEWER_SQLITE_PATH="/path/to/plans.db"

# Path configuration
export PLAN_VIEWER_DIR="/path/to/viewer-dir"
export PLAN_VIEWER_PLANS_DIR="/path/to/claude/plans"

# TUI debug mode
export DEBUG=1
```

### Default Configuration

Without a config file, plan-viewer uses these defaults:

- **Database**: SQLite at `~/.claude-viewer/plans.db`
- **Viewer Directory**: `~/.claude-viewer/`
- **Plans Source**: `~/.claude/plans/`

## Usage

### Sync Plans

Copy and index plans from Claude's storage to the viewer database:

```bash
# Using Make
make sync

# Using binary directly
./bin/plan-viewer sync

# Using rsync (bidirectional sync)
make rsync
./bin/plan-viewer rsync
```

**Output:**
```
✓ Synced 42 plans from /Users/you/.claude/plans to /Users/you/.claude-viewer (backend: sqlite)
```

### Web Server

Start the HTTP server for web-based browsing:

```bash
# Default port (8081)
make serve
./bin/plan-viewer serve

# Custom port
./bin/plan-viewer serve -addr :3000

# Custom port with environment override
./bin/plan-viewer serve -addr :9000
```

**Features:**
- Full-text search across all plans
- Markdown rendering with syntax highlighting
- Version comparison with diff view
- Pagination for large collections
- HTMX-powered dynamic updates
- Responsive design

**Access:** Open `http://localhost:8081` in your browser.

### Terminal UI (TUI)

Launch the interactive terminal interface:

```bash
# Standard mode
make tui
./bin/plan-viewer tui

# Debug mode (logs to ~/.claude-viewer/tui-debug.log)
make tui-debug
DEBUG=1 ./bin/plan-viewer tui
```

**TUI Features:**
- Browse plans with keyboard navigation
- Real-time search
- Version viewing
- Settings configuration
- Connector management (Telegram notifications)
- Watch mode with automatic sync
- Background job execution and management
  - Trigger plan execution manually
  - Schedule jobs for future execution
  - View execution history and logs
  - Cancel running or scheduled jobs

**Keyboard Shortcuts:**
- Press `?` in the TUI for help
- Press `Ctrl+J` to access the Jobs screen

### MCP Server

Enable Claude Code to directly search and retrieve your plans during coding sessions:

```bash
# Start MCP server
./bin/plan-viewer mcp
```

**Configure Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "claude-plans": {
      "command": "/path/to/bin/plan-viewer",
      "args": ["mcp"]
    }
  }
}
```

**MCP Features:**
- `search_plans` - Search by text and/or tags with flexible filtering
- `get_plan` - Retrieve full plan content and metadata
- `list_tools` - Discover available MCP capabilities
- TOON format responses (60% fewer tokens than JSON)

**Use Cases:**
- "Search my plans for authentication patterns"
- "Show me the backend-api plan from last week"
- "Find all plans tagged with #refactoring"

### Database Migrations

Run database migrations manually:

```bash
make migrate
./bin/plan-viewer migrate
```

This is typically only needed when:
- Setting up a new database
- Switching database backends
- After upgrading to a new version with schema changes

## Architecture

### Project Structure

```
claude-plan-viewer/
├── cmd/                        # Application entry points
│   ├── http/                   # Web server and handlers
│   │   ├── templates/          # HTML templates
│   │   └── static/             # CSS, JS assets
│   ├── mcp/                    # MCP server (Claude integration)
│   └── tui/                    # Terminal UI
│       ├── screens/            # TUI screens
│       ├── components/         # Reusable UI components
│       ├── content/            # Content models
│       └── styles/             # Theme and styling
├── services/claude-viewer/     # Core business logic
│   ├── dto/                    # Data transfer objects
│   └── repository/             # Data access layer
│       ├── sqlite/             # SQLite implementation
│       └── postgres/           # PostgreSQL implementation
└── internal/                   # Private application code
    ├── config/                 # Configuration management
    ├── storage/                # Database setup & migrations
    └── connectors/             # Notification connectors
        └── telegram/           # Telegram integration
```

### Key Components

**Service Layer** (`services/claude-viewer/`)
- Orchestrates business logic
- Manages plan synchronization and versioning
- Provides repository abstraction

**Repository Pattern** (`services/claude-viewer/repository/`)
- Abstract interface for data access
- SQLite and PostgreSQL implementations
- Generated code via SQLC for type safety

**Configuration** (`internal/config/`)
- TOML-based configuration
- Environment variable overrides
- Validation and defaults

**Web Server** (`cmd/http/`)
- Echo framework for routing
- HTMX for dynamic interactions
- Server-side rendering with Go templates

**Terminal UI** (`cmd/tui/`)
- Built with Bubble Tea framework
- Component-based architecture
- Keyboard-driven navigation

**Connectors** (`internal/connectors/`)
- Extensible notification system
- Plugin-style architecture
- Currently supports Telegram

## Development

### Prerequisites

Install development tools:

```bash
# SQLC for generating type-safe database code
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### Common Commands

```bash
# Download dependencies
make deps

# Generate SQLC code (after modifying queries)
make generate

# Run tests
make test

# Build the application
make build

# Clean build artifacts
make clean

# View all available commands
make help
```

### Adding Database Queries

1. Edit SQL queries in `services/claude-viewer/repository/{sqlite|postgres}/queries.sql`
2. Run `make generate` to regenerate Go code
3. Use the generated code in your repository implementation

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -cover ./...

# Run specific package tests
go test -v ./services/claude-viewer/...
```

### Code Organization

- **Follow Go conventions**: Use standard project layout
- **Repository pattern**: Keep database logic isolated
- **DTOs for boundaries**: Use data transfer objects between layers
- **Interfaces for flexibility**: Allow easy testing and swapping implementations
- **Error handling**: Always wrap errors with context

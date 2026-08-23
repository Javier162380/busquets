# Busquets

A powerful Go application for indexing, searching, and viewing LLM/AI-assistant plan files with multiple interfaces (TUI, CLI, MCP). Works with any assistant that writes plan files to disk — [Claude Code](https://claude.com/claude-code)'s `~/.claude/plans/` is the built-in default source, and any other directory can be added via configuration.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Sync Plans](#sync-plans)
  - [Dump Plans](#dump-plans)
  - [Terminal UI (TUI)](#terminal-ui-tui)
  - [MCP Server](#mcp-server)
  - [Database Migrations](#database-migrations)
- [Architecture](#architecture)
- [Development](#development)

## Overview

Busquets provides a centralized solution for managing LLM/AI-assistant plan files. It syncs plans from one or more source directories, indexes them in a searchable database, and offers multiple ways to browse and search your plans efficiently.

**Why use this?**
- **Centralized Management**: Keep all your plans organized and searchable in one place
- **Full-Text Search**: Quickly find plans by content, not just filename
- **Version Tracking**: Track changes to plans over time
- **Multiple Interfaces**: Choose between terminal UI, CLI, or MCP based on your workflow
- **Flexible Storage**: Use SQLite for simplicity or PostgreSQL for scale

## Features

### Core Functionality
- **Automatic Synchronization**: Sync plans from configured source directories (e.g. `~/.claude/plans/` for Claude Code) to `~/.busquets/`
- **Bidirectional Sync**: `rsync` command for syncing changes back to source
- **Database Dump**: `dump` command to restore plans from the database back to their source directory
- **Full-Text Search**: Fast content search with SQLite FTS5 or PostgreSQL text search
- **Version Control**: Track multiple versions of plans with diff viewing
- **File Watching**: Automatic background synchronization at configurable intervals
- **Tag Management**: Organize plans with custom tags — created and assigned via the UI or API, stored entirely in the database (not embedded in plan files)

### User Interfaces
- **Terminal UI (TUI)**: Feature-rich terminal interface for command-line enthusiasts
- **MCP Server**: Model Context Protocol integration for MCP-capable AI assistants.
- **CLI Commands**: Direct command-line operations for scripting and automation

### Database Support
- **SQLite**: Zero-configuration, file-based database (default)
- **PostgreSQL**: Production-ready relational database with advanced features
- **Automatic Migrations**: Database schema managed automatically

### Additional Features
- **Connector System**: Extensible plugin system with two roles — a *transmit* connector for sending plans out (Telegram) and a *summary* connector for generating plan TL;DRs (Ollama, against a local LLM)
- **Markdown Rendering**: Beautiful plan display with code syntax highlighting and a selectable colour theme
- **Pagination**: Efficient browsing of large plan collections

## Prerequisites

- **Go 1.26+** for building from source
- **SQLC** (optional, for development): `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- **PostgreSQL** (optional): If using PostgreSQL backend instead of SQLite

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/Javier162380/busquets.git
cd busquets

# Download dependencies
go mod download

# Build the application
make build
```

The binary will be created at `./bin/busquets`.

### Quick Start

```bash
# Run initial sync to import your plans
make sync

# Start the TUI
make tui
```

## Configuration

### Configuration File

Create a `busquets.toml` file in your working directory to customize settings. All configuration is optional with sensible defaults.

```bash
# Copy the example configuration
cp busquets.toml.example busquets.toml
```

### Configuration Options

#### Database Configuration

**SQLite (Default)**
```toml
[database]
backend = "sqlite"

[database.sqlite]
path = "/Users/yourname/.busquets/plans.db"
```

**PostgreSQL**
```toml
[database]
backend = "postgres"

[database.postgres]
connection_string = "postgres://user:password@localhost:5432/busquets_plans?sslmode=disable"
max_open_conns = 10
max_idle_conns = 5
```

#### Path Configuration

```toml
[paths]
# Where busquets stores indexed data
viewer_dir = "/Users/yourname/.busquets"

# Source directories containing LLM/AI-assistant plan files — one block per
# source. `label` is optional and defaults to the directory's base name.
[[paths.plans_dirs]]
  path  = "/Users/yourname/.claude/plans"
  label = "claudeRoot"

[[paths.plans_dirs]]
  path  = "/Users/yourname/work/some-repo/docs/plans"
  label = "workRepo"
```

Labels are slugified (lowercased, keeping only `[a-z0-9-_]`) to name each
source's mirror directory under `viewer_dir`. Two sources may not share a
path or resolve to the same slug — busquets refuses to start if they do.

> **Note:** paths are used verbatim. `~` is **not** expanded — always write
> the full path.

### Environment Variables

Override any configuration with environment variables:

```bash
# Database configuration
export PLAN_VIEWER_DB_BACKEND="postgres"
export PLAN_VIEWER_POSTGRES_URL="postgres://user:pass@host:5432/db"
export PLAN_VIEWER_SQLITE_PATH="/path/to/plans.db"

# Path configuration
export PLAN_VIEWER_DIR="/path/to/viewer-dir"

# TUI debug mode
export DEBUG=1
```

### Default Configuration

Without a config file, busquets uses these defaults:

- **Database**: SQLite at `~/.busquets/plans.db`
- **Viewer Directory**: `~/.busquets/`
- **Plans Source**: `~/.claude/plans/` (Claude Code's own default location; add more via `[[paths.plans_dirs]]`)

## Usage

### Sync Plans

Copy and index plans from the configured source directories to the viewer database:

```bash
# Using Make
make sync

# Using binary directly
./bin/busquets sync

# Using rsync (bidirectional sync)
make rsync
./bin/busquets rsync
```

**Output:**
```
✓ Synced 42 plans from /Users/you/.claude/plans to /Users/you/.busquets (backend: sqlite)
```

### Dump Plans

Write all plans stored in the database back to their source directory (e.g. `~/.claude/plans/`). Useful when the source directory has been lost or you've switched databases and want to restore plan files to disk:

```bash
./bin/busquets dump
```

**Output:**
```
✓ Dumped 42 plans from database to /Users/you/.claude/plans (backend: sqlite)
```

In the TUI, press `d` from the plans list to trigger a dump.

### Terminal UI (TUI)

Launch the interactive terminal interface:

```bash
# Standard mode
make tui
./bin/busquets tui

# Debug mode (logs to ~/.busquets/tui-debug.log)
make tui-debug
DEBUG=1 ./bin/busquets tui
```

**TUI Features:**
- Browse plans with keyboard navigation
- Real-time search
- Version viewing
- Settings configuration
- Connector management (Telegram notifications, Ollama summaries)
- Watch mode with automatic sync

**Keyboard Shortcuts:** Press `?` in the TUI for help.

#### Markdown Rendering

Press `r` in the content pane to switch between raw markdown and rendered
markdown, or set `render_markdown_by_default` in the settings screen to start in
rendered mode.

The `markdown_theme` setting picks the colour theme used for rendered markdown —
`dark`, `light`, `tokyo-night` (default), `ascii`, `dracula`, or `pink`. It
applies to plan content, version content, comments, and TL;DR summaries. The
theme only affects rendered markdown, so it has no visible effect while the
viewer is in raw mode.

### MCP Server

Enable Claude Code — or any other MCP-capable AI assistant — to directly search and retrieve your plans during coding sessions:

```bash
# Start MCP server
./bin/busquets mcp
```

**Configure Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "claude-plans": {
      "command": "/path/to/bin/busquets",
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
- "Find all plans tagged with refactoring"

### Tags

Tags are managed exclusively through the database — they are never read from or written to plan file content.

Create tags and assign them to plans using the TUI or via the `SetPlanTags` service API. Tags persist independently of plan files, so syncing a plan never overwrites its tags.

To search plans by tag via MCP:
```
"Find all plans tagged with refactoring"
"Show me plans tagged with both api and backend"
```

### Database Migrations

Run database migrations manually:

```bash
make migrate
./bin/busquets migrate
```

This is typically only needed when:
- Setting up a new database
- Switching database backends
- After upgrading to a new version with schema changes

## Architecture

### Project Structure

```
busquets/
├── cmd/                        # Application entry points
│   ├── mcp/                    # MCP server (AI assistant integration)
│   └── tui/                    # Terminal UI
│       ├── screens/            # TUI screens
│       ├── components/         # Reusable UI components
│       ├── content/            # Content models
│       └── styles/             # Theme and styling
├── services/busquets/          # Core business logic
│   ├── dto/                    # Data transfer objects
│   └── repository/             # Data access layer
│       ├── sqlite/             # SQLite implementation
│       └── postgres/           # PostgreSQL implementation
└── internal/                   # Private application code
    ├── config/                 # Configuration management
    ├── storage/                # Database setup & migrations
    └── connectors/             # Notification connectors
        ├── ollama/             # Local LLM summaries
        └── telegram/           # Telegram integration
```

### Key Components

**Service Layer** (`services/busquets/`)
- Orchestrates business logic
- Manages plan synchronization and versioning
- Provides repository abstraction

**Repository Pattern** (`services/busquets/repository/`)
- Abstract interface for data access
- SQLite and PostgreSQL implementations
- Generated code via SQLC for type safety

**Configuration** (`internal/config/`)
- TOML-based configuration
- Environment variable overrides
- Validation and defaults

**Terminal UI** (`cmd/tui/`)
- Built with Bubble Tea framework
- Component-based architecture
- Keyboard-driven navigation

**Connectors** (`internal/connectors/`)
- Extensible notification system
- Plugin-style architecture
- Telegram (transmit role) and Ollama (summary role)

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

1. Edit SQL queries in `services/busquets/repository/{sqlite|postgres}/queries.sql`
2. Run `make generate` to regenerate Go code
3. Use the generated code in your repository implementation

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -cover ./...

# Run specific package tests
go test -v ./services/busquets/...
```

### Code Organization

- **Follow Go conventions**: Use standard project layout
- **Repository pattern**: Keep database logic isolated
- **DTOs for boundaries**: Use data transfer objects between layers
- **Interfaces for flexibility**: Allow easy testing and swapping implementations
- **Error handling**: Always wrap errors with context

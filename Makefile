.PHONY: help build run sync tui tui-debug test generate clean migrate migrate-dir

help: ## Show this help
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: ## Generate SQLC code
	@echo "Generating SQLC code..."
	sqlc generate

build: generate ## Build the application
	@echo "Building busquets..."
	go build -o bin/busquets cmd/main.go
	@echo "Build complete: bin/busquets"

sync: build ## Build and run sync command
	./bin/busquets sync

rsync: build
	./bin/busquets rsync

tui: build ## Build and run TUI
	./bin/busquets tui

tui-debug: build ## Build and run TUI in debug mode (logs to ~/.busquets/tui-debug.log)
	DEBUG=1 ./bin/busquets tui

migrate: build ## Run DB schema migrations + migrate plan storage layout (run before tui after upgrading)
	./bin/busquets migrate

migrate-dir: build ## Migrate ~/.claude-viewer → ~/.busquets (dir rename + DB file_path fix-up); run this once before starting the app after upgrading to Busquets
	./bin/busquets migrate

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-integration: ## Run integration tests (SQLite + Postgres, requires Docker)
	@echo "Running integration tests..."
	TESTCONTAINERS_RYUK_DISABLED=true INTEGRATION=1 go test -count=1 -v ./services/busquets/...

lint: ## Lint project
	 @echo "Linting project..."
	 golangci-lint run -c .golangci.yml

clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf services/busquets/repository/

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

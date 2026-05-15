.PHONY: help build run sync serve tui tui-debug test generate clean migrate

help: ## Show this help
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: ## Generate SQLC code
	@echo "Generating SQLC code..."
	sqlc generate

build: generate ## Build the application
	@echo "Building plan-viewer..."
	go build -o bin/plan-viewer cmd/main.go
	@echo "Build complete: bin/plan-viewer"

sync: build ## Build and run sync command
	./bin/plan-viewer sync

rsync: build
	./bin/plan-viewer rsync

serve: build ## Build and run web server
	./bin/plan-viewer serve

tui: build ## Build and run TUI
	./bin/plan-viewer tui

tui-debug: build ## Build and run TUI in debug mode (logs to ~/.claude-viewer/tui-debug.log)
	DEBUG=1 ./bin/plan-viewer tui

migrate: build ## Run database migrations
	./bin/plan-viewer migrate

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-integration: ## Run integration tests (SQLite + Postgres, requires Docker)
	@echo "Running integration tests..."
	TESTCONTAINERS_RYUK_DISABLED=true INTEGRATION=1 go test -count=1 -v ./services/claude-viewer/...

lint: ## Lint project
	 @echo "Linting project..."
	 golangci-lint run -c .golangci.yml

clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf services/claude-viewer/repository/

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

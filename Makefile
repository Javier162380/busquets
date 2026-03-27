.PHONY: help build run sync serve tui test generate clean

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

serve: build ## Build and run web server
	./bin/plan-viewer serve

tui: build ## Build and run TUI
	./bin/plan-viewer tui

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf services/claude-viewer/repository/

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

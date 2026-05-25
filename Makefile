DOMAIN ?= domain.yaml
BRIDGE ?= bridges/csharp
OUTPUT ?= generated
BRIDGE_NAME ?= csharp
SPEC_OUTPUT ?= spec/domain.schema.json
GO_CACHE_DIR ?= $(CURDIR)/.gocache
GO_TMP_DIR ?= $(CURDIR)/bin

export GOCACHE := $(GO_CACHE_DIR)
export GOTMPDIR := $(GO_TMP_DIR)

.PHONY: help build install run test test-verbose test-coverage lint fmt clean install-deps cli-validate cli-generate cli-new cli-bridges regenerate-spec generate-gui-types

help:
	@echo "DomainCraft CLI - Available Commands"
	@echo ""
	@echo "  make build           - Build the binary to bin/domaincraft"
	@echo "  make install         - Build and install to /usr/local/bin (or ~/.local/bin)"
	@echo "  make run             - Build and run with example domain.yaml"
	@echo "  make cli-new         - Run 'new' wizard (uses DOMAIN=$(DOMAIN))"
	@echo "  make cli-validate    - Run 'validate' command (uses DOMAIN=$(DOMAIN))"
	@echo "  make cli-generate    - Run 'generate' command (uses DOMAIN=$(DOMAIN) BRIDGE=$(BRIDGE) OUTPUT=$(OUTPUT))"
	@echo "  make cli-bridges     - List available bridges"
	@echo "  make regenerate-spec - Regenerate spec/domain.schema.json and GUI TypeScript types"
	@echo "  make generate-gui-types - Regenerate only GUI TypeScript types from schema"
	@echo "  make test            - Run all tests"
	@echo "  make test-verbose    - Run tests with verbose output"
	@echo "  make test-coverage   - Run tests and generate coverage report"
	@echo "  make lint            - Run linter (go vet)"
	@echo "  make fmt             - Format code (gofmt)"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make install-deps    - Install dependencies"

install-deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

build: install-deps
	@echo "Building domaincraft..."
	@go build -o bin/domaincraft ./cmd/domaincraft

install: build
	@echo "Installing domaincraft..."
	@if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then \
		cp bin/domaincraft /usr/local/bin/domaincraft; \
		echo "Installed to /usr/local/bin/domaincraft"; \
	elif [ -d "$$HOME/.local/bin" ]; then \
		mkdir -p "$$HOME/.local/bin"; \
		cp bin/domaincraft "$$HOME/.local/bin/domaincraft"; \
		echo "Installed to $$HOME/.local/bin/domaincraft"; \
	else \
		mkdir -p "$$HOME/.local/bin"; \
		cp bin/domaincraft "$$HOME/.local/bin/domaincraft"; \
		echo "Installed to $$HOME/.local/bin/domaincraft"; \
		echo "Make sure ~/.local/bin is in your PATH"; \
	fi

run: build
	@echo "Running domaincraft with example domain.yaml..."
	@./bin/domaincraft

# Convenience targets for development (via go run)
cli-new:
	@go run ./cmd/domaincraft new

cli-validate:
	@go run ./cmd/domaincraft validate --domain $(DOMAIN)

cli-generate:
	@go run ./cmd/domaincraft generate --domain $(DOMAIN) --bridge $(BRIDGE) --output $(OUTPUT)

cli-bridges:
	@go run ./cmd/domaincraft bridges

regenerate-spec:
	@echo "Running: go run ./cmd/schema-gen -o $(SPEC_OUTPUT)"
	@go run ./cmd/schema-gen -o $(SPEC_OUTPUT)
	@echo "Generating TypeScript types for GUI..."
	@cd ../DomainCraftGui && npm run generate:types

generate-gui-types:
	@echo "Generating TypeScript types for GUI..."
	@cd ../DomainCraftGui && npm run generate:types

test: install-deps
	@echo "Running tests..."
	@go test ./...

test-verbose: install-deps
	@echo "Running tests (verbose)..."
	@go test -v ./...

test-coverage: install-deps
	@echo "Running tests with coverage..."
	@go test -cover ./...
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linter..."
	@go vet ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html

# Development targets
dev-watch:
	@echo "Watching for changes (requires entr)..."
	@find . -name "*.go" | entr -r make run

dev-test-watch:
	@echo "Watching for changes and running tests..."
	@find . -name "*.go" | entr -r make test-verbose

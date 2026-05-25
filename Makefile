DOMAIN ?= domain.yaml
BRIDGE ?= bridges/csharp
OUTPUT ?= generated
BRIDGE_NAME ?= csharp
SPEC_OUTPUT ?= spec/domain.schema.json
GO_CACHE_DIR ?= $(CURDIR)/.gocache
GO_TMP_DIR ?= $(CURDIR)/bin

export GOCACHE := $(GO_CACHE_DIR)
export GOTMPDIR := $(GO_TMP_DIR)

.PHONY: help build run test test-verbose test-coverage lint fmt clean install-deps cli-validate cli-generate cli-init regenerate-spec generate-gui-types

help:
	@echo "DomainCraft Parser - Available Commands"
	@echo ""
	@echo "  make build           - Build the parser binary"
	@echo "  make run             - Build and run the parser binary with example domain.yaml"
	@echo "  make cli-validate    - Run the CLI 'validate' command (uses DOMAIN=$(DOMAIN))"
	@echo "  make cli-generate    - Run the CLI 'generate' command (uses DOMAIN=$(DOMAIN) BRIDGE=$(BRIDGE) OUTPUT=$(OUTPUT))"
	@echo "  make cli-init        - Run the CLI 'init' command (uses BRIDGE_NAME=$(BRIDGE_NAME))"
	@echo "  make regenerate-spec - Regenerate spec/domain.schema.json and GUI TypeScript types"
	@echo "  make generate-gui-types - Regenerate only GUI TypeScript types from schema"
	@echo "  make test            - Run all tests"
	@echo "  make test-verbose    - Run tests with verbose output"
	@echo "  make test-coverage   - Run tests and generate coverage report"
	@echo "  make lint            - Run linter (golangci-lint)"
	@echo "  make fmt             - Format code (gofmt)"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make install-deps    - Install dependencies"

install-deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

build: install-deps
	@echo "Building parser..."
	@go build -o bin/parser ./cmd/parser

run: build
	@echo "Running parser with example domain.yaml..."
	@./bin/parser

# Convenience targets to call the CLI without building binary
cli-validate:
	@echo "Running: go run ./cmd/parser validate --domain $(DOMAIN)"
	@go run ./cmd/parser validate --domain $(DOMAIN)

cli-generate:
	@echo "Running: go run ./cmd/parser generate --domain $(DOMAIN) --bridge $(BRIDGE) --output $(OUTPUT)"
	@go run ./cmd/parser generate --domain $(DOMAIN) --bridge $(BRIDGE) --output $(OUTPUT)

cli-init:
	@echo "Running: go run ./cmd/parser init $(BRIDGE_NAME)"
	@go run ./cmd/parser init $(BRIDGE_NAME)

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

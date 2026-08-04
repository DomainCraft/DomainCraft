DOMAIN ?= domain.yaml
BRIDGE ?= bridges/csharp
OUTPUT ?= generated
BRIDGE_NAME ?= csharp
SPEC_OUTPUT ?= spec/domain.schema.json
GUI_DIR ?= ../DomainCraftGui
SKILLS_DIR ?= ../DomainCraft-skills/domaincraft-core
GO_CACHE_DIR ?= $(CURDIR)/.gocache
GO_TMP_DIR ?= $(CURDIR)/bin

# Release version stamped into the CLI (and WASM validator) binary.
ifeq ($(OS),Windows_NT)
VERSION ?= $(shell git describe --tags --abbrev=0 2>nul || echo dev)
else
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
endif

# Cross-platform file copy used by the spec-distribution targets.
# Cross-platform copy used by the spec-distribution targets.
ifeq ($(OS),Windows_NT)
COPY_CMD = copy /Y "$(subst /,\,$(SPEC_OUTPUT))" "$(subst /,\,$(SKILLS_DIR))\domain.schema.json"
GUI_CMD = cd /d "$(subst /,\,$(GUI_DIR))" && call npm run generate:types
else
COPY_CMD = cp "$(SPEC_OUTPUT)" "$(SKILLS_DIR)/domain.schema.json"
GUI_CMD = cd $(GUI_DIR) && npm run generate:types
endif

export GOCACHE := $(GO_CACHE_DIR)
export GOTMPDIR := $(GO_TMP_DIR)

.PHONY: help build install run test test-verbose test-coverage lint fmt clean install-deps cli-validate cli-generate cli-new cli-bridges regenerate-spec generate-gui-types copy-spec-skills build-wasm build-wasm-gui

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
	@echo "  make regenerate-spec - Regenerate spec/domain.schema.json, copy to skills, and regenerate GUI types"
	@echo "  make generate-gui-types - Regenerate only GUI TypeScript types from schema"
	@echo "  make copy-spec-skills - Copy regenerated schema to DomainCraft-skills/domaincraft-core"
	@echo "  make build-wasm      - Build WASM validator binary (GOOS=js GOARCH=wasm)"
	@echo "  make build-wasm-gui  - Build WASM and copy to DomainCraftGui/public/wasm/"
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
	@echo "Building domaincraft ($(VERSION))..."
	@go build -ldflags "-X main.version=$(VERSION)" -o bin/domaincraft ./cmd/domaincraft

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
	@$(MAKE) --no-print-directory copy-spec-skills
	@$(MAKE) --no-print-directory generate-gui-types

generate-gui-types:
ifeq ($(wildcard $(GUI_DIR)/package.json),)
	@echo "WARNING: GUI directory not found at $(GUI_DIR), skipping GUI types."
else
	@echo "Generating TypeScript types for GUI..."
	@$(GUI_CMD)
endif

copy-spec-skills:
ifeq ($(wildcard $(SKILLS_DIR)),)
	@echo "WARNING: skills directory not found at $(SKILLS_DIR), skipping schema copy."
else
	@echo "Copying schema to skills..."
	@$(COPY_CMD)
	@echo "Copied schema to $(SKILLS_DIR)/domain.schema.json"
endif

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

# WASM targets
WASM_OUTPUT ?= bin/validate.wasm
WASM_GUI_DIR ?= ../DomainCraftGui/public/wasm
WASM_LDFLAGS := -X main.version=$(VERSION)

ifeq ($(OS),Windows_NT)
build-wasm:
	@echo "Building WASM validator ($(VERSION))..."
	@cmd /c "set GOOS=js&& set GOARCH=wasm&& go build "-ldflags=$(WASM_LDFLAGS)" -o $(WASM_OUTPUT) ./cmd/wasm-validator/"
	@echo "Built $(WASM_OUTPUT)"
build-wasm-gui: build-wasm
	@echo "Copying WASM to GUI public directory..."
	@if not exist "$(subst /,\,$(WASM_GUI_DIR))" mkdir "$(subst /,\,$(WASM_GUI_DIR))"
	@copy "$(subst /,\,$(WASM_OUTPUT))" "$(subst /,\,$(WASM_GUI_DIR))\validate.wasm" >nul
	@echo "Copied to $(WASM_GUI_DIR)/validate.wasm"
else
build-wasm:
	@echo "Building WASM validator ($(VERSION))..."
	@GOOS=js GOARCH=wasm go build -ldflags "$(WASM_LDFLAGS)" -o $(WASM_OUTPUT) ./cmd/wasm-validator/
	@echo "Built $(WASM_OUTPUT)"
build-wasm-gui: build-wasm
	@echo "Copying WASM to GUI public directory..."
	@mkdir -p $(WASM_GUI_DIR)
	@cp $(WASM_OUTPUT) $(WASM_GUI_DIR)/validate.wasm
	@echo "Copied to $(WASM_GUI_DIR)/validate.wasm"
endif

APP=ghub-desk


.PHONY: build clean install test run-help deps examples dev setup version

# Variables
BINARY_NAME=ghub-desk
BUILD_DIR=./build
GO_FILES=$(shell find . -name "*.go")

# Version information
#  ref: git tag v0.0.1
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Linker flags for version information
LDFLAGS = -ldflags "-s -w -X cmd.appVersion=$(VERSION) -X cmd.appCommit=$(COMMIT) -X cmd.appDate=$(DATE)"

# Default target
all: build

# Build the binary
build:
	@echo "🏗️  Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	@go mod tidy
	@go mod download
	@echo "✅ Dependencies installed"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f ghub-desk.db
	@echo "✅ Clean completed"

# Install the binary to GOPATH/bin
install: build
	@echo "📥 Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS)
	@echo "✅ $(BINARY_NAME) installed"

# Show version information
version: build
	@echo "📋 Version information:"
	@$(BUILD_DIR)/$(BINARY_NAME) version

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test -v ./...
	@echo "✅ Tests completed"

# Run with help
run-help: build
	@echo "🚀 Running $(BINARY_NAME) --help..."
	@$(BUILD_DIR)/$(BINARY_NAME) --help

# Example usage commands
examples: build
	@echo "📚 Example usage:"
	@echo ""
	@echo "Environment setup:"
	@echo "  export GHUB_DESK_ORGANIZATION=\"your-organization\""
	@echo "  export GHUB_DESK_GITHUB_TOKEN=\"your-github-token\""
	@echo ""
	@echo "Pull commands:"
	@echo "  $(BUILD_DIR)/$(BINARY_NAME) pull users"
	@echo "  $(BUILD_DIR)/$(BINARY_NAME) pull --store teams"
	@echo "  $(BUILD_DIR)/$(BINARY_NAME) pull team-name/users"
	@echo ""
	@echo "Push commands (DRYRUN):"
	@echo "  $(BUILD_DIR)/$(BINARY_NAME) push remove team-name"
	@echo "  $(BUILD_DIR)/$(BINARY_NAME) push remove user-name"
	@echo ""
	@echo "Push commands (EXECUTE):"
	@echo "  $(BUILD_DIR)/$(BINARY_NAME) push remove --exec team-name"

# Development mode - build and run with args
dev: build
	@echo "🛠️  Development mode - building and running..."
	@$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Quick setup for development
setup: deps build
	@echo "🎯 Quick setup completed!"
	@echo "Set environment variables:"
	@echo "  export GHUB_DESK_ORGANIZATION=\"your-organization\""
	@echo "  export GHUB_DESK_GITHUB_TOKEN=\"your-github-token\""
	@echo ""
	@echo "Then run: make run-help"

# Check GoReleaser
goreleaser_check:
	@echo "Checking release..."
	@goreleaser check

# Release using GoReleaser
goreleaser:
	@echo "🚀 Building release..."
	@goreleaser release --clean

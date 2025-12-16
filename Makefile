# Makefile for Hubble Installer

.PHONY: all build clean test run run-debug run-clean install uninstall deps fmt lint help

# Variables
BINARY_NAME=hubble-install
VERSION?=0.1.0
BUILD_DIR=bin
GO=go
GOFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Default target
all: clean deps build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .
	@echo "✓ Build complete: ./$(BINARY_NAME)"

# Run the installer
run:
	@$(GO) run main.go

# Run with debug mode
run-debug:
	@$(GO) run main.go -debug

# Run with clean mode (remove deps and exit with verbose output)
run-clean:
	@$(GO) run main.go -clean

# Install dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "✓ Dependencies ready"

# Run tests
test:
	@$(GO) test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "✓ Clean complete"

# Install to /usr/local/bin (requires sudo)
install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "✓ Installed: /usr/local/bin/$(BINARY_NAME)"

# Uninstall from /usr/local/bin
uninstall:
	@echo "Uninstalling from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Uninstalled"

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "✓ Formatting complete"

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@golangci-lint run || echo "Install golangci-lint: brew install golangci-lint"

# Show help
help:
	@echo "Hubble Installer - Makefile Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              - Clean, download deps, and build (default)"
	@echo "  build            - Build for current platform"
	@echo "  run              - Run the installer directly"
	@echo "  run-debug        - Run with debug mode (-debug flag)"
	@echo "  run-clean        - Run clean mode (removes deps with verbose output and exits)"
	@echo "  deps             - Download and tidy Go dependencies"
	@echo "  test             - Run tests"
	@echo "  clean            - Remove build artifacts"
	@echo "  install          - Install to /usr/local/bin (requires sudo)"
	@echo "  uninstall        - Remove from /usr/local/bin (requires sudo)"
	@echo "  fmt              - Format Go code"
	@echo "  lint             - Lint Go code (requires golangci-lint)"
	@echo "  help             - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build               # Build for current platform"
	@echo "  make run                 # Run the installer"
	@echo "  make run-clean           # Remove dependencies (verbose)"
	@echo "  make VERSION=0.2.0 build # Build with custom version"


# Makefile for Hubble Installer

.PHONY: all build clean test run run-debug run-clean install uninstall deps fmt lint help build-windows build-linux build-darwin build-darwin-arm build-all release-windows

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

# Build for Windows (64-bit)
build-windows:
	@echo "Building $(BINARY_NAME) v$(VERSION) for Windows (amd64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

# Build for Linux (64-bit)
build-linux:
	@echo "Building $(BINARY_NAME) v$(VERSION) for Linux (amd64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# Build for macOS (Intel)
build-darwin:
	@echo "Building $(BINARY_NAME) v$(VERSION) for macOS (amd64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"

# Build for macOS (Apple Silicon)
build-darwin-arm:
	@echo "Building $(BINARY_NAME) v$(VERSION) for macOS (arm64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"

# Build for all platforms
build-all: build-windows build-linux build-darwin build-darwin-arm
	@echo "✓ All platform builds complete"

# Create a Windows release build (optimized, with version info)
release-windows:
	@echo "Creating Windows release v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-v$(VERSION)-windows-amd64.exe .
	@echo "✓ Release complete: $(BUILD_DIR)/$(BINARY_NAME)-v$(VERSION)-windows-amd64.exe"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-v$(VERSION)-windows-amd64.exe

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
	@echo "Build Targets:"
	@echo "  all              - Clean, download deps, and build (default)"
	@echo "  build            - Build for current platform"
	@echo "  build-windows    - Build for Windows (amd64)"
	@echo "  build-linux      - Build for Linux (amd64)"
	@echo "  build-darwin     - Build for macOS Intel (amd64)"
	@echo "  build-darwin-arm - Build for macOS Apple Silicon (arm64)"
	@echo "  build-all        - Build for all platforms"
	@echo "  release-windows  - Create optimized Windows release build"
	@echo ""
	@echo "Development Targets:"
	@echo "  run              - Run the installer directly"
	@echo "  run-debug        - Run with debug mode (-debug flag)"
	@echo "  run-clean        - Run clean mode (removes deps with verbose output and exits)"
	@echo "  deps             - Download and tidy Go dependencies"
	@echo "  test             - Run tests"
	@echo "  clean            - Remove build artifacts"
	@echo "  fmt              - Format Go code"
	@echo "  lint             - Lint Go code (requires golangci-lint)"
	@echo ""
	@echo "Installation Targets:"
	@echo "  install          - Install to /usr/local/bin (requires sudo)"
	@echo "  uninstall        - Remove from /usr/local/bin (requires sudo)"
	@echo ""
	@echo "Examples:"
	@echo "  make build-windows           # Build for Windows"
	@echo "  make release-windows         # Create Windows release"
	@echo "  make VERSION=0.2.0 release-windows  # Release with version"
	@echo "  make build-all               # Build for all platforms"
	@echo "  make run                     # Run the installer"


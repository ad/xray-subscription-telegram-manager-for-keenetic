# Makefile for xray-telegram-manager
# Cross-compilation support for MIPS (Keenetic routers)

# Variables
BINARY_NAME := xray-telegram-manager
OUTPUT_DIR := ./dist
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | awk '{print $$3}')

# Build flags for size optimization
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)

# Go build flags
BUILDFLAGS := -ldflags="$(LDFLAGS)" -trimpath

# Default target
.DEFAULT_GOAL := mips

# Help target
.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Clean target
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(OUTPUT_DIR)
	@echo "Clean completed"

# Create output directory
$(OUTPUT_DIR):
	@mkdir -p $(OUTPUT_DIR)

# MIPS softfloat target (most compatible with Keenetic)
.PHONY: mips-softfloat
mips-softfloat: $(OUTPUT_DIR) ## Build for MIPS softfloat (recommended for Keenetic)
	@echo "Building $(BINARY_NAME) for MIPS softfloat..."
	@GOOS=linux GOARCH=mips GOMIPS=softfloat go build $(BUILDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-mips-softfloat .
	@echo "✓ Built $(BINARY_NAME)-mips-softfloat"

# MIPS hardfloat target
.PHONY: mips-hardfloat
mips-hardfloat: $(OUTPUT_DIR) ## Build for MIPS hardfloat
	@echo "Building $(BINARY_NAME) for MIPS hardfloat..."
	@GOOS=linux GOARCH=mips GOMIPS=hardfloat go build $(BUILDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-mips-hardfloat .
	@echo "✓ Built $(BINARY_NAME)-mips-hardfloat"

# Both MIPS targets
.PHONY: mips
mips: mips-softfloat mips-hardfloat ## Build for both MIPS variants (default)

# Linux AMD64 target (for testing)
.PHONY: linux-amd64
linux-amd64: $(OUTPUT_DIR) ## Build for Linux AMD64 (testing)
	@echo "Building $(BINARY_NAME) for Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "✓ Built $(BINARY_NAME)-linux-amd64"

# Linux ARM64 target
.PHONY: linux-arm64
linux-arm64: $(OUTPUT_DIR) ## Build for Linux ARM64
	@echo "Building $(BINARY_NAME) for Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build $(BUILDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "✓ Built $(BINARY_NAME)-linux-arm64"

# macOS targets
.PHONY: darwin-amd64
darwin-amd64: $(OUTPUT_DIR) ## Build for macOS AMD64
	@echo "Building $(BINARY_NAME) for macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "✓ Built $(BINARY_NAME)-darwin-amd64"

.PHONY: darwin-arm64
darwin-arm64: $(OUTPUT_DIR) ## Build for macOS ARM64 (Apple Silicon)
	@echo "Building $(BINARY_NAME) for macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 go build $(BUILDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "✓ Built $(BINARY_NAME)-darwin-arm64"

# Build all targets
.PHONY: all
all: mips linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 ## Build for all supported platforms

# Test compilation (quick check)
.PHONY: test-compile
test-compile: ## Test compilation without creating binaries
	@echo "Testing compilation for MIPS softfloat..."
	@GOOS=linux GOARCH=mips GOMIPS=softfloat go build -o /dev/null .
	@echo "✓ MIPS softfloat compilation successful"
	@echo "Testing compilation for MIPS hardfloat..."
	@GOOS=linux GOARCH=mips GOMIPS=hardfloat go build -o /dev/null .
	@echo "✓ MIPS hardfloat compilation successful"

# Show build info
.PHONY: info
info: ## Show build information
	@echo "Build Information:"
	@echo "  Binary Name: $(BINARY_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Build Time:  $(BUILD_TIME)"
	@echo "  Go Version:  $(GO_VERSION)"
	@echo "  Output Dir:  $(OUTPUT_DIR)"
	@echo "  LDFLAGS:     $(LDFLAGS)"

# Create checksums
.PHONY: checksums
checksums: ## Create checksums for built binaries
	@echo "Creating checksums..."
	@cd $(OUTPUT_DIR) && find . -name "$(BINARY_NAME)-*" -type f -exec sha256sum {} \; > checksums.txt
	@echo "✓ Checksums created in $(OUTPUT_DIR)/checksums.txt"

# Compress binaries with UPX (if available)
.PHONY: compress
compress: ## Compress binaries with UPX
	@if command -v upx >/dev/null 2>&1; then \
		echo "Compressing binaries with UPX..."; \
		for binary in $(OUTPUT_DIR)/$(BINARY_NAME)-*; do \
			if [ -f "$$binary" ]; then \
				echo "Compressing $$(basename $$binary)..."; \
				upx --best --lzma "$$binary" 2>/dev/null || echo "Failed to compress $$(basename $$binary)"; \
			fi; \
		done; \
		echo "✓ Compression completed"; \
	else \
		echo "UPX not found, skipping compression"; \
	fi

# Full build with checksums and compression
.PHONY: release
release: clean mips checksums compress ## Full release build (clean, build MIPS, checksums, compress)
	@echo "Release build completed!"
	@ls -lh $(OUTPUT_DIR)

# Development build (current platform)
.PHONY: dev
dev: ## Build for current platform (development)
	@echo "Building $(BINARY_NAME) for development..."
	@go build $(BUILDFLAGS) -o $(BINARY_NAME) .
	@echo "✓ Built $(BINARY_NAME) for development"

# Install development binary
.PHONY: install
install: dev ## Install development binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	@go install $(BUILDFLAGS) .
	@echo "✓ Installed $(BINARY_NAME)"

# Run tests
.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

# Format code
.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

# Lint code
.PHONY: lint
lint: ## Lint Go code (requires golangci-lint)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Linting code..."; \
		golangci-lint run; \
		echo "✓ Linting completed"; \
	else \
		echo "golangci-lint not found, skipping linting"; \
	fi

# Tidy dependencies
.PHONY: tidy
tidy: ## Tidy Go modules
	@echo "Tidying Go modules..."
	@go mod tidy
	@echo "✓ Go modules tidied"

# Security check
.PHONY: security
security: ## Run security checks (requires gosec)
	@if command -v gosec >/dev/null 2>&1; then \
		echo "Running security checks..."; \
		gosec ./...; \
		echo "✓ Security checks completed"; \
	else \
		echo "gosec not found, skipping security checks"; \
	fi

# Full check (format, lint, test, security)
.PHONY: check
check: fmt lint test security ## Run all checks (format, lint, test, security)
	@echo "✓ All checks completed"
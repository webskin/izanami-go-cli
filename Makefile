# Izanami Go CLI Makefile

# Binary name
BINARY_NAME=iz

# Build directory
BUILD_DIR=build

# Version information
VERSION?=dev
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Linker flags to inject version information
LDFLAGS=-ldflags "\
	-X 'github.com/webskin/izanami-go-cli/internal/cmd.Version=$(VERSION)' \
	-X 'github.com/webskin/izanami-go-cli/internal/cmd.GitCommit=$(GIT_COMMIT)' \
	-X 'github.com/webskin/izanami-go-cli/internal/cmd.BuildDate=$(BUILD_DATE)'"

# Default target
.PHONY: all
all: test build

# Build for current platform
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/iz

# Install to $GOPATH/bin
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) ./cmd/iz

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run integration tests
# Usage: make integration-test [FILTER=Tenants]
.PHONY: integration-test
integration-test:
	@echo "Running integration tests against local Izanami server..."
	./runIntegrationTests.sh $(FILTER)

# Run tests with coverage report
.PHONY: test-coverage
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Build for all platforms
.PHONY: build-all
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/iz

	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/iz

	@echo "Building for macOS AMD64 (Intel)..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/iz

	@echo "Building for macOS ARM64 (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/iz

	@echo "Building for Windows AMD64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/iz

	@echo "Build complete! Binaries are in $(BUILD_DIR)/"

# Run the binary
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# GoReleaser targets
.PHONY: release-test
release-test:
	@echo "Testing release build with GoReleaser..."
	@which goreleaser > /dev/null || (echo "GoReleaser not installed. Run: go install github.com/goreleaser/goreleaser@latest" && exit 1)
	goreleaser release --snapshot --clean
	@echo ""
	@echo "✅ Release artifacts generated in dist/ folder"
	@echo "   To test binaries:"
	@echo "   ./dist/iz_linux_amd64_v1/iz version"
	@echo "   ./dist/iz_darwin_amd64_v1/iz version"

.PHONY: release-build
release-build:
	@echo "Building with GoReleaser (single platform)..."
	@which goreleaser > /dev/null || (echo "GoReleaser not installed. Run: go install github.com/goreleaser/goreleaser@latest" && exit 1)
	goreleaser build --snapshot --single-target --clean

.PHONY: release-clean
release-clean:
	@echo "Cleaning GoReleaser artifacts..."
	rm -rf dist/

# Show help
.PHONY: help
help:
	@echo "Izanami Go CLI - Make targets:"
	@echo ""
	@echo "Daily Development:"
	@echo "  make build            - Build binary for current platform (fast)"
	@echo "  make install          - Install binary to \$$GOPATH/bin"
	@echo "  make test             - Run tests"
	@echo "  make integration-test - Run integration tests (requires local Izanami)"
	@echo "  make test-coverage    - Run tests with coverage report"
	@echo "  make fmt              - Format code"
	@echo "  make lint             - Lint code (requires golangci-lint)"
	@echo "  make run              - Build and run the binary"
	@echo ""
	@echo "Multi-platform Builds:"
	@echo "  make build-all      - Build binaries for all platforms (Make)"
	@echo "  make release-build  - Build with GoReleaser (single platform)"
	@echo "  make release-test   - Test full release build (all platforms)"
	@echo ""
	@echo "Maintenance:"
	@echo "  make tidy           - Tidy Go modules"
	@echo "  make deps           - Download dependencies"
	@echo "  make clean          - Clean Make build artifacts"
	@echo "  make release-clean  - Clean GoReleaser artifacts"
	@echo "  make help           - Show this help message"
	@echo ""
	@echo "Release Process:"
	@echo "  1. Test release:    make release-test"
	@echo "  2. Tag version:     git tag v1.0.0"
	@echo "  3. Push tag:        git push origin v1.0.0"
	@echo "  4. GitHub Actions will automatically build and publish the release"
	@echo ""
	@echo "Cross-compilation targets (Make):"
	@echo "  linux/amd64   - $(BINARY_NAME)-linux-amd64"
	@echo "  linux/arm64   - $(BINARY_NAME)-linux-arm64"
	@echo "  darwin/amd64  - $(BINARY_NAME)-darwin-amd64"
	@echo "  darwin/arm64  - $(BINARY_NAME)-darwin-arm64"
	@echo "  windows/amd64 - $(BINARY_NAME)-windows-amd64.exe"

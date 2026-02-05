# Makefile for vdocstool MCP Server

# Variables
APP_NAME := mcp-server
BUILD_DIR := ./bin
WORK_DIR := ./work
CMD_DIR := ./cmd/mcp-server
DOCKER_IMAGE := vdocstool-mcp-server
DOCKER_TAG := latest

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -ldflags="-w -s"
BUILD_FLAGS := -v

# Lint tool
GOLINT := golangci-lint

.PHONY: all build clean test lint fmt vet deps run docker-build docker-run help

# Default target
all: deps lint test build

# Build the application
build:
	@echo ">>> Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo ">>> Build complete: $(BUILD_DIR)/$(APP_NAME)"

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo ">>> Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(CMD_DIR)

build-darwin:
	@echo ">>> Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(CMD_DIR)

build-windows:
	@echo ">>> Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_DIR)

# Clean build artifacts
clean:
	@echo ">>> Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo ">>> Clean complete"

# Run tests
test:
	@echo ">>> Running tests..."
	$(GOTEST) -v -race -cover ./...
	@echo ">>> Tests complete"

# Run integration tests
test-integration:
	@echo ">>> Running integration tests..."
	$(GOTEST) -v -tags=integration ./...
	@echo ">>> Integration tests complete"

# Run tests with coverage report
test-coverage:
	@echo ">>> Running tests with coverage..."
	@mkdir -p $(WORK_DIR)
	$(GOTEST) -v -coverprofile=$(WORK_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(WORK_DIR)/coverage.out -o $(WORK_DIR)/coverage.html
	$(GOCMD) tool cover -func=$(WORK_DIR)/coverage.out
	@echo ">>> Coverage report: $(WORK_DIR)/coverage.html"

# Run benchmarks
bench:
	@echo ">>> Running benchmarks..."
	@mkdir -p $(WORK_DIR)
	$(GOTEST) -bench=. -benchmem ./... | tee $(WORK_DIR)/bench.txt
	@echo ">>> Benchmark results: $(WORK_DIR)/bench.txt"

# Lint code
lint:
	@echo ">>> Running linter..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "golangci-lint not installed. Running go vet instead..."; \
		$(GOVET) ./...; \
	fi
	@echo ">>> Lint complete"

# Format code
fmt:
	@echo ">>> Formatting code..."
	$(GOFMT) -s -w .
	@echo ">>> Format complete"

# Check formatting
fmt-check:
	@echo ">>> Checking code format..."
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "Code is not formatted. Run 'make fmt'"; \
		$(GOFMT) -l .; \
		exit 1; \
	fi
	@echo ">>> Format check passed"

# Run go vet
vet:
	@echo ">>> Running go vet..."
	$(GOVET) ./...
	@echo ">>> Vet complete"

# Download dependencies
deps:
	@echo ">>> Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo ">>> Dependencies ready"

# Verify dependencies
deps-verify:
	@echo ">>> Verifying dependencies..."
	$(GOMOD) verify
	@echo ">>> Dependencies verified"

# Run the application
run: build
	@echo ">>> Running $(APP_NAME)..."
	@mkdir -p $(WORK_DIR)
	$(BUILD_DIR)/$(APP_NAME) -config $(WORK_DIR)/config.json 2>&1 | tee $(WORK_DIR)/server.log

# Run with specific config
run-dev: build
	@echo ">>> Running $(APP_NAME) in development mode..."
	@mkdir -p $(WORK_DIR)
	$(BUILD_DIR)/$(APP_NAME) -listen :8080 2>&1 | tee $(WORK_DIR)/server.log

# Docker build
docker-build:
	@echo ">>> Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo ">>> Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Docker run
docker-run:
	@echo ">>> Running Docker container..."
	docker run -d --name $(APP_NAME) \
		-p 8080:8080 \
		-v $(PWD)/work:/app/work \
		$(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo ">>> Docker container started"

# Docker stop
docker-stop:
	@echo ">>> Stopping Docker container..."
	docker stop $(APP_NAME) || true
	docker rm $(APP_NAME) || true
	@echo ">>> Docker container stopped"

# Docker logs
docker-logs:
	docker logs -f $(APP_NAME)

# Install development tools
tools:
	@echo ">>> Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo ">>> Tools installed"

# Generate mocks (if needed in future)
generate:
	@echo ">>> Generating code..."
	$(GOCMD) generate ./...
	@echo ">>> Generation complete"

# Create work directory structure
setup-work:
	@echo ">>> Setting up work directory..."
	@mkdir -p $(WORK_DIR)/logs
	@mkdir -p $(WORK_DIR)/data
	@mkdir -p $(WORK_DIR)/attachments
	@echo ">>> Work directory ready"

# CI pipeline simulation
ci: deps fmt-check vet lint test
	@echo ">>> CI pipeline passed"

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Run deps, lint, test, and build (default)"
	@echo "  build            - Build the application"
	@echo "  build-all        - Build for all platforms"
	@echo "  clean            - Remove build artifacts"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  bench            - Run benchmarks"
	@echo "  lint             - Run linter (golangci-lint or go vet)"
	@echo "  fmt              - Format code"
	@echo "  fmt-check        - Check code formatting"
	@echo "  vet              - Run go vet"
	@echo "  deps             - Download dependencies"
	@echo "  deps-verify      - Verify dependencies"
	@echo "  run              - Build and run the application"
	@echo "  run-dev          - Run in development mode"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker container"
	@echo "  docker-stop      - Stop Docker container"
	@echo "  docker-logs      - View Docker logs"
	@echo "  tools            - Install development tools"
	@echo "  setup-work       - Create work directory structure"
	@echo "  ci               - Run CI pipeline"
	@echo "  help             - Show this help message"

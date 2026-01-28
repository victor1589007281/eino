# Makefile for MySQL Expert Agent

SHELL := /bin/bash
PROJECT := mysql-expert-agent
GO := go
GOFLAGS := -v
LDFLAGS := -s -w

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# 路径
BIN_DIR := bin
CMD_DIR := cmd
PKG_DIR := pkg
COVERAGE_DIR := coverage

# 测试配置
TEST_PARALLEL_NUM ?= 4
TEST_PROGRAMS_NUM ?= 4
TEST_TIMEOUT ?= 300s
COVERAGE_THRESHOLD ?= 70

# Docker配置
DOCKER_REGISTRY ?= harbor.example.com
DOCKER_IMAGE ?= $(DOCKER_REGISTRY)/mysql-expert/$(PROJECT)

# Go编译标志
GO_LDFLAGS := -ldflags "-s -w \
	-X main.Version=$(VERSION) \
	-X main.Commit=$(COMMIT) \
	-X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build test lint clean docker help

# 默认目标
all: lint test build

# 帮助
help:
	@echo "MySQL Expert Agent Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all          Run lint, test, and build"
	@echo "  build        Build the binary"
	@echo "  test         Run unit tests"
	@echo "  test-all     Run all tests (unit + integration)"
	@echo "  lint         Run linters"
	@echo "  fmt          Format code"
	@echo "  vet          Run go vet"
	@echo "  coverage     Generate coverage report"
	@echo "  clean        Clean build artifacts"
	@echo "  docker       Build Docker image"
	@echo "  docker-push  Push Docker image"
	@echo "  deps         Download dependencies"
	@echo "  help         Show this help"

# 依赖下载
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod verify

# 代码格式化
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

# Lint检查
lint: vet
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --concurrency=2 --max-issues-per-linter=50; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# 构建
build:
	@echo "Building $(PROJECT)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build $(GOFLAGS) $(GO_LDFLAGS) \
		-o $(BIN_DIR)/$(PROJECT) \
		./$(CMD_DIR)/main.go
	@echo "Binary built: $(BIN_DIR)/$(PROJECT)"

# macOS构建
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		$(GO) build $(GOFLAGS) $(GO_LDFLAGS) \
		-o $(BIN_DIR)/$(PROJECT)-darwin \
		./$(CMD_DIR)/main.go

# 单元测试
test:
	@echo "Running unit tests..."
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test \
		-v \
		-timeout=$(TEST_TIMEOUT) \
		-parallel=$(TEST_PARALLEL_NUM) \
		-p=$(TEST_PROGRAMS_NUM) \
		-count=1 \
		-race \
		-cover \
		-coverprofile=$(COVERAGE_DIR)/coverage.out \
		$$($(GO) list ./... | grep -v /vendor/ | grep -v /mock | grep -v /tests/integration)
	@echo ""
	@echo "Coverage summary:"
	@$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -n 1

# 集成测试
test-integration:
	@echo "Running integration tests..."
	$(GO) test \
		-v \
		-timeout=600s \
		-tags=integration \
		-coverprofile=$(COVERAGE_DIR)/coverage-integration.out \
		./tests/integration/...

# 所有测试
test-all: test test-integration

# 覆盖率报告
coverage: test
	@echo "Generating coverage report..."
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

# 基准测试
benchmark:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

# 清理
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -rf $(COVERAGE_DIR)
	$(GO) clean -cache -testcache

# Docker构建
docker:
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		.
	@echo "Image built: $(DOCKER_IMAGE):$(VERSION)"

# Docker推送
docker-push: docker
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

# 生成Mock
generate-mocks:
	@echo "Generating mocks..."
	$(GO) generate ./...

# 安装开发工具
install-tools:
	@echo "Installing development tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install github.com/golang/mock/mockgen@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest

# 安全扫描
security-scan:
	@echo "Running security scan..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed, skipping..."; \
	fi

# 运行开发服务器
run:
	@echo "Starting development server..."
	$(GO) run ./$(CMD_DIR)/main.go --config=config/config-dev.yaml

# 差异覆盖率 (用于MR/PR)
diff-coverage: test
	@echo "Calculating diff coverage..."
	@if command -v gocov >/dev/null 2>&1 && command -v gocov-xml >/dev/null 2>&1; then \
		gocov convert $(COVERAGE_DIR)/coverage.out | gocov-xml > $(COVERAGE_DIR)/coverage.xml; \
		if command -v diff-cover >/dev/null 2>&1; then \
			diff-cover $(COVERAGE_DIR)/coverage.xml \
				--compare-branch origin/master \
				--html-report $(COVERAGE_DIR)/diff_coverage.html \
				--json-report $(COVERAGE_DIR)/diff_coverage.json \
				--diff-range-notation '..' \
				--show-uncovered \
				--fail-under=$(COVERAGE_THRESHOLD); \
		fi; \
	else \
		echo "gocov or diff-cover not installed, skipping diff coverage..."; \
	fi

# CI入口 - lint
ci-lint: deps vet lint

# CI入口 - test
ci-test: deps test coverage

# CI入口 - build
ci-build: deps build

# 检查代码是否格式化
check-fmt:
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Code not formatted. Run 'make fmt'" && exit 1)

# 检查go.mod/go.sum是否一致
check-mod:
	@echo "Checking go modules..."
	$(GO) mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy. Run 'go mod tidy'" && exit 1)

# 完整CI检查
ci-check: check-fmt check-mod ci-lint ci-test

# 索引重建命令
index-rebuild:
	@echo "Rebuilding index..."
	$(GO) run ./$(CMD_DIR)/main.go --rebuild-index

# 版本信息
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Built:   $(BUILD_TIME)"

# Helm Chart打包
helm-package:
	@echo "Packaging Helm chart..."
	@if [ -d "deploy/helm" ]; then \
		helm package deploy/helm/$(PROJECT) -d $(BIN_DIR); \
	else \
		echo "Helm chart not found at deploy/helm/$(PROJECT)"; \
	fi

# 部署到K8s (dry-run)
deploy-dry-run:
	@echo "Dry-run deployment..."
	kubectl apply -f deploy/k8s/ --dry-run=client

# 部署到K8s
deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deploy/k8s/

.PHONY: all build test lint fmt clean cover bench ci help

# 变量定义
BINARY_NAME=kernel-expert
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"
COVER_PROFILE=coverage.out
COVER_HTML=coverage.html

# 默认目标
all: lint test build

# 帮助信息
help:
	@echo "Linux内核专家Agent - Make命令"
	@echo ""
	@echo "使用方法:"
	@echo "  make build    - 编译项目"
	@echo "  make test     - 运行所有测试"
	@echo "  make lint     - 运行代码检查"
	@echo "  make fmt      - 格式化代码"
	@echo "  make cover    - 生成测试覆盖率报告"
	@echo "  make bench    - 运行基准测试"
	@echo "  make clean    - 清理构建产物"
	@echo "  make ci       - CI完整流程(lint+test+build)"
	@echo "  make deps     - 安装依赖"
	@echo "  make tools    - 安装开发工具"

# 安装依赖
deps:
	$(GO) mod download
	$(GO) mod tidy

# 安装开发工具
tools:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest

# 编译
build:
	@echo ">>> 编译项目..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) .
	@echo ">>> 编译完成: bin/$(BINARY_NAME)"

# 格式化代码
fmt:
	@echo ">>> 格式化代码..."
	$(GO) fmt ./...
	@if command -v goimports > /dev/null; then \
		goimports -w -local kernel_expert .; \
	fi
	@echo ">>> 格式化完成"

# 代码检查
lint: fmt
	@echo ">>> 运行代码检查..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run --timeout=5m ./...; \
	else \
		echo "golangci-lint未安装，使用go vet"; \
		$(GO) vet ./...; \
	fi
	@echo ">>> 代码检查完成"

# 运行测试
test:
	@echo ">>> 运行单元测试..."
	$(GO) test -v -race -timeout 30m ./...
	@echo ">>> 测试完成"

# 快速测试（不带verbose）
test-quick:
	@echo ">>> 快速测试..."
	$(GO) test -race -timeout 10m ./...
	@echo ">>> 测试完成"

# 测试覆盖率
cover:
	@echo ">>> 生成测试覆盖率报告..."
	$(GO) test -v -race -coverprofile=$(COVER_PROFILE) -covermode=atomic ./...
	$(GO) tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)
	$(GO) tool cover -func=$(COVER_PROFILE)
	@echo ">>> 覆盖率报告: $(COVER_HTML)"

# 基准测试
bench:
	@echo ">>> 运行基准测试..."
	$(GO) test -bench=. -benchmem -run=^$$ ./...
	@echo ">>> 基准测试完成"

# 清理
clean:
	@echo ">>> 清理构建产物..."
	rm -rf bin/
	rm -f $(COVER_PROFILE) $(COVER_HTML)
	$(GO) clean -cache
	@echo ">>> 清理完成"

# CI流程
ci: deps lint test build
	@echo ">>> CI流程完成"

# 仅编译检查（不生成二进制）
check:
	@echo ">>> 编译检查..."
	$(GO) build -o /dev/null ./...
	@echo ">>> 编译检查通过"

# 安全扫描
security:
	@echo ">>> 运行安全扫描..."
	@if command -v gosec > /dev/null; then \
		gosec -quiet ./...; \
	else \
		echo "gosec未安装，跳过安全扫描"; \
	fi
	@echo ">>> 安全扫描完成"

# Docker镜像构建
docker:
	@echo ">>> 构建Docker镜像..."
	docker build -t kernel-expert:latest .
	@echo ">>> Docker镜像构建完成"

# 生成mocks（如果使用mockgen）
mocks:
	@echo ">>> 生成mock文件..."
	@if command -v mockgen > /dev/null; then \
		$(GO) generate ./...; \
	fi
	@echo ">>> mock生成完成"

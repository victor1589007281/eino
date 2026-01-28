SHELL := /bin/bash
MODULE := github.com/cloudwego/eino/vdocswx
BINARY := wechat-polish
TEST_PARALLEL_NUM := $(shell [ -n "${GO_TEST_PARALLEL_NUM}" ] && echo "${GO_TEST_PARALLEL_NUM}" || echo "4")
GOCOVERDIR := $(shell [ -n "${CI_PROJECT_DIR}" ] && echo "${CI_PROJECT_DIR}/coveragebin" || echo "$(shell pwd)/coveragebin")
TEST := MallocNanoZone=0 go test -timeout=300s -v -parallel=${TEST_PARALLEL_NUM} -count=1 -race -cover
PKGS := $$(go list ./... | grep -v vendor/ | grep -v mock)

# 版本信息
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date +'%F %T %z')
GO_VERSION := $(shell go version)
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0")

GO_LDFLAGS ?= -s \
              -X '$(MODULE)/cmd.GitCommit=$(GIT_COMMIT)' \
              -X '$(MODULE)/cmd.BuildDate=$(BUILD_DATE)' \
              -X '$(MODULE)/cmd.GoVersion=$(GO_VERSION)' \
              -X '$(MODULE)/cmd.Version=$(VERSION)'

.PHONY: all build test lint clean docker help

default: build

## 帮助信息
help:
	@echo "WeChat Polish Agent - Makefile 帮助"
	@echo ""
	@echo "可用目标:"
	@echo "  build       - 构建二进制文件"
	@echo "  build-linux - 构建Linux二进制文件"
	@echo "  test        - 运行单元测试"
	@echo "  coverage    - 生成测试覆盖率报告"
	@echo "  lint        - 运行代码检查"
	@echo "  fmt         - 格式化代码"
	@echo "  clean       - 清理构建产物"
	@echo "  docker      - 构建Docker镜像"
	@echo "  deploy      - 部署到Kubernetes"
	@echo "  deps        - 下载依赖"

## 格式化代码
fmt:
	@echo "==> Formatting code..."
	go vet $(PKGS)
	go fmt $(PKGS)
	@echo "==> Done"

## 代码检查
lint: fmt
	@echo "==> Running linter..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run --concurrency=2 --max-issues-per-linter=50 --max-same-issues=5; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi
	@echo "==> Done"

## 下载依赖
deps:
	@echo "==> Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "==> Done"

## 构建
build: deps
	@echo "==> Building binary..."
	go build -ldflags "$(GO_LDFLAGS)" -o bin/$(BINARY) ./cmd/main.go
	@echo "==> Done: bin/$(BINARY)"

## 构建Linux版本
build-linux: deps
	@echo "==> Building Linux binary..."
	GOOS=linux GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o bin/$(BINARY)-linux ./cmd/main.go
	@echo "==> Done: bin/$(BINARY)-linux"

## 构建Darwin版本
build-darwin: deps
	@echo "==> Building Darwin binary..."
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o bin/$(BINARY)-darwin ./cmd/main.go
	@echo "==> Done: bin/$(BINARY)-darwin"

## 清理
clean:
	@echo "==> Cleaning..."
	rm -rf bin/
	rm -rf $(GOCOVERDIR)
	rm -f coverage.out coverage.xml coverage.html
	@echo "==> Done"

## 运行测试
test:
	@echo "==> Running tests..."
	mkdir -p $(GOCOVERDIR)
	$(TEST) -gcflags=all=-l $(PKGS)
	@echo "==> Done"

## 生成覆盖率报告
coverage: test
	@echo "==> Generating coverage report..."
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "==> Coverage report: coverage.html"

## 集成测试
integration-test:
	@echo "==> Running integration tests..."
	$(TEST) -tags=integration $(PKGS)
	@echo "==> Done"

## 构建Docker镜像
docker:
	@echo "==> Building Docker image..."
	docker build -t wechat-polish:$(VERSION) -f Dockerfile .
	docker tag wechat-polish:$(VERSION) wechat-polish:latest
	@echo "==> Done: wechat-polish:$(VERSION)"

## 推送Docker镜像
docker-push:
	@echo "==> Pushing Docker image..."
	docker push wechat-polish:$(VERSION)
	docker push wechat-polish:latest
	@echo "==> Done"

## 部署到Kubernetes
deploy:
	@echo "==> Deploying to Kubernetes..."
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/configmap.yaml
	kubectl apply -f deploy/secrets.yaml
	kubectl apply -f deploy/deployment.yaml
	kubectl apply -f deploy/service.yaml
	@echo "==> Done"

## 卸载部署
undeploy:
	@echo "==> Removing deployment..."
	kubectl delete -f deploy/service.yaml || true
	kubectl delete -f deploy/deployment.yaml || true
	kubectl delete -f deploy/secrets.yaml || true
	kubectl delete -f deploy/configmap.yaml || true
	@echo "==> Done"

## 生成mock文件
mock:
	@echo "==> Generating mocks..."
	go generate ./...
	@echo "==> Done"

## 运行服务
run:
	@echo "==> Running service..."
	go run ./cmd/main.go serve --config=config.yaml

## 初始化索引
init-index:
	@echo "==> Initializing index..."
	go run ./cmd/main.go init --config=config.yaml

## 版本信息
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

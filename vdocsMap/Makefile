.PHONY: all build test clean run docker-build docker-push helm-install help

# 变量
APP_NAME := image-gen-agent
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | cut -d ' ' -f 3)
DOCKER_REGISTRY := docker.io
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)

# Go 构建参数
LDFLAGS := -ldflags "-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
BUILD_TAGS := 

# 默认目标
all: build

## 构建
build: ## 构建二进制文件
	@echo "🔨 Building $(APP_NAME) $(VERSION)..."
	@mkdir -p bin
	@go build $(LDFLAGS) -tags "$(BUILD_TAGS)" -o bin/$(APP_NAME) ./cmd/main.go
	@echo "✅ Build complete: bin/$(APP_NAME)"

build-linux: ## 构建 Linux 二进制文件
	@echo "🔨 Building $(APP_NAME) for Linux..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./cmd/main.go
	@echo "✅ Build complete: bin/$(APP_NAME)-linux-amd64"

## 测试
test: ## 运行单元测试
	@echo "🧪 Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -n 1

test-coverage: test ## 生成测试覆盖率报告
	@echo "📊 Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-integration: build ## 运行集成测试
	@echo "🧪 Running integration tests..."
	@go build -o bin/test_integration ./cmd/test_integration/main.go
	@./bin/test_integration -config config/config-dev.yaml -test all

## 运行
run: build ## 运行服务
	@echo "🚀 Starting $(APP_NAME)..."
	@./bin/$(APP_NAME) -config config/config-dev.yaml -mode cli

run-server: build ## 以服务器模式运行
	@echo "🚀 Starting $(APP_NAME) in server mode..."
	@./bin/$(APP_NAME) -config config/config-dev.yaml -mode server

## Docker
docker-build: ## 构建 Docker 镜像
	@echo "🐳 Building Docker image..."
	@docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .
	@echo "✅ Docker image built: $(APP_NAME):$(VERSION)"

docker-push: docker-build ## 推送 Docker 镜像
	@echo "🚀 Pushing Docker image..."
	@docker tag $(APP_NAME):$(VERSION) $(DOCKER_IMAGE):$(VERSION)
	@docker tag $(APP_NAME):latest $(DOCKER_IMAGE):latest
	@docker push $(DOCKER_IMAGE):$(VERSION)
	@docker push $(DOCKER_IMAGE):latest
	@echo "✅ Docker image pushed"

docker-run: ## 运行 Docker 容器
	@echo "🐳 Running Docker container..."
	@docker-compose up -d

docker-stop: ## 停止 Docker 容器
	@docker-compose down

docker-logs: ## 查看 Docker 日志
	@docker-compose logs -f

## Kubernetes
k8s-deploy: ## 部署到 Kubernetes（使用 kubectl）
	@echo "☸️ Deploying to Kubernetes..."
	@kubectl apply -f deploy/k8s/namespace.yaml
	@kubectl apply -f deploy/k8s/configmap.yaml
	@kubectl apply -f deploy/k8s/secret.yaml
	@kubectl apply -f deploy/k8s/pvc.yaml
	@kubectl apply -f deploy/k8s/deployment.yaml
	@kubectl apply -f deploy/k8s/service.yaml
	@kubectl apply -f deploy/k8s/ingress.yaml
	@echo "✅ Kubernetes deployment complete"

k8s-delete: ## 从 Kubernetes 删除
	@echo "☸️ Deleting from Kubernetes..."
	@kubectl delete -f deploy/k8s/ --ignore-not-found
	@echo "✅ Kubernetes resources deleted"

helm-install: ## 使用 Helm 安装
	@echo "⎈ Installing with Helm..."
	@helm upgrade --install $(APP_NAME) ./deploy/helm/$(APP_NAME) \
		--namespace image-gen-agent \
		--create-namespace \
		--set secrets.openaiApiKey=$${OPENAI_API_KEY}
	@echo "✅ Helm installation complete"

helm-uninstall: ## 卸载 Helm release
	@echo "⎈ Uninstalling Helm release..."
	@helm uninstall $(APP_NAME) --namespace image-gen-agent
	@echo "✅ Helm uninstallation complete"

helm-template: ## 渲染 Helm 模板
	@helm template $(APP_NAME) ./deploy/helm/$(APP_NAME)

## 代码质量
lint: ## 运行代码检查
	@echo "🔍 Running linter..."
	@golangci-lint run ./...

fmt: ## 格式化代码
	@echo "📝 Formatting code..."
	@go fmt ./...
	@goimports -w .

vet: ## 运行 go vet
	@echo "🔍 Running go vet..."
	@go vet ./...

## 依赖管理
deps: ## 下载依赖
	@echo "📦 Downloading dependencies..."
	@go mod download
	@go mod tidy

deps-update: ## 更新依赖
	@echo "📦 Updating dependencies..."
	@go get -u ./...
	@go mod tidy

## 清理
clean: ## 清理构建产物
	@echo "🧹 Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

## 帮助
help: ## 显示帮助信息
	@echo "$(APP_NAME) Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

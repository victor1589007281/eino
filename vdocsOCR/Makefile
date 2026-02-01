.PHONY: all build test test-unit test-integration lint clean run-demo docker-build docker-push k8s-deploy help

# 变量
BINARY_NAME := ocr-agent
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
DOCKER_IMAGE := ocr-agent
DOCKER_TAG := $(VERSION)

all: build

# 构建
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/main.go

build-demo:
	@echo "Building demo binary..."
	go build $(LDFLAGS) -o $(BINARY_NAME)-demo ./cmd/demo/main.go

build-test:
	@echo "Building test binary..."
	go build $(LDFLAGS) -o $(BINARY_NAME)-test ./cmd/test/main.go

# 测试
test: test-unit test-integration

test-unit:
	@echo "Running unit tests..."
	go test -v -race -cover -coverprofile=coverage.out ./...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration -coverprofile=coverage-integration.out ./...

test-coverage: test-unit
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet:
	@echo "Running go vet..."
	go vet ./...

# 清理
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-demo $(BINARY_NAME)-test
	rm -f coverage.out coverage-integration.out coverage.html
	rm -rf dist/

# 运行
run-demo:
	@echo "Running demo..."
	./run_demo.sh

run:
	@echo "Running $(BINARY_NAME)..."
	go run ./cmd/main.go

# Docker
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

# Kubernetes
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/secret.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/service.yaml
	kubectl apply -f k8s/ingress.yaml

k8s-delete:
	@echo "Deleting Kubernetes resources..."
	kubectl delete -f k8s/ --ignore-not-found

k8s-status:
	@echo "Checking deployment status..."
	kubectl get all -n ocr-agent

# 依赖
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# 生成
generate:
	@echo "Running go generate..."
	go generate ./...

# 帮助
help:
	@echo "OCR Agent Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build            - Build the binary"
	@echo "  make build-demo       - Build demo binary"
	@echo "  make test             - Run all tests"
	@echo "  make test-unit        - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-coverage    - Generate test coverage report"
	@echo "  make lint             - Run linter"
	@echo "  make fmt              - Format code"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make run-demo         - Run demo"
	@echo "  make docker-build     - Build Docker image"
	@echo "  make docker-push      - Push Docker image"
	@echo "  make k8s-deploy       - Deploy to Kubernetes"
	@echo "  make k8s-delete       - Delete Kubernetes resources"
	@echo "  make deps             - Install dependencies"
	@echo "  make help             - Show this help"

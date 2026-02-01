# Interview Expert Makefile
.PHONY: all build test clean docker deploy lint fmt help

# Variables
APP_NAME := interview-expert
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-w -s -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOLINT := golangci-lint

# Docker parameters
DOCKER := docker
DOCKER_IMAGE := $(APP_NAME)
DOCKER_TAG := $(VERSION)

# Kubernetes parameters
KUBECTL := kubectl
K8S_NAMESPACE := interview-expert

# Default target
all: lint test build

## Build
build: ## Build the application
	@echo "Building $(APP_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/main.go

build-linux: ## Build for Linux
	@echo "Building $(APP_NAME) for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./cmd/main.go

build-darwin: ## Build for macOS
	@echo "Building $(APP_NAME) for macOS..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME)-darwin-amd64 ./cmd/main.go

build-windows: ## Build for Windows
	@echo "Building $(APP_NAME) for Windows..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(APP_NAME)-windows-amd64.exe ./cmd/main.go

build-all: build-linux build-darwin build-windows ## Build for all platforms

## Test
test: ## Run unit tests
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -race -tags=integration ./...

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

## Lint & Format
lint: ## Run linter
	@echo "Running linter..."
	$(GOLINT) run ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) -s -w .

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## Dependencies
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

deps-tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

## Docker
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	$(DOCKER) build -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		.
	$(DOCKER) tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

docker-push: ## Push Docker image
	@echo "Pushing Docker image..."
	$(DOCKER) push $(DOCKER_IMAGE):$(DOCKER_TAG)
	$(DOCKER) push $(DOCKER_IMAGE):latest

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	$(DOCKER) run -it --rm \
		-p 8080:8080 \
		-p 9090:9090 \
		-v $(PWD)/config.yaml:/etc/interview-expert/config.yaml \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## Kubernetes
k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	$(KUBECTL) apply -f k8s/namespace.yaml
	$(KUBECTL) apply -f k8s/configmap.yaml
	$(KUBECTL) apply -f k8s/secret.yaml
	$(KUBECTL) apply -f k8s/deployment.yaml
	$(KUBECTL) apply -f k8s/service.yaml
	$(KUBECTL) apply -f k8s/ingress.yaml

k8s-delete: ## Delete from Kubernetes
	@echo "Deleting from Kubernetes..."
	$(KUBECTL) delete -f k8s/ --ignore-not-found

k8s-status: ## Check Kubernetes deployment status
	@echo "Checking deployment status..."
	$(KUBECTL) get all -n $(K8S_NAMESPACE)

k8s-logs: ## View Kubernetes logs
	@echo "Viewing logs..."
	$(KUBECTL) logs -f -l app=$(APP_NAME) -n $(K8S_NAMESPACE)

k8s-restart: ## Restart Kubernetes deployment
	@echo "Restarting deployment..."
	$(KUBECTL) rollout restart deployment/$(APP_NAME) -n $(K8S_NAMESPACE)

## Run
run: build ## Build and run the application
	@echo "Running $(APP_NAME)..."
	./bin/$(APP_NAME) serve

run-demo: build ## Run demo mode
	@echo "Running demo..."
	./bin/$(APP_NAME) demo

run-interview: build ## Run interactive interview
	@echo "Running interactive interview..."
	./bin/$(APP_NAME) interview

## Clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

clean-docker: ## Clean Docker images
	@echo "Cleaning Docker images..."
	$(DOCKER) rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true
	$(DOCKER) rmi $(DOCKER_IMAGE):latest || true

## Install
install: build ## Install the application
	@echo "Installing $(APP_NAME)..."
	cp bin/$(APP_NAME) /usr/local/bin/

uninstall: ## Uninstall the application
	@echo "Uninstalling $(APP_NAME)..."
	rm -f /usr/local/bin/$(APP_NAME)

## Help
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

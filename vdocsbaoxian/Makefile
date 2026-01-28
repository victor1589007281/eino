SHELL := /bin/bash
MODULE := github.com/cloudwego/eino/vdocsbaoxian
BINARY := insurance-expert
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date +'%F %T %z')
GO_VERSION := $(shell go version)

LDFLAGS := -s -w \
	-X '$(MODULE)/versioninfo.Version=$(VERSION)' \
	-X '$(MODULE)/versioninfo.GitCommit=$(GIT_COMMIT)' \
	-X '$(MODULE)/versioninfo.BuildTime=$(BUILD_TIME)' \
	-X '$(MODULE)/versioninfo.GoVersion=$(GO_VERSION)'

PKGS := $$(go list ./... | grep -v vendor/)
TEST_PARALLEL_NUM ?= 4
GOCOVERDIR := ./coverage

.PHONY: all build test lint clean docker k8s help

default: build

## Help
help:
	@echo "Insurance Expert Agent - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make build        - Build the binary"
	@echo "  make test         - Run tests"
	@echo "  make lint         - Run linters"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker       - Build Docker image"
	@echo "  make k8s-deploy   - Deploy to Kubernetes"
	@echo "  make k8s-delete   - Delete from Kubernetes"
	@echo ""

## Build
build: deps
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/main.go

build-linux: deps
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-linux ./cmd/main.go

build-darwin: deps
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY)-darwin ./cmd/main.go

## Dependencies
deps:
	go mod download
	go mod tidy

vendor: deps
	go mod vendor

## Format and Lint
fmt:
	go fmt $(PKGS)
	gofmt -s -w .

vet:
	go vet $(PKGS)

lint: fmt vet
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## Test
test:
	mkdir -p $(GOCOVERDIR)
	go test -v -race -cover -coverprofile=$(GOCOVERDIR)/coverage.out \
		-parallel=$(TEST_PARALLEL_NUM) $(PKGS)
	go tool cover -func=$(GOCOVERDIR)/coverage.out | tail -n 1

test-coverage: test
	go tool cover -html=$(GOCOVERDIR)/coverage.out -o $(GOCOVERDIR)/coverage.html
	@echo "Coverage report: $(GOCOVERDIR)/coverage.html"

## Clean
clean:
	rm -rf bin/
	rm -rf $(GOCOVERDIR)
	rm -rf vendor/

## Docker
DOCKER_IMAGE := insurance-expert
DOCKER_TAG := $(VERSION)

docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## Kubernetes
k8s-deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/secret.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/service.yaml
	kubectl apply -f k8s/ingress.yaml

k8s-delete:
	kubectl delete -f k8s/ingress.yaml --ignore-not-found
	kubectl delete -f k8s/service.yaml --ignore-not-found
	kubectl delete -f k8s/deployment.yaml --ignore-not-found
	kubectl delete -f k8s/secret.yaml --ignore-not-found
	kubectl delete -f k8s/configmap.yaml --ignore-not-found

k8s-status:
	kubectl get pods -n insurance-expert
	kubectl get svc -n insurance-expert
	kubectl get ingress -n insurance-expert

## Index
index-build:
	./bin/$(BINARY) index build --config=config.yaml

index-rebuild:
	./bin/$(BINARY) index rebuild --config=config.yaml

## Run
run: build
	./bin/$(BINARY) serve --config=config.yaml

run-dev: build
	./bin/$(BINARY) serve --config=config.dev.yaml --log-level=debug

## Generate
generate:
	go generate ./...

## Version
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(GO_VERSION)"

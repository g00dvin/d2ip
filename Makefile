.PHONY: build test lint docker docker-dev clean proto help

BINARY_NAME=d2ip
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
DEV_IMAGE=d2ip-dev:latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

docker-dev: ## Build development Docker image with cached dependencies
	@echo "Building development image $(DEV_IMAGE)..."
	docker build -f Dockerfile.dev -t $(DEV_IMAGE) .

build: ## Build the binary (uses local Go or docker-dev if available)
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@if command -v go >/dev/null 2>&1 && go version | grep -q "go1.2[2-9]"; then \
		go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/d2ip; \
	elif docker image inspect $(DEV_IMAGE) >/dev/null 2>&1; then \
		echo "Using $(DEV_IMAGE) for build..."; \
		docker run --rm -v $(PWD):/work -w /work $(DEV_IMAGE) go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/d2ip; \
	else \
		echo "No suitable Go found. Run 'make docker-dev' first or install Go 1.22+"; \
		exit 1; \
	fi

proto: docker-dev ## Generate Go code from protobuf definitions
	@echo "Generating protobuf code..."
	@mkdir -p internal/domainlist/dlcpb
	docker run --rm -v $(PWD):/work -w /work $(DEV_IMAGE) sh -c \
		"protoc --go_out=. --go_opt=paths=source_relative proto/dlc.proto && \
		 mv proto/dlc.pb.go internal/domainlist/dlcpb/"

test: ## Run tests (uses local Go or docker-dev if available)
	@echo "Running tests..."
	@if command -v go >/dev/null 2>&1 && go version | grep -q "go1.2[2-9]"; then \
		go test -v -race -coverprofile=coverage.out ./...; \
	elif docker image inspect $(DEV_IMAGE) >/dev/null 2>&1; then \
		echo "Using $(DEV_IMAGE) for tests..."; \
		docker run --rm -v $(PWD):/work -w /work $(DEV_IMAGE) go test -v -race -coverprofile=coverage.out ./...; \
	else \
		echo "No suitable Go found. Run 'make docker-dev' first or install Go 1.22+"; \
		exit 1; \
	fi

lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet instead"; \
		go vet ./...; \
	fi

docker: ## Build production Docker image
	@echo "Building production Docker image..."
	docker build -f deploy/Dockerfile -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out
	go clean

.DEFAULT_GOAL := help

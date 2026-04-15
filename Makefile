.PHONY: build test lint docker clean proto help

BINARY_NAME=d2ip
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/d2ip

proto: ## Generate Go code from protobuf definitions
	@echo "Generating protobuf code..."
	@mkdir -p internal/domainlist/dlcpb
	docker run --rm \
		-v $(PWD):/workspace \
		-w /workspace \
		golang:1.22-alpine \
		sh -c "apk add --no-cache protobuf-dev && \
		       go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0 && \
		       protoc --go_out=. --go_opt=paths=source_relative proto/dlc.proto && \
		       mv proto/dlc.pb.go internal/domainlist/dlcpb/"

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet instead"; \
		go vet ./...; \
	fi

docker: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out
	go clean

.DEFAULT_GOAL := help

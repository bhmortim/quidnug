.PHONY: help build test test-race test-integration cover fmt vet lint run \
        docker-build docker-run clean tools

BINARY_NAME ?= quidnug
DOCKER_IMAGE ?= quidnug:latest
GO_BUILD_FLAGS ?= -trimpath -ldflags="-s -w"
GO_TEST_FLAGS ?= -race

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Available targets:\n"} \
	      /^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' \
	      $(MAKEFILE_LIST)

build: ## Build the node binary into bin/
	@mkdir -p bin
	go build $(GO_BUILD_FLAGS) -o bin/$(BINARY_NAME) ./cmd/quidnug

run: build ## Build and run the node
	./bin/$(BINARY_NAME)

test: ## Run unit tests with race detector
	go test $(GO_TEST_FLAGS) ./...

test-race: test ## Alias for `test` (race detector on by default)

test-integration: ## Run integration-tagged tests
	go test $(GO_TEST_FLAGS) -tags=integration ./...

cover: ## Run tests with coverage report (HTML into coverage.html)
	go test $(GO_TEST_FLAGS) -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt: ## Run gofmt on all Go files
	gofmt -s -w ./cmd ./internal

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

tools: ## Install development tools (golangci-lint, gosec, govulncheck)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

docker-build: ## Build the Docker image
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run the Docker image (publishes 8080)
	docker run --rm -p 8080:8080 $(DOCKER_IMAGE)

clean: ## Remove build artifacts and the Go build cache
	rm -rf bin/ coverage.out coverage.html
	go clean -cache

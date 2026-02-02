.PHONY: build test lint docker-build docker-run clean

BINARY_NAME=quidnug
DOCKER_IMAGE=quidnug:latest
GO_BUILD_FLAGS=-ldflags="-s -w"

build:
	go build $(GO_BUILD_FLAGS) -o bin/$(BINARY_NAME) ./src/core

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker run -p 8080:8080 $(DOCKER_IMAGE)

clean:
	rm -rf bin/
	go clean -cache

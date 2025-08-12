.PHONY: build docker-build docker-push dist deps clean

# Variables
BINARY_NAME := image-shepherd
DOCKER_IMAGE := ghcr.io/hackucf/image-shepherd
BUILD_DIR := build
PACKAGE := ./cmd/image-shepherd

# Go build settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Get version from git tag or use "dev"
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default target
all: build

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build binary
build:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME) $(PACKAGE)

# Build for multiple platforms
dist: clean
	mkdir -p $(BUILD_DIR)
	./scripts/go-build.sh linux amd64 $(BUILD_DIR) $(BINARY_NAME) $(PACKAGE)
	./scripts/go-build.sh linux arm64 $(BUILD_DIR) $(BINARY_NAME) $(PACKAGE)
	./scripts/go-build.sh darwin amd64 $(BUILD_DIR) $(BINARY_NAME) $(PACKAGE)
	./scripts/go-build.sh darwin arm64 $(BUILD_DIR) $(BINARY_NAME) $(PACKAGE)
	./scripts/go-build.sh windows amd64 $(BUILD_DIR) $(BINARY_NAME).exe $(PACKAGE)

# Build Docker image
docker-build:
	docker build -f docker/image-shepherd.Dockerfile -t $(DOCKER_IMAGE) .

# Push Docker image
docker-push:
	./scripts/docker-push.sh $(DOCKER_IMAGE)

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Install binary
install: build
	install -m 755 $(BINARY_NAME) /usr/local/bin/

# Run the application
run: build
	./$(BINARY_NAME)

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-push   - Push Docker image to registry"
	@echo "  dist          - Build distributable archives for multiple platforms"
	@echo "  deps          - Install Go dependencies"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Lint Go code"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install binary to /usr/local/bin"
	@echo "  run           - Build and run the application"
	@echo "  help          - Show this help message"

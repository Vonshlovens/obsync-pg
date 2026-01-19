.PHONY: build build-all install clean test lint fmt migrate run help

# Build variables
BINARY_NAME=obsync-pg
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_FLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
all: build

# Build for current platform
build:
	go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME) ./cmd/obsync-pg

# Build for all platforms
build-all:
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/obsync-pg
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/obsync-pg
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/obsync-pg
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/obsync-pg
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/obsync-pg

# Install to GOPATH/bin
install:
	go install $(BUILD_FLAGS) ./cmd/obsync-pg

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Run database migrations
migrate:
	go run ./cmd/obsync-pg migrate

# Run the daemon (for development)
run:
	go run ./cmd/obsync-pg daemon

# Download dependencies
deps:
	go mod download
	go mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build for current platform"
	@echo "  build-all   - Build for all platforms (macOS, Linux, Windows)"
	@echo "  install     - Install to GOPATH/bin"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
	@echo "  migrate     - Run database migrations"
	@echo "  run         - Run the daemon (development)"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  help        - Show this help"

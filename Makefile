# Makefile for genpass

APP_NAME := genpass
BINARY_DIR := bin
GO_FILES := $(wildcard *.go)
VERSION := 0.0.1
LDFLAGS := -s -w -X main.version=$(VERSION)

# Default target
.PHONY: all
all: build

# Build application
.PHONY: build
build: $(BINARY_DIR)/$(APP_NAME)

$(BINARY_DIR)/$(APP_NAME): $(GO_FILES)
	@mkdir -p $(BINARY_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/$(APP_NAME) .

# Run application
.PHONY: run
run: build
	./$(BINARY_DIR)/$(APP_NAME)

# Format code
.PHONY: fmt
fmt:
	go fmt .

# Code check
.PHONY: vet
vet:
	go vet .

# Run tests
.PHONY: test
test:
	go test -v .

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -v -coverprofile=coverage.out .
	go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
.PHONY: bench
bench:
	go test -bench=. -benchmem .

# Run security scan
.PHONY: security
security:
	@which gosec > /dev/null || go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	gosec ./...

# Install to GOPATH/bin
.PHONY: install
install:
	go install .

# Clean build files
.PHONY: clean
clean:
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html
	go clean

# Cross compile
.PHONY: build-all
build-all: clean
	@mkdir -p $(BINARY_DIR)
	# Linux amd64
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/$(APP_NAME)-linux-amd64 .
	# Linux arm64
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/$(APP_NAME)-linux-arm64 .
	# macOS amd64
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/$(APP_NAME)-darwin-amd64 .
	# macOS arm64
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/$(APP_NAME)-darwin-arm64 .

# Show help
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  build         Build application to $(BINARY_DIR)/ directory"
	@echo "  run           Build and run application"
	@echo "  fmt           Format code"
	@echo "  vet           Static code check"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  bench         Run benchmarks"
	@echo "  security      Run security scan"
	@echo "  install       Install to GOPATH/bin"
	@echo "  clean         Clean build files"
	@echo "  build-all     Cross compile for all platforms"
	@echo "  help          Show this help"

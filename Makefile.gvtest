# Makefile for gvtest - Go port of VTest2

.PHONY: all build test clean run install lint fmt

# Binary name
BINARY=gvtest
# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod

all: fmt build test

build:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY) ./cmd/gvtest

test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

run: build
	@echo "Running $(BINARY)..."
	./$(BUILD_DIR)/$(BINARY)

install:
	@echo "Installing $(BINARY)..."
	$(GOCMD) install ./cmd/gvtest

fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, install it from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

# Help target
help:
	@echo "Available targets:"
	@echo "  all       - Format, build, and test"
	@echo "  build     - Build the binary"
	@echo "  test      - Run tests with race detector"
	@echo "  clean     - Clean build artifacts"
	@echo "  run       - Build and run"
	@echo "  install   - Install to GOPATH/bin"
	@echo "  fmt       - Format code"
	@echo "  lint      - Run linter"
	@echo "  tidy      - Tidy go modules"

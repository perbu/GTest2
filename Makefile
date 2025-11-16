.PHONY: all test test-verbose build clean

# Default target: build the binary and run tests
all: build test

# Build the main binary
build:
	go build -o gtest ./cmd/gtest

# Run tests with proper flags to avoid race conditions
test:
	go test -p=1 -parallel=1 ./... -timeout=30s

# Run tests with verbose output
test-verbose:
	go test -v -p=1 -parallel=1 ./... -timeout=30s

# Clean build artifacts
clean:
	rm -f gtest
	go clean -testcache

# Run tests for a specific package
test-pkg:
	go test -v -timeout=10s ./$(PKG)

# Run a specific test
test-one:
	go test -v -run $(TEST) ./$(PKG)

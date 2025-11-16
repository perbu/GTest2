.PHONY: test test-verbose build clean

# Run tests with proper flags to avoid race conditions
test:
	go test -p=1 -parallel=1 ./... -timeout=30s

# Run tests with verbose output
test-verbose:
	go test -v -p=1 -parallel=1 ./... -timeout=30s

# Build the main binary
build:
	go build -o gvtest ./cmd/gvtest

# Clean build artifacts
clean:
	rm -f gvtest
	go clean -testcache

# Run tests for a specific package
test-pkg:
	go test -v -timeout=10s ./$(PKG)

# Run a specific test
test-one:
	go test -v -run $(TEST) ./$(PKG)

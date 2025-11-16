# GTest - Context for Claude

## Project Overview

GTest is an HTTP testing framework written in Go, designed for testing HTTP clients, servers, and proxies. It's a port of VTest2 from the Varnish project, created with AI assistance. The framework can generate both well-formed and intentionally malformed HTTP traffic to test edge cases and protocol violations.

## Core Purpose

- Execute test scripts (`.vtc` files) that simulate HTTP/1 and HTTP/2 traffic
- Test server/proxy behavior with malformed requests
- Validate protocol compliance and error handling
- Support both well-formed and intentionally broken HTTP traffic

## Architecture

### Directory Structure

```
/cmd/gvtest/         - Main executable
/pkg/
  /vtc/              - VTC parser and executor
  /server/           - HTTP server implementation
  /http1/            - HTTP/1 client/server logic
  /http2/            - HTTP/2 client/server logic
  /macro/            - Macro expansion system
  /logging/          - Logging utilities
/tests/              - VTC test files
```

### Key Components

1. **VTC Parser** (`pkg/vtc/`): Parses VTest Case files
2. **Executor**: Runs test scenarios with servers, clients, and barriers
3. **HTTP/1 Handler** (`pkg/http1/`): HTTP/1.x protocol implementation
4. **HTTP/2 Handler** (`pkg/http2/`): HTTP/2 protocol implementation
5. **Server**: Test HTTP servers that can send custom responses
6. **Client**: Test HTTP clients that can send custom requests

## VTC File Format

Tests use VTC (Varnish Test Case) format:

```vtc
vtest "Test description"

server s1 {
    rxreq                           # receive request
    expect req.method == "GET"      # validate request
    txresp -status 200 -body "OK"   # send response
} -start

client c1 -connect ${s1_sock} {
    txreq -url "/test"              # send request
    rxresp                          # receive response
    expect resp.status == 200       # validate response
} -run
```

### Common Commands

- `rxreq` / `rxresp`: Receive request/response
- `txreq` / `txresp`: Transmit request/response
- `expect`: Assert conditions
- `barrier`: Synchronize multiple clients/servers
- `delay`: Sleep for specified duration

## Build & Test

```bash
# Build
make

# Run single test
./cmd/gvtest/gvtest tests/test.vtc

# Run all tests
./cmd/gvtest/gvtest tests/*.vtc

# Verbose mode
./cmd/gvtest/gvtest -v tests/test.vtc
```

## Development Practices

### Testing
- Primary tests are in `.vtc` files in `/tests/`
- Go unit tests use `_test.go` suffix
- Test both success and failure cases

### Error Handling
- Use descriptive error messages
- Include context about what operation failed
- Propagate errors up the stack appropriately

### Code Style
- Standard Go formatting (gofmt)
- Keep functions focused and small
- Comment complex protocol handling

## Known Limitations

See `LIMITATIONS.md` for comprehensive list. Key items:

1. **Not Implemented**:
   - Terminal emulation for process commands
   - Parallel test execution (`-j` flag)
   - Group checking (`feature group`)
   - Full platform detection

2. **Weak Spots**:
   - Performance not optimized
   - Primary support for Linux only
   - Error messages could be clearer
   - No memory profiling done yet

## Recent Work

- Fixed HTTP/2 connection setup deadlock (PR #15)
- Fixed go.mod module URL (PR #14)
- Resolved test hanging issues

## Common Tasks

### Adding a New VTC Command

1. Define command in `pkg/vtc/commands.go` or specific handler
2. Implement command logic in appropriate package
3. Add to command registry
4. Write test case in `/tests/`

### Debugging Test Failures

1. Run with `-v` flag for verbose output
2. Use `-k` to keep temporary directories
3. Check server/client logs in verbose mode
4. Verify barrier synchronization if using multiple entities

### Adding HTTP/2 Features

- Implement in `pkg/http2/`
- Follow existing patterns from HTTP/1 implementation
- Test with both valid and malformed frame sequences

## Dependencies

- Go 1.24.7 (as per go.mod)
- No external runtime dependencies (statically linked)
- Standard library only

## Protocol Support

- **HTTP/1.0, HTTP/1.1**: Full support
- **HTTP/2**: Core functionality implemented
- Supports both TLS and plain TCP

## Git Workflow

- Develop on feature branches with `claude/` prefix
- Push to `claude/*` branches for review
- Main branch is the merge target

## Useful Context

- This is a testing framework, so unusual/broken HTTP traffic is intentional
- VTC format is from Varnish project
- Focus on protocol edge cases and error handling
- Port from C codebase (VTest2) using AI assistance

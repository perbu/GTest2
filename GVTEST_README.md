# GVTest - Go Implementation of VTest2

## Overview

**GVTest** is a Go port of VTest2, an HTTP testing framework designed to test HTTP clients, servers, and proxies with the unique capability to generate broken/malformed HTTP traffic for edge case testing.

This implementation maintains the core philosophy of VTest2: **byte-level control over HTTP traffic generation** to preserve the ability to produce intentionally broken HTTP messages for comprehensive testing scenarios.

## Key Features

- **HTTP/1.1 Support**: Full control over HTTP/1 message construction
- **HTTP/2 Support**: Complete HTTP/2 frame handling with HPACK compression
- **Malformed Traffic Generation**: Ability to create intentionally broken HTTP messages for edge case testing
- **VTC Language**: Compatible with existing .vtc test files
- **Client/Server Testing**: Simulate both HTTP clients and servers
- **Session Management**: Advanced connection handling with keepalive, repeat counts, and flow control
- **Process Management**: Execute and test external processes
- **Barrier Synchronization**: Coordinate complex test scenarios
- **Cross-Platform**: Runs on Linux, macOS, and Windows

## Installation

### Prerequisites

- Go 1.24.7 or later
- Git (for cloning the repository)

### From Source

```bash
# Clone the repository
git clone https://github.com/perbu/gvtest.git
cd gvtest

# Build the binary
make build

# Or use Go directly
go build -o gvtest ./cmd/gvtest

# Install to $GOPATH/bin
go install ./cmd/gvtest
```

### Using `go install`

```bash
go install github.com/perbu/gvtest/cmd/gvtest@latest
```

## Quick Start

### Basic Usage

```bash
# Run a single test file
./gvtest test.vtc

# Run with verbose output
./gvtest -v test.vtc

# Run with custom timeout
./gvtest -t 30 test.vtc

# Keep temporary directories for debugging
./gvtest -k test.vtc

# Show version
./gvtest -version

# Dump AST (for debugging)
./gvtest -dump-ast test.vtc
```

### Command-Line Options

- `-v`: Verbose output (shows detailed logging)
- `-q`: Quiet mode (minimal output)
- `-k`: Keep temporary directories after test completion
- `-t SECONDS`: Set test timeout (default: 60 seconds)
- `-j JOBS`: Number of parallel jobs (not yet implemented)
- `-dump-ast`: Dump parsed AST and exit
- `-version`: Show version information

## VTC Language Basics

VTC (Varnish Test Case) is a domain-specific language for writing HTTP tests. Here's a quick introduction:

### Simple HTTP Test

```vtc
vtest "Basic HTTP/1.1 test"

server s1 {
    rxreq
    expect req.method == "GET"
    expect req.url == "/"
    txresp -status 200 -body "Hello, World!"
} -start

client c1 -connect ${s1_sock} {
    txreq -method GET -url "/"
    rxresp
    expect resp.status == 200
    expect resp.body == "Hello, World!"
} -run
```

### HTTP/2 Test

```vtc
vtest "Basic HTTP/2 test"

server s1 -h2 {
    stream 1 {
        rxreq
        expect req.method == "GET"
        txresp -status 200
    }
} -start

client c1 -connect ${s1_sock} -h2 {
    stream 1 {
        txreq -method GET -url "/"
        rxresp
        expect resp.status == 200
    }
} -run
```

### Malformed HTTP Test

```vtc
vtest "Test malformed Content-Length"

server s1 {
    rxreq
    # Send response with incorrect Content-Length
    send "HTTP/1.1 200 OK\r\n"
    send "Content-Length: 100\r\n"
    send "\r\n"
    send "Short body"
} -start

client c1 -connect ${s1_sock} {
    txreq
    # Client should handle the malformed response
    recv
}
```

## Architecture

GVTest is organized into several packages:

### Core Packages

- **`pkg/vtc`**: VTC language parser and executor
- **`pkg/logging`**: Thread-safe logging infrastructure
- **`pkg/session`**: Session management and lifecycle

### HTTP Packages

- **`pkg/http1`**: HTTP/1.1 protocol engine with raw socket control
- **`pkg/http2`**: HTTP/2 protocol engine with frame-level control
- **`pkg/hpack`**: HPACK compression/decompression

### Network Packages

- **`pkg/net`**: Socket utilities (TCP, Unix domain sockets)
- **`pkg/client`**: HTTP client implementation
- **`pkg/server`**: HTTP server implementation

### Utility Packages

- **`pkg/barrier`**: Barrier synchronization for concurrent tests
- **`pkg/process`**: External process management
- **`pkg/macro`**: Macro expansion system
- **`pkg/util`**: String utilities and helpers

## VTC Commands Reference

### Test Declaration

```vtc
vtest "test description"
```

### Server Commands

```vtc
server NAME {
    # HTTP commands here
} -start

server NAME -wait      # Wait for server to complete
server NAME -stop      # Stop the server
```

### Client Commands

```vtc
client NAME -connect ADDRESS {
    # HTTP commands here
} -run

client NAME -wait      # Wait for client to complete
```

### HTTP/1.1 Commands

#### Transmit Request
```vtc
txreq -method METHOD -url URL -proto PROTO
      -hdr "Name: Value"
      -body "content"
      -bodylen SIZE
```

#### Transmit Response
```vtc
txresp -status CODE -reason "Reason" -proto PROTO
       -hdr "Name: Value"
       -body "content"
       -bodylen SIZE
```

#### Receive Request
```vtc
rxreq
```

#### Receive Response
```vtc
rxresp
```

#### Expect Assertions
```vtc
expect req.method == "GET"
expect req.url == "/"
expect req.http.host == "example.com"
expect resp.status == 200
expect resp.body ~ "pattern"
```

### HTTP/2 Commands

#### Stream Commands
```vtc
stream ID {
    txreq -method METHOD -url URL
    rxresp
}
```

#### Frame Commands
```vtc
txsettings        # Send SETTINGS frame
rxsettings        # Receive SETTINGS frame
txping            # Send PING frame
rxping            # Receive PING frame
txgoaway          # Send GOAWAY frame
```

### Process Commands

```vtc
process NAME -start COMMAND
process NAME -wait
process NAME -write "data"
process NAME -expect-text "expected output"
```

### Barrier Commands

```vtc
barrier bNAME -start COUNT
barrier bNAME -wait
barrier bNAME -sync
```

### Shell Commands

```vtc
shell "command"
shell -exit CODE "command"
shell -match "pattern" "command"
```

### Utility Commands

```vtc
delay SECONDS                    # Sleep
filewrite FILENAME "content"     # Write file
feature cmd COMMAND              # Check if command exists
```

## Design Decisions

### Why Not Use Go's `net/http`?

GVTest needs raw socket control to generate malformed HTTP messages. Go's `net/http` and `http2` packages enforce correctness and won't allow generation of broken HTTP traffic. Therefore, GVTest implements custom HTTP/1 and HTTP/2 parsers/generators that operate on `[]byte` directly.

### Concurrency Model

GVTest uses Go's native concurrency primitives:
- **Goroutines** replace pthreads for lighter, safer concurrency
- **Channels** for inter-goroutine communication
- **sync.Mutex** and **sync.Cond** for synchronization
- Barrier synchronization uses channels for cross-goroutine coordination

### Compatibility with C Version

GVTest maintains compatibility with the VTC file format from the original C implementation. Existing .vtc test files should work without modification (with documented exceptions).

## Performance

GVTest aims to be within 2x performance of the original C implementation. Performance optimizations include:

- Efficient buffer management
- Minimal allocations in hot paths
- Goroutine pooling where appropriate
- Profile-guided optimization

To profile GVTest:

```bash
go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
go tool pprof cpu.prof
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run specific package tests
go test ./pkg/http1
```

### Current Test Coverage

| Package | Coverage |
|---------|----------|
| logging | 65.6% |
| net | 40.8% |
| session | 36.5% |
| util | 64.0% |
| vtc | 26.3% |
| barrier | 0% (needs tests) |
| client | 0% (needs tests) |
| http1 | 0% (needs tests) |
| http2 | 0% (needs tests) |
| hpack | 0% (needs tests) |

*Goal: 80% coverage across all packages*

## Known Limitations

### From Previous Phases

1. **Terminal Emulation**: Full VT100 terminal emulation (Teken) is not implemented. Basic process I/O works, but advanced terminal-based tests may fail.

2. **Parallel Test Execution**: The `-j` flag is parsed but not yet implemented. Tests run sequentially.

3. **Platform Detection**: Feature detection assumes Linux. Some platform-specific features may need enhancement.

### Phase 6 Status

1. **HAProxy/Varnish Integration**: HAProxy and Varnish-specific features (`vtc_haproxy.c`, `vtc_varnish.c`) are not implemented.

2. **Syslog Support**: Syslog integration (`vtc_syslog.c`) not implemented.

3. **VSM Support**: Varnish Shared Memory support (`vtc_vsm.c`) not implemented.

## Differences from C Version

1. **Error Messages**: Go-style error messages with more context
2. **Temporary Directory**: Uses Go's `os.MkdirTemp` instead of C's `mkdtemp`
3. **Process Management**: Simplified process I/O without full terminal emulation
4. **Logging**: Thread-safe by default using Go's goroutine-safe patterns

## Development

### Project Structure

```
gvtest/
├── cmd/gvtest/          # Main binary
│   ├── main.go          # Entry point
│   └── handlers.go      # Command handlers
├── pkg/                 # Packages
│   ├── vtc/            # VTC language parser & executor
│   ├── logging/        # Logging infrastructure
│   ├── session/        # Session management
│   ├── http1/          # HTTP/1 engine
│   ├── http2/          # HTTP/2 engine
│   ├── hpack/          # HPACK implementation
│   ├── process/        # Process management
│   ├── barrier/        # Barrier synchronization
│   ├── client/         # Client implementation
│   ├── server/         # Server implementation
│   ├── net/            # Socket utilities
│   ├── macro/          # Macro system
│   └── util/           # Utilities
├── tests/              # Integration tests
├── testdata/           # VTC test files
└── Makefile            # Build automation
```

### Building from Source

```bash
# Build
make build

# Run tests
make test

# Clean build artifacts
make clean

# Format code
gofmt -w .

# Run linters
golangci-lint run
```

### Contributing

Contributions are welcome! Please:

1. Write tests for new features
2. Follow Go best practices
3. Add GoDoc comments for public APIs
4. Ensure `go test ./...` passes
5. Run `gofmt` before committing

## Migration from C Version

See [MIGRATION.md](MIGRATION.md) for a detailed guide on migrating from the C version of VTest2 to GVTest.

## Roadmap

### Current Phase: Phase 6 (Polish & Documentation)

- [ ] Complete documentation
- [ ] Increase test coverage to 80%
- [ ] Performance benchmarking
- [ ] Cross-platform testing
- [ ] Release automation

### Future Enhancements

- [ ] Parallel test execution (`-j` flag)
- [ ] Full terminal emulation (Teken port)
- [ ] HAProxy integration (optional)
- [ ] Syslog support (optional)
- [ ] WebSocket support
- [ ] HTTP/3 (QUIC) support

## License

GVTest inherits the license from VTest2. See [LICENSE](LICENSE) file for details.

## Credits

GVTest is a port of VTest2, originally created by:
- Poul-Henning Kamp
- Nils Goroll

Go port by the GVTest contributors.

## Support

- **Issues**: [GitHub Issues](https://github.com/perbu/gvtest/issues)
- **Discussions**: [GitHub Discussions](https://github.com/perbu/gvtest/discussions)
- **Documentation**: [Wiki](https://github.com/perbu/gvtest/wiki)

## Related Projects

- [VTest2](https://github.com/varnishcache/varnish-cache/tree/master/bin/varnishtest) - Original C implementation
- [Varnish Cache](https://varnish-cache.org/) - HTTP accelerator
- [HAProxy](http://www.haproxy.org/) - Load balancer

---

**Version**: 0.6.0 (Phase 6)

**Status**: In Development

**Last Updated**: 2025-11-16

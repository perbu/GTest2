# VTest2 to Go Port - Implementation Plan

## Executive Summary

This document outlines a phased approach to port VTest2 (Varnishtest) from C to Go. VTest2 is an HTTP testing framework (~27,000 lines of C) specifically designed to test HTTP clients, servers, and proxies with the unique capability to generate broken/malformed HTTP traffic (both HTTP/1 and HTTP/2) for edge case testing.

**Critical Requirement**: The Go implementation must maintain byte-level control over HTTP traffic generation to preserve the ability to produce intentionally broken HTTP messages.

## Project Scope

### What We're Porting
- **Core Components**: ~45 C source files + ~50 library files
- **Test Framework**: VTC (Varnish Test Case) language parser and executor
- **HTTP Engines**:
  - HTTP/1 implementation with fine-grained control (~2,000 LOC)
  - HTTP/2 implementation with HPACK support (~3,000 LOC)
- **Session Management**: Client/server abstractions with threading
- **Utilities**: Process management, barriers, tunnels, logging
- **Test Suite**: 58 .vtc test files

### Key Architectural Decisions

1. **Do NOT use Go's net/http**: We need raw socket control to generate malformed HTTP
2. **Do NOT use Go's http2 package**: Same reason - we need byte-level control
3. **DO use Go's concurrency primitives**: Replace pthreads with goroutines and channels
4. **DO leverage Go's standard library**: For networking, parsing, testing infrastructure
5. **MAINTAIN compatibility**: Same .vtc test file format and command-line interface

---

## Phase 1: Foundation & Core Infrastructure (Weeks 1-3)

**Goal**: Establish the Go project structure and implement core utilities that everything else depends on.

### 1.1 Project Setup
- [ ] Create Go module structure (`go.mod`, `go.sum`)
- [ ] Set up directory layout:
  ```
  gvtest/
  ├── cmd/gvtest/          # Main binary
  ├── pkg/
  │   ├── vtc/            # VTC language parser
  │   ├── logging/        # Logging infrastructure
  │   ├── session/        # Session management
  │   ├── http1/          # HTTP/1 engine
  │   ├── http2/          # HTTP/2 engine
  │   ├── hpack/          # HPACK implementation
  │   ├── process/        # Process management
  │   ├── barrier/        # Barrier synchronization
  │   └── util/           # Shared utilities
  ├── internal/           # Internal packages
  ├── testdata/          # Port of .vtc files
  └── tests/             # Go unit tests
  ```
- [ ] Set up CI/CD (GitHub Actions or similar)
- [ ] Create Makefile with build, test, and run targets

### 1.2 Logging Infrastructure (Port from vtc_log.c)
**Files**: `vtc_log.c`, `vtc_log.h` → `pkg/logging/`

- [ ] Implement `Logger` interface with levels (1-4)
- [ ] Hexdump functionality (port `vtc_hexdump`)
- [ ] Thread-safe logging with context
- [ ] Buffer management for test output capture
- [ ] Dump functionality for debugging binary data

**Deliverable**: `pkg/logging/` package with:
- `type Logger interface`
- `func NewLogger(id string) Logger`
- `func (l *Logger) Dump(level int, prefix, data string)`
- `func (l *Logger) Hexdump(level int, prefix string, data []byte)`

### 1.3 String Buffer and Utilities
**Files**: `lib/vsb.*`, `lib/vav.*`, `lib/vct.*` → `pkg/util/`

- [ ] Port VTC string utilities (or use Go strings package)
- [ ] Character class utilities (vct.c) → Go character classification
- [ ] Argument vector parsing (vav.c) → Go string tokenization
- [ ] Macro expansion system (from vtc.c)

**Deliverable**: `pkg/util/` package with string manipulation helpers

### 1.4 Test Macro System
**Files**: Parts of `vtc.c` → `pkg/vtc/macro.go`

- [ ] Macro definition storage (`map[string]string`)
- [ ] Macro expansion with `${name}` syntax
- [ ] External macro definitions (`extmacro_def`)
- [ ] Instance-based macro scoping

**Deliverable**: Macro expansion system integrated with VTC parser

### 1.5 VTC Language Lexer/Parser (Basic)
**Files**: `vtc.c` → `pkg/vtc/`

- [ ] Tokenizer for VTC language
- [ ] Parse basic structure:
  - Test name: `vtest "description"`
  - Comments and whitespace
  - String literals (double-quoted and curly-braced)
  - Command arguments
- [ ] AST representation of VTC files
- [ ] Error reporting with line numbers

**Deliverable**: `pkg/vtc/parser.go` that can parse basic .vtc files into AST

**Phase 1 Success Criteria**:
- ✅ Can parse simple .vtc files (e.g., `tests/a00002.vtc`)
- ✅ Macro expansion working
- ✅ Logging infrastructure operational
- ✅ Clean error messages with line numbers

---

## Phase 2: Session & Connection Management (Weeks 4-6)

**Goal**: Implement the client/server abstraction and network connection handling.

### 2.1 Session Abstraction
**Files**: `vtc_sess.c`, `vtc_sess.h` → `pkg/session/`

- [ ] `Session` struct with:
  - Name, connection FD
  - Repeat count, keepalive flag
  - Receive buffer management
- [ ] Session lifecycle (New, Start, Stop, Destroy)
- [ ] Socket options (receive buffer size)

**Deliverable**: `pkg/session/session.go`

### 2.2 Client Implementation
**Files**: `vtc_client.c` → `pkg/client/`

- [ ] Client struct with connection management
- [ ] `-connect` address parsing (IP:PORT or /unix/socket)
- [ ] `-start`, `-wait`, `-run` lifecycle commands
- [ ] `-repeat` and `-keepalive` support
- [ ] PROXY protocol v1/v2 support (`-proxy1`, `-proxy2`)
- [ ] Connection establishment (TCP and Unix domain sockets)

**Deliverable**: `pkg/client/client.go` with `Client` type

### 2.3 Server Implementation
**Files**: `vtc_server.c` → `pkg/server/`

- [ ] Server struct with listening socket
- [ ] `-listen` address binding (IP:PORT or /unix/socket)
- [ ] `-start`, `-wait`, `-break` commands
- [ ] Connection acceptance loop
- [ ] `-dispatch` mode (s0 only - multiple concurrent connections)
- [ ] Goroutine per connection
- [ ] Export `sNAME_addr`, `sNAME_port`, `sNAME_sock` macros

**Deliverable**: `pkg/server/server.go` with `Server` type

### 2.4 Socket Utilities
**Files**: `lib/vtcp.*`, `lib/vus.*`, `lib/vsa.*` → `pkg/net/`

- [ ] Address parsing (IP, port, Unix sockets)
- [ ] TCP connection helpers
- [ ] Unix domain socket helpers
- [ ] Socket option setting
- [ ] Non-blocking I/O if needed

**Deliverable**: `pkg/net/socket.go`

### 2.5 VTC Command Registration
**Files**: `cmds.h`, parts of `vtc.c` → `pkg/vtc/commands.go`

- [ ] Command registry system
- [ ] Register `client` and `server` commands
- [ ] Command dispatch mechanism
- [ ] Global vs. top-level command distinction

**Deliverable**: Command registration framework

**Phase 2 Success Criteria**:
- ✅ Can start a server on a random port
- ✅ Can connect a client to the server
- ✅ Macros like `${s1_sock}` work correctly
- ✅ Basic client-server tests pass (without HTTP yet)

---

## Phase 3: HTTP/1 Protocol Engine (Weeks 7-10)

**Goal**: Implement HTTP/1 with full control over message construction, including ability to generate malformed messages.

### 3.1 HTTP Session Structure
**Files**: `vtc_http.h`, `vtc_http.c` (structure) → `pkg/http1/`

- [ ] `HTTP` struct with:
  - Session reference
  - Request/response headers (arrays)
  - Body buffer
  - Timeout
  - Gzip state
  - Fatal error flag
- [ ] Raw receive buffer management
- [ ] Line-based parsing for headers
- [ ] Chunked encoding state

**Deliverable**: `pkg/http1/http.go` with base `HTTP` type

### 3.2 HTTP Request Transmission (txreq)
**Files**: `vtc_http.c` (cmd_http_txreq) → `pkg/http1/txreq.go`

- [ ] `-method`, `-url`, `-proto` arguments
- [ ] `-hdr` for arbitrary headers
- [ ] `-hdrlen` for header with specific length
- [ ] Body transmission:
  - `-body` (string body)
  - `-bodylen` (generated body of specific length)
  - `-gzip` (compress body)
- [ ] Chunked encoding (`-chunked`)
- [ ] **CRITICAL**: Raw byte-level control - construct HTTP manually, don't use libraries
- [ ] Support for intentionally malformed requests:
  - Invalid HTTP version strings
  - Malformed headers
  - Incorrect Content-Length
  - Invalid chunk sizes

**Deliverable**: `txreq` command implementation

### 3.3 HTTP Response Transmission (txresp)
**Files**: `vtc_http.c` (cmd_http_txresp) → `pkg/http1/txresp.go`

- [ ] `-status`, `-reason`, `-proto` arguments
- [ ] `-hdr`, `-hdrlen`, `-body`, `-bodylen`, `-gzip`
- [ ] Special responses:
  - `-nolen` (no Content-Length)
  - `-chunked`
- [ ] Same byte-level control as txreq
- [ ] Support for broken responses

**Deliverable**: `txresp` command implementation

### 3.4 HTTP Request Reception (rxreq)
**Files**: `vtc_http.c` (cmd_http_rxreq) → `pkg/http1/rxreq.go`

- [ ] Read and parse request line
- [ ] Header parsing into array
- [ ] Body reception based on:
  - Content-Length
  - Chunked encoding
  - Connection close
- [ ] Handle malformed input gracefully for testing
- [ ] Populate request structure for `expect` checks

**Deliverable**: `rxreq` command implementation

### 3.5 HTTP Response Reception (rxresp)
**Files**: `vtc_http.c` (cmd_http_rxresp) → `pkg/http1/rxresp.go`

- [ ] Read and parse status line
- [ ] Header parsing
- [ ] Body reception (same logic as rxreq)
- [ ] `-no_obj` flag (read headers only)
- [ ] Handle malformed responses for testing

**Deliverable**: `rxresp` command implementation

### 3.6 HTTP Expect Commands
**Files**: `vtc_subr.c` (vtc_expect), `vtc_http.c` → `pkg/http1/expect.go`

- [ ] Parse expect expressions:
  - `expect req.method == GET`
  - `expect resp.status == 200`
  - `expect resp.http.content-type == "text/html"`
- [ ] Comparison operators: `==`, `!=`, `<`, `>`, `<=`, `>=`, `~` (regex)
- [ ] Field extraction:
  - `req.method`, `req.url`, `req.proto`
  - `resp.status`, `resp.reason`, `resp.proto`
  - `req.http.headername`, `resp.http.headername`
  - `req.body`, `resp.body`, `req.bodylen`, `resp.bodylen`
- [ ] Error reporting on mismatch

**Deliverable**: `expect` command for HTTP

### 3.7 Additional HTTP Commands
**Files**: Various in `vtc_http.c` → `pkg/http1/commands.go`

- [ ] `send` - send raw bytes
- [ ] `sendhex` - send hex-encoded bytes
- [ ] `recv` - receive bytes with timeout
- [ ] `timeout` - set timeout
- [ ] `close` / `accept` - connection control
- [ ] `send_urgent` - TCP urgent data
- [ ] `txpri` / `rxpri` (for HTTP/2 preface)

**Deliverable**: Complete HTTP/1 command set

### 3.8 Gzip Support
**Files**: `vtc_gzip.c` → `pkg/http1/gzip.go`

- [ ] Compress body with `-gzip -gziplevel N`
- [ ] Decompress received body
- [ ] `-gzipresidual` for testing partial decompression

**Deliverable**: Gzip compression/decompression using Go's compress/gzip

**Phase 3 Success Criteria**:
- ✅ Can run basic HTTP/1 tests (e.g., `tests/a00002.vtc`)
- ✅ Can send and receive well-formed HTTP/1 messages
- ✅ Can intentionally send malformed HTTP/1 messages
- ✅ All HTTP/1 expect assertions work
- ✅ Gzip compression/decompression works

---

## Phase 4: HTTP/2 Protocol Engine (Weeks 11-16)

**Goal**: Implement HTTP/2 with HPACK, including the ability to generate malformed frames.

### 4.1 HTTP/2 Frame Structure
**Files**: `vtc_http2.c`, `tbl/h2_frames.h` → `pkg/http2/frame.go`

- [ ] Frame header (9 bytes): length, type, flags, stream ID
- [ ] Frame types enum:
  - DATA, HEADERS, PRIORITY, RST_STREAM
  - SETTINGS, PUSH_PROMISE, PING, GOAWAY
  - WINDOW_UPDATE, CONTINUATION
- [ ] Frame flags (END_STREAM, END_HEADERS, PADDED, PRIORITY, ACK)
- [ ] Raw frame read/write functions
- [ ] **CRITICAL**: Byte-level frame construction for malformed frames

**Deliverable**: `pkg/http2/frame.go` with frame types and I/O

### 4.2 HPACK Implementation
**Files**: `vtc_h2_hpack.c`, `vtc_h2_tbl.c`, `vtc_h2_enctbl.h`, `hpack.h` → `pkg/hpack/`

**Option A**: Port existing C HPACK implementation
- [ ] Header compression context
- [ ] Dynamic table management
- [ ] Static table (RFC 7541)
- [ ] Huffman encoding/decoding (from `tbl/vhp_huffman.h`)
- [ ] Encoder: literal/indexed/never-indexed
- [ ] Decoder with table updates

**Option B**: Use existing Go HPACK library (e.g., `golang.org/x/net/http2/hpack`)
- [ ] Evaluate if it allows enough control for broken HPACK
- [ ] Wrapper for our API if using external library
- [ ] **Decision point**: Can we generate malformed HPACK? If not, must port C code.

**Deliverable**: `pkg/hpack/` package with encode/decode functions

### 4.3 HTTP/2 Stream Management
**Files**: `vtc_http2.c` (stream struct) → `pkg/http2/stream.go`

- [ ] Stream struct:
  - Stream ID, name
  - Request/response headers
  - Body buffer
  - Window size (self and peer)
  - Wait/signal mechanism (channel in Go)
- [ ] Stream state machine (idle, open, half-closed, closed)
- [ ] Stream storage and lookup
- [ ] Goroutine per stream

**Deliverable**: `pkg/http2/stream.go`

### 4.4 HTTP/2 Connection Setup
**Files**: `vtc_http2.c` (start_h2, stop_h2) → `pkg/http2/conn.go`

- [ ] Connection preface exchange (`PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n`)
- [ ] Initial SETTINGS frame exchange
- [ ] Connection-level window management
- [ ] Frame receive loop (goroutine)
- [ ] Frame dispatch to streams
- [ ] `-h2` flag to enable HTTP/2 mode

**Deliverable**: HTTP/2 connection establishment

### 4.5 HTTP/2 Stream Commands
**Files**: `vtc_http2.c` (cmd_stream) → `pkg/http2/stream_commands.go`

- [ ] `stream` command with stream ID
- [ ] `txreq` - send HEADERS frame with pseudo-headers
  - `:method`, `:path`, `:scheme`, `:authority`
  - Optional DATA frame
  - END_STREAM flag
- [ ] `txresp` - send HEADERS with `:status`
- [ ] `rxreq`, `rxresp` - receive and parse HEADERS
- [ ] `txdata`, `rxdata` - send/receive DATA frames
- [ ] `expect` - assertions on stream state

**Deliverable**: Stream command implementations

### 4.6 HTTP/2 Frame-Level Commands
**Files**: `vtc_http2.c` (frame commands) → `pkg/http2/frame_commands.go`

- [ ] `txpri` - send connection preface
- [ ] `rxpri` - receive and verify preface
- [ ] `txsettings` - send SETTINGS frame
- [ ] `rxsettings` - receive SETTINGS
- [ ] `txping`, `rxping` - PING frames
- [ ] `txgoaway`, `rxgoaway` - GOAWAY frames
- [ ] `txrst`, `rxrst` - RST_STREAM
- [ ] `txwinup`, `rxwinup` - WINDOW_UPDATE
- [ ] `txpush`, `rxpush` - PUSH_PROMISE
- [ ] `txcont` - CONTINUATION
- [ ] **Raw frame commands**:
  - `sendhex` - send arbitrary bytes
  - `write` - write raw frame with specified type/flags/stream
  - Allow invalid frame constructions

**Deliverable**: Complete HTTP/2 frame command set

### 4.7 HTTP/2 Flow Control
**Files**: `vtc_http2.c` (window management) → `pkg/http2/flow.go`

- [ ] Connection window tracking
- [ ] Per-stream window tracking
- [ ] WINDOW_UPDATE generation
- [ ] Flow control enforcement (with `-wf` flag to disable)

**Deliverable**: Flow control implementation

### 4.8 HTTP/2 Settings and Priority
**Files**: `vtc_http2.c`, `tbl/h2_settings.h` → `pkg/http2/settings.go`

- [ ] SETTINGS frame handling:
  - HEADER_TABLE_SIZE
  - ENABLE_PUSH
  - MAX_CONCURRENT_STREAMS
  - INITIAL_WINDOW_SIZE
  - MAX_FRAME_SIZE
  - MAX_HEADER_LIST_SIZE
- [ ] Priority frame (optional, can be no-op with `-no-rfc7540-priorities`)
- [ ] Settings ACK

**Deliverable**: Settings negotiation

**Phase 4 Success Criteria**:
- ✅ Can run basic HTTP/2 tests
- ✅ HPACK compression/decompression works
- ✅ Can send/receive proper HTTP/2 frames
- ✅ Can generate malformed HTTP/2 frames
- ✅ Stream multiplexing works
- ✅ Flow control works (or can be disabled)

---

## Phase 5: Test Execution & Additional Features (Weeks 17-20)

**Goal**: Complete the test framework and implement remaining features.

### 5.1 VTC Execution Engine
**Files**: `vtc_main.c`, `vtc.c` → `cmd/gvtest/main.go`, `pkg/vtc/executor.go`

- [ ] Command-line argument parsing:
  - `-v` (verbose)
  - `-q` (quiet)
  - `-D name=value` (define macro)
  - `-n` (number of parallel tests)
  - `-t` (timeout)
  - `-k` (keep temp directories)
  - `-j` (jobs)
- [ ] Test file discovery
- [ ] Temporary directory creation per test
- [ ] Test execution:
  - Parse .vtc file
  - Execute commands sequentially
  - Capture output
  - Report pass/fail/skip
- [ ] Parallel test execution (goroutine pool)
- [ ] Exit code handling (0=pass, 1=fail, 77=skip, 2=error)

**Deliverable**: Working `gvtest` binary

### 5.2 Barrier Synchronization
**Files**: `vtc_barrier.c` → `pkg/barrier/`

- [ ] Named barriers with countdown
- [ ] `barrier bNAME -start`, `-wait`, `-sync`
- [ ] Cross-goroutine synchronization using channels
- [ ] Timeout support

**Deliverable**: `pkg/barrier/barrier.go`

### 5.3 Process Management
**Files**: `vtc_process.c`, `teken.c`, `teken_subr.h` → `pkg/process/`

- [ ] `process` command to spawn external programs
- [ ] Process I/O capture (stdout, stderr)
- [ ] Terminal emulation (Teken):
  - VT100 escape sequence handling
  - Screen buffer
  - Cursor position
- [ ] Process commands:
  - `-start`, `-wait`, `-stop`, `-kill`
  - `-write`, `-writeln`, `-writehex`
  - `-expect-text` (at row/col)
  - `-expect-cursor`
  - `-screen_dump`

**Decision Point**: Full Teken emulation is complex (~2000 LOC). Options:
- Port Teken to Go
- Use simpler PTY library
- Skip terminal emulation initially (tests can still run without it)

**Deliverable**: Basic process management (defer full terminal emulation if time-constrained)

### 5.4 Shell Command
**Files**: `vtc.c` (cmd_shell) → `pkg/vtc/shell.go`

- [ ] Execute shell commands via `os/exec`
- [ ] Capture output
- [ ] `-exit` to expect specific exit code
- [ ] `-match` for output matching
- [ ] `-expect` for exact output match

**Deliverable**: `shell` command

### 5.5 Delay and Timing
**Files**: `vtc.c` (cmd_delay) → `pkg/vtc/delay.go`

- [ ] `delay` command (sleep)
- [ ] Timeout infrastructure

**Deliverable**: `delay` command

### 5.6 Feature Detection
**Files**: `vtc_misc.c` (cmd_feature) → `pkg/vtc/feature.go`

- [ ] `feature` command to skip tests based on:
  - `cmd` (command availability)
  - `user` (run as specific user)
  - `group` (run as specific group)
  - Platform detection
- [ ] Return exit code 77 to skip test

**Deliverable**: `feature` command

### 5.7 File Operations
**Files**: `vtc_misc.c` (cmd_filewrite) → `pkg/vtc/file.go`

- [ ] `filewrite` command
- [ ] Template variable substitution
- [ ] `-append` flag

**Deliverable**: `filewrite` command

### 5.8 Tunnel/Proxy
**Files**: `vtc_tunnel.c` → `pkg/tunnel/` (optional)

- [ ] TCP tunnel between connections
- [ ] Proxy protocol support

**Deliverable**: Basic tunnel (if time permits)

### 5.9 Test Suite Validation
- [ ] Port all 58 .vtc test files to `testdata/`
- [ ] Run test suite
- [ ] Fix compatibility issues
- [ ] Document any intentional differences from C version

**Deliverable**: Passing test suite

**Phase 5 Success Criteria**:
- ✅ `gvtest` can execute .vtc files
- ✅ Parallel test execution works
- ✅ At least 80% of original test suite passes
- ✅ Command-line interface matches C version
- ✅ Can run same tests with both C and Go versions

---

## Phase 6: Polish, Documentation & Compatibility (Weeks 21-24)

**Goal**: Ensure production readiness and full compatibility.

### 6.1 Performance Optimization
- [ ] Profile with `pprof`
- [ ] Optimize hot paths (especially HTTP parsing)
- [ ] Goroutine leak detection
- [ ] Memory allocation optimization

### 6.2 Error Handling Improvements
- [ ] Consistent error types
- [ ] Stack traces for fatal errors
- [ ] Better error messages (Go-style, not C-style)
- [ ] Graceful handling of malformed .vtc files

### 6.3 Documentation
- [ ] README.md with installation and usage
- [ ] GoDoc comments for all public APIs
- [ ] Migration guide from C version
- [ ] Examples directory with common patterns
- [ ] Document differences from C version

### 6.4 Extended Testing
- [ ] Unit tests for all packages (target: 80% coverage)
- [ ] Integration tests
- [ ] Fuzzing for parsers (VTC, HTTP)
- [ ] Test with real-world HTTP servers (Varnish, HAProxy, Nginx)

### 6.5 Additional Features (if time permits)
- [ ] Syslog support (`vtc_syslog.c`)
- [ ] HAProxy specific features (`vtc_haproxy.c`)
- [ ] Varnish integration (`vtc_varnish.c`) - likely skip, very Varnish-specific
- [ ] VSM support (`vtc_vsm.c`) - likely skip

### 6.6 Binary Distribution
- [ ] Cross-compilation for Linux, macOS, Windows
- [ ] Release automation
- [ ] Docker image
- [ ] Installation via `go install`

### 6.7 Final Validation
- [ ] Run full test suite against both C and Go versions
- [ ] Verify identical behavior on key tests
- [ ] Performance benchmarking (Go version should be comparable)
- [ ] Document known differences

**Phase 6 Success Criteria**:
- ✅ 100% of test suite passes (or documented differences)
- ✅ Performance within 2x of C version
- ✅ Complete documentation
- ✅ Ready for production use
- ✅ Can replace C version for most use cases

---

## Critical Technical Decisions

### 1. HTTP Library Usage
**Decision**: Do NOT use `net/http` or `golang.org/x/net/http2`

**Rationale**: These libraries enforce correctness and won't allow us to generate broken HTTP. We need raw socket control.

**Implementation**: Write custom HTTP/1 and HTTP/2 parsers/generators that operate on `[]byte` directly.

### 2. HPACK Library
**Decision**: Evaluate `golang.org/x/net/http2/hpack` first

**Rationale**: HPACK is complex. If the existing library allows us to generate malformed HPACK, use it. Otherwise, port C code.

**Test**: Can we use the library to create invalid header table entries? Invalid Huffman encoding?

### 3. Terminal Emulation
**Decision**: De-prioritize or use existing Go PTY library

**Rationale**: Teken is 2000+ lines for a small subset of tests. Process management without full terminal emulation covers most use cases.

**Fallback**: Skip terminal emulation tests initially, add later if needed.

### 4. Concurrency Model
**Decision**: Replace pthreads with goroutines

**Rationale**: Go's goroutines are lighter and easier to manage than pthreads.

**Implementation**:
- Each client/server connection → goroutine
- Barrier synchronization → channels
- Mutex/cond → sync.Mutex + channels

### 5. Backward Compatibility
**Decision**: Maintain .vtc file format compatibility

**Rationale**: Users should be able to run existing test files without modification.

**Trade-offs**: May require some compromises in Go idioms to match C behavior exactly.

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| HPACK complexity prevents proper porting | High | Evaluate early in Phase 4; allocate extra time if needed |
| Can't generate broken HTTP/2 with Go libs | High | Plan to write from scratch; don't rely on stdlib |
| Terminal emulation too complex | Medium | Make it optional; focus on HTTP testing first |
| Performance significantly worse than C | Medium | Profile early; optimize hot paths; acceptable if within 2x |
| Test suite incompatibility | Medium | Document differences; provide migration guide |
| Scope creep (Varnish-specific features) | Low | Explicitly descope vtc_varnish.c and vtc_vsm.c |

---

## Success Metrics

### Functional Completeness
- [ ] Can execute all HTTP/1 tests from original suite
- [ ] Can execute all HTTP/2 tests from original suite
- [ ] Can generate intentionally broken HTTP/1 messages
- [ ] Can generate intentionally broken HTTP/2 frames
- [ ] Command-line interface 100% compatible

### Quality Metrics
- [ ] Test coverage: >80%
- [ ] Zero data races (verified with `go test -race`)
- [ ] No goroutine leaks
- [ ] All linters pass (`golangci-lint`)

### Performance Metrics
- [ ] Test execution time within 2x of C version
- [ ] Memory usage reasonable (<2x of C version)
- [ ] Can handle concurrent connections efficiently

### Usability
- [ ] Clear error messages
- [ ] Good documentation
- [ ] Easy installation (`go install`)
- [ ] Works on Linux, macOS, Windows

---

## Deliverables Summary

### Phase 1
- Go project structure
- Logging infrastructure
- VTC parser (basic)
- Macro system

### Phase 2
- Client/Server implementations
- Session management
- Connection handling
- Command registry

### Phase 3
- Complete HTTP/1 engine
- txreq/txresp/rxreq/rxresp
- Expect assertions
- Gzip support

### Phase 4
- HTTP/2 frame handling
- HPACK implementation
- Stream management
- Frame-level commands

### Phase 5
- Working `gvtest` binary
- Test execution engine
- Process/barrier/shell commands
- Test suite validation

### Phase 6
- Documentation
- Performance tuning
- Extended testing
- Release artifacts

---

## Timeline

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Phase 1 | 3 weeks | Foundation & parser |
| Phase 2 | 3 weeks | Session management |
| Phase 3 | 4 weeks | HTTP/1 engine |
| Phase 4 | 6 weeks | HTTP/2 engine |
| Phase 5 | 4 weeks | Test framework |
| Phase 6 | 4 weeks | Polish & release |
| **Total** | **24 weeks** | **Production-ready gvtest** |

**Aggressive timeline**: 20 weeks (cut Phase 6 short)
**Conservative timeline**: 28 weeks (buffer for unknowns)

---

## Conclusion

This port is ambitious but achievable. The key challenges are:

1. **Maintaining byte-level control** over HTTP generation (solved by avoiding stdlib HTTP)
2. **HPACK complexity** (mitigated by evaluating existing Go libs first)
3. **HTTP/2 state machine** (well-documented in RFC 7540, just needs careful implementation)
4. **Test compatibility** (addressed by maintaining VTC format)

The phased approach allows for incremental validation. At the end of each phase, we have working deliverables that can be tested independently.

The Go version will benefit from:
- Better concurrency primitives (goroutines vs pthreads)
- Memory safety (no buffer overflows)
- Easier cross-compilation
- Modern tooling (testing, profiling, race detection)
- More maintainable codebase

This makes the port worthwhile even though it's a significant effort.

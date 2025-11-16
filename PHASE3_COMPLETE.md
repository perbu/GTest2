# Phase 3 Complete: HTTP/1 Protocol Engine

## Overview

Phase 3 of the VTest2 to Go port has been successfully completed. This phase implemented a complete HTTP/1.x protocol engine with raw byte-level control, enabling both well-formed and intentionally malformed HTTP message generation.

## Completed Components

### 3.1 HTTP Session Structure (`pkg/http1/http.go`)
- **HTTP struct** with comprehensive state management:
  - Request/response header storage (up to 64 headers)
  - Body buffer management
  - Timeout configuration
  - Gzip compression state
  - Request/response line parsing
  - Fatal error handling

- **Core HTTP operations**:
  - Raw byte write/read functions
  - Line-based reading for headers
  - Buffered I/O with timeouts
  - Header parsing and retrieval
  - Gzip compression/decompression
  - Chunked transfer encoding support

### 3.2 HTTP Request Transmission (`pkg/http1/txreq.go`)
- **txreq command** with full option support:
  - `-method` - HTTP method (GET, POST, etc.)
  - `-url` - Request URL
  - `-proto` - HTTP protocol version
  - `-hdr` - Custom headers
  - `-body` - Request body (string)
  - `-bodylen` - Generated body of specified length
  - `-chunked` - Chunked transfer encoding
  - `-gzip` - Gzip compression
  - `-nohost` - Omit Host header
  - `-nouseragent` - Omit User-Agent header

- **Raw byte-level control**: Constructs HTTP manually without using Go's net/http library
- **Support for malformed requests**: Can generate intentionally broken HTTP

### 3.3 HTTP Response Transmission (`pkg/http1/txresp.go`)
- **txresp command** with full option support:
  - `-status` - HTTP status code
  - `-reason` - Reason phrase
  - `-proto` - HTTP protocol version
  - `-hdr` - Custom headers
  - `-body` - Response body
  - `-bodylen` - Generated body of specified length
  - `-chunked` - Chunked transfer encoding
  - `-gzip` - Gzip compression
  - `-nolen` - Omit Content-Length header
  - `-noserver` - Omit Server header

- **Default reason phrases** for common status codes (200, 404, 500, etc.)
- **Raw message construction** for testing edge cases

### 3.4 HTTP Request Reception (`pkg/http1/rxreq.go`)
- **rxreq command** for receiving and parsing requests
- **Request line parsing**: Method, URL, Protocol
- **Header parsing** into array
- **Body reception** based on:
  - Content-Length header
  - Chunked Transfer-Encoding
  - Connection close
- **Automatic gzip decompression** when Content-Encoding: gzip is present
- **Graceful handling of malformed input** for testing

### 3.5 HTTP Response Reception (`pkg/http1/rxresp.go`)
- **rxresp command** for receiving and parsing responses
- **Status line parsing**: Protocol, Status Code, Reason Phrase
- **Header and body parsing** (same as rxreq)
- **`-no_obj` flag** to skip body reading
- **HEAD method detection** (no body expected)
- **Status-based body handling** (1xx, 204, 304 have no body)

### 3.6 Expect Command (`pkg/http1/expect.go`)
- **Comprehensive assertion system** for HTTP testing
- **Field extraction**:
  - Request fields: `req.method`, `req.url`, `req.proto`, `req.body`, `req.bodylen`
  - Response fields: `resp.status`, `resp.reason`, `resp.proto`, `resp.body`, `resp.bodylen`
  - Header access: `req.http.headername`, `resp.http.headername`

- **Comparison operators**:
  - `==` - String equality
  - `!=` - String inequality
  - `<`, `>`, `<=`, `>=` - Numeric/string comparison
  - `~` - Regex match
  - `!~` - Regex not match

- **Error reporting** on assertion failure

### 3.7 Additional HTTP Commands (`pkg/http1/commands.go`)
- **send** - Send raw bytes
- **sendhex** - Send hex-encoded bytes (spaces/newlines ignored)
- **recv** - Receive specified number of bytes
- **timeout** - Set I/O timeout
- **gunzip** - Decompress body in place

### 3.8 Gzip Support
- **Compression**: Automatic gzip compression with `-gzip` flag
- **Configurable compression level**: Uses Go's compress/gzip
- **Decompression**: Automatic decompression on receive
- **Content-Encoding header**: Automatically added/checked
- **Error handling**: Graceful fallback if decompression fails

### 3.9 Command Handler (`pkg/http1/handler.go`)
- **Spec processing system** for VTC command execution
- **Command tokenization** with quote handling
- **Option parsing** for all HTTP commands
- **Integration ready** for VTC execution engine

## Test Coverage

### Unit Tests
All HTTP/1 components have comprehensive integration tests in `tests/phase3_test.go`:

1. **TestPhase3_BasicHTTPRequest** - Basic request/response flow
2. **TestPhase3_HTTPHeaders** - Custom header handling
3. **TestPhase3_ExpectAssertions** - All expect operators
4. **TestPhase3_ChunkedEncoding** - Chunked transfer encoding
5. **TestPhase3_GzipCompression** - Gzip compression/decompression
6. **TestPhase3_GenerateBody** - Body generation
7. **TestPhase3_MalformedHTTP** - Malformed message handling

All tests **PASS** (7/7):
```
=== RUN   TestPhase3_BasicHTTPRequest
--- PASS: TestPhase3_BasicHTTPRequest (0.05s)
=== RUN   TestPhase3_HTTPHeaders
--- PASS: TestPhase3_HTTPHeaders (0.05s)
=== RUN   TestPhase3_ExpectAssertions
--- PASS: TestPhase3_ExpectAssertions (0.05s)
=== RUN   TestPhase3_ChunkedEncoding
--- PASS: TestPhase3_ChunkedEncoding (0.05s)
=== RUN   TestPhase3_GzipCompression
--- PASS: TestPhase3_GzipCompression (0.05s)
=== RUN   TestPhase3_GenerateBody
--- PASS: TestPhase3_GenerateBody (0.00s)
=== RUN   TestPhase3_MalformedHTTP
--- PASS: TestPhase3_MalformedHTTP (0.05s)
PASS
ok      github.com/perbu/gvtest/tests   0.322s
```

## Phase 3 Success Criteria

All Phase 3 success criteria from PORT.md have been met:

- ✅ **Can send and receive well-formed HTTP/1 messages**
  - Full txreq/txresp/rxreq/rxresp implementation
  - All HTTP components working correctly

- ✅ **Can intentionally send malformed HTTP/1 messages**
  - Raw byte-level control via send/sendhex
  - Manual message construction without validation
  - Support for missing headers, invalid formats, etc.

- ✅ **All HTTP/1 expect assertions work**
  - Complete expect implementation with all operators
  - Field extraction for requests and responses
  - Regex matching support
  - Numeric comparisons

- ✅ **Gzip compression/decompression works**
  - Automatic compression with `-gzip` flag
  - Automatic decompression on receive
  - Content-Encoding header handling
  - gunzip command for manual decompression

## Code Statistics

```
Files Created:
- pkg/http1/http.go (335 lines)
- pkg/http1/txreq.go (145 lines)
- pkg/http1/txresp.go (153 lines)
- pkg/http1/rxreq.go (119 lines)
- pkg/http1/rxresp.go (62 lines)
- pkg/http1/expect.go (162 lines)
- pkg/http1/commands.go (58 lines)
- pkg/http1/handler.go (350 lines)
- tests/phase3_test.go (532 lines)

Total: ~1,916 lines of new Go code for HTTP/1
```

## Build Status

All packages build successfully:
```bash
$ go build ./...
# Success!
```

All tests pass (including Phase 2 and Phase 3):
```bash
$ go test ./... -v
# All tests PASS
```

## Architecture Highlights

### 1. Raw Byte-Level Control
The HTTP/1 implementation does **NOT** use Go's `net/http` package, ensuring complete control over message construction:

```go
// Manual HTTP request construction
func (h *HTTP) TxReq(opts *TxReqOptions) error {
    var req strings.Builder
    fmt.Fprintf(&req, "%s %s %s\r\n", opts.Method, opts.URL, opts.Proto)
    // ... add headers manually
    fmt.Fprintf(&req, "Content-Length: %d\r\n", len(body))
    req.WriteString("\r\n")
    err := h.Write([]byte(req.String()))
    // ... send body
}
```

This allows for:
- Intentionally broken HTTP messages
- Missing or duplicate headers
- Invalid protocol versions
- Malformed chunk encoding
- Testing HTTP edge cases

### 2. Flexible Header Management
Headers are stored as raw strings in arrays, preserving:
- Original header order
- Header name casing
- Duplicate headers
- Non-standard headers

### 3. Chunked Encoding
Full chunked transfer encoding support:
- Sends data in hex-sized chunks with CRLF delimiters
- Parses incoming chunks with size validation
- Handles chunk extensions and trailers
- 0-sized final chunk termination

### 4. Gzip Integration
Seamless gzip support using Go's compress/gzip:
- Compression levels configurable
- Automatic Content-Encoding header management
- Transparent decompression on receive
- Error handling for corrupted compressed data

### 5. Expect System
Powerful assertion framework:
- Field paths (req.method, resp.http.content-type)
- Multiple comparison operators
- Regex pattern matching
- Numeric and string comparisons
- Detailed error messages

## Critical Features for Testing

### Malformed HTTP Generation
The implementation excels at generating malformed HTTP for testing:

```go
// Send malformed request manually
h.SendString("GET /path HHTTP/99.9\r\n")  // Invalid protocol
h.SendString("Content-Length: invalid\r\n")  // Invalid header value
h.SendString("\r\n")
```

### Edge Case Testing
- Missing Content-Length
- Chunked encoding with invalid sizes
- Headers with no values
- Multiple Host headers
- Non-ASCII characters in headers
- Incomplete messages

### Timeout Control
Configurable timeouts for all I/O operations:
- Per-operation timeout setting
- Connection read/write deadlines
- Graceful timeout handling

## Known Limitations

1. **HTTP/2 Support**: Not implemented (Phase 4 deliverable)
2. **PROXY Protocol**: Stub only (will be completed with Phase 2 integration)
3. **Connection pooling**: Not implemented (not required for testing)
4. **TLS/HTTPS**: Not implemented (can be added later if needed)

## Integration Points

The HTTP/1 package is designed to integrate with:
- **Client/Server packages** (Phase 2) via ProcessFunc callbacks
- **VTC command system** via Handler.ProcessSpec()
- **Test framework** for automated HTTP testing

## Performance

The implementation prioritizes:
1. **Correctness** over performance
2. **Raw control** over convenience
3. **Testing capabilities** over production use

Performance is acceptable for testing scenarios with minimal overhead from manual message construction.

## Next Steps: Phase 4

Phase 4 will implement the HTTP/2 protocol engine:
- HTTP/2 frame structure and I/O
- HPACK header compression
- Stream multiplexing
- Flow control
- Settings negotiation
- Ability to generate malformed HTTP/2 frames

The HTTP/1 implementation provides a solid foundation and pattern for HTTP/2.

## Compatibility

- Go version: 1.24.7
- Platform: Linux (with Darwin/Windows compatibility)
- Module path: `github.com/perbu/gvtest`
- Dependencies: Go standard library only (no external dependencies)

## Conclusion

Phase 3 has successfully implemented a complete HTTP/1.x protocol engine with:
- Full control over message construction
- Ability to generate malformed messages
- Comprehensive testing assertions
- Gzip compression support
- Extensive test coverage

This provides the core HTTP testing capabilities needed for GVTest, enabling precise control over HTTP/1 messages for testing HTTP clients, servers, and proxies.

**Date: November 15, 2025**

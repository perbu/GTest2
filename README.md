# GTest

HTTP testing framework for testing clients, servers, and proxies with support for intentionally malformed traffic.

## What It Does

GTest executes test scripts (`.vtc` files) that can:
- Start HTTP/1 and HTTP/2 servers
- Connect HTTP/1 and HTTP/2 clients
- Generate well-formed and intentionally broken HTTP traffic
- Test edge cases and protocol violations
- Synchronize multiple clients and servers with barriers
- Verify request/response content and headers

## Use Cases

- Testing HTTP server behavior with malformed requests
- Validating proxy handling of edge cases
- Testing HTTP/2 flow control and frame handling
- Protocol compliance testing
- Load balancer and reverse proxy validation
- Testing client error handling

## Building

```bash
make
```

Binary output: `cmd/gvtest/gvtest`

## Running Tests

```bash
./cmd/gvtest/gvtest tests/test.vtc
```

Run multiple tests:
```bash
./cmd/gvtest/gvtest tests/*.vtc
```

Options:
- `-v`: Verbose output
- `-q`: Quiet mode
- `-D name=value`: Define macro
- `-k`: Keep temporary directories
- `-t timeout`: Set test timeout

## Test File Format

Tests are written in VTC (Varnish Test Case) format:

```vtc
vtest "Simple HTTP test"

server s1 {
    rxreq
    expect req.method == "GET"
    expect req.url == "/test"
    txresp -status 200 -body "OK"
} -start

client c1 -connect ${s1_sock} {
    txreq -url "/test"
    rxresp
    expect resp.status == 200
    expect resp.body == "OK"
} -run
```

## Known Limitations

### Not Implemented

1. **Terminal emulation**: Process commands work but don't emulate VT100 terminals
   - No `-expect-text ROW COL` support
   - No `-screen_dump` support
   - Basic process I/O and text matching only

2. **Parallel test execution**: `-j` flag is parsed but ignored
   - All tests run sequentially
   - Workaround: Use GNU parallel or xargs

3. **Group checking**: `feature group` command not implemented
   - Tests requiring group membership will skip

4. **Limited platform detection**: Most features assume Linux
   - May fail on macOS, BSD, Windows
   - IPv4/IPv6 detection not implemented

5. **Process output macros**: `${pNAME_out}` and `${pNAME_err}` not exported

### Partial Implementations

- Platform-specific features (`SO_RCVTIMEO_WORKS`) use simplified checks
- Some rarely-used commands not yet ported

See `LIMITATIONS.md` for detailed technical information.

## Weak Spots

- **Performance**: No optimization work done yet; likely slower than C version
- **Platform support**: Primary target is Linux; other platforms undertested
- **Error messages**: May not always be clear about failures in complex tests
- **Memory usage**: No profiling or optimization; may use more memory than necessary
- **Test coverage**: Core functionality tested, edge cases may have gaps

## Dependencies

### Build Dependencies

Go 1.21 or later

### Runtime Dependencies

None (statically linked binary)

## Platform Support

- **Linux**: Primary target, best tested
- **macOS**: Should work but less tested
- **FreeBSD**: May work but untested
- **Windows**: Not supported

## Documentation

- `LIMITATIONS.md`: Detailed list of missing features
- `MIGRATION.md`: Migration guide from VTest2
- `PORT.md`: Implementation plan and phases

## Development Status

This is a port from VTest2 using AI assistance. Core HTTP/1 and HTTP/2 functionality is implemented. Advanced features and platform-specific code may be incomplete.

Based on VTest2 from the Varnish project.

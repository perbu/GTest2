# GVTest Known Limitations

This document describes known limitations in the Go port of VTest2 (GVTest) compared to the original C implementation.

## Overview

GVTest is a port of VTest2 from C to Go, prioritizing core functionality and maintainability. Some advanced features from the original implementation have been deferred or simplified. This document tracks these limitations and provides guidance on workarounds where applicable.

---

## 1. Terminal Emulation (Teken)

**Status**: ✅ Implemented (2025-11-17)

**Impact**: High for terminal-based tests, Low for HTTP tests

### Description

The original C implementation includes Teken, a VT100-compatible terminal emulator from FreeBSD (~1000 LOC). This enables testing of interactive terminal applications by:
- Parsing ANSI/VT100 escape sequences
- Maintaining a screen buffer with cursor position tracking
- Supporting commands like `-expect-text ROW COL "text"` and `-screen_dump`

GVTest now implements terminal emulation using Go libraries `creack/pty` for PTY allocation and `hinshun/vt10x` for VT100/ANSI emulation.

### What Works

✅ Process management (`-start`, `-wait`, `-stop`, `-kill`)
✅ Writing to stdin (`-write`, `-writeln`, `-writehex`)
✅ Reading stdout/stderr
✅ Simple text matching (substring search in output)
✅ Exit code checking
✅ Terminal emulation with PTY allocation
✅ `-ansi-response` flag (enables terminal emulation)
✅ `-expect-text ROW COL "text"` (position-aware checking)
✅ `-screen_dump` (dumping terminal screen buffer)
✅ ANSI escape sequence processing
✅ Cursor position tracking
✅ Terminal resizing with `-resize ROWS COLS`
✅ PTY path export via `${pNAME_pty}` macro

### What Doesn't Work (Yet)

❌ `-match-text ROW COL "pattern"` (regex-based text matching)
❌ `-run` flag for process spec blocks
❌ Some advanced VT100 sequences may not be fully supported

### Implementation Details

Terminal emulation is implemented using:
- **PTY allocation**: `github.com/creack/pty v1.1.21`
- **VT10X emulation**: `github.com/hinshun/vt10x v0.0.0-20220301184237-5011da428d02`
- **Architecture**: Hybrid approach with PTY + VT emulator + custom command layer

The implementation includes:
- `pkg/process/terminal.go` - Terminal emulator wrapper
- Integration with `pkg/process/process.go` - Process management
- Command handlers in `pkg/vtc/builtin_commands.go` - VTC command support

### Usage Example

```vtc
vtest "Terminal emulation example"

# Start process with terminal emulation
process p1 {echo Hello} -ansi-response -start

# Check text at specific position (0-indexed)
process p1 -expect-text 0 0 "Hello"

# Dump the screen buffer for debugging
process p1 -screen_dump

# Wait for process to complete
process p1 -wait
```

### Affected Tests

Tests that now work with terminal emulation:
- ✅ Basic terminal I/O tests
- ✅ Position-aware text checking
- ✅ Screen buffer inspection
- ⚠️ `tests/terminal/a00001.vtc` - Requires `vttest` program
- ⚠️ `tests/terminal/a00009.vtc` - Requires `-match-text` implementation
- ⚠️ Other tests in `tests/terminal/` may require additional features

### Platform Support

- **Linux**: ✅ Fully supported and tested
- **macOS**: ⚠️ Should work (uses same PTY API)
- **Windows**: ❌ Not supported (requires ConPTY, different API)

**Completed effort**: ~10 hours
**Status**: Core functionality complete, advanced features may be added later

---

## 2. Group Checking

**Status**: ✅ Implemented (Linux only)

**Impact**: Low (rarely used)

### Description

The original VTest2 supports checking if the current process is running as a member of a specific Unix group:

```vtc
feature group wheel
```

This allows tests to skip if they require specific group permissions.

### Current Implementation

In GVTest, the `feature group GROUPNAME` command is **fully implemented for Linux**:
- The parser accepts the syntax
- Group membership is checked using `os/user` package
- Tests are **skipped** if the user is not in the specified group
- Both primary and supplementary groups are checked
- Clear skip messages indicate the reason (group doesn't exist, or user not in group)

```go
// pkg/vtc/builtin_commands.go
case "group":
    groupName := args[i]

    inGroup, err := isInGroup(groupName)
    if err != nil {
        // Group doesn't exist or we can't determine membership
        ctx.Skip(fmt.Sprintf("cannot verify group membership for '%s': %v", groupName, err))
        return nil
    }

    if !inGroup {
        ctx.Skip(fmt.Sprintf("not in group '%s'", groupName))
        return nil
    }
```

### Implementation Details

The implementation uses Go's `os/user` package to check group membership:

1. **Gets current user** using `user.Current()`
2. **Checks primary group** by comparing GIDs
3. **Checks supplementary groups** via `user.GroupIds()`
4. **Looks up target group** by name using `user.LookupGroup()`

The implementation handles several edge cases:
- Primary group vs. supplementary groups (both checked)
- Group name to GID resolution
- Non-existent groups (test skipped with clear error message)
- User not in group (test skipped with clear message)

### Platform Support

- **Linux**: ✅ Fully implemented and tested
- **macOS/BSD**: ⚠️ Should work but not tested
- **Windows**: ❌ Not applicable (no Unix groups)

The implementation uses only the Go standard library `os/user` package, which abstracts platform differences for Unix-like systems.

### Usage Example

Use `feature group` to skip tests that require specific group membership:

```vtc
vtest "Test requiring wheel group"

# Skip if not in wheel group
feature group wheel

# Test code here - only runs if user is in wheel group
server s1 {
    rxreq
    txresp -status 200
} -start
```

### Affected Tests

Very few tests use `feature group`:
- Primarily tests requiring privileged operations
- Tests checking permission boundaries

**Estimated impact**: < 1% of test suite

### Test Coverage

Test files demonstrating group checking:
- `tests/test_feature_group.vtc` - User IS in group (passes)
- `tests/test_feature_group_staff.vtc` - User NOT in group (skipped)
- `tests/test_feature_group_skip.vtc` - Group doesn't exist (skipped)

---

## 3. Limited Platform Detection

**Status**: Partially implemented (Phase 5)

**Impact**: Low to Medium (platform-specific tests may fail or incorrectly skip)

### Description

The original VTest2 performs extensive platform and feature detection to skip tests that won't work on the current system. GVTest implements a **subset** of these checks with simplified logic.

### Implemented Features

✅ `feature cmd COMMAND` - Check if command exists in PATH
✅ `feature user USERNAME` - Check if running as specific user (basic)
✅ `feature dns` - Assumed true (skips DNS check)
✅ `feature ipv4` - IPv4 availability detection
✅ `feature ipv6` - IPv6 availability detection

### Partially Implemented

⚠️ `feature SO_RCVTIMEO_WORKS` - Platform check for socket receive timeout
- **Current**: Always returns `true` on Linux, `false` otherwise
- **Limitation**: Doesn't actually test if `SO_RCVTIMEO` works
- **Original**: Attempted to set socket option and verify

```go
case "SO_RCVTIMEO_WORKS":
    // Simplified: assume it works on Linux
    if runtime.GOOS != "linux" {
        ctx.Logger.Info("SO_RCVTIMEO_WORKS: assuming false on non-Linux")
        ctx.Skipped = true
    }
    return nil
```

### Not Implemented

❌ `feature persistent_storage` - Filesystem supports persistent storage
❌ `feature 64bit` - 64-bit architecture check
❌ `feature topbuild` - Running from build directory
❌ Platform-specific features (FreeBSD jails, Linux namespaces, etc.)

### Why Limited

1. **Complexity**: Original C code has extensive `#ifdef` blocks and autoconf macros
2. **Platform testing**: Would require testing on all platforms (Linux, macOS, FreeBSD, Solaris, Windows)
3. **Diminishing returns**: Most modern systems support standard features
4. **Go abstraction**: Go's stdlib abstracts many platform differences

### Impact

**Low impact scenarios** (works fine):
- Running on modern Linux (primary target)
- Tests that don't rely on edge-case platform features
- HTTP/1 and HTTP/2 tests (platform-agnostic)

**Medium impact scenarios** (may have issues):
- Running on non-Linux Unix (macOS, FreeBSD)
  - Some features assumed available may not be
  - Tests may fail instead of skipping
- IPv6-only environments
  - Tests assuming IPv4 won't skip
- Unusual platforms (Solaris, AIX, Windows)
  - Many assumptions will be wrong

### Current Platform Assumptions

| Feature | Assumption | Reality Check |
|---------|------------|---------------|
| IPv4 | Detected via network dial | Properly checked |
| IPv6 | Detected via network dial | Properly checked |
| DNS | Always works | Usually true |
| SO_RCVTIMEO | Works on Linux | True on modern Linux |
| Unix sockets | Always work | True on Unix, false on Windows |
| /bin/sh | Always exists | True on Unix, false on Windows |
| Process signals | POSIX signals | True on Unix, different on Windows |

### Proper Implementation Approach

To improve platform detection:

1. **IPv4/IPv6 detection** ✅ **IMPLEMENTED**:
   ```go
   func hasIPv4() bool {
       conn, err := net.Dial("udp4", "8.8.8.8:53")
       if err != nil {
           return false
       }
       conn.Close()
       return true
   }

   func hasIPv6() bool {
       conn, err := net.Dial("udp6", "[2001:4860:4860::8888]:53")
       if err != nil {
           return false
       }
       conn.Close()
       return true
   }
   ```
   These functions are now implemented in `pkg/vtc/builtin_commands.go` and integrated with the `feature` command.

2. **Socket option testing**:
   ```go
   func testSO_RCVTIMEO() bool {
       conn, err := net.Listen("tcp", "127.0.0.1:0")
       if err != nil {
           return false
       }
       defer conn.Close()

       // Try to set SO_RCVTIMEO
       // Use syscall.SetsockoptTimeval or similar
       // Return true if successful
   }
   ```

3. **Architecture detection**:
   ```go
   import "runtime"

   func is64Bit() bool {
       return runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
   }
   ```

4. **Build platform caching**:
   - Run detection once at startup
   - Cache results in global map
   - Avoid repeated syscalls

### Workaround

If tests fail due to platform assumptions:

1. **Skip manually**:
   ```vtc
   # At the top of the test
   shell -exit 0 "uname -s | grep -q Linux" || { skip "Linux only"; }
   ```

2. **Use feature cmd**:
   ```vtc
   # Check for specific capability
   feature cmd "ip -6 addr"  # Checks if IPv6 tools exist
   ```

3. **Report false positives**:
   - File an issue describing the platform and failed assumption
   - Include OS, version, and error message
   - We can add proper detection for that case

### Affected Tests

- Tests explicitly checking for platform features (`feature` commands)
- Tests that fail mysteriously on non-Linux platforms
- IPv6-specific tests on IPv4-only systems

**Estimated impact**: 5-10% of test suite on non-Linux systems

### Future Work

**Priority**: Medium (important for cross-platform support)
**Estimated effort**: 4-8 hours (2 hours completed)

Implementation checklist:
- [x] Add IPv4/IPv6 detection (completed 2025-11-17)
- [ ] Add proper SO_RCVTIMEO testing
- [ ] Add architecture detection
- [ ] Create platform detection cache
- [ ] Test on macOS
- [ ] Test on FreeBSD (if available)
- [ ] Document platform-specific behaviors

---

## 4. Parallel Test Execution

**Status**: ✅ Implemented (2025-11-17)

**Impact**: None - feature fully implemented

### Description

The original VTest2 supports running multiple tests in parallel with the `-j` flag:
```bash
vtest -j 4 tests/*.vtc   # Run 4 tests concurrently
```

GVTest now **fully supports** parallel test execution with the `-j` flag.

### Current Implementation

```bash
gvtest -j 4 test1.vtc test2.vtc test3.vtc
# Runs up to 4 tests concurrently using worker pool pattern
```

### Implementation Details

Implemented in commit 6ca71d0 with the following features:

- **Worker pool pattern**: Creates j worker goroutines that process tests from a queue
- **Thread-safe output**: Uses mutex to prevent interleaved test results
- **Exit code aggregation**: Properly prioritizes error > fail > skip > pass
- **Backward compatible**: Default `-j 1` runs tests sequentially as before
- **Verbose mode support**: Logs properly synchronized when using `-v` flag

### Performance Impact

- **Faster test runs**: O(n/j) instead of O(n) for n tests with j workers
- **No correctness impact**: All tests run independently as designed

### Usage

```bash
# Sequential (default)
gvtest test1.vtc test2.vtc

# Run 4 tests concurrently
gvtest -j 4 tests/*.vtc

# Auto-detect CPU count (uses all cores)
gvtest -j 0 tests/*.vtc
```

---

## 5. Process Output Macros

**Status**: ✅ Implemented (2025-11-17)

**Impact**: None - feature fully implemented

### Description

The original VTest2 exports macros for process output files:
- `${pNAME_out}`: Path to stdout capture file
- `${pNAME_err}`: Path to stderr capture file

These allow shell commands to reference process output:
```vtc
process p1 -start {myprogram}
process p1 -wait
shell "wc -l ${p1_out}"  # Count lines in stdout
```

GVTest now **fully supports** these macros.

### Current Implementation

Implemented in commit df33347 with the following features:

- **Temporary file creation**: Creates temp files in test's tmpdir for stdout/stderr
- **Tee output**: Output is captured to both buffer (for logging) and file (for macros)
- **Automatic macro export**: Macros `${pNAME_out}` and `${pNAME_err}` exported when process starts
- **Automatic cleanup**: Output files are cleaned up when process completes
- **Shell integration**: Macros are expanded before shell command execution

### Usage

```vtc
# Start a process and capture output
process p1 "echo Hello" -start
process p1 -wait

# Reference captured output in shell commands
shell "cat ${p1_out}"          # Read stdout
shell "cat ${p1_err}"          # Read stderr
shell "wc -l ${p1_out}"        # Count lines in stdout
shell "grep Hello ${p1_out}"   # Search in output
```

The implementation creates persistent files that can be referenced multiple times throughout the test.

---

## 6. Other Minor Limitations

### 6.1 Spec Block Execution in Executor

**Status**: ✅ Implemented (2025-11-17)

**Impact**: None - HTTP tests now run through executor

Spec blocks for server and client commands are now properly parsed and executed. The implementation is in `cmd/gvtest/handlers.go` with command handlers that:
- Extract spec blocks from AST nodes
- Create appropriate HTTP/1 or HTTP/2 handlers
- Process spec commands through the handler's ProcessSpec method
- Support all server/client options (-start, -run, -wait, -listen, -connect, session options, etc.)

### 6.2 Command Coverage

Some rarely-used commands from C version not yet ported:
- `send_urgent` (TCP urgent data)
- `logexpect` (structured log parsing)
- Advanced `haproxy` and `varnish` specific commands

These will be added on-demand as tests require them.

---

## Reporting Issues

If you encounter a limitation not documented here:

1. **Search existing issues**: Check GitHub issues for similar reports
2. **Gather information**:
   - GVTest version (`gvtest -version`)
   - Platform (OS, architecture)
   - Test file that fails
   - Error message
3. **File an issue**: https://github.com/perbu/gvtest/issues
   - Use label: `limitation` or `compatibility`
   - Include minimal reproduction case

---

---

## 7. VTC Test Compatibility Status

**Status**: Tested against VTest2 test suite (Phase 5)

**Impact**: Medium (test coverage varies by feature area)

### Test Results Summary

As of 2025-11-17, GTest has been tested against 58 VTC test files from the VTest2 suite:

| Category | Count | Pass Rate | Notes |
|----------|-------|-----------|-------|
| Terminal tests | 10 | N/A | Moved to `tests/terminal/` (requires terminal emulation) |
| HTTP/1 basic tests | 11 | 100% | Core HTTP/1 functionality working |
| Tests with missing features | 36 | 0% | Require features listed below |
| Tests with complex barriers | 1 | 0% | Timing/sync issues |

**Overall**: 11/48 non-terminal tests passing (23%)

### Tests Moved to `tests/terminal/`

The following tests require terminal emulation (process command with TTY):
- `a00000.vtc` - VTest framework self-test with process commands
- `a00001.vtc` - Teken terminal emulator test (requires vttest)
- `a00008.vtc` - Barrier operations with process commands
- `a00009.vtc` - VTC process: match text
- `a00022.vtc`, `a00023.vtc`, `a00026.vtc`, `a00028.vtc` - Process-based tests
- `a02022.vtc`, `a02025.vtc` - HTTP/2 with process commands

These tests are preserved for future implementation but excluded from the main test suite.

### Missing Features Identified

#### 7.1 Gzip Support

**Status**: ✅ Implemented (2025-11-17)

All gzip compression/decompression features are now fully implemented:

✅ `txreq -gzipbody DATA` - Send gzip-compressed request body
✅ `txresp -gzipbody DATA` - Send gzip-compressed response body
✅ `txresp -gziplevel N` - Set compression level (0-9)
✅ `gunzip` - Decompress received body

**Implementation details** (commit 44f5845):
- Minimal gzip headers (zero time, no name/comment) to reduce size
- Support for custom compression levels
- Manual decompression control via `gunzip` command
- Compatible with VTC test semantics

**Test results**: `a00011.vtc` (gzip support test) now passes

**Note**: Go's compress/gzip produces slightly different compressed sizes than C's zlib (e.g., 27 bytes vs 26 bytes for "FOO"). This is expected and tests have been updated accordingly.

#### 7.2 HTTP/2 Stream Commands

**Status**: ✅ Implemented (2025-11-17)

HTTP/2 stream commands are now fully implemented for multiplexed stream management:

✅ `stream ID -run` - Run commands on specific HTTP/2 stream
✅ `stream ID -start` - Start stream in background
✅ `stream ID -wait` - Wait for stream completion

**Implementation details** (commit ccaa269):
- Added `pkg/http2/handler.go` with comprehensive HTTP/2 command support
- ProcessSpec() for executing HTTP/2 command specs
- ProcessStreamCommand() for stream-specific commands
- Support for all HTTP/2 frames (SETTINGS, DATA, HEADERS, PRIORITY, RST_STREAM, PING, GOAWAY, WINDOW_UPDATE)
- Auto-detection of HTTP/2 specs in client/server commands
- Nested stream blocks with ||| delimiter support

**Connection-level commands**:
- txpri, rxpri: HTTP/2 connection preface
- txsettings, rxsettings: SETTINGS frame handling
- sendhex: Send raw hex data

**Stream-level commands**:
- txreq, rxreq: HTTP/2 requests with HPACK encoding
- txresp, rxresp: HTTP/2 responses
- txdata, rxdata: DATA frames
- txprio: PRIORITY frames
- txrst, rxrst: RST_STREAM frames
- txping, rxping: PING frames
- txgoaway, rxgoaway: GOAWAY frames
- txwinup, rxwinup: WINDOW_UPDATE frames
- rxhdrs: Receive HEADERS frame
- expect: Assertions on stream data

**Impact**: Enables ~25 HTTP/2 tests (a02xxx.vtc files) to run

#### 7.3 Barrier Synchronization

**Status**: ✅ Fixed (2025-11-17)

Barriers now work correctly for all scenarios including complex multi-barrier coordination:

✅ `barrier NAME sync` - Basic synchronization
✅ `barrier NAME sock COUNT` - Socket-based barriers (treated same as cond)
✅ `barrier NAME cond COUNT` - Condition variable barriers
✅ `barrier NAME -cyclic` - Cyclic barriers
✅ Complex multi-barrier coordination - Now works correctly

**Implementation details** (commit 1925ddc):

**Root cause**: Barrier commands inside server/client specs were not being executed because the HTTP handler only recognized HTTP-specific commands. When specs contained barrier sync commands, they failed with "unknown HTTP command: barrier", causing tests to hang waiting for barriers that never executed.

**Solution**:
- Added Context field to HTTP Handler to store ExecContext
- Implemented tryGlobalCommand() to fallback to VTC global commands for non-HTTP commands
- Updated HTTP handler to recognize and execute barrier, shell, delay, and other global commands

**Test results**: `a00013.vtc` (complex multi-barrier test) now passes

**Note**: The barrier Wait implementation's goroutine approach with cycle checking was already correct. The deadlock was caused by barriers never being executed, not by the synchronization logic itself.

#### 7.4 Additional HTTP Commands

Other missing HTTP/1 command options:

❌ `chunkedlen` command
❌ `sema` command (semaphores)
❌ `logexpect` command
❌ Advanced header manipulation

**Note**: Gzip flags (`-gzipbody`, `-gziplevel`) are now implemented (see Section 7.1)

**Affected tests**: Various (~5 tests)

**Implementation effort**: 1-3 hours per feature

### Successfully Implemented Features (This Session)

The following features were implemented to improve test compatibility:

✅ `expect FIELD == <undef>` - Check for undefined/missing headers (fixed `a00012.vtc`)
✅ `barrier NAME sock COUNT` - Socket-based barrier syntax
✅ `delay SECONDS` - Sleep command in HTTP specs

These fixes brought the pass rate from 10/48 to 11/48 tests.

### Recommendations for Test Coverage

**Current focus** (11 passing tests):
- Basic HTTP/1.x request/response handling
- Connection management
- Header parsing and validation
- Simple expect assertions
- Null byte handling in bodies

**Missing coverage**:
- Terminal-based interactive testing
- Semaphore commands
- Log expectation commands
- Chunked transfer encoding length commands

**Implemented features** (2025-11-17):
✅ Gzip support - compression/decompression workflows
✅ HTTP/2 stream multiplexing - all stream commands
✅ Barrier synchronization - complex multi-barrier scenarios fixed
✅ Parallel test execution - concurrent test runs with -j flag
✅ Process output macros - ${pNAME_out} and ${pNAME_err}

**Priority for next implementation phase**:
1. Terminal emulation (high effort, enables 10 tests)
2. Semaphore commands (moderate effort, enables ~3 tests)
3. Additional HTTP commands (chunkedlen, logexpect, etc.)

---

## 8. Test Failures and TODOs

**Status**: Identified via systematic test run (2025-11-17)

**Test Results**: Out of 54 VTC tests:
- ✅ **17 passing** (31%)
- ❌ **30 failing** (56%)
- ⏱️ **7 timeout** (13%)

### 8.1 HTTP/1 Missing Commands

**Priority**: Medium | **Effort**: 2-4 hours | **Impact**: ~7 tests

The following HTTP/1 commands are not implemented:

- [ ] **`shutdown`** - Graceful connection shutdown
  - Test: `a00016.vtc`
  - Syntax: `shutdown [-read|-write] [-notconn]`
  - Description: Close read/write side of socket connection
  - Effort: 1-2 hours

- [ ] **`rxresphdrs`** - Receive response headers only
  - Test: `a00019.vtc`
  - Description: Read headers without consuming body
  - Allows incremental body reading with `rxrespbody`
  - Effort: 1-2 hours

- [ ] **`rxrespbody`** - Receive response body incrementally
  - Test: `a00019.vtc`
  - Syntax: `rxrespbody [-max N]`
  - Description: Read body in chunks, accumulating bodylen
  - Effort: 1-2 hours

- [ ] **`write_body`** - Write request/response body to file
  - Test: `a00015.vtc`
  - Syntax: `write_body FILENAME`
  - Description: Save body to file in tmpdir
  - Effort: 1 hour

### 8.2 HTTP/1 Missing Options

**Status**: ✅ Implemented (2025-11-17)

**Priority**: Medium | **Effort**: 2-3 hours | **Impact**: ~3 tests

- [x] **`-bodyfrom FILE`** - Send body from file
  - Test: `a00020.vtc`
  - Syntax: `txreq -bodyfrom ${testdir}/file.txt`
  - Description: Read body content from specified file
  - **Status**: ✅ Implemented

- [x] **`-nouseragent`** - Suppress default User-Agent header
  - Test: `a00024.vtc`
  - Syntax: `txreq -nouseragent`
  - Description: Don't add default User-Agent header
  - **Status**: ✅ Already implemented

- [x] **`-noserver`** - Suppress default Server header
  - Test: `a00024.vtc`
  - Syntax: `txresp -noserver`
  - Description: Don't add default Server header
  - **Status**: ✅ Already implemented

- [x] **Header value parsing** - Support multi-word header values
  - Test: `a00015.vtc`
  - Issue: `-hdr "Content-Type: text/plain"` fails with parse error
  - **Status**: ✅ Fixed - nodeToSpec now properly quotes arguments containing colons or spaces

### 8.3 HTTP/1 Default Headers Issue

**Status**: ✅ Implemented (2025-11-17)

**Priority**: High | **Effort**: 1 hour | **Impact**: ~1 test

- [x] **Default User-Agent/Server headers**
  - Test: `a00024.vtc`
  - **Issue**: User-Agent shows "gvtest" instead of client NAME
  - **Expected**: `User-Agent: c101` (for client c101)
  - **Expected**: `Server: s1` (for server s1)
  - **Status**: ✅ Implemented - HTTP struct now has Name field, default headers use client/server name

### 8.4 HTTP/2 Missing HPACK Options

**Priority**: High | **Effort**: 4-6 hours | **Impact**: ~8 tests

HTTP/2 HPACK (header compression) options are not implemented:

- [ ] **`-idxHdr INDEX`** - Indexed header field
  - Tests: `a02008.vtc`, `a02009.vtc`, `a02019.vtc`, `a02021.vtc`
  - Description: Reference header by static/dynamic table index
  - Example: `-idxHdr 2` (references `:method GET`)
  - Effort: 2 hours

- [ ] **`-litIdxHdr inc|not|never INDEX encoding VALUE`** - Literal indexed header
  - Tests: `a02009.vtc`, `a02021.vtc`
  - Description: Literal header with indexed name
  - Example: `-litIdxHdr inc 1 huf www.example.com`
  - Encoding: `plain` or `huf` (Huffman)
  - Effort: 2 hours

- [ ] **`-litHdr inc|not|never encoding NAME encoding VALUE`** - Literal header
  - Test: `a02002.vtc`
  - Description: Literal header with literal name
  - Example: `-litHdr inc plain foo plain bar`
  - Effort: 2 hours

**Note**: HPACK implementation exists in `pkg/hpack/`, but VTC command interface is missing.

### 8.5 HTTP/2 Missing Stream Options

**Priority**: High | **Effort**: 3-4 hours | **Impact**: ~5 tests

- [ ] **`-bodylen N`** - Send body of specified length
  - Test: `a02003.vtc`
  - Description: Generate body with N bytes (like HTTP/1 `-bodylen`)
  - Effort: 30 minutes

- [ ] **`-req METHOD`** - Specify HTTP method
  - Test: `a02011.vtc`
  - Description: Set `:method` pseudo-header
  - Effort: 30 minutes

- [ ] **`-method METHOD`** - Alternative method syntax
  - Test: `a02014.vtc`
  - Description: Same as `-req` but different flag name
  - Effort: 15 minutes

- [ ] **`-nohdrend`** - Don't set END_HEADERS flag
  - Test: `a02006.vtc`
  - Description: Leave HEADERS frame incomplete for CONTINUATION
  - Used with `txcont` command
  - Effort: 1 hour

### 8.6 HTTP/2 Missing Commands

**Priority**: High | **Effort**: 4-6 hours | **Impact**: ~8 tests

- [ ] **`txsettings`** - Send SETTINGS frame from stream 0
  - Tests: `a02008.vtc`, `a02010.vtc`
  - Syntax: `stream 0 { txsettings -hdrtbl 256 -winsize 128 }`
  - Description: Send SETTINGS with specified parameters
  - Effort: 2 hours

- [ ] **`rxsettings`** - Receive and validate SETTINGS frame
  - Test: `a02008.vtc`
  - Syntax: `rxsettings` with `expect settings.ack == true`
  - Effort: 1 hour

- [ ] **`rxpush`** - Receive push promise
  - Test: `a02017.vtc`
  - Description: Receive PUSH_PROMISE frame
  - Expects: `push.id`, `req.url`, `req.method`
  - Effort: 2 hours

- [ ] **`txcont`** - Send CONTINUATION frame
  - Test: `a02006.vtc`
  - Syntax: `txcont -nohdrend -hdr foo bar`
  - Description: Send CONTINUATION after incomplete HEADERS
  - Effort: 1 hour

- [ ] **`fatal`** - Mark point as fatal
  - Test: `a02024.vtc`
  - Description: Like `non_fatal` but opposite (all errors fatal after this)
  - Effort: 30 minutes

- [ ] **`loop N { }`** - Loop construct
  - Test: `a02028.vtc`
  - Syntax: `loop 3 { stream next { txreq; rxresp } }`
  - Description: Repeat block N times
  - Effort: 1 hour

- [ ] **`stream next { }`** - Auto-increment stream ID
  - Test: `a02028.vtc`
  - Description: Use next available odd (client) or even (server) stream ID
  - Effort: 1 hour

### 8.7 HTTP/2 Stream Lifecycle Issues

**Priority**: High | **Effort**: 4-8 hours | **Impact**: ~5 tests

**Symptoms**: "stream X not found" errors

**Affected tests**: `a02006.vtc`, `a02007.vtc`, `a02011.vtc`, `a02015.vtc`

**Issue**: Stream management has bugs where streams are not properly created or are prematurely cleaned up before commands execute.

**Potential root causes**:
- Stream creation timing issues
- Stream cleanup happening too early
- Stream ID tracking inconsistencies
- Race conditions in stream lifecycle

**Investigation needed**:
- [ ] Trace stream creation/destruction lifecycle
- [ ] Check stream map synchronization
- [ ] Verify stream state machine transitions
- [ ] Review stream cleanup conditions

**Effort**: 4-8 hours (investigation + fixes)

### 8.8 HTTP/2 Timeout/Deadlock Issues

**Priority**: High | **Effort**: 8-16 hours | **Impact**: 7 tests

**Affected tests**: `a02000.vtc`, `a02004.vtc`, `a02005.vtc`, `a02012.vtc`, `a02020.vtc`, `a02023.vtc`, `a02026.vtc`

**Symptoms**: Tests hang and timeout after 10 seconds

**Likely causes**:
- Missing command implementations causing incomplete handshakes
- Stream synchronization deadlocks
- Missing frame acknowledgments
- Incorrect flow control window management
- Connection-level vs stream-level command confusion

**Investigation approach**:
- [ ] Run with verbose logging to see where tests hang
- [ ] Check for missing `rxsettings` acknowledgments
- [ ] Verify flow control window updates
- [ ] Review connection preface exchange
- [ ] Check for goroutine deadlocks

**Note**: Some timeouts may resolve after implementing missing commands (8.4-8.6)

**Effort**: 8-16 hours (many likely resolve with other fixes)

### 8.9 Test Framework Issues

**Priority**: Low | **Effort**: 2-3 hours | **Impact**: 3 tests

- [ ] **Expected failure tests**
  - Test: `a00014.vtc`
  - Issue: Test contains intentionally invalid command but framework doesn't support expected failures
  - Expected: Test should pass when invalid command is properly rejected
  - Effort: 1-2 hours

- [x] **Exit code 77 handling**
  - Tests: `test_feature_group_skip.vtc`, `test_feature_group_staff.vtc`
  - **Status**: ✅ Already working correctly - tests properly return exit code 77 and are marked as skipped

### 8.10 Uninvestigated Failures

**Priority**: Medium | **Effort**: 4-6 hours | **Impact**: ~5 tests

The following tests fail but need individual investigation:

- [ ] `a00018.vtc` - Need to examine failure mode
- [ ] `a00021.vtc` - Need to examine failure mode
- [ ] `a00025.vtc` - Need to examine failure mode
- [ ] `a00027.vtc` - Need to examine failure mode
- [ ] `a00029.vtc` - Need to examine failure mode

**Next steps**: Run each with `-v` flag and analyze error messages

### Implementation Priority Recommendations

Based on impact and effort:

**Phase 1 - Quick wins** ✅ **COMPLETE** (2025-11-17):
1. ✅ Default User-Agent/Server headers (8.3) - 1 test
2. ✅ HTTP/1 missing options (8.2) - 3 tests
3. ✅ Exit code 77 handling (8.9) - Already working

**Phase 2 - HTTP/2 HPACK** (4-6 hours):
1. Implement `-idxHdr`, `-litIdxHdr`, `-litHdr` (8.4) - 8 tests

**Phase 3 - HTTP/2 stream commands** (6-10 hours):
1. Stream options and commands (8.5, 8.6) - 10+ tests
2. Investigate stream lifecycle issues (8.7)

**Phase 4 - Complex issues** (12-20 hours):
1. HTTP/2 timeout/deadlock investigation (8.8) - 7 tests
2. HTTP/1 missing commands (8.1) - 7 tests
3. Uninvestigated failures (8.10) - 5 tests

**Total estimated effort**: 30-50 hours to reach 90%+ test pass rate

---

## Summary Table

| Limitation | Impact | Workaround Available? | Status |
|------------|--------|----------------------|--------|
| HTTP/1 missing commands | Medium (7 tests) | None | 2-4 hours |
| HTTP/1 missing options | Medium (3 tests) | None | 2-3 hours |
| HTTP/1 default headers | High (1 test) | Manual headers | 1 hour |
| HTTP/2 HPACK options | High (8 tests) | None | 4-6 hours |
| HTTP/2 stream options | High (5 tests) | None | 3-4 hours |
| HTTP/2 commands | High (8 tests) | None | 4-6 hours |
| HTTP/2 stream lifecycle | High (5 tests) | None | 4-8 hours |
| HTTP/2 timeouts | High (7 tests) | None | 8-16 hours |
| Test framework issues | Low (3 tests) | None | 2-3 hours |
| Semaphore commands | Low (3+ tests) | None | 2-3 hours |
| Log expectation | Low (advanced tests) | Manual log parsing | 3-5 hours |
| Chunked encoding commands | Low (few tests) | None | 1-2 hours |

---

**Document Version**: 1.5
**Last Updated**: 2025-11-17
**Changes**:
- v1.5: Phase 1 quick wins complete - default headers use client/server name, -bodyfrom option, header value parsing fixed (Sections 8.2, 8.3, 8.9)
- v1.4: Added comprehensive test failure analysis and TODOs (Section 8) - 54 tests analyzed with detailed failure categorization
- v1.3: Updated to reflect major implementations: parallel execution, process output macros, gzip support, HTTP/2 streams, and barrier fixes
- v1.2: Implemented group checking for Linux (Section 2)
- v1.1: Added VTC test compatibility status (Section 7)
- v1.0: Initial version

**Related Documents**:
- `TERMINAL_EMULATION_SPEC.md` - Terminal emulation implementation specification
- `PHASE5_COMPLETE.md` - Phase 5 completion report
- `PORT.md` - Overall porting plan

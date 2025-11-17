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

**Status**: Not implemented (Phase 5)

**Impact**: Medium (test suite runs slower)

### Description

The original VTest2 supports running multiple tests in parallel with the `-j` flag:
```bash
vtest -j 4 tests/*.vtc   # Run 4 tests concurrently
```

GVTest **parses** the `-j` flag but **ignores** it - all tests run sequentially.

### Current Behavior

```bash
gvtest -j 4 test1.vtc test2.vtc test3.vtc
# Actually runs: test1 → test2 → test3 (sequential)
```

### Why Not Implemented

1. **Complexity**: Requires goroutine pool, result aggregation, synchronized output
2. **Phase scope**: Deferred to Phase 6 to keep Phase 5 focused
3. **Correctness first**: Ensure single-threaded execution works correctly first

### Impact

- **Slower test runs**: O(n) instead of O(n/j) for n tests
- **No impact on correctness**: All tests still run, just sequentially

### Workaround

Use GNU parallel or xargs:
```bash
find tests -name "*.vtc" | xargs -n 1 -P 4 gvtest
```

### Future Work

**Priority**: Medium
**Estimated effort**: 3-5 hours

See `PHASE5_COMPLETE.md` section 6 for implementation notes.

---

## 5. Process Output Macros

**Status**: Not implemented (Phase 5)

**Impact**: Low (advanced use cases)

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

GVTest does not currently export these macros.

### Workaround

Capture output in a file explicitly:
```vtc
process p1 -start {myprogram}
process p1 -wait
shell "myprogram > output.txt 2> error.txt"
shell "wc -l output.txt"
```

### Future Work

**Priority**: Low
**Estimated effort**: 1-2 hours

Implementation:
- Create temp files for stdout/stderr in tmpdir
- Tee output to both buffer and file
- Export macros when process starts

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

**Status**: Partially implemented

The `gunzip` command exists but compression/decompression options are missing:

❌ `txreq -gzipbody DATA` - Send gzip-compressed request body
❌ `txresp -gzipbody DATA` - Send gzip-compressed response body
❌ `txresp -gziplevel N` - Set compression level
✅ `gunzip` - Decompress received body (implemented but untested)

**Affected tests**: `a00011.vtc` (gzip support test), plus ~5 other tests

**Workaround**: None available for these specific tests

**Implementation effort**: 4-6 hours

#### 7.2 HTTP/2 Stream Commands

**Status**: Not implemented

HTTP/2 tests require the `stream` command for multiplexed stream management:

❌ `stream ID -run` - Run commands on specific HTTP/2 stream
❌ `stream ID -start` - Start stream in background
❌ `stream ID -wait` - Wait for stream completion

**Affected tests**: All `a02xxx.vtc` tests except basic ones

**Impact**: ~25 HTTP/2 tests cannot run

**Workaround**: Use HTTP/1 tests to validate basic HTTP functionality

**Implementation effort**: 8-12 hours (requires HTTP/2 stream state machine)

#### 7.3 Barrier Synchronization

**Status**: Basic implementation with issues

Barriers work for simple cases but complex multi-barrier scenarios (like `a00013.vtc`) experience deadlocks.

✅ `barrier NAME sync` - Basic synchronization
✅ `barrier NAME sock COUNT` - Socket-based barriers (treated same as cond)
✅ `barrier NAME cond COUNT` - Condition variable barriers
✅ `barrier NAME -cyclic` - Cyclic barriers
⚠️ Complex multi-barrier coordination - May deadlock

**Affected tests**: `a00013.vtc` (complex barrier test)

**Issue**: Likely race condition or incorrect barrier reset logic

**Implementation effort**: 2-4 hours (debugging and fixing sync logic)

#### 7.4 Additional HTTP Commands

Other missing HTTP/1 command options:

❌ `txreq/txresp -gzip` flags
❌ `chunkedlen` command
❌ `sema` command (semaphores)
❌ `logexpect` command
❌ Advanced header manipulation

**Affected tests**: Various (~10 tests)

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
- Compression/decompression workflows
- HTTP/2 stream multiplexing
- Complex synchronization scenarios
- Terminal-based interactive testing

**Priority for next implementation phase**:
1. Gzip support (high impact, moderate effort)
2. Fix barrier synchronization (low effort, fixes 1 test)
3. HTTP/2 streams (high effort, enables 25+ tests)
4. Terminal emulation (high effort, enables 10 tests)

---

## Summary Table

| Limitation | Impact | Workaround Available? | Status |
|------------|--------|----------------------|--------|
| ~~Terminal emulation~~ | ✅ **Implemented** | N/A | ~~8-14 hours~~ |
| Gzip support | Medium (6+ tests) | None | 4-6 hours |
| HTTP/2 streams | High (25+ tests) | Use HTTP/1 | 8-12 hours |
| Barrier sync bugs | Low (1 test) | Simplify test | 2-4 hours |
| Group checking | Low | N/A | ✅ Implemented (Linux) |
| Platform detection | Medium (non-Linux) | Yes (manual checks) | 4-8 hours |
| Parallel execution | Medium (performance) | Yes (GNU parallel) | 3-5 hours |
| Process output macros | Low | Yes (temp files) | 1-2 hours |
| ~~Spec block execution~~ | ✅ **Implemented** | N/A | ~~2-4 hours~~ |

**Total technical debt**: ~21-42 hours of development (reduced from ~31-56 hours)

---

**Document Version**: 1.3
**Last Updated**: 2025-11-17
**Changes**:
- v1.3: Implemented terminal emulation (Section 1)
- v1.2: Implemented group checking for Linux (Section 2)
- v1.1: Added VTC test compatibility status (Section 7)
- v1.0: Initial version

**Related Documents**:
- `TERMINAL_EMULATION_SPEC.md` - Terminal emulation implementation specification
- `PHASE5_COMPLETE.md` - Phase 5 completion report
- `PORT.md` - Overall porting plan

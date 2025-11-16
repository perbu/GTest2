# TODO - Weak Spots and Issues

## Test Failures (as of 2025-11-16)

### Fixed Issues
1. **pkg/barrier/barrier.go** - Mutex unlock panic in WaitTimeout
   - **Status**: FIXED
   - **Issue**: Goroutine in WaitTimeout was calling `b.cond.Wait()` without holding the mutex lock
   - **Fix**: Added `b.mutex.Lock()` before calling `b.cond.Wait()` in the goroutine
   - **Location**: pkg/barrier/barrier.go:82-83

2. **pkg/http1/benchmark_test.go** - Field name capitalization errors
   - **Status**: FIXED
   - **Issue**: Benchmark tests were using lowercase field names (headers, method, url, proto, status, reason) instead of the actual struct field names (Method, URL, Proto, Status, Reason). Also used non-existent `headers` map instead of `ReqHeaders`/`RespHeaders` slices.
   - **Fix**: Updated all field references to use correct capitalization and changed headers handling to use ReqHeaders/RespHeaders slices
   - **Location**: pkg/http1/benchmark_test.go

### Remaining Issues

3. **tests/phase4_test.go** - HTTP/2 Connection Setup Timeouts
   - **Status**: NEEDS INVESTIGATION
   - **Tests Failing**:
     - TestPhase4_ConnectionSetup (timeout at phase4_test.go:269)
     - TestPhase4_RequestResponse (timeout at phase4_test.go:332)
   - **Issue**: HTTP/2 connection Start() method appears to hang or not complete properly
   - **Symptoms**:
     - Connection start timeout after 2 seconds
     - TxReq (send request) timeout after 1 second
   - **Potential Causes**:
     - Deadlock in HTTP/2 connection setup sequence
     - Missing synchronization between client and server preface exchange
     - Frame receive loop not properly handling initial SETTINGS frames
     - Potential issue with net.Pipe() blocking on reads/writes
   - **Next Steps**:
     - Debug HTTP/2 Start() method flow
     - Check frameReceiveLoop() for potential blocking
     - Verify SETTINGS frame exchange completes
     - Add logging to trace connection setup sequence
   - **Location**: pkg/http2/conn.go:100-122 (Start method)

## Code Quality Issues

### Benchmark Test Design
- **File**: pkg/http1/benchmark_test.go
- **Issue**: The benchmark tests use simplified dummy functions (parseHeaders, buildRequest, buildResponse) that don't reflect the actual HTTP implementation
- **Impact**: Benchmarks may not accurately measure real-world performance
- **Recommendation**: Consider updating benchmarks to use actual HTTP parsing/building functions or clearly document that these are simplified benchmarks

### Barrier Implementation
- **File**: pkg/barrier/barrier.go
- **Concern**: The WaitTimeout implementation uses a goroutine to handle the wait condition, which adds complexity
- **Note**: The fix works correctly, but the overall design could potentially be simplified
- **Current Behavior**: Works correctly after fix, all barrier tests pass

## Testing Coverage

### Passing Test Suites
- pkg/barrier - All 9 tests passing
- pkg/http1 - Builds successfully (only benchmark tests, no unit tests)
- pkg/logging - All 6 tests passing
- pkg/net - All 3 test groups passing
- pkg/session - All 2 tests passing
- pkg/util - All 4 tests passing
- pkg/vtc - All 12 tests passing
- tests/phase2_test.go - All 6 tests passing
- tests/phase3_test.go - All 7 tests passing

### Failing Test Suites
- tests/phase4_test.go - 2 out of 9 tests failing (HTTP/2 connection tests)

## Recommendations

1. **Immediate**: Fix HTTP/2 connection setup issues to make all tests pass
2. **Short-term**: Add more unit tests for pkg/http1 beyond just benchmarks
3. **Medium-term**: Review and potentially simplify barrier synchronization logic
4. **Long-term**: Improve HTTP/2 implementation robustness and error handling

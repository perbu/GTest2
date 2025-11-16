# TODO - Outstanding Issues

## Test Failures

### HTTP/2 Connection Setup Timeouts (tests/phase4_test.go)
- **Tests Failing**:
  - TestPhase4_ConnectionSetup (timeout at phase4_test.go:269)
  - TestPhase4_RequestResponse (timeout at phase4_test.go:332)
- **Issue**: HTTP/2 connection Start() method hangs or does not complete properly
- **Symptoms**:
  - Connection start timeout after 2 seconds
  - TxReq (send request) timeout after 1 second
- **Potential Causes**:
  - Deadlock in HTTP/2 connection setup sequence
  - Missing synchronization between client and server preface exchange
  - Frame receive loop not properly handling initial SETTINGS frames
  - Potential issue with net.Pipe() blocking on reads/writes
- **Location**: pkg/http2/conn.go:100-122 (Start method)
- **Next Steps**:
  - Debug HTTP/2 Start() method flow
  - Check frameReceiveLoop() for potential blocking
  - Verify SETTINGS frame exchange completes
  - Add logging to trace connection setup sequence

## Code Quality Issues

### Benchmark Test Design (pkg/http1/benchmark_test.go)
- **Issue**: The benchmark tests use simplified dummy functions (parseHeaders, buildRequest, buildResponse) that don't reflect the actual HTTP implementation
- **Impact**: Benchmarks may not accurately measure real-world performance
- **Recommendation**: Update benchmarks to use actual HTTP parsing/building functions or clearly document that these are simplified benchmarks

### Barrier Implementation (pkg/barrier/barrier.go)
- **Concern**: The WaitTimeout implementation uses a goroutine to handle the wait condition, which adds complexity
- **Recommendation**: Consider simplifying the design while maintaining correctness

## Testing Coverage Gaps

### Missing Unit Tests
- **pkg/http1**: Only has benchmark tests, no unit tests for core functionality
- **Recommendation**: Add unit tests for HTTP parsing, building, and edge cases

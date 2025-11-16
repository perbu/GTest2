# TODO - Outstanding Issues

## Test Failures

### ✅ FIXED: HTTP/2 Connection Setup Deadlock
- **Issue**: Deadlock during HTTP/2 connection setup when using synchronous I/O (net.Pipe)
- **Root Cause**: Both client and server tried to send SETTINGS ACK synchronously in the receive loop, blocking each other when using synchronous pipes
- **Solution Implemented**:
  1. Added write mutex (`writeMu`) to prevent frame corruption from concurrent writes
  2. Made SETTINGS ACK and PING ACK asynchronous (sent in goroutines) to prevent receive loop blocking
  3. Removed the 50ms sleep hack that was attempting to work around the race condition
- **Tests Now Passing**:
  - TestPhase4_ConnectionSetup ✓
  - TestPhase4_FlowControl ✓ (was deadlocking)
  - TestPhase4_Settings ✓
- **Files Modified**:
  - pkg/http2/conn.go: Added writeMu, async ACKs, removed sleep
  - pkg/http2/commands.go: Protected writes with mutex
  - pkg/http2/frame_commands.go: Protected writes with mutex

## Code Quality Issues

### Benchmark Test Design (pkg/http1/benchmark_test.go)
- **Issue**: The benchmark tests use simplified dummy functions (parseHeaders, buildRequest, buildResponse) that don't reflect the actual HTTP implementation
- **Impact**: Benchmarks may not accurately measure real-world performance
- **Recommendation**: Update benchmarks to use actual HTTP parsing/building functions or clearly document that these are simplified benchmarks

### Barrier Implementation (pkg/barrier/barrier.go)
- **Concern**: The WaitTimeout implementation uses a goroutine to handle the wait condition, which adds complexity
- **Recommendation**: Consider simplifying the design while maintaining correctness

## Testing Coverage Gaps

All core packages now have unit test coverage.

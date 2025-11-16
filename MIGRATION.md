# Migration Guide: VTest2 (C) to GVTest (Go)

## Overview

This guide helps users migrate from the C implementation of VTest2 to the Go implementation (GVTest). While GVTest maintains compatibility with the VTC file format, there are some differences in behavior, features, and usage that you should be aware of.

## Quick Migration Checklist

- [ ] Install GVTest (Go 1.24.7+ required)
- [ ] Test existing .vtc files with GVTest
- [ ] Review error messages (different format)
- [ ] Update any build scripts to use `gvtest` binary
- [ ] Check for unsupported features (see below)
- [ ] Validate test results match expectations

## Installation

### C Version (VTest2)
```bash
make
./vtest test.vtc
```

### Go Version (GVTest)
```bash
# From source
make build
./gvtest test.vtc

# Or via go install
go install github.com/perbu/gvtest/cmd/gvtest@latest
gvtest test.vtc
```

## Command-Line Interface

The command-line interface is largely compatible:

| C Version | Go Version | Status | Notes |
|-----------|-----------|--------|-------|
| `vtest test.vtc` | `gvtest test.vtc` | ✅ Compatible | Binary name changed |
| `-v` (verbose) | `-v` | ✅ Compatible | |
| `-q` (quiet) | `-q` | ✅ Compatible | |
| `-k` (keep tmpdir) | `-k` | ✅ Compatible | |
| `-t TIMEOUT` | `-t TIMEOUT` | ✅ Compatible | |
| `-j JOBS` | `-j JOBS` | ⚠️ Parsed but not implemented | Sequential execution only |
| `-D name=value` | Not yet implemented | ❌ Not available | Macro definitions |
| `-n NUM` | Not implemented | ❌ Not available | Number of tests |

## Exit Codes

Exit codes are compatible:

| Code | Meaning | C Version | Go Version |
|------|---------|-----------|-----------|
| 0 | Test passed | ✅ | ✅ |
| 1 | Test failed | ✅ | ✅ |
| 77 | Test skipped | ✅ | ✅ |
| 2 | Error (parse/runtime) | ✅ | ✅ |

## VTC Language Compatibility

### Fully Compatible Commands

These VTC commands work identically in both versions:

#### Server/Client Commands
```vtc
server s1 { ... } -start
server s1 -wait
client c1 -connect ${s1_sock} { ... } -run
client c1 -wait
```

#### HTTP/1.1 Commands
```vtc
txreq -method GET -url "/" -proto HTTP/1.1
txresp -status 200 -body "OK"
rxreq
rxresp
expect req.method == "GET"
expect resp.status == 200
```

#### HTTP/2 Commands
```vtc
server s1 -h2 { ... }
stream 1 { txreq; rxresp }
txsettings
rxsettings
```

#### Utility Commands
```vtc
delay 1
shell "echo test"
barrier b1 -start 2
barrier b1 -wait
```

### Partially Compatible Features

#### 1. Process Management

**C Version** (with full terminal emulation):
```vtc
process p1 -start "interactive-program"
process p1 -expect-cursor 5 10
process p1 -screen_dump
```

**Go Version** (basic process I/O only):
```vtc
process p1 -start "program"
process p1 -write "input"
process p1 -expect-text "output"
# Note: No cursor position or screen buffer support
```

**Migration**: If your tests rely on terminal emulation features like cursor positioning or screen dumps, they will need to be rewritten to use simpler text matching.

#### 2. Feature Detection

**C Version**:
```vtc
feature cmd varnishd
feature group varnish
```

**Go Version**:
```vtc
feature cmd go      # ✅ Works
feature group name  # ⚠️ Basic support, may not work correctly
```

**Migration**: Group-based feature detection is not fully implemented. Use command-based detection when possible.

### Incompatible Features

The following features from the C version are **not implemented** in GVTest:

#### 1. Varnish-Specific Integration
```vtc
# NOT SUPPORTED in GVTest
varnish v1 -vcl { ... }
varnish v1 -cliok "command"
```

**Reason**: Varnish-specific features (`vtc_varnish.c`) are out of scope for the core HTTP testing tool.

**Migration**: If you need Varnish-specific testing, continue using the C version or contribute Varnish integration to GVTest.

#### 2. HAProxy-Specific Features
```vtc
# NOT SUPPORTED in GVTest
haproxy h1 -conf { ... }
```

**Reason**: HAProxy-specific features (`vtc_haproxy.c`) not yet ported.

**Migration**: HAProxy testing requires the C version or future GVTest enhancement.

#### 3. Syslog Testing
```vtc
# NOT SUPPORTED in GVTest
syslog S1 -start
syslog S1 -expect "message"
```

**Reason**: Syslog support (`vtc_syslog.c`) not yet implemented.

**Migration**: Use alternative log checking methods (shell commands, file operations).

#### 4. VSM (Varnish Shared Memory)
```vtc
# NOT SUPPORTED in GVTest
varnish v1 -expect client_conn == 10
```

**Reason**: VSM is Varnish-specific and out of scope.

## Error Messages

Error message formatting differs between versions:

### C Version
```
**** top   0.0 Test case: test.vtc
---- c1    0.001 rxresp failed: timeout
```

### Go Version
```
**** dT    0.000
---- c1    rxresp failed: timeout
**** c1    Fatal error in test
```

**Migration**: Update any log parsing scripts to handle the new format.

## Macro System

### Compatible Macros

These macros work in both versions:

| Macro | Description | Example |
|-------|-------------|---------|
| `${s1_addr}` | Server address | `127.0.0.1` |
| `${s1_port}` | Server port | `12345` |
| `${s1_sock}` | Server socket | `127.0.0.1:12345` |
| `${tmpdir}` | Temporary directory | `/tmp/gvtest-xxx` |

### Macro Differences

**C Version**: Uses `${name}` and `$name` syntax
**Go Version**: Primarily uses `${name}` syntax (dollar-only syntax may not work everywhere)

**Migration**: Always use `${name}` syntax for maximum compatibility.

## Performance Differences

### Expected Performance

GVTest targets performance within 2x of the C version. In practice:

- **Simple tests**: Comparable performance
- **Complex HTTP/2 tests**: May be slower due to Go's GC
- **Large file operations**: Should be comparable

### Profiling

**C Version**:
```bash
valgrind --tool=callgrind ./vtest test.vtc
```

**Go Version**:
```bash
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof
```

## Temporary Directory Handling

### C Version
- Uses `mkdtemp` with pattern `vtest.XXXXXX`
- Located in `/tmp/` by default

### Go Version
- Uses `os.MkdirTemp` with pattern `gvtest-*`
- Located in system temp dir (respects `$TMPDIR`)

**Migration**: Update any scripts that reference temporary directory names.

## Logging and Output

### C Version
```
**** top   0.0 Test case: test.vtc
**   c1    0.1 Starting...
*    c1    0.2 Connected
```

### Go Version
```
**** dT    0.000
***  c1    Starting...
**   c1    Connected
```

**Migration**: The logging format is similar but not identical. Timestamps use milliseconds in both versions.

## Building and Integration

### Build System Integration

**C Version** (Makefile):
```makefile
test:
    ./vtest tests/*.vtc
```

**Go Version** (Makefile):
```makefile
test:
    ./gvtest tests/*.vtc
```

### CI/CD Integration

**C Version**:
```yaml
# .github/workflows/test.yml
- name: Build VTest
  run: make
- name: Run Tests
  run: ./vtest tests/*.vtc
```

**Go Version**:
```yaml
# .github/workflows/test.yml
- name: Setup Go
  uses: actions/setup-go@v4
  with:
    go-version: '1.24'
- name: Build GVTest
  run: make build
- name: Run Tests
  run: ./gvtest tests/*.vtc
```

## Testing Your Migration

### Step 1: Test Basic Compatibility

```bash
# Run a simple test with both versions
./vtest tests/simple.vtc
./gvtest tests/simple.vtc

# Compare exit codes
echo $?  # Should be the same
```

### Step 2: Test Your Test Suite

```bash
# Run all tests with C version
for f in tests/*.vtc; do
    echo "Testing $f with C version..."
    ./vtest "$f" || echo "FAILED: $f"
done

# Run all tests with Go version
for f in tests/*.vtc; do
    echo "Testing $f with Go version..."
    ./gvtest "$f" || echo "FAILED: $f"
done
```

### Step 3: Compare Results

Create a comparison script:

```bash
#!/bin/bash
for test in tests/*.vtc; do
    echo "Testing $test..."

    # C version
    ./vtest "$test" > /tmp/c-result 2>&1
    c_exit=$?

    # Go version
    ./gvtest "$test" > /tmp/go-result 2>&1
    go_exit=$?

    if [ $c_exit -eq $go_exit ]; then
        echo "  ✓ Exit codes match ($c_exit)"
    else
        echo "  ✗ Exit codes differ: C=$c_exit Go=$go_exit"
    fi
done
```

## Common Migration Issues

### Issue 1: Process Tests Hanging

**Symptom**: Tests with `process` commands hang indefinitely

**Cause**: Missing terminal emulation in Go version

**Solution**: Rewrite tests to use simple text matching:

```vtc
# Instead of:
process p1 -expect-cursor 5 10

# Use:
process p1 -expect-text "expected output"
```

### Issue 2: Tests Fail with "command not found"

**Symptom**: Tests that worked in C version fail with command not found

**Cause**: Different PATH handling or feature detection

**Solution**: Add feature checks:

```vtc
feature cmd required-command
# Test code here
```

### Issue 3: Macro Expansion Differs

**Symptom**: Macros not expanding as expected

**Cause**: Subtle differences in macro expansion timing

**Solution**: Use explicit `${macro}` syntax and verify expansion with verbose mode:

```bash
./gvtest -v test.vtc 2>&1 | grep "macro"
```

### Issue 4: Performance Regression

**Symptom**: Tests run significantly slower in Go version

**Cause**: Go's garbage collection or different concurrency model

**Solution**:
1. Profile the test to identify bottlenecks
2. Increase timeout if needed: `gvtest -t 120 test.vtc`
3. Report performance issues to GVTest project

## Gradual Migration Strategy

For large test suites, consider a gradual migration:

### Phase 1: Identify Compatible Tests (Week 1)
```bash
# Create two lists
./check-compatibility.sh > compatible-tests.txt
./check-compatibility.sh > incompatible-tests.txt
```

### Phase 2: Run Compatible Tests (Week 2)
```bash
# Run compatible tests with both versions
while read test; do
    ./vtest "$test" && ./gvtest "$test"
done < compatible-tests.txt
```

### Phase 3: Migrate Incompatible Tests (Weeks 3-4)
- Rewrite process-based tests
- Replace Varnish-specific tests
- Update macro usage

### Phase 4: Full Cutover (Week 5)
```bash
# Update CI/CD to use GVTest
# Archive C version
# Document any remaining incompatibilities
```

## Getting Help

If you encounter migration issues:

1. **Check Documentation**: Read [GVTEST_README.md](GVTEST_README.md)
2. **Review Examples**: Look at working tests in `testdata/`
3. **Enable Verbose Mode**: Run with `-v` to see detailed execution
4. **Check GitHub Issues**: Search for similar problems
5. **Ask for Help**: Open an issue with:
   - Your .vtc test file
   - Error messages from both versions
   - Expected vs actual behavior

## Contributing to Compatibility

If you find compatibility issues or missing features:

1. **Report**: Open an issue describing the problem
2. **Document**: Provide example .vtc files
3. **Contribute**: Submit a PR with fixes or enhancements

## Version Compatibility Matrix

| VTest2 Version | GVTest Version | Compatibility | Notes |
|----------------|----------------|---------------|-------|
| Latest (2024) | 0.6.0 | ~80% | Core HTTP testing works |
| Latest (2024) | 0.6.0 + Varnish | ~90% | With Varnish integration |
| Latest (2024) | Future | ~95%+ | Full compatibility planned |

## Summary

GVTest provides strong compatibility with VTest2 for core HTTP testing scenarios. The main limitations are:

1. **No terminal emulation** - affects process tests
2. **No Varnish/HAProxy integration** - affects specific integrations
3. **Some platform differences** - minor behavioral differences

For most HTTP testing use cases, migration should be straightforward. Tests that focus on pure HTTP/1.1 and HTTP/2 protocol testing will work without modification.

---

**Last Updated**: 2025-11-16
**GVTest Version**: 0.6.0 (Phase 6)

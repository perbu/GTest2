# Terminal Emulation Tests

This directory contains VTC test files that require terminal emulation support to run.

## Why These Tests Are Separated

These tests use the `process` command with terminal-specific features:
- `-expect-text ROW COL "text"` - Position-aware text matching
- `-screen_dump` - Terminal screen buffer dumping
- `-ansi-response` - ANSI escape sequence processing
- `shell` - Shell command execution (some variants)

GTest does not currently implement terminal emulation (Teken), so these tests cannot run successfully. They are preserved here for future implementation.

## Test Files

| Test | Description | Blocker |
|------|-------------|---------|
| `a00000.vtc` | VTest framework self-test | process commands with terminal |
| `a00001.vtc` | Teken terminal emulator test | Requires vttest + full terminal emulation |
| `a00008.vtc` | Barrier operations | process + timing issues |
| `a00009.vtc` | Process text matching | -match-text command |
| `a00022.vtc` | Process test | process commands |
| `a00023.vtc` | Process test | process commands |
| `a00026.vtc` | Process test | process commands |
| `a00028.vtc` | Process test | process commands |
| `a02022.vtc` | HTTP/2 + process | process + stream commands |
| `a02025.vtc` | HTTP/2 + process | process + stream commands |

## Implementation Status

See `../../LIMITATIONS.md` section 1 (Terminal Emulation) for implementation plan.

**Estimated effort**: 8-14 hours
**Priority**: Medium (affects ~17% of test suite)

## Running These Tests

Currently, attempting to run these tests will result in:
- Timeouts (for tests with terminal I/O)
- "unknown option" errors (for -run, -expect-text, etc.)
- Hangs (for tests waiting on terminal input)

They are kept for:
1. Future compatibility testing
2. Documentation of required features
3. Reference for terminal emulation implementation

---

**Last Updated**: 2025-11-17

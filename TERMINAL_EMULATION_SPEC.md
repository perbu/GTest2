# Terminal Emulation Specification for GVTest

## Overview

This document specifies how terminal emulation should be integrated into the GVTest (VTest2 Go port) process management system to enable testing of interactive terminal applications.

## Background

The original C implementation of VTest2 uses **Teken**, a VT100-compatible terminal emulator from FreeBSD (~736 LOC in `src/teken.c`). This enables tests to:

1. Run interactive terminal applications (e.g., `vttest`)
2. Check text at specific screen positions
3. Verify terminal output after escape sequences are processed
4. Dump the entire screen buffer for debugging

### Example Use Case

See `tests/a00001.vtc` for a comprehensive example testing `vttest` (VT100 test program):

```vtc
process p4 {vttest} -ansi-response -start
process p4 -expect-text 21 11 "Enter choice number (0 - 12):"
process p4 -writehex "31 0d"
process p4 -expect-text 14 61 "RETURN"
process p4 -screen_dump
```

## Current Implementation Status

The Go port (`pkg/process/process.go`) implements:

✅ Process lifecycle management (`Start`, `Stop`, `Kill`, `Wait`)
✅ I/O pipes (stdin/stdout/stderr)
✅ Output capture in buffers
✅ Basic text matching (`ExpectText` - simple substring search)

❌ Terminal emulation (no VT100/ANSI support)
❌ Screen buffer management
❌ Cursor position tracking
❌ Escape sequence processing

## Requirements

### Functional Requirements

1. **PTY Allocation**: Allocate a pseudo-terminal (PTY) for the process
   - Set terminal size (rows/columns) via `TIOCSWINSZ`
   - Configure as raw mode (no line buffering, echo)
   - Export PTY device path for debugging

2. **Screen Buffer Management**:
   - Maintain a 2D character array (`[][]rune`) representing the terminal screen
   - Default size: 24 rows × 80 columns (configurable)
   - Support for resizing (`-resize ROWS COLS`)
   - Handle scrolling regions

3. **VT100/ANSI Escape Sequence Processing**:
   - Parse and execute escape sequences from process output
   - Minimum required sequences:
     - Cursor movement: `ESC[<row>;<col>H`, `ESC[A/B/C/D` (up/down/right/left)
     - Clear screen: `ESC[2J`, `ESC[K` (clear line)
     - Insert/delete: `ESC[L` (insert line), `ESC[M` (delete line)
     - Scrolling regions: `ESC[<top>;<bottom>r`
     - Character attributes (bold, underline, colors) - optional for display, required for parsing
   - Handle UTF-8 multibyte characters

4. **Commands to Support**:

   **Process start with terminal**:
   ```vtc
   process pNAME {command} -start
   process pNAME {command} -ansi-response -start
   ```
   - `-ansi-response`: Enable ANSI escape sequence processing (requires PTY)

   **Screen position checking**:
   ```vtc
   process pNAME -expect-text ROW COL "expected text"
   ```
   - `ROW`, `COL`: 0-indexed screen coordinates
   - Checks if `expected text` appears at position `(ROW, COL)`
   - Fails test if not found or process hasn't output enough yet
   - Should wait for text to appear (with timeout)

   **Screen dump**:
   ```vtc
   process pNAME -screen_dump
   ```
   - Logs the entire screen buffer to test output
   - Useful for debugging test failures
   - Format: Box-drawn border with row/column indicators

   **Terminal resizing**:
   ```vtc
   process pNAME -resize ROWS COLS
   ```
   - Send `SIGWINCH` to process
   - Update screen buffer dimensions
   - Clear or preserve existing content (preserve preferred)

   **Cursor position query** (optional):
   ```vtc
   process pNAME -expect-cursor ROW COL
   ```
   - Verify cursor is at specific position

5. **Macro Exports**:
   - `${pNAME_out}`: Path to stdout capture file
   - `${pNAME_err}`: Path to stderr capture file
   - `${pNAME_pty}`: Path to PTY device (e.g., `/dev/pts/42`) when using terminal emulation

### Non-Functional Requirements

1. **Performance**: Terminal emulation should not significantly slow down test execution
2. **Memory**: Screen buffer should be bounded (no unlimited scrollback)
3. **Compatibility**: Must work on Linux (primary target) and macOS (secondary)
4. **Error Handling**: Gracefully handle malformed escape sequences (log warning, continue)

## Architecture

### Option 1: Port Teken to Go (Direct Port)

**Approach**: Translate `src/teken.c` and related files to Go

**Pros**:
- Exact behavior match with C version
- Well-tested implementation (from FreeBSD)
- Complete VT100 compatibility

**Cons**:
- ~1000+ LOC to port (teken.c + teken_subr.h + state machine)
- Complex state machine for escape sequence parsing
- Maintenance burden

**Files to port**:
- `src/teken.c` (736 LOC) - main state machine
- `src/teken.h` (100 LOC) - public API
- `src/teken_subr.h` (auto-generated subroutines)
- `src/teken_wcwidth.h` (Unicode width tables)

### Option 2: Use Go PTY Library + Custom Emulator

**Approach**: Use existing Go PTY library (`github.com/creack/pty` or `github.com/hinshun/vt10x`) + custom screen buffer

**Pros**:
- Leverages existing Go libraries
- Smaller implementation (just screen buffer + command handlers)
- Better Go idioms

**Cons**:
- May not match exact C behavior
- Need to verify VT100 compatibility
- Potential library dependency issues

**Recommended libraries**:
1. **PTY allocation**: `github.com/creack/pty` (widely used, 1.9k stars)
2. **Terminal emulation**:
   - `github.com/hinshun/vt10x` (VT10X emulator, 86 stars) - lightweight
   - `github.com/ActiveState/termtest/expect` (higher-level, includes expectations)
   - Roll our own minimal emulator (just what we need)

### Option 3: Hybrid Approach (Recommended)

**Approach**: Use `creack/pty` for PTY + `hinshun/vt10x` for escape sequence parsing + custom command layer

**Why recommended**:
- `creack/pty`: Battle-tested, minimal, pure Go
- `vt10x`: Provides VT10X state machine and screen buffer
- Custom layer: Implements VTest-specific commands (`-expect-text`, `-screen_dump`)

**Implementation outline**:

```go
// pkg/process/terminal.go

import (
    "github.com/creack/pty"
    "github.com/hinshun/vt10x"
)

type Terminal struct {
    PTY      *os.File          // Master side of PTY
    VT       *vt10x.VT         // Terminal emulator
    Rows     int               // Terminal height
    Cols     int               // Terminal width
    mutex    sync.Mutex        // Protect screen access
}

func NewTerminal(rows, cols int) (*Terminal, error) {
    // Create VT emulator
    vt := vt10x.New(vt10x.WithSize(rows, cols))

    return &Terminal{
        VT:   vt,
        Rows: rows,
        Cols: cols,
    }, nil
}

func (t *Terminal) Start(cmd *exec.Cmd) error {
    // Allocate PTY and attach to command
    ptmx, err := pty.Start(cmd)
    if err != nil {
        return err
    }
    t.PTY = ptmx

    // Set terminal size
    _ = pty.Setsize(ptmx, &pty.Winsize{
        Rows: uint16(t.Rows),
        Cols: uint16(t.Cols),
    })

    // Start goroutine to feed PTY output to VT emulator
    go t.readLoop()

    return nil
}

func (t *Terminal) readLoop() {
    buf := make([]byte, 4096)
    for {
        n, err := t.PTY.Read(buf)
        if err != nil {
            return
        }

        t.mutex.Lock()
        t.VT.Write(buf[:n])  // Feed to emulator
        t.mutex.Unlock()
    }
}

func (t *Terminal) ExpectText(row, col int, text string) bool {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    // Get screen content at position
    screen := t.VT.Screen()
    // Extract text starting at (row, col)
    actual := extractText(screen, row, col, len(text))

    return actual == text
}

func (t *Terminal) ScreenDump() string {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    // Format screen buffer as string with borders
    return formatScreen(t.VT.Screen(), t.Rows, t.Cols)
}

func (t *Terminal) Resize(rows, cols int) error {
    t.mutex.Lock()
    defer t.mutex.Unlock()

    t.Rows = rows
    t.Cols = cols

    // Resize PTY
    if t.PTY != nil {
        _ = pty.Setsize(t.PTY, &pty.Winsize{
            Rows: uint16(rows),
            Cols: uint16(cols),
        })
    }

    // Resize VT emulator (if supported)
    // t.VT.Resize(rows, cols)

    return nil
}
```

**Integration with Process**:

```go
// pkg/process/process.go

type Process struct {
    Name      string
    Cmd       *exec.Cmd
    Logger    *logging.Logger

    // Terminal emulation (optional)
    Terminal  *Terminal
    UseTerminal bool

    // ... existing fields ...
}

func (p *Process) Start(useTerminal bool) error {
    if useTerminal {
        p.Terminal = NewTerminal(24, 80)
        return p.Terminal.Start(p.Cmd)
    }

    // Existing pipe-based implementation
    // ... existing code ...
}
```

## Implementation Plan

### Step 1: Evaluate Libraries (1-2 hours)

1. Test `github.com/creack/pty`:
   ```go
   go get github.com/creack/pty
   ```
   - Verify it works on target platforms
   - Check if it handles terminal size correctly

2. Test `github.com/hinshun/vt10x`:
   ```go
   go get github.com/hinshun/vt10x
   ```
   - Run examples
   - Check screen buffer access API
   - Verify escape sequence handling

3. **Decision point**: If libraries work well → proceed with Option 3 (Hybrid)
   - If not → fall back to Option 1 (port Teken)

### Step 2: Implement Terminal Type (2-3 hours)

1. Create `pkg/process/terminal.go`:
   - `Terminal` struct
   - `NewTerminal()` constructor
   - `Start()` method with PTY allocation
   - `readLoop()` for output capture
   - Basic `Write()` method for stdin

2. Write unit test with simple command (e.g., `cat`, `echo`)

### Step 3: Implement Screen Buffer Access (2-3 hours)

1. Implement `ExpectText(row, col, text)`:
   - Wait for text with timeout (polling or notification)
   - Compare at exact position
   - Return error with helpful message if mismatch

2. Implement `ScreenDump()`:
   - Format as box with borders
   - Include row/column numbers
   - Truncate or wrap long lines

3. Test with `vttest` or similar tool

### Step 4: Integrate with Process (1-2 hours)

1. Add `-ansi-response` flag to process command handler
2. Update `pkg/vtc/builtin_commands.go`:
   - Parse `-ansi-response` in process start
   - Route `-expect-text` to `Terminal.ExpectText()`
   - Route `-screen_dump` to `Terminal.ScreenDump()`

3. Update macro export to include `${pNAME_pty}`

### Step 5: Testing & Validation (2-4 hours)

1. Run `tests/a00001.vtc` (if `vttest` available)
2. Create simpler test cases:
   - Test with `cat` (simple echo)
   - Test with `vim` or `nano` (cursor movement)
   - Test with script that outputs ANSI colors

3. Verify:
   - Text appears at correct positions
   - Screen dump is readable
   - Process exit is clean

**Total Estimated Time**: 8-14 hours (1-2 days)

## Testing Strategy

### Unit Tests

```go
// pkg/process/terminal_test.go

func TestTerminalBasic(t *testing.T) {
    term := NewTerminal(24, 80)
    cmd := exec.Command("echo", "Hello")
    term.Start(cmd)

    time.Sleep(100 * time.Millisecond)

    if !term.ExpectText(0, 0, "Hello") {
        t.Error("Expected 'Hello' at (0,0)")
    }
}

func TestTerminalEscapeSequences(t *testing.T) {
    // Test cursor movement, colors, etc.
}
```

### Integration Tests

Create VTC test files:

```vtc
# test_terminal_basic.vtc
vtest "Basic terminal emulation"

process p1 {sh -c "echo 'Hello, World!'"} -ansi-response -start
process p1 -expect-text 0 0 "Hello, World!"
process p1 -wait
```

```vtc
# test_terminal_cursor.vtc
vtest "Terminal cursor movement"

process p2 {
    sh -c "printf 'ABC\033[1;1HX'"
} -ansi-response -start

# 'X' should overwrite 'A' at position (0,0)
process p2 -expect-text 0 0 "XBC"
process p2 -screen_dump
process p2 -wait
```

## Fallback Strategy

If full terminal emulation proves too complex or time-consuming:

1. **Minimal viable implementation**:
   - Parse only common escape sequences (cursor movement, clear)
   - Ignore unsupported sequences (log warning)
   - Focus on making most common tests pass

2. **Skip terminal-heavy tests**:
   - Add feature detection: `feature terminal_emulation`
   - Skip tests that require full VT100 support
   - Document limitation in `LIMITATIONS.md`

3. **Defer to future**:
   - Mark as "Phase 6" or "Future enhancement"
   - Provide stub implementation that fails gracefully
   - Return to this after core functionality is stable

## Dependencies

### External Libraries (Option 3 - Recommended)

```go
// go.mod
require (
    github.com/creack/pty v1.1.21
    github.com/hinshun/vt10x v0.0.0-20220301184237-5011da428d02
)
```

### Platform Support

- **Linux**: Primary target, full support expected
  - Uses `/dev/ptmx` and `/dev/pts/*`
  - Should work on all modern distributions

- **macOS**: Secondary target, best-effort
  - Similar PTY implementation
  - May have minor differences in escape sequence handling

- **Windows**: Not supported initially
  - Windows PTY API is different (ConPTY)
  - Would require separate implementation
  - Document as known limitation

## Documentation Requirements

### User Documentation

Update `README.md` to document:
- Process commands that support terminal emulation
- `-ansi-response` flag
- `-expect-text` row/column indexing (0-based)
- `-screen_dump` output format
- Limitations (which escape sequences are supported)

### Developer Documentation

Create `pkg/process/README.md`:
- Architecture diagram
- How terminal emulation works
- How to add new escape sequences
- Testing guidelines

### Migration Notes

For users migrating from C VTest:
- Note any behavior differences
- List unsupported escape sequences
- Provide workarounds for common issues

## Success Criteria

✅ Can run `tests/a00001.vtc` (or equivalent simpler test)
✅ `-expect-text` works at arbitrary positions
✅ `-screen_dump` produces readable output
✅ Terminal resizing works
✅ No memory leaks or goroutine leaks
✅ Performance acceptable (< 10% overhead vs. non-terminal mode)

## Open Questions

1. **Scrollback buffer**: Should we maintain scrollback history? (C version doesn't)
   - **Recommendation**: No, keep screen buffer fixed size

2. **Character attributes**: Should we preserve bold/color/underline in screen buffer?
   - **Recommendation**: Parse and discard for now (store plain rune)

3. **Alternative character sets**: Support G0/G1 charsets (line drawing)?
   - **Recommendation**: Yes if using vt10x (it handles this), no if custom

4. **Timeout for expect-text**: How long to wait?
   - **Recommendation**: Use process-level timeout (configurable, default 5s)

## References

- Original Teken source: `src/teken.c`, `src/teken.h`
- Original process handler: `src/vtc_process.c`
- VT100 spec: https://vt100.net/docs/vt100-ug/
- ANSI escape codes: https://en.wikipedia.org/wiki/ANSI_escape_code
- `creack/pty`: https://github.com/creack/pty
- `hinshun/vt10x`: https://github.com/hinshun/vt10x

---

**Document Status**: Specification (not implemented)
**Target Phase**: Phase 6 or later (deferred from Phase 5)
**Priority**: Medium (required for ~10% of test suite)
**Complexity**: High (8-14 hours estimated)

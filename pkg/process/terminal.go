// Package process provides terminal emulation support for interactive processes
package process

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/perbu/GTest/pkg/logging"
)

// Terminal provides VT100/ANSI terminal emulation for interactive processes
type Terminal struct {
	PTY        *os.File       // Master side of PTY
	VT         vt10x.Terminal // Terminal emulator
	Rows       int            // Terminal height
	Cols       int            // Terminal width
	logger     *logging.Logger
	mutex      sync.Mutex     // Protect screen access
	done       chan struct{}  // Signal when readLoop completes
	readLoopWg sync.WaitGroup // Track readLoop goroutine
}

// NewTerminal creates a new terminal emulator with specified dimensions
func NewTerminal(rows, cols int, logger *logging.Logger) (*Terminal, error) {
	if rows <= 0 || cols <= 0 {
		return nil, fmt.Errorf("invalid terminal dimensions: %dx%d", rows, cols)
	}

	// Create VT emulator
	vt := vt10x.New(vt10x.WithSize(cols, rows))

	return &Terminal{
		VT:     vt,
		Rows:   rows,
		Cols:   cols,
		logger: logger,
		done:   make(chan struct{}),
	}, nil
}

// Start attaches the terminal to a command and starts the process with PTY
func (t *Terminal) Start(cmd *exec.Cmd) error {
	// Allocate PTY and attach to command
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start process with PTY: %w", err)
	}
	t.PTY = ptmx

	// Set terminal size
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(t.Rows),
		Cols: uint16(t.Cols),
	}); err != nil {
		t.logger.Warning("Failed to set terminal size: %v", err)
	}

	t.logger.Debug("Terminal emulator started with PTY (size: %dx%d)", t.Rows, t.Cols)

	// Start goroutine to feed PTY output to VT emulator
	t.readLoopWg.Add(1)
	go t.readLoop()

	return nil
}

// readLoop continuously reads from PTY and feeds data to VT emulator
func (t *Terminal) readLoop() {
	defer t.readLoopWg.Done()
	defer close(t.done)

	buf := make([]byte, 4096)
	for {
		n, err := t.PTY.Read(buf)
		if err != nil {
			// Process exited or PTY closed
			t.logger.Debug("Terminal readLoop ended: %v", err)
			return
		}

		if n > 0 {
			t.mutex.Lock()
			// Feed raw bytes to VT emulator which will process escape sequences
			if _, err := t.VT.Write(buf[:n]); err != nil {
				t.logger.Warning("VT emulator write error: %v", err)
			}
			t.mutex.Unlock()

			// Log raw output for debugging (optional, can be verbose)
			if t.logger != nil {
				t.logger.Debug("Terminal output (%d bytes): %q", n, buf[:n])
			}
		}
	}
}

// Write sends data to the process via PTY stdin
func (t *Terminal) Write(data []byte) (int, error) {
	if t.PTY == nil {
		return 0, fmt.Errorf("terminal not started")
	}
	return t.PTY.Write(data)
}

// ExpectText checks if the specified text appears at the given row/column position
// Coordinates are 0-indexed. Will wait up to timeout for the text to appear.
func (t *Terminal) ExpectText(row, col int, text string, timeout time.Duration) error {
	if row < 0 || col < 0 {
		return fmt.Errorf("invalid coordinates: row=%d, col=%d", row, col)
	}

	deadline := time.Now().Add(timeout)
	checkInterval := 50 * time.Millisecond

	for {
		t.mutex.Lock()
		actual := t.extractText(row, col, len(text))
		t.mutex.Unlock()

		if actual == text {
			t.logger.Debug("ExpectText matched at (%d,%d): %q", row, col, text)
			return nil
		}

		// Check if we've exceeded timeout
		if time.Now().After(deadline) {
			return fmt.Errorf("expected text %q at position (%d,%d), got %q after timeout %v",
				text, row, col, actual, timeout)
		}

		// Wait a bit before trying again
		time.Sleep(checkInterval)
	}
}

// extractText extracts text from the screen buffer starting at (row, col)
// Must be called with mutex held
func (t *Terminal) extractText(row, col int, length int) string {
	if row < 0 || row >= t.Rows {
		return ""
	}

	var result strings.Builder

	for i := 0; i < length && col+i < t.Cols; i++ {
		// Get cell at position
		cell := t.VT.Cell(col+i, row)
		if cell.Char != 0 {
			// Extract just the rune, ignore formatting attributes
			result.WriteRune(cell.Char)
		} else {
			result.WriteRune(' ')
		}
	}

	return result.String()
}

// ScreenDump returns a formatted representation of the entire screen buffer
func (t *Terminal) ScreenDump() string {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	var buf strings.Builder

	// Top border
	buf.WriteString("┌")
	buf.WriteString(strings.Repeat("─", t.Cols))
	buf.WriteString("┐\n")

	// Screen content with row numbers
	for row := 0; row < t.Rows; row++ {
		buf.WriteString("│")

		for col := 0; col < t.Cols; col++ {
			cell := t.VT.Cell(col, row)
			if cell.Char != 0 {
				// Check if it's a control character or zero
				if cell.Char < 32 || cell.Char == 127 {
					buf.WriteRune(' ')
				} else {
					buf.WriteRune(cell.Char)
				}
			} else {
				buf.WriteRune(' ')
			}
		}

		buf.WriteString("│")
		buf.WriteString(fmt.Sprintf(" %2d\n", row))
	}

	// Bottom border
	buf.WriteString("└")
	buf.WriteString(strings.Repeat("─", t.Cols))
	buf.WriteString("┘\n")

	// Column indicators (every 10 columns)
	buf.WriteString(" ")
	for col := 0; col < t.Cols; col++ {
		if col%10 == 0 {
			buf.WriteString(fmt.Sprintf("%-10d", col))
		}
	}
	buf.WriteString("\n")

	// Cursor position
	cursor := t.VT.Cursor()
	buf.WriteString(fmt.Sprintf("Cursor: (%d, %d)\n", cursor.X, cursor.Y))

	return buf.String()
}

// Resize changes the terminal dimensions
func (t *Terminal) Resize(rows, cols int) error {
	if rows <= 0 || cols <= 0 {
		return fmt.Errorf("invalid terminal dimensions: %dx%d", rows, cols)
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Rows = rows
	t.Cols = cols

	// Resize PTY
	if t.PTY != nil {
		if err := pty.Setsize(t.PTY, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		}); err != nil {
			return fmt.Errorf("failed to resize PTY: %w", err)
		}
	}

	// Resize VT emulator
	t.VT.Resize(cols, rows)

	t.logger.Debug("Terminal resized to %dx%d", rows, cols)
	return nil
}

// GetPTYPath returns the path to the PTY device
func (t *Terminal) GetPTYPath() string {
	if t.PTY == nil {
		return ""
	}
	return t.PTY.Name()
}

// Close closes the terminal and releases resources
func (t *Terminal) Close() error {
	if t.PTY != nil {
		t.PTY.Close()
	}

	// Wait for readLoop to finish
	t.readLoopWg.Wait()

	return nil
}

// Wait waits for the readLoop to complete (process has exited)
func (t *Terminal) Wait() {
	<-t.done
}

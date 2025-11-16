// Package logging provides thread-safe logging infrastructure for gvtest
// Ported from vtc_log.c
package logging

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Log levels
const (
	LevelFatal   = 0 // Fatal error, causes test failure
	LevelError   = 1 // Error
	LevelWarning = 2 // Warning
	LevelInfo    = 3 // Info
	LevelDebug   = 4 // Debug
)

var (
	// Lead prefixes for each log level
	lead = []string{
		"----",
		"*   ",
		"**  ",
		"*** ",
		"****",
	}

	// Global log buffer and mutex for collecting all test output
	globalMutex   sync.Mutex
	globalBuf     bytes.Buffer
	globalStarted bool
	startTime     time.Time
	lastTimestamp int = -1

	// Global verbosity setting
	verboseMode bool
)

// Logger represents a logger instance with a unique ID
type Logger struct {
	id     string
	buf    bytes.Buffer
	mutex  sync.Mutex
	active bool
}

// SetVerbose sets the global verbose mode
func SetVerbose(verbose bool) {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	verboseMode = verbose
}

// IsVerbose returns the current verbose mode
func IsVerbose() bool {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	return verboseMode
}

// NewLogger creates a new logger with the given ID
func NewLogger(id string) *Logger {
	if !globalStarted {
		globalMutex.Lock()
		if !globalStarted {
			startTime = time.Now()
			globalStarted = true
		}
		globalMutex.Unlock()
	}

	return &Logger{
		id: id,
	}
}

// getTimestamp returns the current timestamp in milliseconds since start
func getTimestamp() int {
	if !globalStarted {
		return 0
	}
	elapsed := time.Since(startTime)
	return int(elapsed.Milliseconds())
}

// emit outputs the logger's buffer to the global buffer
func (l *Logger) emit() {
	if l.buf.Len() == 0 {
		return
	}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	// Add timestamp if it changed
	ts := getTimestamp()
	if ts != lastTimestamp {
		fmt.Fprintf(&globalBuf, "**** dT    %d.%03d\n", ts/1000, ts%1000)
		lastTimestamp = ts
	}

	// Copy the logger's buffer to the global buffer
	globalBuf.Write(l.buf.Bytes())
	globalBuf.WriteByte('\n')
}

// leadin writes the log prefix
func (l *Logger) leadin(level int) {
	if level < 0 || level >= len(lead) {
		level = 1
	}
	fmt.Fprintf(&l.buf, "%s %-5s ", lead[level], l.id)
}

// Fatal logs a fatal message and should cause test failure
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.mutex.Lock()
	l.buf.Reset()
	l.active = true

	l.leadin(LevelFatal)
	fmt.Fprintf(&l.buf, format, args...)

	l.active = false
	l.emit()
	l.buf.Reset()
	l.mutex.Unlock()

	// In a real implementation, this would trigger test failure
	panic(fmt.Sprintf("FATAL: "+format, args...))
}

// Log logs a message at the specified level
func (l *Logger) Log(level int, format string, args ...interface{}) {
	if level < 0 {
		return
	}

	// Filter debug messages when not in verbose mode
	if level == LevelDebug && !IsVerbose() {
		return
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.buf.Reset()
	l.active = true

	l.leadin(level)
	fmt.Fprintf(&l.buf, format, args...)

	l.active = false
	l.emit()
	l.buf.Reset()

	if level == LevelFatal {
		panic(fmt.Sprintf("FATAL: "+format, args...))
	}
}

// Dump dumps a string with optional prefix
// If len is negative, the entire string is dumped
func (l *Logger) Dump(level int, prefix string, data string, length int) {
	// Filter debug messages when not in verbose mode
	if level == LevelDebug && !IsVerbose() {
		return
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.buf.Reset()
	l.active = true

	if data == "" {
		l.leadin(level)
		fmt.Fprintf(&l.buf, "%s(null)", prefix)
	} else {
		if length < 0 {
			length = len(data)
		}
		if length > len(data) {
			length = len(data)
		}

		// Truncate if too large
		const maxDump = 8192
		truncated := false
		if length > maxDump {
			truncated = true
			length = maxDump
		}

		l.leadin(level)
		fmt.Fprintf(&l.buf, "%s|%s", prefix, quoteString(data[:length]))

		if truncated {
			fmt.Fprintf(&l.buf, "\n")
			l.leadin(level)
			fmt.Fprintf(&l.buf, "%s [...] (%d bytes truncated)", prefix, len(data)-maxDump)
		}
	}

	l.active = false
	l.emit()
	l.buf.Reset()

	if level == LevelFatal {
		panic("FATAL: dump failed")
	}
}

// Hexdump dumps binary data as hexadecimal
func (l *Logger) Hexdump(level int, prefix string, data []byte) {
	// Filter debug messages when not in verbose mode
	if level == LevelDebug && !IsVerbose() {
		return
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.buf.Reset()
	l.active = true

	if data == nil {
		l.leadin(level)
		fmt.Fprintf(&l.buf, "%s(null)", prefix)
	} else {
		length := len(data)
		if length > 512 {
			length = 512
		}

		for i := 0; i < length; i++ {
			if i%16 == 0 {
				if i > 0 {
					l.buf.WriteByte('\n')
				}
				l.leadin(level)
				fmt.Fprintf(&l.buf, "%s| ", prefix)
			}
			fmt.Fprintf(&l.buf, " %02x", data[i])
		}

		if len(data) > 512 {
			l.buf.WriteString(" ...")
		}
	}

	l.active = false
	l.emit()
	l.buf.Reset()

	if level == LevelFatal {
		panic("FATAL: hexdump failed")
	}
}

// GetOutput returns the accumulated global log output
func GetOutput() string {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	return globalBuf.String()
}

// ResetOutput clears the global log buffer
func ResetOutput() {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	globalBuf.Reset()
	lastTimestamp = -1
	startTime = time.Now()
}

// quoteString quotes a string for safe display
func quoteString(s string) string {
	var buf bytes.Buffer
	for _, c := range s {
		if c >= 32 && c < 127 && c != '\\' && c != '"' {
			buf.WriteRune(c)
		} else if c == '\n' {
			buf.WriteString("\\n")
		} else if c == '\r' {
			buf.WriteString("\\r")
		} else if c == '\t' {
			buf.WriteString("\\t")
		} else if c == '\\' {
			buf.WriteString("\\\\")
		} else if c == '"' {
			buf.WriteString("\\\"")
		} else {
			fmt.Fprintf(&buf, "\\x%02x", c)
		}
	}
	return buf.String()
}

// Logf is a convenience method for formatted logging
func (l *Logger) Logf(level int, format string, args ...interface{}) {
	l.Log(level, format, args...)
}

// Info logs at info level
func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(LevelInfo, format, args...)
}

// Debug logs at debug level
func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(LevelDebug, format, args...)
}

// Error logs at error level
func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(LevelError, format, args...)
}

// Warning logs at warning level
func (l *Logger) Warning(format string, args ...interface{}) {
	l.Log(LevelWarning, format, args...)
}

// ID returns the logger's ID
func (l *Logger) ID() string {
	return l.id
}

// SetID sets the logger's ID
func (l *Logger) SetID(id string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.id = strings.TrimSpace(id)
}

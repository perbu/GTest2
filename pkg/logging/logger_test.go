package logging

import (
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	ResetOutput()
	l := NewLogger("test1")
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
	if l.ID() != "test1" {
		t.Errorf("Expected ID 'test1', got '%s'", l.ID())
	}
}

func TestLogLevels(t *testing.T) {
	ResetOutput()
	SetVerbose(true) // Enable verbose mode to see debug messages
	l := NewLogger("test2")

	l.Info("info message")
	l.Debug("debug message")
	l.Error("error message")
	l.Warning("warning message")

	output := GetOutput()
	if !strings.Contains(output, "info message") {
		t.Error("Output doesn't contain info message")
	}
	if !strings.Contains(output, "debug message") {
		t.Error("Output doesn't contain debug message")
	}

	// Test that debug messages are filtered when verbose is false
	ResetOutput()
	SetVerbose(false)
	l2 := NewLogger("test2b")
	l2.Info("info message 2")
	l2.Debug("debug message 2")

	output2 := GetOutput()
	if !strings.Contains(output2, "info message 2") {
		t.Error("Output doesn't contain info message 2")
	}
	if strings.Contains(output2, "debug message 2") {
		t.Error("Output contains debug message when verbose is false")
	}
}

func TestDump(t *testing.T) {
	ResetOutput()
	l := NewLogger("test3")

	l.Dump(LevelInfo, "DATA", "hello world", -1)

	output := GetOutput()
	if !strings.Contains(output, "DATA") {
		t.Error("Output doesn't contain prefix")
	}
	if !strings.Contains(output, "hello world") {
		t.Error("Output doesn't contain data")
	}
}

func TestHexdump(t *testing.T) {
	ResetOutput()
	l := NewLogger("test4")

	data := []byte{0x01, 0x02, 0x03, 0x04}
	l.Hexdump(LevelInfo, "HEX", data)

	output := GetOutput()
	if !strings.Contains(output, "HEX") {
		t.Error("Output doesn't contain prefix")
	}
	if !strings.Contains(output, "01") {
		t.Error("Output doesn't contain hex data")
	}
}

func TestTimestamp(t *testing.T) {
	ResetOutput()
	l := NewLogger("test5")

	l.Info("message 1")
	l.Info("message 2")

	output := GetOutput()
	if !strings.Contains(output, "dT") {
		t.Error("Output doesn't contain timestamp marker")
	}
}

func TestConcurrentLogging(t *testing.T) {
	ResetOutput()
	l1 := NewLogger("concurrent1")
	l2 := NewLogger("concurrent2")

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 10; i++ {
			l1.Info("message %d", i)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			l2.Info("message %d", i)
		}
		done <- true
	}()

	<-done
	<-done

	// Just verify no panic occurred
	output := GetOutput()
	if len(output) == 0 {
		t.Error("Expected output from concurrent logging")
	}
}

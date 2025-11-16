package session

import (
	"testing"

	"github.com/perbu/GTest/pkg/logging"
)

func TestNew(t *testing.T) {
	logger := logging.NewLogger("test")
	sess := New(logger, "s1")

	if sess.Name != "s1" {
		t.Errorf("Expected name 's1', got '%s'", sess.Name)
	}
	if sess.Repeat != 1 {
		t.Errorf("Expected Repeat=1, got %d", sess.Repeat)
	}
	if sess.Keepalive {
		t.Errorf("Expected Keepalive=false, got true")
	}
	if sess.RcvBuf != 0 {
		t.Errorf("Expected RcvBuf=0, got %d", sess.RcvBuf)
	}
}

func TestParseOption(t *testing.T) {
	logger := logging.NewLogger("test")
	sess := New(logger, "s1")

	tests := []struct {
		args        []string
		wantConsumed int
		wantErr     bool
		checkFunc   func() bool
	}{
		{
			args:        []string{"-repeat", "5"},
			wantConsumed: 2,
			wantErr:     false,
			checkFunc:   func() bool { return sess.Repeat == 5 },
		},
		{
			args:        []string{"-keepalive"},
			wantConsumed: 1,
			wantErr:     false,
			checkFunc:   func() bool { return sess.Keepalive },
		},
		{
			args:        []string{"-rcvbuf", "8192"},
			wantConsumed: 2,
			wantErr:     false,
			checkFunc:   func() bool { return sess.RcvBuf == 8192 },
		},
		{
			args:        []string{"-repeat"},
			wantConsumed: 0,
			wantErr:     true,
			checkFunc:   func() bool { return true },
		},
		{
			args:        []string{"-repeat", "invalid"},
			wantConsumed: 0,
			wantErr:     true,
			checkFunc:   func() bool { return true },
		},
		{
			args:        []string{"-unknown"},
			wantConsumed: 0,
			wantErr:     false,
			checkFunc:   func() bool { return true },
		},
	}

	for _, tt := range tests {
		sess = New(logger, "s1") // Reset session for each test
		consumed, err := sess.ParseOption(tt.args)

		if (err != nil) != tt.wantErr {
			t.Errorf("ParseOption(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			continue
		}
		if consumed != tt.wantConsumed {
			t.Errorf("ParseOption(%v) consumed = %d, want %d", tt.args, consumed, tt.wantConsumed)
			continue
		}
		if !tt.checkFunc() {
			t.Errorf("ParseOption(%v) did not set expected value", tt.args)
		}
	}
}

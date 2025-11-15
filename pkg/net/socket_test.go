package net

import (
	"testing"
	"time"
)

func TestParseAddress(t *testing.T) {
	tests := []struct {
		addr     string
		wantHost string
		wantPort string
		wantUnix bool
		wantErr  bool
	}{
		{"localhost:8080", "localhost", "8080", false, false},
		{"127.0.0.1:9000", "127.0.0.1", "9000", false, false},
		{"[::1]:8080", "::1", "8080", false, false},
		{"/tmp/socket", "/tmp/socket", "", true, false},
		{"@abstract", "@abstract", "", true, false},
		{"localhost", "localhost", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			host, port, isUnix, err := ParseAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if host != tt.wantHost {
				t.Errorf("ParseAddress() host = %v, want %v", host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("ParseAddress() port = %v, want %v", port, tt.wantPort)
			}
			if isUnix != tt.wantUnix {
				t.Errorf("ParseAddress() isUnix = %v, want %v", isUnix, tt.wantUnix)
			}
		})
	}
}

func TestIsUnixSocket(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/tmp/socket", true},
		{"@abstract", true},
		{"localhost:8080", false},
		{"127.0.0.1:9000", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsUnixSocket(tt.path); got != tt.want {
				t.Errorf("IsUnixSocket() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPListenAndConnect(t *testing.T) {
	// Create a listener on a random port
	listener, addrInfo, err := TCPListen("127.0.0.1:0", 10)
	if err != nil {
		t.Fatalf("TCPListen() failed: %v", err)
	}
	defer listener.Close()

	if addrInfo.Addr == "" || addrInfo.Port == "" {
		t.Errorf("TCPListen() returned empty address info")
	}

	// Connect to the listener
	connectAddr := addrInfo.Addr + ":" + addrInfo.Port
	conn, err := TCPConnect(connectAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("TCPConnect() failed: %v", err)
	}
	defer conn.Close()

	// Accept the connection
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatalf("Accept() failed: %v", err)
	}
	defer accepted.Close()

	// Test GetLocalAddr and GetRemoteAddr
	localAddr := GetLocalAddr(conn)
	if localAddr.Addr == "" {
		t.Errorf("GetLocalAddr() returned empty address")
	}

	remoteAddr := GetRemoteAddr(conn)
	if remoteAddr.Port != addrInfo.Port {
		t.Errorf("GetRemoteAddr() port = %v, want %v", remoteAddr.Port, addrInfo.Port)
	}
}

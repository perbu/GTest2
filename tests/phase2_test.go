// Phase 2 Integration Test
// Verifies the success criteria for Phase 2: Session & Connection Management
package tests

import (
	"testing"
	"time"

	"github.com/perbu/GTest/pkg/client"
	"github.com/perbu/GTest/pkg/logging"
	"github.com/perbu/GTest/pkg/macro"
	"github.com/perbu/GTest/pkg/server"
)

// TestPhase2_ServerStartRandomPort tests that a server can start on a random port
func TestPhase2_ServerStartRandomPort(t *testing.T) {
	logger := logging.NewLogger("test")
	macros := macro.New()

	// Create server
	s := server.New(logger, macros, "s1")
	s.SetListen("127.0.0.1:0") // Random port

	// Start server
	err := s.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer s.Stop()

	// Verify server is running
	if !s.Running {
		t.Errorf("Server should be running")
	}

	// Verify server has a port assigned
	if s.Port == "" || s.Port == "0" {
		t.Errorf("Server should have a non-zero port assigned, got: %s", s.Port)
	}

	// Verify server has an address
	if s.Addr == "" {
		t.Errorf("Server should have an address assigned")
	}

	t.Logf("Server started on %s:%s", s.Addr, s.Port)
}

// TestPhase2_MacroExport tests that server macros are exported correctly
func TestPhase2_MacroExport(t *testing.T) {
	logger := logging.NewLogger("test")
	macros := macro.New()

	// Create server
	s := server.New(logger, macros, "s1")
	s.SetListen("127.0.0.1:0")

	// Start server
	err := s.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer s.Stop()

	// Check macros
	tests := []struct {
		name     string
		macroKey string
		wantNonEmpty bool
	}{
		{"addr macro", "s1_addr", true},
		{"port macro", "s1_port", true},
		{"sock macro", "s1_sock", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, exists := macros.Get(tt.macroKey)
			if !exists {
				t.Errorf("Macro ${%s} should exist", tt.macroKey)
				return
			}
			if tt.wantNonEmpty && value == "" {
				t.Errorf("Macro ${%s} should not be empty", tt.macroKey)
			}
			t.Logf("${%s} = %s", tt.macroKey, value)
		})
	}
}

// TestPhase2_ClientServerConnection tests that a client can connect to a server
func TestPhase2_ClientServerConnection(t *testing.T) {
	logger := logging.NewLogger("test")
	macros := macro.New()

	// Start server
	s := server.New(logger, macros, "s1")
	s.SetListen("127.0.0.1:0")

	err := s.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer s.Stop()

	// Get server address from macros
	serverSock, exists := macros.Get("s1_sock")
	if !exists {
		t.Fatalf("Server socket macro not found")
	}

	t.Logf("Server listening on: %s", serverSock)

	// Create client and connect
	c := client.New(logger, "c1")
	c.SetConnect(serverSock)

	// Verify client can establish connection
	conn, err := c.Connect()
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	t.Logf("Client connected successfully to %s", serverSock)
}

// TestPhase2_MacroExpansion tests that macros are expanded correctly
func TestPhase2_MacroExpansion(t *testing.T) {
	logger := logging.NewLogger("test")
	macros := macro.New()

	// Start server
	s := server.New(logger, macros, "s1")
	s.SetListen("127.0.0.1:0")

	err := s.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer s.Stop()

	// Test macro expansion
	text := "Server is at ${s1_addr}:${s1_port} (${s1_sock})"
	expanded, err := macros.Expand(logger, text)
	if err != nil {
		t.Fatalf("Macro expansion failed: %v", err)
	}

	t.Logf("Original: %s", text)
	t.Logf("Expanded: %s", expanded)

	// Verify expansion happened
	if expanded == text {
		t.Errorf("Macro expansion did not occur")
	}
}

// TestPhase2_SessionRepeat tests that sessions can repeat
func TestPhase2_SessionRepeat(t *testing.T) {
	logger := logging.NewLogger("test")
	macros := macro.New()

	// Start server
	s := server.New(logger, macros, "s1")
	s.SetListen("127.0.0.1:0")

	err := s.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer s.Stop()

	// Get server address
	serverSock, _ := macros.Get("s1_sock")

	// Create client with repeat
	c := client.New(logger, "c1")
	c.SetConnect(serverSock)

	// Set repeat to 3
	_, err = c.Session.ParseOption([]string{"-repeat", "3"})
	if err != nil {
		t.Fatalf("Failed to set repeat: %v", err)
	}

	// Verify repeat is set
	if c.Session.Repeat != 3 {
		t.Errorf("Expected repeat=3, got %d", c.Session.Repeat)
	}

	t.Logf("Client session configured with repeat=%d", c.Session.Repeat)
}

// TestPhase2_MultipleServers tests that multiple servers can run simultaneously
func TestPhase2_MultipleServers(t *testing.T) {
	logger := logging.NewLogger("test")
	macros := macro.New()

	// Start multiple servers
	servers := []*server.Server{}
	for i := 0; i < 3; i++ {
		name := []string{"s1", "s2", "s3"}[i]
		s := server.New(logger, macros, name)
		s.SetListen("127.0.0.1:0")

		err := s.Start(nil)
		if err != nil {
			t.Fatalf("Failed to start server %s: %v", name, err)
		}
		defer s.Stop()

		servers = append(servers, s)
	}

	// Verify all servers are running with different ports
	ports := make(map[string]bool)
	for _, s := range servers {
		if !s.Running {
			t.Errorf("Server %s should be running", s.Name)
		}

		if ports[s.Port] {
			t.Errorf("Server %s has duplicate port %s", s.Name, s.Port)
		}
		ports[s.Port] = true

		t.Logf("Server %s running on port %s", s.Name, s.Port)
	}

	// Give servers time to fully start
	time.Sleep(100 * time.Millisecond)
}

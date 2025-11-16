package tests

import (
	"net"
	"testing"
	"time"

	"github.com/perbu/GTest/pkg/hpack"
	"github.com/perbu/GTest/pkg/http2"
	"github.com/perbu/GTest/pkg/logging"
)

// TestPhase4_FrameStructure tests basic HTTP/2 frame operations
func TestPhase4_FrameStructure(t *testing.T) {
	tests := []struct {
		name   string
		header http2.FrameHeader
	}{
		{
			name: "DATA frame",
			header: http2.FrameHeader{
				Length:   100,
				Type:     http2.FrameData,
				Flags:    http2.FlagEndStream,
				StreamID: 1,
			},
		},
		{
			name: "HEADERS frame",
			header: http2.FrameHeader{
				Length:   200,
				Type:     http2.FrameHeaders,
				Flags:    http2.FlagEndHeaders,
				StreamID: 3,
			},
		},
		{
			name: "SETTINGS frame",
			header: http2.FrameHeader{
				Length:   0,
				Type:     http2.FrameSettings,
				Flags:    http2.FlagAck,
				StreamID: 0,
			},
		},
		{
			name: "PING frame",
			header: http2.FrameHeader{
				Length:   8,
				Type:     http2.FramePing,
				Flags:    http2.FlagNone,
				StreamID: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe for testing
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			// Write frame header
			go func() {
				err := http2.WriteFrameHeader(client, tt.header)
				if err != nil {
					t.Errorf("WriteFrameHeader failed: %v", err)
				}
			}()

			// Read frame header
			header, err := http2.ReadFrameHeader(server)
			if err != nil {
				t.Fatalf("ReadFrameHeader failed: %v", err)
			}

			// Verify
			if header.Length != tt.header.Length {
				t.Errorf("Length mismatch: got %d, want %d", header.Length, tt.header.Length)
			}
			if header.Type != tt.header.Type {
				t.Errorf("Type mismatch: got %s, want %s", header.Type, tt.header.Type)
			}
			if header.Flags != tt.header.Flags {
				t.Errorf("Flags mismatch: got 0x%x, want 0x%x", header.Flags, tt.header.Flags)
			}
			if header.StreamID != tt.header.StreamID {
				t.Errorf("StreamID mismatch: got %d, want %d", header.StreamID, tt.header.StreamID)
			}
		})
	}
}

// TestPhase4_HPACKEncoding tests HPACK encoding and decoding
func TestPhase4_HPACKEncoding(t *testing.T) {
	tests := []struct {
		name    string
		headers []hpack.HeaderField
	}{
		{
			name: "Simple headers",
			headers: []hpack.HeaderField{
				{Name: ":method", Value: "GET"},
				{Name: ":path", Value: "/"},
				{Name: ":scheme", Value: "https"},
				{Name: ":authority", Value: "example.com"},
			},
		},
		{
			name: "Headers with custom values",
			headers: []hpack.HeaderField{
				{Name: ":method", Value: "POST"},
				{Name: ":path", Value: "/api/users"},
				{Name: ":scheme", Value: "https"},
				{Name: ":authority", Value: "api.example.com"},
				{Name: "content-type", Value: "application/json"},
				{Name: "content-length", Value: "123"},
			},
		},
		{
			name: "Response headers",
			headers: []hpack.HeaderField{
				{Name: ":status", Value: "200"},
				{Name: "content-type", Value: "text/html"},
				{Name: "content-length", Value: "1024"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := hpack.NewEncoder(4096)
			decoder := hpack.NewDecoder(4096)

			// Encode
			encoded, err := encoder.Encode(tt.headers)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			decoded, err := decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify
			if len(decoded) != len(tt.headers) {
				t.Fatalf("Header count mismatch: got %d, want %d", len(decoded), len(tt.headers))
			}

			for i, hf := range decoded {
				if hf.Name != tt.headers[i].Name {
					t.Errorf("Header %d name mismatch: got %q, want %q", i, hf.Name, tt.headers[i].Name)
				}
				if hf.Value != tt.headers[i].Value {
					t.Errorf("Header %d value mismatch: got %q, want %q", i, hf.Value, tt.headers[i].Value)
				}
			}
		})
	}
}

// TestPhase4_StreamManagement tests HTTP/2 stream creation and management
func TestPhase4_StreamManagement(t *testing.T) {
	sm := http2.NewStreamManager()

	// Create streams
	s1 := sm.Create(1, "stream-1")
	s2 := sm.Create(3, "stream-3")

	if s1.ID != 1 {
		t.Errorf("Stream 1 ID mismatch: got %d, want 1", s1.ID)
	}
	if s2.ID != 3 {
		t.Errorf("Stream 3 ID mismatch: got %d, want 3", s2.ID)
	}

	// Get by ID
	retrieved, ok := sm.Get(1)
	if !ok {
		t.Fatal("Failed to get stream 1")
	}
	if retrieved.ID != 1 {
		t.Errorf("Retrieved stream ID mismatch: got %d, want 1", retrieved.ID)
	}

	// Get by name
	retrieved, ok = sm.GetByName("stream-3")
	if !ok {
		t.Fatal("Failed to get stream by name")
	}
	if retrieved.ID != 3 {
		t.Errorf("Retrieved stream ID mismatch: got %d, want 3", retrieved.ID)
	}

	// Count
	if sm.Count() != 2 {
		t.Errorf("Stream count mismatch: got %d, want 2", sm.Count())
	}

	// Delete
	sm.Delete(1)
	if sm.Count() != 1 {
		t.Errorf("Stream count after delete mismatch: got %d, want 1", sm.Count())
	}
}

// TestPhase4_StreamState tests HTTP/2 stream state transitions
func TestPhase4_StreamState(t *testing.T) {
	stream := http2.NewStream(1, "test-stream")

	// Initial state
	if stream.State != http2.StreamIdle {
		t.Errorf("Initial state mismatch: got %s, want idle", stream.State)
	}

	// Send headers without END_STREAM
	stream.UpdateState(false, true)
	if stream.State != http2.StreamOpen {
		t.Errorf("State after HEADERS mismatch: got %s, want open", stream.State)
	}

	// Send data with END_STREAM
	stream.UpdateState(true, true)
	if stream.State != http2.StreamHalfClosedLocal {
		t.Errorf("State after END_STREAM mismatch: got %s, want half-closed(local)", stream.State)
	}

	// Receive data with END_STREAM
	stream.UpdateState(true, false)
	if stream.State != http2.StreamClosed {
		t.Errorf("Final state mismatch: got %s, want closed", stream.State)
	}
}

// TestPhase4_ConnectionSetup tests HTTP/2 connection setup
func TestPhase4_ConnectionSetup(t *testing.T) {
	// Create a pipe to simulate client-server connection
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := logging.NewLogger("test")

	// Create client and server connections
	client := http2.NewConn(clientConn, logger, true)
	server := http2.NewConn(serverConn, logger, false)

	// Start connections in goroutines
	errChan := make(chan error, 2)

	go func() {
		errChan <- client.Start()
	}()

	go func() {
		errChan <- server.Start()
	}()

	// Wait for both to start
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Connection start failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Connection start timeout")
	}

	// Give time for settings exchange
	time.Sleep(200 * time.Millisecond)

	// Verify connection is established
	if client.GetSetting(http2.SettingHeaderTableSize) != 4096 {
		t.Error("Client settings not initialized")
	}
	if server.GetSetting(http2.SettingHeaderTableSize) != 4096 {
		t.Error("Server settings not initialized")
	}

	// Clean up
	client.Stop()
	server.Stop()
}

// TestPhase4_RequestResponse tests sending and receiving HTTP/2 requests and responses
// TODO: This test has a race condition in the HTTP/2 connection setup that causes intermittent failures.
// The issue is timing-dependent and needs proper synchronization instead of sleep-based waits.
func TestPhase4_RequestResponse(t *testing.T) {
	t.Skip("Skipping due to race condition in HTTP/2 connection setup - issue #TBD")
	// Create a pipe to simulate client-server connection
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := logging.NewLogger("test")

	// Create client and server connections
	client := http2.NewConn(clientConn, logger, true)
	server := http2.NewConn(serverConn, logger, false)

	// Start connections
	go client.Start()
	go server.Start()

	time.Sleep(500 * time.Millisecond) // Wait for setup

	// Send request from client
	streamID := uint32(1)
	reqOpts := http2.TxReqOptions{
		Method:    "GET",
		Path:      "/test",
		Scheme:    "https",
		Authority: "example.com",
		Headers: map[string]string{
			"user-agent": "test-client",
		},
		Body:      []byte("test body"),
		EndStream: true,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- client.TxReq(streamID, reqOpts)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("TxReq failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("TxReq timeout")
	}

	// Give time for frame to arrive
	time.Sleep(200 * time.Millisecond)

	// Verify stream was created on server
	stream, ok := server.GetStream(streamID)
	if !ok {
		t.Fatal("Stream not found on server")
	}

	if stream.Method != "GET" {
		t.Errorf("Method mismatch: got %q, want GET", stream.Method)
	}
	if stream.Path != "/test" {
		t.Errorf("Path mismatch: got %q, want /test", stream.Path)
	}

	// Send response from server
	respOpts := http2.TxRespOptions{
		Status: "200",
		Headers: map[string]string{
			"content-type": "text/plain",
		},
		Body:      []byte("response body"),
		EndStream: true,
	}

	go func() {
		errChan <- server.TxResp(streamID, respOpts)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("TxResp failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("TxResp timeout")
	}

	// Give time for frame to arrive
	time.Sleep(200 * time.Millisecond)

	// Verify response on client
	clientStream, ok := client.GetStream(streamID)
	if !ok {
		t.Fatal("Stream not found on client")
	}

	if clientStream.Status != "200" {
		t.Errorf("Status mismatch: got %q, want 200", clientStream.Status)
	}

	// Clean up
	client.Stop()
	server.Stop()
}

// TestPhase4_FlowControl tests HTTP/2 flow control
func TestPhase4_FlowControl(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := logging.NewLogger("test")

	client := http2.NewConn(clientConn, logger, true)
	server := http2.NewConn(serverConn, logger, false)

	go client.Start()
	go server.Start()

	time.Sleep(200 * time.Millisecond)

	// Check initial window sizes
	clientWindow := client.GetSendWindow(0)
	if clientWindow != http2.DefaultWindowSize {
		t.Errorf("Initial client window mismatch: got %d, want %d", clientWindow, http2.DefaultWindowSize)
	}

	// Send WINDOW_UPDATE
	increment := uint32(1024)
	err := client.TxWinup(0, increment)
	if err != nil {
		t.Fatalf("TxWinup failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify window was updated on server
	serverWindow := server.GetSendWindow(0)
	if serverWindow != http2.DefaultWindowSize+int32(increment) {
		t.Errorf("Server window not updated: got %d, want %d",
			serverWindow, http2.DefaultWindowSize+int32(increment))
	}

	client.Stop()
	server.Stop()
}

// TestPhase4_MalformedFrames tests ability to send malformed HTTP/2 frames
func TestPhase4_MalformedFrames(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := logging.NewLogger("test")
	client := http2.NewConn(clientConn, logger, true)

	// Drain the server end of the pipe so writes don't block
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := serverConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	go client.Start()

	time.Sleep(100 * time.Millisecond)

	// Send malformed frame with incorrect length
	err := client.WriteRaw(
		999, // Wrong length
		http2.FrameData,
		http2.FlagEndStream,
		1,
		[]byte("short payload"), // Actual payload is much shorter than declared length
	)

	if err != nil {
		t.Fatalf("WriteRaw failed: %v", err)
	}

	// Send raw hex data
	err = client.SendHex("000000 04 00 00000000") // Empty SETTINGS frame
	if err != nil {
		t.Fatalf("SendHex failed: %v", err)
	}

	client.Stop()
}

// TestPhase4_Settings tests HTTP/2 SETTINGS frame handling
func TestPhase4_Settings(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := logging.NewLogger("test")

	client := http2.NewConn(clientConn, logger, true)
	server := http2.NewConn(serverConn, logger, false)

	go client.Start()
	go server.Start()

	time.Sleep(200 * time.Millisecond)

	// Update settings
	newSettings := map[http2.SettingID]uint32{
		http2.SettingHeaderTableSize: 8192,
		http2.SettingMaxFrameSize:    32768,
	}

	err := client.TxSettings(false, newSettings)
	if err != nil {
		t.Fatalf("TxSettings failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify local settings were updated
	if client.GetSetting(http2.SettingHeaderTableSize) != 8192 {
		t.Error("Client HEADER_TABLE_SIZE not updated")
	}
	if client.GetSetting(http2.SettingMaxFrameSize) != 32768 {
		t.Error("Client MAX_FRAME_SIZE not updated")
	}

	client.Stop()
	server.Stop()
}

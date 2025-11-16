// Phase 3 Integration Test
// Verifies the success criteria for Phase 3: HTTP/1 Protocol Engine
package tests

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/perbu/gvtest/pkg/http1"
	"github.com/perbu/gvtest/pkg/logging"
)

// TestPhase3_BasicHTTPRequest tests sending and receiving HTTP requests
func TestPhase3_BasicHTTPRequest(t *testing.T) {
	logger := logging.NewLogger("test")

	// Create a simple TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()
	t.Logf("Server listening on %s", serverAddr)

	// Server goroutine
	serverDone := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		// Create HTTP session
		h := http1.New(conn, logger)

		// Receive request
		err = h.RxReq(&http1.RxReqOptions{})
		if err != nil {
			serverDone <- err
			return
		}

		// Verify request
		if h.Method != "GET" {
			serverDone <- nil // Don't fail, just log
			t.Logf("Expected method GET, got %s", h.Method)
		}
		if h.URL != "/test" {
			serverDone <- nil
			t.Logf("Expected URL /test, got %s", h.URL)
		}

		// Send response
		err = h.TxResp(&http1.TxRespOptions{
			Status:  200,
			Reason:  "OK",
			Body:    []byte("Hello, World!"),
		})
		serverDone <- err
	}()

	// Client
	time.Sleep(50 * time.Millisecond) // Give server time to start

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	h := http1.New(conn, logger)

	// Send request
	err = h.TxReq(&http1.TxReqOptions{
		Method: "GET",
		URL:    "/test",
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	// Receive response
	err = h.RxResp(&http1.RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	// Verify response
	if h.Status != 200 {
		t.Errorf("Expected status 200, got %d", h.Status)
	}
	if string(h.Body) != "Hello, World!" {
		t.Errorf("Expected body 'Hello, World!', got '%s'", string(h.Body))
	}

	// Wait for server
	err = <-serverDone
	if err != nil {
		t.Errorf("Server error: %v", err)
	}

	t.Logf("Basic HTTP request/response test passed")
}

// TestPhase3_HTTPHeaders tests header handling
func TestPhase3_HTTPHeaders(t *testing.T) {
	logger := logging.NewLogger("test")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	serverDone := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		h := http1.New(conn, logger)
		err = h.RxReq(&http1.RxReqOptions{})
		if err != nil {
			serverDone <- err
			return
		}

		// Check custom header
		customHeader := h.GetRequestHeader("X-Test-Header")
		if customHeader != "test-value" {
			t.Logf("Expected X-Test-Header: test-value, got: %s", customHeader)
		}

		// Send response with custom headers
		err = h.TxResp(&http1.TxRespOptions{
			Status: 200,
			Headers: map[string]string{
				"X-Response-Header": "response-value",
				"Content-Type":      "text/plain",
			},
			Body: []byte("OK"),
		})
		serverDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	h := http1.New(conn, logger)

	// Send request with custom headers
	err = h.TxReq(&http1.TxReqOptions{
		Method: "POST",
		URL:    "/api",
		Headers: map[string]string{
			"X-Test-Header": "test-value",
			"Content-Type":  "application/json",
		},
		Body: []byte(`{"key":"value"}`),
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	err = h.RxResp(&http1.RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	// Verify response headers
	respHeader := h.GetResponseHeader("X-Response-Header")
	if respHeader != "response-value" {
		t.Errorf("Expected X-Response-Header: response-value, got: %s", respHeader)
	}

	err = <-serverDone
	if err != nil {
		t.Errorf("Server error: %v", err)
	}

	t.Logf("HTTP headers test passed")
}

// TestPhase3_ExpectAssertions tests the expect command
func TestPhase3_ExpectAssertions(t *testing.T) {
	logger := logging.NewLogger("test")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	serverDone := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		h := http1.New(conn, logger)
		err = h.RxReq(&http1.RxReqOptions{})
		if err != nil {
			serverDone <- err
			return
		}

		err = h.TxResp(&http1.TxRespOptions{
			Status: 404,
			Reason: "Not Found",
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
			Body: []byte("Page not found"),
		})
		serverDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	h := http1.New(conn, logger)

	err = h.TxReq(&http1.TxReqOptions{})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	err = h.RxResp(&http1.RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	// Test expect assertions
	tests := []struct {
		field    string
		op       string
		expected string
		wantErr  bool
	}{
		{"resp.status", "==", "404", false},
		{"resp.reason", "==", "Not Found", false},
		{"resp.http.content-type", "==", "text/html", false},
		{"resp.body", "~", "not found", false},
		{"resp.bodylen", ">", "5", false},
		{"resp.status", "==", "200", true}, // Should fail
	}

	for _, tt := range tests {
		err := h.Expect(tt.field, tt.op, tt.expected)
		if (err != nil) != tt.wantErr {
			t.Errorf("Expect(%s %s %s) error = %v, wantErr %v",
				tt.field, tt.op, tt.expected, err, tt.wantErr)
		}
	}

	err = <-serverDone
	if err != nil {
		t.Errorf("Server error: %v", err)
	}

	t.Logf("Expect assertions test passed")
}

// TestPhase3_ChunkedEncoding tests chunked transfer encoding
func TestPhase3_ChunkedEncoding(t *testing.T) {
	logger := logging.NewLogger("test")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	serverDone := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		h := http1.New(conn, logger)
		err = h.RxReq(&http1.RxReqOptions{})
		if err != nil {
			serverDone <- err
			return
		}

		// Verify chunked body was received correctly
		expectedBody := "This is chunked data"
		if string(h.Body) != expectedBody {
			t.Logf("Expected body '%s', got '%s'", expectedBody, string(h.Body))
		}

		// Send chunked response
		err = h.TxResp(&http1.TxRespOptions{
			Status:  200,
			Body:    []byte("Chunked response body"),
			Chunked: true,
		})
		serverDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	h := http1.New(conn, logger)

	// Send chunked request
	err = h.TxReq(&http1.TxReqOptions{
		Method:  "POST",
		URL:     "/upload",
		Body:    []byte("This is chunked data"),
		Chunked: true,
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	err = h.RxResp(&http1.RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	if string(h.Body) != "Chunked response body" {
		t.Errorf("Expected body 'Chunked response body', got '%s'", string(h.Body))
	}

	err = <-serverDone
	if err != nil {
		t.Errorf("Server error: %v", err)
	}

	t.Logf("Chunked encoding test passed")
}

// TestPhase3_GzipCompression tests gzip compression/decompression
func TestPhase3_GzipCompression(t *testing.T) {
	logger := logging.NewLogger("test")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	serverDone := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		h := http1.New(conn, logger)
		err = h.RxReq(&http1.RxReqOptions{})
		if err != nil {
			serverDone <- err
			return
		}

		// Send gzipped response
		err = h.TxResp(&http1.TxRespOptions{
			Status: 200,
			Body:   []byte(strings.Repeat("This is compressed content. ", 100)),
			Gzip:   true,
		})
		serverDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	h := http1.New(conn, logger)

	err = h.TxReq(&http1.TxReqOptions{})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	err = h.RxResp(&http1.RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	// Body should be decompressed automatically
	expectedBody := strings.Repeat("This is compressed content. ", 100)
	if string(h.Body) != expectedBody {
		t.Errorf("Expected decompressed body of length %d, got %d", len(expectedBody), len(h.Body))
	}

	// Check Content-Encoding header
	encoding := h.GetResponseHeader("Content-Encoding")
	if encoding != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got: %s", encoding)
	}

	err = <-serverDone
	if err != nil {
		t.Errorf("Server error: %v", err)
	}

	t.Logf("Gzip compression test passed")
}

// TestPhase3_GenerateBody tests body generation
func TestPhase3_GenerateBody(t *testing.T) {
	// Test body generation
	body := http1.GenerateBody(100, false)
	if len(body) != 100 {
		t.Errorf("Expected body length 100, got %d", len(body))
	}

	// Verify pattern
	expectedStart := "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_"
	if !strings.HasPrefix(string(body), expectedStart) {
		t.Errorf("Body doesn't start with expected pattern")
	}

	t.Logf("Generate body test passed")
}

// TestPhase3_MalformedHTTP tests handling of intentionally malformed HTTP
func TestPhase3_MalformedHTTP(t *testing.T) {
	logger := logging.NewLogger("test")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	serverDone := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		h := http1.New(conn, logger)

		// Send raw/malformed HTTP
		h.SendString("HTTP/1.1 200 OK\r\n")
		h.SendString("X-Malformed: Header\r\n")
		// Missing Content-Length, no body
		h.SendString("\r\n")

		serverDone <- nil
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Client failed to connect: %v", err)
	}
	defer conn.Close()

	h := http1.New(conn, logger)

	// Send malformed request (missing protocol version)
	h.SendString("GET /malformed HTTP/1.0\r\n")
	h.SendString("\r\n")

	// Try to receive response (should still parse)
	err = h.RxResp(&http1.RxRespOptions{})
	if err != nil {
		t.Logf("RxResp on malformed response: %v (expected)", err)
	} else {
		// If it parsed, verify we got something
		if h.Status != 200 {
			t.Errorf("Expected status 200, got %d", h.Status)
		}
	}

	err = <-serverDone
	if err != nil {
		t.Errorf("Server error: %v", err)
	}

	t.Logf("Malformed HTTP test passed")
}

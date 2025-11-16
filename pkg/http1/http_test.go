package http1

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/perbu/gvtest/pkg/logging"
)

// mockConn is a mock connection for testing
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func newMockConn(data string) *mockConn {
	return &mockConn{
		readBuf:  bytes.NewBufferString(data),
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *mockConn) Written() string {
	return m.writeBuf.String()
}

// Test Request Parsing (RxReq)

func TestRxReq_SimpleGET(t *testing.T) {
	data := "GET /index.html HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"User-Agent: test\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxReq(&RxReqOptions{})
	if err != nil {
		t.Fatalf("RxReq failed: %v", err)
	}

	if h.Method != "GET" {
		t.Errorf("Expected method GET, got %s", h.Method)
	}
	if h.URL != "/index.html" {
		t.Errorf("Expected URL /index.html, got %s", h.URL)
	}
	if h.Proto != "HTTP/1.1" {
		t.Errorf("Expected proto HTTP/1.1, got %s", h.Proto)
	}
	if h.GetRequestHeader("Host") != "example.com" {
		t.Errorf("Expected Host header example.com, got %s", h.GetRequestHeader("Host"))
	}
}

func TestRxReq_POSTWithBody(t *testing.T) {
	body := "test=data&foo=bar"
	data := "POST /api/endpoint HTTP/1.1\r\n" +
		"Host: api.example.com\r\n" +
		"Content-Type: application/x-www-form-urlencoded\r\n" +
		"Content-Length: " + strconv.Itoa(len(body)) + "\r\n" +
		"\r\n" +
		body

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxReq(&RxReqOptions{})
	if err != nil {
		t.Fatalf("RxReq failed: %v", err)
	}

	if h.Method != "POST" {
		t.Errorf("Expected method POST, got %s", h.Method)
	}
	if string(h.Body) != body {
		t.Errorf("Expected body %s, got %s", body, string(h.Body))
	}
}

func TestRxReq_ChunkedBody(t *testing.T) {
	data := "POST /upload HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"5\r\n" +
		"Hello\r\n" +
		"6\r\n" +
		" World\r\n" +
		"0\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxReq(&RxReqOptions{})
	if err != nil {
		t.Fatalf("RxReq failed: %v", err)
	}

	expected := "Hello World"
	if string(h.Body) != expected {
		t.Errorf("Expected body '%s', got '%s'", expected, string(h.Body))
	}
}

func TestRxReq_InvalidRequestLine(t *testing.T) {
	data := "INVALID\r\n\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxReq(&RxReqOptions{})
	if err == nil {
		t.Fatal("Expected error for invalid request line, got nil")
	}
	if !strings.Contains(err.Error(), "invalid request line") {
		t.Errorf("Expected 'invalid request line' error, got: %v", err)
	}
}

func TestRxReq_HEADMethod(t *testing.T) {
	data := "HEAD /index.html HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxReq(&RxReqOptions{})
	if err != nil {
		t.Fatalf("RxReq failed: %v", err)
	}

	if !h.HeadMethod {
		t.Error("Expected HeadMethod to be true for HEAD request")
	}
}

// Test Response Parsing (RxResp)

func TestRxResp_SimpleOK(t *testing.T) {
	data := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"Hello"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	if h.Status != 200 {
		t.Errorf("Expected status 200, got %d", h.Status)
	}
	if h.Reason != "OK" {
		t.Errorf("Expected reason OK, got %s", h.Reason)
	}
	if string(h.Body) != "Hello" {
		t.Errorf("Expected body 'Hello', got '%s'", string(h.Body))
	}
}

func TestRxResp_NoContent(t *testing.T) {
	data := "HTTP/1.1 204 No Content\r\n" +
		"Server: test\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	if h.Status != 204 {
		t.Errorf("Expected status 204, got %d", h.Status)
	}
	if h.BodyLen != 0 {
		t.Errorf("Expected no body for 204, got %d bytes", h.BodyLen)
	}
}

func TestRxResp_NotModified(t *testing.T) {
	data := "HTTP/1.1 304 Not Modified\r\n" +
		"ETag: \"abc123\"\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	if h.Status != 304 {
		t.Errorf("Expected status 304, got %d", h.Status)
	}
	if h.BodyLen != 0 {
		t.Errorf("Expected no body for 304, got %d bytes", h.BodyLen)
	}
}

func TestRxResp_ChunkedBody(t *testing.T) {
	data := "HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"7\r\n" +
		"chunked\r\n" +
		"4\r\n" +
		"data\r\n" +
		"0\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	expected := "chunkeddata"
	if string(h.Body) != expected {
		t.Errorf("Expected body '%s', got '%s'", expected, string(h.Body))
	}
}

func TestRxResp_InvalidStatusLine(t *testing.T) {
	data := "INVALID\r\n\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{})
	if err == nil {
		t.Fatal("Expected error for invalid status line, got nil")
	}
	if !strings.Contains(err.Error(), "invalid status line") {
		t.Errorf("Expected 'invalid status line' error, got: %v", err)
	}
}

func TestRxResp_InvalidStatusCode(t *testing.T) {
	data := "HTTP/1.1 ABC OK\r\n\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{})
	if err == nil {
		t.Fatal("Expected error for invalid status code, got nil")
	}
	if !strings.Contains(err.Error(), "invalid status code") {
		t.Errorf("Expected 'invalid status code' error, got: %v", err)
	}
}

func TestRxResp_NoObj(t *testing.T) {
	data := "HTTP/1.1 200 OK\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"Hello"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.RxResp(&RxRespOptions{NoObj: true})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	if h.BodyLen != 0 {
		t.Errorf("Expected no body with NoObj, got %d bytes", h.BodyLen)
	}
}

// Test Request Building (TxReq)

func TestTxReq_SimpleGET(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxReq(&TxReqOptions{
		Method: "GET",
		URL:    "/test",
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "GET /test HTTP/1.1\r\n") {
		t.Errorf("Expected GET request line in output, got: %s", written)
	}
	if !strings.Contains(written, "Host: localhost\r\n") {
		t.Errorf("Expected Host header in output, got: %s", written)
	}
	if !strings.Contains(written, "User-Agent: gvtest\r\n") {
		t.Errorf("Expected User-Agent header in output, got: %s", written)
	}
}

func TestTxReq_POSTWithBody(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body := []byte("test data")
	err := h.TxReq(&TxReqOptions{
		Method: "POST",
		URL:    "/api/endpoint",
		Body:   body,
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "POST /api/endpoint HTTP/1.1\r\n") {
		t.Errorf("Expected POST request line in output")
	}
	if !strings.Contains(written, "Content-Length: 9\r\n") {
		t.Errorf("Expected Content-Length header in output")
	}
	if !strings.Contains(written, "test data") {
		t.Errorf("Expected body in output")
	}
}

func TestTxReq_ChunkedEncoding(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body := []byte("chunked data")
	err := h.TxReq(&TxReqOptions{
		Method:  "POST",
		URL:     "/upload",
		Body:    body,
		Chunked: true,
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "Transfer-Encoding: chunked\r\n") {
		t.Errorf("Expected Transfer-Encoding header in output")
	}
	if !strings.Contains(written, "c\r\n") { // hex for 12 (len of "chunked data")
		t.Errorf("Expected chunk size in output, got: %s", written)
	}
	if !strings.Contains(written, "0\r\n\r\n") {
		t.Errorf("Expected final chunk in output")
	}
}

func TestTxReq_CustomHeaders(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxReq(&TxReqOptions{
		Method: "GET",
		URL:    "/",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "X-Custom-Header: custom-value\r\n") {
		t.Errorf("Expected custom header in output")
	}
	if !strings.Contains(written, "Authorization: Bearer token123\r\n") {
		t.Errorf("Expected Authorization header in output")
	}
}

func TestTxReq_NoHostHeader(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxReq(&TxReqOptions{
		Method: "GET",
		URL:    "/",
		NoHost: true,
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	written := conn.Written()
	if strings.Contains(written, "Host:") {
		t.Errorf("Expected no Host header with NoHost option")
	}
}

func TestTxReq_NoUserAgent(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxReq(&TxReqOptions{
		Method:      "GET",
		URL:         "/",
		NoUserAgent: true,
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	written := conn.Written()
	if strings.Contains(written, "User-Agent:") {
		t.Errorf("Expected no User-Agent header with NoUserAgent option")
	}
}

func TestTxReq_GeneratedBody(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxReq(&TxReqOptions{
		Method:  "POST",
		URL:     "/",
		BodyLen: 100,
	})
	if err != nil {
		t.Fatalf("TxReq failed: %v", err)
	}

	if h.BodyLen != 100 {
		t.Errorf("Expected body length 100, got %d", h.BodyLen)
	}

	written := conn.Written()
	if !strings.Contains(written, "Content-Length: 100\r\n") {
		t.Errorf("Expected Content-Length: 100 in output")
	}
}

// Test Response Building (TxResp)

func TestTxResp_SimpleOK(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxResp(&TxRespOptions{
		Status: 200,
		Body:   []byte("Hello, World!"),
	})
	if err != nil {
		t.Fatalf("TxResp failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("Expected status line in output")
	}
	if !strings.Contains(written, "Server: gvtest\r\n") {
		t.Errorf("Expected Server header in output")
	}
	if !strings.Contains(written, "Content-Length: 13\r\n") {
		t.Errorf("Expected Content-Length header in output")
	}
	if !strings.Contains(written, "Hello, World!") {
		t.Errorf("Expected body in output")
	}
}

func TestTxResp_CustomStatus(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxResp(&TxRespOptions{
		Status: 404,
		Reason: "Not Found",
	})
	if err != nil {
		t.Fatalf("TxResp failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "HTTP/1.1 404 Not Found\r\n") {
		t.Errorf("Expected 404 status line in output, got: %s", written)
	}
}

func TestTxResp_ChunkedEncoding(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body := []byte("response data")
	err := h.TxResp(&TxRespOptions{
		Status:  200,
		Body:    body,
		Chunked: true,
	})
	if err != nil {
		t.Fatalf("TxResp failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "Transfer-Encoding: chunked\r\n") {
		t.Errorf("Expected Transfer-Encoding header in output")
	}
	if !strings.Contains(written, "d\r\n") { // hex for 13 (len of "response data")
		t.Errorf("Expected chunk size in output")
	}
	if !strings.Contains(written, "0\r\n\r\n") {
		t.Errorf("Expected final chunk in output")
	}
}

func TestTxResp_NoServer(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxResp(&TxRespOptions{
		Status:   200,
		NoServer: true,
	})
	if err != nil {
		t.Fatalf("TxResp failed: %v", err)
	}

	written := conn.Written()
	if strings.Contains(written, "Server:") {
		t.Errorf("Expected no Server header with NoServer option")
	}
}

func TestTxResp_NoLen(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxResp(&TxRespOptions{
		Status: 200,
		Body:   []byte("test"),
		NoLen:  true,
	})
	if err != nil {
		t.Fatalf("TxResp failed: %v", err)
	}

	written := conn.Written()
	if strings.Contains(written, "Content-Length:") {
		t.Errorf("Expected no Content-Length header with NoLen option")
	}
}

func TestTxResp_CustomHeaders(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.TxResp(&TxRespOptions{
		Status: 200,
		Headers: map[string]string{
			"X-Custom":     "value",
			"Cache-Control": "no-cache",
		},
	})
	if err != nil {
		t.Fatalf("TxResp failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "X-Custom: value\r\n") {
		t.Errorf("Expected custom header in output")
	}
	if !strings.Contains(written, "Cache-Control: no-cache\r\n") {
		t.Errorf("Expected Cache-Control header in output")
	}
}

// Test Edge Cases and Helper Functions

func TestGenerateBody(t *testing.T) {
	tests := []struct {
		name   string
		length int
		random bool
	}{
		{"Empty", 0, false},
		{"Small", 10, false},
		{"Medium", 100, false},
		{"Large", 1000, false},
		{"Random", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := GenerateBody(tt.length, tt.random)
			if len(body) != tt.length {
				t.Errorf("Expected body length %d, got %d", tt.length, len(body))
			}
			// Check that newlines appear every 64 bytes (at position 63)
			if tt.length > 63 {
				if body[63] != '\n' {
					t.Errorf("Expected newline at position 63, got %c", body[63])
				}
			}
		})
	}
}

func TestGetRequestHeader(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	h.ReqHeaders = []string{
		"Host: example.com",
		"Content-Type: application/json",
		"X-Custom-Header: value",
	}

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"Exact case", "Host", "example.com"},
		{"Lower case", "host", "example.com"},
		{"Upper case", "CONTENT-TYPE", "application/json"},
		{"Mixed case", "x-CuStOm-HeAdEr", "value"},
		{"Not found", "Missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.GetRequestHeader(tt.header)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetResponseHeader(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	h.RespHeaders = []string{
		"Content-Type: text/html",
		"Content-Length: 42",
		"Server: gvtest",
	}

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"Exact case", "Content-Type", "text/html"},
		{"Lower case", "content-length", "42"},
		{"Upper case", "SERVER", "gvtest"},
		{"Not found", "Missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.GetResponseHeader(tt.header)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParseChunkedBody_MultipleChunks(t *testing.T) {
	data := "3\r\n" +
		"foo\r\n" +
		"3\r\n" +
		"bar\r\n" +
		"4\r\n" +
		"baz!\r\n" +
		"0\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body, err := h.ParseChunkedBody()
	if err != nil {
		t.Fatalf("ParseChunkedBody failed: %v", err)
	}

	expected := "foobarbaz!"
	if string(body) != expected {
		t.Errorf("Expected body '%s', got '%s'", expected, string(body))
	}
}

func TestParseChunkedBody_InvalidChunkSize(t *testing.T) {
	data := "INVALID\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	_, err := h.ParseChunkedBody()
	if err == nil {
		t.Fatal("Expected error for invalid chunk size, got nil")
	}
	if !strings.Contains(err.Error(), "invalid chunk size") {
		t.Errorf("Expected 'invalid chunk size' error, got: %v", err)
	}
}

func TestParseChunkedBody_ChunkExtensions(t *testing.T) {
	// Test chunk size with extensions (e.g., "5;name=value")
	data := "5;extension=value\r\n" +
		"Hello\r\n" +
		"0\r\n" +
		"\r\n"

	conn := newMockConn(data)
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body, err := h.ParseChunkedBody()
	if err != nil {
		t.Fatalf("ParseChunkedBody failed: %v", err)
	}

	expected := "Hello"
	if string(body) != expected {
		t.Errorf("Expected body '%s', got '%s'", expected, string(body))
	}
}

func TestCompressDecompress(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	original := []byte("This is a test string that should be compressed and decompressed successfully!")

	// Compress
	compressed, err := h.CompressBody(original)
	if err != nil {
		t.Fatalf("CompressBody failed: %v", err)
	}

	// Verify compressed is smaller (for this size of data)
	if len(compressed) >= len(original) {
		t.Logf("Warning: Compressed size (%d) >= original size (%d)", len(compressed), len(original))
	}

	// Decompress
	decompressed, err := h.DecompressBody(compressed)
	if err != nil {
		t.Fatalf("DecompressBody failed: %v", err)
	}

	// Verify decompressed matches original
	if string(decompressed) != string(original) {
		t.Errorf("Decompressed data doesn't match original.\nExpected: %s\nGot: %s", string(original), string(decompressed))
	}
}

func TestCompressBody_WithGzipLevel(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)
	h.GzipLevel = 9 // Maximum compression

	data := []byte("test data for compression")

	compressed, err := h.CompressBody(data)
	if err != nil {
		t.Fatalf("CompressBody with level 9 failed: %v", err)
	}

	// Verify we can decompress it
	decompressed, err := h.DecompressBody(compressed)
	if err != nil {
		t.Fatalf("DecompressBody failed: %v", err)
	}

	if string(decompressed) != string(data) {
		t.Errorf("Decompressed data doesn't match original")
	}
}

func TestDecompressBody_InvalidData(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	invalidData := []byte("this is not gzipped data")

	_, err := h.DecompressBody(invalidData)
	if err == nil {
		t.Fatal("Expected error for invalid gzip data, got nil")
	}
}

func TestResetRequest(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	// Set some request data
	h.Method = "POST"
	h.URL = "/test"
	h.Proto = "HTTP/1.0"
	h.ReqHeaders = []string{"Host: test.com"}
	h.Body = []byte("test")
	h.BodyLen = 4
	h.HeadMethod = true

	// Reset
	h.ResetRequest()

	// Verify reset
	if h.Method != "" {
		t.Errorf("Expected Method to be empty, got %s", h.Method)
	}
	if h.URL != "" {
		t.Errorf("Expected URL to be empty, got %s", h.URL)
	}
	if h.Proto != "HTTP/1.1" {
		t.Errorf("Expected Proto to be HTTP/1.1, got %s", h.Proto)
	}
	if len(h.ReqHeaders) != 0 {
		t.Errorf("Expected ReqHeaders to be empty, got %d items", len(h.ReqHeaders))
	}
	if h.Body != nil {
		t.Errorf("Expected Body to be nil, got %v", h.Body)
	}
	if h.BodyLen != 0 {
		t.Errorf("Expected BodyLen to be 0, got %d", h.BodyLen)
	}
	if h.HeadMethod {
		t.Error("Expected HeadMethod to be false")
	}
}

func TestResetResponse(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	// Set some response data
	h.Status = 404
	h.Reason = "Not Found"
	h.Proto = "HTTP/1.0"
	h.RespHeaders = []string{"Content-Type: text/html"}
	h.Body = []byte("error")
	h.BodyLen = 5

	// Reset
	h.ResetResponse()

	// Verify reset
	if h.Status != 0 {
		t.Errorf("Expected Status to be 0, got %d", h.Status)
	}
	if h.Reason != "" {
		t.Errorf("Expected Reason to be empty, got %s", h.Reason)
	}
	if h.Proto != "HTTP/1.1" {
		t.Errorf("Expected Proto to be HTTP/1.1, got %s", h.Proto)
	}
	if len(h.RespHeaders) != 0 {
		t.Errorf("Expected RespHeaders to be empty, got %d items", len(h.RespHeaders))
	}
	if h.Body != nil {
		t.Errorf("Expected Body to be nil, got %v", h.Body)
	}
	if h.BodyLen != 0 {
		t.Errorf("Expected BodyLen to be 0, got %d", h.BodyLen)
	}
}

func TestSetTimeout(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	newTimeout := 5 * time.Second
	h.SetTimeout(newTimeout)

	if h.Timeout != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, h.Timeout)
	}
}

func TestClose(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	err := h.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !conn.closed {
		t.Error("Expected connection to be closed")
	}
}

func TestGetDefaultReason(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{503, "Service Unavailable"},
		{999, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.status)), func(t *testing.T) {
			result := getDefaultReason(tt.status)
			if result != tt.expected {
				t.Errorf("For status %d, expected %s, got %s", tt.status, tt.expected, result)
			}
		})
	}
}

func TestReadLine_EOF(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	_, err := h.ReadLine()
	if err == nil {
		t.Fatal("Expected EOF error, got nil")
	}
	if !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "EOF") {
		t.Errorf("Expected EOF error, got %v", err)
	}
}

func TestReadBytes_EOF(t *testing.T) {
	conn := newMockConn("short")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	_, err := h.ReadBytes(100) // Try to read more than available
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestTxReq_GzipCompression(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body := []byte("This is test data that will be compressed with gzip")
	err := h.TxReq(&TxReqOptions{
		Method: "POST",
		URL:    "/api/data",
		Body:   body,
		Gzip:   true,
	})
	if err != nil {
		t.Fatalf("TxReq with Gzip failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "Content-Encoding: gzip\r\n") {
		t.Errorf("Expected Content-Encoding: gzip header in output")
	}

	// Verify body was compressed (stored in h.Body)
	if len(h.Body) >= len(body) {
		t.Logf("Warning: Compressed size (%d) >= original size (%d)", len(h.Body), len(body))
	}
}

func TestTxResp_GzipCompression(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	body := []byte("This is test response data that will be compressed with gzip")
	err := h.TxResp(&TxRespOptions{
		Status: 200,
		Body:   body,
		Gzip:   true,
	})
	if err != nil {
		t.Fatalf("TxResp with Gzip failed: %v", err)
	}

	written := conn.Written()
	if !strings.Contains(written, "Content-Encoding: gzip\r\n") {
		t.Errorf("Expected Content-Encoding: gzip header in output")
	}
}

func TestRxReq_GzipBody(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	// Create gzipped body
	originalBody := []byte("This is the original body content")
	compressedBody, err := h.CompressBody(originalBody)
	if err != nil {
		t.Fatalf("Failed to compress body: %v", err)
	}

	// Prepare request with gzipped body
	data := "POST /api/data HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Content-Encoding: gzip\r\n" +
		"Content-Length: " + strconv.Itoa(len(compressedBody)) + "\r\n" +
		"\r\n" +
		string(compressedBody)

	conn2 := newMockConn(data)
	h2 := New(conn2, logger)

	err = h2.RxReq(&RxReqOptions{})
	if err != nil {
		t.Fatalf("RxReq failed: %v", err)
	}

	// Body should be decompressed automatically
	if string(h2.Body) != string(originalBody) {
		t.Errorf("Expected decompressed body '%s', got '%s'", string(originalBody), string(h2.Body))
	}
}

func TestRxResp_GzipBody(t *testing.T) {
	conn := newMockConn("")
	logger := logging.NewLogger("test")
	h := New(conn, logger)

	// Create gzipped body
	originalBody := []byte("This is the response body")
	compressedBody, err := h.CompressBody(originalBody)
	if err != nil {
		t.Fatalf("Failed to compress body: %v", err)
	}

	// Prepare response with gzipped body
	data := "HTTP/1.1 200 OK\r\n" +
		"Content-Encoding: gzip\r\n" +
		"Content-Length: " + strconv.Itoa(len(compressedBody)) + "\r\n" +
		"\r\n" +
		string(compressedBody)

	conn2 := newMockConn(data)
	h2 := New(conn2, logger)

	err = h2.RxResp(&RxRespOptions{})
	if err != nil {
		t.Fatalf("RxResp failed: %v", err)
	}

	// Body should be decompressed automatically
	if string(h2.Body) != string(originalBody) {
		t.Errorf("Expected decompressed body '%s', got '%s'", string(originalBody), string(h2.Body))
	}
}

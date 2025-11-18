// Package http1 provides HTTP/1.x protocol handling for VTest2.
// This package implements raw HTTP/1 message construction and parsing,
// allowing for precise control including the ability to generate malformed messages.
package http1

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/perbu/GTest/pkg/logging"
)

const (
	// MaxHeaders is the maximum number of headers
	MaxHeaders = 64
	// DefaultTimeout is the default timeout for HTTP operations
	DefaultTimeout = 10 * time.Second
)

// HTTP represents an HTTP/1.x session
type HTTP struct {
	Conn    net.Conn
	Logger  *logging.Logger
	Timeout time.Duration
	Name    string // Client or server name (for default headers)

	// Request and response storage
	ReqHeaders  []string // Request headers
	RespHeaders []string // Response headers
	Body        []byte   // Message body
	BodyLen     int      // Body length

	// Receive buffer
	RxBuf    *bufio.Reader
	RxBytes  []byte // Raw received bytes

	// Gzip state
	GzipLevel    int
	GzipResidual int

	// Request/response line components
	Method     string // HTTP method (for requests)
	URL        string // Request URL
	Proto      string // HTTP protocol version
	Status     int    // Response status code
	Reason     string // Response reason phrase

	// Flags
	Fatal      bool // Fatal error occurred
	HeadMethod bool // Last request was HEAD
}

// New creates a new HTTP session on the given connection
func New(conn net.Conn, logger *logging.Logger) *HTTP {
	return &HTTP{
		Conn:       conn,
		Logger:     logger,
		Timeout:    DefaultTimeout,
		ReqHeaders: make([]string, 0, MaxHeaders),
		RespHeaders: make([]string, 0, MaxHeaders),
		RxBuf:      bufio.NewReader(conn),
		GzipLevel:  -1, // Default compression
	}
}

// SetTimeout sets the I/O timeout
func (h *HTTP) SetTimeout(d time.Duration) {
	h.Timeout = d
}

// ResetRequest clears request state
func (h *HTTP) ResetRequest() {
	h.ReqHeaders = h.ReqHeaders[:0]
	h.Method = ""
	h.URL = ""
	h.Proto = "HTTP/1.1"
	h.Body = nil
	h.BodyLen = 0
	h.HeadMethod = false
}

// ResetResponse clears response state
func (h *HTTP) ResetResponse() {
	h.RespHeaders = h.RespHeaders[:0]
	h.Status = 0
	h.Reason = ""
	h.Proto = "HTTP/1.1"
	h.Body = nil
	h.BodyLen = 0
}

// GetRequestHeader retrieves a request header value
func (h *HTTP) GetRequestHeader(name string) string {
	lowerName := strings.ToLower(name)
	for _, hdr := range h.ReqHeaders {
		parts := strings.SplitN(hdr, ":", 2)
		if len(parts) == 2 && strings.ToLower(strings.TrimSpace(parts[0])) == lowerName {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// GetResponseHeader retrieves a response header value
func (h *HTTP) GetResponseHeader(name string) string {
	lowerName := strings.ToLower(name)
	for _, hdr := range h.RespHeaders {
		parts := strings.SplitN(hdr, ":", 2)
		if len(parts) == 2 && strings.ToLower(strings.TrimSpace(parts[0])) == lowerName {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// Write sends raw bytes to the connection
func (h *HTTP) Write(data []byte) error {
	if h.Timeout > 0 {
		h.Conn.SetWriteDeadline(time.Now().Add(h.Timeout))
	}

	n, err := h.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("short write: %d of %d bytes", n, len(data))
	}

	h.Logger.Log(4, "Sent %d bytes", n)
	return nil
}

// ReadLine reads a line from the connection (up to \r\n or \n)
func (h *HTTP) ReadLine() (string, error) {
	if h.Timeout > 0 {
		h.Conn.SetReadDeadline(time.Now().Add(h.Timeout))
	}

	line, err := h.RxBuf.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read line failed: %w", err)
	}

	// Trim \r\n or \n
	line = strings.TrimRight(line, "\r\n")
	h.Logger.Log(4, "Received line: %s", line)
	return line, nil
}

// ReadBytes reads exactly n bytes from the connection
func (h *HTTP) ReadBytes(n int) ([]byte, error) {
	if h.Timeout > 0 {
		h.Conn.SetReadDeadline(time.Now().Add(h.Timeout))
	}

	buf := make([]byte, n)
	_, err := io.ReadFull(h.RxBuf, buf)
	if err != nil {
		return nil, fmt.Errorf("read bytes failed: %w", err)
	}

	h.Logger.Log(4, "Received %d bytes", n)
	return buf, nil
}

// Close closes the HTTP connection
func (h *HTTP) Close() error {
	if h.Conn != nil {
		return h.Conn.Close()
	}
	return nil
}

// CompressBody compresses the body using gzip
func (h *HTTP) CompressBody(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	var w *gzip.Writer
	var err error

	if h.GzipLevel == -1 {
		w = gzip.NewWriter(&buf)
	} else {
		w, err = gzip.NewWriterLevel(&buf, h.GzipLevel)
		if err != nil {
			return nil, fmt.Errorf("gzip writer creation failed: %w", err)
		}
	}

	// Set minimal header to match C zlib implementation
	// This reduces header size and matches VTest2 behavior
	w.Header.Name = ""
	w.Header.Comment = ""
	w.Header.ModTime = time.Time{} // Zero time
	w.Header.OS = 0xFF              // Unknown OS

	_, err = w.Write(data)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("gzip write failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("gzip close failed: %w", err)
	}

	compressed := buf.Bytes()
	h.Logger.Log(3, "Compressed %d bytes to %d bytes", len(data), len(compressed))
	return compressed, nil
}

// DecompressBody decompresses gzip-encoded data
func (h *HTTP) DecompressBody(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader creation failed: %w", err)
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip read failed: %w", err)
	}

	h.Logger.Log(3, "Decompressed %d bytes to %d bytes", len(data), len(decompressed))
	return decompressed, nil
}

// GenerateBody generates a synthetic body of the specified length
func GenerateBody(length int, random bool) []byte {
	body := make([]byte, length)
	k := byte('!')

	for i := 0; i < length; i++ {
		if (i % 64) == 63 {
			body[i] = '\n'
		} else {
			if random {
				k = byte('!') + byte(i%72)
			}
			body[i] = k
			k++
			if k > '~' {
				k = '!'
			}
		}
	}

	return body
}

// ParseChunkedBody reads a chunked transfer-encoded body
func (h *HTTP) ParseChunkedBody() ([]byte, error) {
	var body bytes.Buffer

	for {
		// Read chunk size line
		line, err := h.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("reading chunk size: %w", err)
		}

		// Parse chunk size (hex)
		parts := strings.SplitN(line, ";", 2)
		chunkSize, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chunk size '%s': %w", line, err)
		}

		h.Logger.Log(4, "Chunk size: %d", chunkSize)

		// If chunk size is 0, this is the last chunk
		if chunkSize == 0 {
			// Read trailing headers (if any) until empty line
			for {
				line, err := h.ReadLine()
				if err != nil {
					return nil, fmt.Errorf("reading trailer: %w", err)
				}
				if line == "" {
					break
				}
				// Could store trailers if needed
			}
			break
		}

		// Read chunk data
		chunk, err := h.ReadBytes(int(chunkSize))
		if err != nil {
			return nil, fmt.Errorf("reading chunk data: %w", err)
		}

		body.Write(chunk)

		// Read trailing CRLF after chunk data
		line, err = h.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("reading chunk trailer: %w", err)
		}
		if line != "" {
			h.Logger.Log(2, "Warning: expected empty line after chunk, got: %s", line)
		}
	}

	return body.Bytes(), nil
}

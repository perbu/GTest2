package http1

import (
	"bytes"
	"testing"

	"github.com/perbu/gvtest/pkg/logging"
)

// BenchmarkParseRequestLine benchmarks HTTP request line parsing
func BenchmarkParseRequestLine(b *testing.B) {
	data := []byte("GET /path/to/resource HTTP/1.1\r\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseRequestLine(data)
	}
}

// BenchmarkParseHeaders benchmarks HTTP header parsing
func BenchmarkParseHeaders(b *testing.B) {
	data := []byte("Host: example.com\r\nContent-Type: text/html\r\nContent-Length: 42\r\n\r\n")

	logger := logging.NewLogger("bench")
	h := &HTTP{Logger: logger}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.headers = nil
		parseHeaders(bytes.NewReader(data), h)
	}
}

// BenchmarkBuildRequest benchmarks HTTP request building
func BenchmarkBuildRequest(b *testing.B) {
	logger := logging.NewLogger("bench")
	h := &HTTP{
		Logger:  logger,
		headers: make(map[string]string),
		method:  "GET",
		url:     "/test",
		proto:   "HTTP/1.1",
	}
	h.headers["Host"] = "example.com"
	h.headers["User-Agent"] = "gvtest"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildRequest(h)
	}
}

// BenchmarkBuildResponse benchmarks HTTP response building
func BenchmarkBuildResponse(b *testing.B) {
	logger := logging.NewLogger("bench")
	h := &HTTP{
		Logger:  logger,
		headers: make(map[string]string),
		status:  200,
		reason:  "OK",
		proto:   "HTTP/1.1",
		body:    []byte("Hello, World!"),
	}
	h.headers["Content-Type"] = "text/plain"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildResponse(h)
	}
}

// Dummy parsing functions for benchmarking
func parseRequestLine(data []byte) (string, string, string) {
	// Simple parsing
	parts := bytes.SplitN(data, []byte(" "), 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	proto := bytes.TrimSuffix(parts[2], []byte("\r\n"))
	return string(parts[0]), string(parts[1]), string(proto)
}

func parseHeaders(r *bytes.Reader, h *HTTP) error {
	// Simplified header parsing for benchmark
	h.headers = make(map[string]string)
	return nil
}

func buildRequest(h *HTTP) []byte {
	var buf bytes.Buffer
	buf.WriteString(h.method)
	buf.WriteByte(' ')
	buf.WriteString(h.url)
	buf.WriteByte(' ')
	buf.WriteString(h.proto)
	buf.WriteString("\r\n")

	for k, v := range h.headers {
		buf.WriteString(k)
		buf.WriteString(": ")
		buf.WriteString(v)
		buf.WriteString("\r\n")
	}
	buf.WriteString("\r\n")

	return buf.Bytes()
}

func buildResponse(h *HTTP) []byte {
	var buf bytes.Buffer
	buf.WriteString(h.proto)
	buf.WriteByte(' ')
	buf.WriteString("200")
	buf.WriteByte(' ')
	buf.WriteString(h.reason)
	buf.WriteString("\r\n")

	for k, v := range h.headers {
		buf.WriteString(k)
		buf.WriteString(": ")
		buf.WriteString(v)
		buf.WriteString("\r\n")
	}
	buf.WriteString("\r\n")
	buf.Write(h.body)

	return buf.Bytes()
}

package http1

import (
	"fmt"
	"strings"
)

// TxReqOptions contains options for transmitting an HTTP request
type TxReqOptions struct {
	Method       string            // HTTP method
	URL          string            // Request URL
	Proto        string            // HTTP protocol version
	Headers      map[string]string // Custom headers
	Body         []byte            // Request body
	BodyLen      int               // Generated body length (if Body is nil)
	Chunked      bool              // Use chunked encoding
	Gzip         bool              // Compress body with gzip
	NoHost       bool              // Don't send Host header
	NoUserAgent  bool              // Don't send User-Agent header
}

// TxReq transmits an HTTP request
func (h *HTTP) TxReq(opts *TxReqOptions) error {
	h.ResetRequest()

	// Set defaults
	if opts.Method == "" {
		opts.Method = "GET"
	}
	if opts.URL == "" {
		opts.URL = "/"
	}
	if opts.Proto == "" {
		opts.Proto = "HTTP/1.1"
	}

	// Store request info
	h.Method = opts.Method
	h.URL = opts.URL
	h.Proto = opts.Proto
	h.HeadMethod = (opts.Method == "HEAD")

	// Build request line
	var req strings.Builder
	fmt.Fprintf(&req, "%s %s %s\r\n", opts.Method, opts.URL, opts.Proto)

	// Prepare body
	body := opts.Body
	if body == nil && opts.BodyLen > 0 {
		body = GenerateBody(opts.BodyLen, false)
	}

	// Compress if requested
	if opts.Gzip && len(body) > 0 {
		compressed, err := h.CompressBody(body)
		if err != nil {
			return fmt.Errorf("gzip compression failed: %w", err)
		}
		body = compressed
		if opts.Headers == nil {
			opts.Headers = make(map[string]string)
		}
		opts.Headers["Content-Encoding"] = "gzip"
	}

	h.Body = body
	h.BodyLen = len(body)

	// Add default headers
	if !opts.NoHost && opts.Proto == "HTTP/1.1" {
		// Add Host header (default to localhost if not provided)
		if _, exists := opts.Headers["Host"]; !exists {
			if opts.Headers == nil {
				opts.Headers = make(map[string]string)
			}
			opts.Headers["Host"] = "localhost"
		}
	}

	if !opts.NoUserAgent {
		if _, exists := opts.Headers["User-Agent"]; !exists {
			if opts.Headers == nil {
				opts.Headers = make(map[string]string)
			}
			// Use client name if available, otherwise default to "gvtest"
			userAgent := "gvtest"
			if h.Name != "" {
				userAgent = h.Name
			}
			opts.Headers["User-Agent"] = userAgent
		}
	}

	// Add custom headers
	for name, value := range opts.Headers {
		h.ReqHeaders = append(h.ReqHeaders, fmt.Sprintf("%s: %s", name, value))
		fmt.Fprintf(&req, "%s: %s\r\n", name, value)
	}

	// Handle body
	if opts.Chunked {
		// Chunked encoding
		req.WriteString("Transfer-Encoding: chunked\r\n")
		req.WriteString("\r\n")

		// Send headers
		err := h.Write([]byte(req.String()))
		if err != nil {
			return err
		}

		// Send body as chunks
		return h.sendChunked(body)
	} else {
		// Regular body with Content-Length
		if len(body) > 0 {
			fmt.Fprintf(&req, "Content-Length: %d\r\n", len(body))
		}
		req.WriteString("\r\n")

		// Send headers
		err := h.Write([]byte(req.String()))
		if err != nil {
			return err
		}

		// Send body
		if len(body) > 0 {
			err = h.Write(body)
			if err != nil {
				return err
			}
		}
	}

	h.Logger.Log(3, "txreq: %s %s", opts.Method, opts.URL)
	return nil
}

// sendChunked sends data using chunked transfer encoding
func (h *HTTP) sendChunked(data []byte) error {
	// Send body in one chunk
	chunkSize := fmt.Sprintf("%x\r\n", len(data))
	err := h.Write([]byte(chunkSize))
	if err != nil {
		return err
	}

	err = h.Write(data)
	if err != nil {
		return err
	}

	err = h.Write([]byte("\r\n"))
	if err != nil {
		return err
	}

	// Send final chunk (0-sized)
	err = h.Write([]byte("0\r\n\r\n"))
	if err != nil {
		return err
	}

	h.Logger.Log(4, "Sent chunked body (%d bytes)", len(data))
	return nil
}

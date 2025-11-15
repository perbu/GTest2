package http1

import (
	"fmt"
	"strings"
)

// TxRespOptions contains options for transmitting an HTTP response
type TxRespOptions struct {
	Status    int               // HTTP status code
	Reason    string            // Reason phrase
	Proto     string            // HTTP protocol version
	Headers   map[string]string // Custom headers
	Body      []byte            // Response body
	BodyLen   int               // Generated body length (if Body is nil)
	Chunked   bool              // Use chunked encoding
	Gzip      bool              // Compress body with gzip
	NoLen     bool              // Don't send Content-Length
	NoServer  bool              // Don't send Server header
}

// TxResp transmits an HTTP response
func (h *HTTP) TxResp(opts *TxRespOptions) error {
	h.ResetResponse()

	// Set defaults
	if opts.Status == 0 {
		opts.Status = 200
	}
	if opts.Reason == "" {
		opts.Reason = getDefaultReason(opts.Status)
	}
	if opts.Proto == "" {
		opts.Proto = "HTTP/1.1"
	}

	// Store response info
	h.Status = opts.Status
	h.Reason = opts.Reason
	h.Proto = opts.Proto

	// Build response line
	var resp strings.Builder
	fmt.Fprintf(&resp, "%s %d %s\r\n", opts.Proto, opts.Status, opts.Reason)

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

	// Add default Server header
	if !opts.NoServer {
		if _, exists := opts.Headers["Server"]; !exists {
			if opts.Headers == nil {
				opts.Headers = make(map[string]string)
			}
			opts.Headers["Server"] = "gvtest"
		}
	}

	// Add custom headers
	for name, value := range opts.Headers {
		h.RespHeaders = append(h.RespHeaders, fmt.Sprintf("%s: %s", name, value))
		fmt.Fprintf(&resp, "%s: %s\r\n", name, value)
	}

	// Handle body
	if opts.Chunked {
		// Chunked encoding
		resp.WriteString("Transfer-Encoding: chunked\r\n")
		resp.WriteString("\r\n")

		// Send headers
		err := h.Write([]byte(resp.String()))
		if err != nil {
			return err
		}

		// Send body as chunks
		return h.sendChunked(body)
	} else {
		// Regular body with Content-Length (unless NoLen is set)
		if !opts.NoLen {
			fmt.Fprintf(&resp, "Content-Length: %d\r\n", len(body))
		}
		resp.WriteString("\r\n")

		// Send headers
		err := h.Write([]byte(resp.String()))
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

	h.Logger.Log(3, "txresp: %d %s", opts.Status, opts.Reason)
	return nil
}

// getDefaultReason returns the default reason phrase for a status code
func getDefaultReason(status int) string {
	reasons := map[int]string{
		100: "Continue",
		101: "Switching Protocols",
		200: "OK",
		201: "Created",
		202: "Accepted",
		204: "No Content",
		206: "Partial Content",
		301: "Moved Permanently",
		302: "Found",
		304: "Not Modified",
		307: "Temporary Redirect",
		308: "Permanent Redirect",
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		408: "Request Timeout",
		500: "Internal Server Error",
		501: "Not Implemented",
		502: "Bad Gateway",
		503: "Service Unavailable",
		504: "Gateway Timeout",
	}

	if reason, ok := reasons[status]; ok {
		return reason
	}
	return "Unknown"
}

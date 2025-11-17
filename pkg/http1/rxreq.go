package http1

import (
	"fmt"
	"strconv"
	"strings"
)

// RxReqOptions contains options for receiving an HTTP request
type RxReqOptions struct {
	// Currently no options, but we keep this for future extensibility
}

// RxReq receives and parses an HTTP request
func (h *HTTP) RxReq(opts *RxReqOptions) error {
	h.ResetRequest()

	// Read request line
	line, err := h.ReadLine()
	if err != nil {
		return fmt.Errorf("reading request line: %w", err)
	}

	// Parse request line: METHOD URL PROTO
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid request line: %s", line)
	}

	h.Method = parts[0]
	h.URL = parts[1]
	h.Proto = parts[2]
	h.HeadMethod = (h.Method == "HEAD")

	h.Logger.Log(3, "rxreq: %s %s", h.Method, h.URL)

	// Read headers
	err = h.readHeaders(true)
	if err != nil {
		return fmt.Errorf("reading headers: %w", err)
	}

	// Read body if present
	err = h.readBody(true)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}

	h.Logger.Log(4, "bodylen = %d", h.BodyLen)
	return nil
}

// readHeaders reads HTTP headers (common for requests and responses)
func (h *HTTP) readHeaders(isRequest bool) error {
	var headers *[]string
	if isRequest {
		headers = &h.ReqHeaders
	} else {
		headers = &h.RespHeaders
	}

	for {
		line, err := h.ReadLine()
		if err != nil {
			return err
		}

		// Empty line marks end of headers
		if line == "" {
			break
		}

		*headers = append(*headers, line)
		h.Logger.Log(4, "Header: %s", line)
	}

	return nil
}

// readBody reads the HTTP body based on Content-Length or chunked encoding
func (h *HTTP) readBody(isRequest bool) error {
	var contentLength int
	var chunked bool
	var header string

	// Get Content-Length and Transfer-Encoding
	if isRequest {
		header = h.GetRequestHeader("Content-Length")
	} else {
		header = h.GetResponseHeader("Content-Length")
	}

	if header != "" {
		cl, err := strconv.Atoi(header)
		if err != nil {
			return fmt.Errorf("invalid Content-Length: %s", header)
		}
		contentLength = cl
	}

	// Check for chunked encoding
	var te string
	if isRequest {
		te = h.GetRequestHeader("Transfer-Encoding")
	} else {
		te = h.GetResponseHeader("Transfer-Encoding")
	}

	chunked = strings.Contains(strings.ToLower(te), "chunked")

	// Read body
	var body []byte
	var err error

	if chunked {
		// Read chunked body
		body, err = h.ParseChunkedBody()
		if err != nil {
			return fmt.Errorf("reading chunked body: %w", err)
		}
	} else if contentLength > 0 {
		// Read fixed-length body
		body, err = h.ReadBytes(contentLength)
		if err != nil {
			return fmt.Errorf("reading body: %w", err)
		}
	}

	// Store the body as-is (don't auto-decompress)
	// VTC tests expect manual decompression via the 'gunzip' command
	h.Body = body
	h.BodyLen = len(body)
	return nil
}

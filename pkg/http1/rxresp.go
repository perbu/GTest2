package http1

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// RxRespOptions contains options for receiving an HTTP response
type RxRespOptions struct {
	NoObj bool // Don't read the body
}

// RxRespBodyOptions contains options for receiving an HTTP response body
type RxRespBodyOptions struct {
	MaxBytes int // Maximum bytes to read (0 = read all remaining)
}

// RxRespHdrs receives and parses HTTP response headers only (not the body)
func (h *HTTP) RxRespHdrs() error {
	// Read status line
	line, err := h.ReadLine()
	if err != nil {
		return fmt.Errorf("reading status line: %w", err)
	}

	// Parse status line: PROTO STATUS REASON
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid status line: %s", line)
	}

	h.Proto = parts[0]
	status, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid status code: %s", parts[1])
	}
	h.Status = status

	if len(parts) == 3 {
		h.Reason = parts[2]
	} else {
		h.Reason = ""
	}

	h.Logger.Log(3, "rxresphdrs: %d %s", h.Status, h.Reason)

	// Read headers
	err = h.readHeaders(false)
	if err != nil {
		return fmt.Errorf("reading headers: %w", err)
	}

	// Initialize body state (will be filled by rxrespbody)
	h.Body = nil
	h.BodyLen = 0

	return nil
}

// RxRespBody reads the response body (or part of it)
func (h *HTTP) RxRespBody(opts *RxRespBodyOptions) error {
	var maxBytes int
	if opts != nil {
		maxBytes = opts.MaxBytes
	}

	// Get Content-Length and Transfer-Encoding
	contentLength := 0
	header := h.GetResponseHeader("Content-Length")
	if header != "" {
		cl, err := strconv.Atoi(header)
		if err != nil {
			return fmt.Errorf("invalid Content-Length: %s", header)
		}
		contentLength = cl
	}

	te := h.GetResponseHeader("Transfer-Encoding")
	chunked := strings.Contains(strings.ToLower(te), "chunked")

	// Read body
	var newData []byte
	var err error

	if chunked {
		// For chunked encoding, read chunks until we have enough or hit the end
		if maxBytes > 0 {
			// Read partial chunked body
			newData, err = h.readPartialChunkedBody(maxBytes)
			if err != nil && err != io.EOF {
				return fmt.Errorf("reading chunked body: %w", err)
			}
		} else {
			// Read all remaining chunked body
			newData, err = h.ParseChunkedBody()
			if err != nil {
				return fmt.Errorf("reading chunked body: %w", err)
			}
		}
	} else if contentLength > 0 {
		// For fixed-length body
		remaining := contentLength - h.BodyLen
		if maxBytes > 0 && maxBytes < remaining {
			// Read partial body
			newData, err = h.ReadBytes(maxBytes)
			if err != nil {
				return fmt.Errorf("reading body: %w", err)
			}
		} else {
			// Read all remaining body
			newData, err = h.ReadBytes(remaining)
			if err != nil {
				return fmt.Errorf("reading body: %w", err)
			}
		}
	} else if maxBytes > 0 {
		// No content-length, read up to maxBytes
		newData, err = h.ReadBytes(maxBytes)
		if err != nil && err != io.EOF {
			return fmt.Errorf("reading body: %w", err)
		}
	} else {
		// No content-length and no max, read until EOF
		buf := make([]byte, 8192)
		for {
			n, err := h.RxBuf.Read(buf)
			if n > 0 {
				newData = append(newData, buf[:n]...)
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("reading body: %w", err)
			}
		}
	}

	// Append to existing body
	if h.Body == nil {
		h.Body = newData
	} else {
		h.Body = append(h.Body, newData...)
	}
	h.BodyLen = len(h.Body)

	h.Logger.Log(4, "rxrespbody: read %d bytes, total bodylen = %d", len(newData), h.BodyLen)
	return nil
}

// readPartialChunkedBody reads chunked body data up to maxBytes
func (h *HTTP) readPartialChunkedBody(maxBytes int) ([]byte, error) {
	var result []byte
	totalRead := 0

	for totalRead < maxBytes {
		// Read chunk size line
		line, err := h.ReadLine()
		if err != nil {
			return result, fmt.Errorf("reading chunk size: %w", err)
		}

		// Parse chunk size (ignore chunk extensions after ';')
		sizePart := strings.Split(line, ";")[0]
		sizePart = strings.TrimSpace(sizePart)
		size, err := strconv.ParseInt(sizePart, 16, 64)
		if err != nil {
			return result, fmt.Errorf("invalid chunk size: %s", line)
		}

		if size == 0 {
			// Last chunk
			// Read trailer headers (if any)
			for {
				line, err := h.ReadLine()
				if err != nil {
					return result, fmt.Errorf("reading trailer: %w", err)
				}
				if line == "" {
					break
				}
			}
			return result, io.EOF
		}

		// Read this chunk's data
		toRead := int(size)
		if totalRead+toRead > maxBytes {
			// Can only read part of this chunk
			toRead = maxBytes - totalRead
		}

		data, err := h.ReadBytes(toRead)
		if err != nil {
			return result, fmt.Errorf("reading chunk data: %w", err)
		}
		result = append(result, data...)
		totalRead += toRead

		if toRead < int(size) {
			// We didn't read the full chunk, need to skip the rest
			remaining := int(size) - toRead
			_, err := h.ReadBytes(remaining)
			if err != nil {
				return result, fmt.Errorf("skipping chunk data: %w", err)
			}
		}

		// Read trailing CRLF after chunk data
		_, err = h.ReadLine()
		if err != nil {
			return result, fmt.Errorf("reading chunk trailer: %w", err)
		}

		if totalRead >= maxBytes {
			break
		}
	}

	return result, nil
}


// RxResp receives and parses an HTTP response
func (h *HTTP) RxResp(opts *RxRespOptions) error {
	h.ResetResponse()

	// Read status line
	line, err := h.ReadLine()
	if err != nil {
		return fmt.Errorf("reading status line: %w", err)
	}

	// Parse status line: PROTO STATUS REASON
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid status line: %s", line)
	}

	h.Proto = parts[0]
	status, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid status code: %s", parts[1])
	}
	h.Status = status

	if len(parts) == 3 {
		h.Reason = parts[2]
	} else {
		h.Reason = ""
	}

	h.Logger.Log(3, "rxresp: %d %s", h.Status, h.Reason)

	// Read headers
	err = h.readHeaders(false)
	if err != nil {
		return fmt.Errorf("reading headers: %w", err)
	}

	// Read body if requested and conditions are met
	if !opts.NoObj && !h.HeadMethod {
		// Check if we should read a body
		// For 1xx, 204, 304, don't read body
		if h.Status < 200 || h.Status == 204 || h.Status == 304 {
			h.Logger.Log(4, "No body expected for status %d", h.Status)
		} else {
			err = h.readBody(false)
			if err != nil {
				return fmt.Errorf("reading body: %w", err)
			}
		}
	}

	h.Logger.Log(4, "bodylen = %d", h.BodyLen)
	return nil
}

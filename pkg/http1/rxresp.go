package http1

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RxRespOptions contains options for receiving an HTTP response
type RxRespOptions struct {
	NoObj bool // Don't read the body
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

// RxRespHdrs receives only the response headers (not the body)
func (h *HTTP) RxRespHdrs() error {
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

	h.Logger.Log(3, "rxresphdrs: %d %s", h.Status, h.Reason)

	// Read headers only
	err = h.readHeaders(false)
	if err != nil {
		return fmt.Errorf("reading headers: %w", err)
	}

	return nil
}

// RxRespBody receives the response body (or part of it)
// maxBytes: maximum bytes to read, -1 for all remaining
// This can be called multiple times to incrementally read the body
func (h *HTTP) RxRespBody(maxBytes int) error {
	// Don't reset response state - we're continuing from rxresphdrs

	// Check if we should read a body based on status
	if h.Status < 200 || h.Status == 204 || h.Status == 304 || h.HeadMethod {
		h.Logger.Log(4, "No body expected for status %d", h.Status)
		return nil
	}

	// Determine how many bytes to read
	var contentLength int
	var chunked bool

	// Get Content-Length
	clHeader := h.GetResponseHeader("Content-Length")
	if clHeader != "" {
		cl, err := strconv.Atoi(clHeader)
		if err != nil {
			return fmt.Errorf("invalid Content-Length: %s", clHeader)
		}
		contentLength = cl
	}

	// Check for chunked encoding
	te := h.GetResponseHeader("Transfer-Encoding")
	chunked = strings.Contains(strings.ToLower(te), "chunked")

	// Read body
	var newBody []byte
	var err error

	if chunked {
		// For chunked encoding, if maxBytes is specified, we need to read chunks until we hit the limit
		if maxBytes > 0 {
			// Read chunks until we have maxBytes or hit the end
			bytesRead := h.BodyLen
			for bytesRead < maxBytes {
				// Read chunk size line
				line, err := h.ReadLine()
				if err != nil {
					return fmt.Errorf("reading chunk size: %w", err)
				}

				// Parse chunk size (hex)
				chunkParts := strings.SplitN(line, ";", 2)
				chunkSize, err := strconv.ParseInt(strings.TrimSpace(chunkParts[0]), 16, 64)
				if err != nil {
					return fmt.Errorf("invalid chunk size '%s': %w", line, err)
				}

				// If chunk size is 0, we're done
				if chunkSize == 0 {
					// Read trailing headers until empty line
					for {
						line, err := h.ReadLine()
						if err != nil {
							return fmt.Errorf("reading trailer: %w", err)
						}
						if line == "" {
							break
						}
					}
					break
				}

				// Read chunk data
				chunk, err := h.ReadBytes(int(chunkSize))
				if err != nil {
					return fmt.Errorf("reading chunk data: %w", err)
				}

				newBody = append(newBody, chunk...)
				bytesRead += len(chunk)

				// Read trailing CRLF
				_, err = h.ReadLine()
				if err != nil {
					return fmt.Errorf("reading chunk trailer: %w", err)
				}

				if bytesRead >= maxBytes {
					break
				}
			}
		} else {
			// Read all remaining chunks
			remainingBody, err := h.ParseChunkedBody()
			if err != nil {
				return fmt.Errorf("reading chunked body: %w", err)
			}
			newBody = remainingBody
		}
	} else if contentLength > 0 {
		// Fixed-length body
		bytesToRead := contentLength - h.BodyLen
		if maxBytes > 0 && bytesToRead > maxBytes {
			bytesToRead = maxBytes
		}

		if bytesToRead > 0 {
			newBody, err = h.ReadBytes(bytesToRead)
			if err != nil {
				return fmt.Errorf("reading body: %w", err)
			}
		}
	}

	// Append new body to existing body
	h.Body = append(h.Body, newBody...)
	h.BodyLen = len(h.Body)

	h.Logger.Log(3, "rxrespbody: read %d bytes, total bodylen = %d", len(newBody), h.BodyLen)
	return nil
}

// WriteBody writes the current body to a file
func (h *HTTP) WriteBody(filename string) error {
	if h.Body == nil {
		return fmt.Errorf("no body to write")
	}

	err := os.WriteFile(filename, h.Body, 0644)
	if err != nil {
		return fmt.Errorf("failed to write body to %s: %w", filename, err)
	}

	h.Logger.Log(3, "write_body: wrote %d bytes to %s", h.BodyLen, filename)
	return nil
}

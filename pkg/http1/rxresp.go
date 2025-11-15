package http1

import (
	"fmt"
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

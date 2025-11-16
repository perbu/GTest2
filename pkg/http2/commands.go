package http2

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/perbu/gvtest/pkg/hpack"
)

// TxReqOptions represents options for sending an HTTP/2 request
type TxReqOptions struct {
	Method    string
	Path      string
	Scheme    string
	Authority string
	Headers   map[string]string
	Body      []byte
	EndStream bool
}

// TxReq sends an HTTP/2 request on a stream
func (c *Conn) TxReq(streamID uint32, opts TxReqOptions) error {
	stream := c.streams.GetOrCreate(streamID, fmt.Sprintf("stream-%d", streamID))

	// Build headers with pseudo-headers first
	headers := []hpack.HeaderField{
		{Name: ":method", Value: opts.Method},
		{Name: ":path", Value: opts.Path},
		{Name: ":scheme", Value: opts.Scheme},
		{Name: ":authority", Value: opts.Authority},
	}

	// Add regular headers
	for name, value := range opts.Headers {
		headers = append(headers, hpack.HeaderField{Name: name, Value: value})
	}

	// Encode headers using HPACK (must be serialized)
	c.encoderMu.Lock()
	headerBlock, err := c.encoder.Encode(headers)
	c.encoderMu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to encode headers: %w", err)
	}

	// Store headers in stream
	for _, hf := range headers {
		stream.AddReqHeader(hf.Name, hf.Value)
	}

	// Determine if we should set END_STREAM
	endStream := opts.EndStream || len(opts.Body) == 0

	// Send HEADERS frame
	err = WriteHeadersFrame(c.conn, streamID, headerBlock, endStream, true)
	if err != nil {
		return fmt.Errorf("failed to write HEADERS frame: %w", err)
	}

	stream.UpdateState(endStream, true)
	c.logger.Log(3, "Sent HEADERS on stream %d (END_STREAM=%v)", streamID, endStream)

	// Send DATA frame if there's a body and we haven't set END_STREAM yet
	if len(opts.Body) > 0 && !endStream {
		err = WriteDataFrame(c.conn, streamID, opts.Body, opts.EndStream)
		if err != nil {
			return fmt.Errorf("failed to write DATA frame: %w", err)
		}

		stream.AppendReqBody(opts.Body)
		stream.UpdateState(opts.EndStream, true)
		c.logger.Log(3, "Sent DATA on stream %d: %d bytes (END_STREAM=%v)",
			streamID, len(opts.Body), opts.EndStream)
	}

	return nil
}

// TxRespOptions represents options for sending an HTTP/2 response
type TxRespOptions struct {
	Status    string
	Headers   map[string]string
	Body      []byte
	EndStream bool
}

// TxResp sends an HTTP/2 response on a stream
func (c *Conn) TxResp(streamID uint32, opts TxRespOptions) error {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	// Build headers with :status pseudo-header first
	headers := []hpack.HeaderField{
		{Name: ":status", Value: opts.Status},
	}

	// Add regular headers
	for name, value := range opts.Headers {
		headers = append(headers, hpack.HeaderField{Name: name, Value: value})
	}

	// Encode headers using HPACK (must be serialized)
	c.encoderMu.Lock()
	headerBlock, err := c.encoder.Encode(headers)
	c.encoderMu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to encode headers: %w", err)
	}

	// Store headers in stream
	for _, hf := range headers {
		stream.AddRespHeader(hf.Name, hf.Value)
	}

	// Determine if we should set END_STREAM
	endStream := opts.EndStream || len(opts.Body) == 0

	// Send HEADERS frame
	err = WriteHeadersFrame(c.conn, streamID, headerBlock, endStream, true)
	if err != nil {
		return fmt.Errorf("failed to write HEADERS frame: %w", err)
	}

	stream.UpdateState(endStream, true)
	c.logger.Log(3, "Sent HEADERS on stream %d (END_STREAM=%v)", streamID, endStream)

	// Send DATA frame if there's a body and we haven't set END_STREAM yet
	if len(opts.Body) > 0 && !endStream {
		err = WriteDataFrame(c.conn, streamID, opts.Body, opts.EndStream)
		if err != nil {
			return fmt.Errorf("failed to write DATA frame: %w", err)
		}

		stream.AppendRespBody(opts.Body)
		stream.UpdateState(opts.EndStream, true)
		c.logger.Log(3, "Sent DATA on stream %d: %d bytes (END_STREAM=%v)",
			streamID, len(opts.Body), opts.EndStream)
	}

	return nil
}

// RxReq receives an HTTP/2 request on a stream
func (c *Conn) RxReq(streamID uint32) error {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	// Wait for the request (headers and potentially body)
	// The frame receive loop will populate the stream
	stream.Wait()

	c.logger.Log(3, "Received request on stream %d: %s %s",
		streamID, stream.Method, stream.Path)

	return nil
}

// RxResp receives an HTTP/2 response on a stream
func (c *Conn) RxResp(streamID uint32) error {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	// Wait for the response
	stream.Wait()

	c.logger.Log(3, "Received response on stream %d: status %s",
		streamID, stream.Status)

	return nil
}

// TxData sends a DATA frame on a stream
func (c *Conn) TxData(streamID uint32, data []byte, endStream bool) error {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	err := WriteDataFrame(c.conn, streamID, data, endStream)
	if err != nil {
		return err
	}

	stream.AppendReqBody(data)
	stream.UpdateState(endStream, true)

	c.logger.Log(3, "Sent DATA on stream %d: %d bytes (END_STREAM=%v)",
		streamID, len(data), endStream)

	return nil
}

// RxData waits to receive a DATA frame on a stream
func (c *Conn) RxData(streamID uint32) ([]byte, error) {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return nil, fmt.Errorf("stream %d not found", streamID)
	}

	// Wait for data
	stream.Wait()

	c.logger.Log(3, "Received DATA on stream %d: %d bytes",
		streamID, len(stream.ReqBody))

	return stream.ReqBody, nil
}

// Expect performs assertions on stream data
func (c *Conn) Expect(streamID uint32, field, op, expected string) error {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()

	// Extract the actual value based on field
	var actual string
	parts := strings.Split(field, ".")

	if len(parts) < 2 {
		return fmt.Errorf("invalid field format: %s", field)
	}

	reqOrResp := parts[0]
	fieldName := strings.Join(parts[1:], ".")

	switch reqOrResp {
	case "req":
		actual = c.getReqField(stream, fieldName)
	case "resp":
		actual = c.getRespField(stream, fieldName)
	default:
		return fmt.Errorf("invalid field prefix: %s (must be 'req' or 'resp')", reqOrResp)
	}

	// Perform comparison
	return c.compare(actual, op, expected, field)
}

// getReqField extracts request field values
func (c *Conn) getReqField(stream *Stream, field string) string {
	switch field {
	case "method":
		return stream.Method
	case "path":
		return stream.Path
	case "scheme":
		return stream.Scheme
	case "authority":
		return stream.Authority
	case "body":
		return string(stream.ReqBody)
	case "bodylen":
		return strconv.Itoa(len(stream.ReqBody))
	default:
		// Check if it's a header
		if strings.HasPrefix(field, "http.") {
			headerName := strings.TrimPrefix(field, "http.")
			return stream.GetHeader(stream.ReqHeaders, headerName)
		}
	}
	return ""
}

// getRespField extracts response field values
func (c *Conn) getRespField(stream *Stream, field string) string {
	switch field {
	case "status":
		return stream.Status
	case "body":
		return string(stream.RespBody)
	case "bodylen":
		return strconv.Itoa(len(stream.RespBody))
	default:
		// Check if it's a header
		if strings.HasPrefix(field, "http.") {
			headerName := strings.TrimPrefix(field, "http.")
			return stream.GetHeader(stream.RespHeaders, headerName)
		}
	}
	return ""
}

// compare performs the comparison operation
func (c *Conn) compare(actual, op, expected, field string) error {
	switch op {
	case "==":
		if actual != expected {
			return fmt.Errorf("expect %s == %q failed: got %q", field, expected, actual)
		}
	case "!=":
		if actual == expected {
			return fmt.Errorf("expect %s != %q failed: got %q", field, expected, actual)
		}
	case "~":
		// Simple substring match (simplified regex)
		if !strings.Contains(actual, expected) {
			return fmt.Errorf("expect %s ~ %q failed: got %q", field, expected, actual)
		}
	case "!~":
		if strings.Contains(actual, expected) {
			return fmt.Errorf("expect %s !~ %q failed: got %q", field, expected, actual)
		}
	default:
		// Numeric comparisons
		actualInt, err1 := strconv.Atoi(actual)
		expectedInt, err2 := strconv.Atoi(expected)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid numeric comparison for %s: %s %s %s", field, actual, op, expected)
		}

		switch op {
		case "<":
			if !(actualInt < expectedInt) {
				return fmt.Errorf("expect %s < %s failed: got %d", field, expected, actualInt)
			}
		case ">":
			if !(actualInt > expectedInt) {
				return fmt.Errorf("expect %s > %s failed: got %d", field, expected, actualInt)
			}
		case "<=":
			if !(actualInt <= expectedInt) {
				return fmt.Errorf("expect %s <= %s failed: got %d", field, expected, actualInt)
			}
		case ">=":
			if !(actualInt >= expectedInt) {
				return fmt.Errorf("expect %s >= %s failed: got %d", field, expected, actualInt)
			}
		default:
			return fmt.Errorf("unknown operator: %s", op)
		}
	}

	c.logger.Log(3, "Expect passed: %s %s %s", field, op, expected)
	return nil
}

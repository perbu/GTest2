package http1

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Send sends raw bytes to the connection
func (h *HTTP) Send(data []byte) error {
	return h.Write(data)
}

// SendString sends a string to the connection
func (h *HTTP) SendString(s string) error {
	return h.Write([]byte(s))
}

// SendHex sends hex-encoded bytes to the connection
// hex string can have spaces and newlines which are ignored
func (h *HTTP) SendHex(hexStr string) error {
	// Remove spaces and newlines
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ReplaceAll(hexStr, "\n", "")
	hexStr = strings.ReplaceAll(hexStr, "\t", "")

	// Decode hex
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Errorf("invalid hex string: %w", err)
	}

	return h.Write(data)
}

// Recv receives a specified number of bytes from the connection
func (h *HTTP) Recv(n int) ([]byte, error) {
	return h.ReadBytes(n)
}

// SetIOTimeout sets the I/O timeout for subsequent operations
func (h *HTTP) SetIOTimeout(d time.Duration) {
	h.SetTimeout(d)
}

// Gunzip decompresses the body in place
func (h *HTTP) Gunzip() error {
	if len(h.Body) == 0 {
		return nil
	}

	decompressed, err := h.DecompressBody(h.Body)
	if err != nil {
		return fmt.Errorf("gunzip failed: %w", err)
	}

	h.Body = decompressed
	h.BodyLen = len(decompressed)
	h.Logger.Log(3, "gunzip: decompressed to %d bytes", h.BodyLen)
	return nil
}

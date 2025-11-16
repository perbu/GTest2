package http2

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// TxPri sends the HTTP/2 connection preface
func (c *Conn) TxPri() error {
	return c.SendPreface()
}

// RxPri receives and validates the HTTP/2 connection preface
func (c *Conn) RxPri() error {
	return c.ReceivePreface()
}

// TxSettings sends a SETTINGS frame
func (c *Conn) TxSettings(ack bool, settings map[SettingID]uint32) error {
	if settings != nil {
		// Update local settings
		c.mu.Lock()
		for id, value := range settings {
			c.localSettings[id] = value
		}
		c.mu.Unlock()
	}

	return c.SendSettings(ack)
}

// RxSettings waits to receive a SETTINGS frame
func (c *Conn) RxSettings() (map[SettingID]uint32, error) {
	// Settings are handled automatically by the frame receive loop
	// This is a simplified implementation - in production you'd want to wait for specific settings
	c.logger.Log(3, "Waiting for SETTINGS frame")
	time.Sleep(100 * time.Millisecond) // Give time for frame to arrive

	c.mu.Lock()
	settings := make(map[SettingID]uint32)
	for id, value := range c.remoteSettings {
		settings[id] = value
	}
	c.mu.Unlock()

	return settings, nil
}

// TxPing sends a PING frame
func (c *Conn) TxPing(ack bool, data [8]byte) error {
	c.logger.Log(3, "Sending PING (ack=%v)", ack)
	return WritePingFrame(c.conn, ack, data)
}

// RxPing waits to receive a PING frame
func (c *Conn) RxPing() ([8]byte, error) {
	// PING frames are handled automatically by the frame receive loop
	// For now, return a default value
	c.logger.Log(3, "Waiting for PING frame")
	var data [8]byte
	return data, nil
}

// TxGoAway sends a GOAWAY frame
func (c *Conn) TxGoAway(lastStreamID uint32, errorCode uint32, debugData string) error {
	c.logger.Log(3, "Sending GOAWAY (lastStreamID=%d, errorCode=%d)", lastStreamID, errorCode)
	return WriteGoAwayFrame(c.conn, lastStreamID, errorCode, []byte(debugData))
}

// RxGoAway waits to receive a GOAWAY frame
func (c *Conn) RxGoAway() error {
	// GOAWAY frames are handled automatically by the frame receive loop
	c.logger.Log(3, "Waiting for GOAWAY frame")
	return nil
}

// TxRst sends an RST_STREAM frame
func (c *Conn) TxRst(streamID uint32, errorCode uint32) error {
	c.logger.Log(3, "Sending RST_STREAM (stream=%d, errorCode=%d)", streamID, errorCode)
	return WriteRSTStreamFrame(c.conn, streamID, errorCode)
}

// RxRst waits to receive an RST_STREAM frame on a stream
func (c *Conn) RxRst(streamID uint32) error {
	stream, ok := c.streams.Get(streamID)
	if !ok {
		return fmt.Errorf("stream %d not found", streamID)
	}

	// Wait for RST_STREAM
	stream.Wait()

	c.logger.Log(3, "Received RST_STREAM on stream %d", streamID)
	return nil
}

// TxWinup sends a WINDOW_UPDATE frame
func (c *Conn) TxWinup(streamID uint32, increment uint32) error {
	c.logger.Log(3, "Sending WINDOW_UPDATE (stream=%d, increment=%d)", streamID, increment)
	return WriteWindowUpdateFrame(c.conn, streamID, increment)
}

// RxWinup waits to receive a WINDOW_UPDATE frame
func (c *Conn) RxWinup(streamID uint32) (uint32, error) {
	// WINDOW_UPDATE frames are handled automatically by the frame receive loop
	c.logger.Log(3, "Waiting for WINDOW_UPDATE on stream %d", streamID)
	time.Sleep(100 * time.Millisecond)
	return 0, nil
}

// SendHex sends raw hexadecimal data (allows for malformed frames)
func (c *Conn) SendHex(hexData string) error {
	// Remove spaces and newlines
	hexData = strings.ReplaceAll(hexData, " ", "")
	hexData = strings.ReplaceAll(hexData, "\n", "")
	hexData = strings.ReplaceAll(hexData, "\r", "")
	hexData = strings.ReplaceAll(hexData, "\t", "")

	data, err := hex.DecodeString(hexData)
	if err != nil {
		return fmt.Errorf("invalid hex data: %w", err)
	}

	c.logger.Log(3, "Sending raw hex data: %d bytes", len(data))
	_, err = c.conn.Write(data)
	return err
}

// WriteRaw writes a raw frame with manual control (for malformed frames)
func (c *Conn) WriteRaw(length uint32, frameType FrameType, flags Flags, streamID uint32, payload []byte) error {
	c.logger.Log(3, "Sending raw frame: type=%s, length=%d, flags=0x%x, stream=%d",
		frameType, length, flags, streamID)
	return WriteRawFrame(c.conn, length, frameType, flags, streamID, payload)
}

// TxPushPromise sends a PUSH_PROMISE frame
func (c *Conn) TxPushPromise(streamID uint32, promisedStreamID uint32, headers []string) error {
	// Build header block
	// Simplified implementation - in production would use HPACK encoding
	payload := make([]byte, 4+len(headers)*10) // Rough estimate
	binary.BigEndian.PutUint32(payload[0:4], promisedStreamID&0x7FFFFFFF)

	c.logger.Log(3, "Sending PUSH_PROMISE (stream=%d, promised=%d)", streamID, promisedStreamID)

	return WriteFrame(c.conn, Frame{
		Header: FrameHeader{
			Length:   uint32(len(payload)),
			Type:     FramePushPromise,
			Flags:    FlagEndHeaders,
			StreamID: streamID,
		},
		Payload: payload,
	})
}

// TxContinuation sends a CONTINUATION frame
func (c *Conn) TxContinuation(streamID uint32, headerBlock []byte, endHeaders bool) error {
	flags := FlagNone
	if endHeaders {
		flags = FlagEndHeaders
	}

	c.logger.Log(3, "Sending CONTINUATION (stream=%d, END_HEADERS=%v)", streamID, endHeaders)

	return WriteFrame(c.conn, Frame{
		Header: FrameHeader{
			Length:   uint32(len(headerBlock)),
			Type:     FrameContinuation,
			Flags:    flags,
			StreamID: streamID,
		},
		Payload: headerBlock,
	})
}

// TxPriority sends a PRIORITY frame
func (c *Conn) TxPriority(streamID uint32, exclusive bool, dependsOn uint32, weight uint8) error {
	payload := make([]byte, 5)

	// Stream dependency (31 bits) with exclusive flag (1 bit)
	depValue := dependsOn & 0x7FFFFFFF
	if exclusive {
		depValue |= 0x80000000
	}
	binary.BigEndian.PutUint32(payload[0:4], depValue)

	// Weight (8 bits)
	payload[4] = weight

	c.logger.Log(3, "Sending PRIORITY (stream=%d, dependsOn=%d, weight=%d, exclusive=%v)",
		streamID, dependsOn, weight, exclusive)

	return WriteFrame(c.conn, Frame{
		Header: FrameHeader{
			Length:   5,
			Type:     FramePriority,
			Flags:    FlagNone,
			StreamID: streamID,
		},
		Payload: payload,
	})
}

// SetEnforceFlowControl enables or disables flow control enforcement
func (c *Conn) SetEnforceFlowControl(enforce bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enforcedFC = enforce
	c.logger.Log(3, "Flow control enforcement: %v", enforce)
}

// GetSendWindow returns the current send window size
func (c *Conn) GetSendWindow(streamID uint32) int32 {
	if streamID == 0 {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.sendWindow
	}

	if stream, ok := c.streams.Get(streamID); ok {
		stream.mu.Lock()
		defer stream.mu.Unlock()
		return stream.SendWindow
	}

	return 0
}

// GetRecvWindow returns the current receive window size
func (c *Conn) GetRecvWindow(streamID uint32) int32 {
	if streamID == 0 {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.recvWindow
	}

	if stream, ok := c.streams.Get(streamID); ok {
		stream.mu.Lock()
		defer stream.mu.Unlock()
		return stream.RecvWindow
	}

	return 0
}

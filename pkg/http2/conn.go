package http2

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/perbu/gvtest/pkg/hpack"
	"github.com/perbu/gvtest/pkg/logging"
)

const (
	// ClientPreface is the HTTP/2 connection preface sent by clients
	ClientPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

	// DefaultMaxFrameSize is the default maximum frame size
	DefaultMaxFrameSize = 16384 // 16KB

	// DefaultWindowSize is the default flow control window size
	DefaultWindowSize = 65535 // 64KB - 1
)

// Conn represents an HTTP/2 connection
type Conn struct {
	conn   net.Conn
	logger *logging.Logger

	// HPACK encoder/decoder
	encoder *hpack.Encoder
	decoder *hpack.Decoder

	// Stream management
	streams *StreamManager

	// Settings
	localSettings  map[SettingID]uint32
	remoteSettings map[SettingID]uint32

	// Flow control
	sendWindow int32
	recvWindow int32

	// Control
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	frameRecvLoop   bool
	lastStreamID    uint32
	nextStreamID    uint32
	isClient        bool
	enforcedFC      bool // Enforce flow control
}

// NewConn creates a new HTTP/2 connection
func NewConn(conn net.Conn, logger *logging.Logger, isClient bool) *Conn {
	ctx, cancel := context.WithCancel(context.Background())

	h2conn := &Conn{
		conn:   conn,
		logger: logger,
		encoder: hpack.NewEncoder(4096), // Default table size
		decoder: hpack.NewDecoder(4096),
		streams: NewStreamManager(),
		localSettings: map[SettingID]uint32{
			SettingHeaderTableSize:      4096,
			SettingEnablePush:           1,
			SettingMaxConcurrentStreams: 100,
			SettingInitialWindowSize:    DefaultWindowSize,
			SettingMaxFrameSize:         DefaultMaxFrameSize,
		},
		remoteSettings: map[SettingID]uint32{
			SettingHeaderTableSize:      4096,
			SettingEnablePush:           1,
			SettingMaxConcurrentStreams: 100,
			SettingInitialWindowSize:    DefaultWindowSize,
			SettingMaxFrameSize:         DefaultMaxFrameSize,
		},
		sendWindow:   DefaultWindowSize,
		recvWindow:   DefaultWindowSize,
		ctx:          ctx,
		cancel:       cancel,
		isClient:     isClient,
		enforcedFC:   true,
		nextStreamID: 1,
	}

	if isClient {
		h2conn.nextStreamID = 1 // Client uses odd stream IDs
	} else {
		h2conn.nextStreamID = 2 // Server uses even stream IDs
	}

	return h2conn
}

// Start initiates the HTTP/2 connection
func (c *Conn) Start() error {
	if c.isClient {
		// Client sends preface
		if err := c.SendPreface(); err != nil {
			return fmt.Errorf("failed to send preface: %w", err)
		}
	} else {
		// Server receives preface
		if err := c.ReceivePreface(); err != nil {
			return fmt.Errorf("failed to receive preface: %w", err)
		}
	}

	// Send initial SETTINGS frame
	if err := c.SendSettings(false); err != nil {
		return fmt.Errorf("failed to send SETTINGS: %w", err)
	}

	// Start frame receive loop
	go c.frameReceiveLoop()

	return nil
}

// Stop closes the HTTP/2 connection
func (c *Conn) Stop() error {
	c.cancel()
	return c.conn.Close()
}

// SendPreface sends the HTTP/2 client connection preface
func (c *Conn) SendPreface() error {
	c.logger.Log(3, "Sending HTTP/2 preface")
	_, err := c.conn.Write([]byte(ClientPreface))
	return err
}

// ReceivePreface receives and validates the HTTP/2 client connection preface
func (c *Conn) ReceivePreface() error {
	c.logger.Log(3, "Receiving HTTP/2 preface")

	buf := make([]byte, len(ClientPreface))
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	defer c.conn.SetReadDeadline(time.Time{})

	_, err := io.ReadFull(c.conn, buf)
	if err != nil {
		return fmt.Errorf("failed to read preface: %w", err)
	}

	if string(buf) != ClientPreface {
		return fmt.Errorf("invalid preface: got %q", buf)
	}

	c.logger.Log(3, "Received valid HTTP/2 preface")
	return nil
}

// SendSettings sends a SETTINGS frame
func (c *Conn) SendSettings(ack bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	settings := make([]Setting, 0, len(c.localSettings))
	if !ack {
		for id, value := range c.localSettings {
			settings = append(settings, Setting{ID: id, Value: value})
		}
	}

	c.logger.Log(3, "Sending SETTINGS (ack=%v, %d settings)", ack, len(settings))
	return WriteSettingsFrame(c.conn, 0, ack, settings)
}

// SendSettingsAck sends a SETTINGS ACK frame
func (c *Conn) SendSettingsAck() error {
	return c.SendSettings(true)
}

// UpdateSetting updates a local setting
func (c *Conn) UpdateSetting(id SettingID, value uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localSettings[id] = value

	// Update encoder/decoder table sizes
	if id == SettingHeaderTableSize {
		c.encoder.SetMaxDynamicTableSize(value)
	}
}

// GetSetting retrieves a local setting value
func (c *Conn) GetSetting(id SettingID) uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.localSettings[id]
}

// frameReceiveLoop continuously receives and processes frames
func (c *Conn) frameReceiveLoop() {
	c.mu.Lock()
	c.frameRecvLoop = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.frameRecvLoop = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Set a read deadline to allow checking context periodically
		c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		frame, err := ReadFrame(c.conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout - check context and continue
				continue
			}
			if err != io.EOF {
				c.logger.Log(1, "Frame receive error: %v", err)
			}
			return
		}

		c.conn.SetReadDeadline(time.Time{})

		// Process the frame
		if err := c.processFrame(frame); err != nil {
			c.logger.Log(1, "Frame process error: %v", err)
			return
		}
	}
}

// processFrame processes a received frame
func (c *Conn) processFrame(frame Frame) error {
	c.logger.Log(4, "Received frame: type=%s, flags=0x%x, stream=%d, length=%d",
		frame.Header.Type, frame.Header.Flags, frame.Header.StreamID, frame.Header.Length)

	switch frame.Header.Type {
	case FrameSettings:
		return c.handleSettings(frame)
	case FramePing:
		return c.handlePing(frame)
	case FrameGoAway:
		return c.handleGoAway(frame)
	case FrameWindowUpdate:
		return c.handleWindowUpdate(frame)
	case FrameHeaders:
		return c.handleHeaders(frame)
	case FrameData:
		return c.handleData(frame)
	case FrameRSTStream:
		return c.handleRSTStream(frame)
	case FrameContinuation:
		return c.handleContinuation(frame)
	default:
		c.logger.Log(2, "Unhandled frame type: %s", frame.Header.Type)
	}

	return nil
}

// handleSettings processes a SETTINGS frame
func (c *Conn) handleSettings(frame Frame) error {
	if frame.Header.Flags.Has(FlagAck) {
		c.logger.Log(3, "Received SETTINGS ACK")
		return nil
	}

	settings, err := ParseSettingsFrame(frame.Payload)
	if err != nil {
		return err
	}

	c.mu.Lock()
	for _, setting := range settings {
		c.logger.Log(3, "Received SETTING: %s = %d", setting.ID, setting.Value)
		c.remoteSettings[setting.ID] = setting.Value

		// Update decoder table size if needed
		if setting.ID == SettingHeaderTableSize {
			c.decoder.SetMaxDynamicTableSize(setting.Value)
		}
	}
	c.mu.Unlock()

	// Send ACK
	return c.SendSettingsAck()
}

// handlePing processes a PING frame
func (c *Conn) handlePing(frame Frame) error {
	if frame.Header.Flags.Has(FlagAck) {
		c.logger.Log(3, "Received PING ACK")
		return nil
	}

	if len(frame.Payload) != 8 {
		return fmt.Errorf("invalid PING payload length: %d", len(frame.Payload))
	}

	var data [8]byte
	copy(data[:], frame.Payload)

	c.logger.Log(3, "Received PING, sending ACK")
	return WritePingFrame(c.conn, true, data)
}

// handleGoAway processes a GOAWAY frame
func (c *Conn) handleGoAway(frame Frame) error {
	if len(frame.Payload) < 8 {
		return fmt.Errorf("invalid GOAWAY payload length: %d", len(frame.Payload))
	}

	c.logger.Log(2, "Received GOAWAY")
	c.cancel() // Stop the connection
	return nil
}

// handleWindowUpdate processes a WINDOW_UPDATE frame
func (c *Conn) handleWindowUpdate(frame Frame) error {
	if len(frame.Payload) != 4 {
		return fmt.Errorf("invalid WINDOW_UPDATE payload length: %d", len(frame.Payload))
	}

	increment := int32(uint32(frame.Payload[0])<<24 | uint32(frame.Payload[1])<<16 |
		uint32(frame.Payload[2])<<8 | uint32(frame.Payload[3]))
	increment &= 0x7FFFFFFF

	if frame.Header.StreamID == 0 {
		// Connection-level window update
		c.mu.Lock()
		c.sendWindow += increment
		c.mu.Unlock()
		c.logger.Log(3, "Connection window update: +%d", increment)
	} else {
		// Stream-level window update
		if stream, ok := c.streams.Get(frame.Header.StreamID); ok {
			stream.UpdateSendWindow(increment)
			c.logger.Log(3, "Stream %d window update: +%d", frame.Header.StreamID, increment)
		}
	}

	return nil
}

// handleHeaders processes a HEADERS frame
func (c *Conn) handleHeaders(frame Frame) error {
	stream := c.streams.GetOrCreate(frame.Header.StreamID, fmt.Sprintf("stream-%d", frame.Header.StreamID))

	// Decode HPACK headers
	headers, err := c.decoder.Decode(frame.Payload)
	if err != nil {
		return fmt.Errorf("failed to decode headers: %w", err)
	}

	for _, hf := range headers {
		stream.AddReqHeader(hf.Name, hf.Value)
	}

	endStream := frame.Header.Flags.Has(FlagEndStream)
	stream.UpdateState(endStream, false)

	c.logger.Log(3, "Received HEADERS on stream %d (END_STREAM=%v)", frame.Header.StreamID, endStream)

	// Signal the stream
	stream.Signal()

	return nil
}

// handleData processes a DATA frame
func (c *Conn) handleData(frame Frame) error {
	stream, ok := c.streams.Get(frame.Header.StreamID)
	if !ok {
		return fmt.Errorf("DATA frame for unknown stream %d", frame.Header.StreamID)
	}

	stream.AppendReqBody(frame.Payload)

	endStream := frame.Header.Flags.Has(FlagEndStream)
	stream.UpdateState(endStream, false)

	c.logger.Log(3, "Received DATA on stream %d: %d bytes (END_STREAM=%v)",
		frame.Header.StreamID, len(frame.Payload), endStream)

	// Signal the stream
	stream.Signal()

	return nil
}

// handleRSTStream processes an RST_STREAM frame
func (c *Conn) handleRSTStream(frame Frame) error {
	c.logger.Log(3, "Received RST_STREAM on stream %d", frame.Header.StreamID)
	if stream, ok := c.streams.Get(frame.Header.StreamID); ok {
		stream.mu.Lock()
		stream.State = StreamClosed
		stream.mu.Unlock()
		stream.Signal()
	}
	return nil
}

// handleContinuation processes a CONTINUATION frame
func (c *Conn) handleContinuation(frame Frame) error {
	c.logger.Log(3, "Received CONTINUATION on stream %d", frame.Header.StreamID)
	// CONTINUATION frames extend HEADERS frames
	// For simplicity, we'll handle them similarly to HEADERS
	return c.handleHeaders(frame)
}

// NextStreamID returns the next stream ID to use
func (c *Conn) NextStreamID() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextStreamID
	c.nextStreamID += 2 // Skip by 2 (client: 1,3,5... server: 2,4,6...)
	return id
}

// WriteFrame writes a frame to the connection
func (c *Conn) WriteFrame(frame Frame) error {
	return WriteFrame(c.conn, frame)
}

// WriteRawFrame writes a raw frame with manual control
func (c *Conn) WriteRawFrame(length uint32, frameType FrameType, flags Flags, streamID uint32, payload []byte) error {
	return WriteRawFrame(c.conn, length, frameType, flags, streamID, payload)
}

// GetStream retrieves a stream by ID
func (c *Conn) GetStream(streamID uint32) (*Stream, bool) {
	return c.streams.Get(streamID)
}

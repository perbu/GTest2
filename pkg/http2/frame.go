package http2

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame types as defined in RFC 7540
const (
	FrameData         FrameType = 0x0
	FrameHeaders      FrameType = 0x1
	FramePriority     FrameType = 0x2
	FrameRSTStream    FrameType = 0x3
	FrameSettings     FrameType = 0x4
	FramePushPromise  FrameType = 0x5
	FramePing         FrameType = 0x6
	FrameGoAway       FrameType = 0x7
	FrameWindowUpdate FrameType = 0x8
	FrameContinuation FrameType = 0x9
)

// Frame flags as defined in RFC 7540
const (
	FlagNone        Flags = 0x0
	FlagAck         Flags = 0x1  // SETTINGS, PING
	FlagEndStream   Flags = 0x1  // DATA, HEADERS
	FlagEndHeaders  Flags = 0x4  // HEADERS, PUSH_PROMISE, CONTINUATION
	FlagPadded      Flags = 0x8  // DATA, HEADERS, PUSH_PROMISE
	FlagPriority    Flags = 0x20 // HEADERS
)

const (
	// FrameHeaderLen is the length of the frame header
	FrameHeaderLen = 9

	// MaxFrameSize is the default maximum frame size
	MaxFrameSize = 1 << 14 // 16KB

	// MaxFrameSizeLimit is the maximum allowed frame size
	MaxFrameSizeLimit = 1<<24 - 1 // 16MB - 1
)

// FrameType represents the type of HTTP/2 frame
type FrameType uint8

func (t FrameType) String() string {
	switch t {
	case FrameData:
		return "DATA"
	case FrameHeaders:
		return "HEADERS"
	case FramePriority:
		return "PRIORITY"
	case FrameRSTStream:
		return "RST_STREAM"
	case FrameSettings:
		return "SETTINGS"
	case FramePushPromise:
		return "PUSH_PROMISE"
	case FramePing:
		return "PING"
	case FrameGoAway:
		return "GOAWAY"
	case FrameWindowUpdate:
		return "WINDOW_UPDATE"
	case FrameContinuation:
		return "CONTINUATION"
	default:
		return fmt.Sprintf("UNKNOWN(0x%x)", uint8(t))
	}
}

// Flags represents frame flags
type Flags uint8

func (f Flags) Has(flag Flags) bool {
	return (f & flag) != 0
}

// FrameHeader represents the 9-byte HTTP/2 frame header
type FrameHeader struct {
	Length   uint32    // 24-bit length
	Type     FrameType // 8-bit type
	Flags    Flags     // 8-bit flags
	StreamID uint32    // 31-bit stream identifier (reserved bit always 0)
}

// Frame represents a complete HTTP/2 frame
type Frame struct {
	Header  FrameHeader
	Payload []byte
}

// WriteFrameHeader writes a frame header to the writer
func WriteFrameHeader(w io.Writer, h FrameHeader) error {
	var buf [FrameHeaderLen]byte

	// Length (24 bits)
	buf[0] = byte(h.Length >> 16)
	buf[1] = byte(h.Length >> 8)
	buf[2] = byte(h.Length)

	// Type (8 bits)
	buf[3] = byte(h.Type)

	// Flags (8 bits)
	buf[4] = byte(h.Flags)

	// Stream ID (31 bits, R bit is always 0)
	binary.BigEndian.PutUint32(buf[5:9], h.StreamID&0x7FFFFFFF)

	_, err := w.Write(buf[:])
	return err
}

// ReadFrameHeader reads a frame header from the reader
func ReadFrameHeader(r io.Reader) (FrameHeader, error) {
	var buf [FrameHeaderLen]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return FrameHeader{}, err
	}

	return ParseFrameHeader(buf[:])
}

// ParseFrameHeader parses a frame header from a byte slice
func ParseFrameHeader(buf []byte) (FrameHeader, error) {
	if len(buf) < FrameHeaderLen {
		return FrameHeader{}, fmt.Errorf("buffer too short for frame header: %d < %d", len(buf), FrameHeaderLen)
	}

	h := FrameHeader{
		Length:   uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2]),
		Type:     FrameType(buf[3]),
		Flags:    Flags(buf[4]),
		StreamID: binary.BigEndian.Uint32(buf[5:9]) & 0x7FFFFFFF,
	}

	return h, nil
}

// WriteFrame writes a complete frame (header + payload) to the writer
func WriteFrame(w io.Writer, f Frame) error {
	// Update header length to match payload
	f.Header.Length = uint32(len(f.Payload))

	if err := WriteFrameHeader(w, f.Header); err != nil {
		return err
	}

	if len(f.Payload) > 0 {
		_, err := w.Write(f.Payload)
		return err
	}

	return nil
}

// ReadFrame reads a complete frame from the reader
func ReadFrame(r io.Reader) (Frame, error) {
	header, err := ReadFrameHeader(r)
	if err != nil {
		return Frame{}, err
	}

	payload := make([]byte, header.Length)
	if header.Length > 0 {
		_, err = io.ReadFull(r, payload)
		if err != nil {
			return Frame{}, err
		}
	}

	return Frame{Header: header, Payload: payload}, nil
}

// WriteRawFrame writes a frame with complete manual control
// This allows intentionally malformed frames for testing
func WriteRawFrame(w io.Writer, length uint32, frameType FrameType, flags Flags, streamID uint32, payload []byte) error {
	var buf [FrameHeaderLen]byte

	// Length (24 bits) - use provided length, may not match payload
	buf[0] = byte(length >> 16)
	buf[1] = byte(length >> 8)
	buf[2] = byte(length)

	// Type (8 bits)
	buf[3] = byte(frameType)

	// Flags (8 bits)
	buf[4] = byte(flags)

	// Stream ID (31 bits) - use provided streamID, may have reserved bit set
	binary.BigEndian.PutUint32(buf[5:9], streamID)

	_, err := w.Write(buf[:])
	if err != nil {
		return err
	}

	if len(payload) > 0 {
		_, err = w.Write(payload)
	}

	return err
}

// Setting represents an HTTP/2 SETTINGS parameter
type Setting struct {
	ID    SettingID
	Value uint32
}

// SettingID identifies an HTTP/2 setting
type SettingID uint16

const (
	SettingHeaderTableSize      SettingID = 0x1
	SettingEnablePush           SettingID = 0x2
	SettingMaxConcurrentStreams SettingID = 0x3
	SettingInitialWindowSize    SettingID = 0x4
	SettingMaxFrameSize         SettingID = 0x5
	SettingMaxHeaderListSize    SettingID = 0x6
)

func (s SettingID) String() string {
	switch s {
	case SettingHeaderTableSize:
		return "HEADER_TABLE_SIZE"
	case SettingEnablePush:
		return "ENABLE_PUSH"
	case SettingMaxConcurrentStreams:
		return "MAX_CONCURRENT_STREAMS"
	case SettingInitialWindowSize:
		return "INITIAL_WINDOW_SIZE"
	case SettingMaxFrameSize:
		return "MAX_FRAME_SIZE"
	case SettingMaxHeaderListSize:
		return "MAX_HEADER_LIST_SIZE"
	default:
		return fmt.Sprintf("UNKNOWN(0x%x)", uint16(s))
	}
}

// WriteSettingsFrame writes a SETTINGS frame
func WriteSettingsFrame(w io.Writer, streamID uint32, ack bool, settings []Setting) error {
	flags := FlagNone
	if ack {
		flags = FlagAck
	}

	payload := make([]byte, len(settings)*6)
	for i, s := range settings {
		binary.BigEndian.PutUint16(payload[i*6:], uint16(s.ID))
		binary.BigEndian.PutUint32(payload[i*6+2:], s.Value)
	}

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   uint32(len(payload)),
			Type:     FrameSettings,
			Flags:    flags,
			StreamID: streamID,
		},
		Payload: payload,
	})
}

// ParseSettingsFrame parses the payload of a SETTINGS frame
func ParseSettingsFrame(payload []byte) ([]Setting, error) {
	if len(payload)%6 != 0 {
		return nil, fmt.Errorf("invalid SETTINGS frame payload length: %d", len(payload))
	}

	settings := make([]Setting, len(payload)/6)
	for i := 0; i < len(settings); i++ {
		offset := i * 6
		settings[i] = Setting{
			ID:    SettingID(binary.BigEndian.Uint16(payload[offset:])),
			Value: binary.BigEndian.Uint32(payload[offset+2:]),
		}
	}

	return settings, nil
}

// WriteDataFrame writes a DATA frame
func WriteDataFrame(w io.Writer, streamID uint32, data []byte, endStream bool) error {
	flags := FlagNone
	if endStream {
		flags = FlagEndStream
	}

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   uint32(len(data)),
			Type:     FrameData,
			Flags:    flags,
			StreamID: streamID,
		},
		Payload: data,
	})
}

// WriteHeadersFrame writes a HEADERS frame
func WriteHeadersFrame(w io.Writer, streamID uint32, headerBlock []byte, endStream, endHeaders bool) error {
	flags := FlagNone
	if endStream {
		flags |= FlagEndStream
	}
	if endHeaders {
		flags |= FlagEndHeaders
	}

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   uint32(len(headerBlock)),
			Type:     FrameHeaders,
			Flags:    flags,
			StreamID: streamID,
		},
		Payload: headerBlock,
	})
}

// WriteRSTStreamFrame writes an RST_STREAM frame
func WriteRSTStreamFrame(w io.Writer, streamID uint32, errorCode uint32) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, errorCode)

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   4,
			Type:     FrameRSTStream,
			Flags:    FlagNone,
			StreamID: streamID,
		},
		Payload: payload,
	})
}

// WritePingFrame writes a PING frame
func WritePingFrame(w io.Writer, ack bool, data [8]byte) error {
	flags := FlagNone
	if ack {
		flags = FlagAck
	}

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   8,
			Type:     FramePing,
			Flags:    flags,
			StreamID: 0,
		},
		Payload: data[:],
	})
}

// WriteGoAwayFrame writes a GOAWAY frame
func WriteGoAwayFrame(w io.Writer, lastStreamID uint32, errorCode uint32, debugData []byte) error {
	payload := make([]byte, 8+len(debugData))
	binary.BigEndian.PutUint32(payload[0:4], lastStreamID&0x7FFFFFFF)
	binary.BigEndian.PutUint32(payload[4:8], errorCode)
	copy(payload[8:], debugData)

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   uint32(len(payload)),
			Type:     FrameGoAway,
			Flags:    FlagNone,
			StreamID: 0,
		},
		Payload: payload,
	})
}

// WriteWindowUpdateFrame writes a WINDOW_UPDATE frame
func WriteWindowUpdateFrame(w io.Writer, streamID uint32, increment uint32) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, increment&0x7FFFFFFF)

	return WriteFrame(w, Frame{
		Header: FrameHeader{
			Length:   4,
			Type:     FrameWindowUpdate,
			Flags:    FlagNone,
			StreamID: streamID,
		},
		Payload: payload,
	})
}

// Error codes as defined in RFC 7540
const (
	ErrCodeNo                 uint32 = 0x0
	ErrCodeProtocol           uint32 = 0x1
	ErrCodeInternal           uint32 = 0x2
	ErrCodeFlowControl        uint32 = 0x3
	ErrCodeSettingsTimeout    uint32 = 0x4
	ErrCodeStreamClosed       uint32 = 0x5
	ErrCodeFrameSize          uint32 = 0x6
	ErrCodeRefusedStream      uint32 = 0x7
	ErrCodeCancel             uint32 = 0x8
	ErrCodeCompression        uint32 = 0x9
	ErrCodeConnect            uint32 = 0xa
	ErrCodeEnhanceYourCalm    uint32 = 0xb
	ErrCodeInadequateSecurity uint32 = 0xc
	ErrCodeHTTP11Required     uint32 = 0xd
)

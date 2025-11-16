package http2

import (
	"fmt"
	"sync"

	"github.com/perbu/gvtest/pkg/hpack"
)

// StreamState represents the state of an HTTP/2 stream
type StreamState int

const (
	StreamIdle StreamState = iota
	StreamReservedLocal
	StreamReservedRemote
	StreamOpen
	StreamHalfClosedLocal
	StreamHalfClosedRemote
	StreamClosed
)

func (s StreamState) String() string {
	switch s {
	case StreamIdle:
		return "idle"
	case StreamReservedLocal:
		return "reserved(local)"
	case StreamReservedRemote:
		return "reserved(remote)"
	case StreamOpen:
		return "open"
	case StreamHalfClosedLocal:
		return "half-closed(local)"
	case StreamHalfClosedRemote:
		return "half-closed(remote)"
	case StreamClosed:
		return "closed"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Stream represents an HTTP/2 stream
type Stream struct {
	ID    uint32
	Name  string
	State StreamState

	// Request/Response data
	ReqHeaders  []hpack.HeaderField
	RespHeaders []hpack.HeaderField
	ReqBody     []byte
	RespBody    []byte

	// Pseudo-headers for HTTP/2
	Method    string
	Path      string
	Scheme    string
	Authority string
	Status    string

	// Flow control windows
	SendWindow int32
	RecvWindow int32

	// Synchronization
	mu     sync.Mutex
	signal chan struct{} // For stream events
}

// NewStream creates a new stream
func NewStream(id uint32, name string) *Stream {
	return &Stream{
		ID:         id,
		Name:       name,
		State:      StreamIdle,
		ReqHeaders: make([]hpack.HeaderField, 0),
		RespHeaders: make([]hpack.HeaderField, 0),
		ReqBody:    make([]byte, 0),
		RespBody:   make([]byte, 0),
		SendWindow: 65535, // Default initial window size
		RecvWindow: 65535,
		signal:     make(chan struct{}, 1),
	}
}

// UpdateState updates the stream state based on an event
func (s *Stream) UpdateState(endStream bool, sending bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.State {
	case StreamIdle:
		if sending {
			if endStream {
				s.State = StreamHalfClosedLocal
			} else {
				s.State = StreamOpen
			}
		} else {
			if endStream {
				s.State = StreamHalfClosedRemote
			} else {
				s.State = StreamOpen
			}
		}

	case StreamOpen:
		if sending && endStream {
			s.State = StreamHalfClosedLocal
		} else if !sending && endStream {
			s.State = StreamHalfClosedRemote
		}

	case StreamHalfClosedLocal:
		if !sending && endStream {
			s.State = StreamClosed
		}

	case StreamHalfClosedRemote:
		if sending && endStream {
			s.State = StreamClosed
		}

	case StreamClosed:
		// Already closed, no state change
	}

	return nil
}

// AddReqHeader adds a request header
func (s *Stream) AddReqHeader(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle pseudo-headers
	switch name {
	case ":method":
		s.Method = value
	case ":path":
		s.Path = value
	case ":scheme":
		s.Scheme = value
	case ":authority":
		s.Authority = value
	}

	s.ReqHeaders = append(s.ReqHeaders, hpack.HeaderField{Name: name, Value: value})
}

// AddRespHeader adds a response header
func (s *Stream) AddRespHeader(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle pseudo-headers
	if name == ":status" {
		s.Status = value
	}

	s.RespHeaders = append(s.RespHeaders, hpack.HeaderField{Name: name, Value: value})
}

// AppendReqBody appends data to the request body
func (s *Stream) AppendReqBody(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ReqBody = append(s.ReqBody, data...)
}

// AppendRespBody appends data to the response body
func (s *Stream) AppendRespBody(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RespBody = append(s.RespBody, data...)
}

// GetHeader retrieves a header value by name
func (s *Stream) GetHeader(headers []hpack.HeaderField, name string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, hf := range headers {
		if hf.Name == name {
			return hf.Value
		}
	}
	return ""
}

// Signal sends a signal to waiting goroutines
func (s *Stream) Signal() {
	select {
	case s.signal <- struct{}{}:
	default:
	}
}

// Wait waits for a signal with a timeout
func (s *Stream) Wait() {
	<-s.signal
}

// UpdateSendWindow updates the send window size
func (s *Stream) UpdateSendWindow(delta int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SendWindow += delta
}

// UpdateRecvWindow updates the receive window size
func (s *Stream) UpdateRecvWindow(delta int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecvWindow += delta
}

// StreamManager manages multiple HTTP/2 streams
type StreamManager struct {
	streams map[uint32]*Stream
	mu      sync.RWMutex
}

// NewStreamManager creates a new stream manager
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[uint32]*Stream),
	}
}

// Create creates a new stream with the given ID and name
func (sm *StreamManager) Create(id uint32, name string) *Stream {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s := NewStream(id, name)
	sm.streams[id] = s
	return s
}

// Get retrieves a stream by ID
func (sm *StreamManager) Get(id uint32) (*Stream, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.streams[id]
	return s, ok
}

// GetByName retrieves a stream by name
func (sm *StreamManager) GetByName(name string) (*Stream, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, s := range sm.streams {
		if s.Name == name {
			return s, true
		}
	}
	return nil, false
}

// GetOrCreate gets an existing stream or creates a new one
func (sm *StreamManager) GetOrCreate(id uint32, name string) *Stream {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[id]; ok {
		return s
	}

	s := NewStream(id, name)
	sm.streams[id] = s
	return s
}

// Delete removes a stream from the manager
func (sm *StreamManager) Delete(id uint32) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.streams, id)
}

// Count returns the number of active streams
func (sm *StreamManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.streams)
}

// List returns all stream IDs
func (sm *StreamManager) List() []uint32 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := make([]uint32, 0, len(sm.streams))
	for id := range sm.streams {
		ids = append(ids, id)
	}
	return ids
}

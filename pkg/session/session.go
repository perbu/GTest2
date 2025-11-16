// Package session provides the session abstraction for VTest2.
// Sessions manage connection lifecycle, including repeat counts,
// keepalive settings, and receive buffer configuration.
package session

import (
	"fmt"
	"net"
	"strconv"

	"github.com/perbu/GTest/pkg/logging"
)

// ConnectFunc is a function that establishes a connection and returns a net.Conn
type ConnectFunc func() (net.Conn, error)

// DisconnectFunc is a function that closes a connection
type DisconnectFunc func(conn net.Conn) error

// ProcessFunc is a function that processes a session (e.g., HTTP processing)
type ProcessFunc func(conn net.Conn, spec string) (net.Conn, error)

// Session represents a VTC session with connection management
type Session struct {
	Name      string
	Logger    *logging.Logger
	Repeat    int
	Keepalive bool
	RcvBuf    int
	FD        net.Conn
}

// New creates a new session with the given name and logger
func New(logger *logging.Logger, name string) *Session {
	return &Session{
		Name:      name,
		Logger:    logger,
		Repeat:    1,
		Keepalive: false,
		RcvBuf:    0,
		FD:        nil,
	}
}

// ParseOption parses session options from command arguments.
// Returns the number of arguments consumed, or an error.
func (s *Session) ParseOption(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}

	switch args[0] {
	case "-rcvbuf":
		if len(args) < 2 {
			return 0, fmt.Errorf("-rcvbuf requires an argument")
		}
		val, err := strconv.Atoi(args[1])
		if err != nil {
			return 0, fmt.Errorf("-rcvbuf: invalid value %s: %w", args[1], err)
		}
		s.RcvBuf = val
		return 2, nil

	case "-repeat":
		if len(args) < 2 {
			return 0, fmt.Errorf("-repeat requires an argument")
		}
		val, err := strconv.Atoi(args[1])
		if err != nil {
			return 0, fmt.Errorf("-repeat: invalid value %s: %w", args[1], err)
		}
		if val < 1 {
			return 0, fmt.Errorf("-repeat: value must be >= 1, got %d", val)
		}
		s.Repeat = val
		return 2, nil

	case "-keepalive":
		s.Keepalive = true
		return 1, nil

	default:
		return 0, nil
	}
}

// Run executes the session with the given connect, disconnect, and process functions.
// This is the main session execution loop that handles repeats and keepalive.
func (s *Session) Run(
	spec string,
	addr string,
	connectFunc ConnectFunc,
	disconnectFunc DisconnectFunc,
	processFunc ProcessFunc,
) error {
	var conn net.Conn
	var err error

	s.Logger.Log(2, "Started on %s (%d iterations%s)", addr, s.Repeat,
		map[bool]string{true: " using keepalive", false: ""}[s.Keepalive])

	for i := 0; i < s.Repeat; i++ {
		// Connect if we don't have a connection
		if conn == nil {
			conn, err = connectFunc()
			if err != nil {
				return fmt.Errorf("connection failed: %w", err)
			}
		}

		// Process the session
		conn, err = processFunc(conn, spec)
		if err != nil {
			if conn != nil {
				conn.Close()
			}
			return fmt.Errorf("process failed: %w", err)
		}

		// Disconnect if not using keepalive
		if !s.Keepalive && conn != nil {
			if disconnectFunc != nil {
				disconnectFunc(conn)
			} else {
				conn.Close()
			}
			conn = nil
		}
	}

	// Close connection if keepalive was used
	if s.Keepalive && conn != nil {
		if disconnectFunc != nil {
			disconnectFunc(conn)
		} else {
			conn.Close()
		}
	}

	s.Logger.Log(2, "Ending")
	return nil
}

// Close closes the session's connection if open
func (s *Session) Close() error {
	if s.FD != nil {
		err := s.FD.Close()
		s.FD = nil
		return err
	}
	return nil
}

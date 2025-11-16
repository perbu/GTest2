// Package server provides server implementation for VTest2.
// Servers listen on TCP or Unix sockets and accept incoming connections.
package server

import (
	"fmt"
	"net"
	"sync"

	"github.com/perbu/GTest/pkg/logging"
	gnet "github.com/perbu/GTest/pkg/net"
	"github.com/perbu/GTest/pkg/session"
	"github.com/perbu/GTest/pkg/vtc"
)

// ProcessFunc is called to process an accepted connection
type ProcessFunc func(conn net.Conn, spec string, listenAddr string) error

// Server represents a listening server
type Server struct {
	Name       string
	Logger     *logging.Logger
	Session    *session.Session
	Spec       string
	Listen     string
	Depth      int // Listen backlog depth
	Listener   net.Listener
	Addr       string
	Port       string
	Running    bool
	IsDispatch bool
	macros     *vtc.MacroStore

	// Internal
	stopChan chan struct{}
	wg       sync.WaitGroup
	mutex    sync.Mutex
}

// New creates a new server with the given name
func New(logger *logging.Logger, macros *vtc.MacroStore, name string) *Server {
	sessLogger := logging.NewLogger(name)
	sess := session.New(sessLogger, name)

	return &Server{
		Name:     name,
		Logger:   logger,
		Session:  sess,
		Listen:   "127.0.0.1:0", // Default to random port
		Depth:    10,
		Running:  false,
		macros:   macros,
		stopChan: make(chan struct{}),
	}
}

// SetListen sets the listen address for the server
func (s *Server) SetListen(addr string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Listen = addr
}

// Start starts the server listening on the configured address
func (s *Server) Start(processFunc ProcessFunc) error {
	s.mutex.Lock()
	if s.Running {
		s.mutex.Unlock()
		return fmt.Errorf("server %s already running", s.Name)
	}
	s.mutex.Unlock()

	s.Logger.Log(2, "Starting server %s", s.Name)

	// Create listener
	listener, addrInfo, err := gnet.TCPListen(s.Listen, s.Depth)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.Listener = listener
	s.Addr = addrInfo.Addr
	s.Port = addrInfo.Port

	// Update listen address with actual bound address
	if addrInfo.Port != "" {
		s.Listen = fmt.Sprintf("%s:%s", s.Addr, s.Port)
	} else {
		s.Listen = s.Addr
	}

	s.Logger.Log(1, "Listen on %s", s.Listen)

	// Define macros for the server
	s.defineMacros()

	s.Running = true

	// Start accept loop in goroutine
	s.wg.Add(1)
	go s.acceptLoop(processFunc)

	return nil
}

// acceptLoop handles incoming connections
func (s *Server) acceptLoop(processFunc ProcessFunc) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// Set a timeout on Accept so we can check stopChan periodically
		// Note: We'll use the raw listener for now
		conn, err := s.Listener.Accept()
		if err != nil {
			// Check if we're stopping
			select {
			case <-s.stopChan:
				return
			default:
				s.Logger.Error("Accept failed: %v", err)
				continue
			}
		}

		// Log the accepted connection
		remoteAddr := gnet.GetRemoteAddr(conn)
		if remoteAddr.Port != "" {
			s.Logger.Log(3, "accepted connection from %s:%s", remoteAddr.Addr, remoteAddr.Port)
		} else {
			s.Logger.Log(3, "accepted connection from %s", remoteAddr.Addr)
		}

		// Handle connection based on session settings
		if s.IsDispatch {
			// Dispatch mode: handle each connection in a new goroutine
			s.wg.Add(1)
			go s.handleConnection(conn, processFunc)
		} else {
			// Regular mode: handle in session (may use keepalive)
			s.wg.Add(1)
			go s.handleSessionConnection(conn, processFunc)
		}
	}
}

// handleConnection processes a single connection (dispatch mode)
func (s *Server) handleConnection(conn net.Conn, processFunc ProcessFunc) {
	defer s.wg.Done()
	defer conn.Close()

	if processFunc != nil {
		err := processFunc(conn, s.Spec, s.Listen)
		if err != nil {
			s.Logger.Error("Connection processing failed: %v", err)
		}
	}

	s.Logger.Log(3, "connection closed")
}

// handleSessionConnection processes connections using session settings (repeat, keepalive)
func (s *Server) handleSessionConnection(conn net.Conn, processFunc ProcessFunc) {
	defer s.wg.Done()

	// Use the session's Run method
	connectFunc := func() (net.Conn, error) {
		// Connection already established
		return conn, nil
	}

	disconnectFunc := func(c net.Conn) error {
		return c.Close()
	}

	procFunc := func(c net.Conn, spec string) (net.Conn, error) {
		if processFunc != nil {
			err := processFunc(c, spec, s.Listen)
			return c, err
		}
		return c, nil
	}

	err := s.Session.Run(s.Spec, s.Listen, connectFunc, disconnectFunc, procFunc)
	if err != nil {
		s.Logger.Error("Session failed: %v", err)
	}
}

// Wait waits for the server to stop
func (s *Server) Wait() {
	s.wg.Wait()
	s.mutex.Lock()
	s.Running = false
	s.mutex.Unlock()
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mutex.Lock()
	if !s.Running {
		s.mutex.Unlock()
		return nil
	}
	s.mutex.Unlock()

	s.Logger.Log(2, "Stopping server %s", s.Name)

	// Signal stop
	close(s.stopChan)

	// Close listener
	if s.Listener != nil {
		s.Listener.Close()
	}

	// Wait for all connections to finish
	s.wg.Wait()

	s.mutex.Lock()
	s.Running = false
	s.mutex.Unlock()

	// Undefine macros
	s.undefineMacros()

	return nil
}

// Break forces the server to stop (cancel operation)
func (s *Server) Break() error {
	return s.Stop()
}

// defineMacros defines the server macros (addr, port, sock)
func (s *Server) defineMacros() {
	if s.macros == nil {
		return
	}

	// Define ${sNAME_addr}
	s.macros.Definef(s.Name+"_addr", "%s", s.Addr)

	// Define ${sNAME_port}
	s.macros.Definef(s.Name+"_port", "%s", s.Port)

	// Define ${sNAME_sock}
	s.macros.Definef(s.Name+"_sock", "%s", s.Listen)
}

// undefineMacros removes the server macros
func (s *Server) undefineMacros() {
	if s.macros == nil {
		return
	}

	s.macros.Delete(s.Name + "_addr")
	s.macros.Delete(s.Name + "_port")
	s.macros.Delete(s.Name + "_sock")
}

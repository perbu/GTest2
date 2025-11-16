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
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mutex          sync.Mutex
	connCount      int // Number of connections handled
	connCountMutex sync.Mutex
	stopping       bool // Track if stop has been initiated
	stoppingMutex  sync.Mutex
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
	s.Logger.Debug("Start called for server %s", s.Name)

	s.mutex.Lock()
	if s.Running {
		s.mutex.Unlock()
		s.Logger.Debug("Server %s already running, returning error", s.Name)
		return fmt.Errorf("server %s already running", s.Name)
	}
	s.mutex.Unlock()

	// Reset connection counter
	s.connCountMutex.Lock()
	s.connCount = 0
	s.connCountMutex.Unlock()
	s.Logger.Debug("Reset connection counter for server %s", s.Name)

	// Reset stop channel and stopping flag
	s.stoppingMutex.Lock()
	s.stopChan = make(chan struct{})
	s.stopping = false
	s.stoppingMutex.Unlock()
	s.Logger.Debug("Reset stop channel for server %s", s.Name)

	s.Logger.Log(2, "Starting server %s", s.Name)

	// Create listener
	s.Logger.Debug("Creating listener on %s with backlog %d", s.Listen, s.Depth)
	listener, addrInfo, err := gnet.TCPListen(s.Listen, s.Depth)
	if err != nil {
		s.Logger.Debug("Failed to create listener: %v", err)
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.Listener = listener
	s.Addr = addrInfo.Addr
	s.Port = addrInfo.Port
	s.Logger.Debug("Listener created, bound to %s:%s", s.Addr, s.Port)

	// Update listen address with actual bound address
	if addrInfo.Port != "" {
		s.Listen = fmt.Sprintf("%s:%s", s.Addr, s.Port)
	} else {
		s.Listen = s.Addr
	}

	s.Logger.Log(1, "Listen on %s", s.Listen)

	// Define macros for the server
	s.Logger.Debug("Defining macros for server %s", s.Name)
	s.defineMacros()

	s.Running = true

	// Start accept loop in goroutine
	s.Logger.Debug("Starting accept loop for server %s", s.Name)
	s.wg.Add(1)
	go s.acceptLoop(processFunc)

	s.Logger.Debug("Server %s start completed", s.Name)
	return nil
}

// acceptLoop handles incoming connections
func (s *Server) acceptLoop(processFunc ProcessFunc) {
	defer s.wg.Done()
	s.Logger.Debug("Accept loop started for server %s", s.Name)

	for {
		select {
		case <-s.stopChan:
			s.Logger.Debug("Accept loop received stop signal for server %s", s.Name)
			return
		default:
		}

		s.Logger.Debug("Waiting to accept connection on server %s", s.Name)
		// Set a timeout on Accept so we can check stopChan periodically
		// Note: We'll use the raw listener for now
		conn, err := s.Listener.Accept()
		if err != nil {
			// Check if we're stopping
			select {
			case <-s.stopChan:
				s.Logger.Debug("Accept loop stopping after error (stop requested) for server %s", s.Name)
				return
			default:
				s.Logger.Error("Accept failed: %v", err)
				s.Logger.Debug("Continuing accept loop after error")
				continue
			}
		}

		// Log the accepted connection
		remoteAddr := gnet.GetRemoteAddr(conn)
		if remoteAddr.Port != "" {
			s.Logger.Log(3, "accepted connection from %s:%s", remoteAddr.Addr, remoteAddr.Port)
			s.Logger.Debug("Connection accepted from %s:%s on server %s", remoteAddr.Addr, remoteAddr.Port, s.Name)
		} else {
			s.Logger.Log(3, "accepted connection from %s", remoteAddr.Addr)
			s.Logger.Debug("Connection accepted from %s on server %s", remoteAddr.Addr, s.Name)
		}

		// Handle connection based on session settings
		if s.IsDispatch {
			// Dispatch mode: handle each connection in a new goroutine
			s.Logger.Debug("Handling connection in dispatch mode for server %s", s.Name)
			s.wg.Add(1)
			go s.handleConnection(conn, processFunc)
		} else {
			// Regular mode: handle in session (may use keepalive)
			s.Logger.Debug("Handling connection in session mode for server %s", s.Name)
			s.wg.Add(1)
			go s.handleSessionConnection(conn, processFunc)
		}
	}
}

// handleConnection processes a single connection (dispatch mode)
func (s *Server) handleConnection(conn net.Conn, processFunc ProcessFunc) {
	defer s.wg.Done()
	defer conn.Close()
	s.Logger.Debug("Starting connection handler (dispatch mode) for server %s", s.Name)

	if processFunc != nil {
		s.Logger.Debug("Calling processFunc for connection on server %s", s.Name)
		err := processFunc(conn, s.Spec, s.Listen)
		if err != nil {
			s.Logger.Error("Connection processing failed: %v", err)
			s.Logger.Debug("processFunc failed: %v", err)
		} else {
			s.Logger.Debug("processFunc completed successfully for server %s", s.Name)
		}
	}

	s.Logger.Log(3, "connection closed")
	s.Logger.Debug("Connection handler completed for server %s", s.Name)
}

// handleSessionConnection processes connections using session settings (repeat, keepalive)
func (s *Server) handleSessionConnection(conn net.Conn, processFunc ProcessFunc) {
	defer s.wg.Done()
	s.Logger.Debug("Starting session connection handler for server %s", s.Name)

	// Use the session's Run method
	connectFunc := func() (net.Conn, error) {
		// Connection already established
		s.Logger.Debug("Session connectFunc called (connection already established)")
		return conn, nil
	}

	disconnectFunc := func(c net.Conn) error {
		s.Logger.Debug("Session disconnectFunc called, closing connection")
		return c.Close()
	}

	procFunc := func(c net.Conn, spec string) (net.Conn, error) {
		if processFunc != nil {
			s.Logger.Debug("Session procFunc calling processFunc")
			err := processFunc(c, spec, s.Listen)
			if err != nil {
				s.Logger.Debug("Session processFunc returned error: %v", err)
			} else {
				s.Logger.Debug("Session processFunc completed successfully")
			}
			return c, err
		}
		return c, nil
	}

	s.Logger.Debug("Calling session.Run for server %s", s.Name)
	err := s.Session.Run(s.Spec, s.Listen, connectFunc, disconnectFunc, procFunc)
	if err != nil {
		s.Logger.Error("Session failed: %v", err)
		s.Logger.Debug("Session.Run failed: %v", err)
	} else {
		s.Logger.Debug("Session.Run completed successfully for server %s", s.Name)
	}

	// Increment connection counter and check if we should stop
	s.connCountMutex.Lock()
	s.connCount++
	count := s.connCount
	s.connCountMutex.Unlock()
	s.Logger.Debug("Connection count for server %s: %d/%d", s.Name, count, s.Session.Repeat)

	// If we've handled the expected number of connections, stop the server
	if !s.IsDispatch && count >= s.Session.Repeat {
		s.Logger.Log(2, "Ending")
		s.Logger.Debug("Reached expected connection count, stopping server %s", s.Name)
		go s.Stop() // Stop in goroutine to avoid deadlock
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
	s.Logger.Debug("Stop called for server %s", s.Name)

	s.mutex.Lock()
	if !s.Running {
		s.mutex.Unlock()
		s.Logger.Debug("Server %s not running, returning early", s.Name)
		return nil
	}
	s.mutex.Unlock()

	// Check if already stopping to prevent double-close of stopChan
	s.stoppingMutex.Lock()
	if s.stopping {
		s.stoppingMutex.Unlock()
		s.Logger.Debug("Server %s already stopping, returning early", s.Name)
		return nil
	}
	s.stopping = true
	s.stoppingMutex.Unlock()

	s.Logger.Log(2, "Stopping server %s", s.Name)

	// Signal stop
	s.Logger.Debug("Closing stop channel for server %s", s.Name)
	close(s.stopChan)

	// Close listener
	if s.Listener != nil {
		s.Logger.Debug("Closing listener for server %s", s.Name)
		s.Listener.Close()
	}

	// Wait for all connections to finish
	s.Logger.Debug("Waiting for connections to finish for server %s", s.Name)
	s.wg.Wait()
	s.Logger.Debug("All connections finished for server %s", s.Name)

	s.mutex.Lock()
	s.Running = false
	s.mutex.Unlock()

	// Undefine macros
	s.Logger.Debug("Undefining macros for server %s", s.Name)
	s.undefineMacros()

	s.Logger.Debug("Server %s stopped successfully", s.Name)
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

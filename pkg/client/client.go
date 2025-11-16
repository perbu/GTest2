// Package client provides client implementation for VTest2.
// Clients connect to servers (TCP or Unix sockets) and send/receive data.
package client

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/perbu/GTest/pkg/logging"
	gnet "github.com/perbu/GTest/pkg/net"
	"github.com/perbu/GTest/pkg/session"
)

// ProxyVersion represents the PROXY protocol version
type ProxyVersion int

const (
	// ProxyNone means no PROXY protocol
	ProxyNone ProxyVersion = 0
	// ProxyV1 is PROXY protocol version 1
	ProxyV1 ProxyVersion = 1
	// ProxyV2 is PROXY protocol version 2
	ProxyV2 ProxyVersion = 2
)

// ProcessFunc is called to process a client connection
type ProcessFunc func(conn net.Conn, spec string) error

// Client represents a client connection
type Client struct {
	Name         string
	Logger       *logging.Logger
	Session      *session.Session
	Spec         string
	ConnectAddr  string
	ProxySpec    string
	ProxyVersion ProxyVersion
	Running      bool

	// Internal
	stopChan chan struct{}
	wg       sync.WaitGroup
	mutex    sync.Mutex
	thread   *time.Timer
}

// New creates a new client with the given name
func New(logger *logging.Logger, name string) *Client {
	sessLogger := logging.NewLogger(name)
	sess := session.New(sessLogger, name)

	return &Client{
		Name:         name,
		Logger:       logger,
		Session:      sess,
		ConnectAddr:  "",
		ProxyVersion: ProxyNone,
		Running:      false,
		stopChan:     make(chan struct{}),
	}
}

// SetConnect sets the connection address
func (c *Client) SetConnect(addr string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ConnectAddr = addr
}

// SetProxy sets the PROXY protocol configuration
func (c *Client) SetProxy(version ProxyVersion, spec string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ProxyVersion = version
	c.ProxySpec = spec
}

// Connect establishes a connection to the server
func (c *Client) Connect() (net.Conn, error) {
	c.Logger.Debug("Connect called for client %s", c.Name)

	if c.ConnectAddr == "" {
		c.Logger.Debug("No connection address specified for client %s", c.Name)
		return nil, fmt.Errorf("no connection address specified")
	}

	c.Logger.Log(3, "Connect to %s", c.ConnectAddr)
	c.Logger.Debug("Attempting to connect to %s with 10s timeout", c.ConnectAddr)

	// Establish connection with timeout
	conn, err := gnet.TCPConnect(c.ConnectAddr, 10*time.Second)
	if err != nil {
		c.Logger.Debug("Connection failed to %s: %v", c.ConnectAddr, err)
		return nil, fmt.Errorf("failed to connect to %s: %w", c.ConnectAddr, err)
	}

	c.Logger.Log(3, "connected fd to %s", c.ConnectAddr)
	c.Logger.Debug("Successfully connected to %s", c.ConnectAddr)

	// Send PROXY protocol header if configured
	if c.ProxyVersion != ProxyNone && c.ProxySpec != "" {
		c.Logger.Debug("Sending PROXY v%d header", c.ProxyVersion)
		err = c.sendProxyHeader(conn)
		if err != nil {
			c.Logger.Debug("Failed to send PROXY header: %v", err)
			conn.Close()
			return nil, fmt.Errorf("failed to send PROXY header: %w", err)
		}
	}

	c.Logger.Debug("Connect completed successfully for client %s", c.Name)
	return conn, nil
}

// sendProxyHeader sends the PROXY protocol header
// TODO: Implement full PROXY protocol support in Phase 3
func (c *Client) sendProxyHeader(conn net.Conn) error {
	// For now, just log that PROXY protocol is not yet implemented
	c.Logger.Log(2, "PROXY protocol v%d not yet implemented (will be added in Phase 3)", c.ProxyVersion)
	return nil
}

// Start starts the client in a goroutine
func (c *Client) Start(processFunc ProcessFunc) error {
	c.mutex.Lock()
	if c.Running {
		c.mutex.Unlock()
		return fmt.Errorf("client %s already running", c.Name)
	}
	c.Running = true
	c.mutex.Unlock()

	c.Logger.Log(2, "Starting client %s", c.Name)

	c.wg.Add(1)
	go c.run(processFunc)

	return nil
}

// Run runs the client synchronously (blocking)
func (c *Client) Run(processFunc ProcessFunc) error {
	c.Logger.Log(2, "Running client %s", c.Name)
	c.Logger.Debug("Run called for client %s", c.Name)

	connectFunc := func() (net.Conn, error) {
		c.Logger.Debug("Session connectFunc calling Connect")
		return c.Connect()
	}

	disconnectFunc := func(conn net.Conn) error {
		c.Logger.Log(3, "closing connection")
		c.Logger.Debug("Session disconnectFunc closing connection")
		return conn.Close()
	}

	procFunc := func(conn net.Conn, spec string) (net.Conn, error) {
		if processFunc != nil {
			c.Logger.Debug("Session procFunc calling processFunc")
			err := processFunc(conn, spec)
			if err != nil {
				c.Logger.Debug("Session processFunc returned error: %v", err)
			} else {
				c.Logger.Debug("Session processFunc completed successfully")
			}
			return conn, err
		}
		return conn, nil
	}

	c.Logger.Debug("Calling session.Run for client %s", c.Name)
	err := c.Session.Run(c.Spec, c.ConnectAddr, connectFunc, disconnectFunc, procFunc)
	if err != nil {
		c.Logger.Debug("Session.Run failed: %v", err)
		return fmt.Errorf("client session failed: %w", err)
	}

	c.Logger.Debug("Client %s run completed successfully", c.Name)
	return nil
}

// run executes the client in a goroutine
func (c *Client) run(processFunc ProcessFunc) {
	defer c.wg.Done()
	defer func() {
		c.mutex.Lock()
		c.Running = false
		c.mutex.Unlock()
	}()

	err := c.Run(processFunc)
	if err != nil {
		c.Logger.Error("Client run failed: %v", err)
	}
}

// Wait waits for the client to complete
func (c *Client) Wait() {
	c.wg.Wait()
	c.mutex.Lock()
	c.Running = false
	c.mutex.Unlock()
}

// Stop stops the client
func (c *Client) Stop() error {
	c.mutex.Lock()
	if !c.Running {
		c.mutex.Unlock()
		return nil
	}
	c.mutex.Unlock()

	c.Logger.Log(2, "Stopping client %s", c.Name)

	// Signal stop
	select {
	case <-c.stopChan:
		// Already closed
	default:
		close(c.stopChan)
	}

	// Wait for the client to finish
	c.wg.Wait()

	c.mutex.Lock()
	c.Running = false
	c.mutex.Unlock()

	return nil
}

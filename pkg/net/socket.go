// Package net provides networking utilities for VTest2.
// This includes TCP and Unix domain socket operations, address parsing,
// and socket option management.
package net

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// AddressBufferSize is the maximum size for address strings
	AddressBufferSize = 64
	// PortBufferSize is the maximum size for port strings
	PortBufferSize = 16
)

// AddrInfo contains address and port information
type AddrInfo struct {
	Addr string
	Port string
}

// IsUnixSocket checks if the given path is a Unix socket path
func IsUnixSocket(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "@")
}

// ParseAddress parses an address string into host and port components.
// Supports formats: "host:port", "/path/to/socket", "@abstract-socket"
func ParseAddress(addr string) (host, port string, isUnix bool, err error) {
	if IsUnixSocket(addr) {
		return addr, "", true, nil
	}

	// Check for IPv6 addresses [host]:port
	if strings.HasPrefix(addr, "[") {
		endBracket := strings.Index(addr, "]")
		if endBracket == -1 {
			return "", "", false, fmt.Errorf("invalid IPv6 address format: %s", addr)
		}
		host = addr[1:endBracket]
		if len(addr) > endBracket+1 && addr[endBracket+1] == ':' {
			port = addr[endBracket+2:]
		}
		return host, port, false, nil
	}

	// Regular host:port format
	lastColon := strings.LastIndex(addr, ":")
	if lastColon == -1 {
		// No port specified
		return addr, "", false, nil
	}

	host = addr[:lastColon]
	port = addr[lastColon+1:]
	return host, port, false, nil
}

// TCPConnect establishes a TCP connection to the given address with timeout
func TCPConnect(addr string, timeout time.Duration) (net.Conn, error) {
	host, port, isUnix, err := ParseAddress(addr)
	if err != nil {
		return nil, err
	}

	if isUnix {
		return UnixConnect(host, timeout)
	}

	// Resolve the address
	var netAddr string
	if port != "" {
		netAddr = net.JoinHostPort(host, port)
	} else {
		netAddr = host
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.Dial("tcp", netAddr)
	if err != nil {
		return nil, fmt.Errorf("TCP connect to %s failed: %w", netAddr, err)
	}

	return conn, nil
}

// UnixConnect establishes a Unix domain socket connection with timeout
func UnixConnect(path string, timeout time.Duration) (net.Conn, error) {
	network := "unix"
	addr := path

	// Handle abstract sockets (Linux-specific)
	if strings.HasPrefix(path, "@") {
		addr = "\x00" + path[1:]
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.Dial(network, addr)
	if err != nil {
		return nil, fmt.Errorf("Unix connect to %s failed: %w", path, err)
	}

	return conn, nil
}

// TCPListen creates a TCP listening socket on the given address
func TCPListen(addr string, backlog int) (net.Listener, *AddrInfo, error) {
	host, port, isUnix, err := ParseAddress(addr)
	if err != nil {
		return nil, nil, err
	}

	if isUnix {
		return UnixListen(host, backlog)
	}

	// If no port specified, use random port
	if port == "" {
		port = "0"
	}

	listenAddr := net.JoinHostPort(host, port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("TCP listen on %s failed: %w", listenAddr, err)
	}

	// Get the actual address
	tcpAddr := listener.Addr().(*net.TCPAddr)
	addrInfo := &AddrInfo{
		Addr: tcpAddr.IP.String(),
		Port: strconv.Itoa(tcpAddr.Port),
	}

	return listener, addrInfo, nil
}

// UnixListen creates a Unix domain socket listening on the given path
func UnixListen(path string, backlog int) (net.Listener, *AddrInfo, error) {
	network := "unix"
	addr := path

	// Handle abstract sockets (Linux-specific)
	if strings.HasPrefix(path, "@") {
		addr = "\x00" + path[1:]
	}

	listener, err := net.Listen(network, addr)
	if err != nil {
		return nil, nil, fmt.Errorf("Unix listen on %s failed: %w", path, err)
	}

	addrInfo := &AddrInfo{
		Addr: path,
		Port: "",
	}

	return listener, addrInfo, nil
}

// SetReceiveBuffer sets the receive buffer size for a connection
func SetReceiveBuffer(conn net.Conn, size int) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		rawConn, err := tcpConn.SyscallConn()
		if err != nil {
			return err
		}

		var setErr error
		err = rawConn.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, size)
		})
		if err != nil {
			return err
		}
		return setErr
	}

	return nil
}

// SetBlocking sets the blocking mode for a connection
func SetBlocking(conn net.Conn, blocking bool) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		rawConn, err := tcpConn.SyscallConn()
		if err != nil {
			return err
		}

		var setErr error
		err = rawConn.Control(func(fd uintptr) {
			setErr = syscall.SetNonblock(int(fd), !blocking)
		})
		if err != nil {
			return err
		}
		return setErr
	}

	return nil
}

// GetLocalAddr returns the local address and port of a connection
func GetLocalAddr(conn net.Conn) *AddrInfo {
	addr := conn.LocalAddr()

	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return &AddrInfo{
			Addr: tcpAddr.IP.String(),
			Port: strconv.Itoa(tcpAddr.Port),
		}
	}

	if unixAddr, ok := addr.(*net.UnixAddr); ok {
		return &AddrInfo{
			Addr: unixAddr.Name,
			Port: "",
		}
	}

	return &AddrInfo{
		Addr: addr.String(),
		Port: "",
	}
}

// GetRemoteAddr returns the remote address and port of a connection
func GetRemoteAddr(conn net.Conn) *AddrInfo {
	addr := conn.RemoteAddr()

	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return &AddrInfo{
			Addr: tcpAddr.IP.String(),
			Port: strconv.Itoa(tcpAddr.Port),
		}
	}

	if unixAddr, ok := addr.(*net.UnixAddr); ok {
		return &AddrInfo{
			Addr: unixAddr.Name,
			Port: "",
		}
	}

	return &AddrInfo{
		Addr: addr.String(),
		Port: "",
	}
}

// SetReadTimeout sets the read timeout for a connection
func SetReadTimeout(conn net.Conn, timeout time.Duration) error {
	if timeout > 0 {
		return conn.SetReadDeadline(time.Now().Add(timeout))
	}
	return conn.SetReadDeadline(time.Time{})
}

// SetWriteTimeout sets the write timeout for a connection
func SetWriteTimeout(conn net.Conn, timeout time.Duration) error {
	if timeout > 0 {
		return conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	return conn.SetWriteDeadline(time.Time{})
}

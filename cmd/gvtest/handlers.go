// Package main provides command handlers for VTC commands
package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/perbu/GTest/pkg/client"
	"github.com/perbu/GTest/pkg/http1"
	"github.com/perbu/GTest/pkg/logging"
	"github.com/perbu/GTest/pkg/server"
	"github.com/perbu/GTest/pkg/vtc"
)

// RegisterBuiltinCommands registers all built-in VTC commands
func RegisterBuiltinCommands() {
	// Register client and server commands (Phase 2+)
	vtc.RegisterCommand("client", cmdClient, vtc.FlagNone)
	vtc.RegisterCommand("server", cmdServer, vtc.FlagNone)
}

// nodeToSpec converts AST child nodes to a spec string
func nodeToSpec(children []*vtc.Node) string {
	var lines []string
	for _, child := range children {
		if child.Type == "command" {
			line := child.Name
			if len(child.Args) > 0 {
				line += " " + strings.Join(child.Args, " ")
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// createHTTP1ProcessFunc creates a processFunc for HTTP/1 server connections
func createHTTP1ProcessFunc(spec string) server.ProcessFunc {
	return func(conn net.Conn, specStr string, listenAddr string) error {
		logger := logging.NewLogger("http")
		h := http1.New(conn, logger)
		handler := http1.NewHandler(h)
		return handler.ProcessSpec(spec)
	}
}

// createHTTP1ClientProcessFunc creates a processFunc for HTTP/1 client connections
func createHTTP1ClientProcessFunc(spec string) client.ProcessFunc {
	return func(conn net.Conn, specStr string) error {
		logger := logging.NewLogger("http")
		h := http1.New(conn, logger)
		handler := http1.NewHandler(h)
		return handler.ProcessSpec(spec)
	}
}

// cmdClient implements the "client" command
func cmdClient(args []string, priv interface{}, logger *logging.Logger) error {
	logger.Debug("cmdClient called with args: %v", args)

	ctx, ok := priv.(*vtc.ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for client command")
	}

	if len(args) == 0 {
		return fmt.Errorf("client: missing client name")
	}

	clientName := args[0]
	args = args[1:]
	logger.Debug("Client name: %s, remaining args: %v", clientName, args)

	// Validate client name starts with 'c'
	if len(clientName) == 0 || clientName[0] != 'c' {
		return fmt.Errorf("client name must start with 'c' (got %s)", clientName)
	}

	// Get or create client
	var c *client.Client
	if existing, ok := ctx.Clients[clientName]; ok {
		c = existing.(*client.Client)
		logger.Debug("Using existing client: %s", clientName)
	} else {
		c = client.New(logger, clientName)
		ctx.Clients[clientName] = c
		logger.Debug("Created new client: %s", clientName)
	}

	// Convert child nodes to spec if present
	if ctx.CurrentNode != nil && len(ctx.CurrentNode.Children) > 0 {
		c.Spec = nodeToSpec(ctx.CurrentNode.Children)
		logger.Debug("Set client spec from child nodes, length: %d", len(c.Spec))
	}

	// Parse command options
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-connect":
			if i+1 >= len(args) {
				return fmt.Errorf("client: -connect requires an argument")
			}
			i++
			addr, err := ctx.Macros.Expand(logger, args[i])
			if err != nil {
				return fmt.Errorf("client: -connect macro expansion failed: %w", err)
			}
			c.SetConnect(addr)

		case "-start":
			// Start client in background
			logger.Debug("Client %s: processing -start flag", clientName)
			processFunc := createHTTP1ClientProcessFunc(c.Spec)
			err := c.Start(processFunc)
			if err != nil {
				logger.Debug("Client %s: -start failed: %v", clientName, err)
				return fmt.Errorf("client: -start failed: %w", err)
			}
			logger.Debug("Client %s: -start completed", clientName)

		case "-wait":
			// Wait for client to complete
			logger.Debug("Client %s: processing -wait flag", clientName)
			c.Wait()
			logger.Debug("Client %s: -wait completed", clientName)

		case "-run":
			// Run client synchronously
			logger.Debug("Client %s: processing -run flag", clientName)
			processFunc := createHTTP1ClientProcessFunc(c.Spec)
			err := c.Run(processFunc)
			if err != nil {
				logger.Debug("Client %s: -run failed: %v", clientName, err)
				return fmt.Errorf("client: -run failed: %w", err)
			}
			logger.Debug("Client %s: -run completed", clientName)

		case "-repeat":
			if i+1 >= len(args) {
				return fmt.Errorf("client: -repeat requires an argument")
			}
			i++
			consumed, err := c.Session.ParseOption([]string{arg, args[i]})
			if err != nil {
				return fmt.Errorf("client: %w", err)
			}
			if consumed == 0 {
				return fmt.Errorf("client: failed to parse -repeat")
			}

		case "-keepalive":
			_, err := c.Session.ParseOption([]string{arg})
			if err != nil {
				return fmt.Errorf("client: %w", err)
			}

		case "-rcvbuf":
			if i+1 >= len(args) {
				return fmt.Errorf("client: -rcvbuf requires an argument")
			}
			i++
			consumed, err := c.Session.ParseOption([]string{arg, args[i]})
			if err != nil {
				return fmt.Errorf("client: %w", err)
			}
			if consumed == 0 {
				return fmt.Errorf("client: failed to parse -rcvbuf")
			}

		case "-proxy1":
			if i+1 >= len(args) {
				return fmt.Errorf("client: -proxy1 requires an argument")
			}
			i++
			c.SetProxy(client.ProxyV1, args[i])

		case "-proxy2":
			if i+1 >= len(args) {
				return fmt.Errorf("client: -proxy2 requires an argument")
			}
			i++
			c.SetProxy(client.ProxyV2, args[i])

		default:
			if arg[0] == '-' {
				return fmt.Errorf("client: unknown option: %s", arg)
			}
			// This is the spec (command script)
			c.Spec = arg
		}
	}

	return nil
}

// cmdServer implements the "server" command
func cmdServer(args []string, priv interface{}, logger *logging.Logger) error {
	logger.Debug("cmdServer called with args: %v", args)

	ctx, ok := priv.(*vtc.ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for server command")
	}

	if len(args) == 0 {
		return fmt.Errorf("server: missing server name")
	}

	serverName := args[0]
	args = args[1:]
	logger.Debug("Server name: %s, remaining args: %v", serverName, args)

	// Validate server name starts with 's'
	if len(serverName) == 0 || serverName[0] != 's' {
		return fmt.Errorf("server name must start with 's' (got %s)", serverName)
	}

	// Get or create server
	var s *server.Server
	if existing, ok := ctx.Servers[serverName]; ok {
		s = existing.(*server.Server)
		logger.Debug("Using existing server: %s", serverName)
	} else {
		s = server.New(logger, ctx.Macros, serverName)
		ctx.Servers[serverName] = s
		logger.Debug("Created new server: %s", serverName)
	}

	// Convert child nodes to spec if present
	if ctx.CurrentNode != nil && len(ctx.CurrentNode.Children) > 0 {
		s.Spec = nodeToSpec(ctx.CurrentNode.Children)
		logger.Debug("Set server spec from child nodes, length: %d", len(s.Spec))
	}

	// Parse command options
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-listen":
			if i+1 >= len(args) {
				return fmt.Errorf("server: -listen requires an argument")
			}
			i++
			addr, err := ctx.Macros.Expand(logger, args[i])
			if err != nil {
				return fmt.Errorf("server: -listen macro expansion failed: %w", err)
			}
			s.SetListen(addr)

		case "-start":
			// Start server with HTTP/1 processFunc
			logger.Debug("Server %s: processing -start flag", serverName)
			processFunc := createHTTP1ProcessFunc(s.Spec)
			err := s.Start(processFunc)
			if err != nil {
				logger.Debug("Server %s: -start failed: %v", serverName, err)
				return fmt.Errorf("server: -start failed: %w", err)
			}
			logger.Debug("Server %s: -start completed", serverName)

		case "-wait":
			// Wait for server to stop
			logger.Debug("Server %s: processing -wait flag", serverName)
			s.Wait()
			logger.Debug("Server %s: -wait completed", serverName)

		case "-break":
			// Force stop the server
			logger.Debug("Server %s: processing -break flag", serverName)
			err := s.Break()
			if err != nil {
				logger.Debug("Server %s: -break failed: %v", serverName, err)
				return fmt.Errorf("server: -break failed: %w", err)
			}
			logger.Debug("Server %s: -break completed", serverName)

		case "-dispatch":
			// Enable dispatch mode (only for s0)
			logger.Debug("Server %s: processing -dispatch flag", serverName)
			if serverName != "s0" {
				return fmt.Errorf("server: -dispatch only works on s0")
			}
			s.IsDispatch = true
			processFunc := createHTTP1ProcessFunc(s.Spec)
			err := s.Start(processFunc)
			if err != nil {
				logger.Debug("Server %s: -dispatch failed: %v", serverName, err)
				return fmt.Errorf("server: -dispatch failed: %w", err)
			}
			logger.Debug("Server %s: -dispatch completed", serverName)

		case "-repeat":
			if i+1 >= len(args) {
				return fmt.Errorf("server: -repeat requires an argument")
			}
			i++
			consumed, err := s.Session.ParseOption([]string{arg, args[i]})
			if err != nil {
				return fmt.Errorf("server: %w", err)
			}
			if consumed == 0 {
				return fmt.Errorf("server: failed to parse -repeat")
			}

		case "-keepalive":
			_, err := s.Session.ParseOption([]string{arg})
			if err != nil {
				return fmt.Errorf("server: %w", err)
			}

		case "-rcvbuf":
			if i+1 >= len(args) {
				return fmt.Errorf("server: -rcvbuf requires an argument")
			}
			i++
			consumed, err := s.Session.ParseOption([]string{arg, args[i]})
			if err != nil {
				return fmt.Errorf("server: %w", err)
			}
			if consumed == 0 {
				return fmt.Errorf("server: failed to parse -rcvbuf")
			}

		default:
			if arg[0] == '-' {
				return fmt.Errorf("server: unknown option: %s", arg)
			}
			// This is the spec (command script)
			s.Spec = arg
		}
	}

	return nil
}

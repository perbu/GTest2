// Package main provides command handlers for VTC commands
package main

import (
	"fmt"

	"github.com/perbu/gvtest/pkg/client"
	"github.com/perbu/gvtest/pkg/logging"
	"github.com/perbu/gvtest/pkg/server"
	"github.com/perbu/gvtest/pkg/vtc"
)

// RegisterBuiltinCommands registers all built-in VTC commands
func RegisterBuiltinCommands() {
	// Register client and server commands (Phase 2+)
	vtc.RegisterCommand("client", cmdClient, vtc.FlagNone)
	vtc.RegisterCommand("server", cmdServer, vtc.FlagNone)
}

// cmdClient implements the "client" command
func cmdClient(args []string, priv interface{}, logger *logging.Logger) error {
	ctx, ok := priv.(*vtc.ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for client command")
	}

	if len(args) == 0 {
		return fmt.Errorf("client: missing client name")
	}

	clientName := args[0]
	args = args[1:]

	// Validate client name starts with 'c'
	if len(clientName) == 0 || clientName[0] != 'c' {
		return fmt.Errorf("client name must start with 'c' (got %s)", clientName)
	}

	// Get or create client
	var c *client.Client
	if existing, ok := ctx.Clients[clientName]; ok {
		c = existing.(*client.Client)
	} else {
		c = client.New(logger, clientName)
		ctx.Clients[clientName] = c
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
			err := c.Start(nil) // processFunc will be added in Phase 3
			if err != nil {
				return fmt.Errorf("client: -start failed: %w", err)
			}

		case "-wait":
			// Wait for client to complete
			c.Wait()

		case "-run":
			// Run client synchronously
			err := c.Run(nil) // processFunc will be added in Phase 3
			if err != nil {
				return fmt.Errorf("client: -run failed: %w", err)
			}

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
	ctx, ok := priv.(*vtc.ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for server command")
	}

	if len(args) == 0 {
		return fmt.Errorf("server: missing server name")
	}

	serverName := args[0]
	args = args[1:]

	// Validate server name starts with 's'
	if len(serverName) == 0 || serverName[0] != 's' {
		return fmt.Errorf("server name must start with 's' (got %s)", serverName)
	}

	// Get or create server
	var s *server.Server
	if existing, ok := ctx.Servers[serverName]; ok {
		s = existing.(*server.Server)
	} else {
		s = server.New(logger, ctx.Macros, serverName)
		ctx.Servers[serverName] = s
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
			// Start server
			err := s.Start(nil) // processFunc will be added in Phase 3
			if err != nil {
				return fmt.Errorf("server: -start failed: %w", err)
			}

		case "-wait":
			// Wait for server to stop
			s.Wait()

		case "-break":
			// Force stop the server
			err := s.Break()
			if err != nil {
				return fmt.Errorf("server: -break failed: %w", err)
			}

		case "-dispatch":
			// Enable dispatch mode (only for s0)
			if serverName != "s0" {
				return fmt.Errorf("server: -dispatch only works on s0")
			}
			s.IsDispatch = true
			err := s.Start(nil)
			if err != nil {
				return fmt.Errorf("server: -dispatch failed: %w", err)
			}

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

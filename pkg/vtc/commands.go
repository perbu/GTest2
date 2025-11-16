// Package vtc provides command registration and execution for VTC language
package vtc

import (
	"fmt"
	"sync"

	"github.com/perbu/GTest/pkg/logging"
)

// CommandFlags represents command execution flags
type CommandFlags uint

const (
	// FlagNone indicates no special flags
	FlagNone CommandFlags = 0
	// FlagGlobal indicates a global command (can be used anywhere)
	FlagGlobal CommandFlags = 1 << 0
	// FlagShutdown indicates the command is valid during shutdown
	FlagShutdown CommandFlags = 1 << 1
)

// CommandFunc is a function that executes a VTC command
// args: command arguments (excluding the command name itself)
// priv: private data (context-specific)
// logger: logger for the command
type CommandFunc func(args []string, priv interface{}, logger *logging.Logger) error

// Command represents a registered VTC command
type Command struct {
	Name  string
	Func  CommandFunc
	Flags CommandFlags
}

// CommandRegistry manages registered VTC commands
type CommandRegistry struct {
	commands map[string]*Command
	mutex    sync.RWMutex
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*Command),
	}
}

// Register registers a new command
func (r *CommandRegistry) Register(name string, fn CommandFunc, flags CommandFlags) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.commands[name] = &Command{
		Name:  name,
		Func:  fn,
		Flags: flags,
	}
}

// Get retrieves a command by name
func (r *CommandRegistry) Get(name string) (*Command, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	cmd, ok := r.commands[name]
	return cmd, ok
}

// Execute executes a command by name
func (r *CommandRegistry) Execute(name string, args []string, priv interface{}, logger *logging.Logger) error {
	cmd, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("unknown command: %s", name)
	}

	return cmd.Func(args, priv, logger)
}

// List returns all registered command names
func (r *CommandRegistry) List() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

// IsGlobal checks if a command is global
func (r *CommandRegistry) IsGlobal(name string) bool {
	cmd, ok := r.Get(name)
	if !ok {
		return false
	}
	return cmd.Flags&FlagGlobal != 0
}

// GlobalRegistry is the global command registry
var GlobalRegistry = NewCommandRegistry()

// RegisterCommand registers a command in the global registry
func RegisterCommand(name string, fn CommandFunc, flags CommandFlags) {
	GlobalRegistry.Register(name, fn, flags)
}

// GetCommand retrieves a command from the global registry
func GetCommand(name string) (*Command, bool) {
	return GlobalRegistry.Get(name)
}

// ExecuteCommand executes a command from the global registry
func ExecuteCommand(name string, args []string, priv interface{}, logger *logging.Logger) error {
	return GlobalRegistry.Execute(name, args, priv, logger)
}

// ListCommands returns all registered command names
func ListCommands() []string {
	return GlobalRegistry.List()
}

// Executor manages command execution with context
type Executor struct {
	Registry *CommandRegistry
	Logger   *logging.Logger
	Macros   *MacroStore
	Context  interface{} // Context-specific data
}

// NewExecutor creates a new command executor
func NewExecutor(logger *logging.Logger, macros *MacroStore) *Executor {
	return &Executor{
		Registry: GlobalRegistry,
		Logger:   logger,
		Macros:   macros,
	}
}

// Execute executes a command with macro expansion
func (e *Executor) Execute(cmdLine string) error {
	// Expand macros in the command line
	expanded, err := e.Macros.Expand(e.Logger, cmdLine)
	if err != nil {
		return fmt.Errorf("macro expansion failed: %w", err)
	}

	// Parse the command line into tokens
	tokens := tokenize(expanded)
	if len(tokens) == 0 {
		return nil // Empty line
	}

	cmdName := tokens[0]
	args := tokens[1:]

	// Execute the command
	return e.Registry.Execute(cmdName, args, e.Context, e.Logger)
}

// tokenize splits a command line into tokens
// This is a simple implementation; a more robust one would handle quotes, escapes, etc.
func tokenize(line string) []string {
	// For now, just split on whitespace
	// TODO: Implement proper tokenization with quote handling
	var tokens []string
	var current string
	inQuote := false

	for _, ch := range line {
		switch ch {
		case ' ', '\t':
			if inQuote {
				current += string(ch)
			} else if len(current) > 0 {
				tokens = append(tokens, current)
				current = ""
			}
		case '"':
			inQuote = !inQuote
		default:
			current += string(ch)
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, current)
	}

	return tokens
}

// Package vtc provides VTC test execution
package vtc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/perbu/GTest/pkg/logging"
)

// ExecContext holds the execution context for a VTC test
type ExecContext struct {
	Macros      *MacroStore
	Logger      *logging.Logger
	TmpDir      string
	Timeout     time.Duration
	Failed      bool
	Skipped     bool
	SkipReason  string
	Clients     map[string]interface{} // Will be *client.Client
	Servers     map[string]interface{} // Will be *server.Server
	Barriers    map[string]interface{} // Will be *barrier.Barrier
	Processes   map[string]interface{} // Will be *process.Process
}

// NewExecContext creates a new execution context
func NewExecContext(logger *logging.Logger, macros *MacroStore, tmpDir string, timeout time.Duration) *ExecContext {
	return &ExecContext{
		Macros:    macros,
		Logger:    logger,
		TmpDir:    tmpDir,
		Timeout:   timeout,
		Clients:   make(map[string]interface{}),
		Servers:   make(map[string]interface{}),
		Barriers:  make(map[string]interface{}),
		Processes: make(map[string]interface{}),
	}
}

// Fail marks the test as failed
func (ctx *ExecContext) Fail(format string, args ...interface{}) {
	ctx.Failed = true
	ctx.Logger.Error(format, args...)
}

// Skip marks the test as skipped
func (ctx *ExecContext) Skip(reason string) {
	ctx.Skipped = true
	ctx.SkipReason = reason
	ctx.Logger.Info("Test skipped: %s", reason)
}

// TestExecutor executes a parsed VTC test
type TestExecutor struct {
	Context  *ExecContext
	Registry *CommandRegistry
}

// NewTestExecutor creates a new test executor
func NewTestExecutor(ctx *ExecContext, registry *CommandRegistry) *TestExecutor {
	return &TestExecutor{
		Context:  ctx,
		Registry: registry,
	}
}

// Execute runs a parsed VTC test
func (e *TestExecutor) Execute(ast *Node) error {
	// Walk the AST and execute each command
	for _, node := range ast.Children {
		if e.Context.Failed {
			return fmt.Errorf("test failed")
		}
		if e.Context.Skipped {
			return nil // Not an error, just skipped
		}

		// Execute the node
		if err := e.executeNode(node); err != nil {
			e.Context.Fail("Command failed: %v", err)
			return err
		}
	}

	return nil
}

// executeNode executes a single AST node
func (e *TestExecutor) executeNode(node *Node) error {
	// Handle different node types
	switch node.Type {
	case "vtest":
		// Test description - just log it
		e.Context.Logger.Info("Test: %s", node.Name)
		return nil

	case "command":
		// Generic command - look up by Name
		cmdName := node.Name
		args := node.Args

		// Execute the command
		return e.Registry.Execute(cmdName, args, e.Context, e.Context.Logger)

	case "comment":
		// Ignore comments
		return nil

	default:
		return fmt.Errorf("unknown node type: %s", node.Type)
	}
}

// RunTest executes a VTC test file
func RunTest(testFile string, logger *logging.Logger, macros *MacroStore, keepTmp bool, timeout time.Duration) (exitCode int, err error) {
	// Create temporary directory for this test
	tmpDir, err := os.MkdirTemp("", "gvtest-*")
	if err != nil {
		return 2, fmt.Errorf("failed to create temp dir: %w", err)
	}

	if !keepTmp {
		defer os.RemoveAll(tmpDir)
	} else {
		logger.Info("Keeping temp directory: %s", tmpDir)
	}

	// Set up tmpdir macro
	macros.Define("tmpdir", tmpDir)

	// Open and parse the test file
	f, err := os.Open(testFile)
	if err != nil {
		return 2, fmt.Errorf("failed to open test file: %w", err)
	}
	defer f.Close()

	parser := NewParser(f, macros, logger)
	ast, err := parser.Parse()
	if err != nil {
		return 2, fmt.Errorf("parse error: %w", err)
	}

	// Create execution context
	ctx := NewExecContext(logger, macros, tmpDir, timeout)

	// Create executor
	executor := NewTestExecutor(ctx, GlobalRegistry)

	// Execute the test
	if err := executor.Execute(ast); err != nil {
		if ctx.Skipped {
			return 77, nil // Skip exit code
		}
		return 1, err // Fail exit code
	}

	if ctx.Failed {
		return 1, fmt.Errorf("test failed")
	}

	if ctx.Skipped {
		return 77, nil
	}

	return 0, nil // Pass
}

// SetupDefaultMacros sets up default macros for a test
func SetupDefaultMacros(macros *MacroStore, testFile string) {
	absPath, _ := filepath.Abs(testFile)
	testDir := filepath.Dir(absPath)
	testName := filepath.Base(testFile)

	macros.Define("testdir", testDir)
	macros.Define("testfile", testName)
	macros.Define("tmpdir", "/tmp") // Will be overridden when test runs

	// Platform-specific macros
	macros.Define("platform", "linux")
	macros.Define("os", "Linux")

	// Version info
	macros.Define("version", "gvtest-0.1.0")
}

// ParseTestFile is a utility function to just parse a test file
func ParseTestFile(testFile string, logger *logging.Logger, macros *MacroStore) (*Node, error) {
	f, err := os.Open(testFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	parser := NewParser(f, macros, logger)
	return parser.Parse()
}

// ParseTestReader parses a VTC test from a reader
func ParseTestReader(r io.Reader, logger *logging.Logger, macros *MacroStore) (*Node, error) {
	parser := NewParser(r, macros, logger)
	return parser.Parse()
}

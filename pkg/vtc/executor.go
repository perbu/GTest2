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
	Macros       *MacroStore
	Logger       *logging.Logger
	TmpDir       string
	Timeout      time.Duration
	Failed       bool
	Skipped      bool
	SkipReason   string
	Clients      map[string]interface{} // Will be *client.Client
	Servers      map[string]interface{} // Will be *server.Server
	Barriers     map[string]interface{} // Will be *barrier.Barrier
	Processes    map[string]interface{} // Will be *process.Process
	CurrentNode  *Node                  // Current AST node being executed
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
	e.Context.Logger.Debug("Starting test execution with %d top-level nodes", len(ast.Children))

	// Walk the AST and execute each command
	for i, node := range ast.Children {
		if e.Context.Failed {
			e.Context.Logger.Debug("Test marked as failed, stopping execution")
			return fmt.Errorf("test failed")
		}
		if e.Context.Skipped {
			e.Context.Logger.Debug("Test marked as skipped, stopping execution")
			return nil // Not an error, just skipped
		}

		e.Context.Logger.Debug("Executing node %d/%d: type=%s name=%s", i+1, len(ast.Children), node.Type, node.Name)

		// Execute the node
		if err := e.executeNode(node); err != nil {
			e.Context.Logger.Debug("Node execution failed: %v", err)
			e.Context.Fail("Command failed: %v", err)
			return err
		}

		e.Context.Logger.Debug("Node %d/%d completed successfully", i+1, len(ast.Children))
	}

	e.Context.Logger.Debug("Test execution completed successfully")
	return nil
}

// executeNode executes a single AST node
func (e *TestExecutor) executeNode(node *Node) error {
	// Handle different node types
	switch node.Type {
	case "vtest":
		// Test description - just log it
		e.Context.Logger.Info("Test: %s", node.Name)
		e.Context.Logger.Debug("Test description node processed")
		return nil

	case "command":
		// Generic command - look up by Name
		cmdName := node.Name
		args := node.Args

		e.Context.Logger.Debug("Executing command: %s with %d args", cmdName, len(args))
		if len(args) > 0 {
			e.Context.Logger.Debug("Command args: %v", args)
		}

		// Set current node in context so command handlers can access children
		e.Context.CurrentNode = node

		// Execute the command
		err := e.Registry.Execute(cmdName, args, e.Context, e.Context.Logger)
		if err != nil {
			e.Context.Logger.Debug("Command %s failed: %v", cmdName, err)
		} else {
			e.Context.Logger.Debug("Command %s completed successfully", cmdName)
		}
		return err

	case "comment":
		// Ignore comments
		e.Context.Logger.Debug("Skipping comment node")
		return nil

	default:
		return fmt.Errorf("unknown node type: %s", node.Type)
	}
}

// RunTest executes a VTC test file
func RunTest(testFile string, logger *logging.Logger, macros *MacroStore, keepTmp bool, timeout time.Duration) (exitCode int, err error) {
	logger.Debug("RunTest starting for file: %s", testFile)
	logger.Debug("Timeout: %v, keepTmp: %v", timeout, keepTmp)

	// Create temporary directory for this test
	tmpDir, err := os.MkdirTemp("", "gvtest-*")
	if err != nil {
		logger.Debug("Failed to create temp dir: %v", err)
		return 2, fmt.Errorf("failed to create temp dir: %w", err)
	}
	logger.Debug("Created temp directory: %s", tmpDir)

	if !keepTmp {
		defer os.RemoveAll(tmpDir)
	} else {
		logger.Info("Keeping temp directory: %s", tmpDir)
	}

	// Set up tmpdir macro
	macros.Define("tmpdir", tmpDir)
	logger.Debug("Defined tmpdir macro: %s", tmpDir)

	// Open and parse the test file
	logger.Debug("Opening test file: %s", testFile)
	f, err := os.Open(testFile)
	if err != nil {
		logger.Debug("Failed to open test file: %v", err)
		return 2, fmt.Errorf("failed to open test file: %w", err)
	}
	defer f.Close()

	logger.Debug("Parsing test file...")
	parser := NewParser(f, macros, logger)
	ast, err := parser.Parse()
	if err != nil {
		logger.Debug("Parse error: %v", err)
		return 2, fmt.Errorf("parse error: %w", err)
	}
	logger.Debug("Parse completed, AST has %d children", len(ast.Children))

	// Create execution context
	logger.Debug("Creating execution context")
	ctx := NewExecContext(logger, macros, tmpDir, timeout)

	// Create executor
	logger.Debug("Creating test executor")
	executor := NewTestExecutor(ctx, GlobalRegistry)

	// Execute the test
	logger.Debug("Beginning test execution")
	if err := executor.Execute(ast); err != nil {
		if ctx.Skipped {
			logger.Debug("Test skipped, returning exit code 77")
			return 77, nil // Skip exit code
		}
		logger.Debug("Test execution failed: %v", err)
		return 1, err // Fail exit code
	}

	if ctx.Failed {
		logger.Debug("Test marked as failed, returning exit code 1")
		return 1, fmt.Errorf("test failed")
	}

	if ctx.Skipped {
		logger.Debug("Test skipped, returning exit code 77")
		return 77, nil
	}

	logger.Debug("Test passed, returning exit code 0")
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

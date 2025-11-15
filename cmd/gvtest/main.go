// gvtest - Go port of VTest2 (Varnishtest)
// HTTP testing framework with byte-level control for malformed traffic generation
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/perbu/gvtest/pkg/logging"
	"github.com/perbu/gvtest/pkg/vtc"
)

var (
	verbose   = flag.Bool("v", false, "Verbose output")
	quiet     = flag.Bool("q", false, "Quiet mode")
	keepTmp   = flag.Bool("k", false, "Keep temp directories")
	jobs      = flag.Int("j", 1, "Number of parallel jobs")
	timeout   = flag.Int("t", 60, "Test timeout in seconds")
	dumpAST   = flag.Bool("dump-ast", false, "Dump AST and exit")
	version   = flag.Bool("version", false, "Show version")
)

const (
	versionString = "gvtest 0.1.0 (Phase 1)"
	exitPass      = 0
	exitFail      = 1
	exitSkip      = 77
	exitError     = 2
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println(versionString)
		os.Exit(exitPass)
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] test.vtc [test2.vtc ...]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(exitError)
	}

	// Process each test file
	exitCode := exitPass
	for _, testFile := range args {
		result := runTest(testFile)
		if result != exitPass {
			exitCode = result
		}
	}

	os.Exit(exitCode)
}

func runTest(testFile string) int {
	// Create logger
	testName := filepath.Base(testFile)
	logger := logging.NewLogger(testName)

	if !*quiet {
		logger.Info("Running test: %s", testFile)
	}

	// Open test file
	f, err := os.Open(testFile)
	if err != nil {
		logger.Error("Failed to open test file: %v", err)
		return exitError
	}
	defer f.Close()

	// Create macro store with default macros
	macros := vtc.NewMacroStore()
	setupDefaultMacros(macros, testFile)

	// Parse the test file
	parser := vtc.NewParser(f, macros, logger)
	ast, err := parser.Parse()
	if err != nil {
		logger.Error("Parse error: %v", err)
		return exitError
	}

	// Dump AST if requested
	if *dumpAST {
		vtc.DumpAST(ast, 0)
		return exitPass
	}

	// Validate AST structure
	if len(ast.Children) == 0 {
		logger.Error("Empty test file")
		return exitError
	}

	// Check for vtest declaration
	hasVTest := false
	for _, child := range ast.Children {
		if child.Type == "vtest" {
			hasVTest = true
			if !*quiet {
				logger.Info("Test: %s", child.Name)
			}
			break
		}
	}

	if !hasVTest {
		logger.Warning("No vtest declaration found")
	}

	// Phase 1: Just parse and validate, don't execute
	if *verbose {
		logger.Info("Parsed successfully with %d top-level nodes", len(ast.Children))
		for i, child := range ast.Children {
			logger.Debug("Node %d: %s %s (args: %d)", i, child.Type, child.Name, len(child.Args))
		}
	}

	if !*quiet {
		logger.Info("Test parsed successfully")
		fmt.Printf("✓ %s\n", testName)
	}

	return exitPass
}

func setupDefaultMacros(macros *vtc.MacroStore, testFile string) {
	// Set up default macros that would be useful
	absPath, _ := filepath.Abs(testFile)
	testDir := filepath.Dir(absPath)
	testName := filepath.Base(testFile)

	macros.Define("testdir", testDir)
	macros.Define("testfile", testName)
	macros.Define("tmpdir", "/tmp") // Will be created per-test in later phases

	// Platform-specific macros
	macros.Define("platform", "linux")
	macros.Define("os", "Linux")

	// Version info
	macros.Define("version", "gvtest-0.1.0")
}

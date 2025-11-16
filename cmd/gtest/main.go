// gtest - Go port of VTest2 (Varnishtest)
// HTTP testing framework with byte-level control for malformed traffic generation
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/perbu/GTest/pkg/logging"
	"github.com/perbu/GTest/pkg/vtc"
)

var (
	verbose   = flag.Bool("v", false, "Verbose output")
	quiet     = flag.Bool("q", false, "Quiet mode")
	keepTmp   = flag.Bool("k", false, "Keep temp directories")
	jobs      = flag.Int("j", 1, "Number of parallel jobs")
	timeoutSec = flag.Int("t", 60, "Test timeout in seconds")
	dumpAST   = flag.Bool("dump-ast", false, "Dump AST and exit")
	version   = flag.Bool("version", false, "Show version")
)

const (
	versionString = "gtest 0.5.0 (Phase 5)"
	exitPass      = 0
	exitFail      = 1
	exitSkip      = 77
	exitError     = 2
)

func init() {
	// Register all built-in commands
	vtc.RegisterBuiltinCommands()
	RegisterBuiltinCommands()
}

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

	// TODO: Set up logging verbosity based on flags
	// For now, logging level is controlled per-logger

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

	// Create macro store with default macros
	macros := vtc.NewMacroStore()
	vtc.SetupDefaultMacros(macros, testFile, os.Args[0])

	// If just dumping AST, do that
	if *dumpAST {
		ast, err := vtc.ParseTestFile(testFile, logger, macros)
		if err != nil {
			logger.Error("Parse error: %v", err)
			return exitError
		}
		vtc.DumpAST(ast, 0)
		return exitPass
	}

	// Run the test
	timeout := time.Duration(*timeoutSec) * time.Second
	code, err := vtc.RunTest(testFile, logger, macros, *keepTmp, timeout)

	// Handle different exit codes
	switch code {
	case exitPass:
		if !*quiet {
			fmt.Printf("✓ %s\n", testName)
		}
	case exitSkip:
		if !*quiet {
			fmt.Printf("⊘ %s (skipped)\n", testName)
		}
	case exitFail:
		if err != nil {
			logger.Error("Test failed: %v", err)
		}
		if !*quiet {
			fmt.Printf("✗ %s\n", testName)
		}
	case exitError:
		if err != nil {
			logger.Error("Test error: %v", err)
		}
		if !*quiet {
			fmt.Printf("✗ %s (error)\n", testName)
		}
	}

	return code
}


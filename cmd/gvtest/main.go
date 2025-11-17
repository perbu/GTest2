// gvtest - Go port of VTest2 (Varnishtest)
// HTTP testing framework with byte-level control for malformed traffic generation
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	versionString = "gvtest 0.5.0 (Phase 5)"
	exitPass      = 0
	exitFail      = 1
	exitSkip      = 77
	exitError     = 2
)

// testResult holds the result of running a single test
type testResult struct {
	testFile string
	exitCode int
	output   string
	err      error
}

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

	// Set up logging verbosity based on flags
	logging.SetVerbose(*verbose)

	// Determine if parallel execution is needed
	var exitCode int
	if *jobs <= 1 {
		// Sequential execution
		exitCode = runTestsSequential(args)
	} else {
		// Parallel execution
		exitCode = runTestsParallel(args, *jobs)
	}

	os.Exit(exitCode)
}

// runTestsSequential runs tests sequentially (original behavior)
func runTestsSequential(testFiles []string) int {
	exitCode := exitPass
	for _, testFile := range testFiles {
		result := runTest(testFile)
		if result != exitPass {
			exitCode = result
		}
	}
	return exitCode
}

// runTestsParallel runs tests in parallel using a worker pool
func runTestsParallel(testFiles []string, numWorkers int) int {
	// Create channels for work distribution and result collection
	testChan := make(chan string, len(testFiles))
	resultChan := make(chan testResult, len(testFiles))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go testWorker(testChan, resultChan, &wg)
	}

	// Send test files to workers
	for _, testFile := range testFiles {
		testChan <- testFile
	}
	close(testChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect and display results
	exitCode := exitPass
	var mu sync.Mutex
	for result := range resultChan {
		mu.Lock()
		displayTestResult(result)

		// Update exit code with priority: error > fail > skip > pass
		if result.exitCode == exitError {
			exitCode = exitError
		} else if result.exitCode == exitFail && exitCode != exitError {
			exitCode = exitFail
		} else if result.exitCode == exitSkip && exitCode == exitPass {
			exitCode = exitSkip
		}
		mu.Unlock()
	}

	return exitCode
}

// testWorker processes test files from the channel
func testWorker(testChan <-chan string, resultChan chan<- testResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for testFile := range testChan {
		result := runTestCapture(testFile)
		resultChan <- result
	}
}

// runTestCapture runs a test and captures its output
func runTestCapture(testFile string) testResult {
	// Create logger
	testName := filepath.Base(testFile)
	logger := logging.NewLogger(testName)

	// Reset output before each test
	logging.ResetOutput()

	// Create macro store with default macros
	macros := vtc.NewMacroStore()
	vtc.SetupDefaultMacros(macros, testFile)

	// Run the test
	timeout := time.Duration(*timeoutSec) * time.Second
	code, err := vtc.RunTest(testFile, logger, macros, *keepTmp, timeout)

	// Capture log output
	logOutput := logging.GetOutput()

	return testResult{
		testFile: testFile,
		exitCode: code,
		output:   logOutput,
		err:      err,
	}
}

// displayTestResult outputs the result of a test
func displayTestResult(result testResult) {
	testName := filepath.Base(result.testFile)

	switch result.exitCode {
	case exitPass:
		if !*quiet {
			fmt.Printf("✓ %s\n", testName)
		}
		if *verbose && result.output != "" {
			fmt.Print(result.output)
		}
	case exitSkip:
		if !*quiet {
			fmt.Printf("⊘ %s (skipped)\n", testName)
		}
		if *verbose && result.output != "" {
			fmt.Print(result.output)
		}
	case exitFail:
		if !*quiet {
			fmt.Printf("✗ %s\n", testName)
		}
		if !*quiet && result.output != "" {
			fmt.Print(result.output)
		}
	case exitError:
		if !*quiet {
			fmt.Printf("✗ %s (error)\n", testName)
		}
		if !*quiet && result.output != "" {
			fmt.Print(result.output)
		}
	}
}

func runTest(testFile string) int {
	// Create logger
	testName := filepath.Base(testFile)
	logger := logging.NewLogger(testName)

	// Reset output before each test
	logging.ResetOutput()

	if !*quiet {
		logger.Info("Running test: %s", testFile)
	}

	// Create macro store with default macros
	macros := vtc.NewMacroStore()
	vtc.SetupDefaultMacros(macros, testFile)

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

	// Get log output
	logOutput := logging.GetOutput()

	// Handle different exit codes
	switch code {
	case exitPass:
		if !*quiet {
			fmt.Printf("✓ %s\n", testName)
		}
		// Print logs in verbose mode
		if *verbose && logOutput != "" {
			fmt.Print(logOutput)
		}
	case exitSkip:
		if !*quiet {
			fmt.Printf("⊘ %s (skipped)\n", testName)
		}
		if *verbose && logOutput != "" {
			fmt.Print(logOutput)
		}
	case exitFail:
		if err != nil {
			logger.Error("Test failed: %v", err)
			// Refresh log output after adding error
			logOutput = logging.GetOutput()
		}
		if !*quiet {
			fmt.Printf("✗ %s\n", testName)
		}
		// Always print logs on failure (unless quiet)
		if !*quiet && logOutput != "" {
			fmt.Print(logOutput)
		}
	case exitError:
		if err != nil {
			logger.Error("Test error: %v", err)
			// Refresh log output after adding error
			logOutput = logging.GetOutput()
		}
		if !*quiet {
			fmt.Printf("✗ %s (error)\n", testName)
		}
		// Always print logs on error (unless quiet)
		if !*quiet && logOutput != "" {
			fmt.Print(logOutput)
		}
	}

	return code
}


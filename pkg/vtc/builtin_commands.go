// Package vtc provides built-in VTC commands
package vtc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/perbu/GTest/pkg/barrier"
	"github.com/perbu/GTest/pkg/logging"
	"github.com/perbu/GTest/pkg/process"
)

// RegisterBuiltinCommands registers all built-in VTC commands
func RegisterBuiltinCommands() {
	RegisterCommand("barrier", cmdBarrier, FlagGlobal)
	RegisterCommand("shell", cmdShell, FlagGlobal)
	RegisterCommand("delay", cmdDelay, FlagGlobal)
	RegisterCommand("feature", cmdFeature, FlagNone)
	RegisterCommand("filewrite", cmdFilewrite, FlagNone)
	RegisterCommand("process", cmdProcess, FlagNone)
	RegisterCommand("vtest", cmdVtest, FlagNone)
}

// cmdVtest handles the "vtest" command
func cmdVtest(args []string, priv interface{}, logger *logging.Logger) error {
	// Just a test description - nothing to do
	if len(args) > 0 {
		logger.Info("Test: %s", args[0])
	}
	return nil
}

// cmdBarrier handles the "barrier" command
func cmdBarrier(args []string, priv interface{}, logger *logging.Logger) error {
	ctx, ok := priv.(*ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for barrier command")
	}

	if len(args) == 0 {
		return fmt.Errorf("barrier: missing barrier name")
	}

	barrierName := args[0]
	args = args[1:]

	// Validate barrier name starts with 'b'
	if len(barrierName) == 0 || barrierName[0] != 'b' {
		return fmt.Errorf("barrier name must start with 'b' (got %s)", barrierName)
	}

	// Get or create barrier
	var b *barrier.Barrier
	if existing, ok := ctx.Barriers[barrierName]; ok {
		b = existing.(*barrier.Barrier)
	} else {
		b = barrier.New(barrierName, logger)
		ctx.Barriers[barrierName] = b
	}

	// Parse options
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "cond":
			// VTest2 syntax: barrier <name> cond <count> [-cyclic]
			if i+1 >= len(args) {
				return fmt.Errorf("barrier: cond requires a count")
			}
			i++
			count, err := strconv.Atoi(args[i])
			if err != nil || count < 1 {
				return fmt.Errorf("barrier: invalid count: %s", args[i])
			}

			// Check for -cyclic flag
			if i+1 < len(args) && args[i+1] == "-cyclic" {
				b.Cyclic = true
				i++
			}

			return b.Start(count)

		case "-start":
			// Initialize barrier with count
			count := 1
			if i+1 < len(args) {
				// Try to parse next arg as count
				if c, err := strconv.Atoi(args[i+1]); err == nil && c > 0 {
					count = c
					i++
				}
			}
			return b.Start(count)

		case "-wait":
			return b.Wait()

		case "sync":
			// VTest2 syntax: barrier <name> sync
			return b.Sync()

		case "-sync":
			return b.Sync()

		case "-timeout":
			if i+1 >= len(args) {
				return fmt.Errorf("barrier: -timeout requires a value")
			}
			i++
			timeout, err := time.ParseDuration(args[i] + "s")
			if err != nil {
				return fmt.Errorf("barrier: invalid timeout: %w", err)
			}
			b.SetTimeout(timeout)

		case "-cyclic":
			b.Cyclic = true

		default:
			return fmt.Errorf("barrier: unknown option: %s", args[i])
		}
	}

	return nil
}

// cmdShell handles the "shell" command
func cmdShell(args []string, priv interface{}, logger *logging.Logger) error {
	ctx, ok := priv.(*ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for shell command")
	}

	if len(args) == 0 {
		return fmt.Errorf("shell: missing command")
	}

	// Parse options
	var (
		shellCmd      string
		expectExit    = 0
		matchPattern  string
		expectOutput  string
		hasExitCode   = false
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-exit":
			if i+1 >= len(args) {
				return fmt.Errorf("shell: -exit requires a value")
			}
			i++
			var err error
			expectExit, err = strconv.Atoi(args[i])
			if err != nil {
				return fmt.Errorf("shell: invalid exit code: %w", err)
			}
			hasExitCode = true

		case "-match":
			if i+1 >= len(args) {
				return fmt.Errorf("shell: -match requires a value")
			}
			i++
			matchPattern = args[i]

		case "-expect":
			if i+1 >= len(args) {
				return fmt.Errorf("shell: -expect requires a value")
			}
			i++
			expectOutput = args[i]

		default:
			// This is the command to execute
			shellCmd = args[i]
		}
	}

	if shellCmd == "" {
		return fmt.Errorf("shell: no command specified")
	}

	// Execute the command
	logger.Debug("Executing shell command: %s", shellCmd)
	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Dir = ctx.TmpDir

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("shell: failed to execute: %w", err)
		}
	}

	// Check exit code
	if hasExitCode && exitCode != expectExit {
		return fmt.Errorf("shell: expected exit code %d, got %d", expectExit, exitCode)
	}

	// Check output match
	if matchPattern != "" {
		matched, err := regexp.MatchString(matchPattern, string(output))
		if err != nil {
			return fmt.Errorf("shell: invalid regex: %w", err)
		}
		if !matched {
			return fmt.Errorf("shell: output did not match pattern %s", matchPattern)
		}
	}

	// Check exact output
	if expectOutput != "" && strings.TrimSpace(string(output)) != expectOutput {
		return fmt.Errorf("shell: expected output %q, got %q", expectOutput, string(output))
	}

	logger.Debug("Shell command output: %s", string(output))
	return nil
}

// cmdDelay handles the "delay" command
func cmdDelay(args []string, priv interface{}, logger *logging.Logger) error {
	if len(args) == 0 {
		return fmt.Errorf("delay: missing duration")
	}

	// Parse duration (in seconds unless unit specified)
	durationStr := args[0]
	if !strings.Contains(durationStr, "s") && !strings.Contains(durationStr, "m") {
		durationStr += "s" // Default to seconds
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		// Try parsing as float seconds
		if seconds, err := strconv.ParseFloat(args[0], 64); err == nil {
			duration = time.Duration(seconds * float64(time.Second))
		} else {
			return fmt.Errorf("delay: invalid duration: %s", args[0])
		}
	}

	logger.Debug("Delaying for %v", duration)
	time.Sleep(duration)
	return nil
}

// cmdFeature handles the "feature" command
func cmdFeature(args []string, priv interface{}, logger *logging.Logger) error {
	ctx, ok := priv.(*ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for feature command")
	}

	if len(args) == 0 {
		return fmt.Errorf("feature: missing feature check")
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "cmd":
			// Check if command exists
			if i+1 >= len(args) {
				return fmt.Errorf("feature: cmd requires a command name")
			}
			i++
			cmdName := args[i]
			if _, err := exec.LookPath(cmdName); err != nil {
				ctx.Skip(fmt.Sprintf("command '%s' not available", cmdName))
				return nil
			}

		case "user":
			// Check if running as specific user
			if i+1 >= len(args) {
				return fmt.Errorf("feature: user requires a username")
			}
			i++
			// For now, just check if we're not root
			if os.Getuid() == 0 && args[i] != "root" {
				ctx.Skip(fmt.Sprintf("not running as user '%s'", args[i]))
				return nil
			}

		case "group":
			// Check if running as specific group
			if i+1 >= len(args) {
				return fmt.Errorf("feature: group requires a group name")
			}
			i++
			// Simplified check - would need proper group checking
			logger.Warning("feature: group check not fully implemented")

		case "SO_RCVTIMEO_WORKS":
			// Platform feature check - assume it works on Linux
			logger.Debug("feature: SO_RCVTIMEO_WORKS check passed")

		case "dns":
			// Check if DNS resolution works
			logger.Debug("feature: dns check - assuming available")

		default:
			return fmt.Errorf("feature: unknown feature check: %s", args[i])
		}
	}

	return nil
}

// cmdFilewrite handles the "filewrite" command
func cmdFilewrite(args []string, priv interface{}, logger *logging.Logger) error {
	ctx, ok := priv.(*ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for filewrite command")
	}

	if len(args) == 0 {
		return fmt.Errorf("filewrite: missing filename")
	}

	var (
		filename string
		content  string
		appendMode bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-append":
			appendMode = true

		default:
			if filename == "" {
				filename = args[i]
			} else {
				// Rest is content
				content = strings.Join(args[i:], " ")
				break
			}
		}
	}

	// Expand macros in filename
	filename, err := ctx.Macros.Expand(logger, filename)
	if err != nil {
		return fmt.Errorf("filewrite: filename expansion failed: %w", err)
	}

	// If relative path, make it relative to tmpdir
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(ctx.TmpDir, filename)
	}

	// Expand macros in content
	content, err = ctx.Macros.Expand(logger, content)
	if err != nil {
		return fmt.Errorf("filewrite: content expansion failed: %w", err)
	}

	// Write file
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(filename, flags, 0644)
	if err != nil {
		return fmt.Errorf("filewrite: failed to open file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("filewrite: failed to write: %w", err)
	}

	logger.Debug("Wrote %d bytes to %s", len(content), filename)
	return nil
}

// cmdProcess handles the "process" command
func cmdProcess(args []string, priv interface{}, logger *logging.Logger) error {
	ctx, ok := priv.(*ExecContext)
	if !ok {
		return fmt.Errorf("invalid context for process command")
	}

	if len(args) == 0 {
		return fmt.Errorf("process: missing process name")
	}

	procName := args[0]
	args = args[1:]

	// Validate process name starts with 'p'
	if len(procName) == 0 || procName[0] != 'p' {
		return fmt.Errorf("process name must start with 'p' (got %s)", procName)
	}

	// Get or create process
	var p *process.Process
	if existing, ok := ctx.Processes[procName]; ok {
		p = existing.(*process.Process)
	}

	// Parse options
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-start":
			if i+1 >= len(args) {
				return fmt.Errorf("process: -start requires a command")
			}
			i++
			cmdStr := args[i]

			// Parse command and arguments
			cmdParts := strings.Fields(cmdStr)
			if len(cmdParts) == 0 {
				return fmt.Errorf("process: empty command")
			}

			p = process.New(procName, logger, cmdParts[0], cmdParts[1:]...)
			ctx.Processes[procName] = p
			return p.Start()

		case "-wait":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			return p.Wait()

		case "-stop":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			return p.Stop()

		case "-kill":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			return p.Kill()

		case "-write":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			if i+1 >= len(args) {
				return fmt.Errorf("process: -write requires data")
			}
			i++
			return p.Write(args[i])

		case "-writeln":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			if i+1 >= len(args) {
				return fmt.Errorf("process: -writeln requires data")
			}
			i++
			return p.WriteLine(args[i])

		case "-writehex":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			if i+1 >= len(args) {
				return fmt.Errorf("process: -writehex requires hex data")
			}
			i++
			return p.WriteHex(args[i])

		case "-expect-text":
			if p == nil {
				return fmt.Errorf("process: process not started")
			}
			if i+1 >= len(args) {
				return fmt.Errorf("process: -expect-text requires text")
			}
			i++
			time.Sleep(100 * time.Millisecond) // Give process time to output
			if !p.ExpectText(args[i]) {
				return fmt.Errorf("process: expected text not found: %s", args[i])
			}

		default:
			return fmt.Errorf("process: unknown option: %s", args[i])
		}
	}

	return nil
}

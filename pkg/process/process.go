// Package process provides external process management for VTC tests
package process

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/perbu/GTest/pkg/logging"
)

// Process represents a managed external process
type Process struct {
	Name      string
	Cmd       *exec.Cmd
	Logger    *logging.Logger

	// I/O
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser

	// Output capture
	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
	mutex     sync.Mutex

	// State
	started   bool
	done      chan struct{}
	err       error
}

// New creates a new process manager
func New(name string, logger *logging.Logger, command string, args ...string) *Process {
	cmd := exec.Command(command, args...)

	return &Process{
		Name:    name,
		Cmd:     cmd,
		Logger:  logger,
		started: false,
		done:    make(chan struct{}),
	}
}

// Start starts the process
func (p *Process) Start() error {
	var err error

	// Set up pipes
	p.stdin, err = p.Cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	p.stdout, err = p.Cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	p.stderr, err = p.Cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := p.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.started = true
	p.Logger.Debug("Process %s started (pid %d)", p.Name, p.Cmd.Process.Pid)

	// Start output capture goroutines
	go p.captureOutput(p.stdout, &p.stdoutBuf, "stdout")
	go p.captureOutput(p.stderr, &p.stderrBuf, "stderr")

	// Wait for process to complete
	go func() {
		p.err = p.Cmd.Wait()
		close(p.done)
		p.Logger.Debug("Process %s exited", p.Name)
	}()

	return nil
}

// captureOutput captures output from a reader
func (p *Process) captureOutput(r io.Reader, buf *bytes.Buffer, name string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		p.mutex.Lock()
		buf.WriteString(line)
		buf.WriteString("\n")
		p.mutex.Unlock()

		p.Logger.Debug("Process %s [%s]: %s", p.Name, name, line)
	}
}

// Write writes data to the process stdin
func (p *Process) Write(data string) error {
	if !p.started {
		return fmt.Errorf("process not started")
	}

	_, err := p.stdin.Write([]byte(data))
	return err
}

// WriteLine writes a line to the process stdin
func (p *Process) WriteLine(line string) error {
	return p.Write(line + "\n")
}

// WriteHex writes hex-encoded data to the process stdin
func (p *Process) WriteHex(hexData string) error {
	// Parse hex string and write binary data
	data := make([]byte, 0, len(hexData)/2)
	for i := 0; i < len(hexData); i += 2 {
		if i+1 >= len(hexData) {
			break
		}
		var b byte
		fmt.Sscanf(hexData[i:i+2], "%02x", &b)
		data = append(data, b)
	}

	_, err := p.stdin.Write(data)
	return err
}

// GetStdout returns the captured stdout
func (p *Process) GetStdout() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.stdoutBuf.String()
}

// GetStderr returns the captured stderr
func (p *Process) GetStderr() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.stderrBuf.String()
}

// Wait waits for the process to complete
func (p *Process) Wait() error {
	if !p.started {
		return fmt.Errorf("process not started")
	}

	<-p.done
	return p.err
}

// WaitTimeout waits with a timeout
func (p *Process) WaitTimeout(timeout time.Duration) error {
	if !p.started {
		return fmt.Errorf("process not started")
	}

	select {
	case <-p.done:
		return p.err
	case <-time.After(timeout):
		return fmt.Errorf("process wait timeout")
	}
}

// Kill kills the process
func (p *Process) Kill() error {
	if !p.started {
		return fmt.Errorf("process not started")
	}

	if p.Cmd.Process == nil {
		return nil
	}

	return p.Cmd.Process.Kill()
}

// Stop gracefully stops the process
func (p *Process) Stop() error {
	if !p.started {
		return fmt.Errorf("process not started")
	}

	// Close stdin to signal end
	if p.stdin != nil {
		p.stdin.Close()
	}

	// Wait for process with timeout
	return p.WaitTimeout(5 * time.Second)
}

// ExitCode returns the exit code of the process
func (p *Process) ExitCode() int {
	if p.Cmd.ProcessState == nil {
		return -1
	}
	return p.Cmd.ProcessState.ExitCode()
}

// ExpectText checks if the stdout contains the expected text
// This is a simplified version - full terminal emulation would be more complex
func (p *Process) ExpectText(text string) bool {
	stdout := p.GetStdout()
	return bytes.Contains([]byte(stdout), []byte(text))
}

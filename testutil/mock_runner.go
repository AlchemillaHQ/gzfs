package testutil

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// MockRunner implements the Runner interface for testing
type MockRunner struct {
	// Commands maps command strings to their expected output
	Commands map[string]MockCommand
	// CallHistory tracks all commands that were executed
	CallHistory []MockCall
}

type MockCommand struct {
	Stdout string
	Stderr string
	Error  error
}

type MockCall struct {
	Name string
	Args []string
	Cmd  string
}

// NewMockRunner creates a new mock runner
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Commands:    make(map[string]MockCommand),
		CallHistory: make([]MockCall, 0),
	}
}

// AddCommand adds a mocked command response
func (m *MockRunner) AddCommand(cmd string, stdout, stderr string, err error) {
	m.Commands[cmd] = MockCommand{
		Stdout: stdout,
		Stderr: stderr,
		Error:  err,
	}
}

// Run implements the Runner interface
func (m *MockRunner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	// Build the command string for lookup
	cmdStr := name + " " + strings.Join(args, " ")

	// Record the call
	call := MockCall{
		Name: name,
		Args: args,
		Cmd:  cmdStr,
	}
	m.CallHistory = append(m.CallHistory, call)

	// Look for exact match first
	if mock, ok := m.Commands[cmdStr]; ok {
		if stdout != nil && mock.Stdout != "" {
			stdout.Write([]byte(mock.Stdout))
		}
		if stderr != nil && mock.Stderr != "" {
			stderr.Write([]byte(mock.Stderr))
		}
		return mock.Error
	}

	// Look for pattern matches (for flexibility)
	for pattern, mock := range m.Commands {
		if strings.Contains(cmdStr, pattern) {
			if stdout != nil && mock.Stdout != "" {
				stdout.Write([]byte(mock.Stdout))
			}
			if stderr != nil && mock.Stderr != "" {
				stderr.Write([]byte(mock.Stderr))
			}
			return mock.Error
		}
	}

	// Command not found
	return fmt.Errorf("mock command not found: %s", cmdStr)
}

// GetLastCall returns the last executed command
func (m *MockRunner) GetLastCall() *MockCall {
	if len(m.CallHistory) == 0 {
		return nil
	}
	return &m.CallHistory[len(m.CallHistory)-1]
}

// Reset clears all recorded calls
func (m *MockRunner) Reset() {
	m.CallHistory = m.CallHistory[:0]
}

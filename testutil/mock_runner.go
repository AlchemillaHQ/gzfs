package testutil

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type MockRunner struct {
	Commands    map[string]MockCommand
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

func NewMockRunner() *MockRunner {
	return &MockRunner{
		Commands:    make(map[string]MockCommand),
		CallHistory: make([]MockCall, 0),
	}
}

func (m *MockRunner) AddCommand(cmd string, stdout, stderr string, err error) {
	m.Commands[cmd] = MockCommand{
		Stdout: stdout,
		Stderr: stderr,
		Error:  err,
	}
}

func (m *MockRunner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmdStr := name + " " + strings.Join(args, " ")

	call := MockCall{
		Name: name,
		Args: args,
		Cmd:  cmdStr,
	}
	m.CallHistory = append(m.CallHistory, call)

	if mock, ok := m.Commands[cmdStr]; ok {
		if stdout != nil && mock.Stdout != "" {
			stdout.Write([]byte(mock.Stdout))
		}
		if stderr != nil && mock.Stderr != "" {
			stderr.Write([]byte(mock.Stderr))
		}
		return mock.Error
	}

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

	return fmt.Errorf("mock command not found: %s", cmdStr)
}

func (m *MockRunner) GetLastCall() *MockCall {
	if len(m.CallHistory) == 0 {
		return nil
	}
	return &m.CallHistory[len(m.CallHistory)-1]
}

func (m *MockRunner) Reset() {
	m.CallHistory = m.CallHistory[:0]
}

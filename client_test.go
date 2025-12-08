package gzfs

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alchemillahq/gzfs/testutil"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{
			name: "default options",
			opts: Options{},
		},
		{
			name: "with sudo",
			opts: Options{Sudo: true},
		},
		{
			name: "custom binaries",
			opts: Options{
				ZFSBin:   "/usr/local/bin/zfs",
				ZpoolBin: "/usr/local/bin/zpool",
				ZDBBin:   "/usr/local/bin/zdb",
			},
		},
		{
			name: "with custom runner",
			opts: Options{
				Runner: testutil.NewMockRunner(),
			},
		},
		{
			name: "with cache TTL",
			opts: Options{
				ZDBCacheTTLSeconds: 300,
			},
		},
		{
			name: "negative cache TTL should use default",
			opts: Options{
				ZDBCacheTTLSeconds: -1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.opts)

			// Verify client is not nil
			if client == nil {
				t.Fatal("NewClient returned nil")
			}

			// Verify all components are initialized
			if client.ZFS == nil {
				t.Error("ZFS client is nil")
			}
			if client.Zpool == nil {
				t.Error("Zpool client is nil")
			}
			if client.ZDB == nil {
				t.Error("ZDB client is nil")
			}

			// Verify default binaries are set
			expectedZFS := "zfs"
			if tt.opts.ZFSBin != "" {
				expectedZFS = tt.opts.ZFSBin
			}
			if client.ZFS.cmd.Bin != expectedZFS {
				t.Errorf("Expected ZFS bin %q, got %q", expectedZFS, client.ZFS.cmd.Bin)
			}

			expectedZpool := "zpool"
			if tt.opts.ZpoolBin != "" {
				expectedZpool = tt.opts.ZpoolBin
			}
			if client.Zpool.cmd.Bin != expectedZpool {
				t.Errorf("Expected Zpool bin %q, got %q", expectedZpool, client.Zpool.cmd.Bin)
			}

			expectedZDB := "zdb"
			if tt.opts.ZDBBin != "" {
				expectedZDB = tt.opts.ZDBBin
			}
			if client.ZDB.cmd.Bin != expectedZDB {
				t.Errorf("Expected ZDB bin %q, got %q", expectedZDB, client.ZDB.cmd.Bin)
			}

			// Verify sudo setting
			if client.ZFS.cmd.Sudo != tt.opts.Sudo {
				t.Errorf("Expected ZFS sudo %v, got %v", tt.opts.Sudo, client.ZFS.cmd.Sudo)
			}
			if client.Zpool.cmd.Sudo != tt.opts.Sudo {
				t.Errorf("Expected Zpool sudo %v, got %v", tt.opts.Sudo, client.Zpool.cmd.Sudo)
			}
			if client.ZDB.cmd.Sudo != tt.opts.Sudo {
				t.Errorf("Expected ZDB sudo %v, got %v", tt.opts.Sudo, client.ZDB.cmd.Sudo)
			}
		})
	}
}

func TestCmd_RunBytes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		cmd            Cmd
		args           []string
		mockOutput     string
		mockError      error
		expectedStdout string
		expectError    bool
	}{
		{
			name: "successful command",
			cmd: Cmd{
				Bin:    "echo",
				Sudo:   false,
				Runner: nil, // Will use default LocalRunner in test
			},
			args:           []string{"hello"},
			mockOutput:     "hello\n",
			expectedStdout: "hello\n",
			expectError:    false,
		},
		{
			name: "command with sudo",
			cmd: Cmd{
				Bin:  "zfs",
				Sudo: true,
			},
			args:        []string{"list"},
			mockOutput:  "output",
			expectError: false,
		},
		{
			name: "command with error",
			cmd: Cmd{
				Bin:  "false",
				Sudo: false,
			},
			args:        []string{},
			mockError:   fmt.Errorf("exit status 1"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock runner for controlled testing
			mockRunner := testutil.NewMockRunner()
			tt.cmd.Runner = mockRunner

			// Set up the mock command
			expectedCmd := tt.cmd.Bin + " " + strings.Join(tt.args, " ")
			if tt.cmd.Sudo {
				expectedCmd = "sudo " + tt.cmd.Bin + " " + strings.Join(tt.args, " ")
			}

			mockRunner.AddCommand(expectedCmd, tt.mockOutput, "", tt.mockError)

			// Execute the command
			stdout, stderr, err := tt.cmd.RunBytes(ctx, nil, tt.args...)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check stdout if no error expected
			if !tt.expectError {
				if string(stdout) != tt.mockOutput {
					t.Errorf("Expected stdout %q, got %q", tt.mockOutput, string(stdout))
				}
			}

			// Verify stderr is returned (even if empty)
			if stderr == nil && !tt.expectError {
				// Note: stderr may be nil if no output written to it
			}

			// Verify the command was called
			lastCall := mockRunner.GetLastCall()
			if lastCall == nil {
				t.Fatal("No command was executed")
			}
		})
	}
}

func TestCmd_RunJSON(t *testing.T) {
	ctx := context.Background()

	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name        string
		mockJSON    string
		mockError   error
		expected    TestStruct
		expectError bool
	}{
		{
			name:      "valid JSON",
			mockJSON:  `{"name":"test","value":42}`,
			expected:  TestStruct{Name: "test", Value: 42},
			mockError: nil,
		},
		{
			name:        "invalid JSON",
			mockJSON:    `{"name":"test","value":}`,
			expectError: true,
		},
		{
			name:        "command error",
			mockError:   fmt.Errorf("command failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			cmd := Cmd{
				Bin:    "test",
				Runner: mockRunner,
			}

			// The RunJSON method appends "-j" to args
			mockRunner.AddCommand("test  -j", tt.mockJSON, "", tt.mockError)

			var result TestStruct
			err := cmd.RunJSON(ctx, &result, "")

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if result != tt.expected {
					t.Errorf("Expected %+v, got %+v", tt.expected, result)
				}
			}
		})
	}
}

func TestCmdError(t *testing.T) {
	tests := []struct {
		name     string
		cmdErr   *CmdError
		expected string
	}{
		{
			name: "error with exit error",
			cmdErr: &CmdError{
				Cmd:      "zfs",
				Args:     []string{"list"},
				ExitErr:  fmt.Errorf("exit status 1"),
				Stderr:   "permission denied",
				Combined: "zfs list",
			},
			expected: "zfs list failed: exit status 1 (stderr: permission denied)",
		},
		{
			name: "error without exit error",
			cmdErr: &CmdError{
				Cmd:      "zfs",
				Args:     []string{"list"},
				Stderr:   "command not found",
				Combined: "zfs list",
			},
			expected: "zfs list failed (stderr: command not found)",
		},
		{
			name: "error with whitespace in stderr",
			cmdErr: &CmdError{
				Cmd:      "zfs",
				Args:     []string{"list"},
				Stderr:   "  permission denied  \n",
				Combined: "zfs list",
			},
			expected: "zfs list failed (stderr: permission denied)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmdErr.Error()
			if result != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, result)
			}

			// Test Unwrap
			if tt.cmdErr.ExitErr != nil {
				unwrapped := tt.cmdErr.Unwrap()
				if unwrapped != tt.cmdErr.ExitErr {
					t.Errorf("Expected unwrapped error %v, got %v", tt.cmdErr.ExitErr, unwrapped)
				}
			}
		})
	}
}

func TestLocalRunner(t *testing.T) {
	ctx := context.Background()
	runner := LocalRunner{}

	t.Run("successful command", func(t *testing.T) {
		// This test requires the 'echo' command to be available
		var stdout, stderr strings.Builder
		err := runner.Run(ctx, nil, &stdout, &stderr, "echo", "hello", "world")

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected := "hello world\n"
		if stdout.String() != expected {
			t.Errorf("Expected stdout %q, got %q", expected, stdout.String())
		}
	})

	t.Run("command not found", func(t *testing.T) {
		var stdout, stderr strings.Builder
		err := runner.Run(ctx, nil, &stdout, &stderr, "nonexistentcommand12345")

		if err == nil {
			t.Error("Expected error for non-existent command")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		var stdout, stderr strings.Builder
		err := runner.Run(cancelCtx, nil, &stdout, &stderr, "sleep", "10")

		if err == nil {
			t.Error("Expected error due to context cancellation")
		}
	})
}

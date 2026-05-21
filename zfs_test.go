package gzfs

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alchemillahq/gzfs/testutil"
)

func TestZFS_List(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		recursive   bool
		dataset     string
		mockJSON    string
		expectError bool
		expectedLen int
	}{
		{
			name:        "successful list all",
			recursive:   false,
			dataset:     "",
			mockJSON:    testutil.ZFSListJSON,
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "successful list recursive",
			recursive:   true,
			dataset:     "tank",
			mockJSON:    testutil.ZFSListJSON,
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "command error",
			recursive:   false,
			dataset:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			// Create ZFS client with mock runner
			client := &zfs{
				cmd: Cmd{
					Bin:    "zfs",
					Runner: mockRunner,
				},
			}

			if tt.expectError {
				// Don't add a mock response to trigger error
			} else {
				// Add expected command with JSON response
				expectedCmd := "zfs list -o " + strings.Join(dsPropList, ",") + " -p"
				if tt.recursive {
					expectedCmd += " -r"
				}
				if tt.dataset != "" {
					expectedCmd += " " + tt.dataset
				}
				expectedCmd += " -j"
				mockRunner.AddCommand(expectedCmd, tt.mockJSON, "", nil)
			}

			// Execute the test
			datasets, err := client.List(ctx, tt.recursive, tt.dataset)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check results
			if !tt.expectError {
				if len(datasets) != tt.expectedLen {
					t.Errorf("Expected %d datasets, got %d", tt.expectedLen, len(datasets))
				}

				// Verify dataset properties are parsed
				for _, ds := range datasets {
					if ds.z == nil {
						t.Error("Dataset should have zfs client reference")
					}

					// Check that parsed values are not zero (they should be populated from JSON)
					if ds.Used == 0 && ds.Name != "" {
						t.Errorf("Dataset %s should have non-zero Used value", ds.Name)
					}
				}
			}
		})
	}
}

func TestZFS_Get(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     string
		recursive   bool
		mockJSON    string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "successful get",
			dataset:     "tank",
			recursive:   false,
			mockJSON:    testutil.ZFSListJSON,
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "dataset not found",
			dataset:     "nonexistent",
			recursive:   false,
			mockJSON:    testutil.ZFSListJSON,
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "command error",
			dataset:     "tank",
			recursive:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			client := &zfs{
				cmd: Cmd{
					Bin:    "zfs",
					Runner: mockRunner,
				},
			}

			if !tt.expectError {
				expectedCmd := "zfs list -o " + strings.Join(dsPropList, ",") + " -p"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset + " -j"
				mockRunner.AddCommand(expectedCmd, tt.mockJSON, "", nil)
			}

			dataset, err := client.Get(ctx, tt.dataset, tt.recursive)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectNil && dataset != nil {
				t.Error("Expected nil dataset")
			}
			if !tt.expectNil && !tt.expectError && dataset == nil {
				t.Error("Expected non-nil dataset")
			}

			if dataset != nil {
				if dataset.z == nil {
					t.Error("Dataset should have zfs client reference")
				}
				if dataset.Name != tt.dataset {
					t.Errorf("Expected dataset name %q, got %q", tt.dataset, dataset.Name)
				}
			}
		})
	}
}

func TestDataset_IsEncrypted(t *testing.T) {
	tests := []struct {
		name     string
		dataset  *Dataset
		expected bool
	}{
		{
			name:     "nil dataset",
			dataset:  nil,
			expected: false,
		},
		{
			name:     "nil properties",
			dataset:  &Dataset{Properties: nil},
			expected: false,
		},
		{
			name:     "encryption off",
			dataset:  &Dataset{Properties: map[string]ZFSProperty{"encryption": {Value: "off"}}},
			expected: false,
		},
		{
			name:     "encryption empty",
			dataset:  &Dataset{Properties: map[string]ZFSProperty{"encryption": {Value: " "}}},
			expected: false,
		},
		{
			name:     "encryption missing",
			dataset:  &Dataset{Properties: map[string]ZFSProperty{}},
			expected: false,
		},
		{
			name:     "encryption aes-256-gcm",
			dataset:  &Dataset{Properties: map[string]ZFSProperty{"encryption": {Value: "aes-256-gcm"}}},
			expected: true,
		},
		{
			name:     "encryption on",
			dataset:  &Dataset{Properties: map[string]ZFSProperty{"encryption": {Value: "on"}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dataset.IsEncrypted()
			if result != tt.expected {
				t.Errorf("IsEncrypted() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDataset_LoadKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     *Dataset
		recursive   bool
		mockError   bool
		expectError bool
	}{
		{
			name: "success",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			recursive:   false,
			expectError: false,
		},
		{
			name: "success recursive",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			recursive:   true,
			expectError: false,
		},
		{
			name: "zfs error",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			mockError:   true,
			expectError: true,
		},
		{
			name:        "nil dataset",
			dataset:     nil,
			expectError: true,
		},
		{
			name: "nil zfs client",
			dataset: &Dataset{
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			expectError: true,
		},
		{
			name: "snapshot rejection",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc@snap",
				Type: DatasetTypeSnapshot,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			if tt.dataset != nil && tt.dataset.z != nil {
				tt.dataset.z.cmd.Runner = mockRunner

				if !tt.expectError || tt.mockError {
					expectedCmd := "zfs load-key"
					if tt.recursive {
						expectedCmd += " -r"
					}
					expectedCmd += " " + tt.dataset.Name

					if tt.mockError {
						mockRunner.AddCommand(expectedCmd, "", "load-key error", fmt.Errorf("exit status 1"))
					} else {
						mockRunner.AddCommand(expectedCmd, "", "", nil)
					}
				}
			}

			err := tt.dataset.LoadKey(ctx, tt.recursive)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && !tt.mockError {
				lastCall := mockRunner.GetLastCall()
				if lastCall == nil {
					t.Fatal("No command was executed")
				}
				expectedCmd := "zfs load-key"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset.Name
				if lastCall.Cmd != expectedCmd {
					t.Errorf("Expected command %q, got %q", expectedCmd, lastCall.Cmd)
				}
			}
		})
	}
}

func TestDataset_LoadKeyWithPassphrase(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     *Dataset
		passphrase  string
		recursive   bool
		mockError   bool
		expectError bool
	}{
		{
			name: "success",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			passphrase:  "test-passphrase-for-encryption-32bytes",
			recursive:   false,
			expectError: false,
		},
		{
			name: "success recursive",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			passphrase:  "test-passphrase-for-encryption-32bytes",
			recursive:   true,
			expectError: false,
		},
		{
			name: "zfs error",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			passphrase:  "test-passphrase-for-encryption-32bytes",
			mockError:   true,
			expectError: true,
		},
		{
			name:        "nil dataset",
			dataset:     nil,
			expectError: true,
		},
		{
			name: "nil zfs client",
			dataset: &Dataset{
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			expectError: true,
		},
		{
			name: "snapshot rejection",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc@snap",
				Type: DatasetTypeSnapshot,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			if tt.dataset != nil && tt.dataset.z != nil {
				tt.dataset.z.cmd.Runner = mockRunner

				expectedCmd := "zfs load-key -L prompt"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset.Name
				if tt.mockError {
					mockRunner.AddCommand(expectedCmd, "", "load-key error", fmt.Errorf("exit status 1"))
				} else {
					mockRunner.AddCommand(expectedCmd, "", "", nil)
				}
			}

			err := tt.dataset.LoadKeyWithPassphrase(ctx, tt.passphrase, tt.recursive)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && !tt.mockError {
				lastCall := mockRunner.GetLastCall()
				if lastCall == nil {
					t.Fatal("No command was executed")
				}
				expectedCmd := "zfs load-key -L prompt"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset.Name
				if lastCall.Cmd != expectedCmd {
					t.Errorf("Expected command %q, got %q", expectedCmd, lastCall.Cmd)
				}
			}
		})
	}
}

func TestDataset_UnloadKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     *Dataset
		recursive   bool
		mockError   bool
		expectError bool
	}{
		{
			name: "success",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			recursive:   false,
			expectError: false,
		},
		{
			name: "success recursive",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			recursive:   true,
			expectError: false,
		},
		{
			name: "zfs error",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			mockError:   true,
			expectError: true,
		},
		{
			name:        "nil dataset",
			dataset:     nil,
			expectError: true,
		},
		{
			name: "nil zfs client",
			dataset: &Dataset{
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			expectError: true,
		},
		{
			name: "snapshot rejection",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc@snap",
				Type: DatasetTypeSnapshot,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			if tt.dataset != nil && tt.dataset.z != nil {
				tt.dataset.z.cmd.Runner = mockRunner

				if !tt.expectError || tt.mockError {
					expectedCmd := "zfs unload-key"
					if tt.recursive {
						expectedCmd += " -r"
					}
					expectedCmd += " " + tt.dataset.Name

					if tt.mockError {
						mockRunner.AddCommand(expectedCmd, "", "unload-key error", fmt.Errorf("exit status 1"))
					} else {
						mockRunner.AddCommand(expectedCmd, "", "", nil)
					}
				}
			}

			err := tt.dataset.UnloadKey(ctx, tt.recursive)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && !tt.mockError {
				lastCall := mockRunner.GetLastCall()
				if lastCall == nil {
					t.Fatal("No command was executed")
				}
				expectedCmd := "zfs unload-key"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset.Name
				if lastCall.Cmd != expectedCmd {
					t.Errorf("Expected command %q, got %q", expectedCmd, lastCall.Cmd)
				}
			}
		})
	}
}

func TestDataset_MountWithKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     *Dataset
		overlay     bool
		options     []string
		mockError   bool
		expectError bool
	}{
		{
			name: "success",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			overlay:     false,
			expectError: false,
		},
		{
			name: "success with overlay",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			overlay:     true,
			expectError: false,
		},
		{
			name: "success with options",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			overlay:     false,
			options:     []string{"ro", "noexec"},
			expectError: false,
		},
		{
			name: "zfs error",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			mockError:   true,
			expectError: true,
		},
		{
			name:        "nil dataset",
			dataset:     nil,
			expectError: true,
		},
		{
			name: "nil zfs client",
			dataset: &Dataset{
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			expectError: true,
		},
		{
			name: "snapshot rejection",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc@snap",
				Type: DatasetTypeSnapshot,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			if tt.dataset != nil && tt.dataset.z != nil {
				tt.dataset.z.cmd.Runner = mockRunner

				if !tt.expectError || tt.mockError {
					expectedCmd := "zfs mount -l"
					if tt.overlay {
						expectedCmd += " -O"
					}
					if len(tt.options) > 0 {
						expectedCmd += " -o " + strings.Join(tt.options, ",")
					}
					expectedCmd += " " + tt.dataset.Name

					if tt.mockError {
						mockRunner.AddCommand(expectedCmd, "", "mount error", fmt.Errorf("exit status 1"))
					} else {
						mockRunner.AddCommand(expectedCmd, "", "", nil)
					}
				}
			}

			err := tt.dataset.MountWithKey(ctx, tt.overlay, tt.options...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && !tt.mockError {
				lastCall := mockRunner.GetLastCall()
				if lastCall == nil {
					t.Fatal("No command was executed")
				}
				expectedCmd := "zfs mount -l"
				if tt.overlay {
					expectedCmd += " -O"
				}
				if len(tt.options) > 0 {
					expectedCmd += " -o " + strings.Join(tt.options, ",")
				}
				expectedCmd += " " + tt.dataset.Name
				if lastCall.Cmd != expectedCmd {
					t.Errorf("Expected command %q, got %q", expectedCmd, lastCall.Cmd)
				}
			}
		})
	}
}

func TestZFS_LoadKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     string
		recursive   bool
		mockError   bool
		expectError bool
	}{
		{
			name:        "success",
			dataset:     "tank/enc",
			recursive:   false,
			expectError: false,
		},
		{
			name:        "success recursive",
			dataset:     "tank/enc",
			recursive:   true,
			expectError: false,
		},
		{
			name:        "zfs error",
			dataset:     "tank/enc",
			mockError:   true,
			expectError: true,
		},
		{
			name:        "empty name",
			dataset:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			client := &zfs{
				cmd: Cmd{Bin: "zfs", Runner: mockRunner},
			}

			if tt.dataset != "" && (!tt.expectError || tt.mockError) {
				expectedCmd := "zfs load-key"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset

				if tt.mockError {
					mockRunner.AddCommand(expectedCmd, "", "load-key error", fmt.Errorf("exit status 1"))
				} else {
					mockRunner.AddCommand(expectedCmd, "", "", nil)
				}
			}

			err := client.LoadKey(ctx, tt.dataset, tt.recursive)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestZFS_UnloadKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     string
		recursive   bool
		mockError   bool
		expectError bool
	}{
		{
			name:        "success",
			dataset:     "tank/enc",
			recursive:   false,
			expectError: false,
		},
		{
			name:        "success recursive",
			dataset:     "tank/enc",
			recursive:   true,
			expectError: false,
		},
		{
			name:        "zfs error",
			dataset:     "tank/enc",
			mockError:   true,
			expectError: true,
		},
		{
			name:        "empty name",
			dataset:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			client := &zfs{
				cmd: Cmd{Bin: "zfs", Runner: mockRunner},
			}

			if tt.dataset != "" && (!tt.expectError || tt.mockError) {
				expectedCmd := "zfs unload-key"
				if tt.recursive {
					expectedCmd += " -r"
				}
				expectedCmd += " " + tt.dataset

				if tt.mockError {
					mockRunner.AddCommand(expectedCmd, "", "unload-key error", fmt.Errorf("exit status 1"))
				} else {
					mockRunner.AddCommand(expectedCmd, "", "", nil)
				}
			}

			err := client.UnloadKey(ctx, tt.dataset, tt.recursive)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDataset_GetEncryptionProperties(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		dataset     *Dataset
		mockError   bool
		expectError bool
	}{
		{
			name: "success all properties",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			expectError: false,
		},
		{
			name: "property fetch error",
			dataset: &Dataset{
				z:    &zfs{cmd: Cmd{Bin: "zfs"}},
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			mockError:   true,
			expectError: true,
		},
		{
			name:        "nil dataset",
			dataset:     nil,
			expectError: true,
		},
		{
			name: "nil zfs client",
			dataset: &Dataset{
				Name: "tank/enc",
				Type: DatasetTypeFilesystem,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()
			if tt.dataset != nil && tt.dataset.z != nil {
				tt.dataset.z.cmd.Runner = mockRunner

				props := []string{"encryption", "keylocation", "keyformat", "keystatus", "encryptionroot"}
				for _, prop := range props {
					cmd := fmt.Sprintf("zfs get -p %s %s -j", prop, tt.dataset.Name)
					if tt.mockError {
						mockRunner.AddCommand(cmd, "", "get error", fmt.Errorf("exit status 1"))
					} else {
						mockRunner.AddCommand(cmd, testutil.ZFSGetEncryptionJSON, "", nil)
					}
				}
			}

			result, err := tt.dataset.GetEncryptionProperties(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				if result.Encryption != "aes-256-gcm" {
					t.Errorf("Expected encryption 'aes-256-gcm', got %q", result.Encryption)
				}
				if result.KeyLocation != "file:///etc/zfs/keys/abc-def-ghijkl" {
					t.Errorf("Expected keylocation 'file:///etc/zfs/keys/abc-def-ghijkl', got %q", result.KeyLocation)
				}
				if result.KeyFormat != "passphrase" {
					t.Errorf("Expected keyformat 'passphrase', got %q", result.KeyFormat)
				}
				if result.KeyStatus != "available" {
					t.Errorf("Expected keystatus 'available', got %q", result.KeyStatus)
				}
				if result.EncryptionRoot != "tank/enc" {
					t.Errorf("Expected encryptionroot 'tank/enc', got %q", result.EncryptionRoot)
				}
			}
		})
	}
}

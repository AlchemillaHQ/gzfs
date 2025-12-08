package gzfs

import (
	"context"
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
				expectedCmd := "zfs list -o all -p"
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
				expectedCmd := "zfs list -o all -p"
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

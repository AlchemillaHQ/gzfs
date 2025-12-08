package gzfs

import (
	"context"
	"testing"

	"github.com/alchemillahq/gzfs/testutil"
)

func TestZpool_List(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		mockJSON    string
		expectError bool
		expectedLen int
	}{
		{
			name:        "successful list",
			mockJSON:    testutil.ZPoolListJSON,
			expectError: false,
			expectedLen: 1,
		},
		{
			name:        "command error",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			client := &zpool{
				cmd: Cmd{
					Bin:    "zpool",
					Runner: mockRunner,
				},
			}

			if tt.expectError {
				// Don't add a mock response to trigger error
			} else {
				expectedCmd := "zpool list -o all -p -P -j"
				mockRunner.AddCommand(expectedCmd, tt.mockJSON, "", nil)
			}

			pools, err := client.List(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if len(pools) != tt.expectedLen {
					t.Errorf("Expected %d pools, got %d", tt.expectedLen, len(pools))
				}

				for _, pool := range pools {
					if pool.z == nil {
						t.Error("Pool should have zpool client reference")
					}
					if pool.Size == 0 {
						t.Error("Pool size should be parsed and non-zero")
					}
				}
			}
		})
	}
}

func TestZpool_Get(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		poolName    string
		mockJSON    string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "successful get",
			poolName:    "tank",
			mockJSON:    testutil.ZPoolListJSON,
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "pool not found",
			poolName:    "nonexistent",
			mockJSON:    testutil.ZPoolListJSON,
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "command error",
			poolName:    "tank",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			client := &zpool{
				cmd: Cmd{
					Bin:    "zpool",
					Runner: mockRunner,
				},
			}

			if !tt.expectError {
				expectedCmd := "zpool list -o all -p " + tt.poolName + " -P -j"
				mockRunner.AddCommand(expectedCmd, tt.mockJSON, "", nil)
			}

			pool, err := client.Get(ctx, tt.poolName)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectNil && pool != nil {
				t.Error("Expected nil pool")
			}
			if !tt.expectNil && !tt.expectError && pool == nil {
				t.Error("Expected non-nil pool")
			}

			if pool != nil {
				if pool.z == nil {
					t.Error("Pool should have zpool client reference")
				}
				if pool.Name != tt.poolName {
					t.Errorf("Expected pool name %q, got %q", tt.poolName, pool.Name)
				}
			}
		})
	}
}

func TestZpool_GetPoolNames(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		mockJSON    string
		expectError bool
		expected    []string
	}{
		{
			name:        "successful get names",
			mockJSON:    testutil.ZPoolListJSON,
			expectError: false,
			expected:    []string{"tank"},
		},
		{
			name:        "command error",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			client := &zpool{
				cmd: Cmd{
					Bin:    "zpool",
					Runner: mockRunner,
				},
			}

			if !tt.expectError {
				expectedCmd := "zpool list -o name -p -P -j"
				mockRunner.AddCommand(expectedCmd, tt.mockJSON, "", nil)
			}

			names, err := client.GetPoolNames(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if len(names) != len(tt.expected) {
					t.Errorf("Expected %d names, got %d", len(tt.expected), len(names))
				}
			}
		})
	}
}

func TestZpool_Create(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		poolName    string
		force       bool
		properties  map[string]string
		args        []string
		expectError bool
	}{
		{
			name:     "simple create",
			poolName: "testpool",
			force:    false,
			properties: map[string]string{
				"ashift": "12",
			},
			args:        []string{"/dev/da0"},
			expectError: false,
		},
		{
			name:        "create with force",
			poolName:    "testpool",
			force:       true,
			properties:  map[string]string{},
			args:        []string{"/dev/da0", "/dev/da1"},
			expectError: false,
		},
		{
			name:        "command error",
			poolName:    "testpool",
			force:       false,
			properties:  map[string]string{},
			args:        []string{"/dev/nonexistent"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			client := &zpool{
				cmd: Cmd{
					Bin:    "zpool",
					Runner: mockRunner,
				},
			}

			if !tt.expectError {
				mockRunner.AddCommand("zpool create", "", "", nil)
			}

			err := client.Create(ctx, tt.poolName, tt.force, tt.properties, tt.args...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestZPool_Methods(t *testing.T) {
	ctx := context.Background()

	mockRunner := testutil.NewMockRunner()
	pool := &ZPool{
		Name:     "tank",
		PoolGUID: "12345678901234567890",
		z: &zpool{
			cmd: Cmd{
				Bin:    "zpool",
				Runner: mockRunner,
			},
		},
	}

	t.Run("Destroy", func(t *testing.T) {
		mockRunner.AddCommand("zpool destroy tank", "", "", nil)

		err := pool.Destroy(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Scrub", func(t *testing.T) {
		mockRunner.AddCommand("zpool scrub tank", "", "", nil)

		err := pool.Scrub(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("AddSpare", func(t *testing.T) {
		mockRunner.AddCommand("zpool add tank spare /dev/da2", "", "", nil)

		err := pool.AddSpare(ctx, "/dev/da2", false)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("RemoveSpare", func(t *testing.T) {
		mockRunner.AddCommand("zpool remove tank /dev/da2", "", "", nil)

		err := pool.RemoveSpare(ctx, "/dev/da2")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("GetProperty", func(t *testing.T) {
		pool.Properties = map[string]ZFSProperty{
			"size": {
				Value: "10G",
				Source: ZFSPropertySource{
					Type: "default",
				},
			},
		}

		prop, err := pool.GetProperty("size")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if prop.Value != "10G" {
			t.Errorf("Expected property value '10G', got %q", prop.Value)
		}

		_, err = pool.GetProperty("nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent property")
		}
	})
}

package gzfs

import (
	"context"
	"testing"
	"time"

	"github.com/alchemillahq/gzfs/testutil"
)

func TestZDB_GetPool(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		poolName     string
		currentGUID  string
		mockOutput   string
		expectError  bool
		expectedName string
		expectedGUID string
	}{
		{
			name:         "successful get pool",
			poolName:     "tank",
			currentGUID:  "",
			mockOutput:   testutil.ZDBOutput,
			expectError:  false,
			expectedName: "tank",
			expectedGUID: "",
		},
		{
			name:        "command error",
			poolName:    "tank",
			currentGUID: "",
			expectError: true,
		},
		{
			name:        "empty output",
			poolName:    "tank",
			currentGUID: "",
			mockOutput:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := testutil.NewMockRunner()

			client := &zdb{
				cmd: Cmd{
					Bin:    "zdb",
					Runner: mockRunner,
				},
				cacheTTL: 0, // Disable caching for tests
			}

			if !tt.expectError {
				expectedCmd := "zdb -C " + tt.poolName
				mockRunner.AddCommand(expectedCmd, tt.mockOutput, "", nil)
			}

			pool, err := client.GetPool(ctx, tt.poolName, tt.currentGUID)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if pool == nil {
					t.Fatal("Expected non-nil pool")
				}
				if pool.Name != tt.expectedName {
					t.Errorf("Expected pool name %q, got %q", tt.expectedName, pool.Name)
				}
				if pool.GUID != tt.expectedGUID {
					t.Errorf("Expected pool GUID %q, got %q", tt.expectedGUID, pool.GUID)
				}

				// Check that we have parsed some children
				if len(pool.Children) == 0 {
					t.Error("Expected pool to have children VDEVs")
				}
			}
		})
	}
}

func TestZDB_Caching(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zdb{
		cmd: Cmd{
			Bin:    "zdb",
			Runner: mockRunner,
		},
		cacheTTL: 100 * time.Millisecond, // Short TTL for testing
	}

	poolName := "tank"
	expectedCmd := "zdb -C " + poolName
	mockRunner.AddCommand(expectedCmd, testutil.ZDBOutput, "", nil)

	t.Run("first call hits command", func(t *testing.T) {
		pool, err := client.GetPool(ctx, poolName, "")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pool == nil {
			t.Fatal("Expected non-nil pool")
		}

		// Should have one call recorded
		if len(mockRunner.CallHistory) != 1 {
			t.Errorf("Expected 1 command call, got %d", len(mockRunner.CallHistory))
		}
	})

	t.Run("second call uses cache", func(t *testing.T) {
		pool, err := client.GetPool(ctx, poolName, "")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pool == nil {
			t.Fatal("Expected non-nil pool")
		}

		// Should still have only one call (cached)
		if len(mockRunner.CallHistory) != 1 {
			t.Errorf("Expected 1 command call (cached), got %d", len(mockRunner.CallHistory))
		}
	})

	t.Run("call after cache expiry hits command again", func(t *testing.T) {
		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)

		pool, err := client.GetPool(ctx, poolName, "")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pool == nil {
			t.Fatal("Expected non-nil pool")
		}

		// Should have two calls now (cache expired)
		if len(mockRunner.CallHistory) != 2 {
			t.Errorf("Expected 2 command calls (cache expired), got %d", len(mockRunner.CallHistory))
		}
	})
}

func TestZDBPoolChild_parseProp(t *testing.T) {
	tests := []struct {
		name     string
		prop     string
		val      string
		expected ZDBPoolChild
		hasError bool
	}{
		{
			name: "parse type",
			prop: "type",
			val:  "root",
			expected: ZDBPoolChild{
				Type: "root",
				Properties: map[string]string{
					"type": "root",
				},
			},
		},
		{
			name: "parse id",
			prop: "id",
			val:  "0",
			expected: ZDBPoolChild{
				ID: 0,
				Properties: map[string]string{
					"id": "0",
				},
			},
		},
		{
			name: "parse guid",
			prop: "guid",
			val:  "12345678901234567890",
			expected: ZDBPoolChild{
				GUID: 12345678901234567890,
				Properties: map[string]string{
					"guid": "12345678901234567890",
				},
			},
		},
		{
			name: "parse path",
			prop: "path",
			val:  "/dev/ada0p3",
			expected: ZDBPoolChild{
				Path: "/dev/ada0p3",
				Properties: map[string]string{
					"path": "/dev/ada0p3",
				},
			},
		},
		{
			name: "parse asize",
			prop: "asize",
			val:  "10737418240",
			expected: ZDBPoolChild{
				Asize: 10737418240,
				Properties: map[string]string{
					"asize": "10737418240",
				},
			},
		},
		{
			name:     "parse invalid id",
			prop:     "id",
			val:      "invalid",
			hasError: true,
		},
		{
			name:     "parse invalid guid",
			prop:     "guid",
			val:      "invalid",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := &ZDBPoolChild{}
			err := child.parseProp(tt.prop, tt.val)

			if tt.hasError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.hasError {
				// Check specific fields
				switch tt.prop {
				case "type":
					if child.Type != tt.expected.Type {
						t.Errorf("Expected Type %q, got %q", tt.expected.Type, child.Type)
					}
				case "id":
					if child.ID != tt.expected.ID {
						t.Errorf("Expected ID %d, got %d", tt.expected.ID, child.ID)
					}
				case "guid":
					if child.GUID != tt.expected.GUID {
						t.Errorf("Expected GUID %d, got %d", tt.expected.GUID, child.GUID)
					}
				case "path":
					if child.Path != tt.expected.Path {
						t.Errorf("Expected Path %q, got %q", tt.expected.Path, child.Path)
					}
				case "asize":
					if child.Asize != tt.expected.Asize {
						t.Errorf("Expected Asize %d, got %d", tt.expected.Asize, child.Asize)
					}
				}

				// Check properties map
				if child.Properties == nil {
					t.Error("Expected properties to be initialized")
				} else if child.Properties[tt.prop] != tt.val {
					t.Errorf("Expected property %q to have value %q, got %q", tt.prop, tt.val, child.Properties[tt.prop])
				}
			}
		})
	}
}

func TestZDBPool_parseLine(t *testing.T) {
	tests := []struct {
		name     string
		prop     string
		val      string
		expected ZDBPool
	}{
		{
			name: "parse version",
			prop: "version",
			val:  "5000",
			expected: ZDBPool{
				Version: "5000",
			},
		},
		{
			name: "parse name",
			prop: "name",
			val:  "tank",
			expected: ZDBPool{
				Name: "tank",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &ZDBPool{}
			pool.parseLine(tt.prop, tt.val)

			switch tt.prop {
			case "version":
				if pool.Version != tt.expected.Version {
					t.Errorf("Expected Version %q, got %q", tt.expected.Version, pool.Version)
				}
			case "name":
				if pool.Name != tt.expected.Name {
					t.Errorf("Expected Name %q, got %q", tt.expected.Name, pool.Name)
				}
			}
		})
	}
}

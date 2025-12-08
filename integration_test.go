//go:build integration
// +build integration

package gzfs_test

import (
	"context"
	"os"
	"testing"

	"github.com/alchemillahq/gzfs"
)

// Integration tests require actual ZFS commands to be available
// Run with: go test -tags=integration

func TestIntegration_ZFS_List(t *testing.T) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}

	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false, // Set to true if ZFS commands require sudo
	})

	datasets, err := client.ZFS.List(ctx, false)
	if err != nil {
		t.Fatalf("Failed to list datasets: %v", err)
	}

	t.Logf("Found %d datasets", len(datasets))
	for _, ds := range datasets {
		t.Logf("Dataset: %s, Type: %s, Used: %d, Available: %d",
			ds.Name, ds.Type, ds.Used, ds.Available)
	}
}

func TestIntegration_ZPool_List(t *testing.T) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}

	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false, // Set to true if zpool commands require sudo
	})

	pools, err := client.Zpool.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list pools: %v", err)
	}

	t.Logf("Found %d pools", len(pools))
	for _, pool := range pools {
		t.Logf("Pool: %s, State: %s, Size: %d, Free: %d",
			pool.Name, pool.State, pool.Size, pool.Free)

		// Test ZDB integration
		zdbPool, err := pool.ZDB(ctx)
		if err != nil {
			t.Errorf("Failed to get ZDB info for pool %s: %v", pool.Name, err)
			continue
		}

		t.Logf("ZDB Pool: %s, Version: %s, Children: %d",
			zdbPool.Name, zdbPool.Version, len(zdbPool.Children))
	}
}

func TestIntegration_ZFS_GetProperty(t *testing.T) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}

	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false,
	})

	// First get a list of datasets to test with
	datasets, err := client.ZFS.List(ctx, false)
	if err != nil {
		t.Fatalf("Failed to list datasets: %v", err)
	}

	if len(datasets) == 0 {
		t.Skip("No datasets found for testing")
	}

	// Test getting a property from the first dataset
	dataset := datasets[0]
	prop, err := client.ZFS.GetProperty(ctx, dataset.Name, "compression")
	if err != nil {
		t.Fatalf("Failed to get compression property for %s: %v", dataset.Name, err)
	}

	t.Logf("Dataset %s compression: %s (source: %s)",
		dataset.Name, prop.Value, prop.Source.Type)
}

// Benchmark tests
func BenchmarkZFS_List(b *testing.B) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") == "" {
		b.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}

	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ZFS.List(ctx, false)
		if err != nil {
			b.Fatalf("Failed to list datasets: %v", err)
		}
	}
}

func BenchmarkZPool_List(b *testing.B) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") == "" {
		b.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}

	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Zpool.List(ctx)
		if err != nil {
			b.Fatalf("Failed to list pools: %v", err)
		}
	}
}

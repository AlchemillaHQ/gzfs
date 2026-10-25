//go:build integration
// +build integration

package gzfs_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alchemillahq/gzfs"
)

func TestIntegration_ZFS_List(t *testing.T) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") == "" {
		t.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}

	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false,
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
		Sudo: false,
	})

	pools, err := client.Zpool.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list pools: %v", err)
	}

	t.Logf("Found %d pools", len(pools))
	for _, pool := range pools {
		t.Logf("Pool: %s, State: %s, Size: %d, Free: %d",
			pool.Name, pool.State, pool.Size, pool.Free)

		zdbPool, err := pool.ZDB(ctx)
		if err != nil {
			t.Errorf("Failed to get ZDB info for pool %s: %v", pool.Name, err)
			continue
		}

		t.Logf("ZDB Pool: %s, Version: %s, Children: %d",
			zdbPool.Name, zdbPool.Version, len(zdbPool.Children))
	}
}

func TestIntegration_ZPool_Lifecycle(t *testing.T) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") != "1" {
		t.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}
	if runtime.GOOS != "freebsd" {
		t.Skip("Pool lifecycle integration test requires FreeBSD")
	}
	if os.Geteuid() != 0 {
		t.Skip("Pool lifecycle integration test requires root privileges")
	}

	dir, err := os.MkdirTemp("", "gzfs-lifecycle-")
	if err != nil {
		t.Fatalf("Failed to create scratch directory: %v", err)
	}
	name := filepath.Base(dir)
	device := filepath.Join(dir, "vdev")
	client := gzfs.NewClient(gzfs.Options{})
	cmd := gzfs.Cmd{Bin: "zpool"}
	importPool := func(ctx context.Context) error {
		_, _, err := cmd.RunBytes(ctx, nil, "import", "-d", dir, "-N", "-o", "cachefile=none", name)
		return err
	}
	created, exported := false, false
	t.Cleanup(func() {
		if created {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if exported {
				if err := importPool(ctx); err != nil {
					t.Errorf("Failed to import scratch pool during cleanup; retaining %s: %v", dir, err)
					return
				}
			}
			if err := client.Zpool.Destroy(ctx, name, true); err != nil {
				t.Errorf("Failed to destroy scratch pool during cleanup; retaining %s: %v", dir, err)
				return
			}
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("Failed to remove scratch directory: %v", err)
		}
	})

	file, err := os.Create(device)
	if err != nil {
		t.Fatalf("Failed to create file vdev: %v", err)
	}
	err = file.Truncate(128 << 20)
	closeErr := file.Close()
	if err != nil {
		t.Fatalf("Failed to size file vdev: %v", err)
	}
	if closeErr != nil {
		t.Fatalf("Failed to close file vdev: %v", closeErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := client.Zpool.CreateWithOptions(ctx, name, gzfs.ZPoolCreateOptions{
		Mountpoint: "none",
		Properties: map[string]string{"cachefile": "none"},
	}, device); err != nil {
		t.Fatalf("Failed to create scratch pool: %v", err)
	}
	created = true
	if err := client.Zpool.Export(ctx, name); err != nil {
		t.Fatalf("Failed to export scratch pool: %v", err)
	}
	exported = true
	if err := importPool(ctx); err != nil {
		t.Fatalf("Failed to import scratch pool: %v", err)
	}
	exported = false
	if err := client.Zpool.Destroy(ctx, name, true); err != nil {
		t.Fatalf("Failed to force-destroy scratch pool: %v", err)
	}
	created = false
	if err := client.Zpool.LabelClear(ctx, device, true); err != nil {
		t.Fatalf("Failed to clear scratch vdev labels: %v", err)
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

	datasets, err := client.ZFS.List(ctx, false)
	if err != nil {
		t.Fatalf("Failed to list datasets: %v", err)
	}

	if len(datasets) == 0 {
		t.Skip("No datasets found for testing")
	}

	dataset := datasets[0]
	prop, err := client.ZFS.GetProperty(ctx, dataset.Name, "compression")
	if err != nil {
		t.Fatalf("Failed to get compression property for %s: %v", dataset.Name, err)
	}

	t.Logf("Dataset %s compression: %s (source: %s)",
		dataset.Name, prop.Value, prop.Source.Type)
}

func TestIntegration_ZFS_Inspection(t *testing.T) {
	if os.Getenv("GZFS_INTEGRATION_TESTS") != "1" {
		t.Skip("Integration tests disabled. Set GZFS_INTEGRATION_TESTS=1 to enable.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := gzfs.NewClient(gzfs.Options{})
	datasets, err := client.ZFS.ListByType(ctx, gzfs.DatasetTypeFilesystem, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(datasets) == 0 {
		t.Skip("No filesystems available for inspection")
	}
	previous := ""
	for _, dataset := range datasets {
		if dataset.Name < previous {
			t.Fatalf("datasets are not ordered by name: %q before %q", previous, dataset.Name)
		}
		previous = dataset.Name
		for _, name := range []string{"sharenfs", "keystatus", "canmount", "acltype"} {
			if _, ok := dataset.Properties[name]; !ok {
				t.Errorf("dataset %s is missing inspection property %s", dataset.Name, name)
			}
		}
	}
	dataset := datasets[0]
	properties, err := dataset.GetProperties(ctx, "sharenfs", "keystatus", "canmount", "acltype")
	if err != nil {
		t.Fatal(err)
	}
	for name, property := range properties {
		t.Logf("%s: %s=%q source=%s inherited-from=%s", dataset.Name, name, property.Value, property.Source.Type, property.Source.Data)
	}
}

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

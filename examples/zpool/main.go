package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/alchemillahq/gzfs"
)

func main() {
	ctx := context.Background()
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false,
	})

	var pools []*gzfs.ZPool
	var err error

	if len(os.Args) >= 2 {
		poolName := os.Args[1]
		pool, err := client.Zpool.Get(ctx, poolName)
		if err != nil {
			log.Fatalf("zpool error for pool %q: %v", poolName, err)
		}

		if pool == nil {
			log.Fatalf("zpool %q not found", poolName)
		}

		fmt.Printf("Zpool info for %q\n", poolName)
		fmt.Printf("  Name:       %s\n", pool.Name)
		fmt.Printf("  Type:       %s\n", pool.Type)
		fmt.Printf("  State:      %s\n", pool.State)
		fmt.Printf("  PoolGUID:   %s\n", pool.PoolGUID)
		fmt.Printf("  TXG:        %s\n", pool.TXG)
		fmt.Printf("  SPAVersion: %s\n", pool.SPAVersion)
		fmt.Printf("  ZPLVersion: %s\n", pool.ZPLVersion)
		fmt.Printf("  Size:       %d\n", pool.Size)
		fmt.Printf("  Free:       %d\n", pool.Free)
		fmt.Printf("  Allocated:  %d\n", pool.Alloc)

		zdbPool, err := pool.ZDB(ctx)
		if err != nil {
			log.Fatalf("zdb error for pool %q: %v", poolName, err)
		}
		fmt.Printf("  ZDB Pool Name: %s, GUID: %s\n", zdbPool.Name, zdbPool.GUID)

		status, err := pool.Status(ctx)
		if err != nil {
			log.Fatalf("zpool status error for pool %q: %v", poolName, err)
		}

		fmt.Printf("  Status State: %s\n", status.Pools[pool.Name].State)
	} else {
		pools, err = client.Zpool.List(ctx)
		if err != nil {
			log.Fatalf("zpool list error: %v", err)
		}

		fmt.Println("Zpool List:")
		for _, pool := range pools {
			fmt.Printf("- Name: %s, Type: %s, State: %s\n", pool.Name, pool.Type, pool.State)
			zdbPool, err := pool.ZDB(ctx)
			if err != nil {
				log.Fatalf("zdb error for pool %q: %v", pool.Name, err)
			}
			fmt.Printf("  ZDB Pool Name: %s, GUID: %s\n", zdbPool.Name, zdbPool.GUID)
			status, err := pool.Status(ctx)
			if err != nil {
				log.Fatalf("zpool status error for pool %q: %v", pool.Name, err)
			}
			fmt.Printf("  Status State: %s\n", status.Pools[pool.Name].State)
		}
	}
}

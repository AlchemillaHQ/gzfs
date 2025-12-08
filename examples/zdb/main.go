package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/alchemillahq/gzfs"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <pool-name>\n", os.Args[0])
		os.Exit(1)
	}
	poolName := os.Args[1]

	ctx := context.Background()

	// Create a client. This assumes zdb/zpool/zfs all need sudo.
	client := gzfs.NewClient(gzfs.Options{
		Sudo: false,
	})

	// We don't have a GUID helper wired yet, so pass "" for now.
	pool, err := client.ZDB.GetPool(ctx, poolName, "")
	if err != nil {
		log.Fatalf("zdb error for pool %q: %v", poolName, err)
	}

	fmt.Printf("ZDB pool info for %q\n", poolName)
	fmt.Printf("  Name:    %s\n", pool.Name)
	fmt.Printf("  GUID:    %s\n", pool.GUID)
	fmt.Printf("  Version: %s\n", pool.Version)
	fmt.Println("  VDEVs:")

	for _, child := range pool.Children {
		printVdev(child, 2)
	}
}

// printVdev prints a ZDBPoolChild tree with indentation.
func printVdev(v gzfs.ZDBPoolChild, indent int) {
	prefix := spaces(indent)
	fmt.Printf("%s- type=%s", prefix, v.Type)
	if v.Path != "" {
		fmt.Printf(" path=%s", v.Path)
	}
	if v.GUID != 0 {
		fmt.Printf(" guid=%d", v.GUID)
	}
	fmt.Println()

	for _, ch := range v.Children {
		printVdev(ch, indent+2)
	}
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%*s", n, "")
}

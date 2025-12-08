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

	var datasets []*gzfs.Dataset
	var err error

	if len(os.Args) >= 2 {
		datasetName := os.Args[1]
		dataset, err := client.ZFS.Get(ctx, datasetName, false)
		if err != nil {
			log.Fatalf("zfs error for dataset %q: %v", datasetName, err)
		}

		if dataset == nil {
			log.Fatalf("dataset %q not found", datasetName)
		}

		fmt.Printf("Dataset info for %q\n", datasetName)
		fmt.Printf("  Name:         %s\n", dataset.Name)
		fmt.Printf("  Type:         %s\n", dataset.Type)
		fmt.Printf("  Mountpoint:   %s\n", dataset.Mountpoint)
		fmt.Printf("  Used:         %d\n", dataset.Used)
		fmt.Printf("  Available:    %d\n", dataset.Available)
		fmt.Printf("  Referenced:   %d\n", dataset.Referenced)
		fmt.Printf("  Compressratio:%f\n", dataset.Compressratio)
	} else {
		datasets, err = client.ZFS.List(ctx, false, "")
		if err != nil {
			log.Fatalf("zfs list error: %v", err)
		}

		fmt.Println("Datasets:")
		for _, ds := range datasets {
			fmt.Printf("  - %s (Type: %s)\n", ds.Name, ds.Type)
		}
	}
}

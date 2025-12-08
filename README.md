# gzfs

A Go library for interacting with ZFS (Zettabyte File System) on FreeBSD and other ZFS-compatible systems. This library provides a clean, idiomatic Go interface for ZFS, ZPool, and ZDB operations.

[![ZFS Version](https://img.shields.io/badge/ZFS-%3E%3D2.3.0-blue)](https://openzfs.org/)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-BSD%202--Clause-green)](LICENSE)

## Features

- **Core ZFS Operations**: Create, list, get, snapshot, clone, destroy datasets and volumes
- **ZPool Management**: Create, destroy, scrub pools, manage spares and cache devices
- **ZDB Integration**: Low-level pool inspection with caching support
- **Type Safety**: Strongly typed structs for all ZFS objects and properties
- **Mock Testing**: Comprehensive test suite with mock runner for development
- **Flexible Command Execution**: Support for sudo, custom binaries, and command runners
- **JSON Parsing**: Native parsing of ZFS JSON output for reliability

## Installation

```bash
go get github.com/alchemillahq/gzfs@latest
```

## Requirements

- OpenZFS >= 2.3.0
- `zfs`, `zpool`, and `zdb` available in PATH
- Root privileges or `sudo` for most write operations

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/alchemillahq/gzfs"
)

func main() {
    ctx := context.Background()
    
    // Create a client (set Sudo: true if ZFS commands require privileges)
    client := gzfs.NewClient(gzfs.Options{
        Sudo: false,
    })

    // List all datasets
    datasets, err := client.ZFS.List(ctx, false)
    if err != nil {
        log.Fatalf("Failed to list datasets: %v", err)
    }

    for _, ds := range datasets {
        fmt.Printf("Dataset: %s, Type: %s, Used: %d bytes\n", 
            ds.Name, ds.Type, ds.Used)
    }

    // List all pools
    pools, err := client.Zpool.List(ctx)
    if err != nil {
        log.Fatalf("Failed to list pools: %v", err)
    }

    for _, pool := range pools {
        fmt.Printf("Pool: %s, State: %s, Size: %d bytes\n", 
            pool.Name, pool.State, pool.Size)
    }
}
```

## License

This project is licensed under the BSD-2-Clause License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- The ZFS development team for creating an amazing filesystem
- The OpenZFS project for cross-platform ZFS support
- The FreeBSD project for excellent ZFS integration

## Support

- **Issues**: [GitHub Issues](https://github.com/alchemillahq/gzfs/issues)
- **Documentation**: See [TESTING.md](TESTING.md) for testing guide

---

**Note**: This library executes ZFS commands directly and requires appropriate permissions. Always test in a safe environment before using in production.

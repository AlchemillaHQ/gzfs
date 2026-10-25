# gzfs

Go wrappers for OpenZFS `zfs`, `zpool`, and `zdb` commands.

## Requirements

- Go 1.24.9+ and OpenZFS 2.3.0+.
- ZFS tools in `PATH`, or custom paths through `Options`.
- Appropriate privileges for the requested operation; `Options.Sudo` enables sudo.

## Installation

```sh
go get github.com/alchemillahq/gzfs@latest
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alchemillahq/gzfs"
)

func main() {
	client := gzfs.NewClient(gzfs.Options{})
	datasets, err := client.ZFS.List(context.Background(), false)
	if err != nil {
		log.Fatal(err)
	}
	for _, dataset := range datasets {
		fmt.Printf("%s: %d bytes used\n", dataset.Name, dataset.Used)
	}
}
```

See [examples](examples) for more dataset, pool, and ZDB usage.

## Testing

See [TESTING.md](TESTING.md). Native integration tests require explicit opt-in.

## License

[BSD-2-Clause](LICENSE.md).

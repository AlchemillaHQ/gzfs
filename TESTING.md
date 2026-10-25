# Testing

Unit tests use mocked ZFS commands:

```sh
go test ./...
go test -race ./...
go vet ./...
```

## Native integration tests

These require OpenZFS tools, `-tags=integration`, and `GZFS_INTEGRATION_TESTS=1`.

Read-only dataset/property inspection:

```sh
GZFS_INTEGRATION_TESTS=1 go test -tags=integration -run '^TestIntegration_ZFS_Inspection$' -v
```

The full suite includes a FreeBSD pool lifecycle test requiring root. It creates
a temporary 128 MiB file vdev, tests create/export/import/destroy/label-clear, and
retains the scratch directory if pool cleanup fails. It does not use existing
pools or disks.

```sh
GZFS_INTEGRATION_TESTS=1 go test -tags=integration ./...
```

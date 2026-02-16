package gzfs

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alchemillahq/gzfs/testutil"
)

func snapshotDatasetJSON(name string) string {
	return fmt.Sprintf(`{
  "output_version": {
    "command": "zfs list",
    "vers_major": 0,
    "vers_minor": 1
  },
  "datasets": {
    %q: {
      "name": %q,
      "type": "SNAPSHOT",
      "pool": "tank",
      "createtxg": "1",
      "properties": {
        "guid": {
          "value": "123",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "mountpoint": {
          "value": "-",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "used": {
          "value": "1",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "available": {
          "value": "0",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "referenced": {
          "value": "1",
          "source": {
            "type": "default",
            "data": ""
          }
        },
        "compressratio": {
          "value": "1.00x",
          "source": {
            "type": "default",
            "data": ""
          }
        }
      }
    }
  }
}`, name, name)
}

func getSnapshotCmd(name string) string {
	return "zfs list -o " + strings.Join(dsPropList, ",") + " -p " + name + " -j"
}

func TestZFS_SendSnapshot(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zfs{
		cmd: Cmd{
			Bin:    "zfs",
			Runner: mockRunner,
		},
	}

	snapshot := "tank/app@snap-001"
	mockRunner.AddCommand(getSnapshotCmd(snapshot), snapshotDatasetJSON(snapshot), "", nil)
	mockRunner.AddCommand("zfs send "+snapshot, "stream-bytes", "", nil)

	var out bytes.Buffer
	err := client.SendSnapshot(ctx, snapshot, &out)
	if err != nil {
		t.Fatalf("SendSnapshot returned error: %v", err)
	}

	if got, want := out.String(), "stream-bytes"; got != want {
		t.Fatalf("unexpected stream output: got %q want %q", got, want)
	}
}

func TestZFS_SendIncremental(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zfs{
		cmd: Cmd{
			Bin:    "zfs",
			Runner: mockRunner,
		},
	}

	base := "tank/app@snap-001"
	target := "tank/app@snap-002"

	mockRunner.AddCommand(getSnapshotCmd(base), snapshotDatasetJSON(base), "", nil)
	mockRunner.AddCommand(getSnapshotCmd(target), snapshotDatasetJSON(target), "", nil)
	mockRunner.AddCommand("zfs send -i "+base+" "+target, "inc-stream", "", nil)

	var out bytes.Buffer
	err := client.SendIncremental(ctx, base, target, &out)
	if err != nil {
		t.Fatalf("SendIncremental returned error: %v", err)
	}

	if got, want := out.String(), "inc-stream"; got != want {
		t.Fatalf("unexpected stream output: got %q want %q", got, want)
	}
}

func TestZFS_SendIncrementalWithIntermediates(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zfs{
		cmd: Cmd{
			Bin:    "zfs",
			Runner: mockRunner,
		},
	}

	base := "tank/app@snap-001"
	target := "tank/app@snap-003"

	mockRunner.AddCommand(getSnapshotCmd(base), snapshotDatasetJSON(base), "", nil)
	mockRunner.AddCommand(getSnapshotCmd(target), snapshotDatasetJSON(target), "", nil)
	mockRunner.AddCommand("zfs send -I "+base+" "+target, "inc-stream-I", "", nil)

	var out bytes.Buffer
	err := client.SendIncrementalWithIntermediates(ctx, base, target, &out)
	if err != nil {
		t.Fatalf("SendIncrementalWithIntermediates returned error: %v", err)
	}

	if got, want := out.String(), "inc-stream-I"; got != want {
		t.Fatalf("unexpected stream output: got %q want %q", got, want)
	}
}

func TestZFS_SendIncremental_DifferentDatasets(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zfs{
		cmd: Cmd{
			Bin:    "zfs",
			Runner: mockRunner,
		},
	}

	base := "tank/a@snap-001"
	target := "tank/b@snap-002"

	mockRunner.AddCommand(getSnapshotCmd(base), snapshotDatasetJSON(base), "", nil)
	mockRunner.AddCommand(getSnapshotCmd(target), snapshotDatasetJSON(target), "", nil)

	var out bytes.Buffer
	err := client.SendIncremental(ctx, base, target, &out)
	if err == nil {
		t.Fatal("expected error for different source datasets, got nil")
	}
	if !strings.Contains(err.Error(), "incremental_snapshots_must_be_same_dataset") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestZFS_ReceiveStream(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zfs{
		cmd: Cmd{
			Bin:    "zfs",
			Runner: mockRunner,
		},
	}

	mockRunner.AddCommand("zfs recv -F tank/restore", "", "", nil)

	err := client.ReceiveStream(ctx, bytes.NewBufferString("stream"), "tank/restore", true)
	if err != nil {
		t.Fatalf("ReceiveStream returned error: %v", err)
	}
}

func TestDataset_SendIncremental(t *testing.T) {
	ctx := context.Background()
	mockRunner := testutil.NewMockRunner()

	client := &zfs{
		cmd: Cmd{
			Bin:    "zfs",
			Runner: mockRunner,
		},
	}

	base := "tank/app@snap-001"
	target := "tank/app@snap-002"

	mockRunner.AddCommand(getSnapshotCmd(base), snapshotDatasetJSON(base), "", nil)
	mockRunner.AddCommand(getSnapshotCmd(target), snapshotDatasetJSON(target), "", nil)
	mockRunner.AddCommand("zfs send -i "+base+" "+target, "inc-stream", "", nil)

	ds := &Dataset{
		z:    client,
		Name: target,
		Type: DatasetTypeSnapshot,
	}

	var out bytes.Buffer
	err := ds.SendIncremental(ctx, base, &out)
	if err != nil {
		t.Fatalf("Dataset.SendIncremental returned error: %v", err)
	}
}

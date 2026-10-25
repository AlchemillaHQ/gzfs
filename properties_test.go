package gzfs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/alchemillahq/gzfs/testutil"
)

const inspectionJSON = `{
  "datasets": {
    "tank/share": {
      "name": "tank/share",
      "type": "FILESYSTEM",
      "pool": "tank",
      "properties": {
        "guid": {"value": "18446744073709551615"},
        "mountpoint": {"value": "/mnt/shared space "},
        "mounted": {"value": "yes"},
        "sharenfs": {"value": "off", "source": {"type": "INHERITED", "data": "tank"}},
        "keystatus": {"value": "available"},
        "canmount": {"value": "on"},
        "acltype": {"value": "nfsv4"}
      }
    }
  }
}`

func TestZFS_GetProperties(t *testing.T) {
	for _, test := range []struct {
		name       string
		properties []string
		query      string
		count      int
	}{
		{"named properties", []string{"sharenfs", "keystatus", "canmount", "acltype"}, "sharenfs,keystatus,canmount,acltype", 4},
		{"deduplicated properties", []string{"sharenfs", "keystatus", "sharenfs"}, "sharenfs,keystatus", 2},
		{"all by default", nil, "all", 7},
		{"explicit all", []string{"all"}, "all", 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			client := NewClient(Options{Runner: runner})
			command := "zfs get -p " + test.query + " tank/share -j"
			runner.AddCommand(command, inspectionJSON, "", nil)
			original := append([]string(nil), test.properties...)

			properties, err := client.ZFS.GetProperties(context.Background(), "tank/share", test.properties...)
			if err != nil {
				t.Fatal(err)
			}
			if len(properties) != test.count {
				t.Fatalf("got %d properties, want %d: %#v", len(properties), test.count, properties)
			}
			if !reflect.DeepEqual(test.properties, original) {
				t.Fatal("query changed the caller's property slice")
			}
			if len(runner.CallHistory) != 1 || runner.CallHistory[0].Cmd != command {
				t.Fatalf("expected one batched command, got %#v", runner.CallHistory)
			}
			if properties["sharenfs"].Value != "off" || properties["sharenfs"].Source.Type != "INHERITED" || properties["sharenfs"].Source.Data != "tank" {
				t.Fatalf("inheritance metadata lost: %#v", properties["sharenfs"])
			}
		})
	}
}

func TestZFS_GetPropertiesRejectsIncompleteResponses(t *testing.T) {
	for _, test := range []struct {
		name string
		json string
	}{
		{"invalid JSON", `{"datasets":`},
		{"missing dataset", `{"datasets":{}}`},
		{"different dataset", `{"datasets":{"tank/other":{"properties":{"sharenfs":{"value":"off"},"keystatus":{"value":"available"}}}}}`},
		{"null dataset", `{"datasets":{"tank/share":null}}`},
		{"missing properties", `{"datasets":{"tank/share":{}}}`},
		{"empty properties", `{"datasets":{"tank/share":{"properties":{}}}}`},
		{"missing requested property", `{"datasets":{"tank/share":{"properties":{"sharenfs":{"value":"off"}}}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			runner.AddCommand("zfs get -p sharenfs,keystatus tank/share -j", test.json, "", nil)
			client := NewClient(Options{Runner: runner})
			properties, err := client.ZFS.GetProperties(context.Background(), "tank/share", "sharenfs", "keystatus")
			if err == nil || properties != nil {
				t.Fatalf("incomplete query returned a successful/partial result: %#v, %v", properties, err)
			}
		})
	}
}

func TestZFS_GetPropertiesRejectsInvalidArgumentsBeforeRunning(t *testing.T) {
	for _, names := range [][]string{
		{""}, {" "}, {"-r"}, {"sharenfs,keystatus"}, {"sharenfs\nacltype"}, {"sharenfs\x00"}, {"all", "sharenfs"},
	} {
		runner := testutil.NewMockRunner()
		client := NewClient(Options{Runner: runner})
		if _, err := client.ZFS.GetProperties(context.Background(), "tank/share", names...); err == nil {
			t.Errorf("accepted invalid property arguments %q", names)
		}
		if len(runner.CallHistory) != 0 {
			t.Errorf("invalid arguments ran a command: %#v", runner.CallHistory)
		}
	}
	for _, name := range []string{"", " ", "-r", "tank/share\x00", "tank/share\nother"} {
		runner := testutil.NewMockRunner()
		client := NewClient(Options{Runner: runner})
		if _, err := client.ZFS.GetProperties(context.Background(), name, "sharenfs"); err == nil {
			t.Errorf("accepted invalid dataset argument %q", name)
		}
		if len(runner.CallHistory) != 0 {
			t.Errorf("invalid dataset ran a command: %#v", runner.CallHistory)
		}
	}
	var client *zfs
	if _, err := client.GetProperties(context.Background(), "tank/share", "sharenfs"); err == nil {
		t.Fatal("nil client did not return an error")
	}
}

func TestZFS_GetPropertiesPreservesRawValues(t *testing.T) {
	for _, value := range []string{"-", "", "on", "rw=@192.0.2.0/24", " leading and trailing spaces "} {
		response := DatasetList{Datasets: map[string]*Dataset{
			"tank/share": {Properties: map[string]ZFSProperty{
				"sharenfs": {Value: value, Source: ZFSPropertySource{Type: "LOCAL", Data: "tank/share"}},
			}},
		}}
		data, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		runner := testutil.NewMockRunner()
		runner.AddCommand("zfs get -p sharenfs tank/share -j", string(data), "", nil)
		client := NewClient(Options{Runner: runner})
		property, err := client.ZFS.GetProperty(context.Background(), "tank/share", "sharenfs")
		if err != nil || property.Value != value || property.Source.Type != "LOCAL" {
			t.Fatalf("value %q was normalized or lost: %#v, %v", value, property, err)
		}
	}
}

func TestZFS_GetPropertyDoesNotReturnZeroValueForMissingProperty(t *testing.T) {
	for _, name := range []string{"missing", "all"} {
		runner := testutil.NewMockRunner()
		client := NewClient(Options{Runner: runner})
		runner.AddCommand("zfs get -p "+name+" tank/share -j", inspectionJSON, "", nil)
		property, err := client.ZFS.GetProperty(context.Background(), "tank/share", name)
		if err == nil || property != (ZFSProperty{}) {
			t.Fatalf("missing single property %q returned a successful value: %#v, %v", name, property, err)
		}
	}
}

func TestZFS_GetPropertiesUsesConfiguredCommandAndSingleArguments(t *testing.T) {
	runner := testutil.NewMockRunner()
	client := NewClient(Options{Runner: runner, Sudo: true, ZFSBin: "/custom/zfs"})
	response := `{"datasets":{"tank/share with spaces":{"properties":{"org.example:nfs":{"value":"metadata"}}}}}`
	runner.AddCommand("sudo /custom/zfs get -p org.example:nfs tank/share with spaces -j", response, "", nil)
	properties, err := client.ZFS.GetProperties(context.Background(), "tank/share with spaces", "org.example:nfs")
	if err != nil || properties["org.example:nfs"].Value != "metadata" {
		t.Fatalf("custom command query failed: %#v, %v", properties, err)
	}
	want := []string{"/custom/zfs", "get", "-p", "org.example:nfs", "tank/share with spaces", "-j"}
	if len(runner.CallHistory) != 1 || !reflect.DeepEqual(runner.CallHistory[0].Args, want) {
		t.Fatalf("arguments were split or changed: %#v", runner.CallHistory)
	}
}

type propertyRunnerFunc func(context.Context, io.Reader, io.Writer, io.Writer, string, ...string) error

func (run propertyRunnerFunc) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	return run(ctx, stdin, stdout, stderr, name, args...)
}

func TestZFS_GetPropertiesPropagatesContextAndCommandErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient(Options{Runner: propertyRunnerFunc(func(got context.Context, _ io.Reader, _, _ io.Writer, _ string, _ ...string) error {
		if got != ctx {
			t.Fatal("query did not pass the caller's context")
		}
		return got.Err()
	})})
	properties, err := client.ZFS.GetProperties(ctx, "tank/share", "sharenfs", "keystatus")
	var commandError *CmdError
	if properties != nil || !errors.Is(err, context.Canceled) || !errors.As(err, &commandError) {
		t.Fatalf("command error was not preserved: %#v, %v", properties, err)
	}
}

func TestDataset_GetPropertiesIsFreshWithoutChangingCachedSnapshot(t *testing.T) {
	runner := testutil.NewMockRunner()
	client := NewClient(Options{Runner: runner})
	dataset := &Dataset{
		z: client.ZFS, Name: "tank/share",
		Properties: map[string]ZFSProperty{"sharenfs": {Value: "on"}},
	}
	runner.AddCommand("zfs get -p sharenfs tank/share -j", inspectionJSON, "", nil)
	properties, err := dataset.GetProperties(context.Background(), "sharenfs")
	if err != nil || properties["sharenfs"].Value != "off" {
		t.Fatalf("fresh properties %#v, %v", properties, err)
	}
	cached, err := dataset.GetProperty(context.Background(), "sharenfs")
	if err != nil || cached.Value != "on" || len(runner.CallHistory) != 1 {
		t.Fatalf("cached property changed or ran a command: %#v, %v", cached, err)
	}
	for _, dataset := range []*Dataset{nil, {Name: "tank/share"}} {
		if _, err := dataset.GetProperties(context.Background(), "sharenfs"); err == nil {
			t.Fatalf("unattached dataset %#v did not return an error", dataset)
		}
	}
}

func TestDataset_GetEncryptionPropertiesRejectsPartialBatch(t *testing.T) {
	runner := testutil.NewMockRunner()
	client := NewClient(Options{Runner: runner})
	dataset := &Dataset{z: client.ZFS, Name: "tank/enc"}
	response := strings.Replace(testutil.ZFSGetEncryptionJSON, `"keystatus":`, `"other":`, 1)
	runner.AddCommand("zfs get -p encryption,keylocation,keyformat,keystatus,encryptionroot tank/enc -j", response, "", nil)
	properties, err := dataset.GetEncryptionProperties(context.Background())
	if err == nil || properties != nil || !strings.Contains(err.Error(), "keystatus") || len(runner.CallHistory) != 1 {
		t.Fatalf("partial encryption state was accepted: %#v, %v", properties, err)
	}
}

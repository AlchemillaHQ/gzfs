package gzfs

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/alchemillahq/gzfs/testutil"
)

func TestDatasetInspectionPropertiesAreListedTogether(t *testing.T) {
	for _, method := range []string{"List", "ListByType", "ListWithPrefix", "Get"} {
		t.Run(method, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			client := NewClient(Options{Runner: runner})
			runner.AddCommand("zfs list ", inspectionJSON, "", nil)
			var datasets []*Dataset
			var err error
			switch method {
			case "List":
				datasets, err = client.ZFS.List(context.Background(), true, "tank")
			case "ListByType":
				datasets, err = client.ZFS.ListByType(context.Background(), DatasetTypeFilesystem, true, "tank")
			case "ListWithPrefix":
				datasets, err = client.ZFS.ListWithPrefix(context.Background(), DatasetTypeFilesystem, "tank", true)
			case "Get":
				var dataset *Dataset
				dataset, err = client.ZFS.Get(context.Background(), "tank/share", false)
				datasets = []*Dataset{dataset}
			}
			if err != nil || len(datasets) != 1 || datasets[0] == nil {
				t.Fatalf("inspection result %#v, %v", datasets, err)
			}
			dataset := datasets[0]
			if dataset.z != client.ZFS || dataset.GUID != "18446744073709551615" || dataset.Mountpoint != "/mnt/shared space " || !dataset.IsMounted() {
				t.Fatalf("incorrect hydrated identity/path: %#v", dataset)
			}
			if len(runner.CallHistory) != 1 {
				t.Fatalf("inspection issued %d commands, want one", len(runner.CallHistory))
			}
			columns := strings.Split(runner.CallHistory[0].Args[2], ",")
			for _, property := range []string{"sharenfs", "keystatus", "canmount", "acltype"} {
				found := 0
				for _, column := range columns {
					if column == property {
						found++
					}
				}
				if found != 1 || dataset.Properties[property].Value == "" {
					t.Errorf("property %s not present exactly once: columns=%v properties=%#v", property, columns, dataset.Properties)
				}
			}
		})
	}
}

func TestDatasetMountpointPreservesWhitespace(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{"/mnt/share ", "/mnt/share "},
		{" /mnt/share", " /mnt/share"},
		{"/mnt/shared space", "/mnt/shared space"},
		{"/mnt/back\\slash", "/mnt/back\\slash"},
		{"legacy", "legacy"},
		{"none", "none"},
		{"-", ""},
		{"", ""},
	} {
		t.Run(test.value, func(t *testing.T) {
			dataset := &Dataset{Properties: map[string]ZFSProperty{"mountpoint": {Value: test.value}}}
			client := &zfs{}
			client.hydrateDataset(dataset)
			if dataset.Mountpoint != test.want || dataset.Properties["mountpoint"].Value != test.value {
				t.Fatalf("path changed: mountpoint=%q raw=%q", dataset.Mountpoint, dataset.Properties["mountpoint"].Value)
			}
		})
	}
}

func TestDataset_IsMounted(t *testing.T) {
	for _, test := range []struct {
		name    string
		dataset *Dataset
		want    bool
	}{
		{"nil", nil, false},
		{"missing state", &Dataset{Type: DatasetTypeFilesystem}, false},
		{"mounted", &Dataset{Type: DatasetTypeFilesystem, Properties: map[string]ZFSProperty{"mounted": {Value: "yes"}}}, true},
		{"unmounted", &Dataset{Type: DatasetTypeFilesystem, Properties: map[string]ZFSProperty{"mounted": {Value: "no"}}}, false},
		{"unknown", &Dataset{Type: DatasetTypeFilesystem, Properties: map[string]ZFSProperty{"mounted": {Value: "-"}}}, false},
		{"volume", &Dataset{Type: DatasetTypeVolume, Properties: map[string]ZFSProperty{"mounted": {Value: "yes"}}}, false},
		{"snapshot", &Dataset{Type: DatasetTypeSnapshot, Properties: map[string]ZFSProperty{"mounted": {Value: "yes"}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.dataset.IsMounted(); got != test.want {
				t.Fatalf("IsMounted = %t, want %t", got, test.want)
			}
		})
	}
}

func TestDatasetListsHaveDeterministicOrderAndRejectNullEntries(t *testing.T) {
	for _, method := range []string{"List", "ListByType", "ListWithPrefix"} {
		t.Run(method, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			client := NewClient(Options{Runner: runner})
			list := func() ([]*Dataset, error) {
				switch method {
				case "ListByType":
					return client.ZFS.ListByType(context.Background(), DatasetTypeFilesystem, true)
				case "ListWithPrefix":
					return client.ZFS.ListWithPrefix(context.Background(), DatasetTypeFilesystem, "tank", true)
				default:
					return client.ZFS.List(context.Background(), true)
				}
			}
			runner.AddCommand("zfs list ", `{"datasets":{"tank/z":{"name":"tank/z"},"tank/a":{"name":"tank/a"},"tank":{"name":"tank"}}}`, "", nil)
			for i := 0; i < 5; i++ {
				datasets, err := list()
				if err != nil {
					t.Fatal(err)
				}
				names := make([]string, 0, len(datasets))
				for _, dataset := range datasets {
					names = append(names, dataset.Name)
				}
				if !reflect.DeepEqual(names, []string{"tank", "tank/a", "tank/z"}) {
					t.Fatalf("unstable dataset order: %v", names)
				}
			}
			runner.AddCommand("zfs list ", `{"datasets":{"tank/a":{"name":"tank/a"},"tank/b":null}}`, "", nil)
			if datasets, err := list(); err == nil || datasets != nil {
				t.Fatalf("null entry returned partial/successful list: %#v, %v", datasets, err)
			}
		})
	}
}

func TestZFS_GetByGUIDRechecksIdentity(t *testing.T) {
	for _, test := range []struct {
		name      string
		response  string
		wantError bool
	}{
		{"unchanged", inspectionJSON, false},
		{"replacement dataset", strings.Replace(inspectionJSON, "18446744073709551615", "2", 1), true},
		{"dataset disappeared or renamed", `{"datasets":{}}`, true},
		{"null dataset", `{"datasets":{"tank/share":null}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			client := NewClient(Options{Runner: runner})
			runner.AddCommand("zfs get -p -H -o name,value guid -j", inspectionJSON, "", nil)
			runner.AddCommand("zfs list ", test.response, "", nil)
			dataset, err := client.ZFS.GetByGUID(context.Background(), "18446744073709551615", true)
			if test.wantError {
				if err == nil || dataset != nil {
					t.Fatalf("changed dataset returned: %#v, %v", dataset, err)
				}
			} else if err != nil || dataset == nil || dataset.GUID != "18446744073709551615" {
				t.Fatalf("unchanged dataset rejected: %#v, %v", dataset, err)
			}
			if len(runner.CallHistory) != 2 || !strings.Contains(runner.CallHistory[1].Cmd, " -r tank/share -j") {
				t.Fatalf("unexpected GUID lookup commands: %#v", runner.CallHistory)
			}
		})
	}
}

func TestZFS_GetByGUIDMissingAmbiguousAndMalformedResults(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries map[string]*Dataset
		wantErr bool
	}{
		{"no datasets", map[string]*Dataset{}, false},
		{"different GUID", map[string]*Dataset{"tank/other": {Properties: map[string]ZFSProperty{"guid": {Value: "2"}}}}, false},
		{"ambiguous", map[string]*Dataset{
			"tank/one": {Properties: map[string]ZFSProperty{"guid": {Value: "1"}}},
			"tank/two": {Properties: map[string]ZFSProperty{"guid": {Value: "1"}}},
		}, true},
		{"null entry", map[string]*Dataset{"tank/share": nil}, true},
		{"missing GUID", map[string]*Dataset{"tank/share": {}}, true},
		{"unknown GUID", map[string]*Dataset{"tank/share": {Properties: map[string]ZFSProperty{"guid": {Value: "-"}}}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			client := NewClient(Options{Runner: runner})
			response, err := json.Marshal(DatasetList{Datasets: test.entries})
			if err != nil {
				t.Fatal(err)
			}
			runner.AddCommand("zfs get -p -H -o name,value guid -j", string(response), "", nil)
			dataset, err := client.ZFS.GetByGUID(context.Background(), "1", false)
			if (err != nil) != test.wantErr || dataset != nil || len(runner.CallHistory) != 1 {
				t.Fatalf("invalid lookup result: %#v, %v, calls=%#v", dataset, err, runner.CallHistory)
			}
		})
	}
	runner := testutil.NewMockRunner()
	client := NewClient(Options{Runner: runner})
	if _, err := client.ZFS.GetByGUID(context.Background(), " ", false); err == nil || len(runner.CallHistory) != 0 {
		t.Fatal("empty GUID ran a command or was accepted")
	}
	failure := errors.New("discovery unavailable")
	runner.AddCommand("zfs get ", "", "", failure)
	if _, err := client.ZFS.GetByGUID(context.Background(), "1", false); !errors.Is(err, failure) {
		t.Fatalf("discovery failure was not preserved: %v", err)
	}
}

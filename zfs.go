package gzfs

import (
	"context"
	"fmt"
	"maps"
	"os"
	"strconv"
	"strings"
)

type zfs struct {
	cmd Cmd
}

type DatasetType string

const (
	DatasetTypeFilesystem DatasetType = "FILESYSTEM"
	DatasetTypeVolume     DatasetType = "VOLUME"
	DatasetTypeSnapshot   DatasetType = "SNAPSHOT"
)

type Dataset struct {
	z *zfs `json:"-"`

	Name      string      `json:"name"`
	GUID      string      `json:"guid"`
	Type      DatasetType `json:"type"`
	Pool      string      `json:"pool"`
	CreateTXG string      `json:"createtxg"`

	Mountpoint    string  `json:"mountpoint"`
	Used          uint64  `json:"used"`
	Available     uint64  `json:"available"`
	Referenced    uint64  `json:"referenced"`
	Compressratio float64 `json:"compressratio"`

	Properties map[string]ZFSProperty `json:"properties"`
}

type DatasetList struct {
	OutputVersion OutputVersion       `json:"output_version"`
	Datasets      map[string]*Dataset `json:"datasets"`
}

func toZfsType(t DatasetType) string {
	switch t {
	case DatasetTypeFilesystem:
		return "filesystem"
	case DatasetTypeVolume:
		return "volume"
	case DatasetTypeSnapshot:
		return "snapshot"
	default:
		return string(t)
	}
}

func (z *zfs) listArgs(name string, recursive bool, t *DatasetType) []string {
	args := append([]string{"list", "-o", "all"}, zfsArgs...)

	if recursive {
		args = append(args, "-r")
	}

	if t != nil {
		args = append(args, "-t", toZfsType(*t))
	}

	if name != "" {
		args = append(args, name)
	}

	return args
}

func (z *zfs) List(ctx context.Context, recursive bool, name ...string) ([]*Dataset, error) {
	var resp DatasetList

	var target string
	if len(name) > 0 {
		target = name[0]
	}

	args := z.listArgs(target, recursive, nil)

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	datasets := make([]*Dataset, 0, len(resp.Datasets))
	for _, d := range resp.Datasets {
		d.z = z

		d.GUID = ParseString(d.Properties["guid"].Value)
		d.Mountpoint = ParseString(d.Properties["mountpoint"].Value)
		d.Used = ParseSize(d.Properties["used"].Value)
		d.Available = ParseSize(d.Properties["available"].Value)
		d.Referenced = ParseSize(d.Properties["referenced"].Value)
		d.Compressratio = ParseRatio(d.Properties["compressratio"].Value)

		datasets = append(datasets, d)
	}

	return datasets, nil
}

func (z *zfs) Get(ctx context.Context, name string, recursive bool) (*Dataset, error) {
	var resp DatasetList

	args := z.listArgs(name, recursive, nil)

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	dataset, ok := resp.Datasets[name]
	if !ok {
		return nil, nil
	}

	dataset.z = z
	dataset.GUID = ParseString(dataset.Properties["guid"].Value)
	dataset.Mountpoint = ParseString(dataset.Properties["mountpoint"].Value)
	dataset.Used = ParseSize(dataset.Properties["used"].Value)
	dataset.Available = ParseSize(dataset.Properties["available"].Value)
	dataset.Referenced = ParseSize(dataset.Properties["referenced"].Value)
	dataset.Compressratio = ParseRatio(dataset.Properties["compressratio"].Value)

	return dataset, nil
}

func (z *zfs) GetProperty(ctx context.Context, datasetName, propName string) (ZFSProperty, error) {
	var resp DatasetList

	args := append([]string{"get"}, zfsArgs...)
	args = append(args, propName, datasetName)

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return ZFSProperty{}, err
	}

	ds, ok := resp.Datasets[datasetName]
	if !ok || ds == nil {
		return ZFSProperty{}, fmt.Errorf("dataset %q not found in zfs get output", datasetName)
	}

	if ds.Properties == nil {
		return ZFSProperty{}, fmt.Errorf("no properties returned for dataset %q", datasetName)
	}

	prop, ok := ds.Properties[propName]
	if !ok {
		return ZFSProperty{}, fmt.Errorf("property %q not found on dataset %q", propName, datasetName)
	}

	return prop, nil
}

func (z *zfs) ListByType(ctx context.Context, t DatasetType, recursive bool, name ...string) ([]*Dataset, error) {
	var resp DatasetList

	var target string
	if len(name) > 0 {
		target = name[0]
	}

	args := z.listArgs(target, recursive, &t)

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	datasets := make([]*Dataset, 0, len(resp.Datasets))
	for _, d := range resp.Datasets {
		d.z = z
		datasets = append(datasets, d)
	}

	return datasets, nil
}

func (z *zfs) CreateVolume(ctx context.Context, name string, size uint64, properties map[string]string) (*Dataset, error) {
	props := make(map[string]string, len(properties))
	maps.Copy(props, properties)

	args := []string{"create", "-p", "-V", strconv.FormatUint(size, 10)}

	if key, ok := props["encryptionKey"]; ok {
		if key != "" && props["encryption"] != "off" {
			if len([]byte(key)) < 32 || len([]byte(key)) > 512 {
				return nil, fmt.Errorf("invalid_encryption_key_length")
			}

			seed := fmt.Sprintf("%s-%s", name, key)
			randomFile := fmt.Sprintf("/etc/zfs/keys/%s", GenerateDeterministicUUID(seed))

			if _, err := os.Stat(randomFile); err == nil {
				return nil, fmt.Errorf("dont_reuse_encryption_keys")
			}

			if err := os.WriteFile(randomFile, []byte(key), 0600); err != nil {
				return nil, fmt.Errorf("failed_to_write_encryption_key")
			}

			props["keylocation"] = fmt.Sprintf("file://%s", randomFile)
			props["keyformat"] = "passphrase"
		}
	}

	delete(props, "encryptionKey")
	delete(props, "parent")
	delete(props, "size")

	for k, v := range props {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, name)

	if _, _, err := z.cmd.RunBytes(ctx, nil, args...); err != nil {
		return nil, err
	}

	return z.Get(ctx, name, false)
}

func (z *zfs) EditVolume(ctx context.Context, name string, props map[string]string) error {
	dataset, err := z.Get(context.Background(), name, false)
	if err != nil {
		return fmt.Errorf("error_getting_dataset: %w", err)
	}

	if len(props) == 0 {
		return fmt.Errorf("no_properties_to_edit")
	}

	delete(props, "encryptionKey")

	if _, ok := props["quota"]; ok {
		if props["quota"] == "" {
			delete(props, "quota")
		}
	}

	var keyValPairs []string
	for k, v := range props {
		keyValPairs = append(keyValPairs, k, v)
	}

	if err := dataset.SetProperties(ctx, keyValPairs...); err != nil {
		return err
	}

	return nil
}

func (z *zfs) CreateFilesystem(ctx context.Context, name string, properties map[string]string) (*Dataset, error) {
	// work on a copy so caller's map isn't mutated
	props := make(map[string]string, len(properties))
	maps.Copy(props, properties)

	args := []string{"create"}

	if key, ok := props["encryptionKey"]; ok {
		if key != "" && props["encryption"] != "off" {
			if len([]byte(key)) < 32 || len([]byte(key)) > 512 {
				return nil, fmt.Errorf("invalid_encryption_key_length")
			}

			seed := fmt.Sprintf("%s-%s", name, key)
			randomFile := fmt.Sprintf("/etc/zfs/keys/%s", GenerateDeterministicUUID(seed))

			if _, err := os.Stat(randomFile); err == nil {
				return nil, fmt.Errorf("dont_reuse_encryption_keys")
			}

			if err := os.WriteFile(randomFile, []byte(key), 0600); err != nil {
				return nil, fmt.Errorf("failed_to_write_encryption_key")
			}

			props["keylocation"] = fmt.Sprintf("file://%s", randomFile)
			props["keyformat"] = "passphrase"
		}
	}

	delete(props, "encryptionKey")

	if q, ok := props["quota"]; ok && q == "" {
		delete(props, "quota")
	}

	for k, v := range props {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, name)

	if _, _, err := z.cmd.RunBytes(ctx, nil, args...); err != nil {
		return nil, err
	}

	return z.Get(ctx, name, false)
}

func (z *zfs) EditFilesystem(ctx context.Context, name string, props map[string]string) error {
	ds, err := z.Get(ctx, name, false)
	if err != nil {
		return fmt.Errorf("error_getting_dataset: %w", err)
	}
	if ds == nil {
		return fmt.Errorf("dataset_not_found")
	}
	if ds.Type != DatasetTypeFilesystem {
		return fmt.Errorf("not_a_filesystem")
	}

	if len(props) == 0 {
		return fmt.Errorf("no_properties_to_edit")
	}

	clean := make(map[string]string, len(props))
	maps.Copy(clean, props)

	delete(clean, "encryptionKey")

	if q, ok := clean["quota"]; ok && (q == "" || q == "0B") {
		delete(clean, "quota")
	}

	if len(clean) == 0 {
		return fmt.Errorf("no_properties_to_edit")
	}

	args := []string{"set"}
	for k, v := range clean {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, name)

	if _, _, err := z.cmd.RunBytes(ctx, nil, args...); err != nil {
		return fmt.Errorf("error_setting_properties: %w", err)
	}

	return nil
}

func (z *zfs) Snapshot(ctx context.Context, dataset, snapName string, recursive bool) (*Dataset, error) {
	if dataset == "" {
		return nil, fmt.Errorf("dataset name is empty")
	}
	if snapName == "" {
		return nil, fmt.Errorf("snapshot name is empty")
	}

	fullName := fmt.Sprintf("%s@%s", dataset, snapName)

	args := []string{"snapshot"}
	if recursive {
		args = append(args, "-r")
	}

	args = append(args, fullName)

	if _, _, err := z.cmd.RunBytes(ctx, nil, args...); err != nil {
		return nil, fmt.Errorf("snapshot_failed: %w", err)
	}

	return z.Get(ctx, fullName, false)
}

func (z *zfs) Rollback(ctx context.Context, name string, destroyMoreRecent bool) error {
	if name == "" {
		return fmt.Errorf("snapshot name is empty")
	}

	args := []string{"rollback"}
	if destroyMoreRecent {
		args = append(args, "-r")
	}
	args = append(args, name)

	if _, _, err := z.cmd.RunBytes(ctx, nil, args...); err != nil {
		return fmt.Errorf("rollback_failed: %w", err)
	}

	return nil
}

func (z *zfs) Clone(ctx context.Context, srcSnapshot, dest string, properties map[string]string) (*Dataset, error) {
	if z == nil {
		return nil, fmt.Errorf("zfs client is nil")
	}

	if srcSnapshot == "" {
		return nil, fmt.Errorf("source snapshot name is empty")
	}

	if dest == "" {
		return nil, fmt.Errorf("destination name is empty")
	}

	srcDs, err := z.Get(ctx, srcSnapshot, false)
	if err != nil {
		return nil, fmt.Errorf("error_getting_source_dataset: %w", err)
	}
	if srcDs == nil {
		return nil, fmt.Errorf("source_snapshot_not_found: %s", srcSnapshot)
	}
	if srcDs.Type != DatasetTypeSnapshot {
		return nil, fmt.Errorf("can_only_clone_from_snapshots")
	}

	args := []string{"clone", "-p"}

	for k, v := range properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, srcSnapshot, dest)

	if _, _, err := z.cmd.RunBytes(ctx, nil, args...); err != nil {
		return nil, fmt.Errorf("clone_failed: %w", err)
	}

	ds, err := z.Get(ctx, dest, false)
	if err != nil {
		return nil, err
	}

	if ds == nil {
		return nil, fmt.Errorf("clone_succeeded_but_dataset_not_found: %s", dest)
	}

	return ds, nil
}

func (d *Dataset) SetProperties(ctx context.Context, kvPairs ...string) error {
	if d == nil {
		return fmt.Errorf("dataset is nil")
	}

	if d.z == nil {
		return fmt.Errorf("no zfs client attached")
	}

	if len(kvPairs)%2 != 0 {
		return fmt.Errorf("invalid_key_value_pairs")
	}

	args := []string{"set"}

	for i := 0; i < len(kvPairs); i += 2 {
		args = append(args, fmt.Sprintf("%s=%s", kvPairs[i], kvPairs[i+1]))
	}

	args = append(args, d.Name)

	_, _, err := d.z.cmd.RunBytes(ctx, nil, args...)
	if err != nil {
		return fmt.Errorf("set_properties_failed: %w", err)
	}

	return nil
}

func (d *Dataset) Destroy(ctx context.Context, recursive bool, deferDeletion bool) error {
	if d == nil {
		return fmt.Errorf("dataset is nil")
	}

	if d.Properties == nil {
		return fmt.Errorf("dataset has no properties")
	}

	if d.Pool == "" {
		return fmt.Errorf("dataset has no pool information")
	}

	if d.z == nil {
		return fmt.Errorf("no zfs client attached")
	}

	args := []string{"destroy"}

	if recursive {
		args = append(args, "-r")
	}

	if deferDeletion {
		args = append(args, "-d")
	}

	args = append(args, d.Name)

	_, _, err := d.z.cmd.RunBytes(ctx, nil, args...)
	if err != nil {
		return fmt.Errorf("dataset_destroy_failed: %w", err)
	}

	return nil
}

func (d *Dataset) GetProperty(ctx context.Context, name string) (ZFSProperty, error) {
	if d == nil {
		return ZFSProperty{}, fmt.Errorf("dataset is nil")
	}

	if d.z == nil {
		return ZFSProperty{}, fmt.Errorf("no zfs client attached")
	}

	if d.Properties == nil {
		return ZFSProperty{}, fmt.Errorf("dataset has no properties")
	}

	prop, ok := d.Properties[name]
	if !ok {
		return ZFSProperty{}, fmt.Errorf("property %q not found", name)
	}

	return prop, nil
}

func (d *Dataset) Snapshots() ([]*Dataset, error) {
	if d == nil {
		return nil, fmt.Errorf("dataset is nil")
	}

	if d.z == nil {
		return nil, fmt.Errorf("no zfs client attached")
	}

	snapshots, err := d.z.ListByType(context.Background(), DatasetTypeSnapshot, false, d.Name)
	if err != nil {
		return nil, fmt.Errorf("error_listing_snapshots: %w", err)
	}

	return snapshots, nil
}

func (d *Dataset) Snapshot(ctx context.Context, name string, recursive bool) (*Dataset, error) {
	if d == nil {
		return nil, fmt.Errorf("dataset is nil")
	}

	if d.z == nil {
		return nil, fmt.Errorf("no zfs client attached")
	}

	return d.z.Snapshot(ctx, d.Name, name, recursive)
}

func (d *Dataset) Rollback(ctx context.Context, destroyMoreRecent bool) error {
	if d == nil {
		return fmt.Errorf("dataset is nil")
	}
	if d.z == nil {
		return fmt.Errorf("no zfs client attached")
	}
	if d.Type != DatasetTypeSnapshot {
		return fmt.Errorf("can only rollback snapshots")
	}

	return d.z.Rollback(ctx, d.Name, destroyMoreRecent)
}

func (d *Dataset) Unmount(ctx context.Context, force bool) error {
	if d == nil {
		return fmt.Errorf("dataset is nil")
	}
	if d.z == nil {
		return fmt.Errorf("no zfs client attached")
	}
	if d.Type == DatasetTypeSnapshot {
		return fmt.Errorf("cannot unmount snapshots")
	}

	args := []string{"unmount"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, d.Name)

	_, _, err := d.z.cmd.RunBytes(ctx, nil, args...)
	if err != nil {
		return fmt.Errorf("unmount_failed: %w", err)
	}

	return nil
}

func (d *Dataset) Mount(ctx context.Context, overlay bool, options ...string) error {
	if d == nil {
		return fmt.Errorf("dataset is nil")
	}
	if d.z == nil {
		return fmt.Errorf("no zfs client attached")
	}
	if d.Type == DatasetTypeSnapshot {
		return fmt.Errorf("cannot mount snapshots")
	}

	args := []string{"mount"}
	if overlay {
		args = append(args, "-O")
	}

	if len(options) > 0 {
		args = append(args, "-o", strings.Join(options, ","))
	}

	args = append(args, d.Name)

	_, _, err := d.z.cmd.RunBytes(ctx, nil, args...)
	if err != nil {
		return fmt.Errorf("mount_failed: %w", err)
	}

	return nil
}

func (d *Dataset) Clone(ctx context.Context, dest string, properties map[string]string) (*Dataset, error) {
	if d == nil {
		return nil, fmt.Errorf("dataset is nil")
	}

	if d.z == nil {
		return nil, fmt.Errorf("no zfs client attached")
	}

	return d.z.Clone(ctx, d.Name, dest, properties)
}

func (d *Dataset) Rename(ctx context.Context, newName string) (*Dataset, error) {
	if d == nil {
		return nil, fmt.Errorf("dataset is nil")
	}

	if d.z == nil {
		return nil, fmt.Errorf("no zfs client attached")
	}

	if newName == "" {
		return nil, fmt.Errorf("new name cannot be empty")
	}

	args := []string{"rename", d.Name, newName}

	_, _, err := d.z.cmd.RunBytes(ctx, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("rename_failed: %w", err)
	}

	renamed, err := d.z.Get(ctx, newName, false)
	if err != nil {
		return nil, fmt.Errorf("error_getting_renamed_dataset: %w", err)
	}

	if renamed == nil {
		return nil, fmt.Errorf("rename_succeeded_but_dataset_not_found: %s", newName)
	}

	return renamed, nil
}

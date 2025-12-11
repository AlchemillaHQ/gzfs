package gzfs

import (
	"context"
	"encoding/json"
	"fmt"
)

type zpool struct {
	cmd Cmd
	zdb *zdb
	zfs *zfs
}

type ZPoolState string

const (
	ZPoolStateOnline      ZPoolState = "ONLINE"
	ZPoolStateDegraded    ZPoolState = "DEGRADED"
	ZPoolStateFaulted     ZPoolState = "FAULTED"
	ZPoolStateOffline     ZPoolState = "OFFLINE"
	ZPoolStateRemoved     ZPoolState = "REMOVED"
	ZPoolStateUnavailible ZPoolState = "UNAVAIL"
	ZPoolStateCorruptData ZPoolState = "CORRUPT_DATA"
	ZPoolStateUnknown     ZPoolState = "UNKNOWN"
)

type ZPoolStatusVDEV struct {
	Name        string `json:"name"`
	VdevType    string `json:"vdev_type"`
	GUID        string `json:"guid"`
	Path        string `json:"path"`
	Class       string `json:"class"`
	State       string `json:"state"`
	AllocSpace  string `json:"alloc_space"`
	TotalSpace  string `json:"total_space"`
	DefSpace    string `json:"def_space"`
	RepDevSize  string `json:"rep_dev_size"`
	ReadErrors  string `json:"read_errors"`
	WriteErrors string `json:"write_errors"`
	ChkErrors   string `json:"checksum_errors"`

	Vdevs map[string]*ZPoolStatusVDEV `json:"vdevs"`
}

type ZPoolVDEV struct {
	Name     string `json:"name"`
	VdevType string `json:"vdev_type"`
	GUID     string `json:"guid"`
	Path     string `json:"path"`
	PhysPath string `json:"phys_path"`
	Class    string `json:"class"`
	State    string `json:"state"`

	Size          uint64  `json:"size"`
	Free          uint64  `json:"free"`
	Alloc         uint64  `json:"allocated"`
	Fragmentation float64 `json:"fragmentation"`

	Properties map[string]ZFSProperty `json:"properties"`
	Vdevs      map[string]*ZPoolVDEV  `json:"vdevs"`
}

type ZPool struct {
	z *zpool `json:"-"`

	Name       string     `json:"name"`
	Type       string     `json:"type"`
	State      ZPoolState `json:"state"`
	PoolGUID   string     `json:"pool_guid"`
	TXG        string     `json:"txg"`
	SPAVersion string     `json:"spa_version"`
	ZPLVersion string     `json:"zpl_version"`

	Size          uint64  `json:"size"`
	Free          uint64  `json:"free"`
	Alloc         uint64  `json:"allocated"`
	Fragmentation float64 `json:"fragmentation"`
	DedupRatio    float64 `json:"dedup_ratio"`

	Properties map[string]ZFSProperty `json:"properties"`

	Vdevs   map[string]*ZPoolVDEV `json:"-"`
	Logs    map[string]*ZPoolVDEV `json:"logs"`
	L2Cache map[string]*ZPoolVDEV `json:"l2cache"`
	Spares  map[string]*ZPoolVDEV `json:"spares"`
}

func (p *ZPool) MarshalJSON() ([]byte, error) {
	type Alias ZPool

	out := struct {
		*Alias
		Vdevs map[string]*ZPoolVDEV `json:"vdevs"`
	}{
		Alias: (*Alias)(p),
		Vdevs: p.Vdevs,
	}

	return json.Marshal(&out)
}

func (p *ZPool) UnmarshalJSON(data []byte) error {
	type Alias ZPool

	aux := struct {
		*Alias
		RawVdevs map[string]json.RawMessage `json:"vdevs"`
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	p.Vdevs = make(map[string]*ZPoolVDEV)
	p.Logs = make(map[string]*ZPoolVDEV)
	p.L2Cache = make(map[string]*ZPoolVDEV)
	p.Spares = make(map[string]*ZPoolVDEV)

	populateParsedFields := func(v *ZPoolVDEV) {
		if v == nil || v.Properties == nil {
			return
		}
		if s, ok := v.Properties["size"]; ok {
			v.Size = ParseSize(s.Value)
		}
		if f, ok := v.Properties["free"]; ok {
			v.Free = ParseSize(f.Value)
		}
		if a, ok := v.Properties["allocated"]; ok {
			v.Alloc = ParseSize(a.Value)
		}
		if frag, ok := v.Properties["fragmentation"]; ok {
			v.Fragmentation = ParsePercentage(frag.Value)
		}
	}

	decodeGroup := func(raw json.RawMessage, dest map[string]*ZPoolVDEV) error {
		var m map[string]*ZPoolVDEV
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		for name, v := range m {
			if v == nil {
				continue
			}
			if v.Name == "" {
				v.Name = name
			}
			populateParsedFields(v)
			dest[name] = v
		}
		return nil
	}

	for k, raw := range aux.RawVdevs {
		switch k {
		case "logs":
			if err := decodeGroup(raw, p.Logs); err != nil {
				return fmt.Errorf("decode %s vdevs: %w", k, err)
			}
		case "l2cache":
			if err := decodeGroup(raw, p.L2Cache); err != nil {
				return fmt.Errorf("decode %s vdevs: %w", k, err)
			}
		case "spares":
			if err := decodeGroup(raw, p.Spares); err != nil {
				return fmt.Errorf("decode %s vdevs: %w", k, err)
			}
		default:
			var v ZPoolVDEV
			if err := json.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("decode vdev %q: %w", k, err)
			}
			if v.Name == "" {
				v.Name = k
			}
			populateParsedFields(&v)
			p.Vdevs[k] = &v
		}
	}

	return nil
}

type ZPoolList struct {
	OutputVersion OutputVersion     `json:"output_version"`
	Pools         map[string]*ZPool `json:"pools"`
}

type ZPoolStatusScanStats struct {
	Function           string `json:"function"`
	State              string `json:"state"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	ToExamine          string `json:"to_examine"`
	Examined           string `json:"examined"`
	Skipped            string `json:"skipped"`
	Processed          string `json:"processed"`
	Errors             string `json:"errors"`
	BytesPerScan       string `json:"bytes_per_scan"`
	PassStart          string `json:"pass_start"`
	ScrubPause         string `json:"scrub_pause"`
	ScrubSpentPaused   string `json:"scrub_spent_paused"`
	IssuedBytesPerScan string `json:"issued_bytes_per_scan"`
	Issued             string `json:"issued"`
}

type ZPoolStatusPool struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	PoolGUID   string `json:"pool_guid"`
	TXG        string `json:"txg"`
	SPAVersion string `json:"spa_version"`
	ZPLVersion string `json:"zpl_version"`
	Status     string `json:"status"`
	Action     string `json:"action"`

	ScanStats *ZPoolStatusScanStats       `json:"scan_stats"`
	Vdevs     map[string]*ZPoolStatusVDEV `json:"vdevs"`
	Logs      map[string]*ZPoolStatusVDEV `json:"logs"`
	Spares    map[string]*ZPoolStatusVDEV `json:"spares"`
	L2Cache   map[string]*ZPoolStatusVDEV `json:"l2cache"`
}

type ZPoolStatus struct {
	OutputVersion OutputVersion               `json:"output_version"`
	Pools         map[string]*ZPoolStatusPool `json:"pools"`
}

func normalizeVdev(v *ZPoolVDEV) {
	if v == nil || v.Properties == nil {
		return
	}

	if prop, ok := v.Properties["size"]; ok {
		v.Size = ParseSize(prop.Value)
	}
	if prop, ok := v.Properties["free"]; ok {
		v.Free = ParseSize(prop.Value)
	}
	if prop, ok := v.Properties["allocated"]; ok {
		v.Alloc = ParseSize(prop.Value)
	}
	if prop, ok := v.Properties["fragmentation"]; ok {
		v.Fragmentation = ParsePercentage(prop.Value)
	}

	for _, child := range v.Vdevs {
		normalizeVdev(child)
	}
}

func normalizePool(p *ZPool, z *zpool) {
	p.z = z

	if p.Properties != nil {
		if prop, ok := p.Properties["size"]; ok {
			p.Size = ParseSize(prop.Value)
		}
		if prop, ok := p.Properties["free"]; ok {
			p.Free = ParseSize(prop.Value)
		}
		if prop, ok := p.Properties["allocated"]; ok {
			p.Alloc = ParseSize(prop.Value)
		}
		if prop, ok := p.Properties["fragmentation"]; ok {
			p.Fragmentation = ParsePercentage(prop.Value)
		}
		if prop, ok := p.Properties["dedupratio"]; ok {
			p.DedupRatio = ParseRatio(prop.Value)
		}
	}

	for _, v := range p.Vdevs {
		normalizeVdev(v)
	}
}

func (z *zpool) List(ctx context.Context) ([]*ZPool, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "all", "-v"}, zpoolArgs...)
	args = append(args, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	pools := make([]*ZPool, 0, len(resp.Pools))
	for _, p := range resp.Pools {
		normalizePool(p, z)
		pools = append(pools, p)
	}

	return pools, nil
}

func (z *zpool) Get(ctx context.Context, name string) (*ZPool, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "all", "-v"}, zpoolArgs...)
	args = append(args, name, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	pool, ok := resp.Pools[name]
	if !ok {
		return nil, nil
	}

	normalizePool(pool, z)

	return pool, nil
}

func (z *zpool) GetByGUID(ctx context.Context, guid string) (*ZPool, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "guid"}, zpoolArgs...)
	args = append(args, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	var found *ZPool

	for _, pool := range resp.Pools {
		if pool.PoolGUID == guid {
			found = pool
			break
		}
	}

	if found == nil {
		return nil, fmt.Errorf("pool with GUID %q not found", guid)
	}

	return z.Get(ctx, found.Name)
}

func (z *zpool) GetPoolNames(ctx context.Context) ([]string, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "name"}, zpoolArgs...)
	args = append(args, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(resp.Pools))
	for name := range resp.Pools {
		names = append(names, name)
	}

	return names, nil
}

func (z *zpool) GetPoolGUID(ctx context.Context, name string) (string, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "guid"}, zpoolArgs...)
	args = append(args, name, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return "", err
	}

	pool, ok := resp.Pools[name]
	if !ok {
		return "", fmt.Errorf("pool %q not found", name)
	}

	return pool.PoolGUID, nil
}

func (z *zpool) GetPoolStatus(ctx context.Context, name string) (*ZPoolStatusPool, error) {
	var resp ZPoolStatus

	args := append([]string{"status"}, zpoolArgs...)
	args = append(args, name, "-P", "-v")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	pool, ok := resp.Pools[name]
	if !ok {
		return nil, fmt.Errorf("pool %q not found in status", name)
	}

	return pool, nil
}

func (z *zpool) SetProperty(ctx context.Context, name, property, value string) error {
	names, err := z.GetPoolNames(ctx)
	if err != nil {
		return err
	}

	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("pool %q not found", name)
	}

	args := []string{"set", fmt.Sprintf("%s=%s", property, value), name}
	_, _, err = z.cmd.RunBytes(ctx, nil, args...)
	return err
}

func (z *zpool) GetProperties(ctx context.Context, name string) (map[string]ZFSProperty, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "all"}, zpoolArgs...)
	args = append(args, name, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	pool, ok := resp.Pools[name]
	if !ok {
		return nil, fmt.Errorf("pool %q not found", name)
	}

	return pool.Properties, nil
}

func (z *zpool) Create(ctx context.Context, name string, force bool, properties map[string]string, args ...string) error {
	cli := make([]string, 0, 8)
	cli = append(cli, "create")

	if force {
		cli = append(cli, "-f")
	}

	for prop, val := range properties {
		cli = append(cli, "-o", fmt.Sprintf("%s=%s", prop, val))
	}

	cli = append(cli, name)
	cli = append(cli, args...)

	_, _, err := z.cmd.RunBytes(ctx, nil, cli...)
	return err
}

func findVdevByPath(v *ZPoolStatusVDEV, devicePath string) *ZPoolStatusVDEV {
	if v == nil {
		return nil
	}

	if v.Path == devicePath {
		return v
	}

	for _, child := range v.Vdevs {
		if found := findVdevByPath(child, devicePath); found != nil {
			return found
		}
	}

	return nil
}

func findVdevByPathInMap(m map[string]*ZPoolStatusVDEV, devicePath string) *ZPoolStatusVDEV {
	for _, v := range m {
		if found := findVdevByPath(v, devicePath); found != nil {
			return found
		}
	}
	return nil
}

func (z *zpool) IsDeviceInZpool(ctx context.Context, devicePath string) (bool, string, error) {
	if devicePath == "" {
		return false, "", fmt.Errorf("devicePath must not be empty")
	}

	pools, err := z.List(ctx)
	if err != nil {
		return false, "", fmt.Errorf("failed to list pools: %w", err)
	}

	for _, p := range pools {
		status, err := p.Status(ctx)
		if err != nil {
			return false, "", fmt.Errorf("failed to get status for pool %q: %w", p.Name, err)
		}

		if v := findVdevByPathInMap(status.Vdevs, devicePath); v != nil {
			return true, p.Name, nil
		}

		if v := findVdevByPathInMap(status.Logs, devicePath); v != nil {
			return true, p.Name, nil
		}

		if v := findVdevByPathInMap(status.Spares, devicePath); v != nil {
			return true, p.Name, nil
		}

		if v := findVdevByPathInMap(status.L2Cache, devicePath); v != nil {
			return true, p.Name, nil
		}
	}

	return false, "", nil
}

func (z *ZPool) ZDB(ctx context.Context) (*ZDBPool, error) {
	if z.z == nil {
		return nil, fmt.Errorf("no zpool client attached")
	}
	return z.z.zdb.GetPool(ctx, z.Name, z.PoolGUID)
}

func (p *ZPool) GetProperty(name string) (ZFSProperty, error) {
	if p == nil || p.Properties == nil {
		return ZFSProperty{}, fmt.Errorf("pool or properties is nil")
	}

	prop, ok := p.Properties[name]
	if !ok {
		return ZFSProperty{}, fmt.Errorf("property %q not found", name)
	}

	return prop, nil
}

func (p *ZPool) Destroy(ctx context.Context) error {
	if p.z == nil {
		return fmt.Errorf("no zpool client attached")
	}

	_, _, err := p.z.cmd.RunBytes(ctx, nil, "destroy", p.Name)
	return err
}

func (p *ZPool) Scrub(ctx context.Context) error {
	if p.z == nil {
		return fmt.Errorf("no zpool client attached")
	}

	_, _, err := p.z.cmd.RunBytes(ctx, nil, "scrub", p.Name)
	if err != nil {
		return fmt.Errorf("pool_scrub_failed: %w", err)
	}
	return nil
}

func (p *ZPool) Status(ctx context.Context) (*ZPoolStatusPool, error) {
	if p.z == nil {
		return nil, fmt.Errorf("no zpool client attached")
	}

	return p.z.GetPoolStatus(ctx, p.Name)
}

func (p *ZPool) AddSpare(ctx context.Context, device string, force bool) error {
	if device == "" {
		return fmt.Errorf("device must not be empty")
	}

	if p.z == nil {
		return fmt.Errorf("no zpool client attached")
	}

	args := []string{"add"}
	if force {
		args = append(args, "-f")
	}

	args = append(args, p.Name, "spare", device)
	_, _, err := p.z.cmd.RunBytes(ctx, nil, args...)

	return err
}

func (p *ZPool) RemoveSpare(ctx context.Context, device string) error {
	if device == "" {
		return fmt.Errorf("device must not be empty")
	}

	if p.z == nil {
		return fmt.Errorf("no zpool client attached")
	}

	_, _, err := p.z.cmd.RunBytes(ctx, nil, "remove", p.Name, device)
	if err != nil {
		return fmt.Errorf("pool_remove_spare_failed: %w", err)
	}
	return nil
}

func (p *ZPool) SetProperty(ctx context.Context, property, value string) error {
	if p.z == nil {
		return fmt.Errorf("no zpool client attached")
	}

	return p.z.SetProperty(ctx, p.Name, property, value)
}

func (p *ZPool) GetProperties(ctx context.Context) (map[string]ZFSProperty, error) {
	if p.z == nil {
		return nil, fmt.Errorf("no zpool client attached")
	}

	return p.z.GetProperties(ctx, p.Name)
}

func (p *ZPool) Datasets(ctx context.Context, t DatasetType) ([]*Dataset, error) {
	if p.z == nil || p.z.zfs == nil {
		return nil, fmt.Errorf("no zpool or zfs client attached")
	}

	return p.z.zfs.ListWithPrefix(ctx, t, p.Name, true)
}

func (p *ZPool) ReplaceDevice(ctx context.Context, oldDevice, newDevice string, force bool) error {
	if oldDevice == "" || newDevice == "" {
		return fmt.Errorf("oldDevice and newDevice must not be empty")
	}

	if p.z == nil {
		return fmt.Errorf("no zpool client attached")
	}

	args := []string{"replace"}
	if force {
		args = append(args, "-f")
	}

	args = append(args, p.Name, oldDevice, newDevice)
	_, _, err := p.z.cmd.RunBytes(ctx, nil, args...)

	return err
}

func maxRepDevSize(v *ZPoolStatusVDEV) uint64 {
	if v == nil {
		return 0
	}

	max := ParseSize(v.RepDevSize)

	for _, child := range v.Vdevs {
		childMax := maxRepDevSize(child)
		if childMax > max {
			max = childMax
		}
	}

	return max
}

func (p *ZPool) RequiredSpareSize(ctx context.Context) (uint64, error) {
	if p.z == nil {
		return 0, fmt.Errorf("no zpool client attached")
	}

	status, err := p.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get pool status: %w", err)
	}

	var maxSize uint64

	for _, v := range status.Vdevs {
		size := maxRepDevSize(v)
		if size > maxSize {
			maxSize = size
		}
	}

	if maxSize == 0 {
		return 0, fmt.Errorf("unable to determine required spare size")
	}

	return maxSize, nil
}

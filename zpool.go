package gzfs

import (
	"context"
	"fmt"
)

type zpool struct {
	cmd Cmd
	zdb *zdb
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

type ZPoolVDEV struct {
	Name     string `json:"name"`
	VdevType string `json:"vdev_type"`
	GUID     string `json:"guid"`
	Path     string `json:"path"`
	Class    string `json:"class"`
	State    string `json:"state"`

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

	ScanStats *ZPoolStatusScanStats            `json:"scan_stats"`
	Vdevs     map[string]*ZPoolStatusScanStats `json:"vdevs"`
	Logs      map[string]*ZPoolStatusScanStats `json:"logs"`
	Spares    map[string]*ZPoolStatusScanStats `json:"spares"`
	Caches    map[string]*ZPoolStatusScanStats `json:"caches"`
}

type ZPoolStatus struct {
	OutputVersion OutputVersion               `json:"output_version"`
	Pools         map[string]*ZPoolStatusPool `json:"pools"`
}

func (z *zpool) List(ctx context.Context) ([]*ZPool, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "all"}, zpoolArgs...)
	args = append(args, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	pools := make([]*ZPool, 0, len(resp.Pools))
	for _, p := range resp.Pools {
		p.z = z
		p.Size = ParseSize(p.Properties["size"].Value)
		p.Free = ParseSize(p.Properties["free"].Value)
		p.Alloc = ParseSize(p.Properties["allocated"].Value)
		p.Fragmentation = ParsePercentage(p.Properties["fragmentation"].Value)
		p.DedupRatio = ParseRatio(p.Properties["dedupratio"].Value)

		pools = append(pools, p)
	}

	return pools, nil
}

func (z *zpool) Get(ctx context.Context, name string) (*ZPool, error) {
	var resp ZPoolList

	args := append([]string{"list", "-o", "all"}, zpoolArgs...)
	args = append(args, name, "-P")

	if err := z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	pool, ok := resp.Pools[name]
	if !ok {
		return nil, nil
	}

	pool.z = z
	pool.Size = ParseSize(pool.Properties["size"].Value)
	pool.Free = ParseSize(pool.Properties["free"].Value)
	pool.Alloc = ParseSize(pool.Properties["allocated"].Value)
	pool.Fragmentation = ParsePercentage(pool.Properties["fragmentation"].Value)
	pool.DedupRatio = ParseRatio(pool.Properties["dedupratio"].Value)

	return pool, nil
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
	pool, err := z.Get(ctx, name)
	if err != nil {
		return "", err
	}

	if pool == nil {
		return "", fmt.Errorf("pool %q not found", name)
	}

	return pool.PoolGUID, nil
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

func (p *ZPool) Status(ctx context.Context) (*ZPoolStatus, error) {
	if p.z == nil {
		return nil, fmt.Errorf("no zpool client attached")
	}

	var resp ZPoolStatus

	args := append([]string{"status"}, zpoolArgs...)
	args = append(args, p.Name, "-P")

	if err := p.z.cmd.RunJSON(ctx, &resp, args...); err != nil {
		return nil, err
	}

	return &resp, nil
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

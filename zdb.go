package gzfs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type zdb struct {
	cmd      Cmd
	cacheTTL time.Duration
}

type ZDBPool struct {
	Name     string         `json:"name"`
	GUID     string         `json:"guid"`
	Version  string         `json:"version"`
	Children []ZDBPoolChild `json:"children,omitempty"`
}

type ZDBPoolChild struct {
	Type          string            `json:"type,omitempty"`
	ID            int               `json:"id,omitempty"`
	GUID          uint64            `json:"guid,omitempty"`
	Path          string            `json:"path,omitempty"`
	WholeDisk     int               `json:"whole_disk,omitempty"`
	MetaslabArray int               `json:"metaslab_array,omitempty"`
	MetaslabShift int               `json:"metaslab_shift,omitempty"`
	Ashift        int               `json:"ashift,omitempty"`
	Asize         uint64            `json:"asize,omitempty"`
	IsLog         int               `json:"is_log,omitempty"`
	CreateTXG     uint64            `json:"create_txg,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
	Children      []ZDBPoolChild    `json:"children,omitempty"`
}

type zdbCacheEntry struct {
	pool   *ZDBPool
	guid   string
	expiry time.Time
}

var (
	zdbCache      = make(map[string]zdbCacheEntry)
	zdbCacheMutex sync.RWMutex
)

func (p *ZDBPool) parseLine(prop, val string) {
	switch prop {
	case "version":
		p.Version = val
	case "name":
		p.Name = val
	}
}

func (c *ZDBPoolChild) parseProp(prop, val string) error {
	if c.Properties == nil {
		c.Properties = make(map[string]string)
	}
	c.Properties[prop] = val

	switch prop {
	case "type":
		c.Type = val
	case "id":
		_, err := fmt.Sscan(val, &c.ID)
		if err != nil {
			return fmt.Errorf("failed to parse id: %w", err)
		}
	case "guid":
		_, err := fmt.Sscan(val, &c.GUID)
		if err != nil {
			return fmt.Errorf("failed to parse guid: %w", err)
		}
	case "path":
		c.Path = val
	case "whole_disk":
		_, err := fmt.Sscan(val, &c.WholeDisk)
		if err != nil {
			return fmt.Errorf("failed to parse whole_disk: %w", err)
		}
	case "metaslab_array":
		_, err := fmt.Sscan(val, &c.MetaslabArray)
		if err != nil {
			return fmt.Errorf("failed to parse metaslab_array: %w", err)
		}
	case "metaslab_shift":
		_, err := fmt.Sscan(val, &c.MetaslabShift)
		if err != nil {
			return fmt.Errorf("failed to parse metaslab_shift: %w", err)
		}
	case "ashift":
		_, err := fmt.Sscan(val, &c.Ashift)
		if err != nil {
			return fmt.Errorf("failed to parse ashift: %w", err)
		}
	case "asize":
		_, err := fmt.Sscan(val, &c.Asize)
		if err != nil {
			return fmt.Errorf("failed to parse asize: %w", err)
		}
	case "is_log":
		_, err := fmt.Sscan(val, &c.IsLog)
		if err != nil {
			return fmt.Errorf("failed to parse is_log: %w", err)
		}
	case "create_txg":
		_, err := fmt.Sscan(val, &c.CreateTXG)
		if err != nil {
			return fmt.Errorf("failed to parse create_txg: %w", err)
		}
	}
	return nil
}

func (z *zdb) zdbOutput(ctx context.Context, args ...string) ([]string, error) {
	stdout, _, err := z.cmd.RunBytes(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	text := strings.TrimRight(string(stdout), "\n")
	if text == "" {
		return nil, nil
	}

	return strings.Split(text, "\n"), nil
}

func (z *zdb) GetPool(ctx context.Context, name string, currentGUID string) (*ZDBPool, error) {
	cacheEnabled := z.cacheTTL > 0

	cacheKey := name
	if currentGUID != "" {
		cacheKey = name + "|" + currentGUID
	}

	if cacheEnabled {
		zdbCacheMutex.RLock()
		if entry, ok := zdbCache[cacheKey]; ok && time.Now().Before(entry.expiry) {
			zdbCacheMutex.RUnlock()
			return entry.pool, nil
		}
		zdbCacheMutex.RUnlock()
	}

	args := append(append([]string{}, zdbArgs...), name)
	lines, err := z.zdbOutput(ctx, args...)

	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("no output from zdb for pool %s", name)
	}

	pool := &ZDBPool{
		Name:     name,
		GUID:     currentGUID,
		Children: make([]ZDBPoolChild, 0),
	}

	type childFrame struct {
		child  *ZDBPoolChild
		indent int
	}

	var stack []childFrame

	for _, rawLine := range lines {
		if strings.TrimSpace(rawLine) == "" {
			continue
		}

		indent := 0
		for _, r := range rawLine {
			if r == '\t' {
				indent++
			} else {
				break
			}
		}

		trimmed := strings.TrimLeft(rawLine, "\t ")
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		prop := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if len(stack) == 0 && (prop == "version" || prop == "name") {
			pool.parseLine(prop, val)
			continue
		}

		if prop == "type" {
			newChild := ZDBPoolChild{
				Type:       val,
				Properties: map[string]string{"type": val},
			}

			for len(stack) > 0 && indent <= stack[len(stack)-1].indent {
				stack = stack[:len(stack)-1]
			}

			if len(stack) == 0 {
				pool.Children = append(pool.Children, newChild)
				stack = append(stack, childFrame{child: &pool.Children[len(pool.Children)-1], indent: indent})
			} else {
				parent := stack[len(stack)-1].child
				parent.Children = append(parent.Children, newChild)
				stack = append(stack, childFrame{child: &parent.Children[len(parent.Children)-1], indent: indent})
			}

			continue
		}

		if len(stack) == 0 {
			continue
		}

		current := stack[len(stack)-1].child
		if err := current.parseProp(prop, val); err != nil {
			return nil, err
		}
	}

	if cacheEnabled {
		zdbCacheMutex.Lock()
		zdbCache[cacheKey] = zdbCacheEntry{
			pool:   pool,
			guid:   currentGUID,
			expiry: time.Now().Add(z.cacheTTL),
		}
		zdbCacheMutex.Unlock()
	}

	return pool, nil
}

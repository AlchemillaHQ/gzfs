package gzfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type Runner interface {
	Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error
}

type LocalRunner struct{}

func (LocalRunner) Run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}

	return cmd.Run()
}

type CmdError struct {
	Cmd      string
	Args     []string
	ExitErr  error
	Stderr   string
	Combined string
}

func (e *CmdError) Error() string {
	if e.ExitErr != nil {
		return fmt.Sprintf("%s failed: %v (stderr: %s)", e.Combined, e.ExitErr, strings.TrimSpace(e.Stderr))
	}
	return fmt.Sprintf("%s failed (stderr: %s)", e.Combined, strings.TrimSpace(e.Stderr))
}

func (e *CmdError) Unwrap() error { return e.ExitErr }

type Cmd struct {
	Bin    string
	Sudo   bool
	Runner Runner
}

func (c Cmd) withDefaults() Cmd {
	if c.Runner == nil {
		c.Runner = LocalRunner{}
	}
	return c
}

func (c Cmd) RunBytes(ctx context.Context, stdin io.Reader, args ...string) ([]byte, []byte, error) {
	c = c.withDefaults()

	var stdout, stderr bytes.Buffer
	name := c.Bin

	if c.Sudo {
		args = append([]string{c.Bin}, args...)
		name = "sudo"
	}

	combined := name + " " + strings.Join(args, " ")

	if err := c.Runner.Run(ctx, stdin, &stdout, &stderr, name, args...); err != nil {
		return nil, nil, &CmdError{
			Cmd:      name,
			Args:     args,
			ExitErr:  err,
			Stderr:   stderr.String(),
			Combined: combined,
		}
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

func (c Cmd) RunJSON(ctx context.Context, v any, args ...string) error {
	args = append(args, "-j")
	out, _, err := c.RunBytes(ctx, nil, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %w", c.Bin, err)
	}
	return nil
}

func (c Cmd) RunStream(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	c = c.withDefaults()

	name := c.Bin
	if c.Sudo {
		args = append([]string{c.Bin}, args...)
		name = "sudo"
	}

	combined := name + " " + strings.Join(args, " ")

	if err := c.Runner.Run(ctx, stdin, stdout, stderr, name, args...); err != nil {
		// Best-effort stderr capture if caller gave a buffer, otherwise just wrap
		var stderrStr string
		if buf, ok := stderr.(*bytes.Buffer); ok {
			stderrStr = buf.String()
		}
		return &CmdError{
			Cmd:      name,
			Args:     args,
			ExitErr:  err,
			Stderr:   stderrStr,
			Combined: combined,
		}
	}

	return nil
}

type Client struct {
	ZFS   *zfs
	Zpool *zpool
	ZDB   *zdb
}

type Options struct {
	Sudo     bool
	Runner   Runner
	ZFSBin   string
	ZpoolBin string
	ZDBBin   string

	ZDBCacheTTLSeconds int32
}

func NewClient(opts Options) *Client {
	if opts.ZFSBin == "" {
		opts.ZFSBin = "zfs"
	}
	if opts.ZpoolBin == "" {
		opts.ZpoolBin = "zpool"
	}
	if opts.ZDBBin == "" {
		opts.ZDBBin = "zdb"
	}

	zfsCmd := Cmd{Bin: opts.ZFSBin, Sudo: opts.Sudo, Runner: opts.Runner}
	zpoolCmd := Cmd{Bin: opts.ZpoolBin, Sudo: opts.Sudo, Runner: opts.Runner}
	zdbCmd := Cmd{Bin: opts.ZDBBin, Sudo: opts.Sudo, Runner: opts.Runner}

	zdbCacheTTL := time.Duration(opts.ZDBCacheTTLSeconds) * time.Second
	if opts.ZDBCacheTTLSeconds < 0 {
		zdbCacheTTL = 5 * time.Minute
	}

	zdbC := &zdb{cmd: zdbCmd, cacheTTL: zdbCacheTTL}
	zpoolC := &zpool{cmd: zpoolCmd, zdb: zdbC}

	return &Client{
		ZFS:   &zfs{cmd: zfsCmd},
		Zpool: zpoolC,
		ZDB:   zdbC,
	}
}

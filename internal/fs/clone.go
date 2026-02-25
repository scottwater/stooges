package fs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	apperrors "github.com/scottwater/stooges/internal/errors"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

type Cloner interface {
	CheckCapability(ctx context.Context) error
	CloneRepo(ctx context.Context, src, dst string) error
}

type SystemCloner struct {
	GOOS   string
	Runner CommandRunner
}

func NewSystemCloner() *SystemCloner {
	return &SystemCloner{GOOS: runtime.GOOS, Runner: ExecRunner{}}
}

func (c *SystemCloner) platform() string {
	if c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *SystemCloner) runner() CommandRunner {
	if c.Runner != nil {
		return c.Runner
	}
	return ExecRunner{}
}

func (c *SystemCloner) CheckCapability(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "stooges-cow-probe-")
	if err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "create probe temp dir", err)
	}
	defer os.RemoveAll(dir)

	srcFile := filepath.Join(dir, "src.txt")
	dstFile := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(srcFile, []byte("probe"), 0o644); err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "write probe file", err)
	}

	var args []string
	switch c.platform() {
	case "darwin":
		args = []string{"-c", srcFile, dstFile}
	case "linux":
		args = []string{"--reflink=always", srcFile, dstFile}
	default:
		return apperrors.New(apperrors.KindUnsupportedPlatform, "platform unsupported: only macOS and Linux are supported")
	}

	if err := c.runner().Run(ctx, "cp", args...); err != nil {
		return apperrors.Wrap(apperrors.KindPreflightFailure, "copy-on-write clone capability unavailable", err)
	}
	return nil
}

func (c *SystemCloner) CloneRepo(ctx context.Context, src, dst string) error {
	var args []string
	switch c.platform() {
	case "darwin":
		args = []string{"-cR", src, dst}
	case "linux":
		args = []string{"--reflink=always", "-R", src, dst}
	default:
		return apperrors.New(apperrors.KindUnsupportedPlatform, "platform unsupported: only macOS and Linux are supported")
	}

	if err := c.runner().Run(ctx, "cp", args...); err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "clone repository", err)
	}
	return nil
}

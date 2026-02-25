package engine

import (
	"context"
	"os"
	"os/exec"
	"runtime"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/fs"
	"github.com/scottwater/stooges/internal/model"
)

type PreflightOptions struct {
	WorkspacePath    string
	RequireSourceGit bool
	SourceRepoPath   string
}

type PreflightChecker struct {
	Cloner fs.Cloner
}

func NewPreflightChecker(cloner fs.Cloner) *PreflightChecker {
	return &PreflightChecker{Cloner: cloner}
}

func (p *PreflightChecker) EnsureMutating(ctx context.Context, opts PreflightOptions) (model.PreflightReport, error) {
	report := p.Report(ctx, opts)
	if !report.GitAvailable || !report.COWCloneSupported || !report.WorkspaceValid || (opts.RequireSourceGit && !report.SourceRepoValid) {
		return report, apperrors.New(apperrors.KindPreflightFailure, "preflight checks failed; run `stooges doctor` for details")
	}
	return report, nil
}

func (p *PreflightChecker) Report(ctx context.Context, opts PreflightOptions) model.PreflightReport {
	report := model.PreflightReport{
		Platform:          detectPlatform(runtime.GOOS),
		GitAvailable:      false,
		COWCloneSupported: false,
		WorkspaceValid:    false,
		SourceRepoValid:   !opts.RequireSourceGit,
	}

	if _, err := exec.LookPath("git"); err == nil {
		report.GitAvailable = true
	}

	if opts.WorkspacePath != "" {
		if stat, err := os.Stat(opts.WorkspacePath); err == nil && stat.IsDir() {
			report.WorkspaceValid = true
		}
	}

	if opts.RequireSourceGit {
		if stat, err := os.Stat(opts.SourceRepoPath); err == nil && stat.IsDir() {
			if gitStat, err := os.Stat(opts.SourceRepoPath + "/.git"); err == nil && gitStat.IsDir() {
				report.SourceRepoValid = true
			}
		}
	}

	if p.Cloner != nil {
		if err := p.Cloner.CheckCapability(ctx); err == nil {
			report.COWCloneSupported = true
		}
	}

	return report
}

func detectPlatform(goos string) model.Platform {
	switch goos {
	case "darwin":
		return model.PlatformDarwin
	case "linux":
		return model.PlatformLinux
	default:
		return model.PlatformUnknown
	}
}

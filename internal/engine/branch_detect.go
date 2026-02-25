package engine

import (
	"context"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/git"
)

type BranchFallback func(context.Context) (string, error)

type BranchDetector struct {
	Git      gitops.Ops
	Fallback BranchFallback
}

func NewBranchDetector(git gitops.Ops) *BranchDetector {
	return &BranchDetector{Git: git}
}

func (d *BranchDetector) DetectDefaultBranch(ctx context.Context, repoPath, override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}

	if d.Git != nil {
		if branch, err := d.Git.CurrentBranch(ctx, repoPath); err == nil && strings.TrimSpace(branch) != "" {
			return strings.TrimSpace(branch), nil
		}
		if branch, err := d.Git.RemoteHEADBranch(ctx, repoPath); err == nil && strings.TrimSpace(branch) != "" {
			return strings.TrimSpace(branch), nil
		}
	}

	if d.Fallback != nil {
		branch, err := d.Fallback(ctx)
		if err != nil {
			return "", err
		}
		branch = strings.TrimSpace(branch)
		if branch != "" {
			return branch, nil
		}
	}

	return "", apperrors.New(apperrors.KindInvalidInput, "could not determine default branch; pass --main-branch")
}

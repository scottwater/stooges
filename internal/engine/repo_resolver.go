package engine

import (
	"context"
	"path/filepath"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/git"
)

type RepoResolver struct {
	Git gitops.Ops
}

func NewRepoResolver(git gitops.Ops) *RepoResolver {
	return &RepoResolver{Git: git}
}

func (r *RepoResolver) Resolve(ctx context.Context, cwd, explicitPath string) (string, error) {
	if explicitPath != "" {
		target, err := filepath.Abs(explicitPath)
		if err != nil {
			return "", apperrors.Wrap(apperrors.KindInvalidInput, "invalid repo path", err)
		}
		if !isGitRepoPath(target) {
			return "", apperrors.New(apperrors.KindInvalidInput, "provided path is not a git repo (missing .git)")
		}
		return target, nil
	}

	mainPath := filepath.Join(cwd, "main")
	if isGitRepoPath(mainPath) {
		return mainPath, nil
	}
	masterPath := filepath.Join(cwd, "master")
	if isGitRepoPath(masterPath) {
		return masterPath, nil
	}

	if r.Git == nil {
		return "", apperrors.New(apperrors.KindInvalidInput, "could not find target repo")
	}

	gitRoot, err := r.Git.TopLevel(ctx, cwd)
	if err != nil {
		return "", apperrors.New(apperrors.KindInvalidInput, "could not find target repo")
	}

	name := filepath.Base(gitRoot)
	if name == "main" || name == "master" {
		return gitRoot, nil
	}

	if isGitRepoPath(filepath.Join(gitRoot, "main")) {
		return filepath.Join(gitRoot, "main"), nil
	}
	if isGitRepoPath(filepath.Join(gitRoot, "master")) {
		return filepath.Join(gitRoot, "master"), nil
	}

	return gitRoot, nil
}

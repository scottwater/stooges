package engine

import (
	"context"
	"path/filepath"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/git"
	"github.com/scottwater/stooges/internal/model"
)

func (s *Service) Rebase(ctx context.Context, opts model.RebaseOptions) (model.RebaseResult, error) {
	syncRes, err := s.syncRepo(ctx, opts.Repo, opts.Prune)
	if err != nil {
		return model.RebaseResult{}, err
	}

	cwd, err := s.cwd()
	if err != nil {
		return model.RebaseResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	_, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.RebaseResult{}, err
	}

	repoDirs, err := listManagedWorkspaces(layout)
	if err != nil {
		return model.RebaseResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "scan managed repos for rebase", err)
	}

	baseRepoPath := syncRes.RepoPath
	baseBranch := layout.MainBranch
	result := model.RebaseResult{
		BaseRepoPath:   baseRepoPath,
		Rebased:        []string{},
		Conflicted:     []string{},
		SkippedDirty:   []string{},
		SkippedCurrent: []string{},
	}

	for _, repo := range repoDirs {
		if repo == baseRepoPath {
			continue
		}
		name := filepath.Base(repo)
		status, err := s.git.StatusPorcelain(ctx, repo)
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(status) != "" {
			result.SkippedDirty = append(result.SkippedDirty, name)
			continue
		}

		branch, err := s.git.CurrentBranch(ctx, repo)
		if err != nil {
			return result, err
		}
		branch = strings.TrimSpace(branch)
		if branch == "" || branch == baseBranch {
			result.SkippedCurrent = append(result.SkippedCurrent, name)
			continue
		}

		isAncestor, err := s.git.IsAncestor(ctx, repo, baseBranch, branch)
		if err != nil {
			return result, err
		}
		if isAncestor {
			result.SkippedCurrent = append(result.SkippedCurrent, name)
			continue
		}

		if err := s.git.Rebase(ctx, repo, baseBranch); err != nil {
			if gitops.IsRebaseConflict(err) {
				abortErr := s.git.AbortRebase(ctx, repo)
				if abortErr != nil {
					return result, apperrors.Wrap(apperrors.KindRollbackFailure, "rebase conflict cleanup failed", abortErr)
				}
				result.Conflicted = append(result.Conflicted, name)
				continue
			}
			return result, err
		}
		result.Rebased = append(result.Rebased, name)
	}
	return result, nil
}

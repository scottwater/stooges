package engine

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/model"
)

func (s *Service) List(ctx context.Context, _ model.ListOptions) (model.ListResult, error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.ListResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	root, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.ListResult{}, err
	}

	entries := make([]model.WorkspaceListEntry, 0, len(layout.ManagedWorkspaces)+1)
	baseEntry, err := s.listEntryForRepo(ctx, "base", layout.BaseRepoPath)
	if err != nil {
		return model.ListResult{}, err
	}
	entries = append(entries, baseEntry)

	workspaces := append([]string(nil), layout.ManagedWorkspaces...)
	slices.Sort(workspaces)
	for _, workspace := range workspaces {
		repoPath := filepath.Join(layout.WorkspaceRoot, workspace)
		entry, entryErr := s.listEntryForRepo(ctx, workspace, repoPath)
		if entryErr != nil {
			return model.ListResult{}, entryErr
		}
		if entry.Missing {
			continue
		}
		entries = append(entries, entry)
	}

	return model.ListResult{
		WorkspaceRoot: root,
		Entries:       entries,
	}, nil
}

func (s *Service) listEntryForRepo(ctx context.Context, name, repoPath string) (model.WorkspaceListEntry, error) {
	entry := model.WorkspaceListEntry{
		Name:              name,
		Path:              repoPath,
		Branch:            "-",
		LastCommitShort:   "-",
		LastCommitMessage: "(missing git repo)",
	}

	if !isGitRepoPath(repoPath) {
		if pathExists(repoPath) {
			entry.LastCommitMessage = "(not a git repo)"
			return entry, nil
		}
		entry.Missing = true
		return entry, nil
	}

	branch, err := s.git.BranchName(ctx, repoPath)
	if err != nil {
		return model.WorkspaceListEntry{}, err
	}
	if strings.EqualFold(strings.TrimSpace(branch), "head") {
		entry.Branch = "(detached)"
	} else if trimmed := strings.TrimSpace(branch); trimmed != "" {
		entry.Branch = trimmed
	}

	shortSHA, subject, err := s.git.HeadCommit(ctx, repoPath)
	if err != nil {
		return model.WorkspaceListEntry{}, err
	}
	if trimmed := strings.TrimSpace(shortSHA); trimmed != "" {
		entry.LastCommitShort = trimmed
		entry.LastCommitMessage = "(no commit message)"
	}
	if trimmed := strings.TrimSpace(subject); trimmed != "" {
		entry.LastCommitMessage = trimmed
	} else if entry.LastCommitShort == "-" {
		entry.LastCommitMessage = "(no commits)"
	}

	return entry, nil
}

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/model"
)

func (s *Service) Undo(ctx context.Context, opts model.UndoOptions) (model.UndoResult, error) {
	if strings.TrimSpace(opts.Base) != "" {
		return model.UndoResult{}, apperrors.New(apperrors.KindInvalidInput, "undo base override is no longer supported; stooges uses .stooges as base repo")
	}
	cwd, err := s.cwd()
	if err != nil {
		return model.UndoResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.UndoResult{}, err
	}

	result := model.UndoResult{
		WorkspaceRoot: workspaceRoot,
		BaseRepoPath:  layout.BaseRepoPath,
		Steps: []string{
			"warning: undo is non-transactional; if a step fails, recover manually from backup path in output",
		},
	}

	repoDirs, err := listManagedWorkspaces(layout)
	if err != nil {
		return result, err
	}
	for _, repo := range repoDirs {
		status, statusErr := s.git.StatusPorcelain(ctx, repo)
		if statusErr != nil {
			return result, statusErr
		}
		if strings.TrimSpace(status) != "" {
			return result, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("undo blocked: pending git changes in %s", repo))
		}
	}
	result.Steps = append(result.Steps, fmt.Sprintf("step 1: verified clean git status in %d managed repo(s)", len(repoDirs)))
	for _, repo := range repoDirs {
		if err := s.perms.UnlockWritable(repo); err != nil {
			return result, err
		}
	}
	result.Steps = append(result.Steps, "step 1b: unlocked managed repos to allow move/delete operations")

	removed := make([]string, 0)
	for _, repo := range repoDirs {
		if repo == layout.BaseRepoPath {
			continue
		}
		name := filepath.Base(repo)
		if err := os.RemoveAll(repo); err != nil {
			return result, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("remove managed workspace %s", name), err)
		}
		removed = append(removed, name)
	}
	if len(removed) == 0 {
		result.Steps = append(result.Steps, "step 2: removed no managed workspaces (only base repo present)")
	} else {
		result.Steps = append(result.Steps, fmt.Sprintf("step 2: removed managed workspaces: %s", strings.Join(removed, ", ")))
	}

	backupPath, err := uniqueBackupPath(filepath.Dir(workspaceRoot), filepath.Base(workspaceRoot))
	if err != nil {
		return result, apperrors.Wrap(apperrors.KindFilesystemFailure, "prepare unique backup path", err)
	}
	result.BackupPath = backupPath

	if err := os.Rename(layout.BaseRepoPath, backupPath); err != nil {
		return result, apperrors.Wrap(apperrors.KindFilesystemFailure, "move base repo to backup path", err)
	}
	result.Steps = append(result.Steps, fmt.Sprintf("step 3: moved base repo to backup path %s", backupPath))

	backupEntries, err := os.ReadDir(backupPath)
	if err != nil {
		return result, apperrors.Wrap(apperrors.KindFilesystemFailure, "read backup path entries", err)
	}
	for _, entry := range backupEntries {
		dst := filepath.Join(workspaceRoot, entry.Name())
		if pathExists(dst) {
			return result, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("undo blocked: destination already exists: %s", entry.Name()))
		}
	}

	if err := os.Remove(layout.MetadataPath); err != nil && !os.IsNotExist(err) {
		return result, apperrors.Wrap(apperrors.KindFilesystemFailure, "remove workspace metadata", err)
	}
	result.Steps = append(result.Steps, "step 4: removed workspace metadata file")

	for _, entry := range backupEntries {
		src := filepath.Join(backupPath, entry.Name())
		dst := filepath.Join(workspaceRoot, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return result, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("restore %s from backup path", entry.Name()), err)
		}
	}
	result.Steps = append(result.Steps, "step 5: restored .stooges repo contents into workspace root")

	if err := os.Remove(backupPath); err != nil {
		return result, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("remove backup container dir %s", backupPath), err)
	}
	result.Steps = append(result.Steps, fmt.Sprintf("step 6: removed empty backup dir %s", backupPath))
	return result, nil
}

func uniqueBackupPath(parentDir, workspaceName string) (string, error) {
	for i := 0; i < 20; i++ {
		candidate := filepath.Join(parentDir, fmt.Sprintf("%s_%d_%d.bak", workspaceName, time.Now().UnixNano(), i))
		if !pathExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique backup path under %s", parentDir)
}

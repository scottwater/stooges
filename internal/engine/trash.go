package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/model"
)

func (s *Service) Trash(ctx context.Context, opts model.TrashOptions) (model.TrashResult, error) {
	workspace := strings.TrimSpace(opts.Workspace)
	if err := validateWorkspaceEntryName(workspace); err != nil {
		return model.TrashResult{}, err
	}

	cwd, err := s.cwd()
	if err != nil {
		return model.TrashResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot := workspaceRootFromCWD(cwd)
	if strings.TrimSpace(workspaceRoot) == "" {
		return model.TrashResult{}, apperrors.New(apperrors.KindInvalidInput, "workspace path is empty")
	}
	layout, err := loadWorkspaceLayout(workspaceRoot)
	if err != nil {
		return model.TrashResult{}, err
	}
	if !slices.Contains(layout.ManagedWorkspaces, workspace) {
		return model.TrashResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("workspace %q is not managed by stooges", workspace))
	}

	workspacePath := filepath.Join(workspaceRoot, workspace)
	if !pathExists(workspacePath) {
		return model.TrashResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("workspace %q is missing", workspace))
	}
	removalPlan, err := planWorkspaceRemoval(opts.Force)
	if err != nil {
		return model.TrashResult{}, err
	}

	teardown := "skipped"
	teardownErrText := ""
	if strings.TrimSpace(layout.TeardownScript) != "" {
		branch, err := s.git.BranchName(ctx, workspacePath)
		if err != nil {
			err = apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("detect branch for teardown in %s", workspacePath), err)
		} else {
			err = runWorkspaceScript(ctx, layout.TeardownScript, workspaceHookEnv{
				CWD:           cwd,
				WorkspaceRoot: workspaceRoot,
				Source:        hookSourceName(""),
				Branch:        strings.TrimSpace(branch),
				Workspace:     workspace,
				WorkspacePath: workspacePath,
			})
		}
		if err != nil && !opts.Force {
			return model.TrashResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("teardown failed; workspace left at %s", workspacePath), err)
		}
		if err != nil {
			teardown = "failed-forced"
			teardownErrText = err.Error()
		} else {
			teardown = "ok"
		}
	}
	if err := s.chdir(workspaceRoot); err != nil {
		return model.TrashResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "switch to workspace root before trash", err)
	}

	removal, err := removalPlan.remove(ctx, workspacePath)
	if err != nil {
		return model.TrashResult{}, err
	}

	layout.ManagedWorkspaces = removeManagedWorkspace(layout.ManagedWorkspaces, workspace)
	if err := writeWorkspaceMetadata(layout); err != nil {
		return model.TrashResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("workspace %q was removed but metadata update failed; .stooges metadata may still list it", workspace), err)
	}

	return model.TrashResult{
		WorkspaceRoot: workspaceRoot,
		Workspace:     workspace,
		WorkspacePath: workspacePath,
		Removal:       removal,
		Teardown:      teardown,
		TeardownError: teardownErrText,
	}, nil
}

type workspaceRemovalPlan struct {
	trashPath string
}

func planWorkspaceRemoval(force bool) (workspaceRemovalPlan, error) {
	trashPath, lookErr := exec.LookPath("trash")
	if lookErr == nil {
		return workspaceRemovalPlan{trashPath: trashPath}, nil
	}
	if !errors.Is(lookErr, exec.ErrNotFound) && !force {
		return workspaceRemovalPlan{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "locate trash command", lookErr)
	}
	if !force {
		return workspaceRemovalPlan{}, apperrors.New(apperrors.KindInvalidInput, "`trash` command not found in PATH; install it or pass --force to permanently delete")
	}
	return workspaceRemovalPlan{}, nil
}

func (p workspaceRemovalPlan) remove(ctx context.Context, workspacePath string) (string, error) {
	if p.trashPath != "" {
		cmd := exec.CommandContext(ctx, p.trashPath, workspacePath)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			message := strings.TrimSpace(out.String())
			if message != "" {
				return "", apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("trash workspace failed: %s", message), err)
			}
			return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "trash workspace failed", err)
		}
		return "trash", nil
	}
	if err := os.RemoveAll(workspacePath); err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "permanently delete workspace", err)
	}
	return "delete", nil
}

func removeManagedWorkspace(workspaces []string, target string) []string {
	filtered := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace == target {
			continue
		}
		filtered = append(filtered, workspace)
	}
	return normalizeManagedWorkspaces(filtered)
}

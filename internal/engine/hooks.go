package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
)

type workspaceHookEnv struct {
	CWD           string
	WorkspaceRoot string
	Source        string
	Branch        string
	Workspace     string
	WorkspacePath string
}

func runWorkspaceScript(ctx context.Context, scriptPath string, env workspaceHookEnv) error {
	script := strings.TrimSpace(scriptPath)
	if script == "" {
		return nil
	}
	resolved, err := resolveWorkspaceScriptPath(env.WorkspaceRoot, script)
	if err != nil {
		return err
	}
	if info, err := os.Stat(resolved); err != nil {
		if os.IsNotExist(err) {
			return apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("workspace script not found: %s", resolved))
		}
		return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("inspect workspace script %s", resolved), err)
	} else if info.IsDir() {
		return apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("workspace script is a directory: %s", resolved))
	}

	cmd := exec.CommandContext(ctx, resolved)
	cmd.Dir = env.WorkspacePath
	cmd.Env = append(os.Environ(),
		"STOOGES_CWD="+env.CWD,
		"STOOGES_MAIN="+env.WorkspaceRoot,
		"STOOGES_SOURCE="+env.Source,
		"STOOGES_BRANCH="+env.Branch,
		"STOOGES_FOLDER="+env.Workspace,
		"STOOGES_FOLDER_PATH="+env.WorkspacePath,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(out.String())
		if message != "" {
			return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("workspace script failed: %s", message), err)
		}
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "workspace script failed", err)
	}
	return nil
}

func resolveWorkspaceScriptPath(workspaceRoot, script string) (string, error) {
	if filepath.IsAbs(script) {
		return filepath.Clean(script), nil
	}
	return filepath.Join(workspaceRoot, script), nil
}

func setupHookEnv(cwd, workspaceRoot, source, workspace, branch string) workspaceHookEnv {
	return workspaceHookEnv{
		CWD:           cwd,
		WorkspaceRoot: workspaceRoot,
		Source:        strings.TrimSpace(source),
		Branch:        strings.TrimSpace(branch),
		Workspace:     workspace,
		WorkspacePath: filepath.Join(workspaceRoot, workspace),
	}
}

package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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
		"STOOGES_PROJECT="+filepath.Base(filepath.Clean(env.WorkspaceRoot)),
		"STOOGES_SOURCE="+env.Source,
		"STOOGES_BRANCH="+env.Branch,
		"STOOGES_FOLDER="+env.Workspace,
		"STOOGES_FOLDER_PATH="+env.WorkspacePath,
	)
	out := newCreationHookWriter(ctx)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		cause := err
		if ctx.Err() != nil {
			cause = ctx.Err()
		}
		message := fmt.Sprintf("workspace script failed: %s", script)
		if !out.hasReporter {
			if tail := strings.TrimSpace(string(out.tailBytes())); tail != "" {
				message = fmt.Sprintf("%s: %s", message, tail)
			}
		}
		return apperrors.Wrap(apperrors.KindFilesystemFailure, message, cause)
	}
	return nil
}

const creationHookTailLimit = 32 * 1024

type creationHookWriter struct {
	mu          sync.Mutex
	ctx         context.Context
	tail        []byte
	hasReporter bool
}

func newCreationHookWriter(ctx context.Context) *creationHookWriter {
	reporter, _ := creationReporterFromContext(ctx)
	return &creationHookWriter{
		ctx:         ctx,
		tail:        make([]byte, 0, creationHookTailLimit),
		hasReporter: reporter != nil,
	}
}

func (w *creationHookWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.appendTail(p)
	WriteCreationHookOutput(w.ctx, p)
	return len(p), nil
}

func (w *creationHookWriter) appendTail(p []byte) {
	if len(p) >= creationHookTailLimit {
		w.tail = append(w.tail[:0], p[len(p)-creationHookTailLimit:]...)
		return
	}
	if overflow := len(w.tail) + len(p) - creationHookTailLimit; overflow > 0 {
		copy(w.tail, w.tail[overflow:])
		w.tail = w.tail[:len(w.tail)-overflow]
	}
	w.tail = append(w.tail, p...)
}

func (w *creationHookWriter) tailBytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.tail...)
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

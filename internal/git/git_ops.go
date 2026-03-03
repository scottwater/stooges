package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
)

type Ops interface {
	CurrentBranch(ctx context.Context, repo string) (string, error)
	RemoteHEADBranch(ctx context.Context, repo string) (string, error)
	BranchExists(ctx context.Context, repo, branch string) (bool, error)
	LocalBranchExists(ctx context.Context, repo, branch string) (bool, error)
	RemoteBranchExists(ctx context.Context, repo, branch string) (bool, error)
	TopLevel(ctx context.Context, dir string) (string, error)
	StatusPorcelain(ctx context.Context, repo string) (string, error)
	IsAncestor(ctx context.Context, repo, ancestor, descendant string) (bool, error)
	Fetch(ctx context.Context, repo string) error
	FetchPrune(ctx context.Context, repo string) error
	Switch(ctx context.Context, repo, branch string) error
	SwitchCreate(ctx context.Context, repo, branch string) error
	SwitchTrack(ctx context.Context, repo, localBranch, remoteBranch string) error
	PullFFOnly(ctx context.Context, repo string) error
	Rebase(ctx context.Context, repo, onto string) error
	AbortRebase(ctx context.Context, repo string) error
}

type SystemOps struct{}

type RebaseConflictError struct {
	Cause error
}

func (e *RebaseConflictError) Error() string {
	if e == nil || e.Cause == nil {
		return "git rebase conflict"
	}
	return e.Cause.Error()
}

func (e *RebaseConflictError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsRebaseConflict(err error) bool {
	var conflictErr *RebaseConflictError
	return errors.As(err, &conflictErr)
}

func NewSystemOps() *SystemOps {
	return &SystemOps{}
}

func (o *SystemOps) CurrentBranch(ctx context.Context, repo string) (string, error) {
	out, err := o.gitOutput(ctx, repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindGitFailure, "detect current branch", err)
	}
	return strings.TrimSpace(out), nil
}

func (o *SystemOps) RemoteHEADBranch(ctx context.Context, repo string) (string, error) {
	out, err := o.gitOutput(ctx, repo, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindGitFailure, "detect remote default branch", err)
	}
	trimmed := strings.TrimSpace(out)
	trimmed = strings.TrimPrefix(trimmed, "refs/remotes/origin/")
	if trimmed == "" {
		return "", apperrors.New(apperrors.KindGitFailure, "remote HEAD branch is empty")
	}
	return trimmed, nil
}

func (o *SystemOps) BranchExists(ctx context.Context, repo, branch string) (bool, error) {
	if strings.TrimSpace(branch) == "" {
		return false, apperrors.New(apperrors.KindInvalidInput, "branch name is required")
	}
	localRef := "refs/heads/" + branch
	ok, err := o.gitRefExists(ctx, repo, localRef)
	if err != nil {
		return false, apperrors.Wrap(apperrors.KindGitFailure, "check local branch ref", err)
	}
	if ok {
		return true, nil
	}
	remoteRef := "refs/remotes/origin/" + branch
	ok, err = o.gitRefExists(ctx, repo, remoteRef)
	if err != nil {
		return false, apperrors.Wrap(apperrors.KindGitFailure, "check remote branch ref", err)
	}
	return ok, nil
}

func (o *SystemOps) LocalBranchExists(ctx context.Context, repo, branch string) (bool, error) {
	if strings.TrimSpace(branch) == "" {
		return false, apperrors.New(apperrors.KindInvalidInput, "branch name is required")
	}
	localRef := "refs/heads/" + branch
	ok, err := o.gitRefExists(ctx, repo, localRef)
	if err != nil {
		return false, apperrors.Wrap(apperrors.KindGitFailure, "check local branch ref", err)
	}
	return ok, nil
}

func (o *SystemOps) RemoteBranchExists(ctx context.Context, repo, branch string) (bool, error) {
	if strings.TrimSpace(branch) == "" {
		return false, apperrors.New(apperrors.KindInvalidInput, "branch name is required")
	}
	remoteRef := "refs/remotes/origin/" + branch
	ok, err := o.gitRefExists(ctx, repo, remoteRef)
	if err != nil {
		return false, apperrors.Wrap(apperrors.KindGitFailure, "check remote branch ref", err)
	}
	return ok, nil
}

func (o *SystemOps) IgnoredPatternsWithMatches(ctx context.Context, repo string) ([]string, error) {
	ignorePath := filepath.Join(repo, ".gitignore")
	if _, err := os.Stat(ignorePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, apperrors.Wrap(apperrors.KindFilesystemFailure, "stat .gitignore", err)
	}

	out, err := o.gitOutput(ctx, repo, "ls-files", "--others", "-i", "--exclude-from=.gitignore", "--directory")
	if err != nil {
		return nil, apperrors.Wrap(apperrors.KindGitFailure, "list ignored files from .gitignore", err)
	}
	paths := splitNonEmptyLines(out)
	if len(paths) == 0 {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, "git", withC(repo, "check-ignore", "-v", "--stdin")...)
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, apperrors.Wrap(apperrors.KindGitFailure, "resolve matching .gitignore patterns", err)
	}

	seen := map[string]struct{}{}
	patterns := make([]string, 0)
	for _, line := range splitNonEmptyLines(string(raw)) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		meta := strings.SplitN(parts[0], ":", 3)
		if len(meta) != 3 {
			continue
		}
		source := meta[0]
		pattern := strings.TrimSpace(meta[2])
		if pattern == "" || !strings.HasSuffix(source, ".gitignore") {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

func (o *SystemOps) TopLevel(ctx context.Context, dir string) (string, error) {
	out, err := o.gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindGitFailure, "resolve git top-level directory", err)
	}
	return strings.TrimSpace(out), nil
}

func (o *SystemOps) StatusPorcelain(ctx context.Context, repo string) (string, error) {
	out, err := o.gitOutput(ctx, repo, "status", "--porcelain")
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindGitFailure, "git status --porcelain failed", err)
	}
	return out, nil
}

func (o *SystemOps) IsAncestor(ctx context.Context, repo, ancestor, descendant string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", withC(repo, "merge-base", "--is-ancestor", ancestor, descendant)...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, apperrors.Wrap(apperrors.KindGitFailure, "git merge-base --is-ancestor failed", err)
	}
	return true, nil
}

func (o *SystemOps) Fetch(ctx context.Context, repo string) error {
	if err := o.gitRun(ctx, repo, "fetch", "origin"); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, "git fetch failed", err)
	}
	return nil
}

func (o *SystemOps) FetchPrune(ctx context.Context, repo string) error {
	if err := o.gitRun(ctx, repo, "fetch", "origin", "--prune"); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, "git fetch --prune failed", err)
	}
	return nil
}

func (o *SystemOps) Switch(ctx context.Context, repo, branch string) error {
	if err := o.gitRun(ctx, repo, "switch", branch); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("git switch %s failed", branch), err)
	}
	return nil
}

func (o *SystemOps) SwitchCreate(ctx context.Context, repo, branch string) error {
	if err := o.gitRun(ctx, repo, "switch", "-c", branch); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("git switch -c %s failed", branch), err)
	}
	return nil
}

func (o *SystemOps) SwitchTrack(ctx context.Context, repo, localBranch, remoteBranch string) error {
	local := strings.TrimSpace(localBranch)
	remote := strings.TrimSpace(remoteBranch)
	if remote == "" {
		return apperrors.New(apperrors.KindInvalidInput, "remote branch name is required")
	}
	if local == "" || local == remote {
		if err := o.gitRun(ctx, repo, "switch", "--track", "origin/"+remote); err != nil {
			return apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("git switch --track origin/%s failed", remote), err)
		}
		return nil
	}
	if err := o.gitRun(ctx, repo, "switch", "-c", local, "--track", "origin/"+remote); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("git switch -c %s --track origin/%s failed", local, remote), err)
	}
	return nil
}

func (o *SystemOps) PullFFOnly(ctx context.Context, repo string) error {
	if err := o.gitRun(ctx, repo, "pull", "--ff-only"); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, "git pull --ff-only failed", err)
	}
	return nil
}

func (o *SystemOps) Rebase(ctx context.Context, repo, onto string) error {
	out, err := o.gitRunWithOutput(ctx, repo, "rebase", onto)
	if err == nil {
		return nil
	}
	wrapped := apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("git rebase %s failed", onto), err)
	lower := strings.ToLower(strings.TrimSpace(out))
	if strings.Contains(lower, "conflict") || strings.Contains(lower, "could not apply") {
		return &RebaseConflictError{Cause: wrapped}
	}
	return wrapped
}

func (o *SystemOps) AbortRebase(ctx context.Context, repo string) error {
	if err := o.gitRun(ctx, repo, "rebase", "--abort"); err != nil {
		return apperrors.Wrap(apperrors.KindGitFailure, "git rebase --abort failed", err)
	}
	return nil
}

func (o *SystemOps) gitRun(ctx context.Context, repo string, args ...string) error {
	_, err := o.gitRunWithOutput(ctx, repo, args...)
	return err
}

func (o *SystemOps) gitRunWithOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmdArgs := withC(repo, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return "", nil
}

func (o *SystemOps) gitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmdArgs := withC(repo, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func withC(repo string, args ...string) []string {
	if strings.TrimSpace(repo) == "" {
		return args
	}
	prefixed := make([]string, 0, len(args)+2)
	prefixed = append(prefixed, "-C", repo)
	prefixed = append(prefixed, args...)
	return prefixed
}

func splitNonEmptyLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (o *SystemOps) gitRefExists(ctx context.Context, repo, ref string) (bool, error) {
	cmdArgs := withC(repo, "show-ref", "--verify", "--quiet", ref)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

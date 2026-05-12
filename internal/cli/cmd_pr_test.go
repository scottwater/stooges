package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apperrors "github.com/scottwater/stooges/internal/errors"
	gitops "github.com/scottwater/stooges/internal/git"
	"github.com/scottwater/stooges/internal/model"
)

type fakeGitHubPRClient struct {
	authErr       error
	authCalls     int
	authRepoPaths []string
	listPRs       []githubPR
	listErr       error
	listRepoPaths []string
	viewPRs       map[int]githubPR
	viewErrs      map[int]error
	viewCalls     []int
	viewRepoPaths []string
	checkoutCalls []checkoutPRCall
	checkoutErr   error
}

type checkoutPRCall struct {
	repoPath string
	number   int
	branch   string
}

func (f *fakeGitHubPRClient) EnsureAuth(_ context.Context, repoPath string) error {
	f.authCalls++
	f.authRepoPaths = append(f.authRepoPaths, repoPath)
	return f.authErr
}

func (f *fakeGitHubPRClient) ListOpen(_ context.Context, repoPath string) ([]githubPR, error) {
	f.listRepoPaths = append(f.listRepoPaths, repoPath)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]githubPR(nil), f.listPRs...), nil
}

func (f *fakeGitHubPRClient) View(_ context.Context, repoPath string, number int) (githubPR, error) {
	f.viewCalls = append(f.viewCalls, number)
	f.viewRepoPaths = append(f.viewRepoPaths, repoPath)
	if err, ok := f.viewErrs[number]; ok {
		return githubPR{}, err
	}
	if pr, ok := f.viewPRs[number]; ok {
		return pr, nil
	}
	return githubPR{}, nil
}

func (f *fakeGitHubPRClient) Checkout(_ context.Context, repoPath string, number int, branch string) error {
	f.checkoutCalls = append(f.checkoutCalls, checkoutPRCall{repoPath: repoPath, number: number, branch: branch})
	return f.checkoutErr
}

func TestPRCommandRequiresGitHubAuth(t *testing.T) {
	svc := &fakeService{}
	gh := &fakeGitHubPRClient{authErr: errors.New("not logged in")}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cmd.SetArgs([]string{"pr", "37"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected auth error")
	}
	if svc.makeCalled {
		t.Fatal("expected auth failure to stop before workspace creation")
	}
	if gh.authCalls != 1 {
		t.Fatalf("expected one auth check, got %d", gh.authCalls)
	}
}

func TestPRCommandTracksSameRepoPullRequest(t *testing.T) {
	svc := &fakeService{}
	gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
		37: {Number: 37, Title: "Fix shell init", HeadRefName: "feature/shell-init", Author: githubPRAuthor{Login: "scott"}},
	}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cmd.SetArgs([]string{"pr", "37"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.makeCalled {
		t.Fatal("expected pr command to create a workspace")
	}
	if svc.lastMake.Agent != "shell-init" || svc.lastMake.Source != "base" || svc.lastMake.Track != "feature/shell-init" {
		t.Fatalf("unexpected make options: %#v", svc.lastMake)
	}
	if len(gh.checkoutCalls) != 0 {
		t.Fatalf("expected same-repo PR to skip gh pr checkout, got %#v", gh.checkoutCalls)
	}
}

func TestPRCommandChecksOutCrossRepoPullRequest(t *testing.T) {
	svc := &fakeService{}
	gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
		37: {Number: 37, Title: "Forked fix", HeadRefName: "feature/forked-fix", IsCrossRepository: true, Author: githubPRAuthor{Login: "alex"}},
	}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cmd.SetArgs([]string{"pr", "37"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Agent != "forked-fix" || svc.lastMake.Track != "" || !svc.lastMake.NoSetup {
		t.Fatalf("expected setup-deferred base workspace add before gh checkout, got %#v", svc.lastMake)
	}
	if len(gh.checkoutCalls) != 1 {
		t.Fatalf("expected one gh checkout call, got %#v", gh.checkoutCalls)
	}
	call := gh.checkoutCalls[0]
	if call.repoPath != "/tmp/workspace/forked-fix" || call.number != 37 || call.branch != "" {
		t.Fatalf("unexpected gh checkout call: %#v", call)
	}
	if !svc.setupCalled || svc.lastSetup.Workspace != "forked-fix" || svc.lastSetup.Source != "base" || svc.lastSetup.Branch != "" {
		t.Fatalf("expected setup after successful checkout, got called=%v opts=%#v", svc.setupCalled, svc.lastSetup)
	}
}

func TestPRCommandSetupFlagsPassThrough(t *testing.T) {
	t.Run("same repo", func(t *testing.T) {
		svc := &fakeService{}
		gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
			37: {Number: 37, Title: "Fix shell init", HeadRefName: "feature/shell-init", Author: githubPRAuthor{Login: "scott"}},
		}}
		cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}, noopUpdater{}, gh, func(context.Context) (string, error) {
			return "/tmp/repo", nil
		})
		cmd.SetArgs([]string{"pr", "37", "--no-setup", "--rollback-on-setup-failure"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if !svc.lastMake.NoSetup || !svc.lastMake.RollbackOnSetupFailure {
			t.Fatalf("expected setup flags to reach same-repo Make, got %#v", svc.lastMake)
		}
		if svc.setupCalled {
			t.Fatal("same-repo PR setup should be handled by Make")
		}
	})

	t.Run("cross repo", func(t *testing.T) {
		svc := &fakeService{}
		gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
			37: {Number: 37, Title: "Forked fix", HeadRefName: "feature/forked-fix", IsCrossRepository: true, Author: githubPRAuthor{Login: "alex"}},
		}}
		cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}, noopUpdater{}, gh, func(context.Context) (string, error) {
			return "/tmp/repo", nil
		})
		cmd.SetArgs([]string{"pr", "37", "--no-setup", "--rollback-on-setup-failure"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if !svc.lastMake.NoSetup {
			t.Fatalf("expected cross-repo Make to defer setup, got %#v", svc.lastMake)
		}
		if svc.setupCalled {
			t.Fatal("--no-setup should skip post-checkout setup")
		}
	})
}

func TestPRCommandBranchOverrideUsesRequestedLocalBranch(t *testing.T) {
	t.Run("same repo", func(t *testing.T) {
		svc := &fakeService{}
		gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
			37: {Number: 37, Title: "Fix shell init", HeadRefName: "feature/shell-init", Author: githubPRAuthor{Login: "scott"}},
		}}
		cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}, noopUpdater{}, gh, func(context.Context) (string, error) {
			return "/tmp/repo", nil
		})
		cmd.SetArgs([]string{"pr", "37", "--branch", "review/pr-37"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if svc.lastMake.Branch != "review/pr-37" {
			t.Fatalf("expected branch override to reach svc.Make, got %#v", svc.lastMake)
		}
	})

	t.Run("cross repo", func(t *testing.T) {
		svc := &fakeService{}
		gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
			37: {Number: 37, Title: "Forked fix", HeadRefName: "feature/forked-fix", IsCrossRepository: true, Author: githubPRAuthor{Login: "alex"}},
		}}
		cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}, noopUpdater{}, gh, func(context.Context) (string, error) {
			return "/tmp/repo", nil
		})
		cmd.SetArgs([]string{"pr", "37", "--branch", "review/pr-37"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
		if len(gh.checkoutCalls) != 1 || gh.checkoutCalls[0].branch != "review/pr-37" {
			t.Fatalf("expected branch override to reach gh checkout, got %#v", gh.checkoutCalls)
		}
		if svc.lastSetup.Branch != "review/pr-37" {
			t.Fatalf("expected branch override to reach post-checkout setup, got %#v", svc.lastSetup)
		}
	})
}

func TestPRCommandWritesCDTargetForShellIntegration(t *testing.T) {
	svc := &fakeService{}
	gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
		37: {Number: 37, Title: "Fix shell init", HeadRefName: "feature/shell-init", Author: githubPRAuthor{Login: "scott"}},
	}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cdFile := t.TempDir() + "/cd-target"
	t.Setenv("STOOGES_CD_FILE", cdFile)
	cmd.SetArgs([]string{"pr", "37"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	data, err := os.ReadFile(cdFile)
	if err != nil {
		t.Fatalf("read cd file: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "/tmp/workspace/shell-init" {
		t.Fatalf("expected cd target /tmp/workspace/shell-init, got %q", got)
	}
}

func TestPRCommandNoCDSkipsShellIntegrationTarget(t *testing.T) {
	svc := &fakeService{}
	gh := &fakeGitHubPRClient{viewPRs: map[int]githubPR{
		37: {Number: 37, Title: "Fix shell init", HeadRefName: "feature/shell-init", Author: githubPRAuthor{Login: "scott"}},
	}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cdFile := t.TempDir() + "/cd-target"
	t.Setenv("STOOGES_CD_FILE", cdFile)
	cmd.SetArgs([]string{"pr", "37", "--no-cd"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if _, err := os.Stat(cdFile); !os.IsNotExist(err) {
		t.Fatalf("expected no cd target file, got err=%v", err)
	}
}

func TestPRCommandCrossRepoCheckoutFailureRollsBackWorkspace(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "forked-fix")
	svc := &fakeService{makeFn: func(context.Context, model.MakeOptions) (model.MakeResult, error) {
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			return model.MakeResult{}, err
		}
		return model.MakeResult{Created: []string{"forked-fix"}, WorkspaceRoot: root}, nil
	}}
	gh := &fakeGitHubPRClient{
		viewPRs: map[int]githubPR{
			37: {Number: 37, Title: "Forked fix", HeadRefName: "feature/forked-fix", IsCrossRepository: true, Author: githubPRAuthor{Login: "alex"}},
		},
		checkoutErr: errors.New("checkout failed"),
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cmd.SetArgs([]string{"pr", "37"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected checkout failure")
	}
	if !strings.Contains(err.Error(), "removed partial workspace and metadata") {
		t.Fatalf("expected rollback context in error, got %v", err)
	}
	if !svc.rollbackCalled || svc.lastRollback != "forked-fix" {
		t.Fatalf("expected metadata-aware rollback for forked-fix, got called=%v workspace=%q", svc.rollbackCalled, svc.lastRollback)
	}
	if _, statErr := os.Stat(workspacePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected workspace rollback to remove %q, got err=%v", workspacePath, statErr)
	}
	if svc.setupCalled {
		t.Fatal("setup should not run when gh checkout fails")
	}
	if strings.Contains(out.String(), "checked out:") {
		t.Fatalf("did not expect success output after checkout failure, got %q", out.String())
	}
}

func TestPRCommandInteractiveSelectionUsesChosenPullRequest(t *testing.T) {
	svc := &fakeService{}
	gh := &fakeGitHubPRClient{
		listPRs: []githubPR{
			{Number: 11, Title: "First PR", HeadRefName: "feature/first", Author: githubPRAuthor{Login: "moe"}},
			{Number: 12, Title: "Second PR", HeadRefName: "feature/second", Author: githubPRAuthor{Login: "larry"}},
		},
		viewPRs: map[int]githubPR{
			12: {Number: 12, Title: "Second PR", HeadRefName: "feature/second", Author: githubPRAuthor{Login: "larry"}},
		},
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := newRootCmd(svc, Streams{In: strings.NewReader("2\n"), Out: out, ErrOut: errOut}, noopUpdater{}, gh, func(context.Context) (string, error) {
		return "/tmp/repo", nil
	})
	cmd.SetArgs([]string{"pr"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(gh.viewCalls) != 1 || gh.viewCalls[0] != 12 {
		t.Fatalf("expected selected PR #12 to be viewed, got %#v", gh.viewCalls)
	}
	if svc.lastMake.Agent != "second" || svc.lastMake.Track != "feature/second" {
		t.Fatalf("unexpected selected make options: %#v", svc.lastMake)
	}
	plain := out.String()
	if !strings.Contains(plain, "#12") || !strings.Contains(plain, "larry") || !strings.Contains(plain, "Second PR") {
		t.Fatalf("expected interactive list details in output, got %q", plain)
	}
}

func TestSelectPullRequestNumberedTruncatesLongTitles(t *testing.T) {
	prs := []githubPR{{Number: 720, Title: strings.Repeat("A very long title ", 10), Author: githubPRAuthor{Login: "c-miles"}}}
	out := &bytes.Buffer{}
	_, err := selectPullRequestNumbered(strings.NewReader("1\n"), out, prs)
	if err != nil {
		t.Fatalf("selection failed: %v", err)
	}
	printed := out.String()
	if !strings.Contains(printed, "…") {
		t.Fatalf("expected truncated title with ellipsis, got %q", printed)
	}
}

func TestRenderPullRequestSelectionUsesCRLFForRawTTYRendering(t *testing.T) {
	prs := []githubPR{{Number: 720, Title: "Integrate PostHog", Author: githubPRAuthor{Login: "c-miles"}}}
	out := &bytes.Buffer{}
	renderPullRequestSelection(out, prs, 0)
	printed := out.String()
	if !strings.Contains(printed, "\r\n") {
		t.Fatalf("expected CRLF line endings for raw TTY rendering, got %q", printed)
	}
	if strings.Contains(printed, "Integrate PostHog\n") && !strings.Contains(printed, "Integrate PostHog\r\n") {
		t.Fatalf("expected rendered rows to avoid bare LF in raw mode, got %q", printed)
	}
}

func TestResolveGitHubRepoPathUsesGitTopLevelRepo(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	resolved, err := resolveGitHubRepoPath(context.Background(), filepath.Join(repo, "subdir"), fakeTopLevelGit{topLevel: repo})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved != repo {
		t.Fatalf("expected %q, got %q", repo, resolved)
	}
}

func TestResolveGitHubRepoPathFallsBackToStoogesBaseRepo(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, ".stooges")
	if err := os.MkdirAll(filepath.Join(base, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	repo, err := resolveGitHubRepoPath(context.Background(), root, fakeTopLevelGit{topLevelErr: assertTopLevelErr})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if repo != base {
		t.Fatalf("expected %q, got %q", base, repo)
	}
}

func TestResolveGitHubRepoPathSupportsGitDirFiles(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /tmp/worktrees/repo"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	resolved, err := resolveGitHubRepoPath(context.Background(), repo, fakeTopLevelGit{topLevel: repo})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved != repo {
		t.Fatalf("expected %q, got %q", repo, resolved)
	}
}

func TestSystemGitHubRepoLocatorPreservesOperationalErrors(t *testing.T) {
	locator := &systemGitHubRepoLocator{
		cwd: func() (string, error) { return "/tmp/repo", nil },
		git: fakeTopLevelGit{topLevelErr: errors.New("permission denied")},
	}

	_, err := locator.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected resolve error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected original failure to be preserved, got %v", err)
	}
	if apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected operational error, got invalid-input fallback: %v", err)
	}
}

type fakeTopLevelGit struct {
	topLevel    string
	topLevelErr error
}

func (fakeTopLevelGit) CurrentBranch(context.Context, string) (string, error) { return "", nil }
func (fakeTopLevelGit) BranchName(context.Context, string) (string, error)    { return "", nil }
func (fakeTopLevelGit) HeadCommit(context.Context, string) (string, string, error) {
	return "", "", nil
}
func (fakeTopLevelGit) RemoteHEADBranch(context.Context, string) (string, error)   { return "", nil }
func (fakeTopLevelGit) BranchExists(context.Context, string, string) (bool, error) { return false, nil }
func (fakeTopLevelGit) LocalBranchExists(context.Context, string, string) (bool, error) {
	return false, nil
}
func (fakeTopLevelGit) RemoteBranchExists(context.Context, string, string) (bool, error) {
	return false, nil
}
func (f fakeTopLevelGit) TopLevel(context.Context, string) (string, error) {
	if f.topLevelErr != nil {
		return "", f.topLevelErr
	}
	return f.topLevel, nil
}
func (fakeTopLevelGit) StatusPorcelain(context.Context, string) (string, error) { return "", nil }
func (fakeTopLevelGit) IsAncestor(context.Context, string, string, string) (bool, error) {
	return false, nil
}
func (fakeTopLevelGit) Fetch(context.Context, string) error                       { return nil }
func (fakeTopLevelGit) FetchPrune(context.Context, string) error                  { return nil }
func (fakeTopLevelGit) Switch(context.Context, string, string) error              { return nil }
func (fakeTopLevelGit) SwitchCreate(context.Context, string, string) error        { return nil }
func (fakeTopLevelGit) SwitchTrack(context.Context, string, string, string) error { return nil }
func (fakeTopLevelGit) PullFFOnly(context.Context, string) error                  { return nil }
func (fakeTopLevelGit) Rebase(context.Context, string, string) error              { return nil }
func (fakeTopLevelGit) AbortRebase(context.Context, string) error                 { return nil }

var assertTopLevelErr = &topLevelErr{}

type topLevelErr struct{}

func (*topLevelErr) Error() string { return "not a git repo" }

func TestRawInputReaderPrefersOriginalTTYHandle(t *testing.T) {
	raw := strings.NewReader("raw")
	buffered := bufio.NewReader(strings.NewReader("buffered"))
	in := interactivePRInput{raw: raw, buffered: buffered}

	if got := rawInputReader(in); got != raw {
		t.Fatalf("expected raw input reader %p, got %p", raw, got)
	}
}

var _ gitops.Ops = fakeTopLevelGit{}

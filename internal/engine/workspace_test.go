package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/git"
	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/testutil"
)

type fakeCloner struct{}

func (fakeCloner) CheckCapability(context.Context) error { return nil }
func (fakeCloner) CloneRepo(_ context.Context, src, dst string) error {
	return testutil.CopyDir(src, dst)
}

type failingCloner struct {
	err error
}

func (f failingCloner) CheckCapability(context.Context) error { return nil }
func (f failingCloner) CloneRepo(context.Context, string, string) error {
	return f.err
}

type failAfterNClones struct {
	failAfter int
	count     int
}

func (f *failAfterNClones) CheckCapability(context.Context) error { return nil }
func (f *failAfterNClones) CloneRepo(_ context.Context, src, dst string) error {
	f.count++
	if f.count > f.failAfter {
		return errors.New("clone failed")
	}
	return testutil.CopyDir(src, dst)
}

type fakePerms struct {
	unlockCalls int
	lockCalls   int
	lastUnlock  string
	lastLock    string
	unlockErr   error
	lockErr     error
}

func (f *fakePerms) UnlockWritable(path string) error {
	f.unlockCalls++
	f.lastUnlock = path
	return f.unlockErr
}
func (f *fakePerms) LockReadOnly(path string) error {
	f.lockCalls++
	f.lastLock = path
	return f.lockErr
}
func (*fakePerms) CountSymlinks(string) (int, error) { return 0, nil }

type fakeGit struct {
	topLevel             string
	currentBranch        string
	currentBranchByRepo  map[string]string
	remoteBranch         string
	branchExistsByName   map[string]bool
	ignoredPatterns      []string
	ignoredPatternsErr   error
	switchErr            map[string]error
	switchCreateErr      map[string]error
	statusByRepo         map[string]string
	statusErr            error
	ancestorByRepoBranch map[string]bool
	ancestorErr          error
	rebaseErrByRepo      map[string]error
	rebaseErr            error
	abortErr             error
	fetchErr             error
	fetchPruneErr        error
	pullErr              error
	switches             []string
	switchCreates        []string
	rebaseCalls          []string
	abortCalls           []string
	fetchCalls           int
	pruneCalls           int
}

func (f *fakeGit) CurrentBranch(_ context.Context, repo string) (string, error) {
	if f.currentBranchByRepo != nil {
		if branch, ok := f.currentBranchByRepo[repo]; ok && branch != "" {
			return branch, nil
		}
	}
	if f.currentBranch == "" {
		return "", errors.New("no branch")
	}
	return f.currentBranch, nil
}

func (f *fakeGit) RemoteHEADBranch(context.Context, string) (string, error) {
	if f.remoteBranch == "" {
		return "", errors.New("no remote")
	}
	return f.remoteBranch, nil
}

func (f *fakeGit) BranchExists(_ context.Context, _ string, branch string) (bool, error) {
	if f.branchExistsByName != nil {
		if ok, exists := f.branchExistsByName[branch]; exists {
			return ok, nil
		}
	}
	if branch == f.currentBranch || branch == f.remoteBranch {
		return true, nil
	}
	return branch == "main" || branch == "master", nil
}

func (f *fakeGit) IgnoredPatternsWithMatches(context.Context, string) ([]string, error) {
	if f.ignoredPatternsErr != nil {
		return nil, f.ignoredPatternsErr
	}
	if len(f.ignoredPatterns) == 0 {
		return nil, nil
	}
	return append([]string(nil), f.ignoredPatterns...), nil
}

func (f *fakeGit) TopLevel(context.Context, string) (string, error) {
	if f.topLevel == "" {
		return "", errors.New("no git root")
	}
	return f.topLevel, nil
}

func (f *fakeGit) StatusPorcelain(_ context.Context, repo string) (string, error) {
	if f.statusErr != nil {
		return "", f.statusErr
	}
	if f.statusByRepo == nil {
		return "", nil
	}
	return f.statusByRepo[repo], nil
}

func (f *fakeGit) IsAncestor(_ context.Context, repo, ancestor, branch string) (bool, error) {
	if f.ancestorErr != nil {
		return false, f.ancestorErr
	}
	if f.ancestorByRepoBranch == nil {
		return false, nil
	}
	key := repo + "|" + ancestor + "|" + branch
	return f.ancestorByRepoBranch[key], nil
}

func (f *fakeGit) Fetch(context.Context, string) error {
	f.fetchCalls++
	return f.fetchErr
}
func (f *fakeGit) FetchPrune(context.Context, string) error {
	f.pruneCalls++
	return f.fetchPruneErr
}
func (f *fakeGit) PullFFOnly(context.Context, string) error { return f.pullErr }
func (f *fakeGit) Switch(_ context.Context, _ string, branch string) error {
	f.switches = append(f.switches, branch)
	if err, ok := f.switchErr[branch]; ok {
		return err
	}
	return nil
}

func (f *fakeGit) SwitchCreate(_ context.Context, _ string, branch string) error {
	f.switchCreates = append(f.switchCreates, branch)
	if err, ok := f.switchCreateErr[branch]; ok {
		return err
	}
	return nil
}

func (f *fakeGit) Rebase(_ context.Context, repo, onto string) error {
	f.rebaseCalls = append(f.rebaseCalls, repo+"|"+onto)
	if f.rebaseErrByRepo != nil {
		if err, ok := f.rebaseErrByRepo[repo]; ok {
			return err
		}
	}
	return f.rebaseErr
}

func (f *fakeGit) AbortRebase(_ context.Context, repo string) error {
	f.abortCalls = append(f.abortCalls, repo)
	return f.abortErr
}

func newTestService(t *testing.T, cwd string, git *fakeGit) *Service {
	t.Helper()
	cloner := fakeCloner{}
	return NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return cwd, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         cloner,
		Perms:          &fakePerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})
}

func TestMakeExplicitAgentCreates(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	mustWriteFile(t, filepath.Join(layout.BaseRepoPath, "README.md"), []byte("ok"))

	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace})
	res, err := svc.Make(context.Background(), model.MakeOptions{Agent: "moe", Source: "base"})
	if err != nil {
		t.Fatalf("make failed: %v", err)
	}
	if len(res.Created) != 1 || res.Created[0] != "moe" {
		t.Fatalf("unexpected result: %#v", res)
	}
	if _, err := os.Stat(filepath.Join(workspace, "moe", "README.md")); err != nil {
		t.Fatalf("expected cloned file: %v", err)
	}
}

func TestMakeNoAgentCreatesMissingOnly(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry")

	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace})
	res, err := svc.Make(context.Background(), model.MakeOptions{Source: "base"})
	if err != nil {
		t.Fatalf("make failed: %v", err)
	}
	if len(res.Created) != 2 {
		t.Fatalf("expected two created agents, got %#v", res.Created)
	}
	if !pathExists(filepath.Join(workspace, "curly")) || !pathExists(filepath.Join(workspace, "moe")) {
		t.Fatal("expected curly and moe directories")
	}
}

func TestMakeNoAgentRollsBackOnPartialFailure(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	mustWriteFile(t, filepath.Join(layout.BaseRepoPath, "README.md"), []byte("ok"))

	cloner := &failAfterNClones{failAfter: 1}
	git := &fakeGit{topLevel: workspace}
	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return workspace, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         cloner,
		Perms:          &fakePerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	_, err := svc.Make(context.Background(), model.MakeOptions{Source: "base"})
	if err == nil {
		t.Fatal("expected add failure")
	}
	for _, dir := range []string{"larry", "curly", "moe"} {
		if pathExists(filepath.Join(workspace, dir)) {
			t.Fatalf("expected rollback to remove %s", dir)
		}
	}
}

func TestMakeNoAgentGuidanceWhenAllExist(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry", "curly", "moe")

	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace})
	res, err := svc.Make(context.Background(), model.MakeOptions{Source: "base"})
	if err != nil {
		t.Fatalf("make failed: %v", err)
	}
	if res.Guidance == "" {
		t.Fatal("expected guidance message when all defaults exist")
	}
}

func TestMakeFailsWhenExplicitExists(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "moe")

	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace})
	_, err := svc.Make(context.Background(), model.MakeOptions{Agent: "moe", Source: "base"})
	if err == nil || !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestMakeExplicitAgentBranchAutoCreatesBranch(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	mustWriteFile(t, filepath.Join(layout.BaseRepoPath, "README.md"), []byte("ok"))
	git := &fakeGit{topLevel: workspace}

	svc := newTestService(t, workspace, git)
	_, err := svc.Make(context.Background(), model.MakeOptions{Agent: "bob", Source: "base", BranchAuto: true})
	if err != nil {
		t.Fatalf("make failed: %v", err)
	}
	if len(git.switchCreates) != 1 || git.switchCreates[0] != "bob" {
		t.Fatalf("expected branch create [bob], got %#v", git.switchCreates)
	}
}

func TestMakeExplicitAgentBranchSwitchesExistingBranch(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	mustWriteFile(t, filepath.Join(layout.BaseRepoPath, "README.md"), []byte("ok"))
	git := &fakeGit{
		topLevel:           workspace,
		branchExistsByName: map[string]bool{"not_bob": true},
	}

	svc := newTestService(t, workspace, git)
	_, err := svc.Make(context.Background(), model.MakeOptions{Agent: "bob", Source: "base", Branch: "not_bob"})
	if err != nil {
		t.Fatalf("make failed: %v", err)
	}
	if len(git.switches) != 1 || git.switches[0] != "not_bob" {
		t.Fatalf("expected branch switch [not_bob], got %#v", git.switches)
	}
	if len(git.switchCreates) != 0 {
		t.Fatalf("expected no branch create call, got %#v", git.switchCreates)
	}
}

func TestMakeNoAgentBranchOverrideRequiresSingleCreatedWorkspace(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry")

	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace})
	_, err := svc.Make(context.Background(), model.MakeOptions{Source: "base", Branch: "feature-x"})
	if err == nil || !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestSyncUsesConfiguredMainBranch(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "master")
	git := &fakeGit{
		topLevel: workspace,
	}

	svc := newTestService(t, workspace, git)
	_, err := svc.Sync(context.Background(), model.SyncOptions{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(git.switches) != 1 || git.switches[0] != "master" {
		t.Fatalf("unexpected switch sequence: %#v", git.switches)
	}
	if git.fetchCalls != 1 {
		t.Fatalf("expected non-prune fetch call, got %d", git.fetchCalls)
	}
	if git.pruneCalls != 0 {
		t.Fatalf("expected no prune call from sync, got %d", git.pruneCalls)
	}
}

func TestCleanUsesFetchPrune(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main")
	git := &fakeGit{topLevel: workspace}

	svc := newTestService(t, workspace, git)
	_, err := svc.Clean(context.Background(), model.CleanOptions{})
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if git.fetchCalls != 0 {
		t.Fatalf("expected no non-prune fetch from clean, got %d", git.fetchCalls)
	}
	if git.pruneCalls != 1 {
		t.Fatalf("expected one prune fetch call, got %d", git.pruneCalls)
	}
}

func TestSyncRelocksAfterFetchError(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	repo := layout.BaseRepoPath
	perms := &fakePerms{}
	git := &fakeGit{topLevel: workspace, fetchErr: errors.New("fetch failed")}
	cloner := fakeCloner{}

	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return workspace, nil },
		Cloner:         cloner,
		Perms:          perms,
		Git:            git,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	_, err := svc.Sync(context.Background(), model.SyncOptions{})
	if err == nil {
		t.Fatal("expected sync error")
	}
	if perms.unlockCalls != 1 {
		t.Fatalf("expected one unlock call, got %d", perms.unlockCalls)
	}
	if perms.lockCalls != 1 {
		t.Fatalf("expected one relock call after failure, got %d", perms.lockCalls)
	}
	if perms.lastLock != repo {
		t.Fatalf("expected relock path %s, got %s", repo, perms.lastLock)
	}
}

func TestLockLocksRepo(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	repo := layout.BaseRepoPath
	perms := &fakePerms{}
	git := &fakeGit{topLevel: workspace}
	cloner := fakeCloner{}

	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return workspace, nil },
		Cloner:         cloner,
		Perms:          perms,
		Git:            git,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	res, err := svc.Lock(context.Background(), model.LockOptions{})
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}
	if res.RepoPath != repo {
		t.Fatalf("expected lock repo %s, got %s", repo, res.RepoPath)
	}
	if perms.lockCalls != 1 {
		t.Fatalf("expected one lock call, got %d", perms.lockCalls)
	}
	if perms.lastLock != repo {
		t.Fatalf("expected lock path %s, got %s", repo, perms.lastLock)
	}
}

func TestRebaseRebasesSafeAndReportsConflicts(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry", "curly", "moe")
	larry := filepath.Join(workspace, "larry")
	curly := filepath.Join(workspace, "curly")
	moe := filepath.Join(workspace, "moe")

	git := &fakeGit{
		topLevel: workspace,
		currentBranchByRepo: map[string]string{
			larry: "feature-a",
			curly: "feature-b",
			moe:   "main",
		},
		statusByRepo: map[string]string{
			larry: "",
			curly: "",
			moe:   "",
		},
		ancestorByRepoBranch: map[string]bool{
			larry + "|main|feature-a": false,
			curly + "|main|feature-b": false,
			moe + "|main|main":        true,
		},
		rebaseErrByRepo: map[string]error{
			curly: &gitops.RebaseConflictError{Cause: errors.New("conflict")},
		},
	}
	svc := newTestService(t, workspace, git)

	res, err := svc.Rebase(context.Background(), model.RebaseOptions{})
	if err != nil {
		t.Fatalf("rebase failed: %v", err)
	}
	if len(res.Rebased) != 1 || res.Rebased[0] != "larry" {
		t.Fatalf("expected larry rebased, got %#v", res.Rebased)
	}
	if len(res.Conflicted) != 1 || res.Conflicted[0] != "curly" {
		t.Fatalf("expected curly conflict, got %#v", res.Conflicted)
	}
	if len(res.SkippedCurrent) != 1 || res.SkippedCurrent[0] != "moe" {
		t.Fatalf("expected moe skipped-current, got %#v", res.SkippedCurrent)
	}
	if len(git.abortCalls) != 1 || git.abortCalls[0] != curly {
		t.Fatalf("expected one abort for curly, got %#v", git.abortCalls)
	}
}

func TestRebaseSkipsDirtyWorkspace(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry")
	larry := filepath.Join(workspace, "larry")

	git := &fakeGit{
		topLevel: workspace,
		currentBranchByRepo: map[string]string{
			larry: "feature-a",
		},
		statusByRepo: map[string]string{
			larry: " M README.md\n",
		},
	}
	svc := newTestService(t, workspace, git)

	res, err := svc.Rebase(context.Background(), model.RebaseOptions{})
	if err != nil {
		t.Fatalf("rebase failed: %v", err)
	}
	if len(res.SkippedDirty) != 1 || res.SkippedDirty[0] != "larry" {
		t.Fatalf("expected dirty skip for larry, got %#v", res.SkippedDirty)
	}
	if len(git.rebaseCalls) != 0 {
		t.Fatalf("expected no rebase calls for dirty workspace, got %#v", git.rebaseCalls)
	}
}

func TestInitRenamesRepoAndCreatesAgents(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	mustWriteFile(t, filepath.Join(repo, "README.md"), []byte("hello"))
	git := &fakeGit{topLevel: repo, currentBranch: "main"}

	svc := newTestService(t, repo, git)
	res, err := svc.Init(context.Background(), model.InitOptions{})
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	expectedBase := filepath.Join(repo, ".stooges")
	if res.BaseDir != expectedBase {
		t.Fatalf("expected base dir %s, got %s", expectedBase, res.BaseDir)
	}
	for _, name := range []string{"larry", "curly", "moe"} {
		if !pathExists(filepath.Join(repo, name)) {
			t.Fatalf("expected agent dir %s", name)
		}
	}
	if !pathExists(filepath.Join(expectedBase, ".git")) {
		t.Fatalf("expected moved repo at %s", expectedBase)
	}
	if !pathExists(filepath.Join(expectedBase, "README.md")) {
		t.Fatalf("expected README moved under .stooges base repo")
	}
	if pathExists(filepath.Join(repo, "README.md")) {
		t.Fatalf("expected workspace root to no longer contain original repo files")
	}
	if !pathExists(filepath.Join(repo, metadataName)) {
		t.Fatal("expected .stooges metadata file")
	}
}

func TestInitRequiresMainBranchByDefault(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	git := &fakeGit{
		topLevel:           repo,
		currentBranch:      "feature-x",
		branchExistsByName: map[string]bool{"main": false},
	}

	svc := newTestService(t, repo, git)
	_, err := svc.Init(context.Background(), model.InitOptions{})
	if err == nil || !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestInitAllowsMainBranchOverride(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	mustWriteFile(t, filepath.Join(repo, "README.md"), []byte("hello"))
	git := &fakeGit{
		topLevel:           repo,
		currentBranch:      "feature-x",
		branchExistsByName: map[string]bool{"master": true, "main": false},
	}

	svc := newTestService(t, repo, git)
	res, err := svc.Init(context.Background(), model.InitOptions{MainBranch: "master"})
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	expectedBase := filepath.Join(repo, ".stooges")
	if res.BaseDir != expectedBase {
		t.Fatalf("expected base dir %s, got %s", expectedBase, res.BaseDir)
	}
	layout, loadErr := loadWorkspaceLayout(repo)
	if loadErr != nil {
		t.Fatalf("expected valid workspace metadata: %v", loadErr)
	}
	if layout.MainBranch != "master" {
		t.Fatalf("expected metadata main branch master, got %q", layout.MainBranch)
	}
}

func TestInitRejectsUnsupportedMainBranchOverride(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	git := &fakeGit{topLevel: repo, currentBranch: "feature-x"}

	svc := newTestService(t, repo, git)
	_, err := svc.Init(context.Background(), model.InitOptions{MainBranch: "develop"})
	if err == nil || !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestInitFailsWhenRepoDirty(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	mustWriteFile(t, filepath.Join(repo, "README.md"), []byte("hello"))
	git := &fakeGit{
		topLevel:      repo,
		currentBranch: "main",
		statusByRepo: map[string]string{
			repo: " M README.md\n",
		},
	}

	svc := newTestService(t, repo, git)
	_, err := svc.Init(context.Background(), model.InitOptions{})
	if err == nil {
		t.Fatal("expected init error")
	}
	if !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
	if !pathExists(filepath.Join(repo, ".git")) {
		t.Fatal("expected repo unchanged when init is blocked")
	}
	if pathExists(filepath.Join(repo, ".stooges")) {
		t.Fatal("expected no .stooges directory when init is blocked")
	}
}

func TestInitRollsBackOnCloneFailure(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	mustWriteFile(t, filepath.Join(repo, "README.md"), []byte("hello"))
	git := &fakeGit{topLevel: repo, currentBranch: "main"}
	cloner := failingCloner{err: errors.New("clone failed")}

	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return repo, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         cloner,
		Perms:          &fakePerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	_, err := svc.Init(context.Background(), model.InitOptions{Agents: []string{"larry"}})
	if err == nil {
		t.Fatal("expected init error")
	}
	if !pathExists(filepath.Join(repo, ".git")) {
		t.Fatal("expected repo .git restored after rollback")
	}
	if !pathExists(filepath.Join(repo, "README.md")) {
		t.Fatal("expected README restored after rollback")
	}
	if pathExists(filepath.Join(repo, ".stooges")) {
		t.Fatal("expected .stooges dir removed after rollback")
	}
	if pathExists(filepath.Join(repo, "larry")) {
		t.Fatal("expected partially-created workspace removed after rollback")
	}
}

func TestInitAbortsWhenStoogesExists(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	mustMkdirAll(t, filepath.Join(repo, ".stooges"))

	svc := newTestService(t, repo, &fakeGit{topLevel: repo, currentBranch: "main"})
	_, err := svc.Init(context.Background(), model.InitOptions{})
	if err == nil || !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestDoctorFreshWorkspaceReturnsInitGuidanceWithoutError(t *testing.T) {
	workspace := t.TempDir()
	svc := newTestService(t, workspace, &fakeGit{})

	report, err := svc.Doctor(context.Background(), model.DoctorOptions{})
	if err != nil {
		t.Fatalf("expected no error for fresh workspace, got %v", err)
	}

	var repoCheck *model.DoctorCheck
	for i := range report.Checks {
		if report.Checks[i].Name == "repo_resolution" {
			repoCheck = &report.Checks[i]
			break
		}
	}
	if repoCheck == nil || !repoCheck.OK {
		t.Fatalf("expected repo_resolution informational check, got %#v", repoCheck)
	}
	if len(report.Suggestions) == 0 {
		t.Fatal("expected init suggestion")
	}
}

func TestDoctorInsideStoogesRepoIsConfigured(t *testing.T) {
	workspace := t.TempDir()
	layout := mustSetupConfiguredWorkspace(t, workspace, "main")
	git := &fakeGit{topLevel: layout.BaseRepoPath}
	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return layout.BaseRepoPath, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         fakeCloner{},
		Perms:          &fakePerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(fakeCloner{}),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	report, err := svc.Doctor(context.Background(), model.DoctorOptions{})
	if err != nil {
		t.Fatalf("expected doctor success, got %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "repo_resolution" && strings.Contains(strings.ToLower(check.Message), "not configured yet") {
			t.Fatalf("expected configured workspace, got check %#v", check)
		}
	}
}

func TestDoctorFromWorkspaceSubdirIsConfigured(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main")
	subdir := filepath.Join(workspace, "apps", "api")
	mustMkdirAll(t, subdir)
	git := &fakeGit{topLevel: workspace}
	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return subdir, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         fakeCloner{},
		Perms:          &fakePerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(fakeCloner{}),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	report, err := svc.Doctor(context.Background(), model.DoctorOptions{})
	if err != nil {
		t.Fatalf("expected doctor success, got %v", err)
	}
	for _, check := range report.Checks {
		if check.Name == "repo_resolution" && strings.Contains(strings.ToLower(check.Message), "not configured yet") {
			t.Fatalf("expected configured workspace from nested directory, got check %#v", check)
		}
	}
}

func TestDoctorReportsGitignorePatternMatches(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main")
	git := &fakeGit{
		topLevel:        workspace,
		ignoredPatterns: []string{"log/", ".env"},
	}
	svc := newTestService(t, workspace, git)

	report, err := svc.Doctor(context.Background(), model.DoctorOptions{})
	if err != nil {
		t.Fatalf("expected doctor success, got %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.Name == "gitignore_matches" {
			found = true
			if !strings.Contains(check.Message, "log/") || !strings.Contains(check.Message, ".env") {
				t.Fatalf("expected gitignore patterns in message, got %q", check.Message)
			}
		}
	}
	if !found {
		t.Fatal("expected gitignore_matches check")
	}
}

func TestUndoRestoresBaseRepoAsWorkspaceRoot(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "project")
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry")
	mustWriteFile(t, filepath.Join(workspace, "larry", "tmp.txt"), []byte("agent"))

	git := &fakeGit{topLevel: workspace}
	perms := &fakePerms{}
	svc := NewServiceWithDeps(Dependencies{
		CWD:   func() (string, error) { return workspace, nil },
		Perms: perms,
		Git:   git,
	})

	res, err := svc.Undo(context.Background(), model.UndoOptions{})
	if err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if res.WorkspaceRoot != workspace {
		t.Fatalf("expected workspace root %s, got %s", workspace, res.WorkspaceRoot)
	}
	if !pathExists(filepath.Join(workspace, ".git")) {
		t.Fatal("expected main repo moved back to workspace root")
	}
	if !pathExists(filepath.Join(workspace, "README.md")) {
		t.Fatal("expected README moved back to workspace root")
	}
	if pathExists(filepath.Join(workspace, ".stooges")) {
		t.Fatal("expected .stooges dir removed after undo")
	}
	if perms.unlockCalls == 0 {
		t.Fatal("expected undo to unlock repos before moves")
	}
}

func TestUndoLeavesNonManagedEntriesUntouched(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "project")
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry")
	mustWriteFile(t, filepath.Join(workspace, "notes.txt"), []byte("keep me"))
	mustMkdirAll(t, filepath.Join(workspace, "docs"))
	mustWriteFile(t, filepath.Join(workspace, "docs", "plan.md"), []byte("keep me"))

	git := &fakeGit{topLevel: workspace}
	svc := NewServiceWithDeps(Dependencies{
		CWD:   func() (string, error) { return workspace, nil },
		Perms: &fakePerms{},
		Git:   git,
	})

	if _, err := svc.Undo(context.Background(), model.UndoOptions{}); err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if !pathExists(filepath.Join(workspace, "notes.txt")) {
		t.Fatal("expected notes.txt untouched")
	}
	if !pathExists(filepath.Join(workspace, "docs", "plan.md")) {
		t.Fatal("expected docs/plan.md untouched")
	}
}

func TestUndoBlocksDirtyRepos(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "project")
	layout := mustSetupConfiguredWorkspace(t, workspace, "main", "larry")

	git := &fakeGit{
		topLevel: workspace,
		statusByRepo: map[string]string{
			layout.BaseRepoPath: " M README.md\n",
		},
	}
	svc := NewServiceWithDeps(Dependencies{
		CWD:   func() (string, error) { return workspace, nil },
		Perms: &fakePerms{},
		Git:   git,
	})

	_, err := svc.Undo(context.Background(), model.UndoOptions{})
	if err == nil {
		t.Fatal("expected undo to fail for dirty repo")
	}
	if !apperrors.IsKind(err, apperrors.KindInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
	if !pathExists(layout.BaseRepoPath) {
		t.Fatal("expected workspace untouched on dirty repo failure")
	}
}

func TestPreviewInitBranch(t *testing.T) {
	workspace := t.TempDir()
	mustMkdirAll(t, filepath.Join(workspace, ".git"))
	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace, currentBranch: "main"})

	branch, err := svc.PreviewInitBranch(context.Background())
	if err != nil {
		t.Fatalf("preview branch failed: %v", err)
	}
	if branch != "main" {
		t.Fatalf("expected main branch, got %q", branch)
	}
}

func TestInitChdirsToWorkspaceRoot(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	git := &fakeGit{topLevel: repo, currentBranch: "main"}
	chdirPath := ""
	cloner := fakeCloner{}

	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return repo, nil },
		Chdir:          func(path string) error { chdirPath = path; return nil },
		Cloner:         cloner,
		Perms:          &fakePerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	if _, err := svc.Init(context.Background(), model.InitOptions{}); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if chdirPath != repo {
		t.Fatalf("expected chdir to %s, got %s", repo, chdirPath)
	}
}

func TestDoctorExplicitRepoStillFailsWhenInvalid(t *testing.T) {
	workspace := t.TempDir()
	svc := newTestService(t, workspace, &fakeGit{topLevel: workspace})

	_, err := svc.Doctor(context.Background(), model.DoctorOptions{Repo: filepath.Join(workspace, "not-a-repo")})
	if err == nil || !apperrors.IsKind(err, apperrors.KindPreflightFailure) {
		t.Fatalf("expected preflight failure for explicit invalid repo, got %v", err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func mustSetupConfiguredWorkspace(t *testing.T, workspaceRoot, mainBranch string, workspaces ...string) WorkspaceLayout {
	t.Helper()
	layout := layoutFromRoot(workspaceRoot)
	layout.MainBranch = mainBranch
	layout.ManagedWorkspaces = appendManagedWorkspaces(nil, workspaces...)

	mustMkdirAll(t, filepath.Join(layout.BaseRepoPath, ".git"))
	mustWriteFile(t, filepath.Join(layout.BaseRepoPath, "README.md"), []byte("base"))
	for _, workspace := range workspaces {
		mustMkdirAll(t, filepath.Join(layout.WorkspaceRoot, workspace, ".git"))
	}
	if err := writeWorkspaceMetadata(layout); err != nil {
		t.Fatalf("write workspace metadata: %v", err)
	}
	return layout
}

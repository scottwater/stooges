package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/testutil"
)

type integrationCloner struct{}

func (integrationCloner) CheckCapability(context.Context) error { return nil }
func (integrationCloner) CloneRepo(_ context.Context, src, dst string) error {
	return testutil.CopyDir(src, dst)
}

type integrationGit struct {
	topLevel      string
	currentBranch string
}

func (g integrationGit) CurrentBranch(context.Context, string) (string, error) {
	return g.currentBranch, nil
}
func (g integrationGit) RemoteHEADBranch(context.Context, string) (string, error) {
	return "main", nil
}
func (integrationGit) BranchExists(context.Context, string, string) (bool, error) { return true, nil }
func (integrationGit) LocalBranchExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (integrationGit) RemoteBranchExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (g integrationGit) TopLevel(context.Context, string) (string, error) { return g.topLevel, nil }
func (integrationGit) StatusPorcelain(context.Context, string) (string, error) {
	return "", nil
}
func (integrationGit) IsAncestor(context.Context, string, string, string) (bool, error) {
	return true, nil
}
func (integrationGit) Fetch(context.Context, string) error          { return nil }
func (integrationGit) FetchPrune(context.Context, string) error     { return nil }
func (integrationGit) Switch(context.Context, string, string) error { return nil }
func (integrationGit) SwitchCreate(context.Context, string, string) error {
	return nil
}
func (integrationGit) SwitchTrack(context.Context, string, string, string) error { return nil }
func (integrationGit) PullFFOnly(context.Context, string) error                  { return nil }
func (integrationGit) Rebase(context.Context, string, string) error              { return nil }
func (integrationGit) AbortRebase(context.Context, string) error                 { return nil }

type integrationPerms struct{}

func (integrationPerms) UnlockWritable(string) error       { return nil }
func (integrationPerms) LockReadOnly(string) error         { return nil }
func (integrationPerms) CountSymlinks(string) (int, error) { return 0, nil }

func TestIntegrationMakeAdditiveDefaults(t *testing.T) {
	workspace := t.TempDir()
	mustSetupConfiguredWorkspace(t, workspace, "main", "larry")

	git := integrationGit{topLevel: workspace, currentBranch: "main"}
	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return workspace, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         integrationCloner{},
		Perms:          integrationPerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(integrationCloner{}),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	res, err := svc.Make(context.Background(), model.MakeOptions{Source: "base"})
	if err != nil {
		t.Fatalf("make failed: %v", err)
	}
	if len(res.Created) != 2 {
		t.Fatalf("expected 2 created dirs, got %#v", res.Created)
	}
	for _, dir := range []string{"curly", "moe"} {
		if _, err := os.Stat(filepath.Join(workspace, dir)); err != nil {
			t.Fatalf("expected %s dir: %v", dir, err)
		}
	}
}

func TestIntegrationInitFlow(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	mustMkdirAll(t, filepath.Join(repo, ".git"))
	mustWriteFile(t, filepath.Join(repo, "README.md"), []byte("hello"))

	git := integrationGit{topLevel: repo, currentBranch: "main"}
	svc := NewServiceWithDeps(Dependencies{
		CWD:            func() (string, error) { return repo, nil },
		Chdir:          func(string) error { return nil },
		Cloner:         integrationCloner{},
		Perms:          integrationPerms{},
		Git:            git,
		Preflight:      NewPreflightChecker(integrationCloner{}),
		Resolver:       NewRepoResolver(git),
		BranchDetector: NewBranchDetector(git),
	})

	result, err := svc.Init(context.Background(), model.InitOptions{})
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	expectedBase := filepath.Join(repo, ".stooges")
	if result.BaseDir != expectedBase {
		t.Fatalf("expected base dir at %s, got %s", expectedBase, result.BaseDir)
	}
	for _, dir := range []string{"larry", "curly", "moe"} {
		if _, err := os.Stat(filepath.Join(repo, dir)); err != nil {
			t.Fatalf("expected %s dir: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(expectedBase, ".git")); err != nil {
		t.Fatalf("expected base git dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(expectedBase, "README.md")); err != nil {
		t.Fatalf("expected base README: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "README.md")); err == nil {
		t.Fatalf("expected root README to be moved into .stooges base repo")
	}
	if _, err := os.Stat(filepath.Join(repo, metadataName)); err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}
}

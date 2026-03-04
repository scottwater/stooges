package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/version"
)

type fakeService struct {
	initCalled   bool
	makeCalled   bool
	doctorCalled bool
	lockCalled   bool
	listCalled   bool
	rebaseCalled bool
	undoCalled   bool
	lastCtx      context.Context
	lastInit     model.InitOptions
	lastMake     model.MakeOptions
	preview      string
}

func (f *fakeService) Init(ctx context.Context, opts model.InitOptions) (model.InitResult, error) {
	f.initCalled = true
	f.lastCtx = ctx
	f.lastInit = opts
	agents := model.NormalizeAgents(opts.Agents)
	return model.InitResult{BaseDir: "main", Agents: agents}, nil
}
func (f *fakeService) Make(ctx context.Context, opts model.MakeOptions) (model.MakeResult, error) {
	f.makeCalled = true
	f.lastCtx = ctx
	f.lastMake = opts
	return model.MakeResult{Created: []string{"larry"}}, nil
}
func (f *fakeService) Sync(ctx context.Context, _ model.SyncOptions) (model.SyncResult, error) {
	f.lastCtx = ctx
	return model.SyncResult{RepoPath: "main"}, nil
}
func (f *fakeService) Clean(ctx context.Context, _ model.CleanOptions) (model.CleanResult, error) {
	f.lastCtx = ctx
	return model.CleanResult{RepoPath: "main"}, nil
}
func (f *fakeService) List(ctx context.Context, _ model.ListOptions) (model.ListResult, error) {
	f.lastCtx = ctx
	f.listCalled = true
	return model.ListResult{
		WorkspaceRoot: "/tmp/workspace",
		Entries: []model.WorkspaceListEntry{
			{Name: "base", Branch: "main", LastCommitShort: "abc1234", LastCommitMessage: "initial commit"},
		},
	}, nil
}
func (f *fakeService) Rebase(ctx context.Context, _ model.RebaseOptions) (model.RebaseResult, error) {
	f.lastCtx = ctx
	f.rebaseCalled = true
	return model.RebaseResult{BaseRepoPath: "main"}, nil
}
func (f *fakeService) Unlock(ctx context.Context, _ model.UnlockOptions) (model.UnlockResult, error) {
	f.lastCtx = ctx
	return model.UnlockResult{RepoPath: "main"}, nil
}
func (f *fakeService) Lock(ctx context.Context, _ model.LockOptions) (model.LockResult, error) {
	f.lastCtx = ctx
	f.lockCalled = true
	return model.LockResult{RepoPath: "main"}, nil
}
func (f *fakeService) Doctor(ctx context.Context, _ model.DoctorOptions) (model.DoctorReport, error) {
	f.lastCtx = ctx
	f.doctorCalled = true
	return model.DoctorReport{Checks: []model.DoctorCheck{{Name: "git", OK: true, Message: "ok"}}}, nil
}
func (f *fakeService) Undo(ctx context.Context, _ model.UndoOptions) (model.UndoResult, error) {
	f.lastCtx = ctx
	f.undoCalled = true
	return model.UndoResult{WorkspaceRoot: "/tmp/workspace"}, nil
}
func (f *fakeService) PreviewInitBranch(context.Context) (string, error) {
	if strings.TrimSpace(f.preview) == "" {
		return "main", nil
	}
	return f.preview, nil
}

func TestAddSubcommandDispatches(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "moe"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.makeCalled {
		t.Fatal("expected add to call workspace make operation")
	}
	if svc.lastMake.Agent != "moe" {
		t.Fatalf("expected add agent moe, got %#v", svc.lastMake)
	}
}

func TestAddBranchFlagAutoUsesWorkspaceName(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "bob", "-b"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.lastMake.BranchAuto || svc.lastMake.Branch != "" {
		t.Fatalf("expected auto branch mode, got %#v", svc.lastMake)
	}
}

func TestAddBranchFlagNamedUsesProvidedBranch(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "bob", "--branch", "not_bob"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.BranchAuto || svc.lastMake.Branch != "not_bob" {
		t.Fatalf("expected explicit branch mode, got %#v", svc.lastMake)
	}
}

func TestAddTrackFlagUsesProvidedRemoteBranch(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "bob", "--track", "feature/foo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Track != "feature/foo" || svc.lastMake.Branch != "" || svc.lastMake.BranchAuto {
		t.Fatalf("expected track flag passthrough, got %#v", svc.lastMake)
	}
}

func TestAddTrackFlagWithBranchOverridePassesBoth(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "bob", "--track", "feature/foo", "--branch", "local-foo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Track != "feature/foo" || svc.lastMake.Branch != "local-foo" || svc.lastMake.BranchAuto {
		t.Fatalf("expected track + branch passthrough, got %#v", svc.lastMake)
	}
}

func TestNoArgsRunsInteractiveAndDoctor(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader("0\n"), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.doctorCalled {
		t.Fatal("expected doctor call from interactive startup")
	}
}

func TestDoctorJSON(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"doctor", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out.String(), "\"checks\"") {
		t.Fatalf("expected json output, got %s", out.String())
	}
}

func TestVersionCommand(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != version.Value {
		t.Fatalf("expected version %q, got %q", version.Value, strings.TrimSpace(out.String()))
	}
	if svc.doctorCalled || svc.makeCalled || svc.lockCalled || svc.listCalled || svc.rebaseCalled || svc.undoCalled {
		t.Fatalf("version command should not call service methods: %#v", svc)
	}
}

func TestVersionFlag(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != version.Value {
		t.Fatalf("expected version %q, got %q", version.Value, strings.TrimSpace(out.String()))
	}
	if svc.doctorCalled || svc.makeCalled || svc.lockCalled || svc.listCalled || svc.rebaseCalled || svc.undoCalled {
		t.Fatalf("version flag should not call service methods: %#v", svc)
	}
}

func TestListCommandDispatches(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.listCalled {
		t.Fatal("expected list command to call workspace list operation")
	}
	if !strings.Contains(out.String(), "workspace") || !strings.Contains(out.String(), "abc1234") {
		t.Fatalf("expected workspace table output, got %q", out.String())
	}
}

func TestListAliasDispatches(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"ls"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.listCalled {
		t.Fatal("expected ls alias to call workspace list operation")
	}
}

func TestUndoCommandDispatches(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"undo", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.undoCalled {
		t.Fatal("expected undo command to call workspace undo operation")
	}
}

func TestLockCommandDispatches(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"lock"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.lockCalled {
		t.Fatal("expected lock command to call workspace lock operation")
	}
}

func TestRebaseCommandDispatches(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"rebase"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.rebaseCalled {
		t.Fatal("expected rebase command to call workspace rebase operation")
	}
}

func TestInitUsesWorkspaceFlags(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader("y\n"), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"init", "--workspace", "alpha", "--workspace", "beta"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.initCalled {
		t.Fatal("expected init command to call init")
	}
	if len(svc.lastInit.Agents) != 2 || svc.lastInit.Agents[0] != "alpha" || svc.lastInit.Agents[1] != "beta" {
		t.Fatalf("expected agents [alpha beta], got %#v", svc.lastInit.Agents)
	}
}

func TestInitDefaultShowsDetectedBranch(t *testing.T) {
	svc := &fakeService{preview: "master"}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader("y\n"), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"init", "--workspace", "moe"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out.String(), `main-branch="master" (default)`) {
		t.Fatalf("expected detected branch in confirm output, got %q", out.String())
	}
}

func TestInitMainBranchFlag(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader("y\n"), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"init", "--workspace", "moe", "-m", "master"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastInit.MainBranch != "master" {
		t.Fatalf("expected main-branch master, got %q", svc.lastInit.MainBranch)
	}
}

func TestInitConfirmSkipsPrompt(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"init", "--confirm", "--workspace", "bob"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.initCalled {
		t.Fatal("expected init to run with --confirm")
	}
	if strings.Contains(out.String(), "Proceed? [y/N]:") {
		t.Fatalf("did not expect prompt with --confirm, got %q", out.String())
	}
}

func TestCommandsUseCmdContext(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"add", "moe"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastCtx == nil {
		t.Fatal("expected service to receive command context")
	}
	if svc.lastCtx.Err() == nil {
		t.Fatalf("expected cancelled context to propagate, got %#v", svc.lastCtx)
	}
}

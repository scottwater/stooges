package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/scottwater/stooges/internal/engine"
	"github.com/scottwater/stooges/internal/model"
	"github.com/scottwater/stooges/internal/update"
	"github.com/scottwater/stooges/internal/version"
)

type fakeService struct {
	initCalled                 bool
	makeCalled                 bool
	syncCalled                 int
	doctorCalled               bool
	lockCalled                 bool
	listCalled                 bool
	rebaseCalled               bool
	undoCalled                 bool
	lastCtx                    context.Context
	lastInit                   model.InitOptions
	lastMake                   model.MakeOptions
	lastSync                   model.SyncOptions
	syncErr                    error
	preview                    string
	currentWorkspace           engine.CurrentWorkspace
	resolveCurrentWorkspaceErr error
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
	created := "larry"
	if strings.TrimSpace(opts.Agent) != "" {
		created = strings.TrimSpace(opts.Agent)
	}
	return model.MakeResult{Created: []string{created}, WorkspaceRoot: "/tmp/workspace"}, nil
}
func (f *fakeService) Sync(ctx context.Context, opts model.SyncOptions) (model.SyncResult, error) {
	f.syncCalled++
	f.lastCtx = ctx
	f.lastSync = opts
	if f.syncErr != nil {
		return model.SyncResult{}, f.syncErr
	}
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

func (f *fakeService) ResolveCurrentWorkspace(context.Context) (engine.CurrentWorkspace, error) {
	if f.resolveCurrentWorkspaceErr != nil {
		return engine.CurrentWorkspace{}, f.resolveCurrentWorkspaceErr
	}
	if strings.TrimSpace(f.currentWorkspace.Name) == "" {
		return engine.CurrentWorkspace{Name: "larry", Path: "/tmp/workspace/larry", WorkspaceRoot: "/tmp/workspace"}, nil
	}
	return f.currentWorkspace, nil
}

type fakeUpdater struct {
	maybeNotifyCalled int
	upgradeCalled     bool
	lastVersion       string
	notifyText        string
	upgradeResult     update.UpgradeResult
	upgradeErr        error
}

func (f *fakeUpdater) MaybeNotify(_ context.Context, out io.Writer, currentVersion string) error {
	f.maybeNotifyCalled++
	f.lastVersion = currentVersion
	if f.notifyText != "" {
		_, _ = io.WriteString(out, f.notifyText)
	}
	return nil
}

func (f *fakeUpdater) Upgrade(_ context.Context, currentVersion string) (update.UpgradeResult, error) {
	f.upgradeCalled = true
	f.lastVersion = currentVersion
	return f.upgradeResult, f.upgradeErr
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
	if svc.syncCalled != 1 {
		t.Fatalf("expected add to auto-sync base once, got %d", svc.syncCalled)
	}
	if svc.lastMake.Agent != "moe" {
		t.Fatalf("expected add agent moe, got %#v", svc.lastMake)
	}
}

func TestAddNoSyncSkipsAutomaticBaseSync(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "moe", "--no-sync"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.syncCalled != 0 {
		t.Fatalf("expected --no-sync to skip auto-sync, got %d sync calls", svc.syncCalled)
	}
}

func TestAddWithNonBaseSourceSkipsAutomaticBaseSync(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "moe", "--source", "larry"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.syncCalled != 0 {
		t.Fatalf("expected non-base source to skip auto-sync, got %d sync calls", svc.syncCalled)
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
	cmd.SetArgs([]string{"add", "bob", "--branch=not_bob"})

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
	if svc.syncCalled != 0 {
		t.Fatalf("expected add --track to skip auto-sync, got %d sync calls", svc.syncCalled)
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
	cmd.SetArgs([]string{"add", "bob", "--track", "feature/foo", "--branch=local-foo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Track != "feature/foo" || svc.lastMake.Branch != "local-foo" || svc.lastMake.BranchAuto {
		t.Fatalf("expected track + branch passthrough, got %#v", svc.lastMake)
	}
}

func TestAddRejectsExtraPositionalWhenBranchFlagHasExplicitValue(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "bob", "typo", "--branch=local-foo"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected add to reject extra positional arg")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg(s), received 2") {
		t.Fatalf("expected arg validation error, got %v", err)
	}
	if svc.makeCalled {
		t.Fatal("expected add arg validation to prevent make call")
	}
}

func TestAddAllowsExtraPositionalWhenBranchFlagUsesAutoMode(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"add", "bob", "feature/foo", "-b"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Branch != "feature/foo" || svc.lastMake.BranchAuto {
		t.Fatalf("expected second positional branch name in auto mode, got %#v", svc.lastMake)
	}
}

func TestTrackCommandDerivesWorkspaceFromLastBranchSegment(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"track", "feature/foo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.makeCalled {
		t.Fatal("expected track to call workspace make operation")
	}
	if svc.syncCalled != 0 {
		t.Fatalf("expected track to skip auto-sync, got %d sync calls", svc.syncCalled)
	}
	if svc.lastMake.Agent != "foo" || svc.lastMake.Track != "feature/foo" {
		t.Fatalf("expected derived workspace foo tracking feature/foo, got %#v", svc.lastMake)
	}
}

func TestTrackCommandSanitizesWorkspaceWhenTrackHasNoSlash(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"track", "release candidate: 2026-04-15 !!!"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Agent != "release-candidate-2026-04-15" {
		t.Fatalf("expected sanitized workspace name, got %#v", svc.lastMake)
	}
}

func TestTrackCommandWithBranchOverridePassesBoth(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"track", "feature/foo", "--branch=local-foo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Agent != "foo" || svc.lastMake.Track != "feature/foo" || svc.lastMake.Branch != "local-foo" || svc.lastMake.BranchAuto {
		t.Fatalf("expected derived workspace + branch override, got %#v", svc.lastMake)
	}
}

func TestTrackRejectsExtraPositionalWhenBranchFlagHasExplicitValue(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"track", "feature/foo", "typo", "--branch=local-foo"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected track to reject extra positional arg")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 2") {
		t.Fatalf("expected arg validation error, got %v", err)
	}
	if svc.makeCalled {
		t.Fatal("expected track arg validation to prevent make call")
	}
}

func TestTrackAllowsExtraPositionalWhenBranchFlagUsesAutoMode(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"track", "feature/foo", "local-foo", "-b"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Agent != "foo" || svc.lastMake.Track != "feature/foo" || svc.lastMake.Branch != "local-foo" || svc.lastMake.BranchAuto {
		t.Fatalf("expected derived workspace + positional branch name in auto mode, got %#v", svc.lastMake)
	}
}

func TestBranchCommandDerivesWorkspaceFromLastBranchSegment(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"branch", "scott/aud-656"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.makeCalled {
		t.Fatal("expected branch to call workspace make operation")
	}
	if svc.syncCalled != 1 {
		t.Fatalf("expected branch to auto-sync base once, got %d", svc.syncCalled)
	}
	if svc.lastMake.Agent != "aud-656" || svc.lastMake.Branch != "scott/aud-656" || svc.lastMake.Track != "" || svc.lastMake.BranchAuto {
		t.Fatalf("expected derived workspace aud-656 with explicit branch, got %#v", svc.lastMake)
	}
}

func TestBranchCommandSanitizesWorkspaceWhenBranchHasNoSlash(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"branch", "release candidate: 2026-04-15 !!!"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Agent != "release-candidate-2026-04-15" || svc.lastMake.Branch != "release candidate: 2026-04-15 !!!" {
		t.Fatalf("expected sanitized workspace name and original branch, got %#v", svc.lastMake)
	}
}

func TestBranchNoSyncSkipsAutomaticBaseSync(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"branch", "scott/aud-656", "--no-sync"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.syncCalled != 0 {
		t.Fatalf("expected branch --no-sync to skip auto-sync, got %d sync calls", svc.syncCalled)
	}
}

func TestForkCommandUsesCurrentWorkspaceAsSource(t *testing.T) {
	svc := &fakeService{currentWorkspace: engine.CurrentWorkspace{Name: "larry", Path: t.TempDir(), WorkspaceRoot: "/tmp/workspace"}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"fork", "scott/aud-656"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !svc.makeCalled {
		t.Fatal("expected fork to call workspace make operation")
	}
	if svc.lastMake.Agent != "aud-656" || svc.lastMake.Source != "larry" || svc.lastMake.Branch != "scott/aud-656" || svc.lastMake.Track != "" || svc.lastMake.BranchAuto || !svc.lastMake.RequireNewBranch {
		t.Fatalf("expected derived workspace aud-656 from current workspace source, got %#v", svc.lastMake)
	}
}

func TestForkCommandSanitizesWorkspaceWhenBranchHasNoSlash(t *testing.T) {
	svc := &fakeService{currentWorkspace: engine.CurrentWorkspace{Name: "larry", Path: t.TempDir(), WorkspaceRoot: "/tmp/workspace"}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cmd.SetArgs([]string{"fork", "release candidate: 2026-04-15 !!!"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if svc.lastMake.Agent != "release-candidate-2026-04-15" || svc.lastMake.Source != "larry" || svc.lastMake.Branch != "release candidate: 2026-04-15 !!!" || !svc.lastMake.RequireNewBranch {
		t.Fatalf("expected sanitized workspace name and current workspace source, got %#v", svc.lastMake)
	}
}

func TestAddWritesCDTargetForShellIntegration(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cdFile := t.TempDir() + "/cd-target"
	t.Setenv("STOOGES_CD_FILE", cdFile)
	cmd.SetArgs([]string{"add", "bob"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	data, err := os.ReadFile(cdFile)
	if err != nil {
		t.Fatalf("read cd file: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "/tmp/workspace/bob" {
		t.Fatalf("expected cd target /tmp/workspace/bob, got %q", got)
	}
}

func TestAddNoCDSkipsShellIntegrationTarget(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cdFile := t.TempDir() + "/cd-target"
	t.Setenv("STOOGES_CD_FILE", cdFile)
	cmd.SetArgs([]string{"add", "bob", "--no-cd"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if _, err := os.Stat(cdFile); !os.IsNotExist(err) {
		t.Fatalf("expected no cd target file, got err=%v", err)
	}
}

func TestAddWarnsWhenAutoCDTargetWriteFails(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut})
	cdFile := t.TempDir()
	t.Setenv("STOOGES_CD_FILE", cdFile)
	cmd.SetArgs([]string{"add", "bob"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	warning := errOut.String()
	if !strings.Contains(warning, `workspace "bob" was created`) {
		t.Fatalf("expected workspace creation warning context, got %q", warning)
	}
	if !strings.Contains(warning, cdFile) {
		t.Fatalf("expected warning to include cd target file %q, got %q", cdFile, warning)
	}
	if !strings.Contains(warning, "/tmp/workspace/bob") {
		t.Fatalf("expected warning to include workspace path, got %q", warning)
	}
	if !svc.makeCalled {
		t.Fatal("expected workspace creation to still succeed")
	}
}

func TestShellInitCommandPrintsWrapper(t *testing.T) {
	svc := &fakeService{}
	out := &bytes.Buffer{}
	cmd := NewRootCmd(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: &bytes.Buffer{}})
	cmd.SetArgs([]string{"shell-init", "zsh"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "stooges() {") || !strings.Contains(body, "STOOGES_CD_FILE") {
		t.Fatalf("expected shell wrapper output, got %q", body)
	}
	if !strings.Contains(body, "failed to read auto-cd target") || !strings.Contains(body, "failed to cd to") || !strings.Contains(body, "failed to remove temp file") {
		t.Fatalf("expected shell wrapper diagnostics, got %q", body)
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

func TestSubcommandPrintsUpdateNoticeToErrOut(t *testing.T) {
	svc := &fakeService{}
	updater := &fakeUpdater{notifyText: "Update available: v0.79\n"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmdWithUpdater(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, updater)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if updater.maybeNotifyCalled != 1 {
		t.Fatalf("expected MaybeNotify once, got %d", updater.maybeNotifyCalled)
	}
	if !strings.Contains(errOut.String(), "Update available: v0.79") {
		t.Fatalf("expected update notice on stderr, got %q", errOut.String())
	}
}

func TestUpgradeCommandSkipsPassiveNotice(t *testing.T) {
	svc := &fakeService{}
	updater := &fakeUpdater{
		notifyText: "should not print\n",
		upgradeResult: update.UpgradeResult{
			CurrentVersion: version.Value,
			LatestVersion:  "v0.79",
			ExecutablePath: "/tmp/stooges",
		},
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmdWithUpdater(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, updater)
	cmd.SetArgs([]string{"upgrade"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if updater.maybeNotifyCalled != 0 {
		t.Fatalf("expected passive notice to be skipped, got %d calls", updater.maybeNotifyCalled)
	}
	if !updater.upgradeCalled {
		t.Fatal("expected upgrade command to call updater")
	}
	if strings.Contains(errOut.String(), "should not print") {
		t.Fatalf("did not expect stderr notice during upgrade, got %q", errOut.String())
	}
	if !strings.Contains(out.String(), "upgraded stooges from") {
		t.Fatalf("expected upgrade output, got %q", out.String())
	}
}

func TestShellInitCommandSkipsPassiveNotice(t *testing.T) {
	svc := &fakeService{}
	updater := &fakeUpdater{notifyText: "should not print\n"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := NewRootCmdWithUpdater(svc, Streams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, updater)
	cmd.SetArgs([]string{"shell-init", "zsh"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if updater.maybeNotifyCalled != 0 {
		t.Fatalf("expected shell-init to skip passive notice, got %d calls", updater.maybeNotifyCalled)
	}
	if strings.Contains(errOut.String(), "should not print") {
		t.Fatalf("did not expect stderr notice during shell-init, got %q", errOut.String())
	}
	if !strings.Contains(out.String(), "stooges() {") {
		t.Fatalf("expected shell wrapper output, got %q", out.String())
	}
}

package interactive

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/scottwater/stooges/internal/model"
)

type fakeService struct {
	doctorCalls int
	makeCalls   int
	initCalls   int
	lastInit    model.InitOptions
	doctorErr   error
	doctor      model.DoctorReport
	previewErr  error
	preview     string
}

func (f *fakeService) Init(_ context.Context, opts model.InitOptions) (model.InitResult, error) {
	f.initCalls++
	f.lastInit = opts
	return model.InitResult{}, nil
}
func (f *fakeService) Make(context.Context, model.MakeOptions) (model.MakeResult, error) {
	f.makeCalls++
	return model.MakeResult{Created: []string{"moe"}}, nil
}
func (f *fakeService) Sync(context.Context, model.SyncOptions) (model.SyncResult, error) {
	return model.SyncResult{}, nil
}
func (f *fakeService) Clean(context.Context, model.CleanOptions) (model.CleanResult, error) {
	return model.CleanResult{}, nil
}
func (f *fakeService) List(context.Context, model.ListOptions) (model.ListResult, error) {
	return model.ListResult{}, nil
}
func (f *fakeService) Rebase(context.Context, model.RebaseOptions) (model.RebaseResult, error) {
	return model.RebaseResult{}, nil
}
func (f *fakeService) Unlock(context.Context, model.UnlockOptions) (model.UnlockResult, error) {
	return model.UnlockResult{}, nil
}
func (f *fakeService) Lock(context.Context, model.LockOptions) (model.LockResult, error) {
	return model.LockResult{}, nil
}
func (f *fakeService) Doctor(context.Context, model.DoctorOptions) (model.DoctorReport, error) {
	f.doctorCalls++
	if len(f.doctor.Checks) == 0 {
		f.doctor = model.DoctorReport{Checks: []model.DoctorCheck{{Name: "git", OK: true, Message: "ok"}}}
	}
	return f.doctor, f.doctorErr
}
func (f *fakeService) Undo(context.Context, model.UndoOptions) (model.UndoResult, error) {
	return model.UndoResult{}, nil
}

func (f *fakeService) PreviewInitBranch(context.Context) (string, error) {
	if f.previewErr != nil {
		return "", f.previewErr
	}
	if strings.TrimSpace(f.preview) == "" {
		return "", errors.New("no preview branch")
	}
	return f.preview, nil
}

func TestRun_ExitImmediately(t *testing.T) {
	svc := &fakeService{
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "workspace not configured yet (missing ./.stooges)"},
			},
			Suggestions: []string{"run `stooges init` from your repo root to bootstrap .stooges workspace"},
		},
	}
	in := strings.NewReader("0\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	if err := Run(context.Background(), svc, in, out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if svc.doctorCalls == 0 {
		t.Fatal("expected doctor to run at startup")
	}
}

func TestRun_MakeFlow(t *testing.T) {
	svc := &fakeService{
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "resolved base repo /tmp/.stooges"},
			},
		},
	}
	input := strings.Join([]string{
		"1",    // make
		"moe",  // agent
		"base", // source
		"y",    // confirm
		"0",    // exit
	}, "\n") + "\n"

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	if err := Run(context.Background(), svc, strings.NewReader(input), out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if svc.makeCalls != 1 {
		t.Fatalf("expected one make call, got %d", svc.makeCalls)
	}
}

func TestRun_ShowsInitHintWithoutVerboseStatus(t *testing.T) {
	svc := &fakeService{
		doctor: model.DoctorReport{
			Checks:      []model.DoctorCheck{{Name: "git", OK: true, Message: "git found"}},
			Suggestions: []string{"run `stooges init` from your repo root to bootstrap .stooges workspace"},
		},
	}
	in := strings.NewReader("0\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	if err := Run(context.Background(), svc, in, out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	plain := stripANSI(out.String())
	if !strings.Contains(strings.ToLower(plain), "run `stooges init`") {
		t.Fatalf("expected init hint, got %q", plain)
	}
	if strings.Contains(plain, "Preflight status:") {
		t.Fatalf("did not expect verbose preflight output, got %q", plain)
	}
}

func TestRun_UnconfiguredMenuHidesMakeSyncCleanUnlock(t *testing.T) {
	svc := &fakeService{
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "workspace not configured yet (missing ./.stooges)"},
			},
			Suggestions: []string{"run `stooges init` from your repo root to bootstrap .stooges workspace"},
		},
	}
	in := strings.NewReader("0\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	if err := Run(context.Background(), svc, in, out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	menu := stripANSI(out.String())
	if strings.Contains(menu, "add workspace") || strings.Contains(menu, "sync") || strings.Contains(menu, "clean") || strings.Contains(menu, "unlock") || strings.Contains(menu, "lock") || strings.Contains(menu, "rebase") || strings.Contains(menu, "undo workspace layout") {
		t.Fatalf("expected unconfigured menu to hide add/sync/clean/unlock, got %q", menu)
	}
}

func TestRun_ConfiguredMenuShowsParityActions(t *testing.T) {
	svc := &fakeService{
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "resolved base repo /tmp/.stooges"},
			},
		},
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	if err := Run(context.Background(), svc, strings.NewReader("0\n"), out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	menu := stripANSI(out.String())
	for _, want := range []string{"add workspace", "sync", "clean", "unlock", "lock", "rebase", "undo workspace layout"} {
		if !strings.Contains(menu, want) {
			t.Fatalf("expected %q in menu, got %q", want, menu)
		}
	}
}

func TestRun_QExits(t *testing.T) {
	svc := &fakeService{
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "workspace not configured yet (missing ./.stooges)"},
			},
		},
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	if err := Run(context.Background(), svc, strings.NewReader("q\n"), out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	plain := stripANSI(out.String())
	if !strings.Contains(plain, "signing off") {
		t.Fatalf("expected exit output, got %q", plain)
	}
}

func TestRun_DoctorFailureReturnsImmediately(t *testing.T) {
	svc := &fakeService{
		doctorErr: errors.New("doctor down"),
	}
	in := strings.NewReader("0\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	err := Run(context.Background(), svc, in, out, errOut)
	if err == nil {
		t.Fatal("expected doctor error")
	}
	if !strings.Contains(stripANSI(errOut.String()), "Safety check failed") {
		t.Fatalf("expected safety warning, got %q", errOut.String())
	}
	if strings.Contains(stripANSI(out.String()), "Choose action:") {
		t.Fatalf("expected no interactive menu on doctor failure, got %q", out.String())
	}
}

func TestRun_InitBlankAgentUsesDefaults(t *testing.T) {
	svc := &fakeService{
		preview: "main",
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "workspace not configured yet (missing ./.stooges)"},
			},
			Suggestions: []string{"run `stooges init` from your repo root to bootstrap .stooges workspace"},
		},
	}

	input := strings.Join([]string{
		"1", // init
		"",  // branch override
		"",  // blank workspace -> defaults
		"y", // confirm
	}, "\n") + "\n"

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	if err := Run(context.Background(), svc, strings.NewReader(input), out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if svc.initCalls != 1 {
		t.Fatalf("expected one init call, got %d", svc.initCalls)
	}
	if svc.lastInit.Agents != nil {
		t.Fatalf("expected nil agents for blank input, got %#v", svc.lastInit.Agents)
	}
	plain := stripANSI(out.String())
	if !strings.Contains(plain, `Action: init main-branch="main" (default) workspaces=larry, curly, and moe (default)`) {
		t.Fatalf("expected explicit auto-detected init summary, got %q", plain)
	}
	if strings.Contains(plain, "Choose action:") && strings.Count(plain, "Choose action:") > 1 {
		t.Fatalf("expected interactive mode to exit after successful init, got %q", plain)
	}
}

func TestRun_InitSingleAgent(t *testing.T) {
	svc := &fakeService{
		preview: "main",
		doctor: model.DoctorReport{
			Checks: []model.DoctorCheck{
				{Name: "git", OK: true, Message: "git found"},
				{Name: "cow_clone", OK: true, Message: "copy-on-write clone supported"},
				{Name: "workspace", OK: true, Message: "workspace path is valid"},
				{Name: "repo_resolution", OK: true, Message: "workspace not configured yet (missing ./.stooges)"},
			},
			Suggestions: []string{"run `stooges init` from your repo root to bootstrap .stooges workspace"},
		},
	}

	input := strings.Join([]string{
		"1",   // init
		"",    // branch override
		"moe", // single workspace
		"y",   // confirm
	}, "\n") + "\n"

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	if err := Run(context.Background(), svc, strings.NewReader(input), out, errOut); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if svc.initCalls != 1 {
		t.Fatalf("expected one init call, got %d", svc.initCalls)
	}
	if len(svc.lastInit.Agents) != 1 || svc.lastInit.Agents[0] != "moe" {
		t.Fatalf("expected one workspace [moe], got %#v", svc.lastInit.Agents)
	}
	plain := stripANSI(out.String())
	if !strings.Contains(plain, `Action: init main-branch="main" (default) workspaces=moe`) {
		t.Fatalf("expected single workspace summary, got %q", plain)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

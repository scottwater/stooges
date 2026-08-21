package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingCreationReporter struct {
	mu        sync.Mutex
	events    []CreationProgress
	output    bytes.Buffer
	reportErr error
	outputErr error
}

func (r *recordingCreationReporter) Report(event CreationProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return r.reportErr
}

func (r *recordingCreationReporter) HookOutput(p []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.output.Write(p)
	return r.outputErr
}

func (r *recordingCreationReporter) snapshot() ([]CreationProgress, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]CreationProgress(nil), r.events...), r.output.String()
}

func TestRunCreationPhaseReportsIdentityElapsedAndCompletion(t *testing.T) {
	reporter := &recordingCreationReporter{}
	ctx := WithCreationReporter(context.Background(), reporter, CreationIdentity{Workspace: "bob", Current: 1, Total: 3})
	err := runCreationPhase(ctx, CreationProgress{Phase: PhaseCopyWorkspace}, func() error {
		time.Sleep(time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("phase failed: %v", err)
	}
	events, _ := reporter.snapshot()
	if len(events) != 2 {
		t.Fatalf("expected two events, got %#v", events)
	}
	if events[0].Status != ProgressStarted || events[1].Status != ProgressCompleted {
		t.Fatalf("unexpected statuses: %#v", events)
	}
	if events[0].Workspace != "bob" || events[0].Current != 1 || events[0].Total != 3 {
		t.Fatalf("missing identity: %#v", events[0])
	}
	if events[1].Elapsed <= 0 {
		t.Fatalf("expected elapsed duration, got %s", events[1].Elapsed)
	}
}

func TestRunCreationPhaseReturnsExactErrorAndReportsStatus(t *testing.T) {
	ordinaryErr := errors.New("ordinary failure")
	for _, tc := range []struct {
		name   string
		opErr  error
		status CreationStatus
	}{
		{name: "failure", opErr: ordinaryErr, status: ProgressFailed},
		{name: "cancellation", opErr: context.Canceled, status: ProgressCancelled},
		{name: "deadline", opErr: context.DeadlineExceeded, status: ProgressCancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reporter := &recordingCreationReporter{reportErr: errors.New("closed stderr")}
			ctx := WithCreationReporter(context.Background(), reporter, CreationIdentity{Workspace: "bob", Current: 1, Total: 1})
			err := runCreationPhase(ctx, CreationProgress{Phase: PhaseSetup, Detail: "setup.sh"}, func() error { return tc.opErr })
			if err != tc.opErr {
				t.Fatalf("operation error changed: got %v want exact %v", err, tc.opErr)
			}
			events, _ := reporter.snapshot()
			if got := events[len(events)-1].Status; got != tc.status {
				t.Fatalf("expected %s, got %s", tc.status, got)
			}
		})
	}
}

func TestRunWorkspaceScriptStreamsBothChannelsAndPreservesBytes(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "bob")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "setup.sh")
	body := "#!/bin/sh\nprintf '\\033[32mstdout\\033[0m\\n'\nprintf 'stderr\\n' >&2\nprintf 'partial'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	reporter := &recordingCreationReporter{}
	ctx := WithCreationReporter(context.Background(), reporter, CreationIdentity{Workspace: "bob", Current: 1, Total: 1})
	err := runWorkspaceScript(ctx, script, workspaceHookEnv{WorkspaceRoot: root, Workspace: "bob", WorkspacePath: workspace})
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	_, output := reporter.snapshot()
	want := "\x1b[32mstdout\x1b[0m\nstderr\npartial"
	if output != want {
		t.Fatalf("hook bytes changed or duplicated: got %q want %q", output, want)
	}
}

func TestRunWorkspaceScriptIgnoresReporterFailures(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "bob")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "setup.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reporter := &recordingCreationReporter{reportErr: errors.New("closed stderr"), outputErr: errors.New("closed stderr")}
	ctx := WithCreationReporter(context.Background(), reporter, CreationIdentity{Workspace: "bob", Current: 1, Total: 1})
	if err := runWorkspaceScript(ctx, script, workspaceHookEnv{WorkspaceRoot: root, Workspace: "bob", WorkspacePath: workspace}); err != nil {
		t.Fatalf("reporter failure changed hook result: %v", err)
	}
}

func TestCreationHookWriterKeepsExactBoundedSuffix(t *testing.T) {
	writer := newCreationHookWriter(context.Background())
	input := bytes.Repeat([]byte("x"), creationHookTailLimit+17)
	if n, err := writer.Write(input); err != nil || n != len(input) {
		t.Fatalf("write failed: n=%d err=%v", n, err)
	}
	tail := writer.tailBytes()
	if len(tail) != creationHookTailLimit || !bytes.Equal(tail, input[len(input)-creationHookTailLimit:]) {
		t.Fatalf("tail is not exact %d-byte suffix", creationHookTailLimit)
	}
}

func TestReporterReceivesDefensiveCopies(t *testing.T) {
	reporter := &mutatingCreationReporter{}
	ctx := WithCreationReporter(context.Background(), reporter, CreationIdentity{})
	completed := []string{"larry"}
	payload := []byte("hook")
	ReportCreationProgress(ctx, CreationProgress{Phase: PhaseCreation, Summary: CreationSummary{Completed: completed}})
	WriteCreationHookOutput(ctx, payload)
	if completed[0] != "larry" || string(payload) != "hook" {
		t.Fatalf("reporter mutated caller-owned values: completed=%q payload=%q", completed, payload)
	}
}

type mutatingCreationReporter struct{}

func (mutatingCreationReporter) Report(event CreationProgress) error {
	if len(event.Summary.Completed) > 0 {
		event.Summary.Completed[0] = "changed"
	}
	return errors.New("ignored")
}

func (mutatingCreationReporter) HookOutput(p []byte) error {
	if len(p) > 0 {
		p[0] = 'X'
	}
	return errors.New("ignored")
}

func TestRunWorkspaceScriptFailureUsesBoundedTailWithoutReporter(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "bob")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "setup.sh")
	body := "#!/bin/sh\nprintf 'BEGIN-'\ni=0; while [ $i -lt 40000 ]; do printf x; i=$((i+1)); done\nprintf -- '-END\\n'\nexit 42\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	err := runWorkspaceScript(context.Background(), script, workspaceHookEnv{WorkspaceRoot: root, Workspace: "bob", WorkspacePath: workspace})
	if err == nil {
		t.Fatal("expected failure")
	}
	if strings.Contains(err.Error(), "BEGIN-") || !strings.Contains(err.Error(), "-END") || !strings.Contains(err.Error(), "exit status 42") {
		t.Fatalf("expected bounded final output and status, got %v", err)
	}
}

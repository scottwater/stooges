package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottwater/stooges/internal/engine"
)

func TestCreationRendererNonTTYPrintsDurableLines(t *testing.T) {
	out := &bytes.Buffer{}
	now := time.Unix(100, 0)
	r := newCreationRenderer(out, rendererConfig{TTY: false, Now: func() time.Time { return now }})
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseCopyWorkspace, Status: engine.ProgressStarted, Workspace: "bob", Current: 1, Total: 1}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(1500 * time.Millisecond)
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseCopyWorkspace, Status: engine.ProgressCompleted, Workspace: "bob", Current: 1, Total: 1, Elapsed: 1500 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	r.Close()
	want := "[1/1] bob: Copying workspace...\n[1/1] bob: Copying workspace done (1.5s)\n"
	if got := out.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCreationRendererTTYDelaysAndShowsElapsed(t *testing.T) {
	out := &bytes.Buffer{}
	now := time.Unix(100, 0)
	delay := make(chan time.Time, 1)
	ticks := make(chan time.Time, 2)
	r := newCreationRenderer(out, rendererConfig{
		TTY:   true,
		Now:   func() time.Time { return now },
		After: func(time.Duration) <-chan time.Time { return delay },
		Tick:  ticks,
	})
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseSetup, Status: engine.ProgressStarted, Workspace: "bob", Current: 1, Total: 1, Detail: "bin/workspace-setup"}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("spinner flashed before delay: %q", out.String())
	}
	now = now.Add(250 * time.Millisecond)
	delay <- now
	waitForRendererOutput(t, r, out, "Running setup: bin/workspace-setup")
	if strings.Contains(rendererOutput(r, out), "0.2s") {
		t.Fatalf("elapsed shown before one second: %q", rendererOutput(r, out))
	}
	now = now.Add(time.Second)
	ticks <- now
	waitForRendererOutput(t, r, out, "(1.2s)")
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseSetup, Status: engine.ProgressCompleted, Workspace: "bob", Current: 1, Total: 1, Detail: "bin/workspace-setup", Elapsed: 1250 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if strings.Contains(rendererOutput(r, out), "done\n") {
		t.Fatalf("TTY success left permanent line: %q", rendererOutput(r, out))
	}
}

func TestCreationRendererSuspendsForCompleteAndPartialHookLines(t *testing.T) {
	out := &bytes.Buffer{}
	delay := make(chan time.Time, 1)
	ticks := make(chan time.Time)
	r := newCreationRenderer(out, rendererConfig{TTY: true, Now: time.Now, After: func(time.Duration) <-chan time.Time { return delay }, Tick: ticks})
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseSetup, Status: engine.ProgressStarted, Workspace: "bob", Current: 1, Total: 1, Detail: "setup.sh"}); err != nil {
		t.Fatal(err)
	}
	delay <- time.Now()
	waitForRendererOutput(t, r, out, "Running setup: setup.sh")
	raw := []byte("\x1b[32mfirst\x1b[0m\npartial")
	if err := r.HookOutput(raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendererOutput(r, out), "partial") {
		t.Fatalf("partial line written before completion: %q", rendererOutput(r, out))
	}
	if err := r.HookOutput([]byte(" line\n")); err != nil {
		t.Fatal(err)
	}
	r.Close()
	got := rendererOutput(r, out)
	if strings.Count(got, "\x1b[32mfirst\x1b[0m\n") != 1 || strings.Count(got, "partial line\n") != 1 {
		t.Fatalf("hook bytes not preserved once: %q", got)
	}
	if !strings.Contains(got, "\r\x1b[2K") || strings.Count(got, "Running setup: setup.sh") < 2 {
		t.Fatalf("status was not cleared and redrawn: %q", got)
	}
}

func TestCreationRendererFlushesIncompleteHookLineOnClose(t *testing.T) {
	out := &bytes.Buffer{}
	r := newCreationRenderer(out, rendererConfig{TTY: false, Now: time.Now})
	if err := r.HookOutput([]byte("last partial")); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if got := out.String(); got != "last partial" {
		t.Fatalf("got %q", got)
	}
}

func TestCreationRendererLeavesFailureAndCancellationLines(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status engine.CreationStatus
		word   string
	}{
		{name: "failure", status: engine.ProgressFailed, word: "failed"},
		{name: "cancellation", status: engine.ProgressCancelled, word: "cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			r := newCreationRenderer(out, rendererConfig{TTY: true, Now: time.Now})
			event := engine.CreationProgress{Phase: engine.PhaseSetup, Status: tc.status, Workspace: "bob", Current: 1, Total: 1, Detail: "setup.sh", Elapsed: 2 * time.Second}
			if err := r.Report(event); err != nil {
				t.Fatal(err)
			}
			r.Close()
			if !strings.Contains(out.String(), "! [1/1] bob — Running setup: setup.sh "+tc.word+" (2.0s)\n") {
				t.Fatalf("unexpected terminal line: %q", out.String())
			}
		})
	}
}

func TestCreationRendererPrintsPartialFailureSummary(t *testing.T) {
	out := &bytes.Buffer{}
	r := newCreationRenderer(out, rendererConfig{TTY: false, Now: time.Now})
	event := engine.CreationProgress{
		Phase:     engine.PhaseCreation,
		Status:    engine.ProgressFailed,
		Workspace: "curly",
		Summary: engine.CreationSummary{
			Completed:    []string{"larry"},
			Failed:       "curly",
			RetainedPath: "/tmp/workspace/curly",
			Unstarted:    []string{"moe"},
		},
	}
	if err := r.Report(event); err != nil {
		t.Fatal(err)
	}
	r.Close()
	want := "completed: larry; failed: curly (retained at /tmp/workspace/curly); not started: moe\n"
	if got := out.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCreationRendererMarksRollbackFailureStateUncertain(t *testing.T) {
	out := &bytes.Buffer{}
	r := newCreationRenderer(out, rendererConfig{TTY: false, Now: time.Now})
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseCreation, Status: engine.ProgressFailed, Summary: engine.CreationSummary{Failed: "bob", RollbackError: "permission denied"}}); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if !strings.Contains(out.String(), "rollback failed (workspace state uncertain): permission denied") {
		t.Fatalf("missing uncertain state warning: %q", out.String())
	}
}

func TestRunCLIProgressPhasePreservesOpaqueCancellation(t *testing.T) {
	out := &bytes.Buffer{}
	r := newCreationRenderer(out, rendererConfig{TTY: false, Now: time.Now})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx = engine.WithCreationReporter(ctx, r, engine.CreationIdentity{Workspace: "bob", Current: 1, Total: 1})
	err := runCLIProgressPhase(ctx, engine.CreationProgress{Phase: engine.PhaseCheckoutPR, Detail: "#37"}, func() error {
		return errors.New("checkout interrupted")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation cause, got %v", err)
	}
	r.Close()
	if !strings.Contains(out.String(), "Checking out PR: #37 cancelled") {
		t.Fatalf("expected cancelled checkout progress, got %q", out.String())
	}
}

func TestCreationRendererWriterFailureIsBestEffort(t *testing.T) {
	r := newCreationRenderer(errorWriter{err: errors.New("closed stderr")}, rendererConfig{TTY: false, Now: time.Now})
	if err := r.Report(engine.CreationProgress{Phase: engine.PhaseCopyWorkspace, Status: engine.ProgressStarted, Workspace: "bob", Current: 1, Total: 1}); err == nil {
		t.Fatal("renderer should expose the diagnostic error for the engine to ignore")
	}
	r.Close()
}

func waitForRendererOutput(t *testing.T, r *creationRenderer, out *bytes.Buffer, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rendererOutput(r, out), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in %q", want, rendererOutput(r, out))
}

func rendererOutput(r *creationRenderer, out *bytes.Buffer) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return out.String()
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

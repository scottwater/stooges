package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/scottwater/stooges/internal/engine"
)

const (
	creationSpinnerDelay = 250 * time.Millisecond
	creationSpinnerTick  = 120 * time.Millisecond
)

var creationSpinnerFrames = []string{"|", "/", "-", `\`}

type rendererConfig struct {
	TTY   bool
	Now   func() time.Time
	After func(time.Duration) <-chan time.Time
	Tick  <-chan time.Time
}

type creationRenderer struct {
	mu       sync.Mutex
	out      io.Writer
	tty      bool
	now      func() time.Time
	after    func(time.Duration) <-chan time.Time
	tick     <-chan time.Time
	closed   bool
	writeErr error

	current   engine.CreationProgress
	startedAt time.Time
	phaseStop chan struct{}
	visible   bool
	frame     int
	partial   []byte
}

func newCreationRenderer(out io.Writer, config rendererConfig) *creationRenderer {
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.After == nil {
		config.After = time.After
	}
	return &creationRenderer{
		out:   out,
		tty:   config.TTY,
		now:   config.Now,
		after: config.After,
		tick:  config.Tick,
	}
}

func newCreationContext(ctx context.Context, streams Streams, identity engine.CreationIdentity) (context.Context, *creationRenderer) {
	renderer := newCreationRenderer(streams.ErrOut, rendererConfig{
		TTY:   isTTYWriter(streams.ErrOut),
		Now:   time.Now,
		After: time.After,
	})
	return engine.WithCreationReporter(ctx, renderer, identity), renderer
}

func (r *creationRenderer) Report(event engine.CreationProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.writeErr
	}

	if event.Phase == engine.PhaseCreation && event.Status != engine.ProgressStarted {
		r.stopPhaseLocked()
		if r.flushPartialLocked() {
			r.writeLocked([]byte("\n"))
		}
		if event.Status != engine.ProgressCompleted {
			r.writeSummaryLocked(event)
		}
		return r.writeErr
	}

	switch event.Status {
	case engine.ProgressStarted:
		r.stopPhaseLocked()
		r.current = event
		r.startedAt = r.now()
		r.visible = false
		r.frame = 0
		if r.tty {
			stop := make(chan struct{})
			r.phaseStop = stop
			go r.animate(event, stop)
		} else {
			r.writeLocked([]byte(fmt.Sprintf("%s: %s...\n", creationIdentityLabel(event), creationPhaseLabel(event))))
		}
	case engine.ProgressCompleted, engine.ProgressFailed, engine.ProgressCancelled:
		r.stopPhaseLocked()
		if r.flushPartialLocked() {
			r.writeLocked([]byte("\n"))
		}
		elapsed := r.eventElapsedLocked(event)
		if r.tty {
			if r.visible {
				r.clearLocked()
			}
			if event.Status != engine.ProgressCompleted {
				word := "failed"
				if event.Status == engine.ProgressCancelled {
					word = "cancelled"
				}
				r.writeLocked([]byte(fmt.Sprintf("! %s — %s %s%s\n", creationIdentityLabel(event), creationPhaseLabel(event), word, elapsedSuffix(elapsed, true))))
			}
		} else {
			word := "done"
			if event.Status == engine.ProgressFailed {
				word = "failed"
			} else if event.Status == engine.ProgressCancelled {
				word = "cancelled"
			}
			r.writeLocked([]byte(fmt.Sprintf("%s: %s %s%s\n", creationIdentityLabel(event), creationPhaseLabel(event), word, elapsedSuffix(elapsed, true))))
		}
	}
	return r.writeErr
}

func (r *creationRenderer) HookOutput(p []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.writeErr
	}

	combined := append(append([]byte(nil), r.partial...), p...)
	r.partial = r.partial[:0]
	for {
		newline := bytes.IndexByte(combined, '\n')
		if newline < 0 {
			r.partial = append(r.partial, combined...)
			break
		}
		line := combined[:newline+1]
		combined = combined[newline+1:]
		if r.tty && r.visible {
			r.clearLocked()
		}
		r.writeLocked(line)
		if r.tty && r.current.Status == engine.ProgressStarted {
			r.renderLocked(creationSpinnerFrames[r.frame%len(creationSpinnerFrames)], r.now())
			r.visible = true
			r.frame++
		}
	}
	return r.writeErr
}

func (r *creationRenderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.stopPhaseLocked()
	r.flushPartialLocked()
	if r.tty && r.visible {
		r.clearLocked()
	}
	r.closed = true
}

func (r *creationRenderer) animate(event engine.CreationProgress, stop <-chan struct{}) {
	select {
	case <-stop:
		return
	case <-r.after(creationSpinnerDelay):
	}

	var ticker *time.Ticker
	ticks := r.tick
	if ticks == nil {
		ticker = time.NewTicker(creationSpinnerTick)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		r.mu.Lock()
		if r.closed || r.phaseStop != stop || r.current.Phase != event.Phase || r.current.Status != engine.ProgressStarted {
			r.mu.Unlock()
			return
		}
		r.renderLocked(creationSpinnerFrames[r.frame%len(creationSpinnerFrames)], r.now())
		r.visible = true
		r.frame++
		r.mu.Unlock()

		select {
		case <-stop:
			return
		case <-ticks:
		}
	}
}

func (r *creationRenderer) renderLocked(frame string, now time.Time) {
	elapsed := now.Sub(r.startedAt)
	r.writeLocked([]byte(fmt.Sprintf("\r\x1b[2K%s %s — %s%s", frame, creationIdentityLabel(r.current), creationPhaseLabel(r.current), elapsedSuffix(elapsed, false))))
}

func (r *creationRenderer) renderForTest(event engine.CreationProgress, frame string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = event
	r.current.Status = engine.ProgressStarted
	r.startedAt = r.now()
	r.renderLocked(frame, r.now())
	r.visible = true
}

func (r *creationRenderer) stopPhaseLocked() {
	if r.phaseStop != nil {
		close(r.phaseStop)
		r.phaseStop = nil
	}
}

func (r *creationRenderer) flushPartialLocked() bool {
	if len(r.partial) == 0 {
		return false
	}
	if r.tty && r.visible {
		r.clearLocked()
	}
	r.writeLocked(r.partial)
	r.partial = r.partial[:0]
	return true
}

func (r *creationRenderer) clearLocked() {
	r.writeLocked([]byte("\r\x1b[2K"))
	r.visible = false
}

func (r *creationRenderer) writeLocked(p []byte) {
	if r.writeErr != nil || len(p) == 0 {
		return
	}
	_, err := r.out.Write(p)
	if err != nil {
		r.writeErr = err
	}
}

func (r *creationRenderer) eventElapsedLocked(event engine.CreationProgress) time.Duration {
	if event.Elapsed > 0 {
		return event.Elapsed
	}
	if r.startedAt.IsZero() {
		return 0
	}
	return r.now().Sub(r.startedAt)
}

func (r *creationRenderer) writeSummaryLocked(event engine.CreationProgress) {
	parts := make([]string, 0, 5)
	if len(event.Summary.Completed) > 0 {
		parts = append(parts, "completed: "+strings.Join(event.Summary.Completed, ", "))
	}
	if event.Summary.Failed != "" {
		failure := "failed: " + event.Summary.Failed
		if event.Summary.RetainedPath != "" {
			failure += " (retained at " + event.Summary.RetainedPath + ")"
		}
		parts = append(parts, failure)
	}
	if len(event.Summary.RolledBack) > 0 {
		parts = append(parts, "rolled back: "+strings.Join(event.Summary.RolledBack, ", "))
	}
	if event.Summary.RollbackError != "" {
		parts = append(parts, "rollback failed: "+event.Summary.RollbackError)
	}
	if len(event.Summary.Unstarted) > 0 {
		parts = append(parts, "not started: "+strings.Join(event.Summary.Unstarted, ", "))
	}
	if len(parts) == 0 {
		return
	}
	if r.tty && r.visible {
		r.clearLocked()
	}
	r.writeLocked([]byte(strings.Join(parts, "; ") + "\n"))
}

func creationIdentityLabel(event engine.CreationProgress) string {
	workspace := strings.TrimSpace(event.Workspace)
	if workspace == "" {
		workspace = "workspace"
	}
	if event.Current > 0 && event.Total > 0 {
		return fmt.Sprintf("[%d/%d] %s", event.Current, event.Total, workspace)
	}
	return workspace
}

func creationPhaseLabel(event engine.CreationProgress) string {
	detail := strings.TrimSpace(event.Detail)
	switch event.Phase {
	case engine.PhaseSyncBase:
		return "Syncing base"
	case engine.PhaseCopyWorkspace:
		return "Copying workspace"
	case engine.PhaseConfigureBranch:
		return detailLabel("Configuring branch", detail)
	case engine.PhaseConfigureTracking:
		return detailLabel("Configuring tracking", detail)
	case engine.PhaseCheckoutPR:
		return detailLabel("Checking out PR", detail)
	case engine.PhaseSetup:
		return detailLabel("Running setup", detail)
	case engine.PhaseRollback:
		return "Rolling back workspace"
	default:
		return "Creating workspace"
	}
}

func detailLabel(label, detail string) string {
	if detail == "" {
		return label
	}
	return label + ": " + detail
}

func elapsedSuffix(elapsed time.Duration, always bool) string {
	if !always && elapsed < time.Second {
		return ""
	}
	return fmt.Sprintf(" (%.1fs)", elapsed.Seconds())
}

func runCLIProgressPhase(ctx context.Context, event engine.CreationProgress, operation func() error) error {
	started := time.Now()
	event.Status = engine.ProgressStarted
	engine.ReportCreationProgress(ctx, event)
	err := operation()
	if err != nil && ctx.Err() != nil && !errors.Is(err, ctx.Err()) {
		err = errors.Join(err, ctx.Err())
	}
	event.Elapsed = time.Since(started)
	switch {
	case err == nil:
		event.Status = engine.ProgressCompleted
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		event.Status = engine.ProgressCancelled
	default:
		event.Status = engine.ProgressFailed
	}
	engine.ReportCreationProgress(ctx, event)
	return err
}

type spinnerStop func(err error)

func startSpinner(out io.Writer, label string) spinnerStop {
	if !isTTYWriter(out) {
		return func(error) {}
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(creationSpinnerTick)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = fmt.Fprintf(out, "\r%s %s", creationSpinnerFrames[frame%len(creationSpinnerFrames)], label)
				frame++
			}
		}
	}()

	return func(err error) {
		close(done)
		<-stopped
		if err != nil {
			_, _ = fmt.Fprintf(out, "\r! %s failed\n", label)
			return
		}
		_, _ = fmt.Fprintf(out, "\r+ %s done\n", label)
	}
}

func isTTYWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

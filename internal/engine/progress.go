package engine

import (
	"context"
	"errors"
	"time"
)

type CreationPhase string

const (
	PhaseSyncBase          CreationPhase = "sync-base"
	PhaseCopyWorkspace     CreationPhase = "copy-workspace"
	PhaseConfigureBranch   CreationPhase = "configure-branch"
	PhaseConfigureTracking CreationPhase = "configure-tracking"
	PhaseCheckoutPR        CreationPhase = "checkout-pr"
	PhaseSetup             CreationPhase = "setup"
	PhaseRollback          CreationPhase = "rollback"
	PhaseCreation          CreationPhase = "creation"
)

type CreationStatus string

const (
	ProgressStarted   CreationStatus = "started"
	ProgressCompleted CreationStatus = "completed"
	ProgressFailed    CreationStatus = "failed"
	ProgressCancelled CreationStatus = "cancelled"
)

type CreationIdentity struct {
	Workspace string
	Current   int
	Total     int
}

type CreationSummary struct {
	Completed     []string
	Failed        string
	RetainedPath  string
	RolledBack    []string
	Unstarted     []string
	RollbackError string
}

type CreationProgress struct {
	Phase     CreationPhase
	Status    CreationStatus
	Workspace string
	Current   int
	Total     int
	Detail    string
	Elapsed   time.Duration
	Summary   CreationSummary
}

type CreationReporter interface {
	Report(CreationProgress) error
	HookOutput([]byte) error
}

type creationReporterContextKey struct{}

type creationReporterContext struct {
	reporter CreationReporter
	identity CreationIdentity
}

func WithCreationReporter(ctx context.Context, reporter CreationReporter, identity CreationIdentity) context.Context {
	return context.WithValue(ctx, creationReporterContextKey{}, creationReporterContext{
		reporter: reporter,
		identity: identity,
	})
}

func withoutCreationReporter(ctx context.Context) context.Context {
	return context.WithValue(ctx, creationReporterContextKey{}, creationReporterContext{})
}

func creationReporterFromContext(ctx context.Context) (CreationReporter, CreationIdentity) {
	value, _ := ctx.Value(creationReporterContextKey{}).(creationReporterContext)
	return value.reporter, value.identity
}

func ReportCreationProgress(ctx context.Context, progress CreationProgress) {
	reporter, identity := creationReporterFromContext(ctx)
	if reporter == nil {
		return
	}
	if progress.Workspace == "" {
		progress.Workspace = identity.Workspace
	}
	if progress.Current == 0 {
		progress.Current = identity.Current
	}
	if progress.Total == 0 {
		progress.Total = identity.Total
	}
	progress.Summary = cloneCreationSummary(progress.Summary)
	_ = reporter.Report(progress)
}

func WriteCreationHookOutput(ctx context.Context, p []byte) {
	reporter, _ := creationReporterFromContext(ctx)
	if reporter == nil {
		return
	}
	_ = reporter.HookOutput(append([]byte(nil), p...))
}

func runCreationPhase(ctx context.Context, progress CreationProgress, operation func() error) error {
	progress.Status = ProgressStarted
	progress.Elapsed = 0
	ReportCreationProgress(ctx, progress)

	started := time.Now()
	err := operation()
	progress.Elapsed = time.Since(started)
	progress.Status = creationStatusForError(err)
	ReportCreationProgress(ctx, progress)
	return err
}

func creationStatusForError(err error) CreationStatus {
	switch {
	case err == nil:
		return ProgressCompleted
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ProgressCancelled
	default:
		return ProgressFailed
	}
}

func cloneCreationSummary(summary CreationSummary) CreationSummary {
	summary.Completed = append([]string(nil), summary.Completed...)
	summary.RolledBack = append([]string(nil), summary.RolledBack...)
	summary.Unstarted = append([]string(nil), summary.Unstarted...)
	return summary
}

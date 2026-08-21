package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/fs"
	"github.com/scottwater/stooges/internal/git"
	"github.com/scottwater/stooges/internal/model"
)

type WorkspaceService interface {
	Init(context.Context, model.InitOptions) (model.InitResult, error)
	Make(context.Context, model.MakeOptions) (model.MakeResult, error)
	Setup(context.Context, model.SetupOptions) (model.SetupResult, error)
	Sync(context.Context, model.SyncOptions) (model.SyncResult, error)
	Clean(context.Context, model.CleanOptions) (model.CleanResult, error)
	List(context.Context, model.ListOptions) (model.ListResult, error)
	Unlock(context.Context, model.UnlockOptions) (model.UnlockResult, error)
	Lock(context.Context, model.LockOptions) (model.LockResult, error)
	Rebase(context.Context, model.RebaseOptions) (model.RebaseResult, error)
	Doctor(context.Context, model.DoctorOptions) (model.DoctorReport, error)
	Enabled(context.Context, model.EnabledOptions) (model.EnabledResult, error)
	Undo(context.Context, model.UndoOptions) (model.UndoResult, error)
	Trash(context.Context, model.TrashOptions) (model.TrashResult, error)
}

type PermissionOps interface {
	UnlockWritable(root string) error
	LockReadOnly(root string) error
	CountSymlinks(root string) (int, error)
}

type Dependencies struct {
	CWD            func() (string, error)
	Chdir          func(string) error
	Cloner         fs.Cloner
	Perms          PermissionOps
	Git            gitops.Ops
	Preflight      *PreflightChecker
	Resolver       *RepoResolver
	BranchDetector *BranchDetector
	RemoveAll      func(string) error
}

type Service struct {
	cwd            func() (string, error)
	chdir          func(string) error
	cloner         fs.Cloner
	perms          PermissionOps
	git            gitops.Ops
	preflight      *PreflightChecker
	resolver       *RepoResolver
	branchDetector *BranchDetector
	removeAll      func(string) error
}

type CurrentWorkspace struct {
	Name          string
	Path          string
	WorkspaceRoot string
}

type gitignoreInspector interface {
	IgnoredPatternsWithMatches(context.Context, string) ([]string, error)
}

func NewService() *Service {
	cloner := fs.NewSystemCloner()
	gitOps := gitops.NewSystemOps()
	perms := fs.NewPermissionManager()
	return NewServiceWithDeps(Dependencies{
		CWD:            os.Getwd,
		Chdir:          os.Chdir,
		Cloner:         cloner,
		Perms:          perms,
		Git:            gitOps,
		Preflight:      NewPreflightChecker(cloner),
		Resolver:       NewRepoResolver(gitOps),
		BranchDetector: NewBranchDetector(gitOps),
	})
}

func NewServiceWithDeps(deps Dependencies) *Service {
	if deps.CWD == nil {
		deps.CWD = os.Getwd
	}
	if deps.Chdir == nil {
		deps.Chdir = os.Chdir
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	return &Service{
		cwd:            deps.CWD,
		chdir:          deps.Chdir,
		cloner:         deps.Cloner,
		perms:          deps.Perms,
		git:            deps.Git,
		preflight:      deps.Preflight,
		resolver:       deps.Resolver,
		branchDetector: deps.BranchDetector,
		removeAll:      deps.RemoveAll,
	}
}

func canonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("resolve absolute path for %q", path), err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Clean(absPath), nil
		}
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("resolve symlinks for %q", path), err)
	}
	return filepath.Clean(resolvedPath), nil
}

func canonicalPathsEqual(left, right string) (bool, error) {
	canonicalLeft, err := canonicalPath(left)
	if err != nil {
		return false, err
	}
	canonicalRight, err := canonicalPath(right)
	if err != nil {
		return false, err
	}
	return canonicalLeft == canonicalRight, nil
}

func (s *Service) ResolveCurrentWorkspace(ctx context.Context) (CurrentWorkspace, error) {
	cwd, err := s.cwd()
	if err != nil {
		return CurrentWorkspace{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return CurrentWorkspace{}, err
	}
	atWorkspaceRoot, err := canonicalPathsEqual(cwd, workspaceRoot)
	if err != nil {
		return CurrentWorkspace{}, err
	}
	if atWorkspaceRoot {
		return CurrentWorkspace{}, apperrors.New(apperrors.KindInvalidInput, "fork must run from inside a managed workspace, not the workspace root")
	}
	repoRoot, err := s.git.TopLevel(ctx, cwd)
	if err != nil {
		return CurrentWorkspace{}, apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("resolve current workspace git root from %q", cwd), err)
	}
	isBaseRepo, err := canonicalPathsEqual(repoRoot, layout.BaseRepoPath)
	if err != nil {
		return CurrentWorkspace{}, err
	}
	if isBaseRepo {
		return CurrentWorkspace{}, apperrors.New(apperrors.KindInvalidInput, "fork cannot run from the base repo; run it from inside a managed workspace")
	}
	for _, workspace := range layout.ManagedWorkspaces {
		workspacePath := filepath.Join(layout.WorkspaceRoot, workspace)
		matchesWorkspace, compareErr := canonicalPathsEqual(repoRoot, workspacePath)
		if compareErr != nil {
			return CurrentWorkspace{}, compareErr
		}
		if matchesWorkspace {
			return CurrentWorkspace{Name: workspace, Path: workspacePath, WorkspaceRoot: workspaceRoot}, nil
		}
	}
	return CurrentWorkspace{}, apperrors.New(apperrors.KindInvalidInput, "fork must run from inside a managed workspace")
}

func (s *Service) PreviewInitBranch(ctx context.Context) (string, error) {
	cwd, err := s.cwd()
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	repoRoot, err := s.git.TopLevel(ctx, cwd)
	if err != nil {
		return "", apperrors.New(apperrors.KindInvalidInput, "init must run from inside a git repository")
	}
	const branch = "main"
	exists, err := s.git.BranchExists(ctx, repoRoot, branch)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", apperrors.New(apperrors.KindInvalidInput, `default branch "main" not found; pass --main-branch master if needed`)
	}
	return branch, nil
}

func (s *Service) Doctor(ctx context.Context, opts model.DoctorOptions) (model.DoctorReport, error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.DoctorReport{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot := workspaceRootFromCWD(cwd)
	report := model.DoctorReport{Workspace: workspaceRoot}
	preflight := s.preflight.Report(ctx, PreflightOptions{WorkspacePath: workspaceRoot})
	report.Platform = preflight.Platform

	report.Checks = append(report.Checks,
		model.DoctorCheck{Name: "git", OK: preflight.GitAvailable, Message: ternary(preflight.GitAvailable, "git found", "git not found in PATH")},
		model.DoctorCheck{Name: "cow_clone", OK: preflight.COWCloneSupported, Message: ternary(preflight.COWCloneSupported, "copy-on-write clone supported", "copy-on-write clone unavailable")},
		model.DoctorCheck{Name: "workspace", OK: preflight.WorkspaceValid, Message: ternary(preflight.WorkspaceValid, "workspace path is valid", "workspace path missing")},
	)

	layoutCandidate := layoutFromRoot(workspaceRoot)
	layoutConfigured := isGitRepoPath(layoutCandidate.BaseRepoPath)
	repoResolutionFailed := false
	var layout WorkspaceLayout
	var layoutErr error
	if layoutConfigured {
		layout, layoutErr = loadWorkspaceLayoutWithCleanup(workspaceRoot)
	}

	if !layoutConfigured {
		if strings.TrimSpace(opts.Repo) != "" {
			repoResolutionFailed = true
			report.Checks = append(report.Checks, model.DoctorCheck{
				Name:    "repo_resolution",
				OK:      false,
				Message: "workspace not configured (missing ./.stooges)",
			})
			report.Suggestions = append(report.Suggestions, "run `stooges init` before passing --repo")
		} else {
			report.Checks = append(report.Checks, model.DoctorCheck{
				Name:    "repo_resolution",
				OK:      true,
				Message: "workspace not configured yet (missing ./.stooges)",
			})
			report.Suggestions = append(report.Suggestions, "run `stooges init` from your repo root to bootstrap .stooges workspace")
		}
	} else if layoutErr != nil {
		repoResolutionFailed = true
		report.Checks = append(report.Checks, model.DoctorCheck{
			Name:    "repo_resolution",
			OK:      false,
			Message: fmt.Sprintf("invalid .stooges workspace: %v", layoutErr),
		})
		report.Suggestions = append(report.Suggestions, "workspace metadata is invalid; run `stooges undo` then `stooges init` to rebuild")
	} else {
		target, resolveErr := resolveBaseRepo(layout, opts.Repo)
		if resolveErr != nil {
			repoResolutionFailed = true
			report.Checks = append(report.Checks, model.DoctorCheck{Name: "repo_resolution", OK: false, Message: resolveErr.Error()})
		} else {
			report.Checks = append(report.Checks, model.DoctorCheck{Name: "repo_resolution", OK: true, Message: fmt.Sprintf("resolved base repo %s", target)})
		}
	}

	if inspector, ok := s.git.(gitignoreInspector); ok {
		inspectRepo := ""
		if layoutConfigured && layoutErr == nil {
			inspectRepo = layout.BaseRepoPath
		} else if repoRoot, topErr := s.git.TopLevel(ctx, cwd); topErr == nil && isGitRepoPath(repoRoot) {
			inspectRepo = repoRoot
		}
		if inspectRepo != "" {
			patterns, inspectErr := inspector.IgnoredPatternsWithMatches(ctx, inspectRepo)
			if inspectErr != nil {
				report.Checks = append(report.Checks, model.DoctorCheck{
					Name:    "gitignore_matches",
					OK:      false,
					Message: fmt.Sprintf("warning: could not inspect .gitignore: %v", inspectErr),
				})
			} else if len(patterns) == 0 {
				report.Checks = append(report.Checks, model.DoctorCheck{
					Name:    "gitignore_matches",
					OK:      true,
					Message: "no active .gitignore patterns matched existing files",
				})
			} else {
				report.Checks = append(report.Checks, model.DoctorCheck{
					Name:    "gitignore_matches",
					OK:      true,
					Message: fmt.Sprintf("warning: active .gitignore patterns matched existing files: %s", strings.Join(patterns, ", ")),
				})
				report.Suggestions = append(report.Suggestions, "ignored paths are still copied by init/add clone operations; review these patterns before cloning")
			}
		}
	}

	if report.HasCriticalPreflightFailure() || (strings.TrimSpace(opts.Repo) != "" && repoResolutionFailed) {
		return report, apperrors.New(apperrors.KindPreflightFailure, "doctor found failing checks")
	}
	return report, nil
}

func (s *Service) Enabled(context.Context, model.EnabledOptions) (model.EnabledResult, error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.EnabledResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot := workspaceRootFromCWD(cwd)
	layoutCandidate := layoutFromRoot(workspaceRoot)
	layout, err := loadWorkspaceLayout(workspaceRoot)
	if err != nil {
		return model.EnabledResult{
			Enabled:       false,
			WorkspaceRoot: workspaceRoot,
			BaseRepoPath:  layoutCandidate.BaseRepoPath,
			MetadataPath:  layoutCandidate.MetadataPath,
			Reason:        err.Error(),
		}, nil
	}
	return model.EnabledResult{
		Enabled:       true,
		WorkspaceRoot: workspaceRoot,
		BaseRepoPath:  layout.BaseRepoPath,
		MetadataPath:  layout.MetadataPath,
	}, nil
}

type creationRollbackError struct {
	err        error
	rolledBack []string
}

func (e *creationRollbackError) Error() string { return e.err.Error() }
func (e *creationRollbackError) Unwrap() error { return e.err }

func (s *Service) rollbackCreatedWorkspaces(workspaceRoot string, created []string) error {
	rolledBack := make([]string, 0, len(created))
	for i := len(created) - 1; i >= 0; i-- {
		target := filepath.Join(workspaceRoot, created[i])
		if err := s.removeAll(target); err != nil {
			return &creationRollbackError{
				err:        apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("rollback remove workspace %s", created[i]), err),
				rolledBack: rolledBack,
			}
		}
		rolledBack = append([]string{created[i]}, rolledBack...)
	}
	return nil
}

func rolledBackWorkspaces(err error, requested []string) []string {
	if err == nil {
		return append([]string(nil), requested...)
	}
	var rollbackErr *creationRollbackError
	if errors.As(err, &rollbackErr) {
		return append([]string(nil), rollbackErr.rolledBack...)
	}
	return nil
}

func (s *Service) runCreationRollback(ctx context.Context, workspaceRoot string, created []string) error {
	if len(created) == 0 {
		return nil
	}
	_, identity := creationReporterFromContext(ctx)
	return runCreationPhase(ctx, CreationProgress{
		Phase:     PhaseRollback,
		Workspace: created[len(created)-1],
		Current:   len(created),
		Total:     identity.Total,
	}, func() error {
		return s.rollbackCreatedWorkspaces(workspaceRoot, created)
	})
}

func resolveTargetBranchForWorkspace(opts model.MakeOptions, workspace string, createdCount int) (string, bool, error) {
	if opts.BranchAuto {
		return workspace, true, nil
	}
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		return "", false, nil
	}
	if createdCount > 1 {
		return "", false, apperrors.New(apperrors.KindInvalidInput, "branch override requires explicit workspace or exactly one created workspace")
	}
	return branch, true, nil
}

func resolveTrackBranches(opts model.MakeOptions) (remoteBranch string, localBranch string, enabled bool, err error) {
	remote := strings.TrimSpace(opts.Track)
	if remote == "" {
		return "", "", false, nil
	}
	if opts.BranchAuto {
		return "", "", false, apperrors.New(apperrors.KindInvalidInput, "--track cannot be combined with auto branch naming (-b without value)")
	}
	local := strings.TrimSpace(opts.Branch)
	if local == "" {
		local = remote
	}
	return remote, local, true, nil
}

func (s *Service) checkoutOrCreateBranch(ctx context.Context, repo, branch string) error {
	exists, err := s.git.BranchExists(ctx, repo, branch)
	if err != nil {
		return err
	}
	if exists {
		return s.git.Switch(ctx, repo, branch)
	}
	return s.git.SwitchCreate(ctx, repo, branch)
}

func (s *Service) createNewLocalBranch(ctx context.Context, repo, branch string) error {
	exists, err := s.git.LocalBranchExists(ctx, repo, branch)
	if err != nil {
		return err
	}
	if exists {
		return apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("local branch %q already exists; choose a different branch name", branch))
	}
	return s.git.SwitchCreate(ctx, repo, branch)
}

func (s *Service) checkoutTrackingBranch(ctx context.Context, repo, remoteBranch, localBranch string) error {
	if err := s.git.Fetch(ctx, repo); err != nil {
		return err
	}
	remoteExists, err := s.git.RemoteBranchExists(ctx, repo, remoteBranch)
	if err != nil {
		return err
	}
	if !remoteExists {
		return apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("remote branch origin/%s does not exist", remoteBranch))
	}
	localExists, err := s.git.LocalBranchExists(ctx, repo, localBranch)
	if err != nil {
		return err
	}
	if localExists {
		if localBranch != remoteBranch {
			return apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("local branch %q already exists; choose a different --branch name", localBranch))
		}
		return apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("local branch %q already exists; refusing to reuse it for tracking; choose a different branch name", localBranch))
	}
	return s.git.SwitchTrack(ctx, repo, localBranch, remoteBranch)
}

func hookSourceName(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return baseRepoAlias
	}
	return trimmed
}

func (s *Service) setupFailureResult(ctx context.Context, workspaceRoot string, created []string, setupErr error, rollback bool) (model.MakeResult, error, error) {
	if rollback {
		rollbackErr := s.runCreationRollback(ctx, workspaceRoot, created)
		if rollbackErr != nil {
			return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "setup failed and rollback failed", errors.Join(setupErr, rollbackErr)), rollbackErr
		}
		return model.MakeResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "setup failed; rolled back created workspace", setupErr), nil
	}
	last := ""
	if len(created) > 0 {
		last = filepath.Join(workspaceRoot, created[len(created)-1])
	}
	message := "setup failed; workspace left in place for inspection/cleanup"
	if last != "" {
		message = fmt.Sprintf("setup failed; workspace left at %s for inspection/cleanup", last)
	}
	return model.MakeResult{Created: created, WorkspaceRoot: workspaceRoot}, apperrors.Wrap(apperrors.KindFilesystemFailure, message, setupErr), nil
}

func setupFailureError(workspaceRoot, workspace string, setupErr error, rollback bool) error {
	message := "setup failed; workspace left in place for inspection/cleanup"
	if rollback {
		message = "setup failed; rolled back created workspace"
	} else if workspace != "" {
		message = fmt.Sprintf("setup failed; workspace left at %s for inspection/cleanup", filepath.Join(workspaceRoot, workspace))
	}
	return apperrors.Wrap(apperrors.KindFilesystemFailure, message, setupErr)
}

func (s *Service) runSetupForWorkspace(ctx context.Context, cwd string, layout WorkspaceLayout, source, workspace, branch string) error {
	if strings.TrimSpace(layout.SetupScript) == "" {
		return nil
	}
	workspacePath := filepath.Join(layout.WorkspaceRoot, workspace)
	return runCreationPhase(ctx, CreationProgress{Phase: PhaseSetup, Workspace: workspace, Detail: layout.SetupScript}, func() error {
		hookBranch := strings.TrimSpace(branch)
		if hookBranch == "" {
			detected, err := s.git.BranchName(ctx, workspacePath)
			if err != nil {
				return contextualOperationError(ctx, apperrors.Wrap(apperrors.KindGitFailure, fmt.Sprintf("detect branch for setup in %s", workspacePath), err))
			}
			hookBranch = strings.TrimSpace(detected)
		}
		return runWorkspaceScript(ctx, layout.SetupScript, setupHookEnv(cwd, layout.WorkspaceRoot, hookSourceName(source), workspace, hookBranch))
	})
}

func contextualOperationError(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil || errors.Is(err, ctx.Err()) {
		return err
	}
	return errors.Join(err, ctx.Err())
}

func (s *Service) RollbackWorkspaceCreation(ctx context.Context, workspace string) error {
	workspace = strings.TrimSpace(workspace)
	if err := validateWorkspaceEntryName(workspace); err != nil {
		return err
	}
	cwd, err := s.cwd()
	if err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot := workspaceRootFromCWD(cwd)
	if strings.TrimSpace(workspaceRoot) == "" {
		return apperrors.New(apperrors.KindInvalidInput, "workspace path is empty")
	}
	layout, err := loadWorkspaceLayout(workspaceRoot)
	if err != nil {
		return err
	}
	return runCreationPhase(ctx, CreationProgress{Phase: PhaseRollback, Workspace: workspace, Detail: filepath.Join(workspaceRoot, workspace)}, func() error {
		removeErr := s.rollbackCreatedWorkspaces(workspaceRoot, []string{workspace})
		layout.ManagedWorkspaces = removeManagedWorkspace(layout.ManagedWorkspaces, workspace)
		metadataErr := writeWorkspaceMetadata(layout)
		if removeErr != nil || metadataErr != nil {
			return apperrors.Wrap(apperrors.KindRollbackFailure, fmt.Sprintf("rollback workspace %s", workspace), errors.Join(removeErr, metadataErr))
		}
		return nil
	})
}

func (s *Service) Setup(ctx context.Context, opts model.SetupOptions) (model.SetupResult, error) {
	workspace := strings.TrimSpace(opts.Workspace)
	if err := validateWorkspaceEntryName(workspace); err != nil {
		return model.SetupResult{}, err
	}
	cwd, err := s.cwd()
	if err != nil {
		return model.SetupResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.SetupResult{}, err
	}
	if !slices.Contains(layout.ManagedWorkspaces, workspace) {
		return model.SetupResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("workspace %q is not managed by stooges", workspace))
	}
	workspacePath := filepath.Join(workspaceRoot, workspace)
	if !pathExists(workspacePath) {
		return model.SetupResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("workspace %q is missing", workspace))
	}

	result := model.SetupResult{WorkspaceRoot: workspaceRoot, Workspace: workspace, WorkspacePath: workspacePath}
	if err := s.runSetupForWorkspace(ctx, cwd, layout, opts.Source, workspace, opts.Branch); err != nil {
		if opts.RollbackOnSetupFailure {
			rollbackErr := runCreationPhase(ctx, CreationProgress{Phase: PhaseRollback, Workspace: workspace, Detail: workspacePath}, func() error {
				removeErr := s.rollbackCreatedWorkspaces(workspaceRoot, []string{workspace})
				layout.ManagedWorkspaces = removeManagedWorkspace(layout.ManagedWorkspaces, workspace)
				metadataErr := writeWorkspaceMetadata(layout)
				if removeErr != nil || metadataErr != nil {
					return errors.Join(removeErr, metadataErr)
				}
				return nil
			})
			if rollbackErr != nil {
				return model.SetupResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "setup failed and rollback failed", errors.Join(err, rollbackErr))
			}
		}
		return result, setupFailureError(workspaceRoot, workspace, err, opts.RollbackOnSetupFailure)
	}
	return result, nil
}

func (s *Service) Make(ctx context.Context, opts model.MakeOptions) (result model.MakeResult, err error) {
	started := time.Now()
	summary := CreationSummary{}
	terminal := CreationProgress{Phase: PhaseCreation, Workspace: strings.TrimSpace(opts.Agent)}
	defer func() {
		if err == nil && len(summary.Completed) == 0 {
			summary.Completed = append([]string(nil), result.Created...)
		}
		terminal.Status = creationStatusForError(err)
		terminal.Elapsed = time.Since(started)
		terminal.Summary = summary
		ReportCreationProgress(ctx, terminal)
	}()

	cwd, err := s.cwd()
	if err != nil {
		return model.MakeResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.MakeResult{}, err
	}

	sourcePath, err := resolveSourceRepo(layout, opts.Source)
	if err != nil {
		return model.MakeResult{}, err
	}
	if _, err := s.preflight.EnsureMutating(ctx, PreflightOptions{WorkspacePath: workspaceRoot, RequireSourceGit: true, SourceRepoPath: sourcePath}); err != nil {
		return model.MakeResult{}, err
	}
	trackRemote, trackLocal, trackingEnabled, err := resolveTrackBranches(opts)
	if err != nil {
		return model.MakeResult{}, err
	}
	if trackingEnabled && strings.TrimSpace(opts.Agent) == "" {
		return model.MakeResult{}, apperrors.New(apperrors.KindInvalidInput, "--track requires an explicit workspace name")
	}

	if strings.TrimSpace(opts.Agent) != "" {
		agent := strings.TrimSpace(opts.Agent)
		if err := validateWorkspaceEntryName(agent); err != nil {
			return model.MakeResult{}, err
		}
		targetBranch, shouldSwitchBranch, err := resolveTargetBranchForWorkspace(opts, agent, 1)
		if err != nil {
			return model.MakeResult{}, err
		}
		dst := filepath.Join(workspaceRoot, agent)
		if pathExists(dst) {
			return model.MakeResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("target already exists: %s (overwrite not allowed)", agent))
		}
		cloned := false
		copyErr := runCreationPhase(ctx, CreationProgress{Phase: PhaseCopyWorkspace, Workspace: agent}, func() error {
			if cloneErr := s.cloner.CloneRepo(ctx, sourcePath, dst); cloneErr != nil {
				return contextualOperationError(ctx, cloneErr)
			}
			cloned = true
			return s.perms.UnlockWritable(dst)
		})
		if copyErr != nil {
			summary.Failed = agent
			if !cloned {
				if pathExists(dst) {
					summary.RetainedPath = dst
				}
				return model.MakeResult{}, copyErr
			}
			rollbackTargets := []string{agent}
			rollbackErr := s.runCreationRollback(ctx, workspaceRoot, rollbackTargets)
			summary.RolledBack = rolledBackWorkspaces(rollbackErr, rollbackTargets)
			if rollbackErr != nil {
				summary.RollbackError = rollbackErr.Error()
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(copyErr, rollbackErr))
			}
			return model.MakeResult{}, copyErr
		}
		if trackingEnabled {
			trackErr := runCreationPhase(ctx, CreationProgress{Phase: PhaseConfigureTracking, Workspace: agent, Detail: trackRemote}, func() error {
				return contextualOperationError(ctx, s.checkoutTrackingBranch(ctx, dst, trackRemote, trackLocal))
			})
			if trackErr != nil {
				summary.Failed = agent
				rollbackTargets := []string{agent}
				rollbackErr := s.runCreationRollback(ctx, workspaceRoot, rollbackTargets)
				summary.RolledBack = rolledBackWorkspaces(rollbackErr, rollbackTargets)
				if rollbackErr != nil {
					summary.RollbackError = rollbackErr.Error()
					return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(trackErr, rollbackErr))
				}
				return model.MakeResult{}, trackErr
			}
		} else if shouldSwitchBranch {
			branchErr := runCreationPhase(ctx, CreationProgress{Phase: PhaseConfigureBranch, Workspace: agent, Detail: targetBranch}, func() error {
				if opts.RequireNewBranch {
					return contextualOperationError(ctx, s.createNewLocalBranch(ctx, dst, targetBranch))
				}
				return contextualOperationError(ctx, s.checkoutOrCreateBranch(ctx, dst, targetBranch))
			})
			if branchErr != nil {
				summary.Failed = agent
				rollbackTargets := []string{agent}
				rollbackErr := s.runCreationRollback(ctx, workspaceRoot, rollbackTargets)
				summary.RolledBack = rolledBackWorkspaces(rollbackErr, rollbackTargets)
				if rollbackErr != nil {
					summary.RollbackError = rollbackErr.Error()
					return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(branchErr, rollbackErr))
				}
				return model.MakeResult{}, branchErr
			}
		}
		hookBranch := targetBranch
		if trackingEnabled {
			hookBranch = trackLocal
		}
		if !opts.NoSetup {
			if setupErr := s.runSetupForWorkspace(ctx, cwd, layout, opts.Source, agent, hookBranch); setupErr != nil {
				created := []string{agent}
				summary.Failed = agent
				if !opts.RollbackOnSetupFailure {
					summary.RetainedPath = dst
					layout.ManagedWorkspaces = appendManagedWorkspaces(layout.ManagedWorkspaces, created...)
					if metadataErr := writeWorkspaceMetadata(layout); metadataErr != nil {
						return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "setup failed and metadata update failed", errors.Join(setupErr, metadataErr))
					}
				}
				failureResult, failureErr, rollbackErr := s.setupFailureResult(ctx, workspaceRoot, created, setupErr, opts.RollbackOnSetupFailure)
				if opts.RollbackOnSetupFailure {
					summary.RolledBack = rolledBackWorkspaces(rollbackErr, created)
					if rollbackErr != nil {
						summary.RollbackError = rollbackErr.Error()
					}
				}
				return failureResult, failureErr
			}
		}
		layout.ManagedWorkspaces = appendManagedWorkspaces(layout.ManagedWorkspaces, agent)
		if metadataErr := writeWorkspaceMetadata(layout); metadataErr != nil {
			summary.Failed = agent
			rollbackTargets := []string{agent}
			rollbackErr := s.runCreationRollback(ctx, workspaceRoot, rollbackTargets)
			summary.RolledBack = rolledBackWorkspaces(rollbackErr, rollbackTargets)
			if rollbackErr != nil {
				summary.RollbackError = rollbackErr.Error()
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(metadataErr, rollbackErr))
			}
			return model.MakeResult{}, metadataErr
		}
		summary.Completed = []string{agent}
		return model.MakeResult{Created: []string{agent}, WorkspaceRoot: workspaceRoot}, nil
	}

	agents := model.NormalizeAgents(opts.Agents)
	missing := make([]string, 0, len(agents))
	for _, agent := range agents {
		if err := validateWorkspaceEntryName(agent); err != nil {
			return model.MakeResult{}, err
		}
		if !pathExists(filepath.Join(workspaceRoot, agent)) {
			missing = append(missing, agent)
		}
	}
	if len(missing) == 0 {
		return model.MakeResult{Guidance: "all default agents already exist; pass an explicit agent name to create another workspace", WorkspaceRoot: workspaceRoot}, nil
	}

	created := make([]string, 0, len(missing))
	targetBranch, shouldSwitchBranch, err := resolveTargetBranchForWorkspace(opts, "", len(missing))
	if err != nil {
		return model.MakeResult{}, err
	}
	for i, agent := range missing {
		reporter, _ := creationReporterFromContext(ctx)
		workspaceCtx := ctx
		if reporter != nil {
			workspaceCtx = WithCreationReporter(ctx, reporter, CreationIdentity{Workspace: agent, Current: i + 1, Total: len(missing)})
		}
		terminal.Workspace = agent
		terminal.Current = i + 1
		terminal.Total = len(missing)
		dst := filepath.Join(workspaceRoot, agent)
		cloned := false
		copyErr := runCreationPhase(workspaceCtx, CreationProgress{Phase: PhaseCopyWorkspace}, func() error {
			if cloneErr := s.cloner.CloneRepo(workspaceCtx, sourcePath, dst); cloneErr != nil {
				return contextualOperationError(workspaceCtx, cloneErr)
			}
			cloned = true
			return s.perms.UnlockWritable(dst)
		})
		if copyErr != nil {
			summary.Completed = append([]string(nil), created...)
			summary.Failed = agent
			summary.Unstarted = append([]string(nil), missing[i+1:]...)
			rollbackTargets := append([]string(nil), created...)
			if cloned {
				rollbackTargets = append(rollbackTargets, agent)
			} else if pathExists(dst) {
				summary.RetainedPath = dst
			}
			rollbackErr := s.runCreationRollback(workspaceCtx, workspaceRoot, rollbackTargets)
			summary.RolledBack = rolledBackWorkspaces(rollbackErr, rollbackTargets)
			if rollbackErr != nil {
				summary.RollbackError = rollbackErr.Error()
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(copyErr, rollbackErr))
			}
			return model.MakeResult{}, copyErr
		}
		if shouldSwitchBranch {
			branch := targetBranch
			if opts.BranchAuto {
				branch = agent
			}
			branchErr := runCreationPhase(workspaceCtx, CreationProgress{Phase: PhaseConfigureBranch, Detail: branch}, func() error {
				return contextualOperationError(workspaceCtx, s.checkoutOrCreateBranch(workspaceCtx, dst, branch))
			})
			if branchErr != nil {
				summary.Completed = append([]string(nil), created...)
				summary.Failed = agent
				summary.Unstarted = append([]string(nil), missing[i+1:]...)
				rollbackTargets := append(append([]string(nil), created...), agent)
				rollbackErr := s.runCreationRollback(workspaceCtx, workspaceRoot, rollbackTargets)
				summary.RolledBack = rolledBackWorkspaces(rollbackErr, rollbackTargets)
				if rollbackErr != nil {
					summary.RollbackError = rollbackErr.Error()
					return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(branchErr, rollbackErr))
				}
				return model.MakeResult{}, branchErr
			}
		}
		created = append(created, agent)
		hookBranch := targetBranch
		if opts.BranchAuto {
			hookBranch = agent
		}
		if !opts.NoSetup {
			if setupErr := s.runSetupForWorkspace(workspaceCtx, cwd, layout, opts.Source, agent, hookBranch); setupErr != nil {
				summary.Completed = append([]string(nil), created[:len(created)-1]...)
				summary.Failed = agent
				summary.Unstarted = append([]string(nil), missing[i+1:]...)
				if !opts.RollbackOnSetupFailure {
					summary.RetainedPath = dst
					layout.ManagedWorkspaces = appendManagedWorkspaces(layout.ManagedWorkspaces, created...)
					if metadataErr := writeWorkspaceMetadata(layout); metadataErr != nil {
						return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "setup failed and metadata update failed", errors.Join(setupErr, metadataErr))
					}
				}
				failureResult, failureErr, rollbackErr := s.setupFailureResult(workspaceCtx, workspaceRoot, created, setupErr, opts.RollbackOnSetupFailure)
				if opts.RollbackOnSetupFailure {
					summary.RolledBack = rolledBackWorkspaces(rollbackErr, created)
					if rollbackErr != nil {
						summary.RollbackError = rollbackErr.Error()
					}
				}
				return failureResult, failureErr
			}
		}
		summary.Completed = append([]string(nil), created...)
	}
	layout.ManagedWorkspaces = appendManagedWorkspaces(layout.ManagedWorkspaces, created...)
	if metadataErr := writeWorkspaceMetadata(layout); metadataErr != nil {
		rollbackErr := s.runCreationRollback(ctx, workspaceRoot, created)
		summary.RolledBack = rolledBackWorkspaces(rollbackErr, created)
		if rollbackErr != nil {
			summary.RollbackError = rollbackErr.Error()
			return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(metadataErr, rollbackErr))
		}
		return model.MakeResult{}, metadataErr
	}

	return model.MakeResult{Created: created, WorkspaceRoot: workspaceRoot}, nil
}

func (s *Service) Sync(ctx context.Context, opts model.SyncOptions) (model.SyncResult, error) {
	reporter, _ := creationReporterFromContext(ctx)
	if reporter == nil {
		return s.syncRepo(ctx, opts.Repo, false)
	}
	var result model.SyncResult
	err := runCreationPhase(ctx, CreationProgress{Phase: PhaseSyncBase, Detail: strings.TrimSpace(opts.Repo)}, func() error {
		var syncErr error
		result, syncErr = s.syncRepo(ctx, opts.Repo, false)
		return contextualOperationError(ctx, syncErr)
	})
	return result, err
}

func (s *Service) Clean(ctx context.Context, opts model.CleanOptions) (model.CleanResult, error) {
	res, err := s.syncRepo(ctx, opts.Repo, true)
	if err != nil {
		return model.CleanResult{}, err
	}
	return model.CleanResult{RepoPath: res.RepoPath, SymlinkCount: res.SymlinkCount}, nil
}

func (s *Service) syncRepo(ctx context.Context, explicitRepo string, prune bool) (res model.SyncResult, err error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.SyncResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.SyncResult{}, err
	}
	repo, err := resolveBaseRepo(layout, explicitRepo)
	if err != nil {
		return model.SyncResult{}, err
	}

	if _, err := s.preflight.EnsureMutating(ctx, PreflightOptions{WorkspacePath: workspaceRoot}); err != nil {
		return model.SyncResult{}, err
	}

	unlocked := false
	defer func() {
		if !unlocked {
			return
		}
		lockErr := s.perms.LockReadOnly(repo)
		if lockErr == nil {
			return
		}
		if err != nil {
			err = apperrors.Wrap(apperrors.KindRollbackFailure, "sync failed and relock failed", errors.Join(err, lockErr))
			return
		}
		err = lockErr
	}()

	if err = s.perms.UnlockWritable(repo); err != nil {
		return model.SyncResult{}, err
	}
	unlocked = true
	if prune {
		if err = s.git.FetchPrune(ctx, repo); err != nil {
			return model.SyncResult{}, err
		}
	} else {
		if err = s.git.Fetch(ctx, repo); err != nil {
			return model.SyncResult{}, err
		}
	}
	if err = s.git.Switch(ctx, repo, layout.MainBranch); err != nil {
		return model.SyncResult{}, err
	}
	if err = s.git.PullFFOnly(ctx, repo); err != nil {
		return model.SyncResult{}, err
	}
	if err = s.perms.LockReadOnly(repo); err != nil {
		return model.SyncResult{}, err
	}
	unlocked = false

	links, err := s.perms.CountSymlinks(repo)
	if err != nil {
		return model.SyncResult{}, err
	}
	return model.SyncResult{RepoPath: repo, SymlinkCount: links}, nil
}

func (s *Service) Unlock(ctx context.Context, opts model.UnlockOptions) (model.UnlockResult, error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.UnlockResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.UnlockResult{}, err
	}
	repo, err := resolveBaseRepo(layout, opts.Repo)
	if err != nil {
		return model.UnlockResult{}, err
	}
	if _, err := s.preflight.EnsureMutating(ctx, PreflightOptions{WorkspacePath: workspaceRoot}); err != nil {
		return model.UnlockResult{}, err
	}
	if err := s.perms.UnlockWritable(repo); err != nil {
		return model.UnlockResult{}, err
	}
	return model.UnlockResult{RepoPath: repo}, nil
}

func (s *Service) Lock(ctx context.Context, opts model.LockOptions) (model.LockResult, error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.LockResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	workspaceRoot, layout, err := resolveWorkspaceAndLayout(cwd)
	if err != nil {
		return model.LockResult{}, err
	}
	repo, err := resolveBaseRepo(layout, opts.Repo)
	if err != nil {
		return model.LockResult{}, err
	}
	if _, err := s.preflight.EnsureMutating(ctx, PreflightOptions{WorkspacePath: workspaceRoot}); err != nil {
		return model.LockResult{}, err
	}
	if err := s.perms.LockReadOnly(repo); err != nil {
		return model.LockResult{}, err
	}
	return model.LockResult{RepoPath: repo}, nil
}

func (s *Service) Init(ctx context.Context, opts model.InitOptions) (res model.InitResult, err error) {
	cwd, err := s.cwd()
	if err != nil {
		return model.InitResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve current working directory", err)
	}
	repoRoot, err := s.git.TopLevel(ctx, cwd)
	if err != nil {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, "init must run from inside a git repository")
	}
	workspaceRoot := repoRoot
	layout := layoutFromRoot(workspaceRoot)
	if pathExists(layout.BaseRepoPath) {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, "init aborted: .stooges already exists")
	}

	mainBranch := strings.TrimSpace(opts.MainBranch)
	if mainBranch == "" {
		mainBranch = "main"
	}
	if mainBranch != "main" && mainBranch != "master" {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, `unsupported main branch; use "main" (default) or pass --main-branch master`)
	}
	branchExists, err := s.git.BranchExists(ctx, repoRoot, mainBranch)
	if err != nil {
		return model.InitResult{}, err
	}
	if !branchExists {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("branch %q does not exist in repo; pass --main-branch with a valid branch", mainBranch))
	}
	currentBranch, err := s.git.CurrentBranch(ctx, repoRoot)
	if err != nil {
		return model.InitResult{}, err
	}
	if strings.TrimSpace(currentBranch) != mainBranch {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("init requires the selected base branch %q to be checked out before creating the hidden, locked base repo in .stooges; currently on %q", mainBranch, strings.TrimSpace(currentBranch)))
	}
	layout.MainBranch = mainBranch

	workspaces := model.NormalizeAgents(opts.Agents)
	for _, workspace := range workspaces {
		if err := validateWorkspaceEntryName(workspace); err != nil {
			return model.InitResult{}, err
		}
		if pathExists(filepath.Join(workspaceRoot, workspace)) {
			return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, fmt.Sprintf("init aborted: target already exists: %s", workspace))
		}
	}

	if _, err := s.preflight.EnsureMutating(ctx, PreflightOptions{WorkspacePath: workspaceRoot, RequireSourceGit: true, SourceRepoPath: repoRoot}); err != nil {
		return model.InitResult{}, err
	}
	status, err := s.git.StatusPorcelain(ctx, repoRoot)
	if err != nil {
		return model.InitResult{}, err
	}
	if hasUnstagedChanges(status) {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, "init requires no unstaged git changes before creating the hidden, locked base repo in .stooges; stage/commit/stash or remove unstaged changes first (ignored files are fine)")
	}

	movedEntries := make([]string, 0)
	createdWorkspaces := make([]string, 0, len(workspaces))
	stoogesCreated := false
	baseLocked := false
	defer func() {
		if err == nil {
			return
		}
		rollbackErr := s.rollbackInit(workspaceRoot, layout.BaseRepoPath, layout.MetadataPath, movedEntries, createdWorkspaces, baseLocked, stoogesCreated)
		if rollbackErr == nil {
			return
		}
		err = apperrors.Wrap(apperrors.KindRollbackFailure, "init failed and rollback failed", errors.Join(err, rollbackErr))
	}()

	if err = os.MkdirAll(layout.BaseRepoPath, 0o755); err != nil {
		return model.InitResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "create .stooges base repo directory", err)
	}
	stoogesCreated = true

	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		return model.InitResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "read workspace root entries", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == stoogesDirName {
			continue
		}
		src := filepath.Join(workspaceRoot, name)
		dst := filepath.Join(layout.BaseRepoPath, name)
		if err = os.Rename(src, dst); err != nil {
			return model.InitResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("move %s into .stooges base repo", name), err)
		}
		movedEntries = append(movedEntries, name)
	}

	if err = s.perms.LockReadOnly(layout.BaseRepoPath); err != nil {
		return model.InitResult{}, err
	}
	baseLocked = true

	for _, workspace := range workspaces {
		dst := filepath.Join(workspaceRoot, workspace)
		if err = s.cloner.CloneRepo(ctx, layout.BaseRepoPath, dst); err != nil {
			return model.InitResult{}, err
		}
		createdWorkspaces = append(createdWorkspaces, workspace)
		if err = s.perms.UnlockWritable(dst); err != nil {
			return model.InitResult{}, err
		}
	}
	layout.ManagedWorkspaces = appendManagedWorkspaces(nil, createdWorkspaces...)

	if err = writeWorkspaceMetadata(layout); err != nil {
		return model.InitResult{}, err
	}

	if err = s.chdir(workspaceRoot); err != nil {
		return model.InitResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "switch to workspace root after init", err)
	}
	return model.InitResult{BaseDir: layout.BaseRepoPath, Agents: createdWorkspaces}, nil
}

func ternary(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func isGitRepoPath(path string) bool {
	stat, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && stat.IsDir()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasUnstagedChanges(status string) bool {
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 2 {
			continue
		}
		if line[0] == '?' && line[1] == '?' {
			return true
		}
		if line[1] != ' ' {
			return true
		}
	}
	return false
}

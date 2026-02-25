package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/scottwater/stooges/internal/errors"
	"github.com/scottwater/stooges/internal/fs"
	"github.com/scottwater/stooges/internal/git"
	"github.com/scottwater/stooges/internal/model"
)

type WorkspaceService interface {
	Init(context.Context, model.InitOptions) (model.InitResult, error)
	Make(context.Context, model.MakeOptions) (model.MakeResult, error)
	Sync(context.Context, model.SyncOptions) (model.SyncResult, error)
	Clean(context.Context, model.CleanOptions) (model.CleanResult, error)
	Unlock(context.Context, model.UnlockOptions) (model.UnlockResult, error)
	Lock(context.Context, model.LockOptions) (model.LockResult, error)
	Rebase(context.Context, model.RebaseOptions) (model.RebaseResult, error)
	Doctor(context.Context, model.DoctorOptions) (model.DoctorReport, error)
	Undo(context.Context, model.UndoOptions) (model.UndoResult, error)
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
	return &Service{
		cwd:            deps.CWD,
		chdir:          deps.Chdir,
		cloner:         deps.Cloner,
		perms:          deps.Perms,
		git:            deps.Git,
		preflight:      deps.Preflight,
		resolver:       deps.Resolver,
		branchDetector: deps.BranchDetector,
	}
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
		layout, layoutErr = loadWorkspaceLayout(workspaceRoot)
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

func rollbackCreatedWorkspaces(workspaceRoot string, created []string) error {
	for i := len(created) - 1; i >= 0; i-- {
		target := filepath.Join(workspaceRoot, created[i])
		if err := os.RemoveAll(target); err != nil {
			return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("rollback remove workspace %s", created[i]), err)
		}
	}
	return nil
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

func (s *Service) Make(ctx context.Context, opts model.MakeOptions) (model.MakeResult, error) {
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
		if err := s.cloner.CloneRepo(ctx, sourcePath, dst); err != nil {
			return model.MakeResult{}, err
		}
		if err := s.perms.UnlockWritable(dst); err != nil {
			rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, []string{agent})
			if rollbackErr != nil {
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
			}
			return model.MakeResult{}, err
		}
		if shouldSwitchBranch {
			if err := s.checkoutOrCreateBranch(ctx, dst, targetBranch); err != nil {
				rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, []string{agent})
				if rollbackErr != nil {
					return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
				}
				return model.MakeResult{}, err
			}
		}
		layout.ManagedWorkspaces = appendManagedWorkspaces(layout.ManagedWorkspaces, agent)
		if err := writeWorkspaceMetadata(layout); err != nil {
			rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, []string{agent})
			if rollbackErr != nil {
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
			}
			return model.MakeResult{}, err
		}
		return model.MakeResult{Created: []string{agent}}, nil
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
		return model.MakeResult{Guidance: "all default agents already exist; pass an explicit agent name to create another workspace"}, nil
	}

	created := make([]string, 0, len(missing))
	targetBranch, shouldSwitchBranch, err := resolveTargetBranchForWorkspace(opts, "", len(missing))
	if err != nil {
		return model.MakeResult{}, err
	}
	for _, agent := range missing {
		dst := filepath.Join(workspaceRoot, agent)
		if err := s.cloner.CloneRepo(ctx, sourcePath, dst); err != nil {
			rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, created)
			if rollbackErr != nil {
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
			}
			return model.MakeResult{}, err
		}
		if err := s.perms.UnlockWritable(dst); err != nil {
			rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, append(created, agent))
			if rollbackErr != nil {
				return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
			}
			return model.MakeResult{}, err
		}
		if shouldSwitchBranch {
			branch := targetBranch
			if opts.BranchAuto {
				branch = agent
			}
			if err := s.checkoutOrCreateBranch(ctx, dst, branch); err != nil {
				rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, append(created, agent))
				if rollbackErr != nil {
					return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
				}
				return model.MakeResult{}, err
			}
		}
		created = append(created, agent)
	}
	layout.ManagedWorkspaces = appendManagedWorkspaces(layout.ManagedWorkspaces, created...)
	if err := writeWorkspaceMetadata(layout); err != nil {
		rollbackErr := rollbackCreatedWorkspaces(workspaceRoot, created)
		if rollbackErr != nil {
			return model.MakeResult{}, apperrors.Wrap(apperrors.KindRollbackFailure, "add failed and rollback failed", errors.Join(err, rollbackErr))
		}
		return model.MakeResult{}, err
	}

	return model.MakeResult{Created: created}, nil
}

func (s *Service) Sync(ctx context.Context, opts model.SyncOptions) (model.SyncResult, error) {
	return s.syncRepo(ctx, opts.Repo, false)
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
	if strings.TrimSpace(status) != "" {
		return model.InitResult{}, apperrors.New(apperrors.KindInvalidInput, "init requires a clean git repo; commit/stash changes before locking")
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

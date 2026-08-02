# Changelog

Newest entries are listed first.

## 0.85 — 2026-08-02

### Added
- `stooges enabled [--json]` for scripts and agents that need to detect whether the current directory belongs to a configured Stooges workspace.
- `stooges stooged` as a compact `yes`/`no` workspace probe with script-friendly exit codes.
- `STOOGES_NO_CD` support so project environments can disable shell-driven auto-`cd` without adding `--no-cd` to each command.

### Updated
- Disabled workspace probes exit quietly without an extra error message or passive update notice.
- The website now includes favicons, touch icons, and a web app manifest.

## 0.84 — 2026-05-29

### Updated
- `stooges pr` now hides draft pull requests from the interactive picker by default; pass `--draft` to include them.

## 0.82 — 2026-04-22

### Added
- `stooges pr [number]` to create a new workspace directly from an open GitHub pull request via the GitHub CLI.
- Interactive open-PR selection for `stooges pr` with no arguments, including PR number, author, and title.
- A `checkout PR` action in the no-argument interactive menu so PR-based workspace setup is available alongside the other guided flows.

### Updated
- PR flows now verify `gh auth status` up front and return a clearer authentication error when GitHub CLI is installed but not logged in.
- Shell-driven auto-`cd` support and help text to include the new `pr` workflow.
- Documentation and website command references to cover PR-based workspace creation.

### Modified
- PR picker rendering now constrains long titles and uses raw-terminal-safe line rendering so interactive selection stays readable.
- PR checkout logic now prefers tracked-branch setup for same-repo pull requests and falls back to `gh pr checkout` for cross-repo pull requests.

## 0.81 — 2026-04-21

### Updated
- Command names now accept unique unambiguous prefixes, so `stooges b` / `stooges br` can dispatch to `stooges branch` while ambiguous prefixes still fail.

## 0.80 — 2026-04-17

### Added
- `stooges branch <branch>` to create a derived workspace from the base repo and check out the requested branch in one step.
- `stooges fork <branch>` to create a new workspace from the current managed workspace, preserving that workspace's current state.
- `stooges track <branch>` to create a workspace that tracks an existing remote branch.
- Optional shell-driven auto-`cd` support for `add`, `branch`, `fork`, and `track` when exactly one workspace is created.

### Updated
- `stooges add` now syncs the base repo before cloning from `.stooges` by default, with `--no-sync` available to opt out.
- `stooges init` now applies stricter safeguards around the selected base branch before it mutates the workspace layout.
- Fork and track flows now reject cases where the destination local branch already exists.

### Modified
- Workspace detection for `fork` is more resilient when invoked from subdirectories, real paths, or failure-prone git states.
- Branch flag parsing and auto-`cd` failure output were tightened to produce clearer, more actionable CLI diagnostics.
- Passive update checks are skipped during shell-init flows so shell startup stays clean.

## 0.79 — 2026-03-06

### Added
- `stooges upgrade` to replace the current binary with the latest GitHub release for the current platform.
- Automatic release-notice checks so the CLI can inform users when a newer tagged version is available.

### Updated
- Version handling and comparison logic so installed and latest-release versions are normalized consistently.
- CLI wiring and tests for version-aware update checks and in-place upgrades.

### Modified
- Non-upgrade commands now perform a best-effort release check and print a short upgrade notice at most once per day.
- Upgrade flows now cache latest-release metadata to avoid unnecessary repeat checks.

## 0.78 — 2026-03-04

### Added
- `stooges list` / `stooges ls` to show managed workspaces, their current branch, short commit SHA, and latest commit subject.

### Updated
- Workspace metadata cleanup so missing workspace folders are pruned automatically instead of lingering in the managed state.
- Workspace-root resolution so listing works correctly even when invoked from workspace subdirectories.

### Modified
- Engine and git support layers were expanded to track managed workspace details needed for the new listing command.

## 0.77 — 2026-03-03

### Added
- `--track <branch>` support for `stooges add` so a newly created workspace can track an existing remote branch immediately.

### Updated
- Workspace creation logic to fetch and wire tracking branches safely during clone/setup.
- CLI validation and tests around tracked-branch creation.

### Modified
- The add workflow can now distinguish between local branch creation and remote-tracking branch setup.

## 0.76 — 2026-02-28

### Added
- Initial public release of the unified `stooges` CLI and interactive engine for managing copy-on-write repo workspaces.
- Core commands: `init`, `add`, `sync`, `clean`, `rebase`, `unlock`, `lock`, `undo`/`remove`, `doctor`, and `version`.
- No-argument interactive mode for guided workspace setup and maintenance.
- Installer script for fetching and installing the CLI.

### Updated
- Repository layout management centered on a locked `.stooges` base repo plus sibling managed workspaces.
- Rebase and sync workflows for keeping derived workspaces aligned with the base branch.

### Modified
- Introduced the project’s base copy-on-write workflow as a simpler alternative to git worktrees for fully independent agent workspaces.

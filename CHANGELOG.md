# Changelog

Newest entries are listed first. `Pending` covers changes on `main` that have not been tagged yet.

## Pending

### Added
- `stooges branch <branch>` to create a derived workspace from the base repo and check out the requested branch in one step.
- `stooges fork <branch>` to create a new workspace from the current managed workspace, preserving that workspace's current state.
- `stooges track <branch>` to create a workspace that tracks an existing remote branch.
- Optional shell-driven auto-`cd` support for `add`, `branch`, `fork`, and `track` when exactly one workspace is created.

### Updated
- `stooges add` now syncs the base repo before cloning from `.stooges` by default, with `--no-sync` available to opt out.
- `stooges init` now applies stricter safeguards around the selected base branch before it mutates the workspace layout.
- Fork and track flows now reject cases where the destination local branch already exists.
- Command names now accept unique unambiguous prefixes, so `stooges b` / `stooges br` can dispatch to `stooges branch` while ambiguous prefixes still fail.

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

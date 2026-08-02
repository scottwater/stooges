# Commands

## Install

```bash
go install github.com/scottwater/stooges/cmd/stooges@latest
```

## No-Arg Interactive Mode

```bash
stooges
```

Shows preflight status first, then guided actions for `init`, `add`, `pr`, `sync`, `clean`, `unlock`, `lock`, `rebase`, `undo`, `doctor`.

All commands except `upgrade` perform a best-effort GitHub release check and print an update notice to stderr at most once per 24 hours.

Command names also accept unique unambiguous prefixes, so `stooges br scott/auto-cd` behaves like `stooges branch scott/auto-cd`. Ambiguous prefixes such as `stooges s` still fail.

## `init`

```bash
stooges init [--main-branch <name>|-m <name>] [--workspace <name> ...] [--agents larry,curly,moe] [--confirm]
```

Behavior:
- Must run from inside a git repo.
- Requires no unstaged repo changes before lock/move. Staged changes are allowed. Git-ignored files are ignored as usual.
- The selected base branch must already be the currently checked out branch before init runs.
- Uses `main` as default base branch.
- If your default branch is `master`, pass `--main-branch` / `-m master`.
- Only `main` (default) and `master` are supported.
- Validates that the selected branch actually exists before mutating files.
- Aborts if `./.stooges` already exists.
- Creates managed layout with base repo at `./.stooges` and clones requested agents as siblings (`./<workspace>`).
- Prompts for confirmation by default and shows resolved branch/workspace plan.
- Pass `--confirm` to skip the prompt and run immediately.
- `--workspace` is repeatable and can be used multiple times.

Examples:

```bash
stooges init
stooges init --confirm
stooges init --workspace alpha --workspace beta
stooges init -m master --agents larry,moe
```

## `add`

```bash
stooges add [workspace] [--source <workspace>] [--track <branch>] [--branch [name]|-b[name]] [--no-cd] [--no-sync] [--no-setup] [--rollback-on-setup-failure]
```

Behavior:
- Requires an initialized `.stooges` workspace.
- `stooges add <workspace>` creates only that workspace and fails if it already exists.
- `stooges add` (no workspace) creates only missing defaults among `larry,curly,moe`.
- Default source is `base` (`.stooges`).
- Automatically runs `stooges sync` first when cloning from base, unless `--no-sync` is passed.
- `--source main` and `--source master` are accepted aliases for `base`.
- `--branch <name>` checks out existing branch (or creates it when missing) in each new workspace.
- `-b` / `--branch` (no value) uses workspace name as branch name.
- Named `--branch` with no explicit workspace is allowed only when exactly one workspace is created.
- `--track <branch>` tracks `origin/<branch>` in the new workspace and fails if the remote branch is missing or the destination local branch already exists.
- Auto-sync does not run for `--track` or for non-base `--source` values.
- `--track` requires an explicit workspace name.
- With `--track`, `--branch <name>` sets local branch name while tracking `origin/<track>`.
- `--track` cannot be combined with `-b` (auto branch naming).
- If all defaults exist, no-op with guidance message.
- Never overwrites existing directories.
- With shell integration enabled via `eval "$(stooges shell-init zsh)"` (or `bash`), `add` automatically `cd`s into the newly created workspace when exactly one workspace is created. The same auto-`cd` behavior applies to `branch`, `fork`, `track`, and `pr`.
- `--no-cd` disables that redirect for a single invocation.
- `--no-sync` skips the automatic base sync.
- If `.stooges-metadata.json` has `setupScript`, runs it after clone/branch checkout.
- `--no-setup` skips the configured setup script.
- `--rollback-on-setup-failure` removes created workspace(s) if setup fails. Default is to leave failed setup workspaces in place and managed.

Examples:

```bash
stooges add moe
stooges add --source base
stooges add auto-cd -b
stooges add auto-cd --branch scott/auto-cd
stooges add shell-init --track feature/shell-init
stooges add shell-init --track feature/shell-init --branch shell-init
```

## `branch`

```bash
stooges branch <branch> [--source <workspace>] [--no-cd] [--no-sync] [--no-setup] [--rollback-on-setup-failure]
```

Behavior:
- Convenience wrapper for `stooges add <derived-workspace> -b <branch>`.
- Automatically runs `stooges sync` first when cloning from base, unless `--no-sync` is passed.
- No auto-sync runs when `--source` points at another workspace.
- Derives workspace name from the last non-empty `/`-separated branch segment.
- When the branch has no `/`, derives the workspace from the first 50 sanitized characters.
- Sanitization keeps letters, digits, `-`, and `_`, and collapses other characters into `-`.
- Fails when derivation produces an empty name or reserved `base`.
- The provided branch name is used verbatim as the local branch to create or switch to in the new workspace.
- Workspace collisions still fail normally, so `scott/foo` and `team/foo` will both try to use workspace `foo`.
- Supports the same setup hook, `--no-setup`, and `--rollback-on-setup-failure` behavior as `add`.

Examples:

```bash
stooges branch scott/auto-cd
stooges branch "shell init polish: 2026-04-15"
```

## `fork`

```bash
stooges fork <branch> [--no-cd] [--no-setup] [--rollback-on-setup-failure]
```

Behavior:
- Convenience wrapper for `stooges add <derived-workspace> --source <current-workspace> --branch <branch>`.
- Must run from inside a managed workspace (including its subdirectories), not from the workspace root or base repo.
- Clones the current managed workspace rather than `.stooges`, so the new workspace starts from the current workspace state instead of the base branch state.
- Derives workspace name from the last non-empty `/`-separated branch segment.
- When the branch has no `/`, derives the workspace from the first 50 sanitized characters.
- Sanitization keeps letters, digits, `-`, and `_`, and collapses other characters into `-`.
- Fails when derivation produces an empty name or reserved `base`.
- Allows normal dirty/untracked changes in the source workspace to come along for the copy.
- Fails if the requested local branch already exists in the copied workspace.
- Refuses to run when the source workspace has an in-progress merge, rebase, cherry-pick, revert, sequencer operation, or git index lock.
- Workspace collisions still fail normally, so `scott/foo` and `team/foo` will both try to use workspace `foo`.
- Supports the same setup hook, `--no-setup`, and `--rollback-on-setup-failure` behavior as `add`.

Examples:

```bash
cd larry
stooges fork scott/auto-cd
stooges fork "shell init polish: 2026-04-15"
```

## `track`

```bash
stooges track <branch> [--source <workspace>] [--branch [name]|-b[name]] [--no-cd] [--no-setup] [--rollback-on-setup-failure]
```

Behavior:
- Convenience wrapper for `stooges add <derived-workspace> --track <branch>`.
- Derives workspace name from the last non-empty `/`-separated branch segment.
- When the branch has no `/`, derives the workspace from the first 50 sanitized characters.
- Sanitization keeps letters, digits, `-`, and `_`, and collapses other characters into `-`.
- Fails when derivation produces an empty name or reserved `base`.
- `--branch <name>` still controls the local git branch name; without it, the local branch defaults to the tracked branch, same as `add --track`.
- Fails if `origin/<branch>` is missing or if the destination local branch already exists in the copied workspace.
- Workspace collisions still fail normally, so `feature/foo` and `bug/foo` will both try to use workspace `foo`.
- Supports the same setup hook, `--no-setup`, and `--rollback-on-setup-failure` behavior as `add`.

Examples:

```bash
stooges track feature/shell-init
stooges track feature/shell-init --branch shell-init
stooges track "feature/shell-init-polish"
```

## `pr`

```bash
stooges pr [number] [--branch <name>] [--draft] [--no-cd] [--no-setup] [--rollback-on-setup-failure]
```

Behavior:
- Requires the GitHub CLI (`gh`) in PATH and an authenticated GitHub session.
- With a numeric argument, looks up that PR in the current repository and creates a workspace for it.
- With no argument, lists open non-draft PRs in the current repository and lets you choose one interactively (arrow-key picker on a TTY; numbered fallback otherwise).
- `--draft` includes draft PRs in the interactive picker.
- The PR list shows the PR number, author login, and title.
- Same-repo PRs derive the workspace from the PR head branch and use tracked-branch setup, equivalent to `track` where possible.
- Cross-repo PRs still derive the workspace from the PR head branch, but fall back to `gh pr checkout` inside the new workspace; setup runs only after that checkout succeeds.
- When derivation would be empty or reserved, falls back to `pr-<number>` for the workspace name.
- `--branch <name>` overrides the local branch name used for the checkout.
- `--no-cd` suppresses shell-wrapper auto-`cd`, same as `add`, `branch`, `fork`, and `track`.
- Supports the same setup hook, `--no-setup`, and `--rollback-on-setup-failure` behavior as `add`.

Examples:

```bash
stooges pr 37
stooges pr 37 --branch review/pr-37
stooges pr
stooges pr --draft
```

## `sync`

```bash
stooges sync [--repo <path>]
```

Behavior:
- Targets `.stooges` by default.
- `--repo` is allowed only when it points to `.stooges`.
- Temporarily unlocks repo, fetches, switches branch, pulls ff-only, relocks.
- Prints symlink count warning context.

## `clean`

```bash
stooges clean [--repo <path>]
```

Behavior:
- Same target repo behavior as `sync` (`.stooges`).
- Temporarily unlocks repo, fetches/prunes, switches branch, pulls ff-only, relocks.
- Prints symlink count warning context.

## `rebase`

```bash
stooges rebase [--repo <path>] [--prune]
```

Behavior:
- Runs `sync` first (`--prune` uses sync+prune behavior).
- Scans managed git workspaces in workspace root (for example: `larry`, `curly`, `moe`).
- For each workspace:
  - skips if dirty
  - skips if already on base branch or already contains base tip
  - rebases onto base branch when clean/safe
  - if conflict occurs, aborts rebase and reports workspace for manual handling
- Prints grouped summary: rebased, dirty-skipped, current-skipped, conflicted.

## `list` / `ls`

```bash
stooges list
stooges ls
```

Behavior:
- Lists `base` (`.stooges`) first, then managed workspaces from metadata.
- Shows current branch, short HEAD commit SHA, and latest commit subject.
- Prunes missing workspace folders from metadata and omits them from output.
- Can run from workspace subdirectories; resolves the configured workspace root from current path.

## `trash`

```bash
stooges trash <workspace> [--force]
```

Behavior:
- Preflights that removal can proceed before running configured `teardownScript`.
- Runs configured `teardownScript` first, when present.
- Uses the `trash` command when available.
- If `trash` is not available, fails unless `--force` is passed.
- With `--force`, permanently deletes with `os.RemoveAll` when `trash` is unavailable.
- If teardown fails, leaves the workspace in place unless `--force` is passed; forced removal reports the teardown failure as a warning.
- Removes the workspace from `.stooges-metadata.json` only after removal succeeds.

## Workspace hooks

Configure hooks by adding `setupScript` and/or `teardownScript` to the existing `.stooges-metadata.json`. Keep the required fields that `stooges init` created, such as `mainBranch` and `managedWorkspaces`:

```json
{
  "mainBranch": "main",
  "managedWorkspaces": ["larry", "curly", "moe"],
  "setupScript": "scripts/stooges-setup.sh",
  "teardownScript": "/absolute/path/to/stooges-teardown.sh"
}
```

Relative paths resolve from the workspace root containing `.stooges`. Hooks run from the workspace directory.

Environment:
- `STOOGES_CWD`: original command cwd
- `STOOGES_MAIN`: workspace root containing `.stooges`
- `STOOGES_SOURCE`: source workspace name (`base` by default)
- `STOOGES_BRANCH`: requested/local branch when known
- `STOOGES_FOLDER`: workspace name
- `STOOGES_FOLDER_PATH`: workspace absolute path

## `unlock`

```bash
stooges unlock [--repo <path>]
```

Behavior:
- Same target repo behavior as `sync`/`clean` (`.stooges`).
- Unlocks files/dirs to user-writable.

## `lock`

```bash
stooges lock [--repo <path>]
```

Behavior:
- Same target repo behavior as `sync`/`clean`/`unlock` (`.stooges`).
- Locks files/dirs to read-only.

## `undo` / `remove`

```bash
stooges undo [--yes]
stooges remove [--yes]
```

Behavior:
- Destructive and non-transactional. Command prints step-by-step log and backup path.
- Verifies `.stooges` plus managed workspace repos have clean `git status --porcelain`.
- Auto-unlocks git repos before filesystem moves/deletes.
- Removes managed non-base git workspace repos.
- Moves `.stooges` to parent as `<project>_<id>.bak`.
- Restores backup contents directly into existing workspace root (avoids stale shell cwd state).

## `doctor`

```bash
stooges doctor [--repo <path>] [--json]
```

Checks:
- `git` availability
- copy-on-write clone support
- workspace validity
- `.stooges` workspace layout / base repo resolution
- active `.gitignore` patterns that currently match on-disk paths (warning-only)

## `enabled`

```bash
stooges enabled [--json]
stooges stooged
```

Behavior:
- Checks only whether the current directory is inside a configured Stooges workspace.
- Walks parent directories to find `.stooges`, then verifies `.stooges` is a git repo and `.stooges-metadata.json` is readable and valid.
- Prints `enabled` and exits `0` when configured.
- Prints `not enabled` and exits `1` when not configured.
- `--json` prints `enabled`, `workspaceRoot`, `baseRepoPath`, `metadataPath`, and `reason` when available.
- `stooges stooged` uses the same check, prints `yes`/`no`, and exits `0`/`1`.

## `shell-init`

```bash
stooges shell-init [bash|zsh|sh]
```

Behavior:
- Prints a shell wrapper function.
- After `eval "$(stooges shell-init zsh)"`, successful single-workspace `stooges add`, `stooges branch`, `stooges fork`, `stooges track`, and `stooges pr` commands automatically `cd` into the created workspace.
- Uses the resolved workspace root, so calling these commands from inside another managed workspace still lands in the new workspace root path.
- `--no-cd` on `add`, `branch`, `fork`, `track`, or `pr` suppresses the redirect for one command.
- A truthy `STOOGES_NO_CD` environment value (`1`, `true`, `yes`, or `on`, case-insensitive) suppresses the redirect for every command in that environment.

A Herdr project using mise can disable auto-cd only inside Herdr-managed panes:

```toml
[env]
STOOGES_NO_CD = "{{ env.HERDR_ENV | default(value='0') }}"
```

## `version`

```bash
stooges version
stooges --version
```

Behavior:
- Prints installed CLI version.

## `upgrade`

```bash
stooges upgrade
```

Behavior:
- Queries GitHub Releases for the latest tagged version.
- Compares the latest tag against the installed CLI version.
- If newer, downloads the matching release archive for the current OS/arch and replaces the current executable in place.
- If already current, prints the latest version and exits without modifying the binary.

## Exit Codes

- `0`: success
- `1`: unknown error
- `2`: invalid input
- `3`: unsupported platform
- `4`: preflight failure
- `5`: git failure
- `6`: filesystem failure
- `7`: rollback failure

# Commands

## Install

```bash
go install github.com/scottwater/stooges/cmd/stooges@latest
```

## No-Arg Interactive Mode

```bash
stooges
```

Shows preflight status first, then guided actions for `init`, `add`, `sync`, `clean`, `unlock`, `lock`, `rebase`, `undo`, `doctor`.

All commands except `upgrade` perform a best-effort GitHub release check and print an update notice to stderr at most once per 24 hours.

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
stooges add [workspace] [--source <workspace>] [--track <branch>] [--branch [name]|-b[name]] [--no-cd] [--no-sync]
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
- `--track <branch>` tracks `origin/<branch>` in the new workspace and fails if remote branch is missing.
- Auto-sync does not run for `--track` or for non-base `--source` values.
- `--track` requires an explicit workspace name.
- With `--track`, `--branch <name>` sets local branch name while tracking `origin/<track>`.
- `--track` cannot be combined with `-b` (auto branch naming).
- If all defaults exist, no-op with guidance message.
- Never overwrites existing directories.
- With shell integration enabled via `eval "$(stooges shell-init zsh)"` (or `bash`), `add` automatically `cd`s into the newly created workspace when exactly one workspace is created. The same auto-`cd` behavior applies to `branch`, `fork`, and `track`.
- `--no-cd` disables that redirect for a single invocation.
- `--no-sync` skips the automatic base sync.

Examples:

```bash
stooges add moe
stooges add --source base
stooges add bob -b
stooges add bob --branch not_bob
stooges add bob --track feature/foo
stooges add bob --track feature/foo --branch local-foo
```

## `branch`

```bash
stooges branch <branch> [--source <workspace>] [--no-cd] [--no-sync]
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

Examples:

```bash
stooges branch scott/aud-656
stooges branch "release candidate: 2026-04-15"
```

## `fork`

```bash
stooges fork <branch> [--no-cd]
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

Examples:

```bash
cd larry
stooges fork scott/aud-656
stooges fork "release candidate: 2026-04-15"
```

## `track`

```bash
stooges track <branch> [--source <workspace>] [--branch [name]|-b[name]] [--no-cd]
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

Examples:

```bash
stooges track feature/foo
stooges track feature/foo --branch local-foo
stooges track "release candidate: 2026-04-15"
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

## `shell-init`

```bash
stooges shell-init [bash|zsh|sh]
```

Behavior:
- Prints a shell wrapper function.
- After `eval "$(stooges shell-init zsh)"`, successful single-workspace `stooges add`, `stooges branch`, `stooges fork`, and `stooges track` commands automatically `cd` into the created workspace.
- Uses the resolved workspace root, so calling these commands from inside another managed workspace still lands in the new workspace root path.
- `--no-cd` on `add`, `branch`, `fork`, or `track` suppresses the redirect.

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

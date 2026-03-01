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

## `init`

```bash
stooges init [--main-branch <name>|-m <name>] [--workspace <name> ...] [--agents larry,curly,moe] [--confirm]
```

Behavior:
- Must run from inside a git repo.
- Requires a clean repo (`git status --porcelain` must be empty) before lock/move.
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
stooges add [workspace] [--source <workspace>] [--branch [name]|-b[name]]
```

Behavior:
- Requires an initialized `.stooges` workspace.
- `stooges add <workspace>` creates only that workspace and fails if it already exists.
- `stooges add` (no workspace) creates only missing defaults among `larry,curly,moe`.
- Default source is `base` (`.stooges`).
- `--source main` and `--source master` are accepted aliases for `base`.
- `--branch <name>` checks out existing branch (or creates it when missing) in each new workspace.
- `-b` / `--branch` (no value) uses workspace name as branch name.
- Named `--branch` with no explicit workspace is allowed only when exactly one workspace is created.
- If all defaults exist, no-op with guidance message.
- Never overwrites existing directories.

Examples:

```bash
stooges add moe
stooges add --source base
stooges add bob -b
stooges add bob --branch not_bob
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

## `version`

```bash
stooges version
stooges --version
```

Behavior:
- Prints installed CLI version (currently `0.76`).

## Exit Codes

- `0`: success
- `1`: unknown error
- `2`: invalid input
- `3`: unsupported platform
- `4`: preflight failure
- `5`: git failure
- `6`: filesystem failure
- `7`: rollback failure

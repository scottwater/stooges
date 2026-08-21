# Stooges

> [!IMPORTANT]
> If you already have a git worktree flow that works for you, stick with it.
> Stooges is a (simple) alternative to worktrees, but if they work for you, go for
> it!

## Why Stooges?

If you work with AI coding agents, run long test suites, or just want to context-switch between tasks without stashing, you need multiple copies of your repo. Your options:

| | Disk cost | Independent? | Tooling |
|---|---|---|---|
| **Plain copies** | Full duplicate | ✅ Yes | None — you manage them yourself |
| **Git worktrees** | Shared `.git` | ❌ Shared index & lock files | Built into git |
| **Stooges** | Copy-on-write (near zero) | ✅ Yes | CLI for create, sync, rebase, cleanup |

Worktrees are great until they aren't — a rebase in one worktree locks the index for all of them, and some tools (like certain editors and AI agents) get confused by the shared `.git` structure. Plain copies work but waste disk space and give you nothing to manage them with.

Stooges gives you fully independent repo clones that take almost no extra disk space, with a small CLI to create, sync, and clean them up.

## How it works

APFS (macOS) and some Linux filesystems support copy-on-write cloning — instant copies that share disk blocks with the original and only use extra space as files diverge. On macOS this has been the default since High Sierra (2017). For Linux, reflink support has been around for a while but varies by filesystem. `stooges doctor` will tell you if your system supports it.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/scottwater/stooges/main/install.sh | bash
```

Or with Go:

```bash
go install github.com/scottwater/stooges/cmd/stooges@latest
```

## Getting started

```bash
# make sure your system supports copy-on-write cloning
stooges doctor

# initialize — this restructures your repo directory
stooges init
```

You can also just run `stooges` with no arguments for an interactive guided mode.

### Optional: auto-cd after creating a workspace

This feature is optional.

By default, the `stooges` binary cannot change your current shell directory on its own, so if you want successful single-workspace `stooges add`, `stooges branch`, `stooges fork`, `stooges track`, or `stooges pr` commands to automatically drop you into the new workspace, enable the shell wrapper once in your shell profile:

```bash
# zsh

eval "$(stooges shell-init zsh)"

# bash

eval "$(stooges shell-init bash)"
```

After that, successful single-workspace `stooges add`, `stooges branch`, `stooges fork`, `stooges track`, and `stooges pr` commands will automatically `cd` into the created workspace. Pass `--no-cd` for one command or set `STOOGES_NO_CD=1` in the environment to disable auto-cd for every command in that environment.

For example, a project that uses [Herdr](https://herdr.dev/) can disable auto-cd only in Herdr-managed panes with this project-level `mise.toml` setting:

```toml
[env]
STOOGES_NO_CD = "{{ env.HERDR_ENV | default(value='0') }}"
```

Herdr sets `HERDR_ENV=1` in managed panes. Outside Herdr, this mise setting resolves to `0` and keeps auto-cd enabled.

### What `init` does

Running `stooges init` inside your repo restructures the directory:

```
# Before                # After
myproject/              myproject/
├── .git/               ├── .stooges/    ← your original repo, locked read-only
├── src/                ├── larry/       ← clone workspace
├── package.json        ├── curly/       ← clone workspace
└── ...                 └── moe/         ← clone workspace
```

1. Your repo contents are moved into a `.stooges` directory
2. The `.stooges` directory is locked read-only (so you don't accidentally edit the base)
3. Three default workspaces are created as copy-on-write clones

`init` must be run while you are already checked out on the branch you want to treat as the base (`main` by default, or `master` via `--main-branch`). It also requires no unstaged changes before it creates the hidden `.stooges` directory; staged changes are fine, and git-ignored files are ignored as usual. This avoids locking an unexpected branch into the hidden `.stooges` directory.

If you don't want the defaults, specify your own with `--workspace`:

```bash
stooges init --workspace agent1 --workspace agent2
```

Changed your mind? `stooges undo` puts everything back the way it was.

## Workflow

```bash
# create a new workspace with its own branch
stooges add auto-cd -b

# with shell integration enabled, you're dropped into auto-cd automatically
# otherwise: cd auto-cd
# edit, commit, push — it's a full independent repo

# keep workspaces up to date with your base branch
stooges rebase

# done? push your branch and trash the workspace
cd ..
stooges trash auto-cd
```

You can add a new workspace at any time with `stooges add`. The `-b` flag creates a branch named after the workspace, or use `--branch name` for a specific branch name.
When plain `add` or `branch` clones from base, Stooges syncs the base repo first by default so new workspaces start from the latest base state; pass `--no-sync` to skip that. `add --track` does not auto-sync first.
Use `stooges branch <branch>` to derive the workspace name from the branch suffix and create/switch that local branch automatically.
Use `stooges fork <branch>` from inside a managed workspace to copy that workspace, keep its current changes, and then create the requested local branch in the new copy; it fails if that local branch already exists in the copied workspace.
Use `--track <branch>` to track `origin/<branch>` in a newly created workspace (optionally with `--branch <local-name>`); it fails if `origin/<branch>` is missing or if the destination local branch already exists. Or use `stooges track <branch>` to derive the workspace name automatically.
Use `stooges pr <number>` to create a workspace for a GitHub pull request in the current repository. With no number, Stooges uses the GitHub CLI (`gh`) to list open non-draft PRs in the current repository, lets you pick one interactively, then creates the workspace and checks out that PR; pass `--draft` to include draft PRs in that picker. Stooges checks `gh auth status` first and asks you to authenticate if needed. Same-repo PRs use tracked-branch setup; cross-repo PRs fall back to `gh pr checkout` in the new workspace, then run setup only after checkout succeeds.
If you optionally enable `eval "$(stooges shell-init zsh)"` (or `bash`), `stooges add`, `stooges branch`, `stooges fork`, `stooges track`, and `stooges pr` will automatically `cd` into the new workspace; pass `--no-cd` for one command or set `STOOGES_NO_CD=1` in the environment to stay where you are.

Optional workspace hooks can be configured by adding `setupScript` and/or `teardownScript` to the existing `.stooges-metadata.json`. Keep the required metadata fields that `stooges init` created:

```json
{
  "mainBranch": "main",
  "managedWorkspaces": ["larry", "curly", "moe"],
  "setupScript": "scripts/stooges-setup.sh",
  "teardownScript": "scripts/stooges-teardown.sh"
}
```

Relative hook paths resolve from the workspace root (the directory containing `.stooges`). Setup runs after clone/branch checkout for new workspaces, and after successful `gh pr checkout` for cross-repo PRs, not during the initial `stooges init`. Teardown runs before `stooges trash <workspace>`. Hooks run from the workspace directory with `STOOGES_CWD`, `STOOGES_MAIN`, `STOOGES_PROJECT` (the workspace root folder name), `STOOGES_SOURCE`, `STOOGES_BRANCH`, `STOOGES_FOLDER`, and `STOOGES_FOLDER_PATH` set. Setup failures leave the workspace in place and managed by default; pass `--rollback-on-setup-failure` to `add`, `branch`, `fork`, `track`, or `pr` to remove created workspace(s), or `--no-setup` to skip the hook.

### Workspace creation progress

`add`, `branch`, `fork`, `track`, and `pr` show the current workspace and creation phase, including base sync, copying, branch or tracking configuration, PR checkout, setup, and rollback when applicable. Single-workspace creation shows only its name; ordinal counters such as `[2/3]` appear only when creating multiple workspaces serially. Interactive terminals use one delayed, transient spinner and show elapsed time for longer phases; redirected or CI stderr receives durable phase start and completion lines instead. Setup-hook stdout and stderr stream live to Stooges' stderr, while command stdout remains the stable result channel for lines such as `created:` and `checked out:`. Successful single-workspace auto-CD still happens only after creation and setup finish. Setup failure identifies a retained managed workspace or successful rollback; if rollback also fails, Stooges reports the cleanup failure and warns that workspace state is uncertain. Failures do not auto-CD, and `--no-cd` plus `STOOGES_NO_CD` keep their existing behavior.

## Keeping in sync

```bash
# fetch latest from remote into the base repo and relock it
stooges sync

# sync + prune stale remote-tracking refs
stooges clean

# sync base + rebase all workspace branches onto the base branch
stooges rebase --prune
```

If you need to manually edit the base repo (e.g., resolve something), use `stooges unlock` and `stooges lock` to temporarily toggle the read-only protection.

## Quickstart

```bash
stooges doctor             # check platform support
stooges enabled            # cheap probe: exit 0 when this directory supports Stooges
stooges stooged            # yes/no probe: prints yes when configured, no otherwise
stooges --version          # print installed version
stooges upgrade            # replace current binary with latest release
stooges shell-init zsh     # optional: print shell wrapper for auto-cd on add/branch/fork/track/pr
stooges                    # guided interactive mode (includes PR checkout)
stooges init               # initialize workspace layout

stooges add                         # create missing default workspaces
stooges add moe                     # create one workspace (auto-syncs base first)
stooges add auto-cd -b              # create workspace + branch named "auto-cd" (auto-syncs base first)
stooges add auto-cd --branch scott/auto-cd        # create workspace + branch named "scott/auto-cd"
stooges add shell-init --track feature/shell-init # track origin/feature/shell-init
stooges add shell-init --track feature/shell-init --branch shell-init # local "shell-init" tracking origin/feature/shell-init
stooges branch scott/auto-cd        # derive workspace "auto-cd" and create/switch branch scott/auto-cd
stooges fork scott/auto-cd          # from inside a managed workspace, copy it and branch from that current state
stooges track feature/shell-init    # derive workspace "shell-init" and track origin/feature/shell-init
stooges pr 37                       # create a workspace for PR #37
stooges pr                          # choose an open non-draft PR interactively with gh, then create a workspace for it
stooges pr --draft                  # include draft PRs in the interactive picker
stooges pr 37 --no-setup            # skip configured setup hook for this PR workspace
stooges branch scott/auto-cd --rollback-on-setup-failure # remove created workspace if setup fails

stooges sync               # sync base repo from remote
stooges clean              # sync + prune stale refs
stooges rebase --prune     # sync + rebase workspaces onto base
stooges list               # list base + workspaces with branch and latest commit
stooges trash auto-cd      # preflight removal, run teardown hook, then move workspace to Trash
stooges trash auto-cd --force # permanently delete if trash is unavailable; warn if forced past teardown failure

stooges undo --yes         # tear down workspace layout (destructive)
```

## Commands

- `init` — initialize the workspace layout
- `add` — create workspaces; supports setup hooks plus `--no-setup` / `--rollback-on-setup-failure`
- `branch` — create a workspace using a derived name and an explicit local branch; supports setup controls
- `fork` — copy the current managed workspace into a new derived workspace and branch from there; supports setup controls
- `track` — create a tracked workspace using a derived workspace name; supports setup controls
- `pr` — create a workspace for a GitHub pull request using `gh`; cross-repo setup runs after checkout succeeds
- `sync` — fetch & update the base repo
- `clean` — sync + prune stale remote-tracking refs
- `rebase` — sync base + rebase workspace branches
- `list` (`ls`) — show base + managed workspaces with git head info
- `trash` — preflight removal, run teardown hook, and move a managed workspace to Trash; `--force` permanently deletes when needed
- `unlock` / `lock` — toggle read-only protection on the base
- `undo` (alias: `remove`) — tear down and restore original layout
- `doctor` — check platform support and workspace health
- `enabled` — check whether the current directory is inside a configured Stooges workspace
- `stooged` — print `yes`/`no` for configured Stooges workspace detection
- `shell-init` — print shell wrapper for auto-cd on `add`, `branch`, `fork`, `track`, and `pr`
- `version` (or `--version`) — print installed version
- `upgrade` — install the latest GitHub release over the current binary
- no args — interactive mode

Command names also accept unique unambiguous prefixes, so `stooges b ...` and `stooges br ...` both dispatch to `stooges branch ...`. Ambiguous prefixes such as `stooges s` still fail.

When a newer release exists, Stooges prints a one-line upgrade notice at most once per 24 hours:

```text
Update available: v0.79 (installed v0.78). Run: stooges upgrade
```

## Documentation

- [Command reference](docs/COMMANDS.md)
- [Architecture notes](docs/ARCHITECTURE.md)
- [Migration mapping](docs/MIGRATION.md)
- [Release checklist](docs/RELEASING.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

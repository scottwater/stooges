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

### Optional: auto-cd after `stooges add`

This feature is optional.

By default, the `stooges` binary cannot change your current shell directory on its own, so if you want `stooges add <workspace>` to automatically drop you into the new workspace, enable the shell wrapper once in your shell profile:

```bash
# zsh

eval "$(stooges shell-init zsh)"

# bash

eval "$(stooges shell-init bash)"
```

After that, successful single-workspace `stooges add` commands will automatically `cd` into the created workspace. Pass `--no-cd` any time you want to stay where you are.

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
stooges add feature-x -b

# with shell integration enabled, you're dropped into feature-x automatically
# otherwise: cd feature-x
# edit, commit, push — it's a full independent repo

# keep workspaces up to date with your base branch
stooges rebase

# done? push your branch and delete the workspace
cd ..
trash feature-x
```

You can add a new workspace at any time with `stooges add`. The `-b` flag creates a branch named after the workspace, or use `--branch name` for a specific branch name.
Use `stooges branch <branch>` to derive the workspace name from the branch suffix and create/switch that local branch automatically.
Use `--track <branch>` to track `origin/<branch>` in a newly created workspace (optionally with `--branch <local-name>`), or use `stooges track <branch>` to derive the workspace name automatically.
If you optionally enable `eval "$(stooges shell-init zsh)"` (or `bash`), `stooges add`, `stooges branch`, and `stooges track` will automatically `cd` into the new workspace; pass `--no-cd` to stay where you are.

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
stooges --version          # print installed version
stooges upgrade            # replace current binary with latest release
stooges shell-init zsh     # optional: print shell wrapper for auto-cd on add/branch/track
stooges                    # guided interactive mode
stooges init               # initialize workspace layout

stooges add                # create missing default workspaces
stooges add moe            # create one workspace
stooges add bob -b         # create workspace + branch named "bob"
stooges add bob --branch x # create workspace + branch named "x"
stooges add bob --track feature/foo              # track origin/feature/foo
stooges add bob --track feature/foo --branch foo # local "foo" tracking origin/feature/foo
stooges branch scott/aud-656                      # derive workspace "aud-656" and create/switch branch scott/aud-656
stooges track feature/foo                         # derive workspace "foo" and track origin/feature/foo

stooges sync               # sync base repo from remote
stooges clean              # sync + prune stale refs
stooges rebase --prune     # sync + rebase workspaces onto base
stooges list               # list base + workspaces with branch and latest commit

stooges undo --yes         # tear down workspace layout (destructive)
```

## Commands

- `init` — initialize the workspace layout
- `add` — create workspaces
- `branch` — create a workspace using a derived name and an explicit local branch
- `track` — create a tracked workspace using a derived workspace name
- `sync` — fetch & update the base repo
- `clean` — sync + prune stale remote-tracking refs
- `rebase` — sync base + rebase workspace branches
- `list` (`ls`) — show base + managed workspaces with git head info
- `unlock` / `lock` — toggle read-only protection on the base
- `undo` (alias: `remove`) — tear down and restore original layout
- `doctor` — check platform support and workspace health
- `shell-init` — print shell wrapper for auto-cd on `add`, `branch`, and `track`
- `version` (or `--version`) — print installed version
- `upgrade` — install the latest GitHub release over the current binary
- no args — interactive mode

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

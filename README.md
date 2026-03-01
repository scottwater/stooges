# stooges

> [!IMPORTANT]
> If you already have a git worktree flow that works for you, stick with it.
> Stooges is (simple) alternative to worktrees, but if they work for you, go for
> it!



Unified AFS workspace CLI using a managed `.stooges` base repo plus sibling agent clones.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/scottwater/stooges/main/install.sh | bash
```

Or with Go:

```bash
go install github.com/scottwater/stooges/cmd/stooges@latest
```

## Quickstart

```bash
# check platform/workspace capability
stooges doctor

# print installed version
stooges --version

# guided mode
stooges

# create missing default workspaces
stooges add

# create one workspace
stooges add moe

# create workspace and branch named after workspace
stooges add bob -b

# create workspace and explicit branch
stooges add bob --branch not_bob

# unlock + sync base repo
stooges unlock
stooges sync
stooges lock

# sync + prune stale remote-tracking refs
stooges clean

# sync base + rebase workspaces onto base branch
stooges rebase --prune

# undo workspace layout (destructive)
stooges undo --yes
```

## Commands
- `init`
- `add`
- `sync`
- `clean`
- `rebase`
- `unlock`
- `lock`
- `undo` (alias: `remove`)
- `doctor`
- `version` (or `--version`)
- no args: interactive mode

Detailed contract: `docs/COMMANDS.md`.
Migration mapping: `docs/MIGRATION.md`.
Architecture notes: `docs/ARCHITECTURE.md`.
Release checklist: `docs/RELEASING.md`.

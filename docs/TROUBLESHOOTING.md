# Troubleshooting

## `preflight checks failed`

Run:

```bash
stooges doctor
```

Common causes:
- `git` missing in `PATH`
- copy-on-write clone unsupported on current filesystem
- source repo missing `.git`

## `workspace not configured (missing .stooges)`

Run:

```bash
stooges init
```

Then retry your command.

## `unsupported repo path ... only base repo ... is supported`

`sync`, `clean`, `unlock`, and `lock` only operate on `.stooges`.
Pass `--repo` only when you need an explicit path to that same repo.

## `init aborted: .stooges already exists`

Workspace already initialized.
Use existing commands (`add/sync/clean/rebase/...`) or run `stooges undo --yes` first if you want to reset layout.

## `init requires the selected base branch ... to be checked out`

Before running `stooges init`, switch to the branch you want to lock in as the base repo (`main` by default, or `master` when passed via `--main-branch`).

## `init requires no unstaged git changes ...`

`stooges init` creates a hidden, locked copy of the repo in `.stooges/`, so run it only when there are no unstaged changes.
Staged changes are allowed. Git-ignored files are ignored as usual.
Stage, commit, stash, or remove unstaged changes first, then retry.

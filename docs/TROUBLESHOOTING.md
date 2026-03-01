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

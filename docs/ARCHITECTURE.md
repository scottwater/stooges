# Stooges Architecture

## Design
Stooges uses one shared engine for all behavior. Two frontends call the same engine methods:

- CLI subcommands (`internal/cli/*`)
- Interactive no-arg mode (`internal/interactive/*`)

This prevents behavior drift between guided and automated usage.

## Layers
- `cmd/stooges/main.go`
  - Process entrypoint.
  - Converts typed errors to stable exit codes.
- `internal/cli`
  - Cobra command tree.
  - Flag parsing and output formatting only.
  - Includes non-engine metadata commands (`version` / `--version`, `upgrade`, release notice checks).
- `internal/interactive`
  - Text menu router.
  - Confirmation/dry-run-style previews.
- `internal/engine`
  - Core workflows (`init`, `add`, `sync`, `clean`, `rebase`, `unlock`, `lock`, `undo`, `doctor`).
  - Preflight enforcement and workspace safety rules.
- `internal/fs`
  - Copy-on-write clone probe + clone execution.
  - Lock/unlock permission primitives.
- `internal/git`
  - Git operation wrappers (`fetch`, `switch`, `pull`, branch detection).
- `internal/model`
  - Option/result structs and normalization helpers.
- `internal/errors`
  - Typed errors and deterministic exit code mapping.

## Safety Invariants
All mutating commands (`init`, `add`, `sync`, `clean`, `unlock`, `lock`) fail closed unless preflight passes:

- `git` must be available
- workspace path must be valid
- copy-on-write clone support must be available (`cp -c` on macOS, `cp --reflink=always` on Linux)
- source repo must be valid when required

No overwrite path exists in `add`; existing targets are not replaced.
`sync`/`clean` always relock repo on both success and error paths.
`init` includes rollback-on-failure for moved/cloned entries.

## Command to Engine Mapping
- `stooges init` -> `Service.Init`
- `stooges add` -> `Service.Make`
- `stooges sync` -> `Service.Sync`
- `stooges clean` -> `Service.Clean`
- `stooges list` / `stooges ls` -> `Service.List`
- `stooges rebase` -> `Service.Rebase`
- `stooges unlock` -> `Service.Unlock`
- `stooges lock` -> `Service.Lock`
- `stooges undo` / `stooges remove` -> `Service.Undo`
- `stooges doctor` -> `Service.Doctor`
- `stooges version` / `stooges --version` -> `internal/version.Value` (no engine call)
- `stooges upgrade` -> GitHub release check + binary replacement (no engine call)
- `stooges` (no args) -> interactive menu -> same service methods

## Testing Strategy
- Unit tests for model and typed errors.
- Engine tests for semantics and safety behavior.
- Integration workflow tests for end-to-end directory operations.
- CLI and interactive adapter tests to prove engine parity.

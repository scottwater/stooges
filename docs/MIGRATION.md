# Migration Guide

Legacy scripts remain unchanged. `stooges` is the unified replacement path.

## Script Mapping

| Old | New | Notes |
|---|---|---|
| `make-agent <name> [main|master]` | `stooges add <name> --source base` | New command never overwrites existing target. `main/master` aliases still map to `base`. |
| `make-agents [main|master]` | `stooges add --source base` | New default behavior is additive: creates only missing `larry,curly,moe`. |
| `unlock-main [repo_path]` | `stooges unlock [--repo <path>]` | Targets `.stooges` only. |
| `update-main [repo_path]` | `stooges sync [--repo <path>]` | Same unlock/fetch/switch/pull/relock pattern (without prune) on `.stooges`. |
| `update-main [repo_path]` + stale ref cleanup | `stooges clean [--repo <path>]` | Sync flow plus `git fetch --prune`. |

## Intentional Behavior Delta

`make-agents` old behavior:
- aborted if any default target already existed.

`stooges add` new no-arg behavior:
- creates only missing defaults (`larry`, `curly`, `moe`).
- if all exist, returns guidance and does not mutate.

## Suggested Rollout

1. Install `stooges` with `go install`.
2. Run `stooges doctor` in your workspace.
3. Run `stooges init` once to create `.stooges` + default workspaces.
4. Start using `stooges add/sync/clean/unlock` in place of old scripts.
5. Keep old scripts as fallback during transition period.

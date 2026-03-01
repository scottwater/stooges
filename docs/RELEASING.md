# Releasing

## Preconditions
- All tests pass: `go test ./...`
- Smoke tests pass: `scripts/smoke_test.sh`
- Docs up to date (`README`, `COMMANDS`, `MIGRATION`)

## Tag Release

```bash
# bump internal/version/version.go first (example: 0.76 -> 0.77)
git tag v0.1.0
git push origin v0.1.0
```

## GitHub Actions
`release.yml` runs on tags and publishes binary artifacts for macOS/Linux.

## Post-Release
- Verify workflow artifacts exist.
- Validate `go install github.com/scottwater/stooges/cmd/stooges@latest` from clean shell.

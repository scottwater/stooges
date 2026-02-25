#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$root_dir"

echo "[smoke] go test ./..."
go test ./...

echo "[smoke] build binary"
go build -o /tmp/stooges-smoke ./cmd/stooges

echo "[smoke] doctor --json"
/tmp/stooges-smoke doctor --json >/tmp/stooges-doctor.json || true

echo "[smoke] command help"
/tmp/stooges-smoke --help >/tmp/stooges-help.txt

rm -f /tmp/stooges-smoke /tmp/stooges-doctor.json /tmp/stooges-help.txt

echo "[smoke] done"

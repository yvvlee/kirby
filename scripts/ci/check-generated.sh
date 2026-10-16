#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-generated.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT INT TERM

snapshot() {
  (
    cd "$repo_dir"
    find server/api -type f -name '*.go'
    find server/internal/model -type f -name '*_lorm_gen.go'
    printf '%s\n' server/cmd/kirby/wire_gen.go
  ) | LC_ALL=C sort | while IFS= read -r path; do
    (cd "$repo_dir" && shasum -a 256 "$path")
  done
}

# Compare actual inputs and outputs, so this also works before a local commit.
snapshot > "$work_dir/before"
(cd "$repo_dir/server" && make generate)
snapshot > "$work_dir/after"
if ! diff -u "$work_dir/before" "$work_dir/after"; then
  echo "generated protobuf, lorm, or Wire source is not current" >&2
  exit 1
fi
echo "Generated source is current."

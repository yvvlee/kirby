#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

(cd "$repo_dir/server" && make generate)
changes=$(git -C "$repo_dir" status --porcelain -- api server/gen)
if [ -n "$changes" ]; then
  printf '%s\n' "$changes" >&2
  echo "generated protobuf source is not current" >&2
  exit 1
fi
echo "Generated source is current."

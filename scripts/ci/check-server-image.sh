#!/bin/sh
set -eu

image=${1:?server image is required}
container="kirby-server-check-$PPID-$$"
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-server-image.XXXXXX")

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

test "$(docker image inspect "$image" --format '{{.Config.User}}')" = "65532:65532"
docker image inspect "$image" --format '{{json .Config.Cmd}}' | grep -q -- '"--web-root","/srv"'
docker create --name "$container" "$image" >/dev/null
docker cp "$container:/srv/index.html" "$work_dir/index.html"
grep -q '<div id="root"></div>' "$work_dir/index.html"
docker cp "$container:/srv/assets" "$work_dir/assets"
test -n "$(find "$work_dir/assets" -type f -print -quit)"
echo "Server image user, command, and web assets passed."

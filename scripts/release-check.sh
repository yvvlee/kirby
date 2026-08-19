#!/bin/sh
set -eu

usage() {
  echo "usage: scripts/release-check.sh [--dry-run] vX.Y.Z" >&2
  exit 2
}

dry_run=false
if [ "${1:-}" = "--dry-run" ]; then
  dry_run=true
  shift
fi
[ "$#" -eq 1 ] || usage
tag=$1

if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)[.](0|[1-9][0-9]*)[.](0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
  echo "release tag must match vX.Y.Z with an optional prerelease suffix" >&2
  exit 1
fi

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
release_dir="$repo_dir/dist/release"
head_commit=$(git -C "$repo_dir" rev-parse HEAD)
version=${tag#v}
build_date=$(git -C "$repo_dir" show -s --format=%cI "$head_commit")
go_image='golang:1.25.13-bookworm@sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48'

for command_name in cmp docker git grep shasum; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "$command_name is required" >&2
    exit 1
  fi
done

changes=$(git -C "$repo_dir" status --porcelain --untracked-files=normal)
if [ -n "$changes" ]; then
  printf '%s\n' "$changes" >&2
  echo "release requires a clean worktree" >&2
  exit 1
fi

if git -C "$repo_dir" show-ref --verify --quiet "refs/tags/$tag"; then
  if [ "$(git -C "$repo_dir" cat-file -t "refs/tags/$tag")" != tag ]; then
    echo "release tag must be an annotated tag" >&2
    exit 1
  fi
  tagged_commit=$(git -C "$repo_dir" rev-parse "$tag^{}")
  if [ "$tagged_commit" != "$head_commit" ]; then
    echo "release tag $tag does not point to HEAD $head_commit" >&2
    exit 1
  fi
  if [ "$dry_run" = false ]; then
    git -C "$repo_dir" verify-tag "$tag"
  fi
elif [ "$dry_run" = false ]; then
  echo "release tag $tag does not exist" >&2
  exit 1
fi

"$repo_dir/scripts/ci/check-generated.sh"
if [ -n "$(git -C "$repo_dir" status --porcelain -- server/api)" ]; then
  echo "generated source changed during the release check" >&2
  exit 1
fi

rm -rf "$release_dir"
mkdir -p "$release_dir"

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e "KIRBY_VERSION=$version" \
  -e "KIRBY_COMMIT=$head_commit" \
  -e "KIRBY_BUILD_DATE=$build_date" \
  -e "KIRBY_GO_PROXY=${KIRBY_GO_PROXY:-https://proxy.golang.org,direct}" \
  -v "$repo_dir:/src:ro" \
  -v "$release_dir:/out" \
  "$go_image" /bin/sh -ec '
    export GOMODCACHE=/tmp/go-mod GOCACHE=/tmp/go-build GOPROXY="$KIRBY_GO_PROXY"
    cd /src/server
    ldflags="-s -w -X github.com/yvvlee/kirby/server/internal/version.Version=$KIRBY_VERSION -X github.com/yvvlee/kirby/server/internal/version.Commit=$KIRBY_COMMIT -X github.com/yvvlee/kirby/server/internal/version.BuildDate=$KIRBY_BUILD_DATE"
    for arch in amd64 arm64; do
      output="/out/kirby-v${KIRBY_VERSION}-linux-${arch}"
      CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOAMD64=v1 \
        go build -buildvcs=false -trimpath -ldflags "$ldflags" -o "$output" .
      CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOAMD64=v1 \
        go build -buildvcs=false -trimpath -ldflags "$ldflags" -o "$output.verify" .
      cmp "$output" "$output.verify"
      rm "$output.verify"
    done
  '

(cd "$release_dir" && shasum -a 256 "kirby-$tag-linux-amd64" "kirby-$tag-linux-arm64" > SHA256SUMS)
test -s "$release_dir/kirby-$tag-linux-amd64"
test -s "$release_dir/kirby-$tag-linux-arm64"
test -s "$release_dir/SHA256SUMS"

echo "Release artifacts are reproducible for $tag at $head_commit."
echo "Output: $release_dir"

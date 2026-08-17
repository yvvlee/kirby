#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-clean-room.XXXXXX")
checkout_dir="$work_dir/kirby"
builder_name="kirby-clean-$PPID-$$"
server_image="kirby-clean-server:$PPID-$$"
web_image="kirby-clean-web:$PPID-$$"

node_version=$(node -p 'process.versions.node' 2>/dev/null || true)
node_major=$(printf '%s' "$node_version" | cut -d. -f1)
node_minor=$(printf '%s' "$node_version" | cut -d. -f2)
if [ -z "$node_major" ] || [ "$node_major" -lt 20 ] || { [ "$node_major" -eq 20 ] && [ "$node_minor" -lt 19 ]; }; then
  echo "Node.js 20.19 or newer is required; found ${node_version:-unavailable}" >&2
  exit 1
fi

cleanup() {
  docker image rm "$server_image" "$web_image" >/dev/null 2>&1 || true
  docker buildx rm --force "$builder_name" >/dev/null 2>&1 || true
  chmod -R u+w "$work_dir" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

mkdir -p "$checkout_dir" "$work_dir/go-mod" "$work_dir/go-build" "$work_dir/npm"
git -C "$repo_dir" checkout-index --all --prefix="$checkout_dir/"

test ! -e "$checkout_dir/dashboard"
test ! -e "$checkout_dir/old"
if grep -R -n -E '/(dashboard|old)(/|$)' \
  --exclude=source-provenance.md --exclude=test-clean-room.sh "$checkout_dir"; then
  echo "clean-room checkout references an excluded source directory" >&2
  exit 1
fi

export GOMODCACHE="$work_dir/go-mod"
export GOCACHE="$work_dir/go-build"
export GOPROXY="https://proxy.golang.org,direct"
export npm_config_cache="$work_dir/npm"
export npm_config_fetch_retries=5
export npm_config_fetch_retry_mintimeout=1000
export npm_config_fetch_retry_maxtimeout=10000
export npm_config_fetch_timeout=300000
if [ -n "${KIRBY_NPM_REGISTRY:-}" ]; then
  export npm_config_registry="$KIRBY_NPM_REGISTRY"
  export npm_config_replace_registry_host=always
fi

(cd "$checkout_dir/server" && go mod download && go test ./... && go vet ./... && go build -trimpath -o "$work_dir/kirby-server" .)
(cd "$checkout_dir/web" && npm ci && npm run lint && npm run test -- --run && npm run build)

docker buildx create --name "$builder_name" --driver docker-container >/dev/null
docker buildx inspect --builder "$builder_name" --bootstrap >/dev/null
docker buildx build --builder "$builder_name" --pull --no-cache --load -t "$server_image" -f "$checkout_dir/server/Dockerfile" "$checkout_dir"
docker buildx build --builder "$builder_name" --pull --no-cache --load -t "$web_image" -f "$checkout_dir/web/Dockerfile" "$checkout_dir"

test "$(docker image inspect "$server_image" --format '{{.Config.User}}')" = "65532:65532"
test "$(docker image inspect "$web_image" --format '{{.Config.User}}')" = "65532:65532"
echo "Clean-room build passed using only Git-tracked files and temporary caches."

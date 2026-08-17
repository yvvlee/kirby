#!/bin/sh
set -eu

image=${1:?web image is required}
container="kirby-web-check-$PPID-$$"

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker run -d --name "$container" --publish 127.0.0.1::8080 "$image" >/dev/null
attempt=0
until docker exec "$container" /kirby-web healthcheck; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done
port=$(docker port "$container" 8080/tcp | sed 's/.*://')
curl -fsS "http://127.0.0.1:$port/projects/example" | grep -q '<div id="app"></div>'
echo "Web image health and SPA fallback passed."

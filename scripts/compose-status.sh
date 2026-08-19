#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(dirname -- "$script_dir")
deploy_dir="$repo_dir/deploy"
env_file=${KIRBY_COMPOSE_ENV_FILE:-"$deploy_dir/.env"}
compose_file="$deploy_dir/docker-compose.yml"

if [ ! -f "$env_file" ]; then
  echo "missing Compose environment file: $env_file" >&2
  exit 1
fi

compose() {
  docker compose --env-file "$env_file" --project-directory "$deploy_dir" -f "$compose_file" "$@"
}

compose ps
compose exec -T server /kirby healthcheck

http_port=$(compose port server 8080 | sed 's/.*://')
curl --fail --silent --show-error "http://127.0.0.1:$http_port/healthz" >/dev/null
curl --fail --silent --show-error "http://127.0.0.1:$http_port/login" | grep -q '<div id="root"></div>'
echo "Kirby API and web application are healthy."

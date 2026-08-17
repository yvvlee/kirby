#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(dirname -- "$script_dir")
deploy_dir="$repo_dir/deploy"
env_file=${KIRBY_COMPOSE_ENV_FILE:-"$deploy_dir/.env"}

if [ ! -f "$env_file" ]; then
  echo "missing Compose environment file: $env_file" >&2
  exit 1
fi

docker compose \
  --env-file "$env_file" \
  --project-directory "$deploy_dir" \
  -f "$deploy_dir/docker-compose.yml" \
  down --remove-orphans

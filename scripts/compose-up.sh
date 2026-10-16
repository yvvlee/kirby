#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(dirname -- "$script_dir")
deploy_dir="$repo_dir/deploy"
env_file=${KIRBY_COMPOSE_ENV_FILE:-"$deploy_dir/.env"}
compose_file="$deploy_dir/docker-compose.yml"

if [ ! -f "$env_file" ]; then
  echo "missing Compose environment file: $env_file" >&2
  echo "create it from deploy/.env.example and replace every CHANGE_ME value" >&2
  exit 1
fi

compose() {
  docker compose --env-file "$env_file" --project-directory "$deploy_dir" -f "$compose_file" "$@"
}

compose config --quiet
compose up -d --build --wait --wait-timeout 180 mysql redis minio
compose --profile init run --rm minio-init

compose up -d --build --wait --wait-timeout 180
http_port=$(compose port server 8080 | sed 's/.*://')
grpc_port=$(compose port server 9090 | sed 's/.*://')
echo "Kirby HTTP is ready at http://localhost:$http_port"
echo "Kirby runtime gRPC is ready at localhost:$grpc_port"

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
compose up -d --wait --wait-timeout 180 mysql redis minio
compose --profile init run --rm minio-init

schema_count=$(compose exec -T mysql /bin/sh -ec \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql --host=127.0.0.1 --user="$MYSQL_USER" --database="$MYSQL_DATABASE" --batch --skip-column-names --execute="SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('\''environments'\'', '\''users'\'', '\''roles'\'', '\''permissions'\'', '\''user_environment_roles'\'', '\''role_permissions'\'', '\''refresh_tokens'\'', '\''projects'\'', '\''project_api_keys'\'', '\''configs'\'', '\''structures'\'', '\''config_enums'\'', '\''snapshots'\'', '\''import_records'\'', '\''audit_logs'\'')"')

if [ "$schema_count" != "15" ]; then
  echo "Kirby database schema is absent or incomplete. Automatic schema creation is disabled." >&2
  echo "Run this command once, then run scripts/compose-up.sh again:" >&2
  echo "docker compose --env-file \"$env_file\" --project-directory \"$deploy_dir\" -f \"$compose_file\" exec -T mysql /bin/sh -ec 'MYSQL_PWD=\"\$MYSQL_PASSWORD\" mysql --user=\"\$MYSQL_USER\" --database=\"\$MYSQL_DATABASE\"' < \"$deploy_dir/schema.sql\"" >&2
  exit 1
fi

compose up -d --build --wait --wait-timeout 180
http_port=$(compose port server 8080 | sed 's/.*://')
grpc_port=$(compose port server 9090 | sed 's/.*://')
echo "Kirby HTTP is ready at http://localhost:$http_port"
echo "Kirby runtime gRPC is ready at localhost:$grpc_port"

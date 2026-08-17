#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-ci-integration.XXXXXX")
config_file="$work_dir/config.yaml"
password_file="$work_dir/admin-password"
backend_log="$work_dir/backend.log"
frontend_log="$work_dir/frontend.log"
backend_pid=
frontend_pid=
http_port=${KIRBY_CI_HTTP_PORT:-18080}
grpc_port=${KIRBY_CI_GRPC_PORT:-19090}
web_port=${KIRBY_CI_WEB_PORT:-14173}
mysql_dsn=${KIRBY_CI_MYSQL_DSN:-root:kirby-ci-mysql-password@tcp(127.0.0.1:3306)/kirby?charset=utf8mb4&parseTime=true&loc=UTC}
redis_address=${KIRBY_CI_REDIS_ADDRESS:-127.0.0.1:6379}
s3_endpoint=${KIRBY_CI_S3_ENDPOINT:-127.0.0.1:9000}
s3_access_key=${KIRBY_CI_S3_ACCESS_KEY:-kirby-ci-minio}
s3_secret_key=${KIRBY_CI_S3_SECRET_KEY:-kirby-ci-minio-password}
s3_public_base_url=${KIRBY_CI_S3_PUBLIC_BASE_URL:-http://127.0.0.1:9000/kirby}

cleanup() {
  exit_status=$?
  trap - EXIT INT TERM
  if [ -n "$frontend_pid" ]; then
    pkill -TERM -f "node .*vite.*--port $web_port" >/dev/null 2>&1 || true
    kill "$frontend_pid" >/dev/null 2>&1 || true
  fi
  if [ -n "$backend_pid" ]; then kill "$backend_pid" >/dev/null 2>&1 || true; fi
  if [ -n "$frontend_pid" ]; then wait "$frontend_pid" >/dev/null 2>&1 || true; fi
  if [ -n "$backend_pid" ]; then wait "$backend_pid" >/dev/null 2>&1 || true; fi
  if [ "$exit_status" -ne 0 ]; then
    sed -n '1,240p' "$backend_log" >&2 || true
    sed -n '1,240p' "$frontend_log" >&2 || true
  fi
  rm -rf "$work_dir"
  exit "$exit_status"
}
trap cleanup EXIT INT TERM

umask 077
cat > "$config_file" <<EOF
mode: single
http:
  address: "127.0.0.1:$http_port"
  timeout: 2m
grpc:
  address: "127.0.0.1:$grpc_port"
  timeout: 10s
mysql:
  dsn: "$mysql_dsn"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 5m
cache:
  driver: redis
  redis:
    address: "$redis_address"
    username: ""
    password: ""
    db: 0
    key_prefix: "kirby-ci:"
jwt:
  issuer: kirby-e2e
  active_kid: e2e-primary
  access_ttl: 15m
  refresh_ttl: 168h
  keys:
    e2e-primary: "kirby-e2e-jwt-key-012345678901234567890123"
security:
  api_key_pepper: "kirby-ci-api-key-pepper-012345678901234567890"
  allowed_origins:
    - "http://127.0.0.1:$web_port"
  trusted_proxies: []
object_storage:
  driver: s3
  local:
    directory: ""
  s3:
    endpoint: "$s3_endpoint"
    presign_endpoint: "$s3_endpoint"
    region: "us-east-1"
    bucket: "kirby"
    access_key: "$s3_access_key"
    secret_key: "$s3_secret_key"
    use_ssl: false
    presign_use_ssl: false
    public_base_url: "$s3_public_base_url"
log:
  level: info
  format: json
EOF
printf '%s\n' 'kirby-e2e-admin-password' > "$password_file"
chmod 600 "$password_file"

(cd "$repo_dir/server" && go build -trimpath -o "$work_dir/kirby" .)
"$work_dir/kirby" create-admin --config "$config_file" --username admin \
  --display-name "CI Admin" --password-file "$password_file"
"$work_dir/kirby" serve --config "$config_file" > "$backend_log" 2>&1 &
backend_pid=$!
KIRBY_DEV_API_TARGET="http://127.0.0.1:$http_port" \
  npm --prefix "$repo_dir/web" run dev -- --host 127.0.0.1 --port "$web_port" \
  > "$frontend_log" 2>&1 &
frontend_pid=$!

wait_for_url() {
  url=$1
  description=$2
  attempt=0
  until curl -fsS "$url" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 120 ]; then
      echo "timed out waiting for $description" >&2
      return 1
    fi
    sleep 0.5
  done
}

wait_for_url "http://127.0.0.1:$http_port/readyz" "Kirby server"
wait_for_url "http://127.0.0.1:$web_port/login" "Vite development server"
(cd "$repo_dir/web" && KIRBY_LOCAL_WEB_URL="http://127.0.0.1:$web_port" \
  npx playwright test --config tests/e2e/playwright.local.config.mjs)
echo "Single-server MySQL, Redis, MinIO, Vite, and browser integration passed."

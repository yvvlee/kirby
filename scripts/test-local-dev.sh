#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-local-dev.XXXXXX")
mysql_container=${KIRBY_TEST_MYSQL_CONTAINER:-mysql}
redis_container=${KIRBY_TEST_REDIS_CONTAINER:-redis}
mysql_port=${KIRBY_TEST_MYSQL_PORT:-3306}
redis_port=${KIRBY_TEST_REDIS_PORT:-6379}
http_port=${KIRBY_TEST_HTTP_PORT:-18080}
grpc_port=${KIRBY_TEST_GRPC_PORT:-19090}
web_port=${KIRBY_TEST_WEB_PORT:-15173}
test_id="${PPID}$$"
database="kirby_local_${test_id}"
database_user="kirby_${test_id}"
database_password="kirby-local-${test_id}-database-password"
config_file="$work_dir/config.yaml"
password_file="$work_dir/admin-password"
binary_file="$work_dir/kirby"
backend_log="$work_dir/backend.log"
frontend_log="$work_dir/frontend.log"
backend_pid=
frontend_pid=

cleanup() {
  exit_status=$?
  trap - EXIT INT TERM
  if [ -n "$frontend_pid" ]; then
    pkill -TERM -f "node .*vite.*--port $web_port" >/dev/null 2>&1 || true
    kill "$frontend_pid" >/dev/null 2>&1 || true
  fi
  if [ -n "$backend_pid" ]; then kill "$backend_pid" >/dev/null 2>&1 || true; fi
  attempts=0
  while { [ -n "$frontend_pid" ] && kill -0 "$frontend_pid" >/dev/null 2>&1; } || \
        { [ -n "$backend_pid" ] && kill -0 "$backend_pid" >/dev/null 2>&1; }; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 50 ]; then
      if [ -n "$frontend_pid" ]; then kill -KILL "$frontend_pid" >/dev/null 2>&1 || true; fi
      if [ -n "$backend_pid" ]; then kill -KILL "$backend_pid" >/dev/null 2>&1 || true; fi
      break
    fi
    sleep 0.1
  done
  if [ -n "$frontend_pid" ]; then wait "$frontend_pid" >/dev/null 2>&1 || true; fi
  if [ -n "$backend_pid" ]; then wait "$backend_pid" >/dev/null 2>&1 || true; fi
  docker exec -i "$mysql_container" sh -ec \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot' <<SQL >/dev/null 2>&1 || true
DROP DATABASE IF EXISTS \`$database\`;
DROP USER IF EXISTS '$database_user'@'%';
SQL
  if [ "$exit_status" -ne 0 ]; then
    sed -n '1,200p' "$backend_log" >&2 || true
    sed -n '1,200p' "$frontend_log" >&2 || true
  fi
  rm -rf "$work_dir"
  exit "$exit_status"
}
trap cleanup EXIT INT TERM

node_version=$(node -p 'process.versions.node' 2>/dev/null || true)
if [ "$node_version" != "24.19.0" ]; then
  echo "Node.js 24.19.0 is required; found ${node_version:-unavailable}" >&2
  exit 1
fi

docker inspect "$mysql_container" >/dev/null
docker inspect "$redis_container" >/dev/null
test "$(docker inspect "$mysql_container" --format '{{.State.Running}}')" = true
test "$(docker inspect "$redis_container" --format '{{.State.Running}}')" = true
docker exec "$redis_container" redis-cli ping | grep -qx PONG

umask 077
docker exec -i "$mysql_container" sh -ec \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot' <<SQL
CREATE DATABASE \`$database\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER '$database_user'@'%' IDENTIFIED BY '$database_password';
GRANT ALL PRIVILEGES ON \`$database\`.* TO '$database_user'@'%';
SQL
docker exec -i "$mysql_container" sh -ec \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot "$1"' sh "$database" \
  < "$repo_dir/deploy/schema.sql"

cat >"$config_file" <<EOF
mode: single
http:
  address: "127.0.0.1:$http_port"
  timeout: 2m
grpc:
  address: "127.0.0.1:$grpc_port"
  timeout: 10s
mysql:
  dsn: "$database_user:$database_password@tcp(127.0.0.1:$mysql_port)/$database?charset=utf8mb4&parseTime=true&loc=UTC"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 5m
cache:
  driver: redis
  redis:
    address: "127.0.0.1:$redis_port"
    username: ""
    password: ""
    db: 0
    key_prefix: "kirby-local-$test_id:"
jwt:
  issuer: kirby-e2e
  active_kid: e2e-primary
  access_ttl: 15m
  refresh_ttl: 168h
  keys:
    e2e-primary: "kirby-e2e-jwt-key-012345678901234567890123"
security:
  api_key_pepper: "kirby-local-api-key-pepper-01234567890123456789"
  allowed_origins:
    - "http://127.0.0.1:$web_port"
    - "http://127.0.0.1:14173"
  trusted_proxies: []
object_storage:
  driver: local
  local:
    directory: "$work_dir/objects"
  s3:
    endpoint: ""
    presign_endpoint: ""
    region: ""
    bucket: ""
    access_key: ""
    secret_key: ""
    use_ssl: false
    presign_use_ssl: false
    public_base_url: ""
log:
  level: info
  format: json
EOF
printf '%s\n' 'kirby-e2e-admin-password' >"$password_file"
chmod 600 "$password_file"

(cd "$repo_dir/server" && go build -o "$binary_file" .)
"$binary_file" create-admin --config "$config_file" --username admin \
  --display-name "Local Admin" --password-file "$password_file"
"$binary_file" serve --config "$config_file" >"$backend_log" 2>&1 &
backend_pid=$!

KIRBY_DEV_API_TARGET="http://127.0.0.1:$http_port" \
  npm --prefix "$repo_dir/web" run dev -- --host 127.0.0.1 --port "$web_port" \
  >"$frontend_log" 2>&1 &
frontend_pid=$!

wait_for_url() {
  url=$1
  description=$2
  attempts=0
  until curl --fail --silent --show-error "$url" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 120 ]; then
      echo "timed out waiting for $description" >&2
      return 1
    fi
    sleep 0.5
  done
}

wait_for_url "http://127.0.0.1:$http_port/readyz" "Kirby backend"
wait_for_url "http://127.0.0.1:$web_port/login" "Vite dev server"

(cd "$repo_dir/web" && KIRBY_LOCAL_WEB_URL="http://127.0.0.1:$web_port" \
  npx playwright test --config tests/e2e/playwright.local.config.mjs)
echo "Local Vite, MySQL, Redis, backend, and browser journey passed."

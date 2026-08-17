#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-security.XXXXXX")
server_image="kirby-security-server:$PPID-$$"
web_image="kirby-security-web:$PPID-$$"
govuln_version=v1.1.4
gosec_version=v2.22.7
trivy_image="aquasec/trivy@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08"
trivy_db_image="${KIRBY_TRIVY_DB_IMAGE:-mirror.gcr.io/aquasec/trivy-db:2}"

cleanup() {
  docker image rm "$server_image" "$web_image" >/dev/null 2>&1 || true
  chmod -R u+w "$work_dir" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

for command_name in curl docker go grep gzip jq npm sort tar; do
  require_command "$command_name"
done

download_govuln_db() {
  database_dir="$work_dir/govulndb"
  mkdir -p "$database_dir/index" "$database_dir/ID"
  curl -fsS --retry 3 --retry-all-errors --max-time 60 \
    https://vuln.go.dev/index/modules.json.gz -o "$database_dir/index/modules.json.gz"
  curl -fsS --retry 3 --retry-all-errors --max-time 60 \
    https://vuln.go.dev/index/db.json.gz -o "$database_dir/index/db.json.gz"
  gzip -dc "$database_dir/index/modules.json.gz" > "$database_dir/index/modules.json"
  gzip -dc "$database_dir/index/db.json.gz" > "$database_dir/index/db.json"

  module_json=$(cd "$repo_dir/server" && go list -m -json all | jq -cs '[.[].Path] + ["stdlib", "toolchain"]')
  jq -r --argjson modules "$module_json" \
    '.[] | select(.path as $module_path | $modules | index($module_path)) | .vulns[].id' \
    "$database_dir/index/modules.json" | sort -u > "$database_dir/ids"
  while IFS= read -r vulnerability_id; do
    curl -fsS --retry 3 --retry-all-errors --max-time 60 \
      "https://vuln.go.dev/ID/$vulnerability_id.json.gz" \
      -o "$database_dir/ID/$vulnerability_id.json.gz"
    gzip -dc "$database_dir/ID/$vulnerability_id.json.gz" \
      > "$database_dir/ID/$vulnerability_id.json"
  done < "$database_dir/ids"
  printf '%s\n' "$database_dir"
}

prepare_trivy_db() {
  attempt=1
  while ! docker pull "$trivy_db_image"; do
    if [ "$attempt" -ge 3 ]; then
      echo "failed to pull the Trivy vulnerability database after 3 attempts" >&2
      exit 1
    fi
    attempt=$((attempt + 1))
  done

  database_archive="$work_dir/trivy-db-image.tar"
  docker save -o "$database_archive" "$trivy_db_image"
  manifest_digest=$(tar -xOf "$database_archive" index.json | jq -er '.manifests[0].digest')
  manifest_path="blobs/sha256/${manifest_digest#sha256:}"
  layer_digest=$(tar -xOf "$database_archive" "$manifest_path" | jq -er \
    '.layers[] | select(.mediaType == "application/vnd.aquasec.trivy.db.layer.v1.tar+gzip") | .digest')
  mkdir -p "$work_dir/trivy-cache/db"
  tar -xOf "$database_archive" "blobs/sha256/${layer_digest#sha256:}" \
    | tar -xzf - -C "$work_dir/trivy-cache/db"
  test -s "$work_dir/trivy-cache/db/trivy.db"
  test -s "$work_dir/trivy-cache/db/metadata.json"
}

echo "Checking reachable Go vulnerabilities..."
govuln_db=$(download_govuln_db)
(cd "$repo_dir/server" && go run "golang.org/x/vuln/cmd/govulncheck@$govuln_version" -db "file://$govuln_db" ./...)

echo "Checking Go source security rules..."
(cd "$repo_dir/server" && go run "github.com/securego/gosec/v2/cmd/gosec@$gosec_version" \
  -quiet -exclude-generated -track-suppressions . ./cmd/... ./internal/...)

echo "Checking frontend dependencies..."
(cd "$repo_dir/web" && npm audit --audit-level=high && npm ls --all >/dev/null)

echo "Checking removed and private dependencies..."
private_source_pattern='git[.]changbaops[.]com|registry[.]changba'
removed_service_pattern='na''cos|ya''haha'
removed_type_pattern='Pri''ze|Audit''ing|Gen''der|Business''Line|Global''Structure|Global''Enum'
unsafe_schema_pattern='Sync''2|Create''Tables|Auto''Migrate'
singular_generator_pattern='protoc-gen-go-err''or([^s]|$)'
if git -C "$repo_dir" grep -n -I -E "$private_source_pattern" -- ':!docs/source-provenance.md'; then exit 1; fi
if git -C "$repo_dir" grep -n -I -E "$removed_service_pattern" -- server web api deploy scripts; then exit 1; fi
if git -C "$repo_dir" grep -n -I -E "$removed_type_pattern" -- server web api deploy scripts; then exit 1; fi
if git -C "$repo_dir" grep -n -I -E "$unsafe_schema_pattern" -- \
  server web api deploy scripts ':!scripts/check-schema.sh' ':!scripts/security-check.sh'; then exit 1; fi
if git -C "$repo_dir" grep -n -I -E "$singular_generator_pattern" -- server web api deploy scripts; then exit 1; fi
if (cd "$repo_dir/server" && go list -m all | grep -E "$private_source_pattern|$removed_service_pattern"); then exit 1; fi
if (cd "$repo_dir/server" && go list -deps ./... | grep -E "$private_source_pattern|$removed_service_pattern"); then exit 1; fi

echo "Running security-boundary tests..."
(cd "$repo_dir/server" && go test \
  ./internal/auth/... \
  ./internal/logic/asset/... \
  ./internal/logic/importer/... \
  ./internal/logic/runtime/... \
  ./internal/middleware/... \
  ./internal/observability/... \
  ./internal/permission/... \
  ./internal/repository/... \
  ./internal/storage/object/...)

echo "Building release images for scanning..."
if [ -n "${KIRBY_GO_PROXY:-}" ]; then
  docker build --build-arg "GOPROXY=$KIRBY_GO_PROXY" -t "$server_image" -f "$repo_dir/server/Dockerfile" "$repo_dir"
else
  docker build -t "$server_image" -f "$repo_dir/server/Dockerfile" "$repo_dir"
fi
docker build -t "$web_image" -f "$repo_dir/web/Dockerfile" "$repo_dir"
docker save -o "$work_dir/server.tar" "$server_image"
docker save -o "$work_dir/web.tar" "$web_image"

echo "Preparing the current Trivy database..."
prepare_trivy_db

echo "Scanning release images for high and critical vulnerabilities..."
for archive in server web; do
  docker run --rm -v "$work_dir:/work" "$trivy_image" image \
    --skip-db-update --cache-dir /work/trivy-cache --no-progress --scanners vuln \
    --severity HIGH,CRITICAL --exit-code 1 --input "/work/$archive.tar"
done

echo "Scanning the source tree for secrets..."
docker run --rm -v "$repo_dir:/src:ro" "$trivy_image" fs \
  --no-progress --scanners secret --exit-code 1 \
  --skip-dirs /src/.git --skip-dirs /src/web/node_modules --skip-dirs /src/web/dist /src

echo "Security checks passed."

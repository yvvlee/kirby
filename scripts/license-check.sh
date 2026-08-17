#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-license.XXXXXX")
go_licenses_version=v1.6.0
go_licenses_sum='h1:MM+VCXf0slYkpWO0mECvdYDVCxZXIQNal5wqUIXEZ/A='
license_checker_version=25.0.1
license_checker_integrity='sha512-mET5AIwl7MR2IAKYYoVBBpV0OnkKQ1xGj2IMMeEFIs42QAkEVjRtFZGWmQ28WeU7MP779iAgOaOy93Mn44mn6g=='

cleanup() {
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

for command_name in git go grep jq npm; do
  require_command "$command_name"
done

test -s "$repo_dir/LICENSE"
test -s "$repo_dir/NOTICE"
grep -q '^MIT License$' "$repo_dir/LICENSE"
grep -q 'may be released under the MIT license' "$repo_dir/docs/source-provenance.md"

echo "Verifying Go module checksums and licenses..."
(cd "$repo_dir/server" && go mod verify)
downloaded_sum=$(cd "$repo_dir/server" && go mod download -json "github.com/google/go-licenses@$go_licenses_version" | jq -er '.Sum')
if [ "$downloaded_sum" != "$go_licenses_sum" ]; then
  echo "unexpected go-licenses module checksum: $downloaded_sum" >&2
  exit 1
fi
(cd "$repo_dir/server" && go run "github.com/google/go-licenses@$go_licenses_version" check \
  --include_tests --confidence_threshold 0.8 \
  --ignore github.com/yvvlee/kirby/server ./...)

verify_generator() {
  name=$1
  tag=$2
  commit=$3
  directory="$repo_dir/server/tools/third_party/$name"
  test -s "$directory/LICENSE"
  test -s "$directory/SOURCE"
  grep -q '^MIT License$' "$directory/LICENSE"
  grep -q "^Source: https://github.com/yvvlee/$name$" "$directory/SOURCE"
  grep -q "^Tag: $tag$" "$directory/SOURCE"
  grep -q "^Commit: $commit$" "$directory/SOURCE"
  grep -q '^License: MIT (see LICENSE)$' "$directory/SOURCE"
  test "$(sed -n '1s/^module //p' "$directory/go.mod")" = "github.com/yvvlee/$name"
  (cd "$directory" && GOWORK=off go run "github.com/google/go-licenses@$go_licenses_version" check \
    --include_tests --confidence_threshold 0.8 --ignore "github.com/yvvlee/$name" ./...)
}

echo "Verifying redistributed generators..."
verify_generator protoc-gen-go-errors v1.0.0 04802e1d2794f6fc2e8a2d0d9d770eb32f8600cd
grep -q 'const release = "v1.0.0"' "$repo_dir/server/tools/third_party/protoc-gen-go-errors/version.go"
verify_generator protoc-gen-go-json v1.0.1 217c673080d2518111c3b3330a7647803058b865

echo "Verifying npm package licenses..."
test -d "$repo_dir/web/node_modules"
(cd "$repo_dir/web" && npm ls --all >/dev/null)
actual_integrity=$(npm view "license-checker@$license_checker_version" dist.integrity)
if [ "$actual_integrity" != "$license_checker_integrity" ]; then
  echo "unexpected license-checker package integrity: $actual_integrity" >&2
  exit 1
fi
(cd "$repo_dir/web" && npm exec --yes --package="license-checker@$license_checker_version" -- \
  license-checker --json --start .) > "$work_dir/npm-licenses.json"
jq -e '
  [to_entries[]
    | select(.key != "@kirby/web@0.1.0")
    | select(.value.licenses as $license
      | (["0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause",
               "BlueOak-1.0.0", "ISC", "MIT", "MIT*", "Python-2.0",
               "(MIT OR CC0-1.0)", "(MPL-2.0 OR Apache-2.0)"]
        | index($license)) == null)]
  | length == 0
' "$work_dir/npm-licenses.json" >/dev/null || {
  echo "unknown or forbidden npm license:" >&2
  jq -r 'to_entries[] | [.key, (.value.licenses // "UNKNOWN")] | @tsv' "$work_dir/npm-licenses.json" >&2
  exit 1
}
jq -r 'to_entries[] | select(.value.licenses == "MIT*") | .value.licenseFile' \
  "$work_dir/npm-licenses.json" | while IFS= read -r license_file; do
  test -s "$license_file"
  grep -q 'Permission is hereby granted' "$license_file"
done

echo "Verifying release stages contain no operating-system packages..."
for dockerfile in "$repo_dir/server/Dockerfile" "$repo_dir/web/Dockerfile"; do
  final_stage=$(awk '/^FROM / { line=$0 } END { print line }' "$dockerfile")
  if [ "$final_stage" != "FROM scratch" ]; then
    echo "$dockerfile final stage is not scratch" >&2
    exit 1
  fi
done

echo "License checks passed."

#!/bin/sh
set -eu

version=35.1
checksum=6930ebf62bd4ea607b98fff052596c6ee564b9835b4ce172c75a3f53ae9d91b7
install_root=${RUNNER_TEMP:?RUNNER_TEMP is required}/protoc-$version
archive="$install_root/protoc.zip"

test "$(uname -s)" = Linux
test "$(uname -m)" = x86_64
mkdir -p "$install_root"
curl -fsSL --retry 3 --retry-all-errors --max-time 120 \
  "https://github.com/protocolbuffers/protobuf/releases/download/v$version/protoc-$version-linux-x86_64.zip" \
  -o "$archive"
actual=$(sha256sum "$archive" | awk '{print $1}')
if [ "$actual" != "$checksum" ]; then
  echo "unexpected protoc archive checksum: $actual" >&2
  exit 1
fi
unzip -q "$archive" -d "$install_root"
test "$("$install_root/bin/protoc" --version)" = "libprotoc $version"
printf '%s\n' "$install_root/bin" >> "${GITHUB_PATH:?GITHUB_PATH is required}"

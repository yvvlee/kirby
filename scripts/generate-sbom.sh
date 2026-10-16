#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
source_mode=commit
case "${1:-}" in
  '') ;;
  --working-tree) source_mode=working-tree; shift ;;
  --help|-h)
    echo "Usage: $0 [--working-tree]"
    echo "Default: archive KIRBY_SOURCE_COMMIT (HEAD when unset)."
    echo "--working-tree: snapshot tracked and unignored files without changing the index."
    exit 0
    ;;
  *) echo "Unknown option: $1" >&2; exit 1 ;;
esac
if [ "$#" -ne 0 ]; then
  echo "Unexpected arguments: $*" >&2
  exit 1
fi
if [ "$source_mode" = working-tree ] && [ -n "${KIRBY_SOURCE_COMMIT:-}" ]; then
  echo "--working-tree uses HEAD as its base; unset KIRBY_SOURCE_COMMIT." >&2
  exit 1
fi
release_version=${KIRBY_RELEASE_VERSION:-dev}
build_date=${KIRBY_BUILD_DATE:-unknown}
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-sbom.XXXXXX")
checkout_dir="$work_dir/source"
server_image="kirby-sbom:$PPID-$$"
trivy_image='aquasec/trivy@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08'

cleanup() {
  docker image rm "$server_image" >/dev/null 2>&1 || true
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

for command_name in docker git jq shasum tar; do
  require_command "$command_name"
done

source_commit=$(git -C "$repo_dir" rev-parse "${KIRBY_SOURCE_COMMIT:-HEAD}^{commit}")
source_ref=$source_commit
source_dirty=false
worktree_sha=''
source_time=$(git -C "$repo_dir" show -s --format=%cI "$source_commit")
mkdir -p "$checkout_dir" "$work_dir/normalized"

snapshot_working_tree() {
  require_command python3
  python3 - "$repo_dir" "$checkout_dir" "$work_dir/source-metadata.json" <<'PY_SNAPSHOT'
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys

repository, destination, metadata_path = map(Path, sys.argv[1:])
# Output and local-only paths are excluded even if someone previously tracked them.
# In particular, excluding dist avoids making an SBOM hash depend on its old copy.
excluded_roots = (
    "dist", "tmp", ".local", ".idea", ".vscode", ".aws", ".ssh", ".codex",
    ".agents", "server/bin", "server/vendor",
    "web/node_modules", "web/dist", "web/coverage", "web/playwright-report",
    "web/test-results",
)
excluded_names = (".git", ".DS_Store", ".env", "config.yaml", "coverage.out")
excluded_patterns = ("*.local.yaml", "*.local.yml", "*.log", "*.swp")


def excluded(relative):
    parts = relative.split("/")
    if any(part in excluded_names for part in parts):
        return True
    if any(part.startswith(".env.") and part != ".env.example" for part in parts):
        return True
    if any(fnmatch.fnmatch(part, pattern) for part in parts for pattern in excluded_patterns):
        return True
    return any(relative == root or relative.startswith(root + "/") for root in excluded_roots)


def write_entry(relative):
    source = repository / relative
    target = destination / relative
    # A replaced parent directory must not expose files outside the source tree.
    resolved_parent = source.parent.resolve()
    resolved_repository = repository.resolve()
    if resolved_parent != resolved_repository and resolved_repository not in resolved_parent.parents:
        raise RuntimeError(f"Source parent escapes repository: {relative}")
    try:
        info = source.lstat()
    except FileNotFoundError:
        return None  # A tracked deletion must stay absent from the snapshot.
    target.parent.mkdir(parents=True, exist_ok=True)
    if stat.S_ISLNK(info.st_mode):
        data = os.fsencode(os.readlink(source))
        target.symlink_to(os.fsdecode(data))
        mode = "120000"
    elif stat.S_ISREG(info.st_mode):
        data = source.read_bytes()
        mode = "100755" if info.st_mode & stat.S_IXUSR else "100644"
        target.write_bytes(data)
        target.chmod(0o755 if mode == "100755" else 0o644)
    else:
        raise RuntimeError(f"Unsupported source entry: {relative}")
    return [relative, mode, hashlib.sha256(data).hexdigest()]


listed = subprocess.check_output([
    "git", "-C", str(repository), "ls-files", "-z", "--cached", "--others", "--exclude-standard",
])
paths = sorted(set(os.fsdecode(path) for path in listed.split(b"\0") if path))
entries = []
for relative in paths:
    if excluded(relative):
        continue
    entry = write_entry(relative)
    if entry is not None:
        entries.append(entry)
# Hash path, type/executable mode and content hash, independent of filesystem times.
manifest = json.dumps(entries, ensure_ascii=True, separators=(",", ":")).encode()
dirty = bool(subprocess.check_output([
    "git", "-C", str(repository), "status", "--porcelain", "--untracked-files=normal",
]))
metadata_path.write_text(json.dumps({
    "sha256": hashlib.sha256(manifest).hexdigest(),
    "dirty": dirty,
    "files": len(entries),
}) + "\n")
PY_SNAPSHOT
  worktree_sha=$(jq -er '.sha256' "$work_dir/source-metadata.json")
  source_dirty=$(jq -r '.dirty' "$work_dir/source-metadata.json")
  source_ref="$source_commit-worktree-$worktree_sha"
  source_time=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
}

if [ "$source_mode" = working-tree ]; then
  snapshot_working_tree
else
  git -C "$repo_dir" archive --format=tar "$source_commit" | tar -xf - -C "$checkout_dir"
fi

go_sum_sha=$(shasum -a 256 "$checkout_dir/server/go.sum" | awk '{print $1}')
npm_lock_sha=$(shasum -a 256 "$checkout_dir/web/package-lock.json" | awk '{print $1}')
echo "Building the local SBOM image from $source_ref..."
if [ -n "${KIRBY_GO_PROXY:-}" ]; then
  docker build --provenance=false --build-arg "GOPROXY=$KIRBY_GO_PROXY" \
    --build-arg "VERSION=$release_version" --build-arg "COMMIT=$source_ref" \
    --build-arg "BUILD_DATE=$build_date" \
    -t "$server_image" -f "$checkout_dir/Dockerfile" "$checkout_dir"
else
  docker build --provenance=false --build-arg "VERSION=$release_version" \
    --build-arg "COMMIT=$source_ref" --build-arg "BUILD_DATE=$build_date" \
    -t "$server_image" \
    -f "$checkout_dir/Dockerfile" "$checkout_dir"
fi
server_digest=$(docker image inspect "$server_image" --format '{{.Id}}')
docker save -o "$work_dir/server.tar" "$server_image"

echo "Generating CycloneDX inventories..."
docker run --rm -v "$checkout_dir:/src:ro" "$trivy_image" fs \
  --include-dev-deps --scanners license --format cyclonedx /src \
  > "$work_dir/source.raw.json"
docker run --rm -v "$checkout_dir/web:/src:ro" "$trivy_image" fs \
  --include-dev-deps --scanners license --format cyclonedx /src \
  > "$work_dir/web.raw.json"
docker run --rm -v "$work_dir:/work:ro" "$trivy_image" image \
  --scanners license --format cyclonedx --input /work/server.tar \
  > "$work_dir/server-image.raw.json"

normalize() {
  input=$1
  output=$2
  component_name=$3
  image_digest=$4
  build_inputs=$5
  uuid_hash=$(printf '%s' "$source_ref:$component_name" | shasum -a 256 | awk '{print $1}' | cut -c1-32)
  serial=$(printf 'urn:uuid:%s-%s-%s-%s-%s' \
    "$(printf '%s' "$uuid_hash" | cut -c1-8)" \
    "$(printf '%s' "$uuid_hash" | cut -c9-12)" \
    "$(printf '%s' "$uuid_hash" | cut -c13-16)" \
    "$(printf '%s' "$uuid_hash" | cut -c17-20)" \
    "$(printf '%s' "$uuid_hash" | cut -c21-32)")
  jq -S \
    --arg serial "$serial" \
    --arg timestamp "$source_time" \
    --arg name "$component_name" \
    --arg commit "$source_commit" \
    --arg source_ref "$source_ref" \
    --arg source_mode "$source_mode" \
    --arg source_dirty "$source_dirty" \
    --arg worktree_sha "$worktree_sha" \
    --arg go_sum "$go_sum_sha" \
    --arg npm_lock "$npm_lock_sha" \
    --arg image_digest "$image_digest" \
    --arg build_inputs "$build_inputs" '
      ([{
          "old": .metadata.component["bom-ref"],
          "new": ("kirby:document:" + $name)
        }] + (.components | map({
          "old": .["bom-ref"],
          "new": (if .purl then .purl
                  else ("kirby:component:" + $name + ":" + .name + ":" + (.version // ""))
                  end)
        }))) as $references
      | def stable_reference($reference):
          (first($references[] | select(.old == $reference) | .new) // $reference);
      .serialNumber = $serial
      | .metadata.timestamp = $timestamp
      | .metadata.component["bom-ref"] = stable_reference(.metadata.component["bom-ref"])
      | .metadata.component.name = $name
      | .metadata.component.version = (if $image_digest == "" then $source_ref else $image_digest end)
      | .metadata.component.licenses = [{"license":{"id":"MIT"}}]
      | .metadata.component.properties = ((.metadata.component.properties // []) + [
          {"name":"kirby:source:mode","value":$source_mode},
          {"name":"kirby:source:reference","value":$source_ref},
          {"name":"kirby:source:dirty","value":$source_dirty},
          {"name":"kirby:source:go-sum-sha256","value":$go_sum},
          {"name":"kirby:source:npm-lock-sha256","value":$npm_lock},
          {"name":"kirby:build:inputs","value":$build_inputs}
        ] + (if $source_mode == "commit" then [
          {"name":"kirby:source:git-commit","value":$commit}
        ] else [
          {"name":"kirby:source:git-base-commit","value":$commit},
          {"name":"kirby:source:working-tree-sha256","value":$worktree_sha}
        ] end) + (if $image_digest == "" then [] else [
          {"name":"kirby:image:digest","value":$image_digest}
        ] end))
      | .components |= (map(
          .["bom-ref"] = stable_reference(.["bom-ref"])
        ) | unique_by(.["bom-ref"]) | sort_by(.["bom-ref"]))
      | .dependencies |= (map(
          .ref = stable_reference(.ref)
          | .dependsOn |= (map(stable_reference(.)) | unique | sort)
        ) | sort_by(.ref) | group_by(.ref) | map({
          "ref": .[0].ref,
          "dependsOn": ([.[].dependsOn[]] | unique | sort)
        }))
    ' "$input" > "$output"
}

normalize "$work_dir/source.raw.json" "$work_dir/normalized/source.cdx.json" \
  kirby-source "" "$source_mode snapshot, server/go.sum, web/package-lock.json"
normalize "$work_dir/web.raw.json" "$work_dir/normalized/web.cdx.json" \
  kirby-web-source "" "$source_mode snapshot, web/package-lock.json"
normalize "$work_dir/server-image.raw.json" "$work_dir/normalized/server-image.cdx.json" \
  kirby-image "$server_digest" \
  "node:24-bookworm-slim; golang:1.27.2-bookworm; scratch runtime"

for sbom in "$work_dir"/normalized/*.cdx.json; do
  test -s "$sbom"
  jq -e --arg commit "$source_commit" --arg source_mode "$source_mode" \
    --arg source_ref "$source_ref" --arg worktree_sha "$worktree_sha" '
    def property($name; $value):
      any(.metadata.component.properties[]; .name == $name and .value == $value);
    .bomFormat == "CycloneDX"
    and .specVersion == "1.6"
    and property("kirby:source:mode"; $source_mode)
    and property("kirby:source:reference"; $source_ref)
    and (if $source_mode == "commit" then property("kirby:source:git-commit"; $commit)
      else property("kirby:source:git-base-commit"; $commit)
        and property("kirby:source:working-tree-sha256"; $worktree_sha)
        and all(.metadata.component.properties[]; .name != "kirby:source:git-commit")
      end)
    and (.components | length > 0)
  ' "$sbom" >/dev/null
done

mkdir -p "$repo_dir/dist/sbom"
cp "$work_dir"/normalized/*.cdx.json "$repo_dir/dist/sbom/"
echo "SBOM generation passed for $source_ref (mode=$source_mode, dirty=$source_dirty)."

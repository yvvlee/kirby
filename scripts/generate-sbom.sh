#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
source_commit=${KIRBY_SOURCE_COMMIT:-$(git -C "$repo_dir" rev-parse HEAD)}
release_version=${KIRBY_RELEASE_VERSION:-dev}
build_date=${KIRBY_BUILD_DATE:-unknown}
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-sbom.XXXXXX")
checkout_dir="$work_dir/source"
server_image="kirby-sbom-server:$PPID-$$"
web_image="kirby-sbom-web:$PPID-$$"
trivy_image='aquasec/trivy@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08'

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

for command_name in docker git jq shasum tar; do
  require_command "$command_name"
done

git -C "$repo_dir" cat-file -e "$source_commit^{commit}"
mkdir -p "$checkout_dir" "$repo_dir/dist/sbom"
git -C "$repo_dir" archive --format=tar "$source_commit" | tar -xf - -C "$checkout_dir"

go_sum_sha=$(shasum -a 256 "$checkout_dir/server/go.sum" | awk '{print $1}')
npm_lock_sha=$(shasum -a 256 "$checkout_dir/web/package-lock.json" | awk '{print $1}')
source_time=$(git -C "$repo_dir" show -s --format=%cI "$source_commit")

echo "Building release images from $source_commit..."
if [ -n "${KIRBY_GO_PROXY:-}" ]; then
  docker build --provenance=false --build-arg "GOPROXY=$KIRBY_GO_PROXY" \
    --build-arg "VERSION=$release_version" --build-arg "COMMIT=$source_commit" \
    --build-arg "BUILD_DATE=$build_date" \
    -t "$server_image" -f "$checkout_dir/server/Dockerfile" "$checkout_dir"
else
  docker build --provenance=false --build-arg "VERSION=$release_version" \
    --build-arg "COMMIT=$source_commit" --build-arg "BUILD_DATE=$build_date" \
    -t "$server_image" \
    -f "$checkout_dir/server/Dockerfile" "$checkout_dir"
fi
docker build --provenance=false -t "$web_image" \
  -f "$checkout_dir/web/Dockerfile" "$checkout_dir"
server_digest=$(docker image inspect "$server_image" --format '{{.Id}}')
web_digest=$(docker image inspect "$web_image" --format '{{.Id}}')
docker save -o "$work_dir/server.tar" "$server_image"
docker save -o "$work_dir/web.tar" "$web_image"

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
docker run --rm -v "$work_dir:/work:ro" "$trivy_image" image \
  --scanners license --format cyclonedx --input /work/web.tar \
  > "$work_dir/web-image.raw.json"

normalize() {
  input=$1
  output=$2
  component_name=$3
  image_digest=$4
  build_inputs=$5
  uuid_hash=$(printf '%s' "$source_commit:$component_name" | shasum -a 256 | awk '{print $1}' | cut -c1-32)
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
      | .metadata.component.version = (if $image_digest == "" then $commit else $image_digest end)
      | .metadata.component.licenses = [{"license":{"id":"MIT"}}]
      | .metadata.component.properties = ((.metadata.component.properties // []) + [
          {"name":"kirby:source:git-commit","value":$commit},
          {"name":"kirby:source:go-sum-sha256","value":$go_sum},
          {"name":"kirby:source:npm-lock-sha256","value":$npm_lock},
          {"name":"kirby:build:inputs","value":$build_inputs}
        ] + (if $image_digest == "" then [] else [
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

normalize "$work_dir/source.raw.json" "$repo_dir/dist/sbom/source.cdx.json" \
  kirby-source "" "Git tree, server/go.sum, web/package-lock.json"
normalize "$work_dir/web.raw.json" "$repo_dir/dist/sbom/web.cdx.json" \
  kirby-web-source "" "Git tree, web/package-lock.json"
normalize "$work_dir/server-image.raw.json" "$repo_dir/dist/sbom/server-image.cdx.json" \
  kirby-server-image "$server_digest" \
  "golang:1.25.13-bookworm@sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48; scratch runtime"
normalize "$work_dir/web-image.raw.json" "$repo_dir/dist/sbom/web-image.cdx.json" \
  kirby-web-image "$web_digest" \
  "node:20.19.5-bookworm-slim@sha256:9e70124bd00f47dd023e349cd587132ae61892acc0e47ed641416c3e18f401c3; golang:1.25.13-bookworm@sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48; scratch runtime"

for sbom in "$repo_dir"/dist/sbom/*.cdx.json; do
  test -s "$sbom"
  jq -e --arg commit "$source_commit" '
    .bomFormat == "CycloneDX"
    and .specVersion == "1.6"
    and any(.metadata.component.properties[];
      .name == "kirby:source:git-commit" and .value == $commit)
    and (.components | length > 0)
  ' "$sbom" >/dev/null
done

echo "SBOM generation passed for $source_commit."

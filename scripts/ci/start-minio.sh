#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
minio_image='kirby-minio:local'

docker build --tag "$minio_image" "$repo_dir/deploy/minio"

docker run -d --name kirby-ci-minio --publish 127.0.0.1:9000:9000 \
  -e MINIO_ROOT_USER=kirby-ci-minio \
  -e MINIO_ROOT_PASSWORD=kirby-ci-minio-password \
  "$minio_image" server /data --address :9000 >/dev/null

attempt=0
until curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    docker logs kirby-ci-minio >&2
    exit 1
  fi
  sleep 1
done

docker run --rm --network container:kirby-ci-minio --entrypoint /bin/sh \
  --volume "$repo_dir/deploy/s3-public-read-policy.json:/policies/public-read.json:ro" \
  --volume "$repo_dir/deploy/s3-upload-lifecycle.json:/policies/upload-lifecycle.json:ro" \
  "$minio_image" -ec '
  mc alias set local http://127.0.0.1:9000 kirby-ci-minio kirby-ci-minio-password
  mc mb --ignore-existing local/kirby
  mc anonymous set-json /policies/public-read.json local/kirby
  mc ilm import local/kirby < /policies/upload-lifecycle.json
'
echo "MinIO is ready."

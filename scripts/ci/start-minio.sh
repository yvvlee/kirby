#!/bin/sh
set -eu

minio_image='quay.io/minio/minio@sha256:a1ea29fa28355559ef137d71fc570e508a214ec84ff8083e39bc5428980b015e'
mc_image='quay.io/minio/mc@sha256:aead63c77f9db9107f1696fb08ecb0faeda23729cde94b0f663edf4fe09728e3'

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

docker run --rm --network host --entrypoint /bin/sh "$mc_image" -ec '
  mc alias set local http://127.0.0.1:9000 kirby-ci-minio kirby-ci-minio-password
  mc mb --ignore-existing local/kirby
'
echo "MinIO is ready."

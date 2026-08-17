# Deployment

## Production requirements

Use MySQL 8.4, Redis 7, S3-compatible object storage, and a TLS ingress. The
backend is stateless. Run one or more identical backend instances behind the
ingress. Do not use local object storage in production.

The repository provides pinned backend and web images. The final image stages
contain only the static binaries and frontend assets. Both processes run as
user `65532:65532`.

## Database initialization

Create an empty database and execute `deploy/schema.sql` once before the first
backend starts:

```sh
mysql --host DB_HOST --user kirby --password --database kirby \
  < deploy/schema.sql
```

Kirby validates all 15 required tables at startup. It never runs migrations or
automatic table synchronization. Apply future schema changes as an explicit
operator step before starting a version that requires them.

Create the first system administrator after applying the schema:

```sh
cd server
go run . create-admin --config /etc/kirby/config.yaml \
  --username admin --display-name Administrator
```

For automation, pass `--password-file`. The file must be regular, must not be a
symbolic link, and must not grant group or other access.

## Compose example

The included Compose stack is a local deployment example with MySQL, Redis,
MinIO, two stateless backend containers, the web image, and Nginx.

```sh
cp deploy/.env.example deploy/.env
cp deploy/config.compose.yaml deploy/config.compose.local.yaml
```

Replace every `CHANGE_ME` value. Keep the values in `.env` and the YAML
consistent. Then start the dependencies:

```sh
docker compose --env-file deploy/.env --project-directory deploy \
  -f deploy/docker-compose.yml up -d mysql redis minio
docker compose --env-file deploy/.env --project-directory deploy \
  -f deploy/docker-compose.yml --profile init run --rm minio-init
```

Apply the schema manually:

```sh
docker compose --env-file deploy/.env --project-directory deploy \
  -f deploy/docker-compose.yml exec -T mysql sh -ec \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql -u"$MYSQL_USER" "$MYSQL_DATABASE"' \
  < deploy/schema.sql
```

Start all services:

```sh
scripts/compose-up.sh
scripts/compose-status.sh
```

The default HTTP address is `http://localhost:8000`. Runtime gRPC is exposed at
`localhost:9000`.

## Ingress

Terminate HTTPS at the public ingress. Forward only from CIDRs listed in
`security.trusted_proxies`. Route:

- `/` to the web server;
- `/api/` to the backend HTTP listener after removing the `/api` prefix;
- the runtime gRPC port to the backend gRPC listener;
- the configured public S3 path or hostname to object storage.

Preserve `Host`, the client address, `X-Request-ID`, and gRPC metadata. Set the
HTTP request body limit to at least 64 MiB if uploads use the included limits.
Use exact HTTPS origins in `security.allowed_origins`.

## Rolling changes

Apply compatible database changes first. Deploy backend instances with the same
configuration and key ring. Deploy the web image after the management API is
available. Remove old JWT keys only after the old access-token lifetime has
elapsed.

No old Kirby data migration is included. Import existing data through a
separately reviewed migration or the supported snapshot import API.

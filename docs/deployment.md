# Deployment

## Production requirements

Use MySQL 8.4 or PostgreSQL, Redis 7, S3-compatible object storage, and a TLS
ingress. The backend is stateless. Run one or more identical backend instances
behind the ingress. Do not use local object storage in production.

The repository publishes one pinned Kirby image. Its final `scratch` stage
contains the Go binary and built React assets. Each container runs one Go
process as user `65532:65532`.

## Image configuration and ports

Copy `deploy/config.example.yaml`, replace its database, Redis, and S3 settings,
and mount the resulting file at `/etc/kirby/config.yaml`. The file must be
readable by container user `65532:65532`.

Both configuration templates use container ports `8080` for HTTP and `9090`
for gRPC, matching the image and the default `healthcheck` command. For a
standalone container, publish `8080:8080` and `9090:9090`, or route the ingress
to those container ports. HTTP includes the web application, API, and S3 proxy.

The Compose example intentionally publishes host ports `8000:8080` and
`9000:9090`. Uploads use the browser's current origin and completed asset URLs
default to that public address. Changing the public domain or mapped host ports
does not require editing S3 configuration.

## S3 prerequisites

Prepare the S3 bucket before starting Kirby. The application checks that it
exists and fails startup when it is missing or inaccessible. It does not create
the bucket, set access policies, configure CORS, or install lifecycle rules.

Grant the configured S3 account permission to check the bucket and read, write,
inspect, and delete its upload and published objects. Published
`environments/` objects must allow anonymous reads, while `uploads/` stays
private. Apply the public-read policy and temporary-upload cleanup rule using
the instructions in [deploy/README.md](../deploy/README.md). Change policy
resources when the bucket is not named `kirby`.

Use `object_storage.s3.endpoint` for the address reachable from the container
and `use_ssl` for that connection's TLS setting. Browser upload tickets use the
relative `/<bucket>/` path and Kirby proxies uploads to S3. Keep all paths,
including `/<bucket>`, routed through Nginx or Ingress without rewriting them.
Same-origin uploads through Kirby do not require S3 CORS configuration.

Completed asset URLs default to the public request's origin plus
`/<bucket>/<object-key>`. Proxy forwarding headers provide the original public
scheme and host. There is no public-domain setting required for this path.
For an independent CDN or resource domain, optionally set
`object_storage.s3.public_base_url` to its full base URL, such as
`https://cdn.example.com/kirby`. Leave it absent or empty to use the default.

## Database initialization

Create an empty database and start Kirby. It creates the model tables from
embedded SQL, then inserts the built-in permissions and roles. It also generates
and stores the JWT signing key and API-key pepper in `application_secrets`.
Each application startup loads those shared values into memory; no key or
origin allowlist needs to be entered in the configuration. The configured
`database.driver` selects `mysql` or `pgsql`.

A new database has no built-in administrator or default password. After the
service starts, open `/setup` from the web application. Enter a username,
display name, and a password of at least 12 bytes to create the first
system administrator, then continue to `/login`. The deployer should complete
this first setup through a controlled network or ingress before opening access
to other users.

The shared `application_secrets.admin_initialized` flag permanently records
completion. Deleting every system administrator does not reopen `/setup`.
Create a replacement administrator with the CLI if access must be recovered.

Initialization only runs when the database has no application tables.
`deploy/schema.sql` is a schema reference, not a complete initialization method:
the application generates persistent signing keys in its initialization
transaction. Start the server or run `create-admin` against the empty database
instead of importing that SQL manually.

For automated deployments or account recovery, the CLI remains available:

```sh
cd server
go run . create-admin --config /etc/kirby/config.yaml \
  --username admin --display-name Administrator
```

For automation, pass `--password-file`. The file must be regular, must not be a
symbolic link, and must not grant group or other access. Creating the first
administrator through the CLI also marks initialization complete and closes the
web setup page.

## Compose example

The included Compose stack is a local deployment example with MySQL, Redis,
MinIO, and one Kirby container. The Kirby process serves the React application,
HTTP APIs, runtime gRPC, and the same-origin object-storage path.

MinIO and its `mc` client are built together from fixed official Go module
versions in `deploy/minio/Dockerfile`. CI uses the same image. MinIO is now
[distributed as source only](https://github.com/minio/minio#source-only-distribution),
and the previously pinned images no longer allow anonymous pulls. The first
startup compiles these tools and takes longer. The build requires access to
Docker Hub, the Go module proxy and checksum database, and Alpine packages.

```sh
cp deploy/.env.example deploy/.env
cp deploy/config.compose.yaml deploy/config.compose.local.yaml
```

Replace every `CHANGE_ME` value. Keep the values in `.env` and the YAML
consistent. Then start the dependencies:

```sh
docker compose --env-file deploy/.env --project-directory deploy \
  -f deploy/docker-compose.yml up -d --build mysql redis minio
docker compose --env-file deploy/.env --project-directory deploy \
  -f deploy/docker-compose.yml --profile init run --rm minio-init
```

Start all services:

```sh
scripts/compose-up.sh
scripts/compose-status.sh
```

The default HTTP address is `http://localhost:8000`. Runtime gRPC is exposed at
`localhost:9000`. Open `http://localhost:8000/setup` to create the first
system administrator unless the CLI already created one.

## Ingress

Terminate HTTPS at the public ingress. Kirby accepts proxy forwarding headers
by default, so no proxy address list is required. Route:

- all HTTP paths to the Kirby HTTP listener without rewriting them;
- the runtime gRPC port to the Kirby gRPC listener.

Kirby serves `/`, removes the public `/api` prefix internally, and forwards the
configured S3 bucket path to object storage while preserving the public Host.
Preserve `Host`, the client address, `X-Request-ID`, and gRPC metadata. Set the
ingress request body limit to at least 64 MiB if uploads use the included
limits. Keep the web application and management API on the same public origin.
The proxy must overwrite forwarding headers supplied by clients, preserve the
original Host, and send `X-Forwarded-Proto` and `X-Forwarded-For`. It may also
send `X-Forwarded-Host`. Kirby still compares browser origins against the public
request address. Optionally set `security.trusted_proxies` to explicit CIDRs to
restrict trusted peers, or `[]` to ignore forwarding headers.

## Shared installation data

Deploy Kirby instances with the same configuration, shared database, and image
version. Include the `application_secrets` record in database backups and
preserve it when restoring the installation.

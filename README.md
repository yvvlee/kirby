# Kirby

Kirby is a standalone configuration platform. One deployment manages multiple
projects, each containing independent environments. Users have one login and
a fixed role in each project. Each environment grants permissions to those roles.

The repository contains:

- a React 19 and TypeScript administration application;
- an HTTP management API protected by user JWTs;
- HTTP and gRPC runtime APIs protected by environment-bound API keys;
- a stateless Go backend backed by MySQL or PostgreSQL, Redis, and S3-compatible storage.

Kirby does not use Nacos. It does not include approval, reward, or legacy data
migration features. The server initializes model tables and system settings from embedded SQL on
the first startup. Self-registration is disabled by default. JWT signing keys
and the API-key pepper are generated in the database and loaded into memory at
startup. No signing keys or browser-origin allowlist need to be configured.
New installations have no default administrator or password; create the first
system administrator on `/setup`.

Nginx and Ingress forwarding headers are trusted by default. Preserve Host,
overwrite client forwarding headers, and send `X-Forwarded-Proto` and
`X-Forwarded-For`. The web application and management API must share the same
public origin. `security.trusted_proxies` is optional when explicit proxy CIDR
restrictions are needed; `[]` ignores forwarding headers.

For S3 deployments, configure the backend connection and credentials. Uploads
use Kirby's same-origin `/<bucket>/` proxy, and completed resource URLs use the
public request address by default. A public domain is not required in the
configuration. Set `object_storage.s3.public_base_url` only for an independent
CDN or resource domain. The bucket, read policy, and lifecycle rule must be
[prepared before startup](deploy/README.md).

## Local start in ten minutes

This path runs one backend process. It reuses existing `mysql` and `redis`
containers and runs the frontend directly with `npm run dev`.

Requirements:

- Go 1.27.2 or a compatible newer toolchain;
- Node.js 24.19.0 and npm;
- running MySQL 8.4 and Redis 7 containers exposed on localhost;
- container names `mysql` and `redis` in the commands below, or equivalent
  commands adjusted for the local names.

Start the existing containers and verify them:

```sh
docker start mysql redis
docker exec redis redis-cli ping
```

Create the database. The backend creates its tables when it starts:

```sh
docker exec -i mysql sh -ec 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot' <<'SQL'
CREATE DATABASE IF NOT EXISTS kirby CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER IF NOT EXISTS 'kirby'@'%' IDENTIFIED BY 'kirby-local-password';
ALTER USER 'kirby'@'%' IDENTIFIED BY 'kirby-local-password';
GRANT ALL PRIVILEGES ON kirby.* TO 'kirby'@'%';
SQL
```

Create `.local/config.yaml` and `.local/objects` from the repository root.
The credentials below are only for local development:

```yaml
mode: single
http:
  address: "127.0.0.1:8080"
  timeout: 120000
grpc:
  address: "127.0.0.1:9090"
  timeout: 10000
database:
  driver: mysql
  dsn: "kirby:kirby-local-password@tcp(127.0.0.1:3306)/kirby?charset=utf8mb4&parseTime=true&loc=UTC"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 300000
cache:
  driver: redis
  redis:
    address: "127.0.0.1:6379"
    username: ""
    password: ""
    db: 0
    key_prefix: "kirby-local:"
jwt:
  issuer: kirby-local
  access_ttl: 900000
  refresh_ttl: 604800000
object_storage:
  driver: local
  local: {directory: "../.local/objects"}
  s3:
    endpoint: ""
    region: ""
    bucket: ""
    access_key: ""
    secret_key: ""
    use_ssl: false
log:
  level: info
```

```sh
mkdir -p .local/objects
```

Start the backend in the first terminal:

```sh
cd server
go run . serve --config ../.local/config.yaml
```

Start the React development server in the second terminal:

```sh
cd web
npm ci
KIRBY_DEV_API_TARGET=http://127.0.0.1:8080 npm run dev
```

Open `http://localhost:15173/setup` from a controlled network or ingress. Enter
a username, nickname, email, and a password of 8–32 visible ASCII bytes using at least two of letters, digits, and symbols. After
creating the system administrator, continue to `/login`. Setup stays closed
once completed, even if every system administrator is later deleted.

The Vite proxy maps `/api` to the backend. The backend readiness endpoint is
`http://127.0.0.1:8080/readyz`.

For automation or account recovery, create an administrator with the CLI:

```sh
cd server
go run . create-admin --config ../.local/config.yaml \
  --username admin --nickname Administrator --email admin@example.com
```

The command prompts twice for a password of 8–32 visible ASCII bytes using at least two of letters, digits, and symbols. The first
CLI-created administrator also closes the web setup page.

For a disposable automated run against the same existing containers, use:

```sh
scripts/test-local-dev.sh
```

It creates and removes its own temporary database, administrator, object
directory, backend, and Vite process.

## Runtime API

Create a project API key in the administration application. The full secret is
shown once. Replace the deliberately invalid placeholder in these examples.

HTTP:

```sh
curl -H 'X-Kirby-API-Key: YOUR_API_KEY' \
  'http://127.0.0.1:8080/api/v1/config?key=Greeting'
```

gRPC:

```sh
grpcurl -plaintext \
  -H 'x-kirby-api-key: YOUR_API_KEY' \
  -d '{"key":"Greeting"}' \
  127.0.0.1:9090 kirby.runtime.v1.Api/Config
```

The API key determines both the project and environment. Requests only specify
the configuration key and cannot select another project or environment.

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [Deployment](docs/deployment.md)
- [Authorization](docs/authorization.md)
- [Project API keys](docs/api-key.md)
- [Snapshot import and export](docs/import-export.md)
- [Development](docs/development.md)
- [Simplified Chinese and English](docs/internationalization.md)
- [Operations](docs/operations.md)
- [Releasing](docs/releasing.md)
- [Security design](docs/security.md)
- [Source provenance](docs/source-provenance.md)

The public API source of truth is under `server/api`. See
[CONTRIBUTING.md](CONTRIBUTING.md) before changing generated code or
dependencies.

## License

Kirby is released under the MIT License. Dependency notices and software bills
of materials are in `NOTICE`, `docs/third-party-licenses.md`, and `dist/sbom`.

Project environments own independent configuration data and API keys. Runtime requests select the environment through the API key. See [project and environment isolation](docs/environment-isolation.md).

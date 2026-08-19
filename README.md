# Kirby

Kirby is a standalone configuration platform. One deployment manages multiple
environments. Administrators use one login, while roles and permissions are
assigned separately in each environment.

The repository contains:

- a React 19 and TypeScript administration application;
- an HTTP management API protected by administrator JWTs;
- HTTP and gRPC runtime APIs protected by project-level API keys;
- a stateless Go backend backed by MySQL, Redis, and S3-compatible storage.

Kirby does not use Nacos. It does not include approval, reward, or legacy data
migration features. The server never creates or changes database tables.

## Local start in ten minutes

This path runs one backend process. It reuses existing `mysql` and `redis`
containers and runs the frontend directly with `npm run dev`.

Requirements:

- Go 1.25.13 or a compatible newer toolchain;
- Node.js 24.19.0 and npm;
- running MySQL 8.4 and Redis 7 containers exposed on localhost;
- container names `mysql` and `redis` in the commands below, or equivalent
  commands adjusted for the local names.

Start the existing containers and verify them:

```sh
docker start mysql redis
docker exec redis redis-cli ping
```

Create the database and apply the fixed schema. This is an explicit,
one-time operation:

```sh
docker exec -i mysql sh -ec 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot' <<'SQL'
CREATE DATABASE IF NOT EXISTS kirby CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER IF NOT EXISTS 'kirby'@'%' IDENTIFIED BY 'kirby-local-password';
ALTER USER 'kirby'@'%' IDENTIFIED BY 'kirby-local-password';
GRANT ALL PRIVILEGES ON kirby.* TO 'kirby'@'%';
SQL

docker exec -i mysql sh -ec \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot kirby' < deploy/schema.sql
```

Create `.local/config.yaml` and `.local/objects` from the repository root.
The credentials below are only for local development:

```yaml
mode: single
http: {address: "127.0.0.1:8080", timeout: 2m}
grpc: {address: "127.0.0.1:9090", timeout: 10s}
mysql:
  dsn: "kirby:kirby-local-password@tcp(127.0.0.1:3306)/kirby?charset=utf8mb4&parseTime=true&loc=UTC"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 5m
cache:
  driver: redis
  redis: {address: "127.0.0.1:6379", username: "", password: "", db: 0, key_prefix: "kirby-local:"}
jwt:
  issuer: kirby-local
  active_kid: local
  access_ttl: 15m
  refresh_ttl: 168h
  keys: {local: "local-development-jwt-key-0000000000000000"}
security:
  api_key_pepper: "local-development-api-pepper-00000000000000"
  allowed_origins: ["http://localhost:15173", "http://127.0.0.1:15173"]
  trusted_proxies: []
object_storage:
  driver: local
  local: {directory: "../.local/objects"}
  s3:
    endpoint: ""
    presign_endpoint: ""
    region: ""
    bucket: ""
    access_key: ""
    secret_key: ""
    use_ssl: false
    presign_use_ssl: false
    public_base_url: ""
log: {level: info, format: text}
```

```sh
mkdir -p .local/objects
```

Create the first administrator. The command prompts twice for a password of at
least 12 bytes:

```sh
cd server
go run . create-admin --config ../.local/config.yaml \
  --username admin --display-name Administrator
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

Open `http://localhost:15173`. The Vite proxy maps `/api` to the backend. The
backend readiness endpoint is `http://127.0.0.1:8080/readyz`.

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
curl -H 'X-Kirby-API-Key: kirby_pk_INVALID.EXAMPLE_ONLY' \
  'http://127.0.0.1:8080/v1/config?project=demo&key=Greeting'
```

gRPC:

```sh
grpcurl -plaintext \
  -H 'x-kirby-api-key: kirby_pk_INVALID.EXAMPLE_ONLY' \
  -d '{"project":"demo","key":"Greeting"}' \
  127.0.0.1:9090 kirby.runtime.v1.Api/Config
```

The key is bound to one project. A key cannot read another project even when
the request changes the `project` parameter.

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [Deployment](docs/deployment.md)
- [Authorization](docs/authorization.md)
- [Project API keys](docs/api-key.md)
- [Snapshot import and export](docs/import-export.md)
- [Development](docs/development.md)
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

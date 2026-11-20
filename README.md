# Kirby

<p align="center">
  <strong>A lightweight, multi-tenant configuration platform for cloud-native microservices.</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8.svg" alt="Go Version"></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/React-19-61DAFB.svg" alt="React 19"></a>
  <a href="https://hub.docker.com/r/mshadow/kirby"><img src="https://img.shields.io/badge/Docker-mshadow%2Fkirby-2496ED.svg" alt="Docker Image"></a>
</p>

<p align="center">
  <b>English (Default)</b> | <a href="README.zh-CN.md">简体中文</a>
</p>

---

## Overview

Kirby is a modern, standalone configuration management platform designed for cloud-native applications and microservice architectures. A single deployment manages multiple independent projects, with each project containing isolated environments (such as development, staging, and production).

Engineered with a stateless Go backend and a modern React 19 administration portal, Kirby provides full lifecycle configuration management—from schema definition, drafting, and snapshot versioning to high-performance, low-latency delivery over HTTP and gRPC.

## Key Features

- 🏢 **Multi-Project & Multi-Environment Isolation**: Strict hierarchical isolation (`Project -> Environment -> Configuration`). Configurations and credentials in different environments never collide.
- ⚡ **High-Performance Dual Runtime Planes**: Sub-millisecond HTTP and gRPC runtime read APIs backed by Redis caching, authenticated via environment-scoped API keys.
- 🎨 **Modern Administration Portal**: Built with React 19, TypeScript, and Ant Design. Features an integrated Monaco editor, JSON schema validation, and visual configuration diffs.
- 🔒 **Fine-Grained Role-Based Access Control (RBAC)**: Fixed project-level roles (Owner, Maintainer, Member, Guest) mapped to granular environment permission matrices (Read, Write, Publish, Manage API Keys).
- 📦 **Configuration Snapshots & Atomic Rollback**: Full draft editing, versioned snapshots, cross-environment import/export, and instant, zero-downtime rollbacks.
- 🗄️ **S3-Compatible Asset Management**: Built-in upload tickets with same-origin reverse proxying for secure handling of static assets and configuration files.
- 🌐 **Native Internationalization (i18n)**: Seamless language switching between Simplified Chinese (`zh-CN`) and English (`en-US`) across both the web UI and backend error responses.
- 🚀 **Stateless & Production-Ready**: Single scratch-based Docker container image, automatic database schema initialization on first boot, and support for both MySQL 8.4 and PostgreSQL.

## Architecture

Kirby cleanly separates concerns into two operational planes:

```text
               ┌────────────────────────────────────────────────────────┐
               │                      Kirby Server                      │
               │                                                        │
Browser  ─────►│  Administration Plane (HTTP / React 19 Web App)        │
               │  - User JWT Authentication                             │
               │  - Project & Environment Management                    │
               │  - Config Schema, Snapshots, Diff, and Publishing      │
               │                                                        │
Client   ─────►│  Runtime Plane (HTTP / gRPC)                           │
Services       │  - Environment API Key Authentication                 │
               │  - Low-latency Published Config Delivery               │
               └───────────┬───────────────────┬───────────────────┬────┘
                           │                   │                   │
                           ▼                   ▼                   ▼
                  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
                  │ Relational DB   │ │ Redis           │ │ Object Storage  │
                  │ (MySQL / PgSQL) │ │ (Cache/RateLim) │ │ (S3 / MinIO)    │
                  └─────────────────┘ └─────────────────┘ └─────────────────┘
```

- **Administration Plane**: The web dashboard and management HTTP API. Users authenticate with access JWTs; authorization is resolved against project membership and environment permissions.
- **Runtime Plane**: Exposes published configurations to workloads over HTTP and gRPC using lightweight environment-bound API keys.
- **State Storage**: The relational database is authoritative for all application state. Redis accelerates runtime reads and manages rate limits. Object storage stores file uploads and published assets.

## Quick Start

### Option 1: Docker Compose (Recommended)

The fastest way to run a complete Kirby stack (Kirby, MySQL, Redis, and MinIO) locally:

```sh
# 1. Clone repository
git clone https://github.com/mshadow/kirby.git && cd kirby

# 2. Configure environment file
cp deploy/.env.example deploy/.env

# 3. Start the entire stack
./scripts/compose-up.sh
```

Once started:
- Access the web console at `http://localhost:8000/setup` to create the initial system administrator.
- Stop the stack anytime with `./scripts/compose-down.sh`.

### Option 2: Local Development from Source

To run the Go backend and React frontend locally:

**Prerequisites**:
- Go 1.27+
- Node.js 24+ and npm
- Running MySQL 8.4 and Redis 7 instances

```sh
# 1. Prepare configuration and local object storage
mkdir -p .local/objects
cp deploy/config.local.example.yaml .local/config.yaml

# 2. Start Go backend (automatically initializes database tables)
cd server
go run . serve --config ../.local/config.yaml

# 3. Start React frontend development server (in another terminal)
cd web
npm ci
KIRBY_DEV_API_TARGET=http://127.0.0.1:8080 npm run dev
```

Open `http://localhost:15173/setup` in your browser to complete initial setup and proceed to `/login`.

> [!TIP]
> You can also create an administrator via the CLI:
> ```sh
> cd server
> go run . create-admin --config ../.local/config.yaml \
>   --username admin --nickname Administrator --email admin@example.com
> ```

## Runtime API Usage

Runtime clients fetch published configurations using an environment API key. Create a key under your project's environment in the web console, then query the runtime plane:

### HTTP

```sh
curl -H "X-Kirby-API-Key: YOUR_API_KEY" \
  "http://127.0.0.1:8080/api/v1/config?key=AppConfig"
```

### gRPC

```sh
grpcurl -plaintext \
  -H "x-kirby-api-key: YOUR_API_KEY" \
  -d '{"key":"AppConfig"}' \
  127.0.0.1:9090 kirby.runtime.v1.Api/Config
```

> The API key is bound to a specific project and environment; client requests only need to provide the configuration key.

## Documentation

> 📖 **Language Notice**: The project README is provided in **English (Default)** and [简体中文](README.zh-CN.md). The in-depth architectural and operational specifications in [`docs/`](docs/) are maintained in English.

Detailed documentation is available in the [`docs/`](docs/) directory:

| Category | Document | Description |
|---|---|---|
| **Architecture** | [Architecture](docs/architecture.md) | System shape, plane separation, and data flow |
| | [Authorization](docs/authorization.md) | Roles, permissions, member invitations, and project deletion |
| | [Environment Isolation](docs/environment-isolation.md) | Data ownership hierarchy and schema requirements |
| | [Security Design](docs/security.md) | Security boundaries, secrets, and session management |
| | [Threat Model](docs/threat-model.md) | Protected assets and trust boundaries |
| **Operations** | [Deployment Guide](docs/deployment.md) | Production setup, ingress routing, and reverse proxies |
| | [Configuration Reference](docs/configuration.md) | Full server configuration schema and parameters |
| | [Operations Guide](docs/operations.md) | Health checks, logging, backup, restore, and troubleshooting |
| | [Docker Compose Guide](deploy/README.md) | Compose stack, S3 bucket setup, and lifecycle rules |
| **Development** | [Development Guide](docs/development.md) | Toolchain, local dev, code generation, and test suite |
| | [Environment API Keys](docs/api-key.md) | API key lifecycle, rotation, and revocation |
| | [Snapshot Import & Export](docs/import-export.md) | Transferring configurations across environments |
| | [Internationalization](docs/internationalization.md) | Frontend and backend i18n guide (zh-CN / en-US) |
| | [Source Provenance](docs/source-provenance.md) | Origin and licensing verification |

## Contributing

We welcome contributions! Please review [CONTRIBUTING.md](CONTRIBUTING.md) and [Development Guide](docs/development.md) before submitting pull requests or reporting issues.

## License

Kirby is open-source software released under the [MIT License](LICENSE). Third-party dependencies and notices are detailed in [NOTICE](NOTICE), [Third-Party Licenses](docs/third-party-licenses.md), and `dist/sbom`.

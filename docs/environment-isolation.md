# Project and Environment Isolation

Kirby enforces hierarchical data ownership and isolation across projects and environments. A single deployment manages multiple projects, each containing independent environments.

The durable hierarchy is:

```text
Project
└── Environment
    ├── Configurations (Key-Value)
    │   ├── Structures (JSON Schemas)
    │   ├── Enums
    │   └── Snapshots (Drafts & Published Versions)
    ├── Environment API Keys (HTTP & gRPC runtime credentials)
    └── Uploaded Assets (Environment-scoped object storage paths)
```

## Data Ownership Model

- **Environments belong to Projects**: Environments link to projects via `environments.project_id`. The project model does not store an environment ID.
- **Configurations belong to Environments**: Configurations link to environments via `configs.environment_id`. Configuration keys are unique within an environment; different environments can define identical configuration keys without collision.
- **Structures, Enums, and Snapshots**: These resources inherit environment ownership from their parent configuration (`config_id`). Publishing state and active runtime versions belong to the environment's configuration.
- **Environment API Keys**: API keys are bound to both a project and a specific environment (`project_id` and `environment_id`). Runtime requests authenticate via the API key; the server derives the target environment directly from the key. Clients cannot switch environments in API requests. Disabling an environment immediately revokes runtime access for all its keys.
- **Runtime Caching**: Published configuration cache keys combine the environment ID, configuration key, and publication version. This guarantees that configurations with identical keys in different environments never collide in Redis.
- **Object Storage Isolation**: File uploads and completion checks validate project and environment boundaries. The immutable storage path includes the environment ID (`environments/<env-id>/...`). Note that published assets are anonymously accessible by design and should not store confidential secrets.
- **API Query Scoping**: An `environment_id` parameter in administrative queries specifies the permission evaluation scope; it does not indicate that a project belongs to an environment.

## Membership and Permissions

Users maintain a single identity across the entire system, with a fixed role in each project: **Owner**, **Maintainer**, **Member**, or **Guest**. Environments do not have independent member rosters.

Instead, each environment manages a permission matrix that maps project roles to four granular permission points:
1. **Read**: View configurations, metadata, snapshots, and exports.
2. **Write**: Create and edit configurations, structures, enums, snapshots, imports, and uploads.
3. **Publish / Unpublish**: Publish snapshots to the runtime plane or unpublish existing versions.
4. **Manage API Keys**: Create, list, rotate, and revoke runtime API keys for that environment.

- Project Owners always retain all permissions across all environments.
- Maintainers, Members, and Guests are restricted according to the environment's permission matrix.
- System administrators have no implicit project access and must be explicitly added to a project to access its resources.
- Snapshot exports require Read permission. Snapshot imports require Read permission on the source environment and Write permission on the target environment; imports only create draft snapshots and do not publish automatically.

## Database Schema Requirements

Kirby requires explicit environment scoping on core tables. Specifically:
- `configs` must include an `environment_id` column with a unique index on `(environment_id, key)`.
- `project_api_keys` must include an `environment_id` column.

If Kirby detects missing `environment_id` columns during startup validation, it halts with:
`database requires environment isolation migration; see docs/environment-isolation.md`.

For new deployments, starting Kirby against an empty database automatically creates all required tables and constraints from embedded DDL. Existing databases without environment isolation must be updated to include `environment_id` before upgrading to this version.

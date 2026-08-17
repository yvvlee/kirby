# Authorization

Authentication answers who the user is. Authorization answers what that user
may do in one environment. Kirby keeps those decisions separate.

An administrator has one access JWT. The token contains stable user and session
identity, not environment permissions. For each management request, the server
resolves current roles for the environment named in the path and request body.

## Built-in roles

| Capability | viewer | editor | publisher | admin | system admin |
|---|:---:|:---:|:---:|:---:|:---:|
| Read projects, configs, structures, enums, and snapshots | Yes | Yes | Yes | Yes | All environments |
| Read project API-key metadata | Yes | Yes | Yes | Yes | All environments |
| Export snapshots | Yes | Yes | Yes | Yes | All environments |
| Edit projects, configs, structures, and enums | No | Yes | Yes | Yes | All environments |
| Create/delete snapshots and upload assets | No | Yes | Yes | Yes | All environments |
| Import snapshots | No | Yes | Yes | Yes | All environments |
| Publish and unpublish snapshots | No | No | Yes | Yes | All environments |
| Create, rotate, and revoke project API keys | No | No | No | Yes | All environments |
| Manage members of the current environment | No | No | No | Yes | All environments |
| Manage users, roles, and environments | No | No | No | No | Yes |

System administration is the `users.is_system_admin` flag. The three system
permissions are not assigned to environment roles. A system administrator
bypasses environment roles and receives all permissions for every existing
environment. Requests for a nonexistent environment still fail.

## Custom roles

System administrators can create custom roles and assign any environment-level
permission. Built-in role records are seeded by `deploy/schema.sql`. Keep
system permissions separate from environment roles.

## Enforcement

The Vue application hides unavailable actions for usability. It is not a
security boundary. The backend checks permissions and scopes repository queries
to the environment. Replacing an environment, project, snapshot, import, key,
or object ID in a request does not bypass the ownership check.

Role changes increment shared generation state. Other requests resolve the new
permissions without requiring the user to log in again. MySQL remains
authoritative during a Redis cache outage, so a cache failure cannot grant
permissions.

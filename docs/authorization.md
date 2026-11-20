# Authorization

Authentication identifies the user. Authorization checks project membership and
permissions in the requested environment. The access JWT carries user and
session identity, not a copy of project roles or environment grants.

## System and project scope

System administrators manage users and system settings. They do not bypass
project membership or environment permissions. Self-registration is disabled by
default and can be enabled in system settings.

Every signed-in user can create a project and becomes its Owner. The project
list shows only projects the user belongs to. Global management is accessed
from the upper-right menu; members, environments, and project settings are
managed inside the project. Login opens the project list.

## Fixed project roles

Roles are fixed: Owner > Maintainer > Member > Guest. A project can have multiple
Owners. Its last Owner cannot be removed or demoted. There are no custom roles
or separate environment memberships.

Only Owners can edit or delete the project and create, edit, or delete its
environments. Owners and Maintainers can add members, change roles, or remove
members whose current role is no higher than their own. They cannot assign a
role higher than their own. Maintainers cannot manage Owners.

## Project deletion

Only the project Owner can delete a project. Deletion removes the project's
environments, configurations, structures, enums, snapshots, API keys, invitations,
members, environment permission grants, and related import records in a single
database transaction, while preserving audit records. Once deleted, runtime
credentials can no longer authenticate.

Uploaded files in object storage are retained. Files may be referenced across
configurations or projects via URL without a central cross-project registry;
deleting these objects immediately could break other projects. To reclaim
storage, verify existing configuration references before purging objects through
storage maintenance routines.

## Environment permission grants

Each environment has four fixed permission points. Its creation initializes the
following grants to project roles:

| Permission | Owner | Maintainer | Member | Guest |
|---|:---:|:---:|:---:|:---:|
| Read configuration, metadata, snapshots, and exports | Yes | Yes | Yes | Yes |
| Write configuration, structures, enums, snapshots, imports, and uploads | Yes | Yes | Yes | No |
| Publish and unpublish | Yes | Yes | No | No |
| Manage API keys | Yes | Yes | No | No |

Owners and Maintainers can tighten or expand these grants independently for each
environment. Owner access is unconditional and cannot be removed. An environment
API key selects exactly one project and environment for runtime reads.

## Members and invitations

Owners and Maintainers can search existing users and add them directly without
confirmation. When self-registration is enabled, they can also generate an
invitation link with a role no higher than their own. The link expires after
24 hours and can be accepted once; its token is stored only as a hash.

A signed-in recipient must confirm before joining. A signed-out recipient goes
to login or registration, returns to the invitation page, and then confirms.
User creation and registration require an email address. Users can log in with
either their username or email; the display field is called nickname.

## Enforcement

The React application hides unavailable actions for usability. The backend is
the security boundary: it checks project membership, environment grants, and
resource ownership on every management request. Replacing project, environment,
snapshot, import, key, or object IDs does not bypass these checks.

Membership and permission changes take effect without another login. The
relational database remains authoritative; cache failures cannot grant access.

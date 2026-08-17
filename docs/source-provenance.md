# Source provenance

Kirby is published as a new standalone repository.

The project author confirmed that the Kirby-related code copied from the two source repositories was written by the author and may be released under the MIT license included in this repository. The source repositories themselves, their unrelated code, and their Git histories are not part of the Kirby release.

## Frozen source revisions

| Source | Revision | Access during extraction |
|---|---|---|
| `dashboard` | `d43577915a0811e5c24e47ac088278b035c6e8fd` | Read-only |
| `old` | `384fd6557b058920299a59087c24d9ac5b505d51` | Read-only |
| Initial `kirby` repository | `59437e33ce790e6cff6c66b6e4d8d07450ad845d` | Extraction target |
| Cached protobuf module `git.changbaops.com/golang/proto@v1.16.22` | Go module checksum recorded by the source project | Read-only protobuf source |

## Frontend source mapping

| Source path | Target area |
|---|---|
| `dashboard/src/client/router/children/config-center.js` | `web/src/router/routes/config-center.js` |
| `dashboard/src/client/store/config-center.js` | `web/src/store/modules/config-center.js` |
| `dashboard/src/client/views/config-center/project-list.vue` | `web/src/features/config-center/projects/` |
| `dashboard/src/client/views/config-center/config-list.vue` | `web/src/features/config-center/configs/` |
| `dashboard/src/client/views/config-center/config-detail.vue` | `web/src/features/config-center/configs/detail/` |
| `dashboard/src/client/views/config-center/model-list.vue` | `web/src/features/config-center/models/` |
| `dashboard/src/client/views/config-center/enum-list.vue` | `web/src/features/config-center/enums/` |
| `dashboard/src/client/views/config-center/snapshot/` | `web/src/features/config-center/snapshots/` |
| Generic editor components and schema utilities under `dashboard/src/client/views/config-center/` | `web/src/components/` and `web/src/features/config-center/schema/` |

The extraction excludes the dashboard shell, gateway request wrapper, PHP session login, activity configuration, internal upload component, reward editor, approval views, build files, dependency manifests, and lock files.

## Backend source mapping

| Source path | Target area |
|---|---|
| `old/cmd/kirby/` | `server/cmd/kirby/` |
| `old/internal/{entity,converter}/` | `server/internal/{entity,converter}/` |
| `old/internal/logic/` | `server/internal/logic/` |
| `old/internal/repository/` | `server/internal/repository/` |
| `old/internal/service/` | `server/internal/service/` |
| `old/internal/server/` | `server/internal/server/` |
| `old/internal/model/` | `server/internal/model/` |
| `old/pkg/set/hashset.go` | `server/internal/set/hashset.go` |

The extraction excludes the source dependency manifests, generated Wire files, Nacos configuration, the Yahaha client, reward logic, approval logic, header-based pseudo-authentication, automatic table synchronization jobs, Dockerfile, CI configuration, and README.

## Protobuf source mapping

The source backend does not contain its protobuf sources. The required definitions are read from the local module cache at:

```text
/Users/mshadow/go/pkg/mod/git.changbaops.com/golang/proto@v1.16.22
```

Only the Kirby admin and runtime definitions, pagination, select-option annotations, and actually used errors are copied. All package names and imports are rewritten to public Kirby packages. Prize, Gender, BusinessLine, Yahaha, approval, and unrelated internal error definitions are excluded.

## History boundary

No Git history is imported from `dashboard` or `old`. Each extraction task creates a new, reviewable commit in this repository. This document records provenance without making either source repository part of the distributed work.

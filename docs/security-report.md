# Security verification report

## 2026-10-15: lorm migration and Go 1.27.2

The working tree passes the security gate after the database migration and
dependency repairs. Reachable Go vulnerabilities, npm audit findings, high or
critical release-image findings, and source-secret findings are zero. These
results describe the checks below; they are not a claim that the application is
free of all vulnerabilities.

| Check | Input | Result |
|---|---|---|
| Reachable Go vulnerabilities | Go 1.27.2, govulncheck v1.9.0, current official Go vulnerability database | 0 reachable; 1 imported-package and 1 module finding have no detected call path |
| Go source analysis | gosec v2.29.0; final hand-written backend source | 0 unsuppressed issues |
| Frontend audit | Node 24.19.0, final npm lockfile, official npm registry | 0 vulnerabilities across production and development dependencies |
| Release image | scratch image built with Go 1.27.2 and the production web assets | 0 high/critical findings |
| Container scanner | Trivy 0.67.2, pinned image digest `sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08` | current database downloaded on 2026-10-15 at 11:47 UTC |
| Source secrets | Trivy secret scanner | 0 detected secrets |
| Removed/private dependency checks | Git source, Go modules/packages, npm tree | passed; source checks include untracked, non-ignored files |
| Backend verification | full Go race suite and final linter rules | passed |
| Real database verification | disposable MySQL and PostgreSQL, including transaction rollback and UTC round trips | passed |
| Browser/frontend verification | real backend browser journey, 113 frontend tests, lint, type check, production build | passed |
| License policy | Go and npm dependency licenses | passed |

Dependency repairs include `golang.org/x/net` v0.60.0, gRPC v1.83.2, Axios
1.20.0, DOMPurify 3.4.17, and Vitest 4.1.11. Compatible lockfile updates also
repair brace-expansion, js-yaml, and source-map-js. Vitest 4 removes the affected
tinypool dependency; the existing tests and configuration remain compatible.

The standalone security scanner retains seven checked integer-conversion
suppressions in `internal/util/safeint` and two cookie Secure suppressions in
`internal/auth/session`. The cookie policy derives Secure from the validated
public origin and supports local HTTP development. Both areas have boundary
tests. No broad business-directory suppression was added.

Database checks cover lorm CRUD, optimistic locking, soft-delete restoration,
NULL JSON, environment scope, rollback, explicit UTC timestamps, concurrent
initialization, and PostgreSQL seed sequences. The sequence repair is tested
for repeated execution and for preserving sequences already ahead of row IDs.
Initial-administrator login and refresh are additionally exercised against
MySQL. Tests use disposable databases; the existing-database repair SQL was not
run against an operator's database.

To repeat the security and license gates from the repository root:

```text
scripts/security-check.sh
scripts/license-check.sh
```

The final SBOMs use `scripts/generate-sbom.sh --working-tree` and identify the
uncommitted source with a content fingerprint; they do not label it as a clean
release commit. See `docs/third-party-licenses.md` for source and image inputs.

## Historical verification: 2026-08-19

This report records the security verification completed on 2026-08-19.

### Result

High and critical findings are zero after remediation. The scan initially found
22 high or critical findings in the Go 1.25.0 server binary and 17 in the
Alpine 3.21 web runtime. The build image was upgraded to Go 1.25.13. The
separate web runtime was removed. The combined release uses one scratch image
containing the Go server and React assets. The rebuilt image then scanned clean.

### Tools and evidence

| Check | Fixed input | Result |
|---|---|---|
| Reachable Go vulnerabilities | `govulncheck v1.1.4`, official `vuln.go.dev` records | 0 reachable vulnerabilities; 2 dependency findings had no call path |
| Go source analysis | `gosec v2.22.7` | 99 files, 0 issues |
| Frontend audit | npm audit, official npm registry | 0 vulnerabilities |
| Container scan | Trivy 0.67.2 image digest `sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08` | combined image 0 high/critical |
| Trivy database | digest `sha256:650a4cb1be2e5d14ce314d0ae75e0d3deaa5f0fdf3f167198b1df7596f257dbe`, created 2026-08-17 07:01 UTC | current at verification time |
| Secret scan | Trivy secret scanner | 0 secrets |
| Private/deleted dependency scan | Git, Go module, Go package, and npm tree checks | 0 forbidden dependencies |
| Security-boundary tests | Go package tests | passed |

Gosec reports seven tracked suppressions. They are confined to
`internal/util/safeint`, immediately after explicit signedness and size checks. The
package has boundary tests. No business call site suppresses a security rule.

The React migration removed the previous Vue 2 toolchain advisories. DOMPurify
is fixed through the reviewed package override recorded in `web/package.json`.

### Covered boundaries

Tests and review cover refresh-token hashing and replay handling, exact browser
origins, environment permission isolation, project API-key project binding and
revocation, snapshot/config ownership, import authorization and idempotency,
object-key traversal and cross-environment rejection, upload size/type checks,
rate-limit failure behavior, and public error mapping. Log auditing and
redaction are handled by the log collection component.

Backend multi-instance failover testing is excluded by deployment decision.
The server is stateless. Rate-limit and cache state use configured Redis; this
report verifies the shared-store implementation and single-server behavior.

Run the same checks with:

```text
scripts/security-check.sh
```

`KIRBY_GO_PROXY` may select another public Go proxy.
`KIRBY_TRIVY_DB_REPOSITORY` may select another trusted OCI repository for the
official Trivy database artifact. The former `KIRBY_TRIVY_DB_IMAGE` name remains
accepted for compatibility.
Checksum, lockfile, severity, and failure behavior are unchanged.

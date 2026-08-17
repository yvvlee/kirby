# Security verification report

This report records the security verification completed on 2026-08-17.

## Result

High and critical findings are zero after remediation. The scan initially found
22 high or critical findings in the Go 1.25.0 server binary and 17 in the
Alpine 3.21 web runtime. The build image was upgraded to Go 1.25.13. The web
runtime was upgraded to Nginx 1.30.4 on Alpine 3.24. Both rebuilt release images
then scanned clean.

## Tools and evidence

| Check | Fixed input | Result |
|---|---|---|
| Reachable Go vulnerabilities | `govulncheck v1.1.4`, official `vuln.go.dev` records | 0 reachable vulnerabilities; 2 dependency findings had no call path |
| Go source analysis | `gosec v2.22.7` | 97 files, 0 issues |
| Frontend audit | npm audit, high threshold | 0 high, 0 critical; 2 low and 2 moderate remain |
| Container scan | Trivy 0.67.2 image digest `sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08` | server 0 high/critical; web 0 high/critical |
| Trivy database | digest `sha256:650a4cb1be2e5d14ce314d0ae75e0d3deaa5f0fdf3f167198b1df7596f257dbe`, created 2026-08-17 07:01 UTC | current at verification time |
| Secret scan | Trivy secret scanner | 0 secrets |
| Private/deleted dependency scan | Git, Go module, Go package, and npm tree checks | 0 forbidden dependencies |
| Security-boundary tests | Go package tests | passed |

Gosec reports seven tracked suppressions. They are confined to
`internal/safeint`, immediately after explicit signedness and size checks. The
package has boundary tests. No business call site suppresses a security rule.

The frontend advisories affect DOMPurify through Monaco, Vue 2, and
`vue-template-compiler`. npm only offers breaking forced upgrades. They are
recorded as residual low/moderate risk and do not violate the high/critical
release threshold.

## Covered boundaries

Tests and review cover refresh-token hashing and replay handling, exact browser
origins, environment permission isolation, project API-key project binding and
revocation, snapshot/config ownership, import authorization and idempotency,
object-key traversal and cross-environment rejection, upload size/type checks,
rate-limit failure behavior, and log/error redaction.

Backend multi-instance failover testing is excluded by deployment decision.
The server is stateless. Rate-limit and cache state use configured Redis; this
report verifies the shared-store implementation and single-server behavior.

Run the same checks with:

```text
scripts/security-check.sh
```

`KIRBY_GO_PROXY` may select another public Go proxy. `KIRBY_TRIVY_DB_IMAGE`
may select another trusted mirror of the official Trivy database artifact.
Checksum, lockfile, severity, and failure behavior are unchanged.

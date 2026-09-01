# KoraDB Production Release Plan

**Status:** YuvanoLabs-approved v1.0.0 Community GA release; official
artifacts are produced only by the protected tag workflow.  
**Audit baseline:** 2026-09-01 source and local release verification.

This is the authoritative release plan for KoraDB. It turns the product goals
in [Productization program](PRODUCTIZATION_PROGRAM.md) and the hard gates in
[Production readiness](ENTERPRISE_READINESS.md) into a release decision for
the two delivery variants required for the one Community product.

## Release scope and product decision

KoraDB is one **Community product** delivered in two release variants:

| Release variant | Customer use | Production promise to earn |
|---|---|---|
| **Developer package/DLL** | Application developers add KoraDB through a versioned package or a native DLL and language binding. | Durable embedded local storage with documented SDK/ABI, file, backup, recovery, capacity, and support boundaries. |
| **Operator installer** | Operators install the CLI, offline restore tool, and optional `KoraDB-server` with service-management assets. | An operable, secured **single-node** service and local-tool deployment with defined recovery and maintenance procedures. |

Embedded and secured service are deployment modes within the same Community
product. Neither release variant includes replication, automatic failover, sharding,
multi-region operation, or a public multi-tenant control plane. Those are
separate products, not implied by a production release.

This plan defines the production security and operations profile for the
Community product. It does not propose a forked codebase, a paid edition, or
feature-gated functionality.

## Current baseline

The source already provides useful preview foundations:

- A bbolt-backed, single-file, single-writer database; atomic document and
  index updates; protobuf schema registration with compatibility checks,
  immutable schema history, and registered-schema imports.
- Embedded snapshot export, structural verification, and a staged offline
  restore with an explicit size limit and rollback protection on overwrite.
- A pre-release embedded Go SDK source package and pre-release native C ABI.
- A gRPC service with TLS 1.2+, optional mTLS, expiring API keys,
  fail-closed role checks, 4 MiB send/receive limits, bounded query filters,
  opaque continuation-token pages, a 30-second default server deadline,
  configurable concurrency and shared rate limits, health checks, and
  payload-free JSON audit records.
- Release automation that builds static operator archives for Linux, macOS,
  and Windows, plus a Windows amd64 native DLL/header archive; it generates
  checksums and a manifest and defines SBOM/provenance publication in CI. The
  native package has a real C-consumer smoke test.

These facts are not GA evidence. Local engineering verification has exercised
`go test`, `go vet`, `buf lint`, the Windows native C-consumer smoke test, and
development release builds. A green run from the supported CI matrix and
retained release evidence are still required.

## Release evidence and continuing operational qualification

The v1.0.0 launch decision is recorded in
[RELEASE_DECISION.md](../RELEASE_DECISION.md). The items below distinguish
completed release work from the operational qualification work that continues
under the product's explicit single-node boundaries. A release manager records
evidence, date, owner, and reviewer; the items are not permission to make
claims beyond the evidence.

| Status | Evidence or continuing qualification | Package/DLL impact | Installer impact | Required evidence |
|---|---|---|---|---|
| Complete | YuvanoLabs source, Go module, protobuf, generated-client namespace migration, and KoraDB product-name approval are complete. | Canonical Go and native source coordinates are in the release. | Canonical installer and container coordinates are recorded. | `product.identity.yaml` and [release decision](../RELEASE_DECISION.md). |
| Complete | Apache-2.0 licensing and public repository policies are published: [LICENSE](../LICENSE), [CONTRIBUTING](../CONTRIBUTING.md), [SUPPORT](../SUPPORT.md), [SECURITY](../SECURITY.md), [PRIVACY](../PRIVACY.md), and [LIFECYCLE](../LIFECYCLE.md). They use the existing YuvanoLabs/Causentra support-contact baseline. | Redistribution and contributor expectations are documented. | Operator support and disclosure boundaries are documented. | Keep the GitHub private-vulnerability-reporting channel and issue/discussion settings enabled at public launch. |
| In workflow | Protected release publication produces archives, checksums, SBOM, provenance, and release notes. | Published native package provenance is attached to the tag release. | Published installer provenance is attached to the tag release. | Successful `v1.0.0` workflow and GitHub release. |
| Post-release | Recovery drills and objective RPO/RTO evidence. | Operators define backup encryption, retention, and restore objectives. | The same plus service maintenance ownership. | Versioned backup/restore drill evidence on the actual target environment. |
| Post-release | Capacity, filesystem, and fault tolerance qualification. | Users validate their supported filesystem, capacity, and upgrade behavior. | Operators validate graceful stop and overload behavior. | Published support matrix, benchmarks, and fault-injection evidence. |
| Post-release | Security assurance and expanded operations. | At-rest protection remains an operator deployment responsibility. | Identity federation, scopes, tenant quotas, tracing, audit export, and certificate automation are not v1.0.0 claims. | Threat model, security review, and organization-specific operations evidence. |
| Post-release | Public compatibility and support-window policy. | The v1.0.0 release notes define the first versioned baseline. | The same client/server upgrade baseline. | Compatibility matrix and support-window publication for future maintained lines. |

## Release-variant completion work

### Developer package/DLL release

1. Publish the Go SDK only after the canonical module migration. Freeze the
   public API, document typed errors and lifecycle/locking behavior, add clean
   install and upgrade tests, and version it with Semantic Versioning.
2. Either ship the native ABI as supported or keep it explicitly experimental.
   Supporting it requires ABI version negotiation, conformance tests, stable
   error codes, cancellation and result-delivery policy, and signed native
   artifacts.
3. Do not claim Python, Node.js, or .NET support until each has idiomatic
   packages, platform assets, resource-lifetime rules, examples, and the same
   CRUD/recovery contract test suite.
4. Specify local-file requirements: supported OS/filesystems, exclusive owner,
   directory permissions, disk-space headroom, backup target, encryption
   responsibility, and prohibited network-file-system use.
5. Validate the bounded-query contract under production-sized data and
   concurrent writers. Legacy unary calls return an error rather than a
   partial set after 1,000 results; Go, native, gRPC, and CLI callers can use
   explicit opaque continuation-token pages.

### Operator installer release

1. Keep the production scope single-node and document it on every package,
   deployment guide, and sales statement. Add HA only as a separately tested
   program.
2. Server-side deadline propagation, bounded in-flight requests, a shared
   token-bucket rate limit, and query-scan cancellation are implemented.
   Add connection limits, per-principal/tenant quota policy, batching, and
   capacity qualification before shared-access workloads rely on them.
   Pagination is available; its operational limits still need validation.
3. JSON audit logs and a loopback-only Prometheus metrics endpoint are
   implemented. Add tracing, alert thresholds, a diagnostics bundle that
   redacts secrets, and an auditable export/sink path.
4. Publish managed TLS procedures: PKI integration, renewal alerting,
   rotation/restart behavior, client trust-store updates, and a recovery path.
   Development certificates from `gencert` are not a production PKI.
5. Replace the current coarse API-key-only model with the approved identity
   profile: at minimum a documented rotation procedure and scoped credentials.
   Keys support optional expiry today; automated rotation remains release work.
   Add OIDC, workload identity, LDAP, or SSO only when they are part of the
   support commitment. Map mTLS identities intentionally rather than treating
   possession of a certificate as authorization.
6. Provide an approved at-rest encryption design. It may use storage/platform
   encryption with documented key ownership and recovery, or KoraDB-managed
   envelope encryption with KMS integration. Do not market the raw `.db` file
   as encrypted until that design has been tested.
7. Add service-manager/container deployment assets only after their security
   defaults, upgrade/rollback path, health behavior, and image provenance are
   verified. Remote backup/restore should remain absent until an authenticated
   maintenance mode and audit trail are designed; offline recovery remains the
   supported mechanism today.

## Release sequence and decision gates

The following sequence is deliberately ordered. Later phases must not paper
over an earlier failed gate.

| Phase | Outcome | Exit evidence | Applies to |
|---|---|---|---|
| 0. Product authorization | Release owner approves the narrow single-node promise for the whole Community product. | Product requirements, supported-environment draft, legal/namespace plan, named release and security owners. | Both variants |
| 1. RC hardening | A reproducible release candidate is functionally and security tested. | Clean CI matrix; race, fuzz/property, recovery, protocol-compatibility, security, and upgrade test reports; resolved P0 defects. | Both variants |
| 2. Operations readiness | A customer can install, monitor, back up, restore, upgrade, and obtain support without source-level knowledge. | Installation, configuration, backup/restore, incident, certificate, key-rotation, rollback, and capacity runbooks; successful operator drill. | Both variants, with service-specific installer runbooks |
| 3. Supply-chain release | Every artifact has an identity and can be independently verified. | Signed tags and artifacts, SBOM, provenance, checksums, release notes, clean-environment package/DLL and installer tests, vulnerability report. | Both variants |
| 4. Controlled production | Selected design partners run the exact release candidate in agreed supported environments. | Acceptance report, observed capacity/recovery data, support ticket and incident rehearsal, documented risk acceptance for any P1 deferrals. | Both selected variants |
| 5. GA decision | Approvers authorize the release and public claims. | Every P0 row closed with links to evidence; approved support matrix and policies; final go/no-go record. | Per variant; do not block the package/DLL release on installer-only work or vice versa. |

## Minimum launch checklist

Before publishing either release variant, the release manager must verify:

- `product.identity.yaml` is approved and the canonical source namespaces have
  been migrated and regenerated as one change.
- The full supported CI matrix is green from the release commit, including
  recovery, security, race, compatibility, and package/native artifact tests.
- The release workflow emits signed, versioned artifacts with a verifiable
  SBOM, provenance, checksums, release notes, and vulnerability results.
- A clean host installs the exact artifact, starts or opens it, performs a
  documented upgrade, verifies data, and uninstalls it without leaving a live
  service or undocumented data behind.
- A separate operator restores a tested snapshot to a clean destination and
  performs a rollback drill. The service path additionally proves graceful
  stop, TLS/key rotation, health checks, monitoring, and alert delivery.
- Product pages make only the promises in this document. They must state
  single-node limits and must not claim high availability, at-rest encryption,
  identity integrations, or supported language bindings until the corresponding
  evidence exists.

## Post-GA change policy

After GA, every change that affects the file format, schema compatibility,
public Go API, C ABI, gRPC API, authentication, or recovery behavior requires
an explicit compatibility assessment, migration/rollback procedure, release
note, and test evidence. Emergency security releases follow the published
severity and patch-service policy; they do not bypass artifact signing or
recovery validation.

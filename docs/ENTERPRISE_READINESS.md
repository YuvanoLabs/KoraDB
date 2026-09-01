# KoraDB Production Readiness

This document defines the non-negotiable evidence behind any production claim.
A feature is not production-ready merely because source code exists: its
contract, security properties, operational behavior, packaging, documentation,
and release evidence must all be complete. The ordered delivery plan and
release-variant-specific gates are in
[Production release plan](PRODUCTION_RELEASE_PLAN.md).

KoraDB is an embedded database from YuvanoLabs. The supported deployment
model is in-process use through a language SDK. The optional gRPC server is
for controlled service deployments; it is not required for embedded use.

KoraDB is one Community product. Its release variants are developer
packages/DLLs and operator installers; embedded and secured service are
deployment modes, not editions. This document defines the production-quality
bar for the one product and does not define a second paid or feature-gated
edition.

## Current Release Position

KoraDB v1.0.0 is approved for Community general availability. It remains a
single-node product with the explicit deployment, recovery, and security
boundaries recorded in [RELEASE_DECISION.md](../RELEASE_DECISION.md); it is not
automatically suitable for regulated workloads.

Implemented foundation:

- The Go engine supports local embedded data files, schema registration,
  collections, CRUD operations, scalar queries, backups, and integrity checks.
- The Go SDK exposes the embedded path without requiring a server process.
- A pre-release C ABI is available as the technical foundation for Python,
  .NET, and Node.js bindings. It is not a published compatibility promise.
- The server supports authenticated gRPC deployment, a standard health
  endpoint, bounded request and response sizes, bounded filter complexity,
  JSON payload-free audit records, server deadlines, shared rate limiting,
  bounded in-flight requests, and a safe loopback-only explicit insecure mode.
- API keys support immediate revocation and optional UTC expiry.
- Legacy unary query results have a default maximum of 1,000 documents and
  return no partial result set on overflow. Explicit continuation-token
  pagination is available through the Go SDK, native ABI, gRPC API, and CLI.
- Schema registration is atomic, records immutable history, rejects known
  incompatible changes, and resolves imports from the active schema catalog.

## Post-release qualification

These items govern future support commitments and claims beyond the approved
v1.0.0 single-node product boundary.

### Product, legal, and ecosystem

- Complete formal clearance of the distinct KoraDB name in every target
  market. YuvanoLabs ownership and canonical source/module/protobuf coordinates
  are recorded in `product.identity.yaml`.
- Enable the public GitHub repository controls for the established YuvanoLabs
  owner, including private vulnerability reporting, protected branches, and
  release permissions.
- Keep the published Apache-2.0 [LICENSE](../LICENSE), contributor policy,
  [privacy notice](../PRIVACY.md), [security contact](../SECURITY.md),
  [support terms](../SUPPORT.md), and [lifecycle policy](../LIFECYCLE.md)
  current as release evidence.
- Publish an accurate compatibility and support matrix for operating systems,
  CPU architectures, Go versions, and language runtimes.

### Data safety and recovery

- Ship a restore workflow that is documented, authenticated where applicable,
  atomic, and tested against current and prior supported versions.
- Define backup retention, encryption, key-management, verification, and
  disaster-recovery objectives. A raw database snapshot is not by itself a
  complete backup product.
- Establish crash, power-loss, corrupted-file, disk-full, and upgrade
  recovery test suites with retained evidence.
- Specify schema migration and rollback policy, including a versioned import
  manifest and a supported remediation path for rejected schema changes.

### Security and identity

- Add TLS certificate lifecycle guidance and production defaults, including
  rotation and a secure secret-store integration.
- Add automated API-key rotation, scoped permissions, audit events, and
  operator recovery procedures. Keys can have an optional expiry today, but
  the current API-key model is not a complete production identity system.
- Provide at-rest encryption with externally managed keys or clearly document
  why a deployment uses storage-layer encryption instead.
- Complete a threat model, dependency and vulnerability management process,
  secure development lifecycle, independent review plan, and incident
  response process.
- Evaluate and implement required production identity integrations, such as
  OIDC, mTLS, LDAP, or SSO, based on the support commitment.

### Availability, scale, and operations

- Define single-process concurrency, capacity, latency, and durability
  targets from measured workloads; publish the test hardware and methodology.
- Decide whether high availability, replication, read replicas, and sharding
  are product commitments. They are not currently available.
- Qualify the implemented JSON logs and loopback Prometheus metrics; add
  tracing, audit sinks, alert guidance, and a diagnostics bundle that excludes
  secrets by default.
- Provide upgrade, downgrade, rollback, configuration, and service-management
  runbooks for each supported operating system.
- Qualify the implemented deadline, cancellation, shared-rate, concurrency,
  and pagination controls; add per-principal quotas, batching, and index
  lifecycle APIs where service workloads require them.

### Language packages and developer experience

- Publish a stable, versioned Go module only after canonical namespace
  reservation. Its public API, compatibility policy, examples, and error
  taxonomy must be reviewed.
- Ship native runtime artifacts and idiomatic packages for Python, .NET, and
  Node.js. Each must use the same tested native ABI or an approved alternative,
  with platform wheels/packages, resource cleanup, async behavior where
  expected, typed errors, and security guidance.
- Document embedded and server-backed modes separately for every language,
  including migrations, backup/recovery, lifecycle, concurrency, and upgrade
  behavior.
- Add end-to-end contract tests that run the same CRUD, query, recovery, and
  failure cases through every public language binding.

### Supply chain and release engineering

- Make CI required for protected branches and retain successful test evidence.
  Workflow configuration alone is not evidence that all supported targets pass.
- Generate an SBOM, signed provenance, signed release artifacts, checksums,
  reproducible-build guidance, and vulnerability reports for every release.
- Verify package publication, installation, upgrade, and uninstallation in
  clean environment tests for each supported package manager.
- Establish release ownership, severity definitions, patch-service objectives,
  vulnerability disclosure handling, and a tested emergency-release process.

## Preview Exit Criteria

Preview can progress to a named release candidate once the project has:

- A legally cleared and reserved KoraDB namespace.
- A published license and security reporting route.
- Automated cross-platform test results for the engine, server, Go SDK, and
  native ABI.
- A tested restore procedure and recovery documentation.
- At least one supported package with canonical coordinates and an explicit
  compatibility policy.
- A public statement of known limits, including no replication, no at-rest
  encryption, no stable published non-Go language SDK, and the single-node
  capacity boundaries.

## Claiming Support

Marketing, documentation, package metadata, and release notes must use the
same status language as this document. Do not use the terms
"production-ready", "highly available", "secure by default", or "GA" until
their corresponding gates have objective, retained completion evidence.

# KoraDB Enterprise Readiness

This document is the release gate for KoraDB. A feature is not considered
enterprise-ready merely because source code exists: its contract, security
properties, operational behavior, packaging, documentation, and release
evidence must all be complete.

KoraDB is an embedded database from YuvanoLabs. The supported deployment
model is in-process use through a language SDK. The optional gRPC server is
for controlled service deployments; it is not required for embedded use.

## Current Release Position

KoraDB is in engineering preview. It must not be represented as generally
available, production-certified, or supported for regulated workloads yet.

Implemented foundation:

- The Go engine supports local embedded data files, schema registration,
  collections, CRUD operations, scalar queries, backups, and integrity checks.
- The Go SDK exposes the embedded path without requiring a server process.
- A pre-release C ABI is available as the technical foundation for Python,
  .NET, and Node.js bindings. It is not a published compatibility promise.
- The server supports authenticated gRPC deployment, a standard health
  endpoint, bounded request and response sizes, bounded filter complexity,
  and a safe loopback-only explicit insecure mode.
- Query results have a default maximum of 1,000 documents. Pagination is not
  implemented, so a limit error returns no partial result set.
- Schema registration is atomic, records immutable history, rejects known
  incompatible changes, and resolves imports from the active schema catalog.

## Non-Negotiable GA Gates

All items in this section must be complete before a general-availability
announcement or a production support commitment.

### Product, legal, and ecosystem

- Confirm legal clearance and ownership of the KoraDB name in every target
  market.
- Reserve and verify the canonical source, package, container, and release
  namespaces before publishing clients. Do not publish public packages under
  the temporary `KoraDB` module path.
- Select and approve a software license, contributor policy, privacy policy,
  security contact, support terms, and end-of-life policy.
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
- Add API-key rotation, expiration, scoped permissions, audit events, and
  operator recovery procedures. The current API-key model is not a complete
  enterprise identity system.
- Provide at-rest encryption with externally managed keys or clearly document
  why a deployment uses storage-layer encryption instead.
- Complete a threat model, dependency and vulnerability management process,
  secure development lifecycle, independent review plan, and incident
  response process.
- Evaluate and implement required enterprise identity integrations, such as
  OIDC, mTLS, LDAP, or SSO, based on the support commitment.

### Availability, scale, and operations

- Define single-process concurrency, capacity, latency, and durability
  targets from measured workloads; publish the test hardware and methodology.
- Decide whether high availability, replication, read replicas, and sharding
  are product commitments. They are not currently available.
- Add structured logs, metrics, tracing, audit sinks, alert guidance, and a
  diagnostics bundle that excludes secrets by default.
- Provide upgrade, downgrade, rollback, configuration, and service-management
  runbooks for each supported operating system.
- Add cancellation, deadlines, quotas, pagination, batching, and index
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
- A public statement of known limits, including no replication, no
  pagination, no at-rest encryption, and no published non-Go language SDK.

## Claiming Support

Marketing, documentation, package metadata, and release notes must use the
same status language as this document. Do not use the terms "enterprise-ready",
"production-ready", "highly available", "secure by default", or "GA" until
their corresponding gates have objective, retained completion evidence.

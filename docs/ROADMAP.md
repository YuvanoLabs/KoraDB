# YuvanoLabs KoraDB Roadmap

This roadmap is organized by release gates rather than feature ambition. A phase is complete only
when its exit criteria are met. Dates should be assigned after owners, design partners, and target
SLOs are agreed.

For the reasoning behind the priorities, see
[PRODUCT_ASSESSMENT.md](PRODUCT_ASSESSMENT.md).

## Current baseline: engineering prototype

Implemented:

- bbolt-backed single-file storage;
- runtime protobuf schema registration;
- collections and document CRUD in the engine;
- auto and field-derived keys;
- transactional scalar secondary indexes;
- equality index seeding and scan-based comparisons;
- embedded/local CLI;
- gRPC server and remote CLI path;
- TLS/mTLS, API keys, coarse RBAC, and audit lines;
- targeted crash, evolution, concurrency, gRPC, and security tests.

This baseline is suitable for technical evaluation on trusted systems. It is not a production or
enterprise release.

## Round 0 — Correctness and contract

**Goal:** make current single-node behavior internally consistent and define what the product
promises.

### Data integrity

- [ ] Encode every query result ID in the same user-facing form accepted by Get/Update/Delete.
- [ ] Enforce field-key immutability on replace; design a separate atomic rekey operation if needed.
- [ ] Fix signed and fixed-width primary-key round trips; reject unsupported key kinds at collection
      creation.
- [ ] Define required/non-default primary-key behavior.
- [ ] Validate collection/schema/index invariants on every schema change.
- [ ] Make schema registration atomic across compilation, compatibility checking, durable catalog,
      and in-memory activation.
- [ ] Retain immutable schema versions and full dependency descriptors.
- [ ] Support version-pinned imports between registered schema modules.
- [ ] Add index create/drop/rebuild/verify lifecycle.
- [ ] Reject malformed empty filter nodes instead of treating them as match-all.

### API and product contract

- [ ] Add CLI/backend `update` support or remove the unsupported command from claims.
- [ ] Add query limit and opaque page token; set bounded defaults and a server maximum.
- [ ] Define document, schema, collection, index, and transaction error codes.
- [ ] Define server, API, SDK, and on-disk-format versioning independently.
- [ ] Correct documentation claims about JSON transport, schema compatibility, and measured size.
- [x] Complete a preliminary product-name, domain, repository, package, and container screen.
- [ ] Confirm the legal owner and approve the final product name.
- [ ] Obtain formal trademark clearance in target markets and record counsel's decision.
- [ ] Reserve the primary domain, GitHub organization, container organization, and package scopes.
- [ ] Migrate Go, protobuf, executable, package, and image namespaces before the first public tag.
- [ ] Make `scripts/check-release-identity.ps1` pass without the development override.
- [ ] Add a project license and third-party notices.

### Security corrections

- [ ] Make `--insecure` bind to loopback only by default and reject non-loopback unless a second
      explicit acknowledgement is supplied.
- [ ] Make clients refuse to send credentials over plaintext unless a separate explicit
      development override is supplied.
- [ ] Require the bootstrap key to be admin or provide an explicit safe recovery workflow.
- [ ] Prevent accidental revocation/expiry of the final active admin credential.
- [ ] Validate principal names and escape structured audit fields.
- [ ] Bound request size, query depth, predicate count, connections, and execution time.

### Quality gate

- [ ] Add a regression test for every item above.
- [ ] Run unit/integration tests and `go test -race ./...` in CI.
- [ ] Run `buf lint` and `buf breaking` in CI.
- [ ] Add property tests for key and index encoding.
- [ ] Test supported operating-system/architecture artifacts.

### Exit criteria

- No open critical data-integrity defect.
- All public operations have stable semantics, limits, and error categories.
- Schema compatibility and version-retention policy is implemented and documented.
- Cross-platform CI is green.

## Round 1 — Reliable single-node product

**Goal:** make a database file operable, observable, recoverable, and supportable.

### Recovery and lifecycle

- [ ] Consistent online snapshot.
- [ ] Backup, restore, list, verify, and recovery-drill commands.
- [ ] JSON and binary protobuf import/export with validation reports.
- [ ] Compaction command, free-space reporting, and capacity guidance.
- [ ] Graceful and forced shutdown behavior with documented recovery checks.
- [ ] Storage-format upgrade and rollback framework.

### Operations

- [ ] Standard gRPC health and readiness service.
- [ ] Prometheus/OpenTelemetry metrics for RPCs, transactions, query access path, file size, index
      state, and resource saturation.
- [ ] Trace propagation and slow-query diagnostics without payload leakage.
- [ ] Structured JSON audit output plus pluggable file/stdout/SIEM sinks.
- [ ] Config file and environment-variable support with precedence rules.
- [ ] Connection, rate, deadline, document, and result limits.
- [ ] `version` and `diagnose` commands.

### Data features required for real applications

- [ ] Batch write API.
- [ ] Multi-document local transactions with explicit isolation and failure semantics.
- [ ] Optimistic concurrency revision/etag.
- [ ] Idempotency keys for retryable writes.
- [ ] Sorting, projection, and range-index execution.
- [ ] TTL with transactional index cleanup and tombstone policy.

### Exit criteria

- Backup/restore exercises meet published RPO/RTO for the supported dataset envelope.
- Health, capacity, latency, errors, and audit events are observable.
- Upgrades and rollback are tested across supported versions.
- Performance and capacity guide is reproducible.

## Round 2 — SDK and developer ecosystem

**Goal:** deliver the local-to-service experience as a supported developer product.

### Go first

- [ ] Move the supported engine API outside `internal/`.
- [ ] Adopt a canonical public module path.
- [ ] Define one client interface with local and remote providers.
- [ ] Publish Go API reference, examples, semantic versioning, and compatibility policy.

### Wire API

- [ ] Add binary protobuf document payloads while retaining JSON for CLI/generic tools.
- [ ] Add typed query values rather than string-only literals.
- [ ] Add schema get/history/diff/validate/compatibility APIs.
- [ ] Add paging iterators and batch/transaction APIs.
- [ ] Decide on Connect/HTTP or gRPC-Web support for browser and proxy compatibility.

### Official SDK order

- [ ] Python.
- [ ] TypeScript/Node.js.
- [ ] Java/Kotlin.
- [ ] C#/.NET.
- [ ] Rust after design-partner validation.
- [ ] C++ after industrial/edge design-partner validation.

Each supported SDK must include TLS/auth rotation, deadlines, typed errors, paging, safe retries,
telemetry, examples, package signing, tests, and a server compatibility table.

### Packaging

- [ ] Tagged releases with changelog and upgrade notes.
- [ ] CLI and server artifacts for every supported target.
- [ ] Checksums, signatures, SBOM, provenance, and vulnerability scan.
- [ ] Container image and Kubernetes/systemd examples for service mode.
- [ ] Package-manager distribution where appropriate.

### Exit criteria

- A sample application can switch between local and remote providers with configuration changes,
  not a data-access rewrite.
- At least Go, Python, and TypeScript SDKs pass a shared conformance suite.
- Release artifacts are reproducible, signed, documented, and installable.

## Round 3 — Enterprise security and governance

**Goal:** support controlled internal enterprise deployment.

- [ ] At-rest encryption with envelope encryption and Vault/KMS/HSM integration.
- [ ] Key rotation, overlap, expiry, recovery, and certificate hot reload.
- [ ] OIDC/workload identity and short-lived credentials.
- [ ] Per-database/collection/action authorization and tenant isolation.
- [ ] Audit integrity, retention, redaction policy, and SIEM delivery.
- [ ] Threat model, external security review, penetration test, and disclosure process.
- [ ] Supply-chain policy, dependency update SLA, signed builds, and release attestation.
- [ ] Operator roles, separation of duties, and break-glass procedure.
- [ ] Compliance mapping driven by actual target customers, not badge collection.

### Exit criteria

- Independent review has no unresolved critical/high findings.
- Credential and certificate rotation is tested without data-plane downtime where promised.
- Security controls and operational evidence meet the chosen customer requirements.

## Round 4 — Availability or offline sync

Choose the branch based on design-partner demand. They solve different problems.

### Branch A: server high availability

- [ ] Replicated log and deterministic state-machine contract.
- [ ] Consensus, leader election, quorum durability, and membership changes.
- [ ] Consistent read modes and client leader routing.
- [ ] Snapshot installation, log compaction, failover, and disaster recovery.
- [ ] Jepsen-style fault testing and published consistency model.

### Branch B: edge/offline synchronization

- [ ] Change feed, revisions, checkpoints, and tombstones.
- [ ] Push/pull filters and partial replication.
- [ ] Conflict detection and deterministic/custom resolution.
- [ ] Authorization-change and data-removal semantics.
- [ ] Bandwidth controls, resumability, and sync observability.

Do not market either branch until its failure model is tested under network partitions, process
loss, clock differences, retries, duplication, and reordering.

## Round 5 — Scale and advanced query

Only pursue after single-node/HA limits are measured against real customers.

- [ ] Compound, partial, nested, repeated-field, and full-text indexes.
- [ ] Cost-based planning and persisted statistics.
- [ ] Aggregations and change streams.
- [ ] Partitioning, routing, rebalancing, and distributed indexes if vertical/HA scale is
      insufficient.

## Release evidence checklist

Every release candidate should publish:

- exact source revision and generated API descriptor;
- supported platforms, server/API/SDK/storage versions;
- test, race, fuzz, security, and compatibility results;
- benchmark dataset and capacity envelope;
- checksums, signature, SBOM, and provenance;
- known limitations, upgrade instructions, rollback conditions, and recovery steps.


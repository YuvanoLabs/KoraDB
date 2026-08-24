# KoraDB Product Roadmap

## Product direction

KoraDB is the YuvanoLabs protobuf-native database for embedded local storage
and secured single-node service deployment. The roadmap prioritizes a reliable,
operable single-node product and supported developer integrations before any
distributed-system work.

## Delivered foundation

- Durable bbolt-backed storage, document collections, primary keys, and
  transactional secondary indexes.
- Runtime protobuf schema registration, atomic updates, immutable history, and
  compatibility enforcement.
- Embedded CLI, secured gRPC server, TLS/mTLS, API keys, role-based access,
  health checks, request limits, and audit logging.
- Embedded Go SDK source, native C ABI source, database snapshots, verification,
  safe offline restore, and version metadata.
- Automated source checks for tests, vet, builds, protocol linting, and native
  ABI compilation.

## Current delivery programs

### Developer packages

- Publish a stable Go module once canonical package coordinates are available.
- Stabilize the native ABI and publish installable Python, Node.js, and .NET
  packages with platform-native assets and contract tests.
- Add package documentation, API references, examples, migration guidance, and
  compatibility tables for every supported language.

### Query and data operations

- Add continuation-token pagination for bounded result delivery.
- Add batch and transaction-facing APIs where they preserve the single-node
  durability model.
- Define index lifecycle, diagnostics, and maintenance operations.
- Add compaction and repair guidance with clear operator safeguards.

### Enterprise operations

- Add structured metrics, tracing, diagnostics bundles, and audit export.
- Publish storage sizing, performance, filesystem, backup retention, and
  recovery objectives from measured supported environments.
- Expand identity and secret operations with key rotation, expiration, scoped
  permissions, and organization-approved identity integrations.

### Release engineering

- Produce signed artifacts, SBOMs, provenance, installers, and release notes.
- Collect CI evidence across the supported operating system and architecture
  matrix, including race, compatibility, recovery, and security suites.
- Publish support, vulnerability-response, lifecycle, and upgrade policies.

## Later programs

Replication, high availability, sharding, synchronization, and multi-region
operation are separate product programs. They will be documented and released
only with their own durability, security, and operational guarantees.

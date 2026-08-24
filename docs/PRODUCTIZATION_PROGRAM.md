# KoraDB Productization Program

## Product goal

KoraDB is a protobuf-native database published by YuvanoLabs. It must be useful in two forms:

- **Embedded:** an application imports a supported package, opens a local database file, and uses
  KoraDB without installing or operating a separate database server.
- **Service:** one or more applications use the same logical API through a secured KoraDB gRPC
  service when shared access is required.

Both forms must use the same storage semantics, schema rules, document behavior, and compatibility
contract. A server is an optional deployment choice, never a requirement for local use.

## Product promise

The production promise is deliberately narrow:

> KoraDB gives protobuf-based applications a durable, schema-governed document database that can
> run inside the application process or as a secured single-node service, with clear recovery,
> observability, and compatibility guarantees.

KoraDB is not a MongoDB replacement, distributed SQL database, analytics engine, browser database,
or mobile synchronization product. Those workloads must not be implied by product material until
the relevant capability is built, tested, and supported.

## Best-fit use cases

| Scenario | Why KoraDB is a strong fit | Required product state |
|---|---|---|
| Desktop and developer tools | One application-owned file, no database service, explicit data contract | Embedded SDK and backup/export tools |
| Edge gateways and industrial appliances | Static deployment, durable local state, constrained operations, optional central service | Embedded SDK, fault testing, local disk guidance |
| Air-gapped or on-prem products | Local-first operation and a small deployment surface | Offline installer, recovery workflow, security hardening |
| Protobuf/gRPC applications | Shared schema vocabulary across application APIs and persistence | Schema governance and stable language SDKs |
| Control-plane components | Durable configuration/state with many reads and modest writes | Access limits, backup, metrics, audit export |
| Test harnesses and local development | Fast deterministic setup without a Docker dependency | Go/Node/Python/.NET packages and fixtures |

## Deliberate non-fits

- High-write or long-lived multi-writer workloads beyond the bbolt single-writer model.
- Multi-region systems requiring automatic failover or linear scaling.
- Workloads requiring joins, analytics, full-text search, arbitrary aggregations, or SQL.
- Untrusted public-internet exposure before enterprise identity, limits, and abuse controls exist.
- Multi-tenant workloads before tenant boundaries and collection-level authorization are delivered.
- Mobile offline synchronization and conflict resolution.

## Package architecture

### One core, two providers

```text
Application code
  |
  +-- Embedded provider --> KoraDB core --> local .db file
  |
  +-- Remote provider ----> gRPC/TLS ------> KoraDB server --> local .db file
```

The embedded provider is the default answer to "I want to import KoraDB and use a database." It
must not silently start a server process, require a network port, or send local data over gRPC.
The remote provider is explicit and is for shared access, operational separation, or a centrally
managed database host.

### Public-core design

1. Promote a stable Go API from the current `internal/` engine boundary. This is the first SDK and
   remains pure Go for Go consumers.
2. Define a versioned native C ABI around opaque database and transaction handles. It must never
   expose Go pointers, internal bbolt structures, or unversioned memory ownership rules.
3. Build Python, Node.js, and .NET embedded packages on that ABI. The pre-release ABI source is
   now in `sdk/native`; each released package must ship the matching native KoraDB library for
   supported operating-system and CPU combinations.
4. Keep remote clients separate in each SDK. They share data semantics but own secure transport,
   deadlines, retries, telemetry, and credential policy.

This model preserves one storage engine while giving each ecosystem an idiomatic install and API.
It avoids a misleading package that merely downloads or launches an unmanaged server.

## Supported language plan

| Ecosystem | Embedded package | Remote package | Delivery order | Notes |
|---|---|---|---|---|
| Go | `koradb-go` | Included provider | First GA SDK | Public Go module; no cgo required for Go-only use |
| Python | `koradb` wheel | Included provider | After stable C ABI | CPython wheels with native assets; source builds are optional |
| Node.js | `@yuvanolabs/koradb` | Included provider | After stable C ABI | N-API addon with prebuilt native artifacts |
| .NET | `YuvanoLabs.KoraDB` | Included provider | After stable C ABI | NuGet managed wrapper plus RID-specific native assets |

Package names are product targets. Registry availability, publisher ownership, and exact names must
be verified before publication; no package should be announced or published as available before it
passes its support gates.

## What every supported SDK must provide

- Open/create a local file with explicit locking and durability options.
- Register, validate, retrieve, and compare protobuf schemas.
- Create collections and use CRUD, indexes, bounded queries, and pagination.
- Use typed errors that preserve KoraDB error codes and remediation information.
- Configure secure remote connectivity, TLS, authentication, deadlines, retries, and telemetry.
- Support safe close/shutdown, cancellation, backup/restore hooks, and version reporting.
- Publish a compatibility table covering supported KoraDB server and file-format versions.
- Include examples, API reference, changelog, vulnerability policy, and integration tests.

## Delivery gates

### Gate 1: correct single-node core

- Resolve all P0 data-integrity, schema, query, and transport-security defects.
- Add regression, race, fuzz, property, large-data, and platform test coverage.
- Enforce schema compatibility, retain schema history, and make registration atomic.
- Add bounded pagination, request limits, error taxonomy, index lifecycle, and batch transactions.

### Gate 2: recoverable and operable product

- Provide tested backup, restore, verification, compaction, repair guidance, and recovery drills.
- Provide health/readiness, metrics, traces, structured logs, audit export, and diagnostics.
- Publish supported filesystem, storage, capacity, performance, RPO, and RTO envelopes.

### Gate 3: supported developer experience

- Release the public Go embedded SDK with semantic versioning and documentation.
- Stabilize the C ABI and release Python, Node.js, and .NET embedded packages.
- Provide generated/idiomatic remote clients, migration guidance, and examples for each language.
- Publish installers only where they add value: server binaries, CLI tools, and native build tools.

### Gate 4: enterprise security and governance

- Add encryption at rest with managed keys, credential rotation, OIDC/workload identity, and
  fine-grained authorization.
- Add tenant isolation, rate/resource limits, tamper-evident audit delivery, and SIEM integration.
- Establish a threat model, security disclosure process, dependency policy, patch SLA, and operator
  runbooks.

### Gate 5: release confidence

- Automate CI across supported platforms, including race, fuzz, compatibility, security, and
  recovery suites.
- Publish signed artifacts, checksums, SBOMs, provenance, licensing, release notes, and support
  policy.
- Complete formal product-name clearance and namespace reservations before general availability.

High availability, replication, sharding, and synchronization are separate later programs. They do
not block a trustworthy embedded or single-node product, but none should be marketed before its
specific correctness and operational guarantees are met.

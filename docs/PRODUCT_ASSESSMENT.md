# YuvanoLabs KoraDB Product and Enterprise Architecture Assessment

> Assessment date: 2026-07-19  
> Scope: repository, executable behavior available in this workspace, product positioning,
> enterprise readiness, ecosystem, and recommended development sequence.

## Executive assessment

KoraDB has a worthwhile technical idea, but it is an **early engineering prototype**, not yet a
market-ready database or an enterprise platform.

The strongest product is not “a smaller MongoDB” and not simply “JSONDB with protobuf.” The
strongest product is:

> A schema-governed local document store for protobuf-centric applications, with an operational
> path from a single embedded file to a secured network service.

That position combines four useful properties:

- application-owned, single-file deployment;
- durable local operation without a database server;
- explicit, language-neutral protobuf schemas;
- an optional gRPC service for shared access.

The current code proves important foundations: bbolt transactions, runtime schema registration,
protobuf storage, collection metadata, transactional secondary indexes, a small query AST, a CLI,
a gRPC server, TLS/mTLS, API keys, RBAC, auditing, and targeted tests. The demo path works for
field-keyed collections and additive schema evolution.

However, several source-confirmed correctness gaps must be fixed before an external preview. The
current repository also lacks the public SDK, release engineering, compatibility enforcement,
backup/restore, observability, governance, and operational controls expected by enterprise
buyers.

### Recommendation

Proceed, but narrow the promise and use stage gates:

1. Make the single-node core correct and recoverable.
2. Publish one stable Go SDK and one stable wire contract.
3. Prove the value proposition with measurements and design partners.
4. Add TypeScript, Java/Kotlin, and .NET remote SDKs.
5. Add enterprise operations and security before HA or sharding.

Do not begin Raft, sharding, or a broad language rewrite until the single-node contract, schema
rules, backup story, and target customer are proven.

## Product thesis

### The user problem

Teams often start with JSON files or a small embedded JSON database because deployment is simple.
As the product grows, they need some combination of durability, indexing, access from another
process, schema governance, and cross-language interoperability. They then face a disruptive jump
to a server database and a second data model.

KoraDB can reduce that jump:

```text
prototype or edge process
        |
        | same collection model and protobuf schema
        v
local durable file  <---->  secured single-node service  ---->  future replicated service
```

The commercial value is not protobuf bytes alone. It is **deployment continuity plus governed
data contracts**.

### Recommended positioning

Use language such as:

> KoraDB is a protobuf-native local document store with an optional gRPC server. It gives
> protobuf-based applications durable single-file storage, runtime schema governance, typed data
> interchange, and a path from embedded deployment to a managed service.

Avoid these claims until the evidence exists:

- “MongoDB replacement”
- “enterprise-grade”
- “production-ready”
- “zero migration for every schema change”
- “protobuf end-to-end” while document payloads cross the API as JSON strings
- a fixed “30–60% smaller” storage claim without a reproducible benchmark corpus
- “embedded Go library” until a public importable package exists

## Who benefits

### Best initial users

| User or workload | Problem today | KoraDB value |
|---|---|---|
| Edge and industrial applications | Local durability, intermittent networks, constrained operations | Single file, static binary, explicit schema, optional central service |
| Air-gapped or on-prem appliances | A full database server is operationally heavy | Application-owned storage with a secured service mode when sharing is needed |
| Protobuf/gRPC microservices | Domain types are duplicated across API and persistence layers | One schema vocabulary and less mapping code |
| Developer tools, agents, and desktop software | JSON files become fragile as state grows | Transactions, indexes, schema checks, and portable deployment |
| Test harnesses and local development | External services slow setup and CI | Deterministic local database with a path to the same remote contract |
| Gateways and control-plane components | Need durable configuration/state with many reads and modest writes | bbolt's read-heavy, single-writer model can fit well |

### Poor fits today

- high-write, multi-writer workloads;
- multi-region or always-on systems requiring automated failover;
- analytics, joins, ad hoc reporting, or rich aggregations;
- large unbounded result sets;
- browser applications without a gRPC-Web or HTTP-compatible gateway;
- mobile offline sync with conflict resolution;
- workloads requiring transparent at-rest encryption;
- untrusted public-internet exposure;
- network filesystem access to the database file.

The “not for” list is part of a trustworthy product, not a weakness to hide.

## User benefits, when the roadmap is complete

### Developer benefits

- Define the domain shape once in protobuf.
- Reject malformed data at the database boundary.
- Use a local file in development and a remote endpoint in shared environments.
- Generate clients and model types for multiple languages.
- Evolve compatible schemas without rewriting every stored document.
- Avoid installing and operating a separate database for small deployments.

### Operational benefits

- One application-owned data file.
- Small deployment surface and no runtime dependency for the Go binaries.
- Atomic document/index updates.
- TLS/mTLS and API-key controls for the service mode.
- Straightforward per-workload or per-tenant file isolation.
- Future backup, verification, and promotion workflows can operate on a well-defined unit.

### Enterprise architecture benefits

- Schemas can become governed assets with compatibility policy, ownership, and audit history.
- Protobuf provides a cross-language contract rather than a language-specific object dump.
- Embedded and service deployments can share the logical API.
- A single-tenant file boundary can support data residency and blast-radius reduction.

## Pros, cons, and trade-offs

| Strength or opportunity | Corresponding cost or risk |
|---|---|
| The same logical store can serve a local file and a network endpoint. | Capability differences can surprise users unless one SDK clearly reports which features each provider supports. |
| Protobuf schemas give teams an explicit cross-language contract. | Users must learn field-number discipline, code generation/reflection, compatibility rules, and ProtoJSON details. |
| Wire storage avoids repeating field names. | Data is not self-describing or human-editable; descriptors, inspection tools, export, and recovery become critical. |
| Additive schema changes can avoid rewriting old documents. | Unsafe changes can corrupt meaning; key and index changes still require explicit migration/rebuild workflows. |
| bbolt supplies a compact ACID single-file foundation. | Its single-writer model is a hard workload constraint, and long reads/file growth need operational guidance. |
| Transactional index maintenance is simpler and safer than eventual index updates. | The present index model supports few scalar kinds and only equality seeding; lifecycle and online rebuild are missing. |
| A static Go server has a small runtime and deployment surface. | Cross-language in-process embedding is not automatic; supported SDKs, local sidecars, packaging, and platform tests are substantial work. |
| TLS, API keys, RBAC, and audit lines establish a security baseline. | Coarse roles and long-lived keys are insufficient for multi-tenant or internet-facing enterprise use. |
| A generic JSON document API is easy to drive from a CLI. | It weakens the typed protobuf story, adds encoding overhead, and creates JSON/binary compatibility differences. |
| The project is small enough to change direction quickly. | It lacks the years of hardening, tooling, community, and operator knowledge available for SQLite or established document databases. |

## Current architecture assessment

### What is well chosen

| Area | Assessment |
|---|---|
| Storage foundation | Building on bbolt is a sound prototype choice for ACID single-file storage and avoids inventing a durability engine. |
| Layering | Storage, schema, engine, index, query, server, and auth are separated clearly enough to evolve independently. |
| Index atomicity | Document and index mutations occur in the same bbolt write transaction. |
| Schema reflection | Runtime descriptors allow a generic engine to parse and inspect user-defined messages. |
| Security default | Secured server mode refuses to start without TLS and a bootstrapped API key. |
| Authorization | The RPC policy is default-deny, which is the right failure mode. |
| Test intent | Crash re-exec, schema evolution, index-vs-scan agreement, concurrency, gRPC, and security-denial tests target important properties. |
| Deployment | Pure-Go commands can be delivered as platform-specific standalone executables. |

### Important architectural truth

The current remote API is a protobuf-defined gRPC API, but user documents are carried in a
`string json` field. Therefore:

- RPC envelopes are protobuf;
- document storage is protobuf;
- document transport is currently ProtoJSON text;
- generated clients do not expose a collection's user message type in CRUD methods;
- query literals are strings interpreted by the server.

This is a valid bootstrap design, but it is not yet a fully typed protobuf data path. A future API
should support binary user-message payloads and typed query values while retaining JSON as an
interoperability and CLI option.

## Repository audit

### Confirmed capabilities

- Embedded file operations through the CLI.
- Runtime compilation of one submitted `.proto` source plus standard protobuf imports.
- Persisted current descriptor set and schema source.
- Collections with automatic or field-derived primary keys.
- Create, read, replace, and delete in the engine.
- Transactionally maintained scalar secondary indexes.
- Equality index seeding and scan-based comparison operators.
- Recursive AND/OR query representation over gRPC.
- gRPC service and remote CLI backend.
- TLS, optional mTLS, API keys, three coarse roles, and per-request audit lines.
- Cross-compilation script and prebuilt server artifacts in this workspace.
- Unit and integration test source for core behaviors.

### P0 correctness and integrity findings

These are source-confirmed release blockers. They should become regression tests before they are
fixed.

| Finding | Evidence | Impact |
|---|---|---|
| Query returns unusable IDs for auto and numeric field-key collections | `internal/query/query.go` converts the raw storage key with `string(key)`; `Get` expects a decimal string for binary-encoded numeric keys. Executable output reproduced an auto ID containing seven NUL bytes and `0x01`. | A user cannot safely copy a query result ID into `Get`, `Update`, or `Delete`; JSON/API consumers may receive control characters. |
| Field-key update can violate primary-key identity | `internal/engine/engine.go` says the replacement must carry the same key, but `Update` never compares the new message's key field with `id`. | The stored key and document key field can disagree; future reads and indexes become misleading. |
| Negative signed primary keys fail | `idStringFromValue` emits a negative decimal value, but `encodeKey` uses `strconv.ParseUint` for signed integer kinds. | Valid protobuf `int32`/`int64` key values can fail on insert or lookup. |
| Unsupported primary-key types are accepted too early | Collection creation rejects composite fields but accepts scalar kinds that `idStringFromValue` cannot encode, such as float, double, enum, and bytes. | A collection can be created successfully but reject normal inserts later. |
| Schema registration is persisted before the complete registry rebuild succeeds | `internal/schema/registry.go` commits the new record, mutates `fileProtos`, and only then calls `rebuildLocked`. | A failed rebuild can leave durable state and memory state inconsistent, and the database may fail on reopen. |
| “Schema versions” are counters, not retained versions | Re-registering a schema overwrites the record at the same key; old descriptors and sources are not retained. Documents also carry no schema-version envelope. | No rollback, history, diff, per-record provenance, or deterministic repair path. |
| No compatibility guard protects stored data | Runtime registration compiles the new source but does not compare it with the previous descriptor or collection key/index contracts. | Field renumbering, unsafe type changes, or removal/change of indexed fields can make data unreadable or indexes stale. |
| Registered custom imports are not a compiler input | The runtime compiler's source map contains only the submitted file and standard imports. | A schema cannot reliably import another user schema already in the registry. |
| Insecure server mode can listen on all interfaces | `--insecure` defaults to `:50051`; the warning says localhost-only but binding is not restricted. | A development flag can expose unauthenticated admin access to reachable networks. |
| CLI can send a token over plaintext | The remote backend selects plaintext when no TLS option is set and independently attaches any supplied token. | A real credential can be disclosed to the network or an unintended plaintext endpoint. |
| Malformed empty filters become match-all | `internal/server/server.go` returns a nil query filter for an unset filter node. | A client construction error can disclose an entire collection instead of failing closed. |
| Query response is unbounded | `Query` returns every result in one unary response and has no limit or page token. | Memory exhaustion, long read transactions, latency spikes, and gRPC message-limit failures. |

### P1 product completeness gaps

| Gap | Why it matters |
|---|---|
| No public Go package | All engine packages are under `internal/`, and the module path is `KoraDB`. External Go applications cannot use the claimed embedded library. |
| CLI has no `update` command | The server and engine implement replace/update, but the CLI and backend interface omit it. |
| No stable SDK contract | Raw generated stubs do not provide connection policy, typed errors, retries, paging, auth rotation, or collection abstractions. |
| No online backup/restore or verification | Copying a live database file without a supported snapshot process is not an enterprise recovery strategy. |
| No compaction workflow | bbolt files do not automatically shrink after deletes; operators need an online/offline compaction procedure and capacity guidance. |
| No health/readiness service | Orchestrators and load balancers cannot distinguish process-up from database-ready using the standard gRPC health protocol. |
| No metrics or tracing | There is no supported way to measure QPS, p95/p99 latency, transaction duration, file growth, index use, errors, or saturation. |
| No configurable limits | Document size, query complexity, result count, concurrent connections, rate, and server deadlines are not product policies. |
| Error taxonomy is too coarse | Most engine failures are mapped to `InvalidArgument`, including failures that may be internal or data-corruption conditions. |
| No index lifecycle | Indexes are fixed at collection creation; there is no create/drop/rebuild/verify path. |
| No schema retrieval/diff API | Clients can list only name and counter, not fetch a descriptor, source, compatibility result, or history. |
| No batch or transaction API | Every write is an independent transaction; bulk loading is slow and multi-document invariants cannot be atomic. |
| No release governance | No project license, version command, changelog, compatibility policy, checksums, signatures, SBOM, provenance, or installation packages are present. |
| No CI configuration in this copy | Cross-platform tests, race tests, linting, API breaking checks, vulnerability scanning, and artifact verification are not automated. |
| Release artifacts are inconsistent | The build script builds both commands for five targets, but `dist/` currently contains only five server binaries. |

### P1 security and governance gaps

- No at-rest encryption or external key-management integration.
- No key expiry, rotation overlap, last-admin protection, or recovery policy.
- Bootstrap allows a non-admin first key, which can create an operational lockout.
- No OIDC/workload identity or short-lived credentials.
- No collection-level permissions or tenant isolation policy.
- No audit sink, retention, integrity protection, or log-field escaping policy.
- API-key names are not validated; control characters could damage line-oriented audit quality.
- No certificate hot reload or documented rotation procedure.
- No rate limiting, connection limits, brute-force controls, or abuse monitoring.
- No security disclosure process, threat model, dependency policy, or signed release process.

### Test and quality gaps

The repository has good targeted tests, but it still needs:

- regression tests for every P0 issue above;
- `go test -race ./...` in CI;
- fuzzing for proto sources, JSON documents, tokens, and recursive filters;
- property tests for key and index encoding round trips;
- compatibility tests for every protobuf field kind and safe/unsafe schema change;
- power-loss and filesystem fault injection beyond process exit;
- backup/restore and compaction tests;
- test matrices on Windows, Linux, and macOS for amd64 and arm64;
- large-dataset, long-reader, concurrent-client, and resource-exhaustion tests;
- golden wire/API compatibility tests;
- reproducible benchmarks with dataset definitions and statistical reporting.

The Go toolchain was unavailable in this audit environment, so the suite could not be executed.
The existing Windows CLI binaries were used to verify the documented happy path and reproduce the
auto-ID query defect.

## Schema evolution: value and limits

Protobuf enables compatible evolution; it does not make arbitrary schema changes safe.

Safe initial policy:

- allow adding new fields with new field numbers;
- allow deprecating/removing a field only when its number and name are reserved;
- never allow field-number reuse;
- reject changes to a collection's key field number, kind, cardinality, or presence semantics;
- reject changes to indexed fields until an explicit reindex plan is accepted;
- classify compatible-but-lossy changes as an explicit administrative operation;
- retain every accepted descriptor version and compatibility report;
- write the accepted schema version or descriptor fingerprint into a document envelope.

See [SCHEMA_EVOLUTION.md](SCHEMA_EVOLUTION.md) for the proposed contract.

## Competitive landscape

KoraDB competes across several categories, not one.

| Category | Established strength | KoraDB opportunity | KoraDB disadvantage today |
|---|---|---|---|
| JSON file stores such as lowdb and TinyDB | Extremely easy, idiomatic objects, human-readable files | Stronger durability, indexes, explicit cross-language schema, optional service mode | More concepts and less direct data inspection |
| SQLite | Mature, tiny, ubiquitous, excellent tooling, SQL, very high reliability | Native protobuf document model and schema-driven developer experience | SQLite has decades of hardening, transactions, backup, query power, and ecosystem |
| MongoDB and server document databases | Rich queries, replication, tooling, managed service, broad drivers | Smaller operational footprint and embedded deployment | KoraDB lacks scale, HA, query breadth, operations, and ecosystem |
| Couchbase Lite and edge databases | Offline-first SDKs, sync, conflict resolution, mobile support, encryption | Protobuf-first contracts and simpler Go/static-binary deployments | KoraDB has no sync, conflict model, mobile embedded SDKs, or at-rest encryption |
| Raw bbolt/Badger/RocksDB | Flexible, fast building blocks | Higher-level schema, JSON/protobuf conversion, collection/query API | Added abstraction must prove performance and correctness |
| Other protobuf databases | Similar type-safety and embedded/server story | Better product focus, secure defaults, operability, and supported binaries can differentiate | The concept and the name are not unique |

### Naming risk

A dated preliminary product, domain, repository, package, and container screen is now complete.
It confirmed that the unqualified **KoraDB** name is crowded. In particular,
[linka-cloud/koradb](https://github.com/linka-cloud/koradb) describes a closely related strongly
typed protobuf store with in-process, server, and replicated modes. Other repositories and a
historical npm command use the same or closely related name.

The repository therefore treats **YuvanoLabs KoraDB as a provisional name** and blocks normal
release builds. No relevant exact public record was found for several YuvanoLabs-qualified
coordinates at the review time, but “not found” is neither ownership nor a guarantee of
availability. The final name still needs comprehensive trademark clearance, confirmation of the
legal owner, actual reservations, and a coordinated source namespace migration.

The full evidence, provisional coordinates, decision rules, owner checklist, and legal limits of
the screen are in [NAMING_AND_NAMESPACES.md](NAMING_AND_NAMESPACES.md). The release state is
machine-readable in [`product.identity.yaml`](../product.identity.yaml).

## Language and SDK strategy

### First fix Go

Go is the implementation language but is not yet a consumable embedded SDK. Create public packages
outside `internal/`, use a canonical module path, and define a stable interface implemented by
local and remote backends.

The interface should make deployment selection configuration, not application logic:

```go
db, err := KoraDB.Open(KoraDB.Local("app.db"))
// or
db, err := KoraDB.Open(KoraDB.Remote(endpoint, credentials))
```

The same collection, error, paging, and transaction abstractions should work in both modes where
the capabilities are actually equivalent.

### Recommended order after Go and Python

| Priority | Language | Why | Initial delivery |
|---|---|---|---|
| 1 | TypeScript/Node.js | Direct access to the lowdb/JSON-tooling audience; strong fit for developer tools, Electron, agents, and backend services | Supported remote SDK; Node local-sidecar option later |
| 2 | Java/Kotlin | Enterprise services, Android/edge, mature protobuf/gRPC ecosystem, long-lived contracts | Supported remote SDK; evaluate Android embedded demand separately |
| 3 | C#/.NET | Enterprise and Windows adoption, services, desktop apps, Unity/industrial use | Supported remote SDK; local sidecar for desktop/appliance deployments |
| 4 | Rust | Systems, edge, security-sensitive services, and growing infrastructure use | Remote SDK first; native embedding only after demand is proven |
| 5 | C++ | Industrial, robotics, appliances, and existing protobuf-heavy systems | Remote SDK; embedded binding is high-cost and should be design-partner driven |
| Later | Swift and Dart | iOS/macOS and cross-platform mobile | Only after offline sync, conflict handling, and mobile lifecycle requirements exist |

PHP and Ruby can initially use generated gRPC clients with examples rather than receive full
official SDK investment.

### “Supported language” must mean more than generated stubs

Each official SDK should include:

- generated API types pinned to a server compatibility range;
- TLS/mTLS setup and pluggable token providers;
- safe defaults for deadlines and message limits;
- typed error mapping;
- pagination iterators;
- retries only for operations with defined idempotency semantics;
- connection lifecycle and health checks;
- OpenTelemetry hooks;
- examples, API reference, test matrix, semantic versioning, and package signing;
- a compatibility table against supported server versions.

### Embedded support across languages

There are three options:

1. **Native in-process SDK per language** — best experience, highest engineering and correctness
   cost; do not duplicate the storage engine in several languages.
2. **Stable C ABI around the Go engine** — possible but introduces cgo, ABI, memory ownership,
   packaging, callback, and platform-support complexity.
3. **Managed local sidecar** — each SDK starts or connects to a local KoraDB process over a Unix
   socket/named pipe and presents the same client API. It is not literally in-process, but it
   preserves “no separately operated database server” and keeps one engine implementation.

Recommended approach: true in-process embedding for Go; a managed local sidecar for TypeScript,
Java/Kotlin, .NET, and Python; native bindings only where customer demand justifies their lifetime
cost.

## Recommended target architecture

```text
Application
   |
   +-- Official SDK ----------------------------------------------+
   |      typed errors, auth, deadlines, paging, telemetry        |
   |                                                              |
   +-- Local provider --> public Go engine --> single data file   |
   |                                                              |
   +-- Remote provider --> gRPC/Connect API --> single-node DB ---+
                                                |
                                                +-- schema catalog
                                                +-- collection catalog
                                                +-- data + indexes
                                                +-- auth/audit metadata
```

Near-term architectural boundaries:

- **Data plane:** document reads/writes, queries, indexes, transactions.
- **Control plane:** schemas, collections, index lifecycle, keys/policies, backup operations.
- **Operational plane:** health, metrics, traces, audit export, configuration, diagnostics.

Long-term replication and offline synchronization should be treated as different products:

- server HA replication needs consensus, leader routing, quorum durability, and failover;
- offline/edge sync needs revisions, checkpoints, tombstones, conflict detection, resolution,
  authorization changes, and partial replication.

Do not assume a Raft log automatically provides an offline-sync model.

## API evolution recommendations

### Keep the stable generic service, add a typed data path

The current JSON API is useful for the CLI and generic tools. Add a versioned binary path:

- document payload as `bytes` plus collection-bound message type;
- response includes primary ID, schema version/fingerprint, and payload;
- typed query literal using a `oneof` for string, signed/unsigned integer, bool, float, bytes,
  enum number, and well-known temporal types;
- limit, page token, ordering, and projection in `Query`;
- batch operations and explicit transaction semantics;
- idempotency key for retryable writes;
- optimistic concurrency version/etag to prevent lost updates;
- standard gRPC health service and optional server reflection;
- schema fetch, history, diff, validate, and compatibility endpoints;
- index create/drop/status/rebuild/verify endpoints;
- backup/restore/verify administrative endpoints.

### Versioning

- Keep `KoraDB.v1` backward compatible once published.
- Use `buf lint` and `buf breaking` in CI against the last released descriptor image.
- Publish generated SDKs from the same tagged API source.
- Separate server version, API version, storage-format version, and SDK version.
- Define minimum/maximum compatibility and deprecation windows.

## Enterprise non-functional requirements

Before using “enterprise-ready,” publish measurable targets for:

| Quality | Required definition |
|---|---|
| Durability | What is durable after success; behavior under process, OS, and power failure |
| Availability | Supported topology, recovery time objective, and failover behavior |
| Recovery | Backup consistency, restore time, point objective, verification, and drills |
| Performance | Supported dataset shape, read/write mix, concurrency, p95/p99, and file growth |
| Scalability | File-size, document-size, collection, index, connection, and result limits |
| Security | Threat model, identity, encryption, rotation, tenant isolation, audit, and patch SLA |
| Compatibility | API, SDK, schema, and on-disk format policies |
| Operability | Health, metrics, tracing, logs, diagnostics, upgrades, rollback, and compaction |
| Supportability | Supported platforms, lifecycle, severity definitions, and response targets |

## Go-to-market sequence

### Design-partner preview

Recruit three to five protobuf-heavy teams with local or edge storage needs. Require at least two
different workloads rather than optimizing for one internal demo.

Measure:

- time to first successful local insert/query;
- time to switch the same sample application to remote mode;
- storage size versus their current JSON representation;
- p50/p95/p99 read and write latency at realistic concurrency;
- backup/restore success and recovery time;
- number of schema changes accepted/rejected correctly;
- operational incidents and manual steps;
- SDK and documentation friction.

### Public preview gate

- all P0 findings fixed with regression tests;
- public Go SDK and one non-Go SDK;
- license and name cleared;
- tagged, signed, reproducible release with checksums and SBOM;
- backup/restore/verify and operator runbook;
- pagination and resource limits;
- health, metrics, and structured audit export;
- compatibility and security policies;
- supported-platform CI green.

### General availability gate

- at least two production design partners;
- published SLO and tested recovery objectives;
- independent security review;
- upgrade/rollback compatibility across supported releases;
- performance and capacity guide;
- no open critical correctness or security findings;
- defined support and patch process.

## External research basis

These sources anchor the market and standards observations:

- [Protobuf language guide: updating message types](https://protobuf.dev/programming-guides/proto3/)
- [Buf breaking-change detection](https://buf.build/docs/breaking/)
- [gRPC supported languages](https://grpc.io/docs/languages/)
- [gRPC health checking](https://grpc.io/docs/guides/health-checking/)
- [gRPC OpenTelemetry metrics](https://grpc.io/docs/guides/opentelemetry-metrics/)
- [bbolt caveats and single-writer model](https://pkg.go.dev/go.etcd.io/bbolt)
- [SQLite: appropriate uses](https://www.sqlite.org/whentouse.html)
- [lowdb project](https://github.com/typicode/lowdb)
- [TinyDB documentation](https://tinydb.readthedocs.io/)
- [MongoDB schema validation](https://www.mongodb.com/docs/manual/core/schema-validation/)
- [Couchbase Lite positioning and capabilities](https://docs.couchbase.com/couchbase-lite/current/index.html)
- [Existing linka-cloud KoraDB project](https://github.com/linka-cloud/koradb)

## Bottom line

KoraDB's idea is commercially interesting when framed as **local-to-service continuity for
schema-governed protobuf data**. The prototype demonstrates enough to justify another development
round. The next round should be a correctness, compatibility, recovery, and SDK round—not a
distributed-systems round.


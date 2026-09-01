# YuvanoLabs KoraDB Architecture

KoraDB is built in layers. Each layer depends only on those below it and was proven with
tests before the next was started. This document explains how each works and why.

```
Layer 4  query     internal/query    AST + index-aware executor
Layer 3  index     internal/index    persistent secondary indexes
Layer 2  engine    internal/engine   collections, documents, schema evolution
Layer 1  schema    internal/schema   runtime .proto compilation + descriptor registry
Layer 0  storage   internal/storage  durable, crash-safe key/value spine (bbolt)
```

The whole database is a **single file** on disk. It currently contains user documents, collection
metadata, schema records, secondary indexes, and API-key records.

## Deployment and data flow

KoraDB has two operational paths today:

```text
Local CLI
   |
   +--> engine --> schema/query/index/storage --> .db file

Remote CLI or generated gRPC client
   |
   +--> TLS + API key --> gRPC server --> engine --> .db file
```

The CLI abstracts both paths. The embedded Go SDK source package is available
at [`sdk/go/koradb`](../sdk/go/koradb) under the canonical module
`github.com/YuvanoLabs/KoraDB`. A stable public Go SDK API and remote SDK
provider remain release work; see [Language integrations](INTEGRATIONS.md).

The remote API uses protobuf messages for RPC envelopes. User documents cross that API as
ProtoJSON strings, then the server validates and converts them to protobuf wire bytes. This is not
yet a binary user-message path.

## Layer 0 — Storage spine (`internal/storage`)

A thin wrapper over [bbolt](https://github.com/etcd-io/bbolt), a pure-Go B+tree with
single-writer / many-reader MVCC. Everything above this layer is bytes-by-key inside named
buckets.

- `Update(fn)` runs a read-write transaction; on success it commits **atomically and durably**
  (bbolt fsyncs before returning). On error or panic it rolls back — the file is untouched.
- `View(fn)` runs a read-only snapshot transaction; many run concurrently with one writer.
- Helpers: `Put`, `Get` (returns a copy; bbolt values are only valid inside the txn),
  `Delete`, `Scan`, `PrefixScan` (for index lookups), `NextSequence` (auto IDs), `DeleteBucket`.

**Why build on bbolt instead of hand-rolling durability?** Crash-safe append logs, WAL, fsync
ordering, and recovery are exactly where databases hide silent data-loss bugs. bbolt's
copy-on-write design and commit protocol provide the durability foundation. We put our novelty in
the protobuf layers rather than inventing a new storage engine. KoraDB must still test and
document behavior across its supported filesystems, operating systems, storage devices, and fault
modes.

### The crash-recovery gate (`test/crash_recovery_test.go`)

A targeted process-termination test. It re-execs the test binary as a child process that:

1. commits one record (durable after `Update` returns), then
2. begins a second write and calls `os.Exit(99)` **inside the transaction**, before commit.

The parent then reopens the same file and asserts: (1) it opens without corruption, (2) the
previously committed record survived the abrupt process termination, and (3) the uncommitted write
left no trace.

This test exits **before the second transaction enters bbolt's commit path**. It does not simulate
a crash during page write/fsync, kernel failure, power loss, controller reordering, disk error, or
filesystem corruption. Those require additional fault-injection and target-platform recovery
tests.

## Layer 1 — Schema registry (`internal/schema`)

This layer is what makes KoraDB a *protobuf* database rather than a key/value store that
happens to hold protobuf bytes.

Protobuf wire bytes are **not self-describing**: `08 96 01` is meaningless without the schema
that says "field 1 is an int32." So the registry:

- compiles `.proto` **source text at runtime** via `bufbuild/protocompile` (pure Go — no
  `protoc` binary, well-known imports like `timestamp.proto` supported),
- serializes the full descriptor closure (`FileDescriptorSet`, topologically sorted so imports
  resolve) and persists it in a reserved `__schemas__` bucket,
- rebuilds a `protoregistry.Files` in memory for fast lookup, and reloads it from disk on open,
- hands out `MessageDescriptor`s and `dynamicpb.Message`s that every higher layer uses.

Re-registering a schema name validates a complete candidate catalog before
activation, rejects known incompatible changes, bumps a version counter, and
stores an immutable history record. Collections continue to use the active
descriptor for their bound message type; schema evolution must preserve the
collection key and index invariants. See [Schema evolution](SCHEMA_EVOLUTION.md)
for the precise compatibility contract.

Runtime compilation receives the submitted file, protobuf standard imports,
and the active registered-schema catalog. A schema may import another
registered user schema by its logical registered name.

## Layer 2 — Document engine (`internal/engine`)

Ties storage + schema together into the database users talk to.

- A **Collection** binds a bucket to a fully-qualified message type (e.g. `example.User`).
  Metadata (type, key strategy, indexes) lives in a reserved `__collections__` bucket.
- **Documents** enter/leave as JSON (parsed and rendered with `protojson` against the schema)
  but are stored as compact protobuf wire bytes in `data:<collection>`.
- **Primary keys**: either auto-generated (`uint64` sequence, stored big-endian so keys sort
  numerically) or taken from a designated field (`--key-field`).
- `Insert` / `Get` / `Update` / `Delete` / `Each` — `Update` and `Delete` load the prior
  document so stale index entries can be removed in the same transaction.
- **`Insert` is create-only.** For field-keyed collections it rejects a duplicate primary key
  with `ErrDuplicateKey` (like MongoDB's `_id`) rather than silently overwriting; use `Update`
  to replace. Auto-keyed collections mint a fresh unique id per insert.
- **Primary-key and indexed fields must be scalars.** Repeated, map, and message fields are
  rejected at collection-create time (their value is a List/Map/Message, not a scalar), and the
  query layer refuses comparisons against them.

### Schema evolution (the differentiator)

A document written under schema v1 stays readable after a **compatible additive** evolution to v2,
because protobuf matches fields by number: newly added fields are absent in old wire bytes and new
records can use them immediately. This case is verified by `engine.TestDocumentSchemaEvolution`
and demonstrable via the CLI.

Arbitrary changes are not safe. Field renumbering, field-number reuse,
incompatible wire-type changes, or changes to key/index fields can break reads
or metadata invariants. KoraDB rejects known incompatible schema replacement;
application teams still own review and rollout of compatible additions. See
[Schema evolution](SCHEMA_EVOLUTION.md) for the required contract.

## Layer 3 — Secondary indexes (`internal/index` + `engine/index.go`)

Lets queries find documents by field value without scanning the whole collection.

- Each indexed field gets a bucket `idx:<collection>:<field>` whose keys are
  `<order-preserving encoded value> 0x00 <primary key>`, values empty. Equality lookup is a
  prefix scan.
- Value encoding is order-preserving (big-endian integers, sign-flipped signed integers,
  raw strings, bool) so the design extends to range scans later.
- **Crucially, index entries are written/deleted in the *same* `bbolt` transaction as the
  document.** Data and indexes therefore commit or roll back together and can never diverge,
  even across a crash.

## Layer 4 — Query (`internal/query`)

A small but real query engine.

- **AST**: `Cmp{Field, Op, Value}` with operators `== != > >= < <=`, composed with `And` / `Or`.
  A nil filter matches everything.
- **Executor**: if the filter has an equality predicate on an indexed field, it seeds its
  candidate set from that index (prefix scan); otherwise it falls back to a full collection
  scan. **Either way the complete filter is re-checked against each candidate**, so results are
  always correct regardless of the access path — verified by `TestIndexAndScanAgree`.
- `Or` is deliberately not index-seeded (a missing branch could drop valid matches), so it uses
  the scan path.

Query literals are typed against the field they compare with (so `"42"` compares as an integer,
not as text).

## Concurrency & durability model (today)

- **Single writer, many readers** (bbolt). Every write is one ACID transaction across data and
  its indexes.
- **Durable on commit** (fsync). Crash-safe by design.

Multi-document transactions, replication, and horizontal scale are roadmap items — see
[ROADMAP.md](ROADMAP.md).

## Current architecture limits and release work

- Query results use canonical document IDs, including auto and supported
  numeric field keys; a replacement update cannot change a field-backed key.
- The legacy unary query API has a 1,000-document materialization limit and
  returns no partial result set when it is exceeded. The core, Go SDK, native
  ABI, gRPC API, and CLI also support explicit bounded pages with opaque
  continuation tokens. Filter limits are enforced by the service.
- Snapshot, verification, and guarded offline restore primitives exist, but
  backup retention, off-host storage, encryption, RPO/RTO objectives,
  compaction/repair policy, and production recovery drills remain release
  work.
- The service is single-node. It has health checks, JSON audit records, and a
  loopback-only Prometheus metrics endpoint, but not distributed tracing,
  diagnostics, external audit delivery, capacity qualification, or HA
  guarantees required for a production support promise.

The complete, evidence-based release gates are in
[Production release plan](PRODUCTION_RELEASE_PLAN.md).

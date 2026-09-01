# KoraDB

KoraDB is a protobuf-native document database from YuvanoLabs. It gives
applications durable local storage in a single database file, explicit protobuf
schema governance, secondary indexes, and an optional secured gRPC service when
multiple processes need shared access.

## Product model

KoraDB supports two deliberate deployment modes:

| Mode | Use it when | What runs |
|---|---|---|
| Embedded | One application owns its data | The application opens a local KoraDB file directly. No server or network port is required. |
| Service | Multiple processes need shared access | `KoraDB-server` owns the database file and exposes a TLS-protected gRPC API. |

Both modes use the same collection, schema, document, query, and file-format
semantics. A database file is a single-writer resource; use service mode rather
than sharing one file between processes or hosts.

## Release status

KoraDB v1.0.0 is approved for Community general availability by YuvanoLabs.
Official artifacts are published only by the protected `v1.0.0` tag workflow;
development builds must not be treated as official artifacts.
KoraDB is one Community product. It will be released through two delivery
variants: developer packages/DLLs and operator installers. Embedded and
secured service are deployment modes available from the same product, not
separate editions. Their production release gates are in
[Production release plan](docs/PRODUCTION_RELEASE_PLAN.md); do not make
availability, encryption, HA, identity, or language-package claims beyond the
evidence recorded there.

KoraDB is published under [Apache-2.0](LICENSE). Public contribution,
security, support, privacy, governance, and lifecycle policies are available
in [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md),
[SUPPORT.md](SUPPORT.md), [PRIVACY.md](PRIVACY.md),
[GOVERNANCE.md](GOVERNANCE.md), and [LIFECYCLE.md](LIFECYCLE.md).

KoraDB is published under [Apache-2.0](LICENSE). Public contribution,
security, support, privacy, governance, and lifecycle policies are available
in [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md),
[SUPPORT.md](SUPPORT.md), [PRIVACY.md](PRIVACY.md),
[GOVERNANCE.md](GOVERNANCE.md), and [LIFECYCLE.md](LIFECYCLE.md).

## What KoraDB provides

- Protobuf schema registration and immutable schema history.
- Atomic collection and document CRUD operations.
- Primary keys, persistent secondary indexes, and bounded scalar queries with
  opaque continuation-token pagination.
- Consistent database snapshots, integrity verification, and safe offline
  restore with explicit rollback protection.
- An embedded Go SDK source package and a versioned native C ABI foundation for
  language bindings.
- An optional gRPC service with TLS/mTLS, API-key authentication, role-based
  access control, request limits, filter limits, health checks, JSON audit
  logs, and loopback-only Prometheus metrics.

## Quick start: embedded database

```sh
# Build the CLI
go build -o bin/KoraDB ./cmd/KoraDB

# Register a schema and create a collection
./bin/KoraDB --db users.db schema add user.proto ./examples/user.proto
./bin/KoraDB --db users.db collection create users example.User --key-field=email --index=city

# Write and query protobuf-backed documents through JSON
./bin/KoraDB --db users.db insert users '{"name":"Alice","email":"alice@example.com","city":"Pune"}'
./bin/KoraDB --db users.db query users city == Pune
```

Use `KoraDB backup <snapshot.db>` to create a consistent snapshot and restore it
only while the destination database is offline:

```sh
KoraDB-restore --max-bytes 1073741824 snapshot.db restored-users.db
KoraDB-restore --max-bytes 1073741824 --overwrite --rollback users-before-restore.db snapshot.db users.db
```

See [offline restore](docs/RESTORE.md) for the safety model.

## Run a secured service

```sh
# Generate development certificates. Use organization-managed PKI in production.
./KoraDB-server gencert --dir certs --host your.host,127.0.0.1

# Create the first administrator while the server is stopped.
./KoraDB-server bootstrap --db data.db --name admin --role admin
# Prints a token once, for example: kdb_<id>_<secret>

# Start the server with TLS.
./KoraDB-server serve --addr :50051 --db data.db \
  --tls-cert certs/server.crt --tls-key certs/server.key

# Connect with the CLI.
export KoraDB_TOKEN=kdb_<id>_<secret>
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt schema add user.proto ./examples/user.proto
```

Use `--tls-client-ca` to require client certificates. The server refuses a
network listener without TLS and an administrator key; `--insecure` is limited
to explicit loopback development addresses.

## Integrations

- **Go embedded SDK:** source is available in
  [`sdk/go/koradb`](sdk/go/koradb). It opens a local database file without a
  server process.
- **Native ABI:** source and C header are available in
  [`sdk/native`](sdk/native). The ABI is the foundation for Python, Node.js,
  and .NET packages.
- **gRPC:** the service contract is
  [`api/proto/kora_db.proto`](api/proto/kora_db.proto) for controlled service
  deployments and generated clients.

Language-specific integration guidance, package scope, and lifecycle rules are
in [Language integrations](docs/INTEGRATIONS.md).

## Architecture

```text
Application
  |
  +-- Embedded SDK --> KoraDB engine --> local database file
  |
  +-- gRPC client ----> KoraDB server --> local database file
```

The storage engine is layered for durability and schema-aware queries:

```text
Layer 4  query        bounded predicates and index-aware execution
Layer 3  index        persistent secondary indexes maintained transactionally
Layer 2  engine       collections, documents, JSON-to-protobuf conversion
Layer 1  schema       runtime .proto compilation and descriptor registry
Layer 0  storage      bbolt-backed durable database file
```

Read [Architecture](docs/ARCHITECTURE.md), [Schema evolution](docs/SCHEMA_EVOLUTION.md),
[Security](docs/SECURITY.md), [Operations](docs/OPERATIONS.md), and
[Offline restore](docs/RESTORE.md) for
the corresponding contracts.

## Use cases

KoraDB is designed for application-owned state in desktop software, developer
tools, edge gateways, appliances, air-gapped systems, protobuf/gRPC services,
and control-plane components with durable local data and modest write
concurrency.

KoraDB is not a distributed database, analytics engine, SQL database, mobile
synchronization system, or automatic multi-region failover product. Use a
system designed for those workloads when those capabilities are required.

## Build and verify

```sh
go test ./...
go vet ./...
go build ./...
buf lint
```

`KoraDB version` and `KoraDB-server version` report the embedded version, commit,
and build timestamp. Release builds produce checksums for their artifacts.

## Documentation

- [Documentation index](docs/README.md)
- [Product architecture and adoption](docs/PRODUCT_ASSESSMENT.md)
- [Language integrations](docs/INTEGRATIONS.md)
- [Installation](docs/INSTALLATION.md)
- [Operations runbook](docs/OPERATIONS.md)
- [Production release plan](docs/PRODUCTION_RELEASE_PLAN.md)
- [Production readiness](docs/ENTERPRISE_READINESS.md)
- [Product roadmap](docs/ROADMAP.md)
- [Productization program](docs/PRODUCTIZATION_PROGRAM.md)
- [Public-release owner checklist](PUBLIC_REPOSITORY_CHECKLIST.md)

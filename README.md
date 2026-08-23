# YuvanoLabs KoraDB

A **protobuf-native local document store with an optional gRPC server**, written in Go.

Where JSON document stores (NeDB, lowdb, TinyDB) and MongoDB store self-describing
records (every field name repeated in every document), KoraDB stores documents as
**protobuf wire bytes** — field *numbers*, not field names, go to disk. This creates two
potential advantages:

1. **Less repeated field-name overhead** — the actual saving depends on the document shape and
   must be measured on a representative dataset.
2. **Compatible schema evolution without rewriting every document** — additive protobuf changes
   can leave existing records readable because protobuf matches fields by number.

The trade-off protobuf forces — wire bytes are *not* self-describing — is handled by a
**schema registry** that compiles `.proto` files at runtime (pure Go, no `protoc` binary)
and stores the descriptors alongside the data, so documents remain fully queryable by field.

> **Status: engineering prototype.** The repository contains a crash-safe single-node engine,
> local CLI, and secured gRPC server with TLS/mTLS, API-key authentication, coarse RBAC, and audit
> lines. It is suitable for evaluation, not production or untrusted internet exposure. The public
> embedded Go SDK, backup/restore, pagination, compatibility enforcement, observability, and
> release hardening are not complete. Start with the
> [product assessment](docs/PRODUCT_ASSESSMENT.md) and [roadmap](docs/ROADMAP.md).
>
> **Naming status:** `YuvanoLabs KoraDB` is a provisional brand. The unqualified `KoraDB` name is
> crowded, and public releases are blocked until formal trademark clearance, namespace reservations,
> and the source migration are complete. See the [naming and namespace review](docs/NAMING_AND_NAMESPACES.md).

The intended product direction is one logical data API with two deployment providers:

- **local:** the application owns a single database file;
- **service:** clients connect to a secured KoraDB server.

Today the CLI demonstrates both paths. A stable public Go package that lets external applications
embed the engine directly is a roadmap deliverable; current engine packages are under Go's
`internal/` boundary.

## Deployment: standalone executables, no runtime to install

KoraDB is **pure Go with no cgo**, so each command compiles to a **standalone executable** with
every dependency (storage engine, protobuf runtime, gRPC) baked in. There is **no runtime to
install**, no shared libraries, and no `protoc` on the target — unlike MongoDB. Deploying means
copying the CLI and/or server executable needed by that deployment.

It cross-compiles from any host to Linux, macOS, and Windows on amd64/arm64:

```sh
# Build static binaries for every platform into dist/
CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -o dist/KoraDB-server-linux-amd64  ./cmd/KoraDB-server
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/KoraDB-server-darwin-arm64 ./cmd/KoraDB-server
# ...or use scripts/build-release.ps1 -Development for internal test artifacts.
# A normal release build is blocked until the product identity is cleared.
```

### Run it as a server (the mongod equivalent) — secured

The server is **fail-closed**: it refuses to start without TLS and at least one API key (unless
you pass `--insecure` for localhost dev, which prints a loud warning). See
[docs/SECURITY.md](docs/SECURITY.md) for the full model.

```sh
# 1. Generate dev TLS certs (use your org PKI in production)
./KoraDB-server gencert --dir certs --host your.host,127.0.0.1

# 2. Create the first admin key (server must be stopped — the db file is exclusively locked)
./KoraDB-server bootstrap --db data.db --name admin --role admin
#    -> prints a token ONCE, e.g. pdb_<id>_<secret>

# 3. Start the secured server
./KoraDB-server serve --addr :50051 --db data.db \
    --tls-cert certs/server.crt --tls-key certs/server.key
#    add --tls-client-ca certs/ca.crt to require client certs (mTLS)

# 4. Use the CLI as a client (token via --token or KoraDB_TOKEN)
export KoraDB_TOKEN=pdb_<id>_<secret>
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt schema add user.proto ./examples/user.proto
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt collection create users example.User --key-field=email --index=city
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt insert users '{"name":"Alice","email":"a@x.com","city":"NYC"}'
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt query users city == NYC

# 5. Manage keys at runtime (admin only), e.g. a read-only key for reporting
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt key create reporting readonly
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt key list
./KoraDB --server your.host:50051 --tls-ca certs/ca.crt key revoke <key-id>
```

**Roles:** `readonly` (Get/Query/List) ⊂ `readwrite` (+Insert/Update/Delete) ⊂ `admin`
(+schema/collection/key management). Every request is audited (who/method/outcome/peer) with no
document contents logged.

Any supported gRPC language can generate a raw client from
[`api/proto/KoraDB.proto`](api/proto/KoraDB.proto). Generated stubs are not the same as a
supported KoraDB SDK: official SDKs still need connection policy, typed errors, paging, safe
retries, telemetry, tests, and a compatibility promise. See the
[language strategy](docs/PRODUCT_ASSESSMENT.md#language-and-sdk-strategy).

The current RPC envelope is protobuf, but document payloads are carried as ProtoJSON strings and
converted to protobuf bytes by the server. A future versioned API will add a binary typed document
path while retaining JSON for the CLI and generic tools.

## Why it's built this way

| Concern | Choice | Rationale |
|---|---|---|
| Language | Go | First-class protobuf/gRPC; single-binary deploy; the modern DB ecosystem (etcd, CockroachDB) is Go |
| Durability | [bbolt](https://github.com/etcd-io/bbolt) B+tree | Crash-safe by design (copy-on-write + fsync per commit). We build *on* a proven engine rather than hand-rolling durability, where silent data-loss bugs live. |
| Schema | [protocompile](https://github.com/bufbuild/protocompile) | Compiles `.proto` at runtime in pure Go — no external toolchain |
| Encoding | `google.golang.org/protobuf` (`dynamicpb`, `protoreflect`) | Encode/decode/inspect documents from stored descriptors |

## Architecture (layered, each proven before the next)

```
Layer 4  query        AST (eq/ne/gt/gte/lt/lte, AND/OR) + index-aware executor
Layer 3  index        persistent secondary indexes, maintained in the write txn
Layer 2  engine       collections, documents, JSON<->protobuf, schema evolution
Layer 1  schema       runtime .proto compilation + descriptor registry
Layer 0  storage      bbolt wrapper — durable, crash-safe key/value spine
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details, including the crash-recovery test and
the current concurrency model. See [docs/SCHEMA_EVOLUTION.md](docs/SCHEMA_EVOLUTION.md) for the
schema changes that can and cannot safely avoid a document rewrite.

## Quick start

```sh
# Build the CLI
go build -o bin/KoraDB ./cmd/KoraDB

# 1. Register a schema (compiled at runtime)
./bin/KoraDB --db users.db schema add user.proto ./examples/user.proto

# 2. Create a collection: primary key = email, secondary index on city
./bin/KoraDB --db users.db collection create users example.User --key-field=email --index=city

# 3. Insert documents (JSON in, protobuf bytes on disk)
./bin/KoraDB --db users.db insert users '{"name":"Alice","email":"alice@x.com","city":"NYC"}'
./bin/KoraDB --db users.db insert users '{"name":"Carol","email":"carol@x.com","city":"NYC"}'

# 4. Query by the indexed field
./bin/KoraDB --db users.db query users city == NYC

# 5. Make a compatible additive schema change — no existing-document rewrite
./bin/KoraDB --db users.db schema add user.proto ./examples/user_v2.proto
./bin/KoraDB --db users.db get users alice@x.com    # old record still reads fine
./bin/KoraDB --db users.db insert users '{"name":"Dave","email":"dave@x.com","age":42,"roles":["admin"]}'
```

A full scripted walkthrough lives in [`scripts/demo.sh`](scripts/demo.sh) /
[`scripts/demo.ps1`](scripts/demo.ps1).

## Testing

```sh
go test ./...                              # everything
go test ./test -run CrashRecovery -v       # durability gate: kill mid-write, reopen, verify
go test ./internal/engine -run Evolution   # schema evolution: v1 docs readable under v2
```

The crash-recovery test launches the test binary as its own crash victim, hard-exits inside an
uncommitted write, then reopens the file and asserts that previously committed data survived, the
uncommitted write left no trace, and the file opens cleanly. It does not simulate a failure during
commit/fsync or physical power loss; those are roadmap test gates.

## Project layout

```
cmd/KoraDB/         CLI — embedded (--db) or remote client (--server)
cmd/KoraDB-server/  gRPC server daemon
api/proto/           KoraDB.proto — the gRPC service contract
api/gen/             generated Go stubs (committed; no protoc needed to build)
internal/storage/    Layer 0 — bbolt wrapper
internal/schema/     Layer 1 — runtime .proto registry
internal/engine/     Layer 2 — collections & documents  (+ Layer 3 index maintenance)
internal/index/      Layer 3 — index key encoding
internal/query/      Layer 4 — query AST & executor
internal/server/     gRPC service + auth/authz/audit interceptors
internal/auth/       API keys (SHA-256), roles, fail-closed RBAC policy
internal/certgen/    dev TLS certificate generation
examples/            sample .proto schemas (v1 and evolved v2)
test/                integration tests (crash recovery, gRPC, security denials)
docs/                architecture, security, schema contract, assessment, and roadmap
product.identity.yaml provisional public coordinates and fail-closed launch approvals
```

## Regenerating the gRPC stubs (only if you change `api/proto/KoraDB.proto`)

```sh
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
buf generate        # pure Go — no protoc binary required
```

## Licensing and naming

bbolt (MIT); `google.golang.org/protobuf`, `google.golang.org/grpc`, `bufbuild/protocompile`
(Apache-2.0 / BSD-3) use permissive licenses. Confirm the exact dependency graph with your
organization's OSS/legal process before shipping.

This repository copy does not yet contain a project `LICENSE`; adding one is a public-release
gate. A dated name and namespace screen has been completed, and it found substantial collision
risk for the unqualified `KoraDB` name. Formal legal clearance, legal-owner confirmation, and
actual reservations remain pending. The decision, exact provisional coordinates, evidence, and owner
checklist are in the
[naming and namespace review](docs/NAMING_AND_NAMESPACES.md); the machine-readable release state
is in [`product.identity.yaml`](product.identity.yaml).

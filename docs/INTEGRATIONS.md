# KoraDB Language Integrations

KoraDB is designed to be embedded directly into an application or accessed
through its secured gRPC service. Embedded use owns a local database file and
does not install, start, or hide a server process.

## Choose an integration mode

| Need | Integration | Server required? |
|---|---|---|
| One application owns local data | Embedded SDK or native binding | No |
| Desktop, appliance, CLI, test, or air-gapped software | Embedded SDK or native binding | No |
| Multiple processes share one database | gRPC service client | Yes |
| Centralized access policy and audit operations | gRPC service client | Yes |

## Go embedded SDK

The embedded Go SDK source is available at
[`sdk/go/koradb`](../sdk/go/koradb). It wraps the local KoraDB engine, opens a
single database file in-process, and provides schema registration, collection
operations, CRUD, scalar queries, backup, verification, and version reporting.

Use the SDK when Go code owns the database lifecycle. Do not open the same file
from a Go process while KoraDB-server owns it.

## Native ABI

The pre-release C ABI source and header are at [`sdk/native`](../sdk/native).
It uses opaque database handles and caller-owned strings, and is built as a
platform-native shared library. The ABI is the foundation for idiomatic Python,
Node.js, and .NET embedded packages.

Binding authors must preserve explicit resource ownership, database locking,
structured errors, size limits, cancellation policy, backup and restore
safeguards, and ABI compatibility checks.

## Python, Node.js, and .NET

KoraDB will publish functional packages for these ecosystems only when each
package contains a local database API, matching native assets, platform support
matrix, lifecycle semantics, typed errors, examples, and contract tests.

- **Python:** a wheel-based package with CPython-native assets and explicit
  context-manager lifecycle.
- **Node.js:** an asynchronous N-API package with prebuilt binaries and
  TypeScript definitions.
- **.NET:** a NuGet package with safe handles, `IDisposable`/
  `IAsyncDisposable`, cancellation, and runtime-specific native assets.

Until a package is published, use the gRPC service contract for remote language
access or integrate directly with the native ABI in a controlled build.

## gRPC service integration

The protocol is defined by [`api/proto/kora_db.proto`](../api/proto/kora_db.proto).
Service clients must use TLS, configured deadlines, explicit credential handling,
and bounded request behavior. gRPC is a shared-access deployment option; it is
not a substitute for an embedded package when an application owns local data.

## Package and installer policy

Packages are for application integration. Installers are for operators and
developers who need the CLI, server, offline restore tool, or a platform-native
service wrapper. Every released package and installer must include checksums,
version compatibility, security update information, and release notes.

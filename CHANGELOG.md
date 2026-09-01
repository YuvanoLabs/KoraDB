# Changelog

All notable KoraDB releases are documented here.

## 1.0.0 — 2026-09-01

First Community general-availability release.

- One Community product delivered as developer package/DLL artifacts and
  operator installer archives.
- Embedded Go SDK, native C ABI, CLI, secure single-node gRPC server, backup,
  verification, and guarded offline restore.
- TLS/mTLS, API-key roles and expiry, strict bearer handling, JSON audit logs,
  request deadlines, concurrency/rate controls, and loopback Prometheus
  metrics.
- Opaque continuation-token pagination across Go, native C, gRPC, and CLI.
- YuvanoLabs canonical Go/protobuf source coordinates, release automation,
  SBOM/provenance generation, archive checksums, native DLL smoke testing, and
  verified Windows/Linux/macOS installer archives.

Known product boundaries remain documented in [README.md](README.md),
[SECURITY.md](SECURITY.md), and [docs/OPERATIONS.md](docs/OPERATIONS.md):
KoraDB is a single-node database and does not claim HA, replication, sharding,
multi-tenancy, at-rest encryption, or unreleased language packages.

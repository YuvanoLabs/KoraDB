# KoraDB v1.0.0 release decision

**Decision date:** 2026-09-01  
**Publisher:** YuvanoLabs  
**Decision:** Approve KoraDB v1.0.0 Community general availability.

The product owner confirmed approval of the KoraDB name, YuvanoLabs release
ownership, the public repository, and the GA launch. The protected `v1.0.0`
tag workflow is the sole publication path for release artifacts.

## Release evidence completed locally

- `go test ./...`, `go test -race ./...`, `go vet ./...`, and `buf lint`
- Native Windows DLL build and C-consumer ABI smoke test
- Cross-platform development release build for Linux amd64/arm64, macOS
  amd64/arm64, Windows amd64, and the Windows amd64 native DLL package
- Checksum, manifest, Windows installer, and POSIX installer verification

## Known boundaries carried into the release

KoraDB is a secure, single-node embedded/service database. It does not claim
high availability, replication, sharding, multi-tenancy, at-rest encryption,
external identity federation, or supported Python/Node.js/.NET packages.
Operators remain responsible for supported-environment qualification, backup
retention, off-host encryption, recovery objectives, and their own security
review. These are documented product boundaries, not hidden feature gaps.

# KoraDB Embedded Go SDK

`sdk/go/koradb` is the first public-facing embedded SDK source for KoraDB. It
opens a database file directly in the application process and requires no
KoraDB server, port, container, or external runtime.

## Current status

This SDK uses the canonical source module
`github.com/YuvanoLabs/KoraDB`, but remains pre-release until its public API
compatibility policy, signed release, and package-install evidence are
complete. Do not depend on it as a production package yet.

Its API covers schema registration, collection creation, CRUD, scalar queries
with opaque continuation-token pagination, consistent snapshot export, and
storage-integrity verification using ProtoJSON. The implementation deliberately
does not claim transactions, restore verification, remote-provider parity, or
typed protobuf payloads until those contracts are complete.

See [language integrations](../../docs/INTEGRATIONS.md) for the multi-language
package plan and [the productization program](../../docs/PRODUCTIZATION_PROGRAM.md)
for the release gates.

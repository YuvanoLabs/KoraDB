# KoraDB Embedded Go SDK

`sdk/go/koradb` is the first public-facing embedded SDK source for KoraDB. It
opens a database file directly in the application process and requires no
KoraDB server, port, container, or external runtime.

## Current status

This SDK is pre-release and is built from this repository while KoraDB's
canonical public Go module path, compatibility policy, release signing, and
package publication are completed. Do not depend on an unversioned source path
as a production package yet.

Its API covers schema registration, collection creation, CRUD, scalar queries,
consistent snapshot export, and storage-integrity verification using ProtoJSON.
The implementation deliberately does not claim stable pagination, transactions,
restore verification, remote-provider parity, or typed protobuf payloads until
those contracts are complete.

See [language integrations](../../docs/INTEGRATIONS.md) for the multi-language
package plan and [the productization program](../../docs/PRODUCTIZATION_PROGRAM.md)
for the release gates.

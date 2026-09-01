# KoraDB Product Identity and Release Coordinates

## Product identity

| Item | Value |
|---|---|
| Product | KoraDB |
| Publisher | YuvanoLabs |
| Product category | Protobuf-native embedded and single-node service database |
| Primary user promise | Durable, schema-governed document data without requiring a separate server for local use |

Use **KoraDB** as the product name and **YuvanoLabs** as the publisher name in
product documentation, examples, package metadata, and release material.

## Technical naming

- CLI: `KoraDB`
- Service: `KoraDB-server`
- Offline restore: `KoraDB-restore`
- API-key prefix: `kdb_`
- gRPC protocol package: `yuvanolabs.koradb.v1`
- Protocol source: `api/proto/kora_db.proto`

These names form the current KoraDB product surface and must be applied
consistently across code, release artifacts, installers, documentation, and
language integrations.

## Release coordination

KoraDB uses the same YuvanoLabs ownership, Apache-2.0 licensing, and public
repository policy baseline as the released Causentra product. Formal clearance
of the distinct KoraDB product name, artifact signing, and release approvals
remain release-management responsibilities and are tracked in
`product.identity.yaml` and release automation.

When public package coordinates are approved, migrate all import, artifact, and
registry metadata in one versioned release. Do not publish placeholder packages
or use employee-owned namespaces for product artifacts.

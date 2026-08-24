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
- gRPC protocol package: `koradb.v1`
- Protocol source: `api/proto/kora_db.proto`

These names form the current KoraDB product surface and must be applied
consistently across code, release artifacts, installers, documentation, and
language integrations.

## Release coordination

Public registry ownership, legal review, license selection, artifact signing,
and release approvals are release-management responsibilities. They are
maintained as internal release records in `product.identity.yaml` and release
automation rather than presented as product positioning.

When public package coordinates are approved, migrate all import, artifact, and
registry metadata in one versioned release. Do not publish placeholder packages
or use employee-owned namespaces for product artifacts.

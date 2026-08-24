# KoraDB Documentation

KoraDB is the protobuf-native embedded and single-node service database from
YuvanoLabs. These documents describe the product, how to operate it, and how to
integrate it into an application.

## Start here

1. [Root README](../README.md) - deployment modes and quick start.
2. [Product architecture and adoption](PRODUCT_ASSESSMENT.md) - product model,
   best-fit workloads, and operating boundaries.
3. [Architecture](ARCHITECTURE.md) - storage, schemas, documents, indexes, and
   query execution.
4. [Schema evolution](SCHEMA_EVOLUTION.md) - compatible and incompatible schema
   changes.
5. [Security](SECURITY.md) - TLS, authentication, authorization, and audit
   behavior.
6. [Offline restore](RESTORE.md) - snapshot validation, restore, and rollback
   safety.
7. [Language integrations](INTEGRATIONS.md) - embedded, native ABI, and gRPC
   integration guidance.
8. [Product roadmap](ROADMAP.md) - product delivery programs and priorities.

## Product identity

- **Product:** KoraDB
- **Publisher:** YuvanoLabs
- **Deployment:** embedded local database or secured single-node gRPC service

## Documentation standards

Product documentation describes implemented behavior and operational
responsibilities. Package availability, support commitments, and release
artifacts are stated only when the corresponding package or artifact exists.
Internal release coordination is maintained separately from product guidance.

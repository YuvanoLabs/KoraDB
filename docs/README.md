# YuvanoLabs KoraDB Documentation

KoraDB is currently an engineering prototype. Read the documents in this order:

1. [Naming and namespace launch review](NAMING_AND_NAMESPACES.md) — current naming decision,
   collision evidence, provisional coordinates, and reserved release gates.
2. [Product and enterprise assessment](PRODUCT_ASSESSMENT.md) — target users, benefits,
   competitive position, source-confirmed gaps, SDK priorities, and launch gates.
3. [Roadmap](ROADMAP.md) — prioritized development rounds and exit criteria.
4. [Architecture](ARCHITECTURE.md) — implemented layers, deployment paths, persistence, query
   model, and current constraints.
5. [Schema evolution contract](SCHEMA_EVOLUTION.md) — safe changes, rejected changes, versioning,
   imports, document envelopes, and index implications.
6. [Security model](SECURITY.md) — implemented controls, trust boundaries, tests, limitations, and
   prototype hazards.

## Status vocabulary

| Label | Meaning |
|---|---|
| Implemented | Source exists in this repository. It may still have documented defects or missing operational hardening. |
| Verified by source test | A test exists for the stated property. This does not mean it ran in every supported environment. |
| Reproduced in this assessment | The behavior was observed with the Windows executables in this workspace. |
| Roadmap | Not implemented; do not design a production dependency around it. |
| Release gate | Required before the named release stage. |

## Product promise today

The current prototype demonstrates a local single-file protobuf document engine and an optional
secured gRPC service. It does not yet promise:

- a public embedded SDK;
- arbitrary schema evolution;
- backup/restore or point-in-time recovery;
- pagination or bounded multi-tenant queries;
- at-rest encryption;
- high availability, offline synchronization, or sharding;
- production or enterprise readiness.

Use the [root README](../README.md) for the quick start.

